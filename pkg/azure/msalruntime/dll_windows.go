//go:build windows

package msalruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// dllName is the broker DLL we bind to. Windows builds embed Microsoft's copy
// (see embed_windows_*.go) and extract it on first use, but a copy already on
// the machine wins: AZ_MSALRUNTIME_DLL, then one sitting next to our own
// executable. That order lets a user pin a newer or older broker without
// rebuilding.
const dllName = "msalruntime.dll"

// dllPath resolves an already-present DLL: an explicit override, else a copy
// sitting next to our own executable. Returns "" when neither exists, in
// which case Startup extracts the embedded copy.
//
// Deliberately cheap: this runs at package init on every command, so it only
// stats, never writes. Extraction is deferred to Startup, which only the
// broker path calls.
//
// Deliberately never falls back to the bare name: the standard LoadLibrary
// search order includes the current directory, so a bare name both makes
// broker availability depend on where the user happens to be cd'd and opens a
// DLL preloading hole. It also wouldn't find the copies Office, Teams and the
// Python Azure CLI install, which all live in app-private directories that
// are not on the search path.
func dllPath() string {
	if p := os.Getenv("AZ_MSALRUNTIME_DLL"); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), dllName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

var resolvedDLLPath = dllPath()

var dll = windows.NewLazyDLL(resolvedDLLPath)

// boundProcs is every MSALRuntime export this package calls. Startup resolves
// them all up front: windows.LazyProc.Call panics when an export is missing,
// and the DLL version on any given machine is out of our control, so an older
// copy must make the broker unavailable rather than abort the process.
var boundProcs []*windows.LazyProc

func newProc(name string) *windows.LazyProc {
	p := dll.NewProc(name)
	boundProcs = append(boundProcs, p)
	return p
}

// All MSALRuntime handles are opaque pointer-sized values passed by value;
// functions that produce one take a pointer to it as an out-param. Distinct
// Go types keep them from being mixed up.
type (
	errorHandle             uintptr
	asyncHandle             uintptr
	authParamsHandle        uintptr
	accountHandle           uintptr
	authResultHandle        uintptr
	readAccountResultHandle uintptr
)

var (
	procStartup                   = newProc("MSALRUNTIME_Startup")
	procReadAccountByIdAsync      = newProc("MSALRUNTIME_ReadAccountByIdAsync")
	procSignInInteractivelyAsync  = newProc("MSALRUNTIME_SignInInteractivelyAsync")
	procAcquireTokenSilentlyAsync = newProc("MSALRUNTIME_AcquireTokenSilentlyAsync")

	procReleaseAccount = newProc("MSALRUNTIME_ReleaseAccount")
	procGetAccountId   = newProc("MSALRUNTIME_GetAccountId")
	procGetClientInfo  = newProc("MSALRUNTIME_GetClientInfo")

	procCreateAuthParameters  = newProc("MSALRUNTIME_CreateAuthParameters")
	procReleaseAuthParameters = newProc("MSALRUNTIME_ReleaseAuthParameters")
	procSetRequestedScopes    = newProc("MSALRUNTIME_SetRequestedScopes")
	procSetRedirectUri        = newProc("MSALRUNTIME_SetRedirectUri")

	procReleaseAsyncHandle   = newProc("MSALRUNTIME_ReleaseAsyncHandle")
	procCancelAsyncOperation = newProc("MSALRUNTIME_CancelAsyncOperation")

	procReleaseError = newProc("MSALRUNTIME_ReleaseError")
	procGetStatus    = newProc("MSALRUNTIME_GetStatus")
	procGetErrorCode = newProc("MSALRUNTIME_GetErrorCode")
	procGetTag       = newProc("MSALRUNTIME_GetTag")
	procGetContext   = newProc("MSALRUNTIME_GetContext")

	procReleaseAuthResult      = newProc("MSALRUNTIME_ReleaseAuthResult")
	procGetAccount             = newProc("MSALRUNTIME_GetAccount")
	procGetRawIdToken          = newProc("MSALRUNTIME_GetRawIdToken")
	procGetAccessToken         = newProc("MSALRUNTIME_GetAccessToken")
	procGetError               = newProc("MSALRUNTIME_GetError")
	procIsPopAuthorization     = newProc("MSALRUNTIME_IsPopAuthorization")
	procGetAuthorizationHeader = newProc("MSALRUNTIME_GetAuthorizationHeader")

	procReleaseReadAccountResult = newProc("MSALRUNTIME_ReleaseReadAccountResult")
	procGetReadAccount           = newProc("MSALRUNTIME_GetReadAccount")
	procGetReadAccountError      = newProc("MSALRUNTIME_GetReadAccountError")
)

var (
	startupOnce sync.Once
	startupErr  error
)

// Startup loads msalruntime.dll and calls MSALRUNTIME_Startup once per
// process. Every other entry point calls it first.
func Startup() error {
	startupOnce.Do(func() {
		if resolvedDLLPath == "" {
			// Nothing on the machine to prefer, so use our own copy. Safe to
			// retarget the LazyDLL here: nothing loads it before Startup.
			path, err := extractEmbeddedDLL()
			if err != nil {
				startupErr = err
				return
			}
			resolvedDLLPath = path
			dll.Name = path
		}
		if err := dll.Load(); err != nil {
			startupErr = fmt.Errorf("%w: %v", ErrNotAvailable, err)
			return
		}
		for _, p := range boundProcs {
			if err := p.Find(); err != nil {
				startupErr = fmt.Errorf("%w: %v", ErrNotAvailable, err)
				return
			}
		}
		r, _, _ := procStartup.Call()
		if err := toError(errorHandle(r)); err != nil {
			startupErr = fmt.Errorf("%w: %v", ErrNotAvailable, err)
		}
	})
	return startupErr
}

// --- error handling -------------------------------------------------------

// toError converts a returned MSALRUNTIME_ERROR_HANDLE into a Go error and
// releases it. A zero handle means success.
func toError(h errorHandle) error {
	if h == 0 {
		return nil
	}
	defer func() { discard(release(procReleaseError, uintptr(h))) }()

	e := &Error{}
	var status, tag int32
	var code int64
	discard(errorHandle(callRet(procGetStatus, uintptr(h), uintptr(unsafe.Pointer(&status)))))
	discard(errorHandle(callRet(procGetTag, uintptr(h), uintptr(unsafe.Pointer(&tag)))))
	discard(errorHandle(callRet(procGetErrorCode, uintptr(h), uintptr(unsafe.Pointer(&code)))))
	e.Status, e.Tag, e.Code = Status(status), tag, code
	e.Context = readString(func(buf *uint16, size *int32) errorHandle {
		return errorHandle(callRet(procGetContext, uintptr(h), uintptr(unsafe.Pointer(buf)), uintptr(unsafe.Pointer(size))))
	})
	return e
}

// callRet forwards to LazyProc.Call. Callers convert unsafe.Pointer to
// uintptr in this function's argument list, so it needs the same
// //go:uintptrescapes contract LazyProc.Call carries: the referents are
// forced to the heap and kept alive for the duration of the call.
//
//go:uintptrescapes
func callRet(p *windows.LazyProc, args ...uintptr) uintptr {
	r, _, _ := p.Call(args...)
	return r
}

func release(p *windows.LazyProc, h uintptr) errorHandle {
	if h == 0 {
		return 0
	}
	return errorHandle(callRet(p, h))
}

// discard drops an error handle we can do nothing about (release paths, and
// the error-parsing calls themselves, where reporting would recurse).
func discard(h errorHandle) {
	if h != 0 {
		callRet(procReleaseError, uintptr(h))
	}
}

// --- strings --------------------------------------------------------------

// readString runs MSALRuntime's two-call UTF-16 out-param convention: the
// first call passes a nil buffer and gets back the required length in
// characters (with status INSUFFICIENTBUFFER), the second fills the buffer.
// Errors yield "" - every caller treats an empty value as the failure.
func readString(get func(buf *uint16, size *int32) errorHandle) string {
	var size int32
	first := get(nil, &size)
	if first == 0 || size <= 0 {
		discard(first)
		return ""
	}
	var status int32
	discard(errorHandle(callRet(procGetStatus, uintptr(first), uintptr(unsafe.Pointer(&status)))))
	discard(first)
	if Status(status) != StatusInsufficientBuffer {
		return ""
	}

	buf := make([]uint16, size)
	second := get(&buf[0], &size)
	runtime.KeepAlive(buf)
	if second != 0 {
		discard(second)
		return ""
	}
	return windows.UTF16ToString(buf)
}

func utf16Ptr(s string) (*uint16, error) {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return nil, fmt.Errorf("msalruntime: invalid string argument: %w", err)
	}
	return p, nil
}

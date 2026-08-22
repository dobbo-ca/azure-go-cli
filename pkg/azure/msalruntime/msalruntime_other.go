//go:build !windows

package msalruntime

import (
	"context"
	"fmt"
	"runtime"
)

func unsupported() error {
	return fmt.Errorf("%w: the WAM broker is Windows-only (running on %s)", ErrNotAvailable, runtime.GOOS)
}

// Startup always fails off Windows.
func Startup() error { return unsupported() }

// SignInInteractively always fails off Windows.
func SignInInteractively(ctx context.Context, authority, clientID string, scopes []string, parentHWND uintptr) (*AuthResult, error) {
	return nil, unsupported()
}

// AcquireTokenSilently always fails off Windows.
func AcquireTokenSilently(ctx context.Context, account *Account, authority, clientID string, scopes []string) (*AuthResult, error) {
	return nil, unsupported()
}

// ReadAccountByID always fails off Windows.
func ReadAccountByID(ctx context.Context, accountID string) (*Account, error) {
	return nil, unsupported()
}

// Close is a no-op off Windows.
func (a *Account) Close() {}

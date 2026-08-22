// Package msalruntime is a thin Go binding over Microsoft's msalruntime.dll
// (the WAM broker) on Windows.
//
// The DLL is never shipped with this CLI - its license forbids
// redistribution. The binding loads whatever copy is already installed on the
// machine, and every entry point returns an error wrapping ErrNotAvailable
// when it isn't, so callers can fall back to the browser flow.
package msalruntime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNotAvailable reports that the broker can't be used: msalruntime.dll was
// not found, could not be loaded, failed to start up, or the platform isn't
// Windows. Test for it with errors.Is to fall back to the browser flow.
var ErrNotAvailable = errors.New("msalruntime broker not available")

// Status mirrors MSALRuntime's MSALRUNTIME_RESPONSE_STATUS enum.
type Status int32

const (
	StatusUnexpected Status = iota
	StatusReserved
	StatusInteractionRequired
	StatusNoNetwork
	StatusNetworkTemporarilyUnavailable
	StatusServerTemporarilyUnavailable
	StatusAPIContractViolation
	StatusUserCanceled
	StatusApplicationCanceled
	StatusIncorrectConfiguration
	StatusInsufficientBuffer
	StatusAuthorityUntrusted
	StatusUserSwitch
	StatusAccountUnusable
	StatusUserDataRemovalRequired
	StatusKeyNotFound
	StatusAccountNotFound
	// The statuses below are absent from Microsoft's javamsalruntime 0.13.10
	// contract but present in shipping DLLs (verified against pymsalruntime
	// 0.20.6's Response_Status enum). The DLL on a user's machine is whatever
	// some other app installed, so we must recognise them.
	StatusTransientError
	StatusAccountSwitch
	StatusRequiredBrokerMissing
	StatusDeviceNotRegistered
	StatusFallbackToNativeMsal
)

// Error is a failure reported by MSALRuntime, built from an
// MSALRUNTIME_ERROR_HANDLE. Callers can errors.As for it and branch on Status
// (most usefully StatusInteractionRequired and StatusUserCanceled).
type Error struct {
	Status  Status
	Tag     int32
	Code    int64
	Context string
}

func (e *Error) Error() string {
	var msg string
	switch e.Status {
	case StatusInteractionRequired:
		msg = "user interaction required"
	case StatusNoNetwork, StatusNetworkTemporarilyUnavailable:
		msg = "network unavailable"
	case StatusServerTemporarilyUnavailable:
		msg = "server temporarily unavailable"
	case StatusUserCanceled:
		msg = "user canceled the sign-in"
	case StatusAuthorityUntrusted:
		msg = "authority is not trusted by the broker"
	case StatusAccountNotFound:
		msg = "account not found for this client ID"
	case StatusTransientError:
		msg = "transient broker error"
	case StatusAccountSwitch:
		msg = "the broker signed in a different account"
	case StatusRequiredBrokerMissing:
		msg = "the required broker component is not installed"
	case StatusDeviceNotRegistered:
		msg = "this device is not registered with Entra ID"
	case StatusFallbackToNativeMsal:
		msg = "the broker asked us to fall back to browser sign-in"
	default:
		msg = "broker error"
	}
	return fmt.Sprintf("msalruntime: %s (context: %s, status: %d, tag: %d, code: %d)",
		msg, e.Context, e.Status, e.Tag, e.Code)
}

// Unwrap reports ErrNotAvailable for the statuses that mean this machine can't
// broker at all, so callers already testing errors.Is(err, ErrNotAvailable)
// fall back to the browser flow instead of failing the command. A transient
// error is deliberately excluded: it's worth retrying, not worth abandoning
// the broker over.
func (e *Error) Unwrap() error {
	switch e.Status {
	case StatusRequiredBrokerMissing, StatusDeviceNotRegistered, StatusFallbackToNativeMsal:
		return ErrNotAvailable
	}
	return nil
}

// AuthResult is a token acquired through the broker.
type AuthResult struct {
	AccessToken   string
	ExpiresOn     time.Time
	IDToken       string
	AccountID     string
	HomeAccountID string
}

// Account is a broker account. It owns a native handle, so it must be closed.
type Account struct {
	ID            string
	HomeAccountID string
	ClientInfo    string

	handle uintptr
}

// homeAccountIDFromClientInfo turns MSALRuntime's client info blob (base64url
// JSON with uid/utid) into MSAL's "uid.utid" home account ID. Returns "" if
// the blob isn't parseable.
func homeAccountIDFromClientInfo(clientInfo string) string {
	var ci struct {
		UID  string `json:"uid"`
		UTID string `json:"utid"`
	}
	if err := decodeB64JSON(clientInfo, &ci); err != nil || ci.UID == "" || ci.UTID == "" {
		return ""
	}
	return ci.UID + "." + ci.UTID
}

// accessTokenExpiry reads the exp claim out of a JWT access token. The DLL
// does export MSALRUNTIME_GetExpiresOn, but Microsoft's Java binding - the
// only signature contract we have - doesn't declare it, and it isn't present
// in every version, so the token stays the source of truth. Returns the zero
// time for opaque (non-JWT) tokens, which callers see as already expired.
func accessTokenExpiry(accessToken string) time.Time {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := decodeB64JSON(parts[1], &claims); err != nil || claims.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0).UTC()
}

func decodeB64JSON(s string, v any) error {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

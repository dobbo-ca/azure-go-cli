package msalruntime

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

// Status values verified against pymsalruntime 0.20.6's Response_Status enum.
// Microsoft's javamsalruntime 0.13.10 contract stops at AccountNotFound (16),
// so the tail below is the part a stale contract would silently omit.
func TestStatusValues(t *testing.T) {
	for _, c := range []struct {
		status Status
		want   int32
	}{
		{StatusInteractionRequired, 2},
		{StatusAccountNotFound, 16},
		{StatusTransientError, 17},
		{StatusAccountSwitch, 18},
		{StatusRequiredBrokerMissing, 19},
		{StatusDeviceNotRegistered, 20},
		{StatusFallbackToNativeMsal, 21},
	} {
		if int32(c.status) != c.want {
			t.Errorf("status %d, want %d", c.status, c.want)
		}
	}
}

func TestErrorUnwrapsToNotAvailable(t *testing.T) {
	for _, s := range []Status{StatusRequiredBrokerMissing, StatusDeviceNotRegistered, StatusFallbackToNativeMsal} {
		if !errors.Is(&Error{Status: s}, ErrNotAvailable) {
			t.Errorf("status %d should report the broker unavailable", s)
		}
	}
	for _, s := range []Status{StatusInteractionRequired, StatusTransientError, StatusUserCanceled} {
		if errors.Is(&Error{Status: s}, ErrNotAvailable) {
			t.Errorf("status %d should not abandon the broker", s)
		}
	}
}

func TestHomeAccountIDFromClientInfo(t *testing.T) {
	ci := base64.RawURLEncoding.EncodeToString([]byte(`{"uid":"abc-123","utid":"tenant-9"}`))
	if got := homeAccountIDFromClientInfo(ci); got != "abc-123.tenant-9" {
		t.Fatalf("got %q", got)
	}
	if got := homeAccountIDFromClientInfo("not base64 json"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestAccessTokenExpiry(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":1700000000}`))
	tok := "header." + payload + ".sig"
	if got := accessTokenExpiry(tok); !got.Equal(time.Unix(1700000000, 0)) {
		t.Fatalf("got %v", got)
	}
	if got := accessTokenExpiry("opaque-token"); !got.IsZero() {
		t.Fatalf("got %v, want zero", got)
	}
}

package msalruntime

import (
	"encoding/base64"
	"testing"
	"time"
)

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

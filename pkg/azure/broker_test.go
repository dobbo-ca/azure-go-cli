package azure

import (
	"encoding/base64"
	"testing"
)

func TestUsernameFromIDToken(t *testing.T) {
	tok := func(payload string) string {
		return "h." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".s"
	}
	cases := map[string]string{
		tok(`{"preferred_username":"a@b.com","upn":"x@y.com"}`): "a@b.com",
		tok(`{"upn":"x@y.com"}`):                                "x@y.com",
		tok(`{}`):                                               "",
		"not-a-jwt":                                             "",
	}
	for in, want := range cases {
		if got := usernameFromIDToken(in); got != want {
			t.Errorf("usernameFromIDToken(%q) = %q, want %q", in, got, want)
		}
	}
}

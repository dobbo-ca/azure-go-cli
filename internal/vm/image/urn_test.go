package image

import "testing"

func TestParseURN(t *testing.T) {
	p, o, s, v, err := parseURN("Canonical:ubuntu:22_04-lts:latest")
	if err != nil || p != "Canonical" || o != "ubuntu" || s != "22_04-lts" || v != "latest" {
		t.Fatalf("got %q %q %q %q %v", p, o, s, v, err)
	}
	for _, bad := range []string{"", "a:b:c", "a:b:c:d:e"} {
		if _, _, _, _, err := parseURN(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

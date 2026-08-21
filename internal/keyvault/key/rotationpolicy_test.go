package key

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// flatten renders the parsed policy as a compact comparable string.
func flatten(t *testing.T, value string) string {
	t.Helper()
	policy, err := parseRotationPolicy(value)
	if err != nil {
		t.Fatalf("parseRotationPolicy(%q) failed: %v", value, err)
	}
	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return string(data)
}

func TestParseRotationPolicyKeyForms(t *testing.T) {
	// The runbook policy, in the four spellings azure-cli accepts.
	want := `{"attributes":{"expiryTime":"P90D"},"lifetimeActions":[{"action":{"type":"Rotate"},"trigger":{"timeAfterCreate":"P60D"}}]}`
	cases := map[string]string{
		"camel":                          `{"lifetimeActions":[{"action":{"type":"Rotate"},"trigger":{"timeAfterCreate":"P60D"}}],"attributes":{"expiryTime":"P90D"}}`,
		"snake":                          `{"lifetime_actions":[{"action":{"type":"Rotate"},"trigger":{"time_after_create":"P60D"}}],"attributes":{"expiry_time":"P90D"}}`,
		"flat trigger and string action": `{"lifetime_actions":[{"action":"Rotate","time_after_create":"P60D"}],"expires_in":"P90D"}`,
		"expiresIn inside attributes":    `{"lifetimeActions":[{"action":{"type":"Rotate"},"trigger":{"timeAfterCreate":"P60D"}}],"attributes":{"expires_in":"P90D"}}`,
	}
	for name, input := range cases {
		if got := flatten(t, input); got != want {
			t.Errorf("%s:\n got %s\nwant %s", name, got, want)
		}
	}
}

func TestParseRotationPolicyEmptyAttributesDoesNotShadow(t *testing.T) {
	// An empty attributes object is falsy in Python, so the top level wins.
	got := flatten(t, `{"expires_in":"P90D","attributes":{}}`)
	want := `{"attributes":{"expiryTime":"P90D"},"lifetimeActions":[]}`
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	// A populated attributes object shadows the top level entirely.
	got = flatten(t, `{"expires_in":"P90D","attributes":{"foo":"bar"}}`)
	want = `{"lifetimeActions":[]}`
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestParseRotationPolicyTimeBeforeExpiry(t *testing.T) {
	got := flatten(t, `{"lifetimeActions":[{"action":{"type":"Notify"},"trigger":{"timeBeforeExpiry":"P30D"}}]}`)
	want := `{"lifetimeActions":[{"action":{"type":"Notify"},"trigger":{"timeBeforeExpiry":"P30D"}}]}`
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestParseRotationPolicyReadsAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	body := `{"lifetimeActions":[{"action":{"type":"Rotate"},"trigger":{"timeAfterCreate":"P60D"}}],"attributes":{"expiryTime":"P90D"}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := flatten(t, path), flatten(t, body); got != want {
		t.Errorf("file form gave %s, inline form gave %s", got, want)
	}
}

func TestParseRotationPolicyRejectsGarbage(t *testing.T) {
	if _, err := parseRotationPolicy("not json"); err == nil {
		t.Error("expected an error for a non-JSON value")
	}
}

func TestParseKeyTime(t *testing.T) {
	for _, in := range []string{"2027-01-31T12:59:59Z", "2027-01-31T12:59Z", "2027-01-31T12Z", "2027-01-31"} {
		got, err := parseKeyTime(in)
		if err != nil {
			t.Fatalf("parseKeyTime(%q) failed: %v", in, err)
		}
		if got.Year() != 2027 || got.Month() != 1 || got.Day() != 31 {
			t.Errorf("parseKeyTime(%q) = %v", in, got)
		}
	}
	if _, err := parseKeyTime("31-01-2027"); err == nil {
		t.Error("expected an error for an unsupported layout")
	}
}

func TestParseKeyTags(t *testing.T) {
	tags := parseKeyTags([]string{"env=prod", "bare"})
	if *tags["env"] != "prod" || *tags["bare"] != "" {
		t.Errorf("got %v", tags)
	}
	if parseKeyTags(nil) != nil {
		t.Error("parseKeyTags(nil) should be nil")
	}
}

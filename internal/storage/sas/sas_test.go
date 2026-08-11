package sas

import (
	"strings"
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"2017-12-31T01:11:59Z", "2017-12-31T01:11:59Z"},
		{"2017-12-31T01:11Z", "2017-12-31T01:11:00Z"},
		{"2017-12-31T01Z", "2017-12-31T01:00:00Z"},
		{"2017-12-31", "2017-12-31T00:00:00Z"},
	}
	for _, c := range cases {
		got, err := ParseTime(c.in)
		if err != nil {
			t.Fatalf("ParseTime(%q) returned error: %v", c.in, err)
		}
		if got.Format("2006-01-02T15:04:05Z") != c.want {
			t.Errorf("ParseTime(%q) = %s, want %s", c.in, got.Format("2006-01-02T15:04:05Z"), c.want)
		}
	}
}

func TestParseTimeRejectsGarbage(t *testing.T) {
	if _, err := ParseTime("tomorrow"); err == nil {
		t.Fatal("expected an error for unparseable input")
	}
}

func TestCanonicalReordersAndDedupes(t *testing.T) {
	got, err := Canonical("wr", AccountPerms, "permission")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "rw" {
		t.Errorf("Canonical(\"wr\") = %q, want \"rw\"", got)
	}

	got, err = Canonical("rrw", AccountPerms, "permission")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "rw" {
		t.Errorf("Canonical(\"rrw\") = %q, want \"rw\"", got)
	}
}

func TestCanonicalRejectsUnknownLetter(t *testing.T) {
	// 'y' is valid on a blob but not on a container. See azure-go-cli-gig.
	if _, err := Canonical("ry", ContainerPerms, "permission"); err == nil {
		t.Fatal("expected 'y' to be rejected for a container")
	}
	if _, err := Canonical("ry", BlobPerms, "permission"); err != nil {
		t.Fatalf("expected 'y' to be accepted for a blob, got %v", err)
	}
}

func TestParseIPRange(t *testing.T) {
	r, err := ParseIPRange("168.1.5.60-168.1.5.70")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.String() != "168.1.5.60-168.1.5.70" {
		t.Errorf("got %q", r.String())
	}

	r, err = ParseIPRange("168.1.5.60")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.String() != "168.1.5.60" {
		t.Errorf("got %q", r.String())
	}

	if _, err := ParseIPRange("not-an-ip"); err == nil {
		t.Fatal("expected an error for a non-IP value")
	}
}

func TestQuoteMatchesPythonSafeSet(t *testing.T) {
	// Python: quote(token, safe="&%()$=',~")
	got := Quote("a=b&c/d+e%2Ff", "&%()$=',~")
	if got != "a=b&c%2Fd%2Be%2Ff" {
		t.Errorf("Quote = %q, want %q", got, "a=b&c%2Fd%2Be%2Ff")
	}
}

func TestValidateAsUser(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	within := now.Add(24 * time.Hour)
	beyond := now.Add(8 * 24 * time.Hour)

	if err := ValidateAsUser(false, "login", "", time.Time{}, now); err == nil || !strings.Contains(err.Error(), "specify --as-user") {
		t.Errorf("--auth-mode login without --as-user must fail with the --as-user message, got %v", err)
	}
	if err := ValidateAsUser(true, "login", "", time.Time{}, now); err == nil || !strings.Contains(err.Error(), "specify --expiry") {
		t.Errorf("--as-user without --expiry must fail with the --expiry message, got %v", err)
	}
	if err := ValidateAsUser(true, "key", "2026-01-02", within, now); err == nil || !strings.Contains(err.Error(), "--auth-mode login") {
		t.Errorf("--as-user without --auth-mode login must fail with the --auth-mode message, got %v", err)
	}
	if err := ValidateAsUser(true, "login", "2026-01-09", beyond, now); err == nil || !strings.Contains(err.Error(), "within 7 days") {
		t.Errorf("--as-user beyond 7 days must fail with the 7-day message, got %v", err)
	}
	if err := ValidateAsUser(true, "login", "2026-01-02", within, now); err != nil {
		t.Errorf("valid --as-user usage must pass, got %v", err)
	}
	if err := ValidateAsUser(false, "key", "", time.Time{}, now); err != nil {
		t.Errorf("plain key usage must pass, got %v", err)
	}
}

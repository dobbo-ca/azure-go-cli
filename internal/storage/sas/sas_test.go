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
	if _, err := Canonical("rq", ContainerPerms, "permission"); err == nil {
		t.Fatal("expected 'q' to be rejected for a container")
	}
	// 'y' (permanent delete) is valid on both a blob and a container.
	// See azure-go-cli-gig.
	if _, err := Canonical("ry", ContainerPerms, "permission"); err != nil {
		t.Fatalf("expected 'y' to be accepted for a container, got %v", err)
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

func TestServiceEndpoint(t *testing.T) {
	cases := []struct {
		name, flag, env, account, want string
	}{
		{
			name:    "defaults to the public blob suffix",
			account: "myaccount",
			want:    "https://myaccount.blob.core.windows.net",
		},
		{
			name:    "flag wins over the default",
			flag:    "http://127.0.0.1:10000/devstoreaccount1",
			account: "devstoreaccount1",
			want:    "http://127.0.0.1:10000/devstoreaccount1",
		},
		{
			name:    "trailing slash is trimmed so callers can always append one",
			flag:    "http://127.0.0.1:10000/devstoreaccount1/",
			account: "devstoreaccount1",
			want:    "http://127.0.0.1:10000/devstoreaccount1",
		},
		{
			name:    "env var is used when the flag is absent",
			env:     "https://myaccount.blob.core.chinacloudapi.cn",
			account: "myaccount",
			want:    "https://myaccount.blob.core.chinacloudapi.cn",
		},
		{
			name:    "flag beats env var",
			flag:    "http://127.0.0.1:10000/devstoreaccount1",
			env:     "https://ignored.example",
			account: "devstoreaccount1",
			want:    "http://127.0.0.1:10000/devstoreaccount1",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("AZURE_STORAGE_SERVICE_ENDPOINT", c.env)
			if got := ServiceEndpoint(c.flag, c.account); got != c.want {
				t.Errorf("ServiceEndpoint(%q, %q) = %q, want %q", c.flag, c.account, got, c.want)
			}
		})
	}
}

func TestAccountFromEndpoint(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"public endpoint", "https://myaccount.blob.core.windows.net", "myaccount"},
		{"public endpoint, trailing slash", "https://myaccount.blob.core.windows.net/", "myaccount"},
		{"IP-addressed emulator", "http://127.0.0.1:10000/devstoreaccount1", "devstoreaccount1"},
		{"IP-addressed emulator, trailing slash", "http://127.0.0.1:10000/devstoreaccount1/", "devstoreaccount1"},
		{"empty endpoint", "", ""},
		{"path carries container and blob too, account is still the first segment", "http://127.0.0.1:10000/devstoreaccount1/mycontainer/my.blob", "devstoreaccount1"},
		{"hostname emulator with account in the path", "https://azurite:10000/devstoreaccount1", "devstoreaccount1"},
		// No path segment to fall back to, so the hostname itself is taken as
		// the account name - wrong, but that is the pre-existing behaviour
		// this bead pins rather than changes. See azure-go-cli-h8z.
		{"hostname emulator with no path", "https://azurite:10000", "azurite"},
		{"IP-literal emulator with account in the path", "https://10.0.0.4/devstoreaccount1", "devstoreaccount1"},
		{"IPv6-literal emulator with account in the path", "http://[::1]:10000/devstoreaccount1", "devstoreaccount1"},
		{"localhost emulator with account in the path", "http://localhost:10000/devstoreaccount1", "devstoreaccount1"},
		{"sovereign cloud suffix, China", "https://myaccount.blob.core.chinacloudapi.cn", "myaccount"},
		{"sovereign cloud suffix, US Gov", "https://myaccount.blob.core.usgovcloudapi.net", "myaccount"},
		{"sovereign cloud suffix, Germany", "https://myaccount.blob.core.cloudapi.de", "myaccount"},
		// KNOWN LIMITATION: a custom domain has no ".blob.core." label and no
		// path segment, so the first dot-label of the host is taken as the
		// account name - here "blob", not the real account. Python
		// azure-storage-blob's BlobServiceClient.account_name returns None in
		// this case instead of guessing (_shared/base_client.py:139-148,
		// verified against the installed package). Pinned deliberately: see
		// azure-go-cli-h8z.
		{"custom domain, KNOWN LIMITATION", "https://blob.contoso.com", "blob"},
		// KNOWN LIMITATION: a bare dotless host with no path is returned
		// verbatim as the account name. Python returns None here too
		// (base_client.py:139-148: split on ".blob.core." fails, and the
		// path-derivation fallback only fires for localhost/127.0.0.1).
		// Pinned deliberately: see azure-go-cli-h8z.
		{"dotless host, KNOWN LIMITATION", "https://myaccount", "myaccount"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AccountFromEndpoint(c.in); got != c.want {
				t.Errorf("AccountFromEndpoint(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

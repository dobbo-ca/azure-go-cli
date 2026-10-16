package account

import "testing"

func TestCheckTenantFlags(t *testing.T) {
	if checkTenantFlags("sub", "ten") == nil {
		t.Fatal("expected conflict error")
	}
	if checkTenantFlags("sub", "") != nil || checkTenantFlags("", "ten") != nil {
		t.Fatal("unexpected error")
	}
}

func TestResolveResource(t *testing.T) {
	if r, err := resolveResource("", "oss-rdbms"); err != nil || r != "https://ossrdbms-aad.database.windows.net" {
		t.Fatalf("oss-rdbms: got %q, %v", r, err)
	}
	if r, err := resolveResource("https://vault.azure.net", ""); err != nil || r != "https://vault.azure.net" {
		t.Fatalf("passthrough: got %q, %v", r, err)
	}
	if _, err := resolveResource("https://vault.azure.net", "arm"); err == nil {
		t.Fatal("expected conflict error")
	}
	if _, err := resolveResource("", "bogus"); err == nil {
		t.Fatal("expected invalid type error")
	}
}

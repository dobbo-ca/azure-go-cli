package sas

import "testing"

func TestParseConnectionStringKeepsBase64Padding(t *testing.T) {
	cs := "DefaultEndpointsProtocol=https;AccountName=acct;AccountKey=YWJjZA==;EndpointSuffix=core.windows.net"
	got := ParseConnectionString(cs)
	if got["AccountName"] != "acct" {
		t.Errorf("AccountName = %q", got["AccountName"])
	}
	// The key contains '=' padding. Splitting on every '=' would truncate it.
	if got["AccountKey"] != "YWJjZA==" {
		t.Errorf("AccountKey = %q, want \"YWJjZA==\"", got["AccountKey"])
	}
}

func TestResolveInputsPrefersConnectionString(t *testing.T) {
	got := ResolveInputs("flagname", "flagkey",
		"AccountName=csname;AccountKey=cskey")
	if got.AccountName != "csname" || got.AccountKey != "cskey" {
		t.Errorf("connection string should win, got %+v", got)
	}
}

func TestResolveInputsFallsBackToEnv(t *testing.T) {
	t.Setenv("AZURE_STORAGE_ACCOUNT", "envname")
	t.Setenv("AZURE_STORAGE_KEY", "envkey")
	got := ResolveInputs("", "", "")
	if got.AccountName != "envname" || got.AccountKey != "envkey" {
		t.Errorf("expected env fallback, got %+v", got)
	}
}

func TestResolveInputsFlagsBeatEnv(t *testing.T) {
	t.Setenv("AZURE_STORAGE_ACCOUNT", "envname")
	t.Setenv("AZURE_STORAGE_KEY", "envkey")
	got := ResolveInputs("flagname", "flagkey", "")
	if got.AccountName != "flagname" || got.AccountKey != "flagkey" {
		t.Errorf("expected flags to win, got %+v", got)
	}
}

func TestResolveInputsReadsConnectionStringFromEnv(t *testing.T) {
	t.Setenv("AZURE_STORAGE_CONNECTION_STRING", "AccountName=csname;AccountKey=cskey")
	got := ResolveInputs("", "", "")
	if got.AccountName != "csname" || got.AccountKey != "cskey" {
		t.Errorf("expected env connection string, got %+v", got)
	}
}

func TestResolveInputsExplicitAccountKeySuppressesEnvConnectionString(t *testing.T) {
	t.Setenv("AZURE_STORAGE_CONNECTION_STRING", "AccountName=envacct;AccountKey=envkey")
	got := ResolveInputs("flagname", "flagkey", "")
	if got.AccountName != "flagname" || got.AccountKey != "flagkey" {
		t.Errorf("explicit --account-key should suppress env connection string, got %+v", got)
	}
}

package sas

import (
	"net/url"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	azsas "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

// A syntactically valid but fake key. base64 of 32 bytes.
const testKey = "bXlzdXBlcnNlY3JldHRlc3RrZXkxMjM0NTY3ODkwYWI="

func TestSignAccountMatchesSDKForBlobOnly(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expiry := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	cred, err := azblob.NewSharedKeyCredential("myaccount", testKey)
	if err != nil {
		t.Fatalf("NewSharedKeyCredential: %v", err)
	}
	sdkParams, err := azsas.AccountSignatureValues{
		Protocol:      azsas.ProtocolHTTPS,
		StartTime:     start,
		ExpiryTime:    expiry,
		Permissions:   "rl",
		ResourceTypes: "sco",
	}.SignWithSharedKey(cred)
	if err != nil {
		t.Fatalf("SDK SignWithSharedKey: %v", err)
	}

	ours, err := SignAccount(AccountOptions{
		AccountName:   "myaccount",
		Permissions:   "rl",
		Services:      "b",
		ResourceTypes: "sco",
		Start:         start,
		Expiry:        expiry,
		Protocol:      "https",
	}, testKey)
	if err != nil {
		t.Fatalf("SignAccount: %v", err)
	}

	sdkSig := mustQuery(t, sdkParams.Encode()).Get("sig")
	ourSig := mustQuery(t, ours).Get("sig")
	if sdkSig != ourSig {
		t.Errorf("signature mismatch\n SDK: %s\nours: %s", sdkSig, ourSig)
	}
}

func TestSignAccountCarriesAllServices(t *testing.T) {
	ours, err := SignAccount(AccountOptions{
		AccountName:   "myaccount",
		Permissions:   "rl",
		Services:      "tqbf", // deliberately out of order
		ResourceTypes: "oc",   // deliberately out of order
		Expiry:        time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}, testKey)
	if err != nil {
		t.Fatalf("SignAccount: %v", err)
	}
	q := mustQuery(t, ours)
	if q.Get("ss") != "bqtf" {
		t.Errorf("ss = %q, want \"bqtf\" (this is the regression this signer exists to prevent)", q.Get("ss"))
	}
	if q.Get("srt") != "co" {
		t.Errorf("srt = %q, want \"co\"", q.Get("srt"))
	}
	if q.Get("sig") == "" {
		t.Error("sig is empty")
	}
}

func TestSignAccountRejectsBadServiceLetter(t *testing.T) {
	_, err := SignAccount(AccountOptions{
		AccountName:   "myaccount",
		Permissions:   "r",
		Services:      "bz",
		ResourceTypes: "o",
		Expiry:        time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}, testKey)
	if err == nil {
		t.Fatal("expected 'z' to be rejected as a service")
	}
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	v, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", raw, err)
	}
	return v
}

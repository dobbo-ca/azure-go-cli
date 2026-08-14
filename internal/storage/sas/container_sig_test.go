package sas

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	azsas "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

const pinnedSASVersion = "2026-04-06"

// TestSignContainerSharedKeyMatchesSDKWithoutY is the regression guard: it
// proves SignContainerSharedKey is byte-identical to the SDK for every input
// that does not contain y. Verified in scratchpad/probe_gig: 7/7 identical.
func TestSignContainerSharedKeyMatchesSDKWithoutY(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expiry := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		perms string
		o     BlobScopeOptions
	}{
		{name: "rl", perms: "rl"},
		{name: "r", perms: "r"},
		{name: "racwdxltfmei", perms: "racwdxltfmei"},
		{name: "racwdxltfmeopi", perms: "racwdxltfmeopi"},
		{name: "rwdl", perms: "rwdl"},
		{
			name:  "full-option",
			perms: "racwdxltfmeopi",
			o: BlobScopeOptions{
				Identifier:         "mypolicy",
				Protocol:           "https",
				IPRange:            "168.1.5.60-168.1.5.70",
				EncryptionScope:    "myscope",
				CacheControl:       "no-cache",
				ContentDisposition: `inline; filename="a b.txt"`,
				ContentEncoding:    "gzip",
				ContentLanguage:    "en-CA",
				ContentType:        "text/plain",
			},
		},
	}

	cred, err := azblob.NewSharedKeyCredential("myaccount", testKey)
	if err != nil {
		t.Fatalf("NewSharedKeyCredential: %v", err)
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := c.o
			o.AccountName = "myaccount"
			o.ContainerName = "mycontainer"
			o.Start = start
			o.Expiry = expiry

			ipRange, err := ParseIPRange(o.IPRange)
			if err != nil {
				t.Fatalf("ParseIPRange: %v", err)
			}

			ours, err := SignContainerSharedKey(o, c.perms, ipRange, testKey)
			if err != nil {
				t.Fatalf("SignContainerSharedKey: %v", err)
			}

			sdkParams, err := azsas.BlobSignatureValues{
				Protocol:           azsas.Protocol(o.Protocol),
				StartTime:          o.Start,
				ExpiryTime:         o.Expiry,
				Permissions:        c.perms,
				IPRange:            ipRange,
				Identifier:         o.Identifier,
				ContainerName:      o.ContainerName,
				CacheControl:       o.CacheControl,
				ContentDisposition: o.ContentDisposition,
				ContentEncoding:    o.ContentEncoding,
				ContentLanguage:    o.ContentLanguage,
				ContentType:        o.ContentType,
				EncryptionScope:    o.EncryptionScope,
			}.SignWithSharedKey(cred)
			if err != nil {
				t.Fatalf("SDK SignWithSharedKey: %v", err)
			}
			want := sdkParams.Encode()

			if ours != want {
				t.Errorf("token mismatch\nours: %s\n SDK: %s", ours, want)
			}
		})
	}
}

// TestSignBlobScopeContainerPermanentDelete pins an expected signature that
// was generated, not guessed: Python azure-storage-blob 12.30.0,
// BlobSharedAccessSignature.generate_container with X_MS_VERSION pinned to
// 2026-04-06, same key/account/container/times, matches the Go signer byte
// for byte (see EVIDENCE-gig.txt section 5). As a control, the same Python
// run at perms racwdxltfmei (no y) yields
// ZuOZtYA8Gy3RVbBdUjAYTeKiC3xOoTK9hfY6KjjZbag= which is exactly what
// ./bin/az/az emits today.
func TestSignBlobScopeContainerPermanentDelete(t *testing.T) {
	if azsas.Version != pinnedSASVersion {
		t.Fatalf("azblob SAS service version moved %s -> %s; regenerate "+
			"the expected signature (see EVIDENCE-gig.txt section 5)",
			pinnedSASVersion, azsas.Version)
	}

	token, err := SignBlobScope(context.Background(), BlobScopeOptions{
		AccountName:   "myaccount",
		ContainerName: "mycontainer",
		Permissions:   "racwdxyltfmei",
		Start:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Expiry:        time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}, testKey, false)
	if err != nil {
		t.Fatalf("SignBlobScope: %v", err)
	}

	q := mustQuery(t, token)
	if q.Get("sp") != "racwdxyltfmei" {
		t.Errorf("sp = %q, want %q", q.Get("sp"), "racwdxyltfmei")
	}
	if q.Get("sr") != "c" {
		t.Errorf("sr = %q, want \"c\"", q.Get("sr"))
	}
	if q.Get("sv") != "2026-04-06" {
		t.Errorf("sv = %q, want \"2026-04-06\"", q.Get("sv"))
	}
	if q.Get("sig") != "1DNJv0txj/LHnBnlFRBcysUtqmY7zTJDsJcmAFVTA+Y=" {
		t.Errorf("sig = %q, want %q", q.Get("sig"), "1DNJv0txj/LHnBnlFRBcysUtqmY7zTJDsJcmAFVTA+Y=")
	}

	want := "se=2026-01-02T00%3A00%3A00Z&sig=1DNJv0txj%2FLHnBnlFRBcysUtqmY7zTJDsJcmAFVTA%2BY%3D&sp=racwdxyltfmei&sr=c&st=2026-01-01T00%3A00%3A00Z&sv=2026-04-06"
	if token != want {
		t.Errorf("token mismatch\ngot:  %s\nwant: %s", token, want)
	}
}

// TestSignBlobScopeContainerReordersPermanentDelete checks canonical
// reordering with y included. Python from_string('iemftlyxdwcar') ->
// 'racwdxyltfmei' (EVIDENCE-gig.txt section 2).
func TestSignBlobScopeContainerReordersPermanentDelete(t *testing.T) {
	token, err := SignBlobScope(context.Background(), BlobScopeOptions{
		AccountName:   "myaccount",
		ContainerName: "mycontainer",
		Permissions:   "iemftlyxdwcar",
		Expiry:        time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}, testKey, false)
	if err != nil {
		t.Fatalf("SignBlobScope: %v", err)
	}
	q := mustQuery(t, token)
	if q.Get("sp") != "racwdxyltfmei" {
		t.Errorf("sp = %q, want %q", q.Get("sp"), "racwdxyltfmei")
	}
}

// TestSignBlobScopeRejectsPermanentDeleteAsUser checks the authored
// --as-user error, and that the blob-scope y path (which already works
// today) is untouched by the container-only dispatch.
func TestSignBlobScopeRejectsPermanentDeleteAsUser(t *testing.T) {
	_, err := SignBlobScope(context.Background(), BlobScopeOptions{
		AccountName:   "myaccount",
		ContainerName: "mycontainer",
		Permissions:   "racwdxyltfmei",
		Expiry:        time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}, "", true)
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "--as-user") || !strings.Contains(msg, "'y'") || !strings.Contains(msg, "permanent delete") {
		t.Errorf("error message missing expected substrings: %q", msg)
	}
	if strings.Contains(msg, "121") {
		t.Errorf("error message leaks the raw SDK error code 121: %q", msg)
	}
}

// The container-only dispatch must not affect the blob-scope y path, which
// already works today via the SDK's parseBlobPermissions.
func TestSignBlobScopeBlobPermanentDeleteUnaffected(t *testing.T) {
	token, err := SignBlobScope(context.Background(), BlobScopeOptions{
		AccountName:   "myaccount",
		ContainerName: "mycontainer",
		BlobName:      "b.txt",
		Permissions:   "racwdxyltmeopi",
		Expiry:        time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}, testKey, false)
	if err != nil {
		t.Fatalf("SignBlobScope: %v", err)
	}
	q := mustQuery(t, token)
	if !strings.ContainsRune(q.Get("sp"), 'y') {
		t.Errorf("sp = %q, want it to contain 'y'", q.Get("sp"))
	}
}

// TestSignContainerSharedKeyPrecondition pins the guard that mirrors
// BlobSignatureValues.SignWithSharedKey's opening precondition (azblob v1.7.0
// sas/service.go:97-99). It is checked DIFFERENTIALLY rather than against a
// hand-written string: for the same missing fields, our signer must refuse
// exactly when the SDK refuses, and with the same message. Without this, a
// container SAS carrying y could be minted with no expiry while the identical
// request without y was rejected -- an inconsistency the service would then
// surface as an opaque failure.
func TestSignContainerSharedKeyPrecondition(t *testing.T) {
	cred, err := azblob.NewSharedKeyCredential("myaccount", testKey)
	if err != nil {
		t.Fatalf("NewSharedKeyCredential: %v", err)
	}
	ipRange, err := ParseIPRange("")
	if err != nil {
		t.Fatalf("ParseIPRange: %v", err)
	}

	base := BlobScopeOptions{AccountName: "myaccount", ContainerName: "mycontainer"}

	// The SDK's own refusal for the same shape, minus the y it cannot parse.
	_, sdkErr := azsas.BlobSignatureValues{
		Permissions:   "r",
		ContainerName: base.ContainerName,
	}.SignWithSharedKey(cred)
	if sdkErr == nil {
		t.Fatal("SDK accepted a service SAS with no expiry and no identifier; " +
			"the precondition this test pins has moved, so re-derive the guard")
	}

	t.Run("no-expiry-no-identifier-is-refused", func(t *testing.T) {
		_, err := SignContainerSharedKey(base, "ry", ipRange, testKey)
		if err == nil {
			t.Fatal("signed a container SAS with no expiry and no identifier; " +
				"the SDK refuses the same input without y")
		}
		if err.Error() != sdkErr.Error() {
			t.Errorf("error text diverges from the SDK\nours: %s\n SDK: %s", err, sdkErr)
		}
	})

	t.Run("identifier-without-expiry-is-allowed", func(t *testing.T) {
		o := base
		o.Identifier = "mypolicy"
		if _, err := SignContainerSharedKey(o, "ry", ipRange, testKey); err != nil {
			t.Errorf("refused a stored-access-policy SAS the SDK allows: %v", err)
		}
	})

	t.Run("expiry-without-identifier-is-allowed", func(t *testing.T) {
		o := base
		o.Expiry = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
		if _, err := SignContainerSharedKey(o, "ry", ipRange, testKey); err != nil {
			t.Errorf("refused a SAS carrying an expiry: %v", err)
		}
	})
}

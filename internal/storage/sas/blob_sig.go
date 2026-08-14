package sas

import (
	"context"
	"fmt"
	"strings"
	"time"

	azsas "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

// BlobScopeOptions are the inputs shared by container-scope and blob-scope SAS.
// Leave BlobName empty for a container SAS.
type BlobScopeOptions struct {
	AccountName string
	// ServiceEndpoint is the blob service base URL, without a trailing slash.
	// Only the --as-user path reads it, to reach GetUserDelegationCredential;
	// shared-key signing is entirely local. Empty means the public cloud.
	ServiceEndpoint       string
	ContainerName         string
	BlobName              string
	Permissions           string
	Identifier            string
	IPRange               string
	Protocol              string
	EncryptionScope       string
	CacheControl          string
	ContentDisposition    string
	ContentEncoding       string
	ContentLanguage       string
	ContentType           string
	Snapshot              string
	DelegatedUserObjectID string
	DelegatedUserTenantID string
	Start                 time.Time
	Expiry                time.Time
}

// UserDelegationExpiryLimit is the service cap on a user delegation key.
const UserDelegationExpiryLimit = 7 * 24 * time.Hour

// SnapshotTimeFormat mirrors the unexported exported.SnapshotTimeFormat in the
// SDK, which is not importable from outside the module.
const SnapshotTimeFormat = "2006-01-02T15:04:05.0000000Z07:00"

// ValidateAsUser applies the --as-user rules from _validators.py:1541.
func ValidateAsUser(asUser bool, authMode, expiry string, expiryTime time.Time, now time.Time) error {
	if authMode == "login" && !asUser {
		return fmt.Errorf("incorrect usage: specify --as-user when --auth-mode login is used to get user delegation key")
	}
	if !asUser {
		return nil
	}
	if expiry == "" {
		return fmt.Errorf("incorrect usage: specify --expiry when --as-user is enabled")
	}
	if expiryTime.After(now.Add(UserDelegationExpiryLimit)) {
		return fmt.Errorf("incorrect usage: --expiry should be within 7 days from now")
	}
	if authMode != "login" {
		return fmt.Errorf("incorrect usage: specify '--auth-mode login' when --as-user is enabled")
	}
	return nil
}

// UserDelegationKeyInfo builds the KeyInfo request body for
// GetUserDelegationCredential. tid is only set when non-empty, so omitting
// --user-delegation-tid leaves the DelegatedUserTid element out of the
// request entirely rather than sending it empty.
func UserDelegationKeyInfo(start, expiry time.Time, tid string) service.KeyInfo {
	startStr := start.UTC().Format(azsas.TimeFormat)
	expiryStr := expiry.UTC().Format(azsas.TimeFormat)
	info := service.KeyInfo{
		Start:  &startStr,
		Expiry: &expiryStr,
	}
	if tid != "" {
		info.DelegatedUserTenantID = &tid
	}
	return info
}

// SignBlobScope signs a container-scope or blob-scope SAS. When asUser is true
// it fetches a user delegation key over AAD; otherwise it signs with accountKey.
func SignBlobScope(ctx context.Context, o BlobScopeOptions, accountKey string, asUser bool) (string, error) {
	allowed := ContainerPerms
	label := "container permission"
	if o.BlobName != "" {
		allowed = BlobPerms
		label = "blob permission"
	}
	perms, err := Canonical(o.Permissions, allowed, label)
	if err != nil {
		return "", err
	}
	ipRange, err := ParseIPRange(o.IPRange)
	if err != nil {
		return "", err
	}

	// azblob cannot sign y (permanent delete) for a container: see
	// SignContainerSharedKey. For shared key we sign it ourselves; for
	// --as-user the signature is produced by the SDK's
	// SignWithUserDelegation, which routes container permissions through the
	// same parseContainerPermissions, so y is genuinely unavailable there and
	// we say so instead of leaking `invalid permission: '121'`.
	if o.BlobName == "" && o.Snapshot == "" && strings.ContainsRune(perms, 'y') {
		if asUser {
			return "", fmt.Errorf("--as-user does not support container permission 'y' (permanent delete): a user delegation SAS is signed by the Azure SDK, which does not model permanent delete for container scope. Use shared key auth (--auth-mode key) or drop 'y' from --permissions")
		}
		return SignContainerSharedKey(o, perms, ipRange, accountKey)
	}

	// A snapshot SAS signs resource "bs" rather than "b". The SDK switches on
	// SnapshotTime being non-zero, so the opaque --snapshot value is parsed here.
	var snapshotTime time.Time
	if o.Snapshot != "" {
		snapshotTime, err = time.Parse(SnapshotTimeFormat, o.Snapshot)
		if err != nil {
			return "", fmt.Errorf("invalid --snapshot value %q: %w", o.Snapshot, err)
		}
	}

	values := azsas.BlobSignatureValues{
		SnapshotTime:                snapshotTime,
		Protocol:                    azsas.Protocol(o.Protocol),
		StartTime:                   o.Start,
		ExpiryTime:                  o.Expiry,
		Permissions:                 perms,
		IPRange:                     ipRange,
		Identifier:                  o.Identifier,
		ContainerName:               o.ContainerName,
		BlobName:                    o.BlobName,
		CacheControl:                o.CacheControl,
		ContentDisposition:          o.ContentDisposition,
		ContentEncoding:             o.ContentEncoding,
		ContentLanguage:             o.ContentLanguage,
		ContentType:                 o.ContentType,
		SignedDelegatedUserObjectID: o.DelegatedUserObjectID,
		EncryptionScope:             o.EncryptionScope,
	}

	serviceURL := ServiceEndpoint(o.ServiceEndpoint, o.AccountName) + "/"

	// KNOWN LIMITATION, --as-user against a non-public endpoint: azblob
	// v1.7.0's service/client.go:117 derives the SAS account name for
	// GetUserDelegationCredential from strings.Split(url.Host, ".")[0] of
	// this client's URL, ignoring o.AccountName entirely. That is correct
	// only when the account is the first label of a public hostname
	// (myaccount.blob.core.windows.net); for an IP-addressed or dotless host
	// (an emulator, or a private-endpoint IP) it signs against the wrong
	// name - e.g. "127" for 127.0.0.1 or "azurite:10000" for a bare
	// hostname:port - and the service rejects the resulting SAS. There is no
	// fallback to the correctly-parsed IPEndpointStyleInfo.AccountName, and
	// for a dotless host that field is empty anyway: sas.ParseURL only fills
	// it when the host is a real IP literal (url_parts.go:58-60, gated by
	// shared.IsIPEndpointStyle -> net.ParseIP, shared.go:230-244). This is
	// not fixable by building our own credential here: sas.UserDelegationCredential
	// is a type alias of the SDK-internal exported.UserDelegationCredential
	// (sas/account.go:20), so a vendored copy would be a distinct type that
	// SignWithUserDelegation cannot accept - the whole signing path would
	// have to be vendored too. Fixed upstream would need azblob to consult
	// service.ClientOptions or the parsed IPEndpointStyleInfo instead of
	// re-deriving the name from the host. The derivation is character-
	// identical in v1.6.3:120, v1.7.0:117 and v1.8.0:127, so bumping does
	// not help. A workaround that builds the client at a canonical hostname
	// and rewrites the request in a RoundTripper was considered and
	// REJECTED: it would decouple the host azcore authenticates from the
	// host that actually receives the token. Reported upstream instead.
	// See azure-go-cli-h8z. The shared-key path below is unaffected: it reads
	// sharedKeyCredential.AccountName() (sas/service.go:145), which is our
	// own o.AccountName, not derived from the URL.
	if asUser {
		cred, err := azure.GetCredential()
		if err != nil {
			return "", err
		}
		client, err := service.NewClient(serviceURL, cred, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create blob service client: %w", err)
		}
		start := o.Start
		if start.IsZero() {
			start = time.Now().UTC()
		}
		udc, err := client.GetUserDelegationCredential(ctx, UserDelegationKeyInfo(start, o.Expiry, o.DelegatedUserTenantID), nil)
		if err != nil {
			return "", fmt.Errorf("failed to get a user delegation key: %w", err)
		}
		params, err := values.SignWithUserDelegation(udc)
		if err != nil {
			return "", err
		}
		return params.Encode(), nil
	}

	sharedKey, err := NewSharedKey(o.AccountName, accountKey)
	if err != nil {
		return "", err
	}
	params, err := values.SignWithSharedKey(sharedKey)
	if err != nil {
		return "", err
	}
	return params.Encode(), nil
}

package sas

import (
	"context"
	"fmt"
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
	ServiceEndpoint    string
	ContainerName      string
	BlobName           string
	Permissions        string
	Identifier         string
	IPRange            string
	Protocol           string
	EncryptionScope    string
	CacheControl       string
	ContentDisposition string
	ContentEncoding    string
	ContentLanguage    string
	ContentType        string
	Snapshot           string
	AuthorizedObjectID string
	Start              time.Time
	Expiry             time.Time
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
		SnapshotTime:       snapshotTime,
		Protocol:           azsas.Protocol(o.Protocol),
		StartTime:          o.Start,
		ExpiryTime:         o.Expiry,
		Permissions:        perms,
		IPRange:            ipRange,
		Identifier:         o.Identifier,
		ContainerName:      o.ContainerName,
		BlobName:           o.BlobName,
		CacheControl:       o.CacheControl,
		ContentDisposition: o.ContentDisposition,
		ContentEncoding:    o.ContentEncoding,
		ContentLanguage:    o.ContentLanguage,
		ContentType:        o.ContentType,
		AuthorizedObjectID: o.AuthorizedObjectID,
		EncryptionScope:    o.EncryptionScope,
	}

	serviceURL := ServiceEndpoint(o.ServiceEndpoint, o.AccountName) + "/"

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
		startStr := start.UTC().Format(azsas.TimeFormat)
		expiryStr := o.Expiry.UTC().Format(azsas.TimeFormat)
		udc, err := client.GetUserDelegationCredential(ctx, service.KeyInfo{
			Start:  &startStr,
			Expiry: &expiryStr,
		}, nil)
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

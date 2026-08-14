package sas

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"

	azsas "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

// SignContainerSharedKey signs a container-scope (sr=c) service SAS with the
// account key.
//
// It exists only because of the permanent-delete letter y. azblob v1.7.0's
// sas.ContainerPermissions has no PermanentDelete field
// (sas/service.go:345-348) and parseContainerPermissions
// (sas/service.go:400-437) rejects y outright - verified by calling
// BlobSignatureValues{ContainerName: "..."}.SignWithSharedKey with
// Permissions "ry", which returns `invalid permission: '121'`. Python
// azure-storage-blob emits y for a container (ContainerSasPermissions._str:
// r a c w d x y l t f m e i) and azure-cli validates --permissions with that
// same class (_params.py:1731-1733), so azure-cli accepts what the Go SDK
// will not.
//
// perms MUST already be canonically ordered by Canonical(..., ContainerPerms,
// ...). This function does not reorder.
//
// The field order is the service SAS string-to-sign for sv 2020-12-06 and
// later, taken from the REST reference "Create a service SAS" and
// cross-checked against Python's _BlobSharedAccessHelper.add_resource_signature
// and against azblob v1.7.0 SignWithSharedKey; all three agree.
// signedSnapshotTime is always empty here because a container SAS has no
// snapshot.
func SignContainerSharedKey(o BlobScopeOptions, perms string, ipRange azsas.IPRange, accountKey string) (string, error) {
	start := ""
	if !o.Start.IsZero() {
		start = o.Start.UTC().Format(azsas.TimeFormat)
	}
	expiry := ""
	if !o.Expiry.IsZero() {
		expiry = o.Expiry.UTC().Format(azsas.TimeFormat)
	}

	// Mirrors BlobSignatureValues.SignWithSharedKey's opening precondition
	// (azblob v1.7.0 sas/service.go:97-99): a service SAS with no stored
	// access policy must carry both an expiry and permissions, or the
	// service can never honour it.
	if o.Identifier == "" && (o.Expiry.IsZero() || perms == "") {
		return "", errors.New("service SAS is missing at least one of these: ExpiryTime or Permissions")
	}

	stringToSign := strings.Join([]string{
		perms,
		start,
		expiry,
		"/blob/" + o.AccountName + "/" + o.ContainerName,
		o.Identifier,
		ipRange.String(),
		o.Protocol,
		azsas.Version,
		"c", // signedResource
		"",  // signedSnapshotTime, never set for a container SAS
		o.EncryptionScope,
		o.CacheControl,
		o.ContentDisposition,
		o.ContentEncoding,
		o.ContentLanguage,
		o.ContentType,
	}, "\n")

	key, err := base64.StdEncoding.DecodeString(accountKey)
	if err != nil {
		return "", fmt.Errorf("the account key is not valid base64: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Emitted through url.Values so the result is byte-identical to the SDK's
	// QueryParameters.Encode (azblob v1.7.0 sas/query_params.go:316-404),
	// which is also url.Values.Encode: keys sorted, values url.QueryEscape'd,
	// empty values omitted.
	v := url.Values{}
	add := func(k, val string) {
		if val != "" {
			v.Add(k, val)
		}
	}
	add("sv", azsas.Version)
	add("spr", o.Protocol)
	add("st", start)
	add("se", expiry)
	add("sip", ipRange.String())
	add("si", o.Identifier)
	add("sr", "c")
	add("sp", perms)
	add("sig", signature)
	add("rscc", o.CacheControl)
	add("rscd", o.ContentDisposition)
	add("rsce", o.ContentEncoding)
	add("rscl", o.ContentLanguage)
	add("rsct", o.ContentType)
	add("ses", o.EncryptionScope)
	return v.Encode(), nil
}

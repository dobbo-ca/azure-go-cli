package sas

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	azsas "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

// Service and resource-type letters, in the order the service expects.
const (
	AccountServices      = "bqtf"
	AccountResourceTypes = "sco"
)

// AccountOptions are the inputs to an account-scope SAS.
type AccountOptions struct {
	AccountName     string
	Permissions     string
	Services        string
	ResourceTypes   string
	IPRange         string
	Protocol        string
	EncryptionScope string
	Start           time.Time
	Expiry          time.Time
}

// SignAccount builds an account SAS token and returns it as a query string.
//
// This does not use sas.AccountSignatureValues because that type has no
// Services field and hardcodes "b" into the signature, which would silently
// reduce --services bqtf to blob-only.
func SignAccount(o AccountOptions, accountKey string) (string, error) {
	if o.Expiry.IsZero() || o.Permissions == "" || o.Services == "" || o.ResourceTypes == "" {
		return "", fmt.Errorf("account SAS is missing at least one of these: --expiry, --permissions, --services, or --resource-types")
	}

	perms, err := Canonical(o.Permissions, AccountPerms, "permission")
	if err != nil {
		return "", err
	}
	services, err := Canonical(o.Services, AccountServices, "service")
	if err != nil {
		return "", err
	}
	resourceTypes, err := Canonical(o.ResourceTypes, AccountResourceTypes, "resource type")
	if err != nil {
		return "", err
	}
	ipRange, err := ParseIPRange(o.IPRange)
	if err != nil {
		return "", err
	}

	start := ""
	if !o.Start.IsZero() {
		start = o.Start.UTC().Format(azsas.TimeFormat)
	}
	expiry := ""
	if !o.Expiry.IsZero() {
		expiry = o.Expiry.UTC().Format(azsas.TimeFormat)
	}

	stringToSign := strings.Join([]string{
		o.AccountName,
		perms,
		services,
		resourceTypes,
		start,
		expiry,
		ipRange.String(),
		o.Protocol,
		azsas.Version,
		o.EncryptionScope,
		"", // the account SAS requires a terminating newline
	}, "\n")

	key, err := base64.StdEncoding.DecodeString(accountKey)
	if err != nil {
		return "", fmt.Errorf("the account key is not valid base64: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Emitted in azure-cli's field order. Order is cosmetic; the service
	// recomputes the signature from the values it receives.
	pairs := []struct{ k, v string }{
		{"sv", azsas.Version},
		{"ss", services},
		{"srt", resourceTypes},
		{"sp", perms},
		{"se", expiry},
		{"st", start},
		{"sip", ipRange.String()},
		{"spr", o.Protocol},
		{"ses", o.EncryptionScope},
		{"sig", signature},
	}
	var b strings.Builder
	for _, p := range pairs {
		if p.v == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('&')
		}
		b.WriteString(p.k)
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(p.v))
	}
	return b.String(), nil
}

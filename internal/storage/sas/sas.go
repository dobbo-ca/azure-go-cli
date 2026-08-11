// Package sas builds Azure Storage shared access signatures.
//
// It exists because all three generate-sas commands need the same expiry
// parsing, permission validation and credential resolution, and because the
// account-scope signature cannot go through the SDK (see account_sig.go).
package sas

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	azsas "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

// NOTE: this package is itself named sas, so the SDK's sas package MUST be
// imported under the azsas alias in every file here. Importing it unaliased
// compiles but shadows the package's own name and reads as a bug.

// Permission letters in the order the service expects them. These mirror the
// String() methods on sas.AccountPermissions, sas.ContainerPermissions and
// sas.BlobPermissions, so validation here cannot drift from signing there.
const (
	AccountPerms   = "rwdxylacupfti"
	ContainerPerms = "racwdxltfmeopi"
	BlobPerms      = "racwdxyltmeopi"
)

// timeLayouts are the four forms azure-cli accepts for --expiry and --start,
// tried in this order. See _validators.py:get_datetime_type.
var timeLayouts = []string{
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04Z",
	"2006-01-02T15Z",
	"2006-01-02",
}

// ParseTime parses a --expiry or --start value as UTC.
func ParseTime(s string) (time.Time, error) {
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("input '%s' not valid. Valid example: 2000-12-31T12:59:59Z", s)
}

// Canonical validates every character of input against order, then returns the
// characters sorted into that order with duplicates removed. The service
// requires these strings in a fixed order; label names the flag for errors.
func Canonical(input, order, label string) (string, error) {
	for _, r := range input {
		if !strings.ContainsRune(order, r) {
			return "", fmt.Errorf("invalid %s '%c': valid values are %s or a combination thereof", label, r, order)
		}
	}
	var b strings.Builder
	for _, r := range order {
		if strings.ContainsRune(input, r) {
			b.WriteRune(r)
		}
	}
	return b.String(), nil
}

// ParseIPRange parses a single IPv4 address or an "ip1-ip2" range.
func ParseIPRange(s string) (azsas.IPRange, error) {
	if s == "" {
		return azsas.IPRange{}, nil
	}
	startStr, endStr, hasEnd := strings.Cut(s, "-")
	start := net.ParseIP(strings.TrimSpace(startStr))
	if start == nil {
		return azsas.IPRange{}, fmt.Errorf("invalid --ip value %q: expected an IPv4 address or a range like 168.1.5.60-168.1.5.70", s)
	}
	r := azsas.IPRange{Start: start}
	if hasEnd {
		end := net.ParseIP(strings.TrimSpace(endStr))
		if end == nil {
			return azsas.IPRange{}, fmt.Errorf("invalid --ip value %q: expected an IPv4 address or a range like 168.1.5.60-168.1.5.70", s)
		}
		r.End = end
	}
	return r, nil
}

// alwaysSafe are the characters Python's urllib.parse.quote never escapes.
const alwaysSafe = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_.-~"

// Quote reproduces Python's urllib.parse.quote(s, safe=safe), which is what
// azure-cli applies to a blob SAS token (operations/blob.py:906). Go's
// url.QueryEscape uses a different safe set and cannot be substituted.
func Quote(s, safe string) string {
	var b bytes.Buffer
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.IndexByte(alwaysSafe, c) >= 0 || strings.IndexByte(safe, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

// NewSharedKey wraps the SDK constructor with a clearer error.
func NewSharedKey(accountName, accountKey string) (*azsas.SharedKeyCredential, error) {
	cred, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("invalid storage account key: %w", err)
	}
	return cred, nil
}

// OutputFormat reads the global --output flag for a SAS token result.
//
// A SAS token is a bare string, and output.PrintFormatted only renders json
// and tsv, so "table" is mapped to "json" the way internal/role/list.go does.
func OutputFormat(cmd *cobra.Command) string {
	format, _ := cmd.Flags().GetString("output")
	if format == "table" || format == "" {
		return "json"
	}
	return format
}

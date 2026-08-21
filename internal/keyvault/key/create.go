package key

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// CreateOptions carries the flags of az keyvault key create. The field set
// follows create_key (custom.py:1091).
type CreateOptions struct {
	VaultName string
	Name      string
	Kty       string
	Curve     string
	Size      int32
	Ops       []string
	Expires   string
	NotBefore string
	Tags      []string
	Disabled  bool
}

// keyTimeLayouts are the four forms azure-cli accepts for --expires and
// --not-before, tried in this order. See the keyvault module's own
// datetime_type at _validators.py:503.
var keyTimeLayouts = []string{
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04Z",
	"2006-01-02T15Z",
	"2006-01-02",
}

func parseKeyTime(s string) (time.Time, error) {
	for _, layout := range keyTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("input '%s' not valid. Valid example: 2000-12-31T12:59:59Z", s)
}

// parseKeyTags turns "key=value" pairs into a tag map. A pair without '=' gets
// an empty value, as azure-cli does.
func parseKeyTags(pairs []string) map[string]*string {
	if len(pairs) == 0 {
		return nil
	}
	tags := make(map[string]*string, len(pairs))
	for _, pair := range pairs {
		key, value, _ := strings.Cut(pair, "=")
		tags[key] = to.Ptr(value)
	}
	return tags
}

func Create(ctx context.Context, cmd *cobra.Command, opts CreateOptions) error {
	params := azkeys.CreateKeyParameters{
		Kty:           to.Ptr(azkeys.KeyType(opts.Kty)),
		Tags:          parseKeyTags(opts.Tags),
		KeyAttributes: &azkeys.KeyAttributes{Enabled: to.Ptr(!opts.Disabled)},
	}
	if opts.Curve != "" {
		params.Curve = to.Ptr(azkeys.CurveName(opts.Curve))
	}
	if opts.Size > 0 {
		params.KeySize = to.Ptr(opts.Size)
	}
	for _, op := range opts.Ops {
		params.KeyOps = append(params.KeyOps, to.Ptr(azkeys.KeyOperation(op)))
	}
	if opts.Expires != "" {
		expires, err := parseKeyTime(opts.Expires)
		if err != nil {
			return err
		}
		params.KeyAttributes.Expires = &expires
	}
	if opts.NotBefore != "" {
		notBefore, err := parseKeyTime(opts.NotBefore)
		if err != nil {
			return err
		}
		params.KeyAttributes.NotBefore = &notBefore
	}

	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", opts.VaultName)
	client, err := azkeys.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create key client: %w", err)
	}
	resp, err := client.CreateKey(ctx, opts.Name, params, nil)
	if err != nil {
		return fmt.Errorf("failed to create key: %w", err)
	}
	return output.PrintJSON(cmd, resp.KeyBundle)
}

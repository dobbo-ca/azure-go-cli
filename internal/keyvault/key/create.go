package key

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, vaultName, name, kty, curve string, size int32) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)
	client, err := azkeys.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create key client: %w", err)
	}
	params := azkeys.CreateKeyParameters{Kty: to.Ptr(azkeys.KeyType(kty))}
	if curve != "" {
		params.Curve = to.Ptr(azkeys.CurveName(curve))
	}
	if size > 0 {
		params.KeySize = to.Ptr(size)
	}
	resp, err := client.CreateKey(ctx, name, params, nil)
	if err != nil {
		return fmt.Errorf("failed to create key: %w", err)
	}
	return output.PrintJSON(cmd, resp.KeyBundle)
}

package key

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func SetAttributes(ctx context.Context, cmd *cobra.Command, vaultName, name, version string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)
	client, err := azkeys.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create key client: %w", err)
	}
	var enabled *bool
	if cmd.Flags().Changed("enabled") {
		b, _ := cmd.Flags().GetBool("enabled")
		enabled = &b
	}
	params := azkeys.UpdateKeyParameters{KeyAttributes: &azkeys.KeyAttributes{Enabled: enabled}}
	resp, err := client.UpdateKey(ctx, name, version, params, nil)
	if err != nil {
		return fmt.Errorf("failed to update key: %w", err)
	}
	return output.PrintJSON(cmd, resp.KeyBundle)
}

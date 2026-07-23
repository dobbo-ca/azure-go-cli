package key

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Delete(ctx context.Context, cmd *cobra.Command, vaultName, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)
	client, err := azkeys.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create key client: %w", err)
	}
	resp, err := client.DeleteKey(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}
	return output.PrintJSON(cmd, resp.DeletedKey)
}

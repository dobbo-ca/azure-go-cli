package key

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Purge(ctx context.Context, cmd *cobra.Command, vaultName, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)
	client, err := azkeys.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create key client: %w", err)
	}
	_, err = client.PurgeDeletedKey(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to purge deleted key: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": "purged"})
}

package key

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func ListVersions(ctx context.Context, cmd *cobra.Command, vaultName, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)
	client, err := azkeys.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create key client: %w", err)
	}
	pager := client.NewListKeyPropertiesVersionsPager(name, nil)
	var items []*azkeys.KeyProperties
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list key versions: %w", err)
		}
		items = append(items, page.Value...)
	}
	return output.PrintJSON(cmd, items)
}

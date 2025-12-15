package secret

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

func List(ctx context.Context, vaultName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	// Key Vault URL format: https://{vault-name}.vault.azure.net/
	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)

	client, err := azsecrets.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create secrets client: %w", err)
	}

	pager := client.NewListSecretPropertiesPager(nil)

	fmt.Printf("%-40s %-20s %-30s\n", "NAME", "ENABLED", "UPDATED")
	fmt.Println("------------------------------------------------------------------------------------------------")

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get next page: %w", err)
		}

		for _, secret := range page.Value {
			name := ""
			if secret.ID != nil {
				// Extract secret name from ID (last segment of URL path)
				id := string(*secret.ID)
				for i := len(id) - 1; i >= 0; i-- {
					if id[i] == '/' {
						name = id[i+1:]
						break
					}
				}
			}

			enabled := "false"
			if secret.Attributes != nil && secret.Attributes.Enabled != nil && *secret.Attributes.Enabled {
				enabled = "true"
			}

			updated := ""
			if secret.Attributes != nil && secret.Attributes.Updated != nil {
				updated = secret.Attributes.Updated.Format("2006-01-02 15:04:05")
			}

			fmt.Printf("%-40s %-20s %-30s\n", name, enabled, updated)
		}
	}

	return nil
}

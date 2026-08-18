package keyvault

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armkeyvault.NewVaultsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create key vaults client: %w", err)
	}

	var vaults []map[string]interface{}

	if resourceGroup != "" {
		// List vaults in specific resource group
		pager := client.NewListByResourceGroupPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list key vaults: %w", err)
			}

			for _, vault := range page.Value {
				vaults = append(vaults, formatVault(vault))
			}
		}
	} else {
		// List all vaults in subscription
		pager := client.NewListBySubscriptionPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list key vaults: %w", err)
			}

			for _, vault := range page.Value {
				vaults = append(vaults, formatVault(vault))
			}
		}
	}

	return output.PrintJSON(cmd, vaults)
}

func formatVault(vault *armkeyvault.Vault) map[string]interface{} {
	result := map[string]interface{}{
		"name":          azure.GetStringValue(vault.Name),
		"location":      azure.GetStringValue(vault.Location),
		"resourceGroup": getResourceGroupFromID(azure.GetStringValue(vault.ID)),
	}

	if vault.Properties != nil {
		if vault.Properties.VaultURI != nil {
			result["vaultUri"] = *vault.Properties.VaultURI
		}
		if vault.Properties.TenantID != nil {
			result["tenantId"] = *vault.Properties.TenantID
		}
		if vault.Properties.SKU != nil && vault.Properties.SKU.Name != nil {
			result["sku"] = string(*vault.Properties.SKU.Name)
		}
		if vault.Properties.EnabledForDeployment != nil {
			result["enabledForDeployment"] = *vault.Properties.EnabledForDeployment
		}
		if vault.Properties.EnabledForDiskEncryption != nil {
			result["enabledForDiskEncryption"] = *vault.Properties.EnabledForDiskEncryption
		}
		if vault.Properties.EnabledForTemplateDeployment != nil {
			result["enabledForTemplateDeployment"] = *vault.Properties.EnabledForTemplateDeployment
		}
		if vault.Properties.EnableSoftDelete != nil {
			result["enableSoftDelete"] = *vault.Properties.EnableSoftDelete
		}
	}

	return result
}

func getResourceGroupFromID(id string) string {
	parsed, err := arm.ParseResourceID(id)
	if err != nil {
		return ""
	}
	return parsed.ResourceGroupName
}

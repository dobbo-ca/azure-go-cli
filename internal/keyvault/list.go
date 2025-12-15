package keyvault

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func List(ctx context.Context, resourceGroup string) error {
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

	data, err := json.MarshalIndent(vaults, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format key vaults: %w", err)
	}

	fmt.Println(string(data))
	return nil
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
	// ID format: /subscriptions/{sub}/resourceGroups/{rg}/providers/...
	parts := make([]string, 0)
	for _, part := range []rune(id) {
		if part == '/' {
			parts = append(parts, "")
		} else if len(parts) > 0 {
			parts[len(parts)-1] += string(part)
		}
	}

	for i, part := range parts {
		if part == "resourceGroups" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

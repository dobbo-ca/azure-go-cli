package account

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
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

	client, err := armstorage.NewAccountsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create storage accounts client: %w", err)
	}

	var accounts []map[string]interface{}

	if resourceGroup != "" {
		// List accounts in specific resource group
		pager := client.NewListByResourceGroupPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list storage accounts: %w", err)
			}

			for _, account := range page.Value {
				accounts = append(accounts, formatAccount(account))
			}
		}
	} else {
		// List all accounts in subscription
		pager := client.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list storage accounts: %w", err)
			}

			for _, account := range page.Value {
				accounts = append(accounts, formatAccount(account))
			}
		}
	}

	data, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format storage accounts: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func formatAccount(account *armstorage.Account) map[string]interface{} {
	result := map[string]interface{}{
		"name":          azure.GetStringValue(account.Name),
		"location":      azure.GetStringValue(account.Location),
		"resourceGroup": getResourceGroupFromID(azure.GetStringValue(account.ID)),
	}

	if account.SKU != nil {
		if account.SKU.Name != nil {
			result["sku"] = string(*account.SKU.Name)
		}
	}

	if account.Kind != nil {
		result["kind"] = string(*account.Kind)
	}

	if account.Properties != nil {
		if account.Properties.PrimaryEndpoints != nil {
			endpoints := map[string]interface{}{}
			if account.Properties.PrimaryEndpoints.Blob != nil {
				endpoints["blob"] = *account.Properties.PrimaryEndpoints.Blob
			}
			if account.Properties.PrimaryEndpoints.File != nil {
				endpoints["file"] = *account.Properties.PrimaryEndpoints.File
			}
			if account.Properties.PrimaryEndpoints.Queue != nil {
				endpoints["queue"] = *account.Properties.PrimaryEndpoints.Queue
			}
			if account.Properties.PrimaryEndpoints.Table != nil {
				endpoints["table"] = *account.Properties.PrimaryEndpoints.Table
			}
			if len(endpoints) > 0 {
				result["primaryEndpoints"] = endpoints
			}
		}
		if account.Properties.ProvisioningState != nil {
			result["provisioningState"] = string(*account.Properties.ProvisioningState)
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

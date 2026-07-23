package account

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Update(ctx context.Context, cmd *cobra.Command, resourceGroup, name, sku, accessTier string, tags map[string]string, allowBlobPublicAccess *bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armstorage.NewAccountsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create storage accounts client: %w", err)
	}

	params := armstorage.AccountUpdateParameters{}
	if sku != "" {
		params.SKU = &armstorage.SKU{Name: to.Ptr(armstorage.SKUName(sku))}
	}
	if len(tags) > 0 {
		azureTags := make(map[string]*string)
		for k, v := range tags {
			azureTags[k] = to.Ptr(v)
		}
		params.Tags = azureTags
	}
	if accessTier != "" || allowBlobPublicAccess != nil {
		props := &armstorage.AccountPropertiesUpdateParameters{
			AllowBlobPublicAccess: allowBlobPublicAccess,
		}
		if accessTier != "" {
			props.AccessTier = to.Ptr(armstorage.AccessTier(accessTier))
		}
		params.Properties = props
	}

	resp, err := client.Update(ctx, resourceGroup, name, params, nil)
	if err != nil {
		return fmt.Errorf("failed to update storage account: %w", err)
	}

	return output.PrintJSON(cmd, resp.Account)
}

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

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, sku string, tags map[string]string) error {
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

	// Convert tags to Azure format
	azureTags := make(map[string]*string)
	for k, v := range tags {
		azureTags[k] = to.Ptr(v)
	}

	// Parse SKU name
	skuName := armstorage.SKUNameStandardLRS
	switch sku {
	case "Standard_LRS":
		skuName = armstorage.SKUNameStandardLRS
	case "Standard_GRS":
		skuName = armstorage.SKUNameStandardGRS
	case "Standard_RAGRS":
		skuName = armstorage.SKUNameStandardRAGRS
	case "Standard_ZRS":
		skuName = armstorage.SKUNameStandardZRS
	case "Premium_LRS":
		skuName = armstorage.SKUNamePremiumLRS
	case "Premium_ZRS":
		skuName = armstorage.SKUNamePremiumZRS
	case "Standard_GZRS":
		skuName = armstorage.SKUNameStandardGZRS
	case "Standard_RAGZRS":
		skuName = armstorage.SKUNameStandardRAGZRS
	}

	parameters := armstorage.AccountCreateParameters{
		Location: to.Ptr(location),
		SKU: &armstorage.SKU{
			Name: to.Ptr(skuName),
		},
		Kind: to.Ptr(armstorage.KindStorageV2),
		Tags: azureTags,
		Properties: &armstorage.AccountPropertiesCreateParameters{
			AllowBlobPublicAccess: to.Ptr(false),
			MinimumTLSVersion:     to.Ptr(armstorage.MinimumTLSVersionTLS12),
		},
	}

	fmt.Printf("Creating storage account '%s'...\n", name)
	poller, err := client.BeginCreate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create storage account: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create storage account: %w", err)
	}

	return output.PrintJSON(cmd, result.Account)
}

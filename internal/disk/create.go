package disk

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location string, sizeGB int32, sku string, tags map[string]string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armcompute.NewDisksClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create disk client: %w", err)
	}

	// Convert tags
	azureTags := make(map[string]*string)
	for k, v := range tags {
		azureTags[k] = to.Ptr(v)
	}

	// Parse SKU
	var skuName armcompute.DiskStorageAccountTypes
	switch sku {
	case "Standard_LRS":
		skuName = armcompute.DiskStorageAccountTypesStandardLRS
	case "Premium_LRS":
		skuName = armcompute.DiskStorageAccountTypesPremiumLRS
	case "StandardSSD_LRS":
		skuName = armcompute.DiskStorageAccountTypesStandardSSDLRS
	case "UltraSSD_LRS":
		skuName = armcompute.DiskStorageAccountTypesUltraSSDLRS
	case "Premium_ZRS":
		skuName = armcompute.DiskStorageAccountTypesPremiumZRS
	case "StandardSSD_ZRS":
		skuName = armcompute.DiskStorageAccountTypesStandardSSDZRS
	default:
		return fmt.Errorf("invalid SKU: %s (valid values: Standard_LRS, Premium_LRS, StandardSSD_LRS, UltraSSD_LRS, Premium_ZRS, StandardSSD_ZRS)", sku)
	}

	parameters := armcompute.Disk{
		Location: to.Ptr(location),
		Tags:     azureTags,
		SKU: &armcompute.DiskSKU{
			Name: to.Ptr(skuName),
		},
		Properties: &armcompute.DiskProperties{
			CreationData: &armcompute.CreationData{
				CreateOption: to.Ptr(armcompute.DiskCreateOptionEmpty),
			},
			DiskSizeGB: to.Ptr(sizeGB),
		},
	}

	fmt.Printf("Creating disk '%s'...\n", name)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to create disk: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete disk creation: %w", err)
	}

	fmt.Printf("Created disk '%s'\n", name)
	return output.PrintJSON(cmd, result.Disk)
}

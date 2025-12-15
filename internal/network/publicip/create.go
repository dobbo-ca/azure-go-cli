package publicip

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, allocationMethod, sku string, tags map[string]string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create public IP client: %w", err)
	}

	// Convert tags
	azureTags := make(map[string]*string)
	for k, v := range tags {
		azureTags[k] = to.Ptr(v)
	}

	// Parse allocation method
	var allocation armnetwork.IPAllocationMethod
	switch allocationMethod {
	case "Static":
		allocation = armnetwork.IPAllocationMethodStatic
	case "Dynamic":
		allocation = armnetwork.IPAllocationMethodDynamic
	default:
		return fmt.Errorf("invalid allocation method: %s (must be Static or Dynamic)", allocationMethod)
	}

	// Parse SKU
	var skuName armnetwork.PublicIPAddressSKUName
	switch sku {
	case "Basic":
		skuName = armnetwork.PublicIPAddressSKUNameBasic
	case "Standard":
		skuName = armnetwork.PublicIPAddressSKUNameStandard
	default:
		return fmt.Errorf("invalid SKU: %s (must be Basic or Standard)", sku)
	}

	parameters := armnetwork.PublicIPAddress{
		Location: to.Ptr(location),
		Tags:     azureTags,
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(skuName),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(allocation),
		},
	}

	fmt.Printf("Creating public IP address '%s'...\n", name)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to create public IP: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete public IP creation: %w", err)
	}

	fmt.Printf("Created public IP address '%s'\n", name)
	return output.PrintJSON(cmd, result.PublicIPAddress)
}

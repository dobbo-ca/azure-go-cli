package prefix

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location string, length int32, version, zones string, tags map[string]string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewPublicIPPrefixesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create public IP prefix client: %w", err)
	}

	// Convert tags
	azureTags := make(map[string]*string)
	for k, v := range tags {
		azureTags[k] = to.Ptr(v)
	}

	// Parse IP version
	var ipVersion armnetwork.IPVersion
	switch version {
	case "IPv4":
		ipVersion = armnetwork.IPVersionIPv4
	case "IPv6":
		ipVersion = armnetwork.IPVersionIPv6
	default:
		return fmt.Errorf("invalid version: %s (must be IPv4 or IPv6)", version)
	}

	// Parse zones
	var zoneList []*string
	for _, z := range strings.Split(zones, ",") {
		z = strings.TrimSpace(z)
		if z != "" {
			zoneList = append(zoneList, to.Ptr(z))
		}
	}

	parameters := armnetwork.PublicIPPrefix{
		Location: to.Ptr(location),
		Tags:     azureTags,
		Zones:    zoneList,
		SKU: &armnetwork.PublicIPPrefixSKU{
			Name: to.Ptr(armnetwork.PublicIPPrefixSKUNameStandard),
		},
		Properties: &armnetwork.PublicIPPrefixPropertiesFormat{
			PrefixLength:           to.Ptr(length),
			PublicIPAddressVersion: to.Ptr(ipVersion),
		},
	}

	fmt.Printf("Creating public IP prefix '%s'...\n", name)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to create public IP prefix: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete public IP prefix creation: %w", err)
	}

	fmt.Printf("Created public IP prefix '%s'\n", name)
	return output.PrintJSON(cmd, result.PublicIPPrefix)
}

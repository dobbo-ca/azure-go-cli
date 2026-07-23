package localgateway

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, gatewayIP string, addressPrefixes []string, tags map[string]string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewLocalNetworkGatewaysClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create local network gateway client: %w", err)
	}

	// Convert tags to Azure format
	azureTags := make(map[string]*string)
	for k, v := range tags {
		azureTags[k] = to.Ptr(v)
	}

	props := &armnetwork.LocalNetworkGatewayPropertiesFormat{}
	if gatewayIP != "" {
		props.GatewayIPAddress = to.Ptr(gatewayIP)
	}
	if len(addressPrefixes) > 0 {
		prefixes := make([]*string, 0, len(addressPrefixes))
		for _, p := range addressPrefixes {
			prefixes = append(prefixes, to.Ptr(p))
		}
		props.LocalNetworkAddressSpace = &armnetwork.AddressSpace{AddressPrefixes: prefixes}
	}

	parameters := armnetwork.LocalNetworkGateway{
		Location:   to.Ptr(location),
		Tags:       azureTags,
		Properties: props,
	}

	fmt.Printf("Creating local network gateway '%s'...\n", name)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to create local network gateway: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete local network gateway creation: %w", err)
	}

	fmt.Printf("Created local network gateway '%s'\n", name)
	return output.PrintJSON(cmd, result.LocalNetworkGateway)
}

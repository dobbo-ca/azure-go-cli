package route

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

func Create(ctx context.Context, cmd *cobra.Command, name, routeTableName, resourceGroup, addressPrefix, nextHopType, nextHopIP string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewRoutesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create routes client: %w", err)
	}

	props := &armnetwork.RoutePropertiesFormat{
		AddressPrefix: to.Ptr(addressPrefix),
		NextHopType:   to.Ptr(armnetwork.RouteNextHopType(nextHopType)),
	}
	if nextHopIP != "" {
		props.NextHopIPAddress = to.Ptr(nextHopIP)
	}

	parameters := armnetwork.Route{
		Properties: props,
	}

	fmt.Printf("Creating route '%s' in route table '%s'...\n", name, routeTableName)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, routeTableName, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create route: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create route: %w", err)
	}

	return output.PrintJSON(cmd, result.Route)
}

// ValidateNextHopType returns nil if the provided next-hop-type is one of the
// allowed values; otherwise an error.
func ValidateNextHopType(v string) error {
	switch v {
	case "VirtualNetworkGateway", "VnetLocal", "Internet", "VirtualAppliance", "None":
		return nil
	}
	return fmt.Errorf("invalid --next-hop-type %q (must be one of: VirtualNetworkGateway, VnetLocal, Internet, VirtualAppliance, None)", v)
}

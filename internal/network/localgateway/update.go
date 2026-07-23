package localgateway

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Update(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, noWait bool) error {
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

	current, err := client.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get local network gateway: %w", err)
	}

	if current.Properties == nil {
		current.Properties = &armnetwork.LocalNetworkGatewayPropertiesFormat{}
	}
	props := current.Properties

	flags := cmd.Flags()

	if flags.Changed("gateway-ip-address") {
		v, _ := flags.GetString("gateway-ip-address")
		props.GatewayIPAddress = to.Ptr(v)
	}

	if flags.Changed("local-address-prefixes") {
		v, _ := flags.GetString("local-address-prefixes")
		if props.LocalNetworkAddressSpace == nil {
			props.LocalNetworkAddressSpace = &armnetwork.AddressSpace{}
		}
		prefixes := splitCSV(v)
		addrs := make([]*string, 0, len(prefixes))
		for _, p := range prefixes {
			addrs = append(addrs, to.Ptr(p))
		}
		props.LocalNetworkAddressSpace.AddressPrefixes = addrs
	}

	if flags.Changed("tags") {
		tags, _ := flags.GetStringToString("tags")
		azureTags := make(map[string]*string)
		for k, v := range tags {
			azureTags[k] = to.Ptr(v)
		}
		current.Tags = azureTags
	}

	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, current.LocalNetworkGateway, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update local network gateway: %w", err)
	}

	if noWait {
		fmt.Printf("Started update of local network gateway '%s'\n", name)
		return nil
	}

	fmt.Printf("Updating local network gateway '%s'...\n", name)
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update local network gateway: %w", err)
	}

	return output.PrintJSON(cmd, result.LocalNetworkGateway)
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

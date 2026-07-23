package routetable

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

func Update(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewRouteTablesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create route tables client: %w", err)
	}

	current, err := client.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get route table: %w", err)
	}

	flags := cmd.Flags()

	if flags.Changed("disable-bgp-route-propagation") {
		v, _ := flags.GetBool("disable-bgp-route-propagation")
		if current.Properties == nil {
			current.Properties = &armnetwork.RouteTablePropertiesFormat{}
		}
		current.Properties.DisableBgpRoutePropagation = to.Ptr(v)
	}

	if flags.Changed("tags") {
		tags, _ := flags.GetStringToString("tags")
		tagPtrs := make(map[string]*string, len(tags))
		for k, v := range tags {
			tagPtrs[k] = to.Ptr(v)
		}
		current.Tags = tagPtrs
	}

	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, current.RouteTable, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update route table: %w", err)
	}

	if noWait {
		fmt.Printf("Started update of route table '%s'\n", name)
		return nil
	}

	fmt.Printf("Updating route table '%s'...\n", name)
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update route table: %w", err)
	}

	return output.PrintJSON(cmd, result.RouteTable)
}

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

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location string, disableBGPRoutePropagation bool, tags map[string]string) error {
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

	tagPtrs := make(map[string]*string, len(tags))
	for k, v := range tags {
		v := v
		tagPtrs[k] = &v
	}

	parameters := armnetwork.RouteTable{
		Location: to.Ptr(location),
		Tags:     tagPtrs,
		Properties: &armnetwork.RouteTablePropertiesFormat{
			DisableBgpRoutePropagation: to.Ptr(disableBGPRoutePropagation),
		},
	}

	fmt.Printf("Creating route table '%s'...\n", name)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create route table: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create route table: %w", err)
	}

	return output.PrintJSON(cmd, result.RouteTable)
}

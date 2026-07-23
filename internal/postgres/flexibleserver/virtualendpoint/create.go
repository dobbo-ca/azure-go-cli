package virtualendpoint

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, endpointName, endpointType string, members []string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewVirtualEndpointsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create virtual endpoints client: %w", err)
	}

	memberPtrs := make([]*string, 0, len(members))
	for _, m := range members {
		memberPtrs = append(memberPtrs, to.Ptr(m))
	}

	parameters := armpostgresqlflexibleservers.VirtualEndpointResource{
		Properties: &armpostgresqlflexibleservers.VirtualEndpointResourceProperties{
			EndpointType: to.Ptr(armpostgresqlflexibleservers.VirtualEndpointType(endpointType)),
			Members:      memberPtrs,
		},
	}

	fmt.Printf("Creating virtual endpoint '%s' on server '%s'...\n", endpointName, serverName)
	poller, err := client.BeginCreate(ctx, resourceGroup, serverName, endpointName, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin virtual endpoint create: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "Virtual endpoint create started."})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("virtual endpoint create failed: %w", err)
	}

	return output.PrintJSON(cmd, resp.VirtualEndpointResource)
}

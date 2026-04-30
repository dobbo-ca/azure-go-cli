package route

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, name, routeTableName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armnetwork.NewRoutesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create routes client: %w", err)
	}

	resp, err := client.Get(ctx, resourceGroup, routeTableName, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get route: %w", err)
	}

	return output.PrintJSON(cmd, resp.Route)
}

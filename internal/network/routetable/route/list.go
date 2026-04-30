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

func List(ctx context.Context, cmd *cobra.Command, routeTableName, resourceGroup string) error {
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

	pager := client.NewListPager(resourceGroup, routeTableName, nil)
	var routes []*armnetwork.Route

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list routes: %w", err)
		}
		routes = append(routes, page.Value...)
	}

	return output.PrintJSON(cmd, routes)
}

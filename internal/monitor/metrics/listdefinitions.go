package metrics

import (
	"context"
	"fmt"

	armmonitor "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func ListDefinitions(ctx context.Context, cmd *cobra.Command, resource string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewMetricDefinitionsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create metric definitions client: %w", err)
	}

	var items []*armmonitor.MetricDefinition
	pager := client.NewListPager(resource, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list metric definitions: %w", err)
		}
		items = append(items, page.Value...)
	}
	return output.PrintJSON(cmd, items)
}

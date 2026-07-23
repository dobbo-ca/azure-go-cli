package application

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/hdinsight/armhdinsight"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armhdinsight.NewApplicationsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create applications client: %w", err)
	}

	var items []*armhdinsight.Application
	pager := client.NewListByClusterPager(resourceGroup, clusterName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list applications: %w", err)
		}
		items = append(items, page.Value...)
	}

	return output.PrintJSON(cmd, items)
}

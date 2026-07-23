package scriptaction

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/hdinsight/armhdinsight"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func ListExecutionHistory(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	history, err := armhdinsight.NewScriptExecutionHistoryClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create script execution history client: %w", err)
	}

	var items []*armhdinsight.RuntimeScriptActionDetail
	pager := history.NewListByClusterPager(resourceGroup, clusterName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list script execution history: %w", err)
		}
		items = append(items, page.Value...)
	}

	return output.PrintJSON(cmd, items)
}

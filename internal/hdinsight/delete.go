package hdinsight

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/hdinsight/armhdinsight"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Delete(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armhdinsight.NewClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create clusters client: %w", err)
	}

	fmt.Printf("Deleting cluster '%s'...\n", clusterName)
	poller, err := client.BeginDelete(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "delete started"})
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("'%s' deleted.", clusterName)})
}

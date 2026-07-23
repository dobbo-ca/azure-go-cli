package azuremonitor

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/hdinsight/armhdinsight"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Enable(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName, workspaceID, primaryKey string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armhdinsight.NewExtensionsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create extensions client: %w", err)
	}

	req := armhdinsight.AzureMonitorRequest{
		WorkspaceID: to.Ptr(workspaceID),
	}
	if primaryKey != "" {
		req.PrimaryKey = to.Ptr(primaryKey)
	}

	fmt.Printf("Enabling azure monitor for '%s'...\n", clusterName)
	poller, err := client.BeginEnableAzureMonitor(ctx, resourceGroup, clusterName, req, nil)
	if err != nil {
		return fmt.Errorf("failed to begin enable azure monitor: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "enable azure monitor started"})
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("enable azure monitor failed: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("azure monitor enabled for '%s'.", clusterName)})
}

package azuremonitor

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/hdinsight/armhdinsight"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Disable(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName string, noWait bool) error {
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

	fmt.Printf("Disabling azure monitor for '%s'...\n", clusterName)
	poller, err := client.BeginDisableAzureMonitor(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin disable azure monitor: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "disable azure monitor started"})
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("disable azure monitor failed: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("azure monitor disabled for '%s'.", clusterName)})
}

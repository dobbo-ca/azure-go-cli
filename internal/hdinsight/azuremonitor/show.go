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

func Show(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName string) error {
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

	resp, err := client.GetAzureMonitorStatus(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to get azure monitor status: %w", err)
	}

	return output.PrintJSON(cmd, resp.AzureMonitorResponse)
}

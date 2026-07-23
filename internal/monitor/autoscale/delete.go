package autoscale

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Delete(ctx context.Context, cmd *cobra.Command, resourceGroup, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewAutoscaleSettingsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create autoscale settings client: %w", err)
	}

	if _, err := client.Delete(ctx, resourceGroup, name, nil); err != nil {
		return fmt.Errorf("failed to delete autoscale setting: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("'%s' deleted.", name)})
}

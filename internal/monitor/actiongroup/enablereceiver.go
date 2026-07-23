package actiongroup

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func EnableReceiver(ctx context.Context, cmd *cobra.Command, resourceGroup, name, receiverName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewActionGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create action group client: %w", err)
	}
	if _, err := client.EnableReceiver(ctx, resourceGroup, name, armmonitor.EnableRequest{ReceiverName: to.Ptr(receiverName)}, nil); err != nil {
		return fmt.Errorf("failed to enable receiver: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("receiver '%s' enabled.", receiverName)})
}

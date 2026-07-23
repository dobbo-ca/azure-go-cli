package nodepool

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func OperationAbort(ctx context.Context, cmd *cobra.Command, clusterName, nodepoolName, resourceGroup string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewAgentPoolsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create agent pools client: %w", err)
	}

	fmt.Printf("Aborting latest operation on node pool '%s'...\n", nodepoolName)

	poller, err := client.BeginAbortLatestOperation(ctx, resourceGroup, clusterName, nodepoolName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin abort latest operation: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "abort started"})
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("abort latest operation failed: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("aborted latest operation on node pool '%s'.", nodepoolName)})
}

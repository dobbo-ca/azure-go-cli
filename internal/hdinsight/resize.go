package hdinsight

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

func Resize(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName, roleName string, targetInstanceCount int32, noWait bool) error {
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

	fmt.Printf("Resizing role '%s' of cluster '%s' to %d instances...\n", roleName, clusterName, targetInstanceCount)
	poller, err := client.BeginResize(ctx, resourceGroup, clusterName, armhdinsight.RoleName(roleName), armhdinsight.ClusterResizeParameters{
		TargetInstanceCount: to.Ptr(targetInstanceCount),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin resize: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "resize started"})
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("resize failed: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("'%s' resized.", clusterName)})
}

package nodepool

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Add(ctx context.Context, clusterName, nodepoolName, resourceGroup string, nodeCount int32, vmSize string) error {
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

	// Check if nodepool already exists
	_, err = client.Get(ctx, resourceGroup, clusterName, nodepoolName, nil)
	if err == nil {
		return fmt.Errorf("node pool '%s' already exists in cluster '%s'", nodepoolName, clusterName)
	}

	fmt.Printf("Creating node pool '%s' in cluster '%s'...\n", nodepoolName, clusterName)

	// Create node pool with basic configuration
	nodePool := armcontainerservice.AgentPool{
		Properties: &armcontainerservice.ManagedClusterAgentPoolProfileProperties{
			Count:  &nodeCount,
			VMSize: &vmSize,
			OSType: azure.GetOSType("Linux"),
			Mode:   azure.GetAgentPoolMode("User"),
		},
	}

	// Start the create operation (long-running operation)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, clusterName, nodepoolName, nodePool, nil)
	if err != nil {
		return fmt.Errorf("failed to start node pool create operation: %w", err)
	}

	fmt.Println("Node pool create operation started. Waiting for completion...")

	// Wait for the operation to complete
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("node pool create operation failed: %w", err)
	}

	count := int32(0)
	if result.Properties != nil && result.Properties.Count != nil {
		count = *result.Properties.Count
	}

	fmt.Printf("Successfully created node pool '%s' with %d nodes\n", nodepoolName, count)
	return nil
}

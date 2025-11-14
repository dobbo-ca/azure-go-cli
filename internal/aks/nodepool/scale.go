package nodepool

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Scale(ctx context.Context, clusterName, nodepoolName, resourceGroup string, nodeCount int32) error {
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

	// Get current nodepool configuration
	fmt.Printf("Getting current configuration for node pool '%s'...\n", nodepoolName)
	pool, err := client.Get(ctx, resourceGroup, clusterName, nodepoolName, nil)
	if err != nil {
		return fmt.Errorf("failed to get node pool: %w", err)
	}

	// Verify the nodepool exists and has properties
	if pool.Properties == nil {
		return fmt.Errorf("node pool '%s' has no properties", nodepoolName)
	}

	currentCount := int32(0)
	if pool.Properties.Count != nil {
		currentCount = *pool.Properties.Count
	}

	fmt.Printf("Current node count: %d\n", currentCount)
	fmt.Printf("Scaling node pool '%s' to %d nodes...\n", nodepoolName, nodeCount)

	// Update the count
	pool.Properties.Count = &nodeCount

	// Start the scale operation (long-running operation)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, clusterName, nodepoolName, pool.AgentPool, nil)
	if err != nil {
		return fmt.Errorf("failed to start scale operation: %w", err)
	}

	fmt.Println("Scale operation started. Waiting for completion...")

	// Wait for the operation to complete
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("scale operation failed: %w", err)
	}

	newCount := int32(0)
	if result.Properties != nil && result.Properties.Count != nil {
		newCount = *result.Properties.Count
	}

	fmt.Printf("Successfully scaled node pool '%s' to %d nodes\n", nodepoolName, newCount)
	return nil
}

package nodepool

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, clusterName, nodepoolName, resourceGroup string, noWait bool) error {
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

	// Verify nodepool exists
	pool, err := client.Get(ctx, resourceGroup, clusterName, nodepoolName, nil)
	if err != nil {
		return fmt.Errorf("failed to get node pool: %w", err)
	}

	nodeCount := int32(0)
	if pool.Properties != nil && pool.Properties.Count != nil {
		nodeCount = *pool.Properties.Count
	}

	// Prompt for confirmation
	fmt.Printf("WARNING: This will delete node pool '%s' (%d nodes) from cluster '%s'\n", nodepoolName, nodeCount, clusterName)
	fmt.Print("Are you sure you want to continue? (yes/no): ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "yes" && response != "y" {
		fmt.Println("Delete operation cancelled")
		return nil
	}

	fmt.Printf("Deleting node pool '%s'...\n", nodepoolName)

	// Start the delete operation (long-running operation)
	poller, err := client.BeginDelete(ctx, resourceGroup, clusterName, nodepoolName, nil)
	if err != nil {
		return fmt.Errorf("failed to start node pool delete operation: %w", err)
	}

	if noWait {
		fmt.Println("Node pool delete operation started (running in background)")
		return nil
	}

	fmt.Println("Node pool delete operation started. Waiting for completion...")

	// Wait for the operation to complete
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("node pool delete operation failed: %w", err)
	}

	fmt.Printf("Successfully deleted node pool '%s'\n", nodepoolName)
	return nil
}

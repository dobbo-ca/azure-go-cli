package addon

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Disable(ctx context.Context, clusterName, resourceGroup, addonName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create AKS client: %w", err)
	}

	// Get current cluster configuration
	fmt.Printf("Getting cluster configuration for '%s'...\n", clusterName)
	cluster, err := client.Get(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	if cluster.Properties == nil || cluster.Properties.AddonProfiles == nil {
		return fmt.Errorf("no addons found for cluster '%s'", clusterName)
	}

	// Check if addon exists
	profile, exists := cluster.Properties.AddonProfiles[addonName]
	if !exists {
		return fmt.Errorf("addon '%s' not found in cluster '%s'", addonName, clusterName)
	}

	if profile.Enabled != nil && !*profile.Enabled {
		fmt.Printf("Addon '%s' is already disabled\n", addonName)
		return nil
	}

	// Disable the addon
	disabled := false
	profile.Enabled = &disabled

	fmt.Printf("Disabling addon '%s' on cluster '%s'...\n", addonName, clusterName)

	// Start the update operation (long-running operation)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, clusterName, cluster.ManagedCluster, nil)
	if err != nil {
		return fmt.Errorf("failed to start addon disable operation: %w", err)
	}

	fmt.Println("Addon disable operation started. Waiting for completion...")

	// Wait for the operation to complete
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("addon disable operation failed: %w", err)
	}

	fmt.Printf("Successfully disabled addon '%s' on cluster '%s'\n", addonName, clusterName)
	return nil
}

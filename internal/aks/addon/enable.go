package addon

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Enable(ctx context.Context, clusterName, resourceGroup, addonName string, addonConfig map[string]string) error {
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

	if cluster.Properties == nil {
		return fmt.Errorf("cluster properties are nil")
	}

	// Initialize addon profiles if needed
	if cluster.Properties.AddonProfiles == nil {
		cluster.Properties.AddonProfiles = make(map[string]*armcontainerservice.ManagedClusterAddonProfile)
	}

	// Check if addon already exists
	profile, exists := cluster.Properties.AddonProfiles[addonName]
	if exists && profile.Enabled != nil && *profile.Enabled {
		fmt.Printf("Addon '%s' is already enabled\n", addonName)
		return nil
	}

	// Create or update addon profile
	enabled := true
	if !exists {
		profile = &armcontainerservice.ManagedClusterAddonProfile{
			Enabled: &enabled,
		}
		cluster.Properties.AddonProfiles[addonName] = profile
	} else {
		profile.Enabled = &enabled
	}

	// Add any configuration
	if len(addonConfig) > 0 {
		if profile.Config == nil {
			profile.Config = make(map[string]*string)
		}
		for key, value := range addonConfig {
			v := value
			profile.Config[key] = &v
		}
	}

	fmt.Printf("Enabling addon '%s' on cluster '%s'...\n", addonName, clusterName)

	// Start the update operation (long-running operation)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, clusterName, cluster.ManagedCluster, nil)
	if err != nil {
		return fmt.Errorf("failed to start addon enable operation: %w", err)
	}

	fmt.Println("Addon enable operation started. Waiting for completion...")

	// Wait for the operation to complete
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("addon enable operation failed: %w", err)
	}

	fmt.Printf("Successfully enabled addon '%s' on cluster '%s'\n", addonName, clusterName)
	return nil
}

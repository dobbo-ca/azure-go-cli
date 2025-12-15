package operation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, clusterName, resourceGroup, operationID string) error {
	// Note: The v6 SDK doesn't have a direct way to query specific operation status by ID
	// This functionality may require using the Azure Resource Manager operations API directly
	// For now, return an error indicating this is not yet implemented
	return fmt.Errorf("operation status lookup by ID is not yet implemented in SDK v6")
}

func ShowLatest(ctx context.Context, clusterName, resourceGroup string) error {
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
		return fmt.Errorf("failed to create clusters client: %w", err)
	}

	// Get cluster to find latest operation
	cluster, err := client.Get(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// The cluster properties don't directly expose operations in a list format
	// We'll return the provisioning state which represents the latest cluster operation
	result := map[string]interface{}{
		"cluster": clusterName,
	}

	if cluster.Properties != nil {
		if cluster.Properties.ProvisioningState != nil {
			result["provisioningState"] = *cluster.Properties.ProvisioningState
		}
		if cluster.Properties.PowerState != nil && cluster.Properties.PowerState.Code != nil {
			result["powerState"] = *cluster.Properties.PowerState.Code
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format cluster status: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

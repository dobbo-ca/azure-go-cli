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

func List(ctx context.Context, cmd *cobra.Command, clusterName, resourceGroup string) error {
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

	pager := client.NewListPager(resourceGroup, clusterName, nil)
	var nodepools []map[string]interface{}

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list node pools: %w", err)
		}

		for _, pool := range page.Value {
			nodepools = append(nodepools, formatNodePool(pool))
		}
	}

	return output.PrintJSON(cmd, nodepools)
}

func formatNodePool(pool *armcontainerservice.AgentPool) map[string]interface{} {
	result := map[string]interface{}{
		"name": azure.GetStringValue(pool.Name),
	}

	if pool.Properties != nil {
		if pool.Properties.Count != nil {
			result["count"] = *pool.Properties.Count
		}
		if pool.Properties.VMSize != nil {
			result["vmSize"] = *pool.Properties.VMSize
		}
		if pool.Properties.OSDiskSizeGB != nil {
			result["osDiskSizeGB"] = *pool.Properties.OSDiskSizeGB
		}
		if pool.Properties.OSType != nil {
			result["osType"] = string(*pool.Properties.OSType)
		}
		if pool.Properties.ProvisioningState != nil {
			result["provisioningState"] = *pool.Properties.ProvisioningState
		}
		if pool.Properties.Mode != nil {
			result["mode"] = string(*pool.Properties.Mode)
		}
		if pool.Properties.OrchestratorVersion != nil {
			result["kubernetesVersion"] = *pool.Properties.OrchestratorVersion
		}
		if pool.Properties.MaxCount != nil {
			result["maxCount"] = *pool.Properties.MaxCount
		}
		if pool.Properties.MinCount != nil {
			result["minCount"] = *pool.Properties.MinCount
		}
		if pool.Properties.EnableAutoScaling != nil {
			result["enableAutoScaling"] = *pool.Properties.EnableAutoScaling
		}
	}

	return result
}

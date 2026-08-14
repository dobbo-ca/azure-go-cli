package addon

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

	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create AKS client: %w", err)
	}

	cluster, err := client.Get(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	if cluster.Properties == nil || cluster.Properties.AddonProfiles == nil {
		return output.PrintJSON(cmd, []map[string]interface{}{})
	}

	addons := []map[string]interface{}{}
	for name, profile := range cluster.Properties.AddonProfiles {
		addon := map[string]interface{}{
			"name": name,
		}
		if profile.Enabled != nil {
			addon["enabled"] = *profile.Enabled
		}
		if profile.Config != nil {
			addon["config"] = profile.Config
		}
		addons = append(addons, addon)
	}

	return output.PrintJSON(cmd, addons)
}

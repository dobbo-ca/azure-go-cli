package addon

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, clusterName, resourceGroup, addonName string) error {
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
		return fmt.Errorf("no addons found for cluster %s", clusterName)
	}

	profile, exists := cluster.Properties.AddonProfiles[addonName]
	if !exists {
		return fmt.Errorf("addon %s not found in cluster %s", addonName, clusterName)
	}

	addon := map[string]interface{}{
		"name": addonName,
	}
	if profile.Enabled != nil {
		addon["enabled"] = *profile.Enabled
	}
	if profile.Config != nil {
		addon["config"] = profile.Config
	}
	if profile.Identity != nil {
		addon["identity"] = map[string]interface{}{
			"clientId":   azure.GetStringValue(profile.Identity.ClientID),
			"objectId":   azure.GetStringValue(profile.Identity.ObjectID),
			"resourceId": azure.GetStringValue(profile.Identity.ResourceID),
		}
	}

	data, err := json.MarshalIndent(addon, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format addon: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

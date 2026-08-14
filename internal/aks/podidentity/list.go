package podidentity

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
		return fmt.Errorf("failed to create clusters client: %w", err)
	}

	cluster, err := client.Get(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	var identities []map[string]interface{}

	if cluster.Properties != nil && cluster.Properties.PodIdentityProfile != nil {
		if cluster.Properties.PodIdentityProfile.UserAssignedIdentities != nil {
			for _, identity := range cluster.Properties.PodIdentityProfile.UserAssignedIdentities {
				id := map[string]interface{}{
					"name":      azure.GetStringValue(identity.Name),
					"namespace": azure.GetStringValue(identity.Namespace),
				}
				if identity.Identity != nil {
					id["resourceId"] = azure.GetStringValue(identity.Identity.ResourceID)
					id["clientId"] = azure.GetStringValue(identity.Identity.ClientID)
					id["objectId"] = azure.GetStringValue(identity.Identity.ObjectID)
				}
				if identity.ProvisioningState != nil {
					id["provisioningState"] = string(*identity.ProvisioningState)
				}
				identities = append(identities, id)
			}
		}
	}

	return output.PrintJSON(cmd, identities)
}

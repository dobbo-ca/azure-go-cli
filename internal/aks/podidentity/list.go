package podidentity

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func List(ctx context.Context, clusterName, resourceGroup string) error {
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

  data, err := json.MarshalIndent(identities, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format pod identities: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

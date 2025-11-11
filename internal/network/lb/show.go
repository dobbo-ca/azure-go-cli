package lb

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, lbName, resourceGroup string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armnetwork.NewLoadBalancersClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create load balancers client: %w", err)
  }

  lb, err := client.Get(ctx, resourceGroup, lbName, nil)
  if err != nil {
    return fmt.Errorf("failed to get load balancer: %w", err)
  }

  data, err := json.MarshalIndent(lb, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format load balancer: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

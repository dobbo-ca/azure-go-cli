package feature

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armfeatures"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, provider, featureName string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armfeatures.NewClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create features client: %w", err)
  }

  feature, err := client.Get(ctx, provider, featureName, nil)
  if err != nil {
    return fmt.Errorf("failed to get feature: %w", err)
  }

  data, err := json.MarshalIndent(feature, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format feature: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

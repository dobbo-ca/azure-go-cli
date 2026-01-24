package feature

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armfeatures"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Register(ctx context.Context, provider, featureName string) error {
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

  fmt.Printf("Registering feature '%s' for provider '%s'...\n", featureName, provider)

  feature, err := client.Register(ctx, provider, featureName, nil)
  if err != nil {
    return fmt.Errorf("failed to register feature: %w", err)
  }

  if feature.Properties != nil && feature.Properties.State != nil {
    state := *feature.Properties.State
    fmt.Printf("Feature registration initiated. State: %s\n", state)

    if state != "Registered" {
      fmt.Println("\nNote: Feature registration may take several minutes.")
      fmt.Printf("Check status with: az feat show --provider %s --name %s\n",
        provider, featureName)
    }
  }

  data, err := json.MarshalIndent(feature, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format feature: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

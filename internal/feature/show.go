package feature

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armfeatures"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, provider, featureName string) error {
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

  return output.PrintJSON(cmd, feature)
}

package resource

import (
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

// resolveSubscription returns the subscription ID from --subscription if set,
// otherwise from the default profile.
func resolveSubscription(cmd *cobra.Command) (string, error) {
  if sub, _ := cmd.Flags().GetString("subscription"); sub != "" {
    return sub, nil
  }
  return config.GetDefaultSubscription()
}

func newGenericClient(cmd *cobra.Command) (*armresources.Client, azcore.TokenCredential, string, error) {
  cred, err := azure.GetCredential()
  if err != nil {
    return nil, nil, "", err
  }
  sub, err := resolveSubscription(cmd)
  if err != nil {
    return nil, nil, "", err
  }
  c, err := armresources.NewClient(sub, cred, nil)
  if err != nil {
    return nil, nil, "", fmt.Errorf("failed to create resources client: %w", err)
  }
  return c, cred, sub, nil
}

func newTagsClient(cmd *cobra.Command) (*armresources.TagsClient, error) {
  cred, err := azure.GetCredential()
  if err != nil {
    return nil, err
  }
  sub, err := resolveSubscription(cmd)
  if err != nil {
    return nil, err
  }
  c, err := armresources.NewTagsClient(sub, cred, nil)
  if err != nil {
    return nil, fmt.Errorf("failed to create tags client: %w", err)
  }
  return c, nil
}

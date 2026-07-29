package lock

import (
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
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

func newLocksClient(cmd *cobra.Command) (*armlocks.ManagementLocksClient, error) {
  cred, err := azure.GetCredential()
  if err != nil {
    return nil, err
  }
  sub, err := resolveSubscription(cmd)
  if err != nil {
    return nil, err
  }
  c, err := armlocks.NewManagementLocksClient(sub, cred, nil)
  if err != nil {
    return nil, fmt.Errorf("failed to create locks client: %w", err)
  }
  return c, nil
}

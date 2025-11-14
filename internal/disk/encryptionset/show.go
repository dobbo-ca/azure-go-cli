package encryptionset

import (
  "context"
  "encoding/json"
  "fmt"
  "os"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
  var output string
  var resourceGroup string
  var name string

  cmd := &cobra.Command{
    Use:   "show",
    Short: "Get information about a disk encryption set",
    Long:  "Show detailed information about a disk encryption set",
    RunE: func(cmd *cobra.Command, args []string) error {
      ctx := context.Background()
      return showDiskEncryptionSet(ctx, output, resourceGroup, name)
    },
  }

  cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format: json")
  cmd.Flags().StringVarP(&resourceGroup, "resource-group", "g", "", "Resource group name")
  cmd.Flags().StringVarP(&name, "name", "n", "", "Disk encryption set name")
  cmd.MarkFlagRequired("name")
  cmd.MarkFlagRequired("resource-group")

  return cmd
}

func showDiskEncryptionSet(ctx context.Context, output, resourceGroup, name string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return fmt.Errorf("failed to get credentials: %w", err)
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  client, err := armcompute.NewDiskEncryptionSetsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create disk encryption sets client: %w", err)
  }

  set, err := client.Get(ctx, resourceGroup, name, nil)
  if err != nil {
    return fmt.Errorf("failed to get disk encryption set: %w", err)
  }

  enc := json.NewEncoder(os.Stdout)
  enc.SetIndent("", "  ")
  return enc.Encode(set)
}

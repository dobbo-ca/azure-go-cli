package backupinstance

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "list",
    Short: "List backup instances in a vault",
    Long:  "Lists all backup instances in a backup vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      return ListBackupInstances(context.Background(), resourceGroup, vaultName)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  return cmd
}

func ListBackupInstances(ctx context.Context, resourceGroup, vaultName string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupInstancesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instances client: %w", err)
  }

  pager := client.NewListPager(resourceGroup, vaultName, nil)
  var instances []*armdataprotection.BackupInstanceResource
  for pager.More() {
    page, err := pager.NextPage(ctx)
    if err != nil {
      return fmt.Errorf("failed to list backup instances: %w", err)
    }
    instances = append(instances, page.Value...)
  }

  output, err := json.MarshalIndent(instances, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}

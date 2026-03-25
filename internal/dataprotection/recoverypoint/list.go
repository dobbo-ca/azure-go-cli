package recoverypoint

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
    Short: "List recovery points for a backup instance",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      backupInstanceName, _ := cmd.Flags().GetString("backup-instance-name")
      return ListRecoveryPoints(context.Background(), resourceGroup, vaultName, backupInstanceName)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().String("backup-instance-name", "", "Name of the backup instance")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("backup-instance-name")
  return cmd
}

func ListRecoveryPoints(ctx context.Context, resourceGroup, vaultName, backupInstanceName string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewRecoveryPointsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create recovery points client: %w", err)
  }

  var points []*armdataprotection.AzureBackupRecoveryPointResource
  pager := client.NewListPager(resourceGroup, vaultName, backupInstanceName, nil)
  for pager.More() {
    page, err := pager.NextPage(ctx)
    if err != nil {
      return fmt.Errorf("failed to list recovery points: %w", err)
    }
    points = append(points, page.Value...)
  }

  output, err := json.MarshalIndent(points, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format recovery points: %w", err)
  }

  fmt.Println(string(output))
  return nil
}

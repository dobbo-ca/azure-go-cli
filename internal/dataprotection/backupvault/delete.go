package backupvault

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newDeleteCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a backup vault",
    Long:  "Deletes a backup vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return DeleteBackupVault(context.Background(), resourceGroup, vaultName, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().StringP("vault-name", "v", "", "Name of the backup vault")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  return cmd
}

func DeleteBackupVault(ctx context.Context, resourceGroup, vaultName string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupVaultsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup vaults client: %w", err)
  }

  poller, err := client.BeginDelete(ctx, resourceGroup, vaultName, nil)
  if err != nil {
    return fmt.Errorf("failed to delete backup vault: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Delete operation started. Use 'az dataprotection backup-vault list' to confirm deletion."}`)
    return nil
  }

  _, err = poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("delete backup vault operation failed: %w", err)
  }

  fmt.Println(`{"status": "Backup vault deleted successfully."}`)
  return nil
}

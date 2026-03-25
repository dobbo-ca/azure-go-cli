package backupinstance

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newSuspendBackupCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "suspend-backup",
    Short: "Suspend backups for a backup instance",
    Long:  "Suspends scheduled backups for a backup instance in a backup vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return SuspendBackup(context.Background(), resourceGroup, vaultName, name, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().StringP("name", "n", "", "Name of the backup instance")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("name")
  return cmd
}

func SuspendBackup(ctx context.Context, resourceGroup, vaultName, name string, noWait bool) error {
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

  poller, err := client.BeginSuspendBackups(ctx, resourceGroup, vaultName, name, nil)
  if err != nil {
    return fmt.Errorf("failed to suspend backups: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Suspend-backup operation started. Use 'az dataprotection backup-instance show' to check status."}`)
    return nil
  }

  _, err = poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("suspend-backup operation failed: %w", err)
  }

  fmt.Println(`{"status": "Backups suspended successfully."}`)
  return nil
}

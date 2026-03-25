package backupinstance

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newAdhocBackupCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "adhoc-backup",
    Short: "Trigger an ad-hoc backup for a backup instance",
    Long:  "Triggers an ad-hoc backup for a backup instance using a specified backup rule",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      ruleName, _ := cmd.Flags().GetString("rule-name")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return AdhocBackup(cmd, context.Background(), resourceGroup, vaultName, name, ruleName, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().StringP("name", "n", "", "Name of the backup instance")
  cmd.Flags().String("rule-name", "", "Name of the backup rule to use")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("name")
  cmd.MarkFlagRequired("rule-name")
  return cmd
}

func AdhocBackup(cmd *cobra.Command, ctx context.Context, resourceGroup, vaultName, name, ruleName string, noWait bool) error {
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

  backupRequest := armdataprotection.TriggerBackupRequest{
    BackupRuleOptions: &armdataprotection.AdHocBackupRuleOptions{
      RuleName: to.Ptr(ruleName),
      TriggerOption: &armdataprotection.AdhocBackupTriggerOption{
        RetentionTagOverride: to.Ptr("Default"),
      },
    },
  }

  poller, err := client.BeginAdhocBackup(ctx, resourceGroup, vaultName, name, backupRequest, nil)
  if err != nil {
    return fmt.Errorf("failed to trigger adhoc backup: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Adhoc backup operation started. Use 'az dataprotection job list' to monitor progress."}`)
    return nil
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("adhoc backup operation failed: %w", err)
  }

  return output.PrintJSON(cmd, result)
}

package backupvault

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newUpdateCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "update",
    Short: "Update a backup vault",
    Long:  "Updates tags on a backup vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      tags, _ := cmd.Flags().GetStringToString("tags")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return UpdateBackupVault(context.Background(), resourceGroup, vaultName, tags, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().StringP("vault-name", "v", "", "Name of the backup vault")
  cmd.Flags().StringToString("tags", nil, "Space-separated tags in key=value format")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  return cmd
}

func UpdateBackupVault(ctx context.Context, resourceGroup, vaultName string, tags map[string]string, noWait bool) error {
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

  tagPtrs := make(map[string]*string, len(tags))
  for k, v := range tags {
    v := v
    tagPtrs[k] = &v
  }

  patchInput := armdataprotection.PatchResourceRequestInput{
    Tags: tagPtrs,
  }

  poller, err := client.BeginUpdate(ctx, resourceGroup, vaultName, patchInput, nil)
  if err != nil {
    return fmt.Errorf("failed to update backup vault: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Update operation started. Use 'az dataprotection backup-vault show' to check status."}`)
    return nil
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("update backup vault operation failed: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}

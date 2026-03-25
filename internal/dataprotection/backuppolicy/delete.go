package backuppolicy

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
    Short: "Delete a backup policy",
    Long:  "Deletes a backup policy from a backup vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      return DeleteBackupPolicy(context.Background(), resourceGroup, vaultName, name)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().StringP("name", "n", "", "Name of the backup policy")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("name")
  return cmd
}

func DeleteBackupPolicy(ctx context.Context, resourceGroup, vaultName, name string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupPoliciesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup policies client: %w", err)
  }

  _, err = client.Delete(ctx, resourceGroup, vaultName, name, nil)
  if err != nil {
    return fmt.Errorf("failed to delete backup policy: %w", err)
  }

  fmt.Println(`{"status": "Backup policy deleted successfully."}`)
  return nil
}

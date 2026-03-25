package backupvault

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newCreateCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "create",
    Short: "Create a backup vault",
    Long:  "Creates or updates a backup vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      location, _ := cmd.Flags().GetString("location")
      datastoreType, _ := cmd.Flags().GetString("datastore-type")
      storageType, _ := cmd.Flags().GetString("type")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return CreateBackupVault(context.Background(), resourceGroup, vaultName, location, datastoreType, storageType, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().StringP("vault-name", "v", "", "Name of the backup vault")
  cmd.Flags().StringP("location", "l", "", "Location of the backup vault")
  cmd.Flags().String("datastore-type", "VaultStore", "Datastore type for storage settings")
  cmd.Flags().String("type", "LocallyRedundant", "Storage redundancy type")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("location")
  return cmd
}

func CreateBackupVault(ctx context.Context, resourceGroup, vaultName, location, datastoreType, storageType string, noWait bool) error {
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

  vaultResource := armdataprotection.BackupVaultResource{
    Location: to.Ptr(location),
    Properties: &armdataprotection.BackupVault{
      StorageSettings: []*armdataprotection.StorageSetting{
        {
          DatastoreType: to.Ptr(armdataprotection.StorageSettingStoreTypes(datastoreType)),
          Type:          to.Ptr(armdataprotection.StorageSettingTypes(storageType)),
        },
      },
    },
  }

  poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, vaultName, vaultResource, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup vault: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Create operation started. Use 'az dataprotection backup-vault show' to check status."}`)
    return nil
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("create backup vault operation failed: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}

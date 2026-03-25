package backupinstance

import (
  "context"
  "encoding/json"
  "fmt"
  "os"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newCreateCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "create",
    Short: "Create or update a backup instance",
    Long:  "Creates or updates a backup instance in a backup vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      backupInstanceFile, _ := cmd.Flags().GetString("backup-instance")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return CreateBackupInstance(context.Background(), resourceGroup, vaultName, backupInstanceFile, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().String("backup-instance", "", "Path to JSON file containing backup instance resource")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("backup-instance")
  return cmd
}

func CreateBackupInstance(ctx context.Context, resourceGroup, vaultName, backupInstanceFile string, noWait bool) error {
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

  data, err := os.ReadFile(backupInstanceFile)
  if err != nil {
    return fmt.Errorf("failed to read backup instance file %s: %w", backupInstanceFile, err)
  }

  var instanceResource armdataprotection.BackupInstanceResource
  if err := json.Unmarshal(data, &instanceResource); err != nil {
    return fmt.Errorf("failed to parse backup instance JSON: %w", err)
  }

  // Determine instance name from resource Name or Properties.FriendlyName
  instanceName := ""
  if instanceResource.Name != nil {
    instanceName = *instanceResource.Name
  } else if instanceResource.Properties != nil && instanceResource.Properties.FriendlyName != nil {
    instanceName = *instanceResource.Properties.FriendlyName
  }
  if instanceName == "" {
    return fmt.Errorf("backup instance name could not be determined from JSON (set 'name' or 'properties.friendlyName')")
  }

  poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, vaultName, instanceName, instanceResource, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instance: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Create operation started. Use 'az dataprotection backup-instance show' to check status."}`)
    return nil
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("create backup instance operation failed: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}

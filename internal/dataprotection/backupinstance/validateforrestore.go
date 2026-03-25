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

func newValidateForRestoreCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "validate-for-restore",
    Short: "Validate a restore request for a backup instance",
    Long:  "Validates a restore request for a backup instance before triggering the actual restore",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      backupInstanceName, _ := cmd.Flags().GetString("backup-instance-name")
      restoreRequestFile, _ := cmd.Flags().GetString("restore-request-object")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return ValidateForRestore(context.Background(), resourceGroup, vaultName, backupInstanceName, restoreRequestFile, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().String("backup-instance-name", "", "Name of the backup instance")
  cmd.Flags().String("restore-request-object", "", "Path to JSON file containing restore request object")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("backup-instance-name")
  cmd.MarkFlagRequired("restore-request-object")
  return cmd
}

func ValidateForRestore(ctx context.Context, resourceGroup, vaultName, backupInstanceName, restoreRequestFile string, noWait bool) error {
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

  data, err := os.ReadFile(restoreRequestFile)
  if err != nil {
    return fmt.Errorf("failed to read restore request file %s: %w", restoreRequestFile, err)
  }

  var raw map[string]interface{}
  if err := json.Unmarshal(data, &raw); err != nil {
    return fmt.Errorf("failed to parse restore request JSON: %w", err)
  }

  objectType, _ := raw["objectType"].(string)

  var restoreRequest armdataprotection.AzureBackupRestoreRequestClassification
  switch objectType {
  case "AzureBackupRecoveryPointBasedRestoreRequest":
    var req armdataprotection.AzureBackupRecoveryPointBasedRestoreRequest
    if err := json.Unmarshal(data, &req); err != nil {
      return fmt.Errorf("failed to parse recovery point based restore request: %w", err)
    }
    restoreRequest = &req
  case "AzureBackupRecoveryTimeBasedRestoreRequest":
    var req armdataprotection.AzureBackupRecoveryTimeBasedRestoreRequest
    if err := json.Unmarshal(data, &req); err != nil {
      return fmt.Errorf("failed to parse recovery time based restore request: %w", err)
    }
    restoreRequest = &req
  case "AzureBackupRestoreWithRehydrationRequest":
    var req armdataprotection.AzureBackupRestoreWithRehydrationRequest
    if err := json.Unmarshal(data, &req); err != nil {
      return fmt.Errorf("failed to parse restore with rehydration request: %w", err)
    }
    restoreRequest = &req
  default:
    var req armdataprotection.AzureBackupRecoveryPointBasedRestoreRequest
    if err := json.Unmarshal(data, &req); err != nil {
      return fmt.Errorf("failed to parse restore request (defaulting to recovery-point-based): %w", err)
    }
    restoreRequest = &req
  }

  validateRequest := armdataprotection.ValidateRestoreRequestObject{
    RestoreRequestObject: restoreRequest,
  }

  poller, err := client.BeginValidateForRestore(ctx, resourceGroup, vaultName, backupInstanceName, validateRequest, nil)
  if err != nil {
    return fmt.Errorf("failed to validate restore request: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Validate-for-restore operation started. Use 'az dataprotection job list' to monitor progress."}`)
    return nil
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("validate-for-restore operation failed: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}

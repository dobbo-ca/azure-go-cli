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

func newShowCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a recovery point",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      backupInstanceName, _ := cmd.Flags().GetString("backup-instance-name")
      recoveryPointID, _ := cmd.Flags().GetString("recovery-point-id")
      return ShowRecoveryPoint(context.Background(), resourceGroup, vaultName, backupInstanceName, recoveryPointID)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().String("backup-instance-name", "", "Name of the backup instance")
  cmd.Flags().String("recovery-point-id", "", "ID of the recovery point")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("backup-instance-name")
  cmd.MarkFlagRequired("recovery-point-id")
  return cmd
}

func ShowRecoveryPoint(ctx context.Context, resourceGroup, vaultName, backupInstanceName, recoveryPointID string) error {
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

  result, err := client.Get(ctx, resourceGroup, vaultName, backupInstanceName, recoveryPointID, nil)
  if err != nil {
    return fmt.Errorf("failed to get recovery point: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format recovery point: %w", err)
  }

  fmt.Println(string(output))
  return nil
}

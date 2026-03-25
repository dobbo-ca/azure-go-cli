package backuppolicy

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
    Short: "Create a backup policy",
    Long:  "Creates or updates a backup policy within a backup vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      policyFile, _ := cmd.Flags().GetString("policy")
      return CreateBackupPolicy(context.Background(), resourceGroup, vaultName, name, policyFile)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().StringP("name", "n", "", "Name of the backup policy")
  cmd.Flags().String("policy", "", "Path to JSON file containing the backup policy resource")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("name")
  cmd.MarkFlagRequired("policy")
  return cmd
}

func CreateBackupPolicy(ctx context.Context, resourceGroup, vaultName, name, policyFile string) error {
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

  data, err := os.ReadFile(policyFile)
  if err != nil {
    return fmt.Errorf("failed to read policy file %s: %w", policyFile, err)
  }

  var policyResource armdataprotection.BaseBackupPolicyResource
  if err := json.Unmarshal(data, &policyResource); err != nil {
    return fmt.Errorf("failed to parse policy JSON: %w", err)
  }

  result, err := client.CreateOrUpdate(ctx, resourceGroup, vaultName, name, policyResource, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup policy: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}

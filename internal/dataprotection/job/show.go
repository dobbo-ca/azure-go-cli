package job

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newShowCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a backup or restore job",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      jobID, _ := cmd.Flags().GetString("job-id")
      return ShowJob(context.Background(), cmd, resourceGroup, vaultName, jobID)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().String("job-id", "", "ID of the job")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("job-id")
  return cmd
}

func ShowJob(ctx context.Context, cmd *cobra.Command, resourceGroup, vaultName, jobID string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewJobsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create jobs client: %w", err)
  }

  result, err := client.Get(ctx, resourceGroup, vaultName, jobID, nil)
  if err != nil {
    return fmt.Errorf("failed to get job: %w", err)
  }

  return output.PrintJSON(cmd, result)
}

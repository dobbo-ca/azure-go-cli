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

func newListCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "list",
    Short: "List backup and restore jobs in a vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      return ListJobs(context.Background(), cmd, resourceGroup, vaultName)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  return cmd
}

func ListJobs(ctx context.Context, cmd *cobra.Command, resourceGroup, vaultName string) error {
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

  var jobs []*armdataprotection.AzureBackupJobResource
  pager := client.NewListPager(resourceGroup, vaultName, nil)
  for pager.More() {
    page, err := pager.NextPage(ctx)
    if err != nil {
      return fmt.Errorf("failed to list jobs: %w", err)
    }
    jobs = append(jobs, page.Value...)
  }

  return output.PrintJSON(cmd, jobs)
}

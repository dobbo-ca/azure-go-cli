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

func newListCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "list",
    Short: "List backup vaults",
    Long:  "Lists backup vaults in a resource group or subscription",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return ListBackupVaults(context.Background(), resourceGroup)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group (optional; lists all in subscription if omitted)")
  return cmd
}

func ListBackupVaults(ctx context.Context, resourceGroup string) error {
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

  var vaults []*armdataprotection.BackupVaultResource

  if resourceGroup != "" {
    pager := client.NewGetInResourceGroupPager(resourceGroup, nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to list backup vaults: %w", err)
      }
      vaults = append(vaults, page.Value...)
    }
  } else {
    pager := client.NewGetInSubscriptionPager(nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to list backup vaults: %w", err)
      }
      vaults = append(vaults, page.Value...)
    }
  }

  output, err := json.MarshalIndent(vaults, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}

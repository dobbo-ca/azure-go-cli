package account

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/storage/account/blobservice"
	"github.com/cdobbyn/azure-go-cli/internal/storage/account/encryptionscope"
	"github.com/cdobbyn/azure-go-cli/internal/storage/account/keys"
	"github.com/cdobbyn/azure-go-cli/internal/storage/account/managementpolicy"
	"github.com/cdobbyn/azure-go-cli/internal/storage/account/networkrule"
	"github.com/spf13/cobra"
)

func NewAccountCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage Azure storage accounts",
		Long:  "Commands to manage Azure storage accounts",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List storage accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), resourceGroup)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			accountName, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), accountName, resourceGroup)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Storage account name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			sku, _ := cmd.Flags().GetString("sku")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Create(context.Background(), cmd, name, resourceGroup, location, sku, tags)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Storage account name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	createCmd.Flags().String("sku", "Standard_LRS", "SKU (Standard_LRS, Standard_GRS, Standard_RAGRS, Standard_ZRS, Premium_LRS, Premium_ZRS, Standard_GZRS, Standard_RAGZRS)")
	createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("location")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Delete(context.Background(), name, resourceGroup)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Storage account name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("resource-group")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			sku, _ := cmd.Flags().GetString("sku")
			accessTier, _ := cmd.Flags().GetString("access-tier")
			tags, _ := cmd.Flags().GetStringToString("tags")
			var allowBlobPublicAccess *bool
			if cmd.Flags().Changed("allow-blob-public-access") {
				v, _ := cmd.Flags().GetBool("allow-blob-public-access")
				allowBlobPublicAccess = &v
			}
			return Update(context.Background(), cmd, resourceGroup, name, sku, accessTier, tags, allowBlobPublicAccess)
		},
	}
	updateCmd.Flags().StringP("name", "n", "", "Storage account name")
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().String("sku", "", "SKU (Standard_LRS, Standard_GRS, Standard_RAGRS, Standard_ZRS, Premium_LRS, Premium_ZRS, Standard_GZRS, Standard_RAGZRS)")
	updateCmd.Flags().String("access-tier", "", "Access tier (Hot, Cool, Cold, Premium)")
	updateCmd.Flags().StringToString("tags", nil, "Tags: key1=value1 key2=value2")
	updateCmd.Flags().Bool("allow-blob-public-access", false, "Allow or disallow public access to all blobs or containers")
	updateCmd.MarkFlagRequired("name")
	updateCmd.MarkFlagRequired("resource-group")

	showConnStrCmd := &cobra.Command{
		Use:   "show-connection-string",
		Short: "Show the connection string for a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return ShowConnectionString(context.Background(), cmd, resourceGroup, name)
		},
	}
	showConnStrCmd.Flags().StringP("name", "n", "", "Storage account name")
	showConnStrCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showConnStrCmd.MarkFlagRequired("name")
	showConnStrCmd.MarkFlagRequired("resource-group")

	failoverCmd := &cobra.Command{
		Use:   "failover",
		Short: "Failover a storage account to its secondary region",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Failover(context.Background(), cmd, resourceGroup, name, noWait)
		},
	}
	failoverCmd.Flags().StringP("name", "n", "", "Storage account name")
	failoverCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	failoverCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	failoverCmd.MarkFlagRequired("name")
	failoverCmd.MarkFlagRequired("resource-group")

	checkNameCmd := &cobra.Command{
		Use:   "check-name",
		Short: "Check whether a storage account name is available",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			return CheckName(context.Background(), cmd, name)
		},
	}
	checkNameCmd.Flags().StringP("name", "n", "", "Storage account name to check")
	checkNameCmd.MarkFlagRequired("name")

	showUsageCmd := &cobra.Command{
		Use:   "show-usage",
		Short: "Show the storage account usage for a location",
		RunE: func(cmd *cobra.Command, args []string) error {
			location, _ := cmd.Flags().GetString("location")
			return ShowUsage(context.Background(), cmd, location)
		},
	}
	showUsageCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	showUsageCmd.MarkFlagRequired("location")

	cmd.AddCommand(
		listCmd, showCmd, createCmd, deleteCmd,
		updateCmd, showConnStrCmd, failoverCmd, checkNameCmd, showUsageCmd,
		keys.NewKeysCommand(),
		networkrule.NewNetworkRuleCommand(),
		managementpolicy.NewManagementPolicyCommand(),
		encryptionscope.NewEncryptionScopeCommand(),
		blobservice.NewBlobServicePropertiesCommand(),
	)
	return cmd
}

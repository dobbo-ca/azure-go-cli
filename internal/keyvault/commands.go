package keyvault

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/keyvault/certificate"
	"github.com/cdobbyn/azure-go-cli/internal/keyvault/key"
	"github.com/cdobbyn/azure-go-cli/internal/keyvault/secret"
	"github.com/spf13/cobra"
)

func NewKeyVaultCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keyvault",
		Short: "Manage Azure Key Vault",
		Long:  "Commands to manage Azure Key Vault instances",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List key vaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, resourceGroup)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, vaultName, resourceGroup)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Key vault name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Create(context.Background(), cmd, name, resourceGroup, location, tags)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Key vault name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("location")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Delete(context.Background(), name, resourceGroup)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Key vault name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("resource-group")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Update(context.Background(), cmd, name, resourceGroup, tags)
		},
	}
	updateCmd.Flags().StringP("name", "n", "", "Key vault name")
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	updateCmd.Flags().Bool("enable-rbac-authorization", false, "Enable Azure RBAC for data-plane authorization")
	updateCmd.Flags().Bool("enabled-for-deployment", false, "Allow VMs to retrieve certificates as secrets")
	updateCmd.Flags().Bool("enabled-for-disk-encryption", false, "Allow Azure Disk Encryption to retrieve secrets and unwrap keys")
	updateCmd.Flags().Bool("enabled-for-template-deployment", false, "Allow Resource Manager to retrieve secrets")
	updateCmd.MarkFlagRequired("name")
	updateCmd.MarkFlagRequired("resource-group")

	listDeletedCmd := &cobra.Command{
		Use:   "list-deleted",
		Short: "List soft-deleted key vaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ListDeleted(context.Background(), cmd)
		},
	}

	showDeletedCmd := &cobra.Command{
		Use:   "show-deleted",
		Short: "Show a soft-deleted key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			location, _ := cmd.Flags().GetString("location")
			return ShowDeleted(context.Background(), cmd, name, location)
		},
	}
	showDeletedCmd.Flags().StringP("name", "n", "", "Key vault name")
	showDeletedCmd.Flags().StringP("location", "l", "", "Location of the deleted vault")
	showDeletedCmd.MarkFlagRequired("name")
	showDeletedCmd.MarkFlagRequired("location")

	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Permanently delete a soft-deleted key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			location, _ := cmd.Flags().GetString("location")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Purge(context.Background(), cmd, name, location, noWait)
		},
	}
	purgeCmd.Flags().StringP("name", "n", "", "Key vault name")
	purgeCmd.Flags().StringP("location", "l", "", "Location of the deleted vault")
	purgeCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	purgeCmd.MarkFlagRequired("name")
	purgeCmd.MarkFlagRequired("location")

	recoverCmd := &cobra.Command{
		Use:   "recover",
		Short: "Recover a soft-deleted key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Recover(context.Background(), cmd, name, resourceGroup, location, noWait)
		},
	}
	recoverCmd.Flags().StringP("name", "n", "", "Key vault name")
	recoverCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	recoverCmd.Flags().StringP("location", "l", "", "Location of the deleted vault")
	recoverCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	recoverCmd.MarkFlagRequired("name")
	recoverCmd.MarkFlagRequired("resource-group")
	recoverCmd.MarkFlagRequired("location")

	checkNameCmd := &cobra.Command{
		Use:   "check-name",
		Short: "Check whether a key vault name is available",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			return CheckName(context.Background(), cmd, name)
		},
	}
	checkNameCmd.Flags().StringP("name", "n", "", "Key vault name to check")
	checkNameCmd.MarkFlagRequired("name")

	cmd.AddCommand(
		listCmd,
		showCmd,
		createCmd,
		deleteCmd,
		updateCmd,
		listDeletedCmd,
		showDeletedCmd,
		purgeCmd,
		recoverCmd,
		checkNameCmd,
		key.NewKeyCommand(),
		certificate.NewCertificateCommand(),
		secret.NewSecretCommand(),
	)
	return cmd
}

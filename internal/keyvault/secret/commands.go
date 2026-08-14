package secret

import (
	"context"

	"github.com/spf13/cobra"
)

func NewSecretCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage Key Vault secrets",
		Long:  "Commands to manage secrets in Azure Key Vault",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List secrets in a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			return List(context.Background(), cmd, vaultName)
		},
	}
	listCmd.Flags().String("vault-name", "", "Key vault name")
	listCmd.MarkFlagRequired("vault-name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a secret from a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			showValue, _ := cmd.Flags().GetBool("show-value")
			return Show(context.Background(), cmd, vaultName, name, showValue)
		},
	}
	showCmd.Flags().String("vault-name", "", "Key vault name")
	showCmd.Flags().StringP("name", "n", "", "Secret name")
	showCmd.Flags().Bool("show-value", false, "Show the secret value (WARNING: displays sensitive data)")
	showCmd.MarkFlagRequired("vault-name")
	showCmd.MarkFlagRequired("name")

	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Set a secret in a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			value, _ := cmd.Flags().GetString("value")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Set(context.Background(), cmd, vaultName, name, value, tags)
		},
	}
	setCmd.Flags().String("vault-name", "", "Key vault name")
	setCmd.Flags().StringP("name", "n", "", "Secret name")
	setCmd.Flags().String("value", "", "Secret value")
	setCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	setCmd.MarkFlagRequired("vault-name")
	setCmd.MarkFlagRequired("name")
	setCmd.MarkFlagRequired("value")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a secret from a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return Delete(context.Background(), vaultName, name)
		},
	}
	deleteCmd.Flags().String("vault-name", "", "Key vault name")
	deleteCmd.Flags().StringP("name", "n", "", "Secret name")
	deleteCmd.MarkFlagRequired("vault-name")
	deleteCmd.MarkFlagRequired("name")

	listVersionsCmd := &cobra.Command{
		Use:   "list-versions",
		Short: "List versions of a secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return ListVersions(context.Background(), cmd, vaultName, name)
		},
	}
	listVersionsCmd.Flags().String("vault-name", "", "Key vault name")
	listVersionsCmd.Flags().StringP("name", "n", "", "Secret name")
	listVersionsCmd.MarkFlagRequired("vault-name")
	listVersionsCmd.MarkFlagRequired("name")

	listDeletedCmd := &cobra.Command{
		Use:   "list-deleted",
		Short: "List deleted secrets in a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			return ListDeleted(context.Background(), cmd, vaultName)
		},
	}
	listDeletedCmd.Flags().String("vault-name", "", "Key vault name")
	listDeletedCmd.MarkFlagRequired("vault-name")

	showDeletedCmd := &cobra.Command{
		Use:   "show-deleted",
		Short: "Show a deleted secret from a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return ShowDeleted(context.Background(), cmd, vaultName, name)
		},
	}
	showDeletedCmd.Flags().String("vault-name", "", "Key vault name")
	showDeletedCmd.Flags().StringP("name", "n", "", "Secret name")
	showDeletedCmd.MarkFlagRequired("vault-name")
	showDeletedCmd.MarkFlagRequired("name")

	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Permanently purge a deleted secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return Purge(context.Background(), cmd, vaultName, name)
		},
	}
	purgeCmd.Flags().String("vault-name", "", "Key vault name")
	purgeCmd.Flags().StringP("name", "n", "", "Secret name")
	purgeCmd.MarkFlagRequired("vault-name")
	purgeCmd.MarkFlagRequired("name")

	recoverCmd := &cobra.Command{
		Use:   "recover",
		Short: "Recover a deleted secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return Recover(context.Background(), cmd, vaultName, name)
		},
	}
	recoverCmd.Flags().String("vault-name", "", "Key vault name")
	recoverCmd.Flags().StringP("name", "n", "", "Secret name")
	recoverCmd.MarkFlagRequired("vault-name")
	recoverCmd.MarkFlagRequired("name")

	setAttributesCmd := &cobra.Command{
		Use:   "set-attributes",
		Short: "Update the attributes of a secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			version, _ := cmd.Flags().GetString("version")
			return SetAttributes(context.Background(), cmd, vaultName, name, version)
		},
	}
	setAttributesCmd.Flags().String("vault-name", "", "Key vault name")
	setAttributesCmd.Flags().StringP("name", "n", "", "Secret name")
	setAttributesCmd.Flags().String("version", "", "Secret version")
	setAttributesCmd.Flags().Bool("enabled", false, "Whether the secret is enabled")
	setAttributesCmd.MarkFlagRequired("vault-name")
	setAttributesCmd.MarkFlagRequired("name")

	backupCmd := &cobra.Command{
		Use:   "backup",
		Short: "Back up a secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			file, _ := cmd.Flags().GetString("file")
			return Backup(context.Background(), cmd, vaultName, name, file)
		},
	}
	backupCmd.Flags().String("vault-name", "", "Key vault name")
	backupCmd.Flags().StringP("name", "n", "", "Secret name")
	backupCmd.Flags().String("file", "", "File to write the backup blob to")
	backupCmd.MarkFlagRequired("vault-name")
	backupCmd.MarkFlagRequired("name")

	restoreCmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a secret from a backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			file, _ := cmd.Flags().GetString("file")
			return Restore(context.Background(), cmd, vaultName, file)
		},
	}
	restoreCmd.Flags().String("vault-name", "", "Key vault name")
	restoreCmd.Flags().String("file", "", "Backup file to restore from")
	restoreCmd.MarkFlagRequired("vault-name")
	restoreCmd.MarkFlagRequired("file")

	downloadCmd := &cobra.Command{
		Use:   "download",
		Short: "Download a secret value to a file",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			file, _ := cmd.Flags().GetString("file")
			return Download(context.Background(), cmd, vaultName, name, file)
		},
	}
	downloadCmd.Flags().String("vault-name", "", "Key vault name")
	downloadCmd.Flags().StringP("name", "n", "", "Secret name")
	downloadCmd.Flags().String("file", "", "File to write the secret value to")
	downloadCmd.MarkFlagRequired("vault-name")
	downloadCmd.MarkFlagRequired("name")
	downloadCmd.MarkFlagRequired("file")

	cmd.AddCommand(listCmd, showCmd, setCmd, deleteCmd)
	cmd.AddCommand(listVersionsCmd, listDeletedCmd, showDeletedCmd, purgeCmd, recoverCmd, setAttributesCmd, backupCmd, restoreCmd, downloadCmd)
	return cmd
}

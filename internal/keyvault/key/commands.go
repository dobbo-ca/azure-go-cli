package key

import (
	"context"

	"github.com/spf13/cobra"
)

func NewKeyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage Key Vault keys",
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a key in a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			kty, _ := cmd.Flags().GetString("kty")
			curve, _ := cmd.Flags().GetString("curve")
			size, _ := cmd.Flags().GetInt32("size")
			return Create(context.Background(), cmd, vaultName, name, kty, curve, size)
		},
	}
	createCmd.Flags().String("vault-name", "", "Key vault name")
	createCmd.Flags().StringP("name", "n", "", "Key name")
	createCmd.Flags().String("kty", "RSA", "Key type")
	createCmd.Flags().String("curve", "", "Elliptic curve name")
	createCmd.Flags().Int32("size", 0, "Key size in bits")
	createCmd.MarkFlagRequired("vault-name")
	createCmd.MarkFlagRequired("name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a key from a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			version, _ := cmd.Flags().GetString("version")
			return Show(context.Background(), cmd, vaultName, name, version)
		},
	}
	showCmd.Flags().String("vault-name", "", "Key vault name")
	showCmd.Flags().StringP("name", "n", "", "Key name")
	showCmd.Flags().String("version", "", "Key version")
	showCmd.MarkFlagRequired("vault-name")
	showCmd.MarkFlagRequired("name")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List keys in a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			return List(context.Background(), cmd, vaultName)
		},
	}
	listCmd.Flags().String("vault-name", "", "Key vault name")
	listCmd.MarkFlagRequired("vault-name")

	listVersionsCmd := &cobra.Command{
		Use:   "list-versions",
		Short: "List versions of a key",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return ListVersions(context.Background(), cmd, vaultName, name)
		},
	}
	listVersionsCmd.Flags().String("vault-name", "", "Key vault name")
	listVersionsCmd.Flags().StringP("name", "n", "", "Key name")
	listVersionsCmd.MarkFlagRequired("vault-name")
	listVersionsCmd.MarkFlagRequired("name")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a key from a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return Delete(context.Background(), cmd, vaultName, name)
		},
	}
	deleteCmd.Flags().String("vault-name", "", "Key vault name")
	deleteCmd.Flags().StringP("name", "n", "", "Key name")
	deleteCmd.MarkFlagRequired("vault-name")
	deleteCmd.MarkFlagRequired("name")

	listDeletedCmd := &cobra.Command{
		Use:   "list-deleted",
		Short: "List deleted keys in a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			return ListDeleted(context.Background(), cmd, vaultName)
		},
	}
	listDeletedCmd.Flags().String("vault-name", "", "Key vault name")
	listDeletedCmd.MarkFlagRequired("vault-name")

	showDeletedCmd := &cobra.Command{
		Use:   "show-deleted",
		Short: "Show a deleted key from a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return ShowDeleted(context.Background(), cmd, vaultName, name)
		},
	}
	showDeletedCmd.Flags().String("vault-name", "", "Key vault name")
	showDeletedCmd.Flags().StringP("name", "n", "", "Key name")
	showDeletedCmd.MarkFlagRequired("vault-name")
	showDeletedCmd.MarkFlagRequired("name")

	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Permanently purge a deleted key",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return Purge(context.Background(), cmd, vaultName, name)
		},
	}
	purgeCmd.Flags().String("vault-name", "", "Key vault name")
	purgeCmd.Flags().StringP("name", "n", "", "Key name")
	purgeCmd.MarkFlagRequired("vault-name")
	purgeCmd.MarkFlagRequired("name")

	recoverCmd := &cobra.Command{
		Use:   "recover",
		Short: "Recover a deleted key",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return Recover(context.Background(), cmd, vaultName, name)
		},
	}
	recoverCmd.Flags().String("vault-name", "", "Key vault name")
	recoverCmd.Flags().StringP("name", "n", "", "Key name")
	recoverCmd.MarkFlagRequired("vault-name")
	recoverCmd.MarkFlagRequired("name")

	setAttributesCmd := &cobra.Command{
		Use:   "set-attributes",
		Short: "Update attributes of a key",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			version, _ := cmd.Flags().GetString("version")
			return SetAttributes(context.Background(), cmd, vaultName, name, version)
		},
	}
	setAttributesCmd.Flags().String("vault-name", "", "Key vault name")
	setAttributesCmd.Flags().StringP("name", "n", "", "Key name")
	setAttributesCmd.Flags().String("version", "", "Key version")
	setAttributesCmd.Flags().Bool("enabled", false, "Whether the key is enabled")
	setAttributesCmd.MarkFlagRequired("vault-name")
	setAttributesCmd.MarkFlagRequired("name")

	backupCmd := &cobra.Command{
		Use:   "backup",
		Short: "Back up a key",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			file, _ := cmd.Flags().GetString("file")
			return Backup(context.Background(), cmd, vaultName, name, file)
		},
	}
	backupCmd.Flags().String("vault-name", "", "Key vault name")
	backupCmd.Flags().StringP("name", "n", "", "Key name")
	backupCmd.Flags().String("file", "", "File to write the backup blob to")
	backupCmd.MarkFlagRequired("vault-name")
	backupCmd.MarkFlagRequired("name")

	restoreCmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a key from a backup",
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

	rotateCmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate a key",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return Rotate(context.Background(), cmd, vaultName, name)
		},
	}
	rotateCmd.Flags().String("vault-name", "", "Key vault name")
	rotateCmd.Flags().StringP("name", "n", "", "Key name")
	rotateCmd.MarkFlagRequired("vault-name")
	rotateCmd.MarkFlagRequired("name")

	cmd.AddCommand(
		createCmd,
		showCmd,
		listCmd,
		listVersionsCmd,
		deleteCmd,
		listDeletedCmd,
		showDeletedCmd,
		purgeCmd,
		recoverCmd,
		setAttributesCmd,
		backupCmd,
		restoreCmd,
		rotateCmd,
	)
	return cmd
}

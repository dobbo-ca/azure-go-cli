package certificate

import (
	"context"

	"github.com/spf13/cobra"
)

func NewCertificateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "certificate",
		Short: "Manage Key Vault certificates",
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			version, _ := cmd.Flags().GetString("version")
			return Show(context.Background(), cmd, vaultName, name, version)
		},
	}
	showCmd.Flags().String("vault-name", "", "Key vault name")
	showCmd.Flags().StringP("name", "n", "", "Certificate name")
	showCmd.Flags().String("version", "", "Certificate version")
	showCmd.MarkFlagRequired("vault-name")
	showCmd.MarkFlagRequired("name")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List certificates in a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			return List(context.Background(), cmd, vaultName)
		},
	}
	listCmd.Flags().String("vault-name", "", "Key vault name")
	listCmd.MarkFlagRequired("vault-name")

	listVersionsCmd := &cobra.Command{
		Use:   "list-versions",
		Short: "List versions of a certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return ListVersions(context.Background(), cmd, vaultName, name)
		},
	}
	listVersionsCmd.Flags().String("vault-name", "", "Key vault name")
	listVersionsCmd.Flags().StringP("name", "n", "", "Certificate name")
	listVersionsCmd.MarkFlagRequired("vault-name")
	listVersionsCmd.MarkFlagRequired("name")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return Delete(context.Background(), cmd, vaultName, name)
		},
	}
	deleteCmd.Flags().String("vault-name", "", "Key vault name")
	deleteCmd.Flags().StringP("name", "n", "", "Certificate name")
	deleteCmd.MarkFlagRequired("vault-name")
	deleteCmd.MarkFlagRequired("name")

	listDeletedCmd := &cobra.Command{
		Use:   "list-deleted",
		Short: "List deleted certificates in a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			return ListDeleted(context.Background(), cmd, vaultName)
		},
	}
	listDeletedCmd.Flags().String("vault-name", "", "Key vault name")
	listDeletedCmd.MarkFlagRequired("vault-name")

	showDeletedCmd := &cobra.Command{
		Use:   "show-deleted",
		Short: "Show a deleted certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return ShowDeleted(context.Background(), cmd, vaultName, name)
		},
	}
	showDeletedCmd.Flags().String("vault-name", "", "Key vault name")
	showDeletedCmd.Flags().StringP("name", "n", "", "Certificate name")
	showDeletedCmd.MarkFlagRequired("vault-name")
	showDeletedCmd.MarkFlagRequired("name")

	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Permanently purge a deleted certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return Purge(context.Background(), cmd, vaultName, name)
		},
	}
	purgeCmd.Flags().String("vault-name", "", "Key vault name")
	purgeCmd.Flags().StringP("name", "n", "", "Certificate name")
	purgeCmd.MarkFlagRequired("vault-name")
	purgeCmd.MarkFlagRequired("name")

	recoverCmd := &cobra.Command{
		Use:   "recover",
		Short: "Recover a deleted certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return Recover(context.Background(), cmd, vaultName, name)
		},
	}
	recoverCmd.Flags().String("vault-name", "", "Key vault name")
	recoverCmd.Flags().StringP("name", "n", "", "Certificate name")
	recoverCmd.MarkFlagRequired("vault-name")
	recoverCmd.MarkFlagRequired("name")

	setAttributesCmd := &cobra.Command{
		Use:   "set-attributes",
		Short: "Update certificate attributes",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			version, _ := cmd.Flags().GetString("version")
			return SetAttributes(context.Background(), cmd, vaultName, name, version)
		},
	}
	setAttributesCmd.Flags().String("vault-name", "", "Key vault name")
	setAttributesCmd.Flags().StringP("name", "n", "", "Certificate name")
	setAttributesCmd.Flags().String("version", "", "Certificate version")
	setAttributesCmd.Flags().Bool("enabled", false, "Whether the certificate is enabled")
	setAttributesCmd.MarkFlagRequired("vault-name")
	setAttributesCmd.MarkFlagRequired("name")

	backupCmd := &cobra.Command{
		Use:   "backup",
		Short: "Back up a certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			file, _ := cmd.Flags().GetString("file")
			return Backup(context.Background(), cmd, vaultName, name, file)
		},
	}
	backupCmd.Flags().String("vault-name", "", "Key vault name")
	backupCmd.Flags().StringP("name", "n", "", "Certificate name")
	backupCmd.Flags().String("file", "", "File to write the backup blob to")
	backupCmd.MarkFlagRequired("vault-name")
	backupCmd.MarkFlagRequired("name")

	restoreCmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a certificate from a backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			file, _ := cmd.Flags().GetString("file")
			return Restore(context.Background(), cmd, vaultName, file)
		},
	}
	restoreCmd.Flags().String("vault-name", "", "Key vault name")
	restoreCmd.Flags().String("file", "", "File containing the backup blob")
	restoreCmd.MarkFlagRequired("vault-name")
	restoreCmd.MarkFlagRequired("file")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Key Vault certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			policy, _ := cmd.Flags().GetString("policy")
			validity, _ := cmd.Flags().GetInt32("validity")
			disabled, _ := cmd.Flags().GetBool("disabled")
			tags, _ := cmd.Flags().GetStringSlice("tags")
			return Create(context.Background(), cmd, vaultName, name, policy, validity, disabled, tags)
		},
	}
	createCmd.Flags().String("vault-name", "", "Key vault name")
	createCmd.Flags().StringP("name", "n", "", "Name of the certificate")
	createCmd.Flags().StringP("policy", "p", "", "JSON encoded policy definition. Use @{file} to load from a file (e.g. @my_policy.json)")
	createCmd.Flags().Int32("validity", 0, "Number of months the certificate is valid for. Overrides the value specified with --policy/-p")
	createCmd.Flags().Bool("disabled", false, "Create certificate in disabled state")
	createCmd.Flags().StringSlice("tags", nil, "Resource tags as key=value pairs")
	createCmd.MarkFlagRequired("vault-name")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("policy")

	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import a certificate into a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			file, _ := cmd.Flags().GetString("file")
			password, _ := cmd.Flags().GetString("password")
			policy, _ := cmd.Flags().GetString("policy")
			disabled, _ := cmd.Flags().GetBool("disabled")
			tags, _ := cmd.Flags().GetStringSlice("tags")
			return Import(context.Background(), cmd, vaultName, name, file, password, policy, disabled, tags)
		},
	}
	importCmd.Flags().String("vault-name", "", "Key vault name")
	importCmd.Flags().StringP("name", "n", "", "Name of the certificate")
	importCmd.Flags().StringP("file", "f", "", "PKCS12 file or PEM file containing the certificate and private key")
	importCmd.Flags().String("password", "", "If the private key in the certificate is encrypted, the password used for encryption")
	importCmd.Flags().StringP("policy", "p", "", "JSON encoded policy definition. Use @{file} to load from a file (e.g. @my_policy.json)")
	importCmd.Flags().Bool("disabled", false, "Import the certificate in disabled state")
	importCmd.Flags().StringSlice("tags", nil, "Resource tags as key=value pairs")
	importCmd.MarkFlagRequired("vault-name")
	importCmd.MarkFlagRequired("name")
	importCmd.MarkFlagRequired("file")

	downloadCmd := &cobra.Command{
		Use:   "download",
		Short: "Download the public portion of a Key Vault certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			version, _ := cmd.Flags().GetString("version")
			file, _ := cmd.Flags().GetString("file")
			encoding, _ := cmd.Flags().GetString("encoding")
			return Download(context.Background(), cmd, vaultName, name, version, file, encoding)
		},
	}
	downloadCmd.Flags().String("vault-name", "", "Key vault name")
	downloadCmd.Flags().StringP("name", "n", "", "Name of the certificate")
	downloadCmd.Flags().String("version", "", "The certificate version. If omitted, uses the latest version")
	downloadCmd.Flags().StringP("file", "f", "", "File to receive the binary certificate contents")
	downloadCmd.Flags().StringP("encoding", "e", "PEM", "Encoding of the certificate: PEM, DER")
	downloadCmd.MarkFlagRequired("vault-name")
	downloadCmd.MarkFlagRequired("name")
	downloadCmd.MarkFlagRequired("file")

	getDefaultPolicyCmd := &cobra.Command{
		Use:   "get-default-policy",
		Short: "Get the default policy for self-signed certificates",
		RunE: func(cmd *cobra.Command, args []string) error {
			scaffold, _ := cmd.Flags().GetBool("scaffold")
			return GetDefaultPolicy(cmd, scaffold)
		},
	}
	getDefaultPolicyCmd.Flags().Bool("scaffold", false, "Create a fully formed policy structure with default values")

	pendingCmd := &cobra.Command{
		Use:   "pending",
		Short: "Manage pending certificate creation operations",
	}

	pendingShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Get a pending certificate operation",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return PendingShow(context.Background(), cmd, vaultName, name)
		},
	}
	pendingShowCmd.Flags().String("vault-name", "", "Key vault name")
	pendingShowCmd.Flags().StringP("name", "n", "", "Name of the pending certificate")
	pendingShowCmd.MarkFlagRequired("vault-name")
	pendingShowCmd.MarkFlagRequired("name")

	pendingDeleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a pending certificate operation",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return PendingDelete(context.Background(), cmd, vaultName, name)
		},
	}
	pendingDeleteCmd.Flags().String("vault-name", "", "Key vault name")
	pendingDeleteCmd.Flags().StringP("name", "n", "", "Name of the pending certificate")
	pendingDeleteCmd.MarkFlagRequired("vault-name")
	pendingDeleteCmd.MarkFlagRequired("name")

	pendingMergeCmd := &cobra.Command{
		Use:   "merge",
		Short: "Merge a certificate or certificate chain with a pending certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			file, _ := cmd.Flags().GetString("file")
			disabled, _ := cmd.Flags().GetBool("disabled")
			tags, _ := cmd.Flags().GetStringSlice("tags")
			return PendingMerge(context.Background(), cmd, vaultName, name, file, disabled, tags)
		},
	}
	pendingMergeCmd.Flags().String("vault-name", "", "Key vault name")
	pendingMergeCmd.Flags().StringP("name", "n", "", "Name of the pending certificate")
	pendingMergeCmd.Flags().StringP("file", "f", "", "File containing the certificate or certificate chain to merge")
	pendingMergeCmd.Flags().Bool("disabled", false, "Create certificate in disabled state")
	pendingMergeCmd.Flags().StringSlice("tags", nil, "Resource tags as key=value pairs")
	pendingMergeCmd.MarkFlagRequired("vault-name")
	pendingMergeCmd.MarkFlagRequired("name")
	pendingMergeCmd.MarkFlagRequired("file")

	pendingCmd.AddCommand(pendingShowCmd, pendingDeleteCmd, pendingMergeCmd)

	cmd.AddCommand(
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
		createCmd,
		importCmd,
		downloadCmd,
		getDefaultPolicyCmd,
		pendingCmd,
	)

	return cmd
}

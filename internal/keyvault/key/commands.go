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
			opts := CreateOptions{}
			opts.VaultName, _ = cmd.Flags().GetString("vault-name")
			opts.Name, _ = cmd.Flags().GetString("name")
			opts.Kty, _ = cmd.Flags().GetString("kty")
			opts.Curve, _ = cmd.Flags().GetString("curve")
			opts.Size, _ = cmd.Flags().GetInt32("size")
			opts.Ops, _ = cmd.Flags().GetStringSlice("ops")
			opts.Expires, _ = cmd.Flags().GetString("expires")
			opts.NotBefore, _ = cmd.Flags().GetString("not-before")
			opts.Tags, _ = cmd.Flags().GetStringSlice("tags")
			opts.Disabled, _ = cmd.Flags().GetBool("disabled")
			return Create(context.Background(), cmd, opts)
		},
	}
	createCmd.Flags().String("vault-name", "", "Key vault name")
	createCmd.Flags().StringP("name", "n", "", "Key name")
	createCmd.Flags().String("kty", "RSA", "Key type")
	createCmd.Flags().String("curve", "", "Elliptic curve name")
	createCmd.Flags().Int32("size", 0, "Key size in bits")
	createCmd.Flags().StringSlice("ops", nil, "List of permitted JSON web key operations: decrypt, encrypt, import, sign, unwrapKey, verify, wrapKey")
	createCmd.Flags().String("expires", "", "Expiration UTC datetime (Y-m-d'T'H:M:S'Z')")
	createCmd.Flags().String("not-before", "", "Key not usable before the provided UTC datetime (Y-m-d'T'H:M:S'Z')")
	createCmd.Flags().StringSlice("tags", nil, "Resource tags as key=value pairs")
	createCmd.Flags().Bool("disabled", false, "Create key in disabled state")
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

	rotationPolicyCmd := &cobra.Command{
		Use:   "rotation-policy",
		Short: "Manage key rotation policy",
	}

	rotationPolicyShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Get the rotation policy of a Key Vault key",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			return ShowRotationPolicy(context.Background(), cmd, vaultName, name)
		},
	}
	rotationPolicyShowCmd.Flags().String("vault-name", "", "Key vault name")
	rotationPolicyShowCmd.Flags().StringP("name", "n", "", "Key name")
	rotationPolicyShowCmd.MarkFlagRequired("vault-name")
	rotationPolicyShowCmd.MarkFlagRequired("name")

	rotationPolicyUpdateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update the rotation policy of a Key Vault key",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			name, _ := cmd.Flags().GetString("name")
			value, _ := cmd.Flags().GetString("value")
			return UpdateRotationPolicy(context.Background(), cmd, vaultName, name, value)
		},
	}
	rotationPolicyUpdateCmd.Flags().String("vault-name", "", "Key vault name")
	rotationPolicyUpdateCmd.Flags().StringP("name", "n", "", "Key name")
	rotationPolicyUpdateCmd.Flags().String("value", "", "The rotation policy file definition as JSON, or a path to a file containing JSON policy definition")
	rotationPolicyUpdateCmd.MarkFlagRequired("vault-name")
	rotationPolicyUpdateCmd.MarkFlagRequired("name")
	rotationPolicyUpdateCmd.MarkFlagRequired("value")

	rotationPolicyCmd.AddCommand(rotationPolicyShowCmd, rotationPolicyUpdateCmd)

	cryptoFlags := func(c *cobra.Command) {
		c.Flags().String("vault-name", "", "Key vault name")
		c.Flags().StringP("name", "n", "", "Key name")
		c.Flags().StringP("version", "v", "", "The key version. If omitted, uses the latest version")
		c.Flags().StringP("algorithm", "a", "", "Algorithm identifier")
		c.MarkFlagRequired("vault-name")
		c.MarkFlagRequired("name")
		c.MarkFlagRequired("algorithm")
	}
	cryptoOpts := func(cmd *cobra.Command) CryptoOptions {
		opts := CryptoOptions{}
		opts.VaultName, _ = cmd.Flags().GetString("vault-name")
		opts.Name, _ = cmd.Flags().GetString("name")
		opts.Version, _ = cmd.Flags().GetString("version")
		opts.Algorithm, _ = cmd.Flags().GetString("algorithm")
		opts.Value, _ = cmd.Flags().GetString("value")
		opts.DataType, _ = cmd.Flags().GetString("data-type")
		opts.IV, _ = cmd.Flags().GetString("iv")
		opts.AAD, _ = cmd.Flags().GetString("aad")
		opts.Tag, _ = cmd.Flags().GetString("tag")
		opts.Digest, _ = cmd.Flags().GetString("digest")
		opts.Signature, _ = cmd.Flags().GetString("signature")
		return opts
	}

	encryptCmd := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt an arbitrary sequence of bytes using an encryption key stored in a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Encrypt(context.Background(), cmd, cryptoOpts(cmd))
		},
	}
	cryptoFlags(encryptCmd)
	encryptCmd.Flags().String("value", "", "The value to be encrypted. Default data type is Base64 encoded string")
	encryptCmd.Flags().String("data-type", "base64", "The type of the original data: base64, plaintext")
	encryptCmd.Flags().String("iv", "", "Initialization vector. Required for only AES-CBC(PAD) encryption")
	encryptCmd.Flags().String("aad", "", "Optional data that is authenticated but not encrypted. For use with AES-GCM encryption")
	encryptCmd.MarkFlagRequired("value")

	decryptCmd := &cobra.Command{
		Use:   "decrypt",
		Short: "Decrypt a single block of encrypted data",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Decrypt(context.Background(), cmd, cryptoOpts(cmd))
		},
	}
	cryptoFlags(decryptCmd)
	decryptCmd.Flags().String("value", "", "The value to be decrypted, which should be the result of \"az keyvault key encrypt\"")
	decryptCmd.Flags().String("data-type", "base64", "The type of the original data: base64, plaintext")
	decryptCmd.Flags().String("iv", "", "The initialization vector used during encryption. Required for AES decryption")
	decryptCmd.Flags().String("aad", "", "Optional data that is authenticated but not encrypted. For use with AES-GCM decryption")
	decryptCmd.Flags().String("tag", "", "The authentication tag generated during encryption. Required for only AES-GCM decryption")
	decryptCmd.MarkFlagRequired("value")

	signCmd := &cobra.Command{
		Use:   "sign",
		Short: "Create a signature from a digest using a key",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Sign(context.Background(), cmd, cryptoOpts(cmd))
		},
	}
	cryptoFlags(signCmd)
	signCmd.Flags().String("digest", "", "The value to sign (base64 encoded)")
	signCmd.MarkFlagRequired("digest")

	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify a signature against a digest using a key",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Verify(context.Background(), cmd, cryptoOpts(cmd))
		},
	}
	cryptoFlags(verifyCmd)
	verifyCmd.Flags().String("digest", "", "The value to sign (base64 encoded)")
	verifyCmd.Flags().String("signature", "", "signature to verify")
	verifyCmd.MarkFlagRequired("digest")
	verifyCmd.MarkFlagRequired("signature")

	downloadCmd := &cobra.Command{
		Use:   "download",
		Short: "Download the public part of a stored key",
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
	downloadCmd.Flags().StringP("name", "n", "", "Key name")
	downloadCmd.Flags().StringP("version", "v", "", "The key version. If omitted, uses the latest version")
	downloadCmd.Flags().StringP("file", "f", "", "File to receive the key contents")
	downloadCmd.Flags().StringP("encoding", "e", "PEM", "Encoding of the key: PEM, DER")
	downloadCmd.MarkFlagRequired("vault-name")
	downloadCmd.MarkFlagRequired("name")
	downloadCmd.MarkFlagRequired("file")

	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import a private key",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := ImportOptions{}
			opts.VaultName, _ = cmd.Flags().GetString("vault-name")
			opts.Name, _ = cmd.Flags().GetString("name")
			opts.Protection, _ = cmd.Flags().GetString("protection")
			opts.Ops, _ = cmd.Flags().GetStringSlice("ops")
			opts.Disabled, _ = cmd.Flags().GetBool("disabled")
			opts.Expires, _ = cmd.Flags().GetString("expires")
			opts.NotBefore, _ = cmd.Flags().GetString("not-before")
			opts.Tags, _ = cmd.Flags().GetStringSlice("tags")
			opts.PemFile, _ = cmd.Flags().GetString("pem-file")
			opts.PemString, _ = cmd.Flags().GetString("pem-string")
			opts.PemPassword, _ = cmd.Flags().GetString("pem-password")
			opts.ByokFile, _ = cmd.Flags().GetString("byok-file")
			opts.ByokString, _ = cmd.Flags().GetString("byok-string")
			opts.Kty, _ = cmd.Flags().GetString("kty")
			opts.Curve, _ = cmd.Flags().GetString("curve")
			return Import(context.Background(), cmd, opts)
		},
	}
	importCmd.Flags().String("vault-name", "", "Key vault name")
	importCmd.Flags().StringP("name", "n", "", "Key name")
	importCmd.Flags().StringP("protection", "p", "", "Specifies the type of key protection: software, hsm")
	importCmd.Flags().StringSlice("ops", nil, "List of permitted JSON web key operations: decrypt, encrypt, import, sign, unwrapKey, verify, wrapKey")
	importCmd.Flags().Bool("disabled", false, "Create key in disabled state")
	importCmd.Flags().String("expires", "", "Expiration UTC datetime (Y-m-d'T'H:M:S'Z')")
	importCmd.Flags().String("not-before", "", "Key not usable before the provided UTC datetime (Y-m-d'T'H:M:S'Z')")
	importCmd.Flags().StringSlice("tags", nil, "Resource tags as key=value pairs")
	importCmd.Flags().String("pem-file", "", "PEM file containing the key to be imported")
	importCmd.Flags().String("pem-string", "", "PEM string containing the key to be imported")
	importCmd.Flags().String("pem-password", "", "Password of PEM file")
	importCmd.Flags().String("byok-file", "", "BYOK file containing the key to be imported. Must not be password protected")
	importCmd.Flags().String("byok-string", "", "BYOK string containing the key to be imported. Must not be password protected")
	importCmd.Flags().String("kty", "RSA", "The type of key to import (only for BYOK)")
	importCmd.Flags().String("curve", "", "The curve name of the key to import (only for BYOK)")
	importCmd.MarkFlagRequired("vault-name")
	importCmd.MarkFlagRequired("name")

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
		rotationPolicyCmd,
		encryptCmd,
		decryptCmd,
		signCmd,
		verifyCmd,
		downloadCmd,
		importCmd,
	)
	return cmd
}

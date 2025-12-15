package encryptionset

import "github.com/spf13/cobra"

func NewEncryptionSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disk-encryption-set",
		Short: "Manage disk encryption sets",
		Long:  "Commands to manage disk encryption sets in Azure",
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newShowCmd())

	return cmd
}

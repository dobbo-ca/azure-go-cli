package encryptionset

import (
	"github.com/cdobbyn/azure-go-cli/internal/disk/encryptionset/identity"
	"github.com/spf13/cobra"
)

func NewEncryptionSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disk-encryption-set",
		Short: "Manage disk encryption sets",
		Long:  "Commands to manage disk encryption sets in Azure",
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newListAssociatedResourcesCmd())
	cmd.AddCommand(newWaitCmd())
	cmd.AddCommand(identity.NewIdentityCommand())

	return cmd
}

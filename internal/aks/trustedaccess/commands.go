package trustedaccess

import (
	"github.com/spf13/cobra"
)

func NewTrustedAccessCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trustedaccess",
		Short: "Manage trusted access between AKS and other Azure resources",
	}

	cmd.AddCommand(newRoleCmd(), newRoleBindingCmd())
	return cmd
}

package pim

import "github.com/spf13/cobra"

// NewPIMCommand returns the top-level `az pim` command with its subcommands wired.
func NewPIMCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pim",
		Short: "Manage Azure Privileged Identity Management (PIM) assignments",
		Long:  "List and activate eligible PIM role assignments and Entra group memberships.",
	}
	activate := &cobra.Command{
		Use:   "activate",
		Short: "Activate an eligible PIM assignment",
	}
	activate.AddCommand(newActivateResourceCmd(), newActivateGroupCmd())
	cmd.AddCommand(newListCmd(), activate)
	return cmd
}

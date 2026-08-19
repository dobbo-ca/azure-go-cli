package repos

import "github.com/spf13/cobra"

// newRefCommand returns the "az repos ref" command group: create, delete,
// list, lock, unlock. Ported from azext_devops/dev/repos/ref.py.
func newRefCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ref",
		Short: "Manage Git references.",
	}

	cmd.AddCommand(newRefCreateCmd())
	cmd.AddCommand(newRefDeleteCmd())
	cmd.AddCommand(newRefListCmd())
	cmd.AddCommand(newRefLockCmd())
	cmd.AddCommand(newRefUnlockCmd())

	return cmd
}

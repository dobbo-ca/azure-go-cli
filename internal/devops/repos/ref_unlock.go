package repos

import (
	"context"

	"github.com/spf13/cobra"
)

func newRefUnlockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Unlock a reference.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefUpdateLock(context.Background(), cmd, false)
		},
	}

	cmd.Flags().String("name", "", "Name of the reference to update (example: heads/my_branch).")
	refAddFlags(cmd)
	cmd.MarkFlagRequired("name")

	return cmd
}

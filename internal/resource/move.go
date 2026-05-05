package resource

import "github.com/spf13/cobra"

func newMoveCmd() *cobra.Command {
  return &cobra.Command{Use: "move", Short: "Move resources", RunE: notImplemented("move")}
}

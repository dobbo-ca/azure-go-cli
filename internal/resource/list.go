package resource

import "github.com/spf13/cobra"

func newListCmd() *cobra.Command {
  return &cobra.Command{Use: "list", Short: "List resources", RunE: notImplemented("list")}
}

package resource

import "github.com/spf13/cobra"

func newDeleteCmd() *cobra.Command {
  return &cobra.Command{Use: "delete", Short: "Delete a resource", RunE: notImplemented("delete")}
}

package resource

import "github.com/spf13/cobra"

func newCreateCmd() *cobra.Command {
  return &cobra.Command{Use: "create", Short: "Create a resource", RunE: notImplemented("create")}
}

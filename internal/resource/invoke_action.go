package resource

import "github.com/spf13/cobra"

func newInvokeActionCmd() *cobra.Command {
  return &cobra.Command{Use: "invoke-action", Short: "Invoke an action on a resource", RunE: notImplemented("invoke-action")}
}

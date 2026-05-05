package resource

import "github.com/spf13/cobra"

func newWaitCmd() *cobra.Command {
  return &cobra.Command{Use: "wait", Short: "Wait for a resource state", RunE: notImplemented("wait")}
}

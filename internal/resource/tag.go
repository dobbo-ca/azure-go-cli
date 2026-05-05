package resource

import "github.com/spf13/cobra"

func newTagCmd() *cobra.Command {
  return &cobra.Command{Use: "tag", Short: "Tag a resource", RunE: notImplemented("tag")}
}

package job

import "github.com/spf13/cobra"

func NewJobCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "job",
    Short: "Manage backup and restore jobs",
    Long:  "Commands to monitor backup and restore job status",
  }
  cmd.AddCommand(newListCommand(), newShowCommand())
  return cmd
}

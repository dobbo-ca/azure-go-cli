package assignment

import (
  "github.com/spf13/cobra"
)

// NewAssignmentCmd creates the role assignment command
func NewAssignmentCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "assignment",
    Short: "Manage Azure role assignments",
    Long:  "Manage Azure RBAC role assignments for users, groups, and service principals",
  }

  cmd.AddCommand(newListCmd())
  cmd.AddCommand(newCreateCmd())
  cmd.AddCommand(newDeleteCmd())

  return cmd
}

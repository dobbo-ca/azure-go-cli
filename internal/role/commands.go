package role

import (
  "github.com/cdobbyn/azure-go-cli/internal/role/assignment"
  "github.com/spf13/cobra"
)

// NewRoleCmd creates the role command
func NewRoleCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "role",
    Short: "Manage Azure role definitions and assignments",
    Long:  "Manage Azure RBAC role definitions and assignments",
  }

  cmd.AddCommand(newListCmd())
  cmd.AddCommand(newShowCmd())
  cmd.AddCommand(assignment.NewAssignmentCmd())

  return cmd
}

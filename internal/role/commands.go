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
	cmd.AddCommand(newDefinitionCmd())
	cmd.AddCommand(assignment.NewAssignmentCmd())

	return cmd
}

// newDefinitionCmd exposes role definitions under the azure-cli-compatible
// `az role definition ...` path. The list/show commands mirror the top-level
// `az role list`/`az role show` (kept for backward compatibility).
func newDefinitionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "definition",
		Short: "Manage Azure role definitions",
		Long:  "Manage Azure RBAC role definitions",
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newUpdateCmd())

	return cmd
}

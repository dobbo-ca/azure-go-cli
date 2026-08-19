package devops

import (
	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// securityNewClient is a test seam so security_test.go can point commands at
// an httptest server without depending on real Azure DevOps auth (mirrors
// extensionNewClient in extension.go and getCredential in ado/auth.go).
var securityNewClient = ado.NewClient

// newSecurityCommand wires `az devops security ...`: security group,
// security group membership, security permission, security permission
// namespace (dev/team/commands.py:149-171).
func newSecurityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "security",
		Short: "Manage security related operations for Azure DevOps.",
	}

	cmd.AddCommand(securityGroupCommand())
	cmd.AddCommand(securityPermissionCommand())

	return cmd
}

func securityGroupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage security groups.",
	}

	cmd.AddCommand(securityGroupListCmd())
	cmd.AddCommand(securityGroupShowCmd())
	cmd.AddCommand(securityGroupCreateCmd())
	cmd.AddCommand(securityGroupUpdateCmd())
	cmd.AddCommand(securityGroupDeleteCmd())
	cmd.AddCommand(securityGroupMembershipCommand())

	return cmd
}

func securityGroupMembershipCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "membership",
		Short: "Manage security group memberships.",
	}

	cmd.AddCommand(securityGroupMembershipListCmd())
	cmd.AddCommand(securityGroupMembershipAddCmd())
	cmd.AddCommand(securityGroupMembershipRemoveCmd())

	return cmd
}

func securityPermissionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permission",
		Short: "Manage security permissions.",
	}

	cmd.AddCommand(securityPermissionListCmd())
	cmd.AddCommand(securityPermissionShowCmd())
	cmd.AddCommand(securityPermissionUpdateCmd())
	cmd.AddCommand(securityPermissionResetAllCmd())
	cmd.AddCommand(securityPermissionResetCmd())
	cmd.AddCommand(securityPermissionNamespaceCommand())

	return cmd
}

func securityPermissionNamespaceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespace",
		Short: "Manage security permission namespaces.",
	}

	cmd.AddCommand(securityPermissionNamespaceListCmd())
	cmd.AddCommand(securityPermissionNamespaceShowCmd())

	return cmd
}

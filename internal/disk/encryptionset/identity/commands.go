package identity

import (
	"context"

	"github.com/spf13/cobra"
)

func NewIdentityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "identity",
		Short: "Manage the managed identity of a disk encryption set",
		Long:  "Assign, remove, or show the system-assigned and user-assigned managed identities of a disk encryption set.",
	}

	assignCmd := &cobra.Command{
		Use:   "assign",
		Short: "Add system-assigned or user-assigned identities to a disk encryption set",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			system, _ := cmd.Flags().GetBool("system-assigned")
			userAssigned, _ := cmd.Flags().GetStringSlice("user-assigned")
			return assignIdentity(context.Background(), cmd, name, rg, system, userAssigned)
		},
	}
	assignCmd.Flags().StringP("name", "n", "", "Disk encryption set name")
	assignCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	assignCmd.Flags().Bool("system-assigned", false, "Enable the system-assigned managed identity")
	assignCmd.Flags().StringSlice("user-assigned", nil, "Resource IDs of user-assigned managed identities to add")
	assignCmd.MarkFlagRequired("name")
	assignCmd.MarkFlagRequired("resource-group")

	removeCmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove system-assigned or user-assigned identities from a disk encryption set",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			system, _ := cmd.Flags().GetBool("system-assigned")
			userAssigned, _ := cmd.Flags().GetStringSlice("user-assigned")
			return removeIdentity(context.Background(), cmd, name, rg, system, userAssigned)
		},
	}
	removeCmd.Flags().StringP("name", "n", "", "Disk encryption set name")
	removeCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	removeCmd.Flags().Bool("system-assigned", false, "Remove the system-assigned managed identity")
	removeCmd.Flags().StringSlice("user-assigned", nil, "Resource IDs of user-assigned managed identities to remove")
	removeCmd.MarkFlagRequired("name")
	removeCmd.MarkFlagRequired("resource-group")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show the managed identity of a disk encryption set",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			return showIdentity(context.Background(), cmd, name, rg)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Disk encryption set name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(assignCmd, removeCmd, showCmd)
	return cmd
}

// identityType maps the presence of system/user identities to the ARM enum value.
func identityType(hasSystem, hasUser bool) string {
	switch {
	case hasSystem && hasUser:
		return "SystemAssigned, UserAssigned"
	case hasSystem:
		return "SystemAssigned"
	case hasUser:
		return "UserAssigned"
	default:
		return "None"
	}
}

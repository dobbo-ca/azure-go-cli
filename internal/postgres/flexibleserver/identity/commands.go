package identity

import (
	"context"

	"github.com/spf13/cobra"
)

func NewIdentityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "identity",
		Short: "Manage user-assigned managed identities on a PostgreSQL flexible server",
		Long:  "Assign, remove, list, and show user-assigned managed identities associated with an Azure Database for PostgreSQL flexible server.",
	}

	assignCmd := &cobra.Command{
		Use:   "assign",
		Short: "Assign user-assigned managed identities to a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			ids, _ := cmd.Flags().GetStringSlice("identity")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Assign(context.Background(), cmd, rg, server, ids, noWait)
		},
	}
	assignCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	assignCmd.Flags().String("server-name", "", "Flexible server name")
	assignCmd.Flags().StringSlice("identity", nil, "User-assigned managed identity resource ID (repeatable)")
	assignCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	assignCmd.MarkFlagRequired("resource-group")
	assignCmd.MarkFlagRequired("server-name")
	assignCmd.MarkFlagRequired("identity")

	removeCmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove user-assigned managed identities from a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			ids, _ := cmd.Flags().GetStringSlice("identity")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Remove(context.Background(), cmd, rg, server, ids, noWait)
		},
	}
	removeCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	removeCmd.Flags().String("server-name", "", "Flexible server name")
	removeCmd.Flags().StringSlice("identity", nil, "User-assigned managed identity resource ID to remove (repeatable)")
	removeCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	removeCmd.MarkFlagRequired("resource-group")
	removeCmd.MarkFlagRequired("server-name")
	removeCmd.MarkFlagRequired("identity")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show the managed identities of a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			return Show(context.Background(), cmd, rg, server)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("server-name", "", "Flexible server name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("server-name")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List the managed identities of a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			return List(context.Background(), cmd, rg, server)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.Flags().String("server-name", "", "Flexible server name")
	listCmd.MarkFlagRequired("resource-group")
	listCmd.MarkFlagRequired("server-name")

	cmd.AddCommand(assignCmd, removeCmd, showCmd, listCmd)
	return cmd
}

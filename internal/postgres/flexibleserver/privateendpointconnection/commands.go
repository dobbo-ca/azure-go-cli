package privateendpointconnection

import (
	"context"

	"github.com/spf13/cobra"
)

func NewPrivateEndpointConnectionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "private-endpoint-connection",
		Short: "Manage private endpoint connections for a PostgreSQL flexible server",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List private endpoint connections for a PostgreSQL flexible server",
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

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a private endpoint connection for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, rg, server, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("server-name", "", "Flexible server name")
	showCmd.Flags().StringP("name", "n", "", "Private endpoint connection name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("server-name")
	showCmd.MarkFlagRequired("name")

	approveCmd := &cobra.Command{
		Use:   "approve",
		Short: "Approve a private endpoint connection for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("name")
			description, _ := cmd.Flags().GetString("description")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Approve(context.Background(), cmd, rg, server, name, description, noWait)
		},
	}
	approveCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	approveCmd.Flags().String("server-name", "", "Flexible server name")
	approveCmd.Flags().StringP("name", "n", "", "Private endpoint connection name")
	approveCmd.Flags().String("description", "", "Reason for the approval")
	approveCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	approveCmd.MarkFlagRequired("resource-group")
	approveCmd.MarkFlagRequired("server-name")
	approveCmd.MarkFlagRequired("name")

	rejectCmd := &cobra.Command{
		Use:   "reject",
		Short: "Reject a private endpoint connection for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("name")
			description, _ := cmd.Flags().GetString("description")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Reject(context.Background(), cmd, rg, server, name, description, noWait)
		},
	}
	rejectCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	rejectCmd.Flags().String("server-name", "", "Flexible server name")
	rejectCmd.Flags().StringP("name", "n", "", "Private endpoint connection name")
	rejectCmd.Flags().String("description", "", "Reason for the rejection")
	rejectCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	rejectCmd.MarkFlagRequired("resource-group")
	rejectCmd.MarkFlagRequired("server-name")
	rejectCmd.MarkFlagRequired("name")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a private endpoint connection for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), cmd, rg, server, name, noWait)
		},
	}
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().String("server-name", "", "Flexible server name")
	deleteCmd.Flags().StringP("name", "n", "", "Private endpoint connection name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("server-name")
	deleteCmd.MarkFlagRequired("name")

	cmd.AddCommand(listCmd, showCmd, approveCmd, rejectCmd, deleteCmd)
	return cmd
}

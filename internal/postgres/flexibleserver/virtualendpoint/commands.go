package virtualendpoint

import (
	"context"

	"github.com/spf13/cobra"
)

func NewVirtualEndpointCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "virtual-endpoint",
		Short: "Manage virtual endpoints for a PostgreSQL flexible server",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List virtual endpoints for a PostgreSQL flexible server",
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
		Short: "Show a virtual endpoint for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("endpoint-name")
			return Show(context.Background(), cmd, rg, server, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("server-name", "", "Flexible server name")
	showCmd.Flags().StringP("endpoint-name", "n", "", "Virtual endpoint name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("server-name")
	showCmd.MarkFlagRequired("endpoint-name")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a virtual endpoint for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("endpoint-name")
			endpointType, _ := cmd.Flags().GetString("endpoint-type")
			members, _ := cmd.Flags().GetStringSlice("members")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Create(context.Background(), cmd, rg, server, name, endpointType, members, noWait)
		},
	}
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().String("server-name", "", "Flexible server name")
	createCmd.Flags().StringP("endpoint-name", "n", "", "Virtual endpoint name")
	createCmd.Flags().String("endpoint-type", "ReadWrite", "Endpoint type for the virtual endpoint")
	createCmd.Flags().StringSlice("members", nil, "List of members for the virtual endpoint")
	createCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("server-name")
	createCmd.MarkFlagRequired("endpoint-name")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a virtual endpoint for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("endpoint-name")
			members, _ := cmd.Flags().GetStringSlice("members")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Update(context.Background(), cmd, rg, server, name, members, noWait)
		},
	}
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().String("server-name", "", "Flexible server name")
	updateCmd.Flags().StringP("endpoint-name", "n", "", "Virtual endpoint name")
	updateCmd.Flags().StringSlice("members", nil, "List of members for the virtual endpoint")
	updateCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	updateCmd.MarkFlagRequired("resource-group")
	updateCmd.MarkFlagRequired("server-name")
	updateCmd.MarkFlagRequired("endpoint-name")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a virtual endpoint for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("endpoint-name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), cmd, rg, server, name, noWait)
		},
	}
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().String("server-name", "", "Flexible server name")
	deleteCmd.Flags().StringP("endpoint-name", "n", "", "Virtual endpoint name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("server-name")
	deleteCmd.MarkFlagRequired("endpoint-name")

	cmd.AddCommand(listCmd, showCmd, createCmd, updateCmd, deleteCmd)
	return cmd
}

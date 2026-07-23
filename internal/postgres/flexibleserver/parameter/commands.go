package parameter

import (
	"context"

	"github.com/spf13/cobra"
)

func NewParameterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parameter",
		Short: "Manage PostgreSQL flexible server parameters (server configurations)",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List parameters for a PostgreSQL flexible server",
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
		Short: "Show a parameter for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, rg, server, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("server-name", "", "Flexible server name")
	showCmd.Flags().StringP("name", "n", "", "Parameter name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("server-name")
	showCmd.MarkFlagRequired("name")

	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Set a parameter value for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("name")
			value, _ := cmd.Flags().GetString("value")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Set(context.Background(), cmd, rg, server, name, value, noWait)
		},
	}
	setCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	setCmd.Flags().String("server-name", "", "Flexible server name")
	setCmd.Flags().StringP("name", "n", "", "Parameter name")
	setCmd.Flags().String("value", "", "Parameter value")
	setCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	setCmd.MarkFlagRequired("resource-group")
	setCmd.MarkFlagRequired("server-name")
	setCmd.MarkFlagRequired("name")
	setCmd.MarkFlagRequired("value")

	cmd.AddCommand(listCmd, showCmd, setCmd)
	return cmd
}

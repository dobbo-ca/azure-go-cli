package privatelinkresource

import (
	"context"

	"github.com/spf13/cobra"
)

func NewPrivateLinkResourceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "private-link-resource",
		Short: "Manage private link resources for a PostgreSQL flexible server",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List private link resources",
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
		Short: "Show a private link resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, rg, server, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("server-name", "", "Flexible server name")
	showCmd.Flags().StringP("name", "n", "postgresqlServer", "Private link resource group name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("server-name")

	cmd.AddCommand(listCmd, showCmd)
	return cmd
}

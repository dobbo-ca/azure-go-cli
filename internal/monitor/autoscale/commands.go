package autoscale

import (
	"context"

	"github.com/spf13/cobra"
)

func NewAutoscaleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autoscale",
		Short: "Manage Azure Monitor autoscale settings",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List autoscale settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, resourceGroup)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional; lists across the subscription if omitted)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of an autoscale setting",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, resourceGroup, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().StringP("name", "n", "", "Autoscale setting name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("name")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an autoscale setting",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return Delete(context.Background(), cmd, resourceGroup, name)
		},
	}
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().StringP("name", "n", "", "Autoscale setting name")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("name")

	cmd.AddCommand(listCmd, showCmd, deleteCmd)
	return cmd
}

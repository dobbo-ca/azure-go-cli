package diagnosticsettings

import (
	"context"

	"github.com/spf13/cobra"
)

func NewDiagnosticSettingsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnostic-settings",
		Short: "Manage diagnostic settings",
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create or update a diagnostic setting",
		RunE: func(cmd *cobra.Command, args []string) error {
			resource, _ := cmd.Flags().GetString("resource")
			name, _ := cmd.Flags().GetString("name")
			workspace, _ := cmd.Flags().GetString("workspace")
			storageAccount, _ := cmd.Flags().GetString("storage-account")
			eventHubRule, _ := cmd.Flags().GetString("event-hub-rule")
			logs, _ := cmd.Flags().GetString("logs")
			metrics, _ := cmd.Flags().GetString("metrics")
			return Create(context.Background(), cmd, resource, name, workspace, storageAccount, eventHubRule, logs, metrics)
		},
	}
	createCmd.Flags().String("resource", "", "Full resource ID the diagnostic setting is attached to")
	createCmd.Flags().StringP("name", "n", "", "Name of the diagnostic setting")
	createCmd.Flags().String("workspace", "", "Resource ID of the Log Analytics workspace")
	createCmd.Flags().String("storage-account", "", "Resource ID of the storage account")
	createCmd.Flags().String("event-hub-rule", "", "Resource ID of the event hub authorization rule")
	createCmd.Flags().String("logs", "", "JSON array of log settings")
	createCmd.Flags().String("metrics", "", "JSON array of metric settings")
	createCmd.MarkFlagRequired("resource")
	createCmd.MarkFlagRequired("name")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a diagnostic setting",
		RunE: func(cmd *cobra.Command, args []string) error {
			resource, _ := cmd.Flags().GetString("resource")
			name, _ := cmd.Flags().GetString("name")
			return Delete(context.Background(), cmd, resource, name)
		},
	}
	deleteCmd.Flags().String("resource", "", "Full resource ID the diagnostic setting is attached to")
	deleteCmd.Flags().StringP("name", "n", "", "Name of the diagnostic setting")
	deleteCmd.MarkFlagRequired("resource")
	deleteCmd.MarkFlagRequired("name")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List diagnostic settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			resource, _ := cmd.Flags().GetString("resource")
			return List(context.Background(), cmd, resource)
		},
	}
	listCmd.Flags().String("resource", "", "Full resource ID the diagnostic setting is attached to")
	listCmd.MarkFlagRequired("resource")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a diagnostic setting",
		RunE: func(cmd *cobra.Command, args []string) error {
			resource, _ := cmd.Flags().GetString("resource")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, resource, name)
		},
	}
	showCmd.Flags().String("resource", "", "Full resource ID the diagnostic setting is attached to")
	showCmd.Flags().StringP("name", "n", "", "Name of the diagnostic setting")
	showCmd.MarkFlagRequired("resource")
	showCmd.MarkFlagRequired("name")

	cmd.AddCommand(createCmd, deleteCmd, listCmd, showCmd, newCategoriesCommand())
	return cmd
}

func newCategoriesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "categories",
		Short: "Manage diagnostic setting categories",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List diagnostic setting categories for a resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			resource, _ := cmd.Flags().GetString("resource")
			return CategoriesList(context.Background(), cmd, resource)
		},
	}
	listCmd.Flags().String("resource", "", "Full resource ID the diagnostic setting is attached to")
	listCmd.MarkFlagRequired("resource")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a diagnostic setting category",
		RunE: func(cmd *cobra.Command, args []string) error {
			resource, _ := cmd.Flags().GetString("resource")
			name, _ := cmd.Flags().GetString("name")
			return CategoriesShow(context.Background(), cmd, resource, name)
		},
	}
	showCmd.Flags().String("resource", "", "Full resource ID the diagnostic setting is attached to")
	showCmd.Flags().StringP("name", "n", "", "Name of the diagnostic setting category")
	showCmd.MarkFlagRequired("resource")
	showCmd.MarkFlagRequired("name")

	cmd.AddCommand(listCmd, showCmd)
	return cmd
}

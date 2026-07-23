package logprofiles

import (
	"context"

	"github.com/spf13/cobra"
)

func NewLogProfilesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log-profiles",
		Short: "Manage Azure Monitor log profiles",
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create or update a log profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			locations, _ := cmd.Flags().GetStringSlice("location")
			categories, _ := cmd.Flags().GetStringSlice("categories")
			days, _ := cmd.Flags().GetInt32("days")
			storageAccountID, _ := cmd.Flags().GetString("storage-account")
			serviceBusRuleID, _ := cmd.Flags().GetString("service-bus-rule-id")
			return Create(context.Background(), cmd, name, locations, categories, days, storageAccountID, serviceBusRuleID)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Log profile name")
	createCmd.Flags().StringSlice("location", nil, "Regions to collect from")
	createCmd.Flags().StringSlice("categories", nil, "e.g. Write Delete Action")
	createCmd.Flags().Int32("days", 0, "Retention in days (0 = retain indefinitely)")
	createCmd.Flags().String("storage-account", "", "Storage account resource ID")
	createCmd.Flags().String("service-bus-rule-id", "", "Service bus rule ID for streaming")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("location")
	createCmd.MarkFlagRequired("categories")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a log profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			return Delete(context.Background(), cmd, name)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Log profile name")
	deleteCmd.MarkFlagRequired("name")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List log profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return List(context.Background(), cmd)
		},
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a log profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, name)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Log profile name")
	showCmd.MarkFlagRequired("name")

	cmd.AddCommand(createCmd, deleteCmd, listCmd, showCmd)
	return cmd
}

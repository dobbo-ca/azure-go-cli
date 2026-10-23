package activitylog

import (
	"context"
	"time"

	"github.com/spf13/cobra"
)

func NewActivityLogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activity-log",
		Short: "Query Azure Monitor activity logs",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List activity log events",
		RunE: func(cmd *cobra.Command, args []string) error {
			startTime, _ := cmd.Flags().GetString("start-time")
			endTime, _ := cmd.Flags().GetString("end-time")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			resourceID, _ := cmd.Flags().GetString("resource-id")
			correlationID, _ := cmd.Flags().GetString("correlation-id")
			offset, _ := cmd.Flags().GetString("offset")
			start, end, err := timeRange(startTime, endTime, offset, time.Now())
			if err != nil {
				return err
			}
			return List(context.Background(), cmd, resourceGroup, resourceID, correlationID, start.Format(time.RFC3339), end.Format(time.RFC3339))
		},
	}
	listCmd.Flags().String("start-time", "", "ISO8601 e.g. 2024-01-01T00:00:00Z")
	listCmd.Flags().String("end-time", "", "ISO8601 e.g. 2024-01-01T00:00:00Z")
	listCmd.Flags().StringP("resource-group", "g", "", "Filter by resource group name")
	listCmd.Flags().String("resource-id", "", "Filter by resource ID")
	listCmd.Flags().String("correlation-id", "", "Filter by correlation ID")
	listCmd.Flags().String("offset", "6h", "Time range length in ##d##h format, used with --start-time, --end-time or now")

	listCategoriesCmd := &cobra.Command{
		Use:   "list-categories",
		Short: "List available activity log event categories",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ListCategories(context.Background(), cmd)
		},
	}

	cmd.AddCommand(listCmd, listCategoriesCmd)
	return cmd
}

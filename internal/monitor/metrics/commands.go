package metrics

import (
	"context"

	"github.com/spf13/cobra"
)

func NewMetricsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Manage Azure Monitor metrics",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List metric values for a resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			resource, _ := cmd.Flags().GetString("resource")
			metricNames, _ := cmd.Flags().GetString("metrics")
			aggregation, _ := cmd.Flags().GetString("aggregation")
			interval, _ := cmd.Flags().GetString("interval")
			startTime, _ := cmd.Flags().GetString("start-time")
			endTime, _ := cmd.Flags().GetString("end-time")
			filter, _ := cmd.Flags().GetString("filter")
			return List(context.Background(), cmd, resource, metricNames, aggregation, interval, startTime, endTime, filter)
		},
	}
	listCmd.Flags().String("resource", "", "Full resource ID")
	listCmd.Flags().String("metrics", "", "Metric names (comma-separated)")
	listCmd.Flags().String("aggregation", "", "Aggregation types (comma-separated, e.g. average, minimum, maximum)")
	listCmd.Flags().String("interval", "", "Interval (timegrain) in ISO 8601 duration format (e.g. PT1M, PT1H)")
	listCmd.Flags().String("start-time", "", "Start time (ISO 8601)")
	listCmd.Flags().String("end-time", "", "End time (ISO 8601)")
	listCmd.Flags().String("filter", "", "$filter to reduce the set of metric data returned")
	listCmd.MarkFlagRequired("resource")

	listDefinitionsCmd := &cobra.Command{
		Use:   "list-definitions",
		Short: "List metric definitions for a resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			resource, _ := cmd.Flags().GetString("resource")
			return ListDefinitions(context.Background(), cmd, resource)
		},
	}
	listDefinitionsCmd.Flags().String("resource", "", "Full resource ID")
	listDefinitionsCmd.MarkFlagRequired("resource")

	cmd.AddCommand(listCmd, listDefinitionsCmd)
	return cmd
}

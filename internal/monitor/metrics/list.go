package metrics

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armmonitor "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resource, metricNames, aggregation, interval, startTime, endTime, filter string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewMetricsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create metrics client: %w", err)
	}

	opts := &armmonitor.MetricsClientListOptions{}
	if metricNames != "" {
		opts.Metricnames = to.Ptr(metricNames)
	}
	if aggregation != "" {
		opts.Aggregation = to.Ptr(aggregation)
	}
	if interval != "" {
		opts.Interval = to.Ptr(interval)
	}
	if filter != "" {
		opts.Filter = to.Ptr(filter)
	}
	if startTime != "" && endTime != "" {
		opts.Timespan = to.Ptr(startTime + "/" + endTime)
	}

	resp, err := client.List(ctx, resource, opts)
	if err != nil {
		return fmt.Errorf("failed to list metrics: %w", err)
	}
	return output.PrintJSON(cmd, resp.Response)
}

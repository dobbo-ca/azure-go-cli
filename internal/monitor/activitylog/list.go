package activitylog

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resourceGroup, resourceID, correlationID, startTime, endTime string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewActivityLogsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create activity logs client: %w", err)
	}

	parts := []string{
		fmt.Sprintf("eventTimestamp ge '%s'", startTime),
		fmt.Sprintf("eventTimestamp le '%s'", endTime),
	}
	if resourceGroup != "" {
		parts = append(parts, fmt.Sprintf("resourceGroupName eq '%s'", resourceGroup))
	}
	if resourceID != "" {
		parts = append(parts, fmt.Sprintf("resourceId eq '%s'", resourceID))
	}
	if correlationID != "" {
		parts = append(parts, fmt.Sprintf("correlationId eq '%s'", correlationID))
	}
	filter := strings.Join(parts, " and ")

	var events []*armmonitor.EventData
	pager := client.NewListPager(filter, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list activity logs: %w", err)
		}
		events = append(events, page.Value...)
	}
	return output.PrintJSON(cmd, events)
}

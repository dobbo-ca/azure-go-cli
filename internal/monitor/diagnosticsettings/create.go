package diagnosticsettings

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, resource, name, workspace, storageAccount, eventHubRuleID, logsJSON, metricsJSON string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewDiagnosticSettingsClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create diagnostic settings client: %w", err)
	}

	props := &armmonitor.DiagnosticSettings{}
	if workspace != "" {
		props.WorkspaceID = to.Ptr(workspace)
	}
	if storageAccount != "" {
		props.StorageAccountID = to.Ptr(storageAccount)
	}
	if eventHubRuleID != "" {
		props.EventHubAuthorizationRuleID = to.Ptr(eventHubRuleID)
	}
	if logsJSON != "" {
		if err := json.Unmarshal([]byte(logsJSON), &props.Logs); err != nil {
			return fmt.Errorf("failed to parse logs JSON: %w", err)
		}
	}
	if metricsJSON != "" {
		if err := json.Unmarshal([]byte(metricsJSON), &props.Metrics); err != nil {
			return fmt.Errorf("failed to parse metrics JSON: %w", err)
		}
	}

	resp, err := client.CreateOrUpdate(ctx, resource, name, armmonitor.DiagnosticSettingsResource{Properties: props}, nil)
	if err != nil {
		return fmt.Errorf("failed to create diagnostic setting: %w", err)
	}
	return output.PrintJSON(cmd, resp.DiagnosticSettingsResource)
}

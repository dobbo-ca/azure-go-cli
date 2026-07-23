package diagnosticsettings

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resource string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewDiagnosticSettingsClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create diagnostic settings client: %w", err)
	}

	var items []*armmonitor.DiagnosticSettingsResource
	pager := client.NewListPager(resource, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list diagnostic settings: %w", err)
		}
		items = append(items, page.Value...)
	}
	return output.PrintJSON(cmd, items)
}

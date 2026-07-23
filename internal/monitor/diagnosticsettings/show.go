package diagnosticsettings

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, resource, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewDiagnosticSettingsClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create diagnostic settings client: %w", err)
	}

	resp, err := client.Get(ctx, resource, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get diagnostic setting: %w", err)
	}
	return output.PrintJSON(cmd, resp.DiagnosticSettingsResource)
}

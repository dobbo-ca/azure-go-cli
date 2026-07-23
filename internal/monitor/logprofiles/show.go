package logprofiles

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewLogProfilesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create log profiles client: %w", err)
	}

	resp, err := client.Get(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get log profile: %w", err)
	}
	return output.PrintJSON(cmd, resp.LogProfileResource)
}

package advancedthreatprotection

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Update(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, state string, noWait bool) error {
	var protectionState armpostgresqlflexibleservers.ThreatProtectionState
	switch state {
	case "Enabled":
		protectionState = armpostgresqlflexibleservers.ThreatProtectionStateEnabled
	case "Disabled":
		protectionState = armpostgresqlflexibleservers.ThreatProtectionStateDisabled
	default:
		return fmt.Errorf("invalid state %q: must be Enabled or Disabled", state)
	}

	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewServerThreatProtectionSettingsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create threat protection settings client: %w", err)
	}

	model := armpostgresqlflexibleservers.ServerThreatProtectionSettingsModel{
		Properties: &armpostgresqlflexibleservers.ServerThreatProtectionProperties{
			State: &protectionState,
		},
	}

	fmt.Printf("Updating threat protection settings for server '%s'...\n", serverName)

	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, serverName, armpostgresqlflexibleservers.ThreatProtectionNameDefault, model, nil)
	if err != nil {
		return fmt.Errorf("failed to begin threat protection settings update: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "Threat protection settings update started."})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("threat protection settings update failed: %w", err)
	}
	return output.PrintJSON(cmd, resp.ServerThreatProtectionSettingsModel)
}

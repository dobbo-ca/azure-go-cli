package firewallrule

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Delete(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, ruleName string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewFirewallRulesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create firewall rules client: %w", err)
	}

	poller, err := client.BeginDelete(ctx, resourceGroup, serverName, ruleName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin firewall rule delete: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "Firewall rule delete started."})
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("firewall rule delete failed: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Firewall rule '%s' deleted.", ruleName)})
}

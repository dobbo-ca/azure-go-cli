package firewallrule

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Update(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, ruleName, startIP, endIP string, noWait bool) error {
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

	rule := armpostgresqlflexibleservers.FirewallRule{
		Properties: &armpostgresqlflexibleservers.FirewallRuleProperties{
			StartIPAddress: to.Ptr(startIP),
			EndIPAddress:   to.Ptr(endIP),
		},
	}

	fmt.Printf("Updating firewall rule '%s' on server '%s'...\n", ruleName, serverName)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, serverName, ruleName, rule, nil)
	if err != nil {
		return fmt.Errorf("failed to begin firewall rule update: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "Firewall rule update started."})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("firewall rule update failed: %w", err)
	}

	return output.PrintJSON(cmd, resp.FirewallRule)
}

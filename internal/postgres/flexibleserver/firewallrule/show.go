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

func Show(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, ruleName string) error {
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

	resp, err := client.Get(ctx, resourceGroup, serverName, ruleName, nil)
	if err != nil {
		return fmt.Errorf("failed to get firewall rule: %w", err)
	}

	return output.PrintJSON(cmd, resp.FirewallRule)
}

package rule

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, name, nsgName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewSecurityRulesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create security rules client: %w", err)
	}

	result, err := client.Get(ctx, resourceGroup, nsgName, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get security rule: %w", err)
	}

	return output.PrintJSON(cmd, result.SecurityRule)
}

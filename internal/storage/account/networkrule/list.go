package networkrule

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, account, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armstorage.NewAccountsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create storage accounts client: %w", err)
	}

	resp, err := client.GetProperties(ctx, resourceGroup, account, nil)
	if err != nil {
		return fmt.Errorf("failed to get storage account properties: %w", err)
	}
	if resp.Properties == nil || resp.Properties.NetworkRuleSet == nil {
		return output.PrintJSON(cmd, map[string]interface{}{})
	}
	return output.PrintJSON(cmd, resp.Properties.NetworkRuleSet)
}

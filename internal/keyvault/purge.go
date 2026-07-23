package keyvault

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Purge(ctx context.Context, cmd *cobra.Command, name, location string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armkeyvault.NewVaultsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create key vaults client: %w", err)
	}

	fmt.Printf("Purging deleted key vault '%s'...\n", name)
	poller, err := client.BeginPurgeDeleted(ctx, name, location, nil)
	if err != nil {
		return fmt.Errorf("failed to begin purge: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "purge started"})
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("purge failed: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("'%s' purged.", name)})
}

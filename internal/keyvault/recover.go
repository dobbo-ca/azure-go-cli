package keyvault

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// Recover recovers a soft-deleted vault by re-creating it with createMode=recover.
func Recover(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location string, noWait bool) error {
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

	tenantID, err := config.GetTenantID(subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to get tenant ID: %w", err)
	}

	parameters := armkeyvault.VaultCreateOrUpdateParameters{
		Location: to.Ptr(location),
		Properties: &armkeyvault.VaultProperties{
			TenantID:   to.Ptr(tenantID),
			CreateMode: to.Ptr(armkeyvault.CreateModeRecover),
			SKU: &armkeyvault.SKU{
				Family: to.Ptr(armkeyvault.SKUFamilyA),
				Name:   to.Ptr(armkeyvault.SKUNameStandard),
			},
		},
	}

	fmt.Printf("Recovering key vault '%s'...\n", name)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin recover: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "recover started"})
	}
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("recover failed: %w", err)
	}
	return output.PrintJSON(cmd, result.Vault)
}

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

func CheckName(ctx context.Context, cmd *cobra.Command, name string) error {
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

	resp, err := client.CheckNameAvailability(ctx, armkeyvault.VaultCheckNameAvailabilityParameters{
		Name: to.Ptr(name),
		Type: to.Ptr("Microsoft.KeyVault/vaults"),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to check name availability: %w", err)
	}
	return output.PrintJSON(cmd, resp.CheckNameAvailabilityResult)
}

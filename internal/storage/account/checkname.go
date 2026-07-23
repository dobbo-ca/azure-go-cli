package account

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
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
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armstorage.NewAccountsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create storage accounts client: %w", err)
	}

	resp, err := client.CheckNameAvailability(ctx, armstorage.AccountCheckNameAvailabilityParameters{
		Name: to.Ptr(name),
		Type: to.Ptr("Microsoft.Storage/storageAccounts"),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to check name availability: %w", err)
	}

	return output.PrintJSON(cmd, resp.CheckNameAvailabilityResult)
}

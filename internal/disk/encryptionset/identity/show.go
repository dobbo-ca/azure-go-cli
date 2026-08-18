package identity

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func newClient() (*armcompute.DiskEncryptionSetsClient, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armcompute.NewDiskEncryptionSetsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create disk encryption sets client: %w", err)
	}
	return client, nil
}

func showIdentity(ctx context.Context, cmd *cobra.Command, name, resourceGroup string) error {
	client, err := newClient()
	if err != nil {
		return err
	}

	resp, err := client.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get disk encryption set: %w", err)
	}

	return output.PrintJSON(cmd, resp.Identity)
}

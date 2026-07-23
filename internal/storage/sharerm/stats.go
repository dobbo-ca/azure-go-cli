package sharerm

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

func Stats(ctx context.Context, cmd *cobra.Command, resourceGroup, account, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armstorage.NewFileSharesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create file shares client: %w", err)
	}

	resp, err := client.Get(ctx, resourceGroup, account, name, &armstorage.FileSharesClientGetOptions{Expand: to.Ptr("stats")})
	if err != nil {
		return fmt.Errorf("failed to get file share stats: %w", err)
	}
	return output.PrintJSON(cmd, resp.FileShare)
}

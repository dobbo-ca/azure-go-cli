package sharerm

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Exists(ctx context.Context, cmd *cobra.Command, resourceGroup, account, name string) error {
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

	if _, err := client.Get(ctx, resourceGroup, account, name, nil); err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 404 {
			return output.PrintJSON(cmd, map[string]bool{"exists": false})
		}
		return fmt.Errorf("failed to get file share: %w", err)
	}
	return output.PrintJSON(cmd, map[string]bool{"exists": true})
}

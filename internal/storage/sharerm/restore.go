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

func Restore(ctx context.Context, cmd *cobra.Command, resourceGroup, account, name, deletedVersion string) error {
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

	deletedShare := armstorage.DeletedShare{
		DeletedShareName:    to.Ptr(name),
		DeletedShareVersion: to.Ptr(deletedVersion),
	}
	if _, err := client.Restore(ctx, resourceGroup, account, name, deletedShare, nil); err != nil {
		return fmt.Errorf("failed to restore file share: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("share '%s' restore requested.", name)})
}

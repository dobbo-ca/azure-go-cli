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

func Update(ctx context.Context, cmd *cobra.Command, resourceGroup, account, name string, quota *int32, accessTier string, metadata map[string]string) error {
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

	fs := armstorage.FileShare{FileShareProperties: &armstorage.FileShareProperties{}}
	if quota != nil {
		fs.FileShareProperties.ShareQuota = quota
	}
	if accessTier != "" {
		fs.FileShareProperties.AccessTier = to.Ptr(armstorage.ShareAccessTier(accessTier))
	}
	if len(metadata) > 0 {
		md := make(map[string]*string, len(metadata))
		for k, v := range metadata {
			md[k] = to.Ptr(v)
		}
		fs.FileShareProperties.Metadata = md
	}

	resp, err := client.Update(ctx, resourceGroup, account, name, fs, nil)
	if err != nil {
		return fmt.Errorf("failed to update file share: %w", err)
	}
	return output.PrintJSON(cmd, resp.FileShare)
}

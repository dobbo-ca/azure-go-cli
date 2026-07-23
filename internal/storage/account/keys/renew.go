package keys

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

func Renew(ctx context.Context, cmd *cobra.Command, account, resourceGroup, keyName string) error {
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

	resp, err := client.RegenerateKey(ctx, resourceGroup, account, armstorage.AccountRegenerateKeyParameters{KeyName: to.Ptr(keyName)}, nil)
	if err != nil {
		return fmt.Errorf("failed to regenerate storage account key: %w", err)
	}
	return output.PrintJSON(cmd, resp.Keys)
}

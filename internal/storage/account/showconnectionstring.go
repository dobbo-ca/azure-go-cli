package account

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func ShowConnectionString(ctx context.Context, cmd *cobra.Command, resourceGroup, name string) error {
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

	resp, err := client.ListKeys(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to list storage account keys: %w", err)
	}
	if len(resp.Keys) == 0 {
		return fmt.Errorf("no keys returned")
	}

	key := azure.GetStringValue(resp.Keys[0].Value)
	// ponytail: public-cloud suffix hardcoded; add cloud-awareness if sovereign clouds needed
	cs := fmt.Sprintf("DefaultEndpointsProtocol=https;AccountName=%s;AccountKey=%s;EndpointSuffix=core.windows.net", name, key)

	return output.PrintJSON(cmd, map[string]string{"connectionString": cs})
}

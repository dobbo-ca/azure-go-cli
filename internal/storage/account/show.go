package account

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, accountName, resourceGroup string) error {
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

	account, err := client.GetProperties(ctx, resourceGroup, accountName, nil)
	if err != nil {
		return fmt.Errorf("failed to get storage account: %w", err)
	}

	data, err := json.MarshalIndent(account, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format storage account: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

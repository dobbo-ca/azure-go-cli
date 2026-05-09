package backup

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func List(ctx context.Context, resourceGroup, serverName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewBackupsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create backups client: %w", err)
	}

	var backups []*armpostgresqlflexibleservers.ServerBackup
	pager := client.NewListByServerPager(resourceGroup, serverName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list backups: %w", err)
		}
		backups = append(backups, page.Value...)
	}

	data, err := json.MarshalIndent(backups, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format backups: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

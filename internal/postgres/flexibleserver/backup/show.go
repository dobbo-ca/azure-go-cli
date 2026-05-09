package backup

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, resourceGroup, serverName, backupName string) error {
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

	resp, err := client.Get(ctx, resourceGroup, serverName, backupName, nil)
	if err != nil {
		return fmt.Errorf("failed to get backup: %w", err)
	}

	data, err := json.MarshalIndent(resp.ServerBackup, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format backup: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

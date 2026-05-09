package backup

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, resourceGroup, serverName, backupName string, noWait bool) error {
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

	poller, err := client.BeginDelete(ctx, resourceGroup, serverName, backupName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin backup delete: %w", err)
	}

	if noWait {
		fmt.Println(`{"status": "Backup delete started."}`)
		return nil
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("backup delete failed: %w", err)
	}
	fmt.Printf(`{"status": "Backup '%s' deleted."}`+"\n", backupName)
	return nil
}

package backup

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, backupName string, noWait bool) error {
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

	fmt.Printf("Triggering on-demand backup '%s' on server '%s'...\n", backupName, serverName)
	poller, err := client.BeginCreate(ctx, resourceGroup, serverName, backupName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin backup: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "On-demand backup started. Use 'az postgres flexible-server backup show' to monitor."})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("backup operation failed: %w", err)
	}

	return output.PrintJSON(cmd, resp.ServerBackup)
}

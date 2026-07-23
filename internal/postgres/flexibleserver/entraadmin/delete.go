package entraadmin

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Delete(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, objectID string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewAdministratorsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create administrators client: %w", err)
	}

	fmt.Printf("Removing Microsoft Entra administrator '%s' from server '%s'...\n", objectID, serverName)
	poller, err := client.BeginDelete(ctx, resourceGroup, serverName, objectID, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "delete started"})
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("'%s' deleted.", objectID)})
}

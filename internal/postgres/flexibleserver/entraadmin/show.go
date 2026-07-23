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

func Show(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, objectID string) error {
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

	resp, err := client.Get(ctx, resourceGroup, serverName, objectID, nil)
	if err != nil {
		return fmt.Errorf("failed to get administrator: %w", err)
	}

	return output.PrintJSON(cmd, resp.ActiveDirectoryAdministrator)
}

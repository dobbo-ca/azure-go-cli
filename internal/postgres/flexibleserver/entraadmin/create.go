package entraadmin

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, objectID, displayName, principalType string, noWait bool) error {
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

	tenantID, err := config.GetTenantID(subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to resolve tenant ID: %w", err)
	}

	props := &armpostgresqlflexibleservers.AdministratorPropertiesForAdd{
		PrincipalName: to.Ptr(displayName),
		PrincipalType: to.Ptr(armpostgresqlflexibleservers.PrincipalType(principalType)),
		TenantID:      to.Ptr(tenantID),
	}

	fmt.Printf("Adding Microsoft Entra administrator '%s' to server '%s'...\n", objectID, serverName)
	poller, err := client.BeginCreate(ctx, resourceGroup, serverName, objectID, armpostgresqlflexibleservers.ActiveDirectoryAdministratorAdd{Properties: props}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "create started"})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("create operation failed: %w", err)
	}

	return output.PrintJSON(cmd, resp.ActiveDirectoryAdministrator)
}

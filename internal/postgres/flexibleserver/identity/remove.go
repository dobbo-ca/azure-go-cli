package identity

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

func Remove(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName string, identityIDs []string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewServersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create servers client: %w", err)
	}

	current, err := client.Get(ctx, resourceGroup, serverName, nil)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	identities := map[string]*armpostgresqlflexibleservers.UserIdentity{}
	if current.Identity != nil {
		for k, v := range current.Identity.UserAssignedIdentities {
			identities[k] = v
		}
	}
	for _, id := range identityIDs {
		identities[id] = nil
	}

	fmt.Printf("Removing user-assigned identities from '%s'...\n", serverName)

	poller, err := client.BeginUpdate(ctx, resourceGroup, serverName, armpostgresqlflexibleservers.ServerForUpdate{
		Identity: &armpostgresqlflexibleservers.UserAssignedIdentity{
			Type:                   to.Ptr(armpostgresqlflexibleservers.IdentityTypeUserAssigned),
			UserAssignedIdentities: identities,
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin identity remove: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "Identity remove started."})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("identity remove failed: %w", err)
	}
	return output.PrintJSON(cmd, resp.Server.Identity)
}

package identity

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, name, resourceGroup, subscriptionOverride string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetSubscription(subscriptionOverride)
	if err != nil {
		return err
	}

	client, err := armmsi.NewUserAssignedIdentitiesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create managed identities client: %w", err)
	}

	identity, err := client.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get managed identity: %w", err)
	}

	return output.PrintJSON(cmd, identity)
}

// ShowByIDs shows one or more managed identities by their resource IDs
func ShowByIDs(ctx context.Context, cmd *cobra.Command, ids []string, subscriptionOverride string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	var results []interface{}

	for _, id := range ids {
		// Parse resource ID
		// Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.ManagedIdentity/userAssignedIdentities/{name}
		parts := strings.Split(id, "/")
		if len(parts) < 9 {
			return fmt.Errorf("invalid resource ID format: %s", id)
		}

		subscriptionID := parts[2]
		resourceGroup := parts[4]
		name := parts[8]

		// Override subscription if specified
		if subscriptionOverride != "" {
			subscriptionID, err = config.GetSubscription(subscriptionOverride)
			if err != nil {
				return err
			}
		}

		client, err := armmsi.NewUserAssignedIdentitiesClient(subscriptionID, cred, nil)
		if err != nil {
			return fmt.Errorf("failed to create managed identities client: %w", err)
		}

		identity, err := client.Get(ctx, resourceGroup, name, nil)
		if err != nil {
			return fmt.Errorf("failed to get managed identity %s: %w", id, err)
		}

		results = append(results, identity)
	}

	// Output results - if single ID, output just the object; if multiple, output array
	if len(results) == 1 {
		return output.PrintJSON(cmd, results[0])
	}
	return output.PrintJSON(cmd, results)
}

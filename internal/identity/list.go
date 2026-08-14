package identity

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resourceGroup, subscriptionOverride string) error {
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

	var identities []map[string]interface{}

	// List by resource group if specified, otherwise list all in subscription
	if resourceGroup != "" {
		pager := client.NewListByResourceGroupPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list managed identities: %w", err)
			}

			for _, identity := range page.Value {
				identities = append(identities, formatIdentity(identity))
			}
		}
	} else {
		pager := client.NewListBySubscriptionPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list managed identities: %w", err)
			}

			for _, identity := range page.Value {
				identities = append(identities, formatIdentity(identity))
			}
		}
	}

	return output.PrintJSON(cmd, identities)
}

func formatIdentity(identity *armmsi.Identity) map[string]interface{} {
	result := map[string]interface{}{
		"name":     azure.GetStringValue(identity.Name),
		"id":       azure.GetStringValue(identity.ID),
		"location": azure.GetStringValue(identity.Location),
		"type":     azure.GetStringValue(identity.Type),
	}

	if identity.Properties != nil {
		if identity.Properties.ClientID != nil {
			result["clientId"] = azure.GetStringValue(identity.Properties.ClientID)
		}
		if identity.Properties.PrincipalID != nil {
			result["principalId"] = azure.GetStringValue(identity.Properties.PrincipalID)
		}
		if identity.Properties.TenantID != nil {
			result["tenantId"] = azure.GetStringValue(identity.Properties.TenantID)
		}
	}

	if identity.Tags != nil && len(identity.Tags) > 0 {
		result["tags"] = identity.Tags
	}

	return result
}

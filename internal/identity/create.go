package identity

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location string, tags map[string]string, subscriptionOverride string) error {
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

	// Convert tags to Azure format
	azureTags := make(map[string]*string)
	for k, v := range tags {
		azureTags[k] = to.Ptr(v)
	}

	parameters := armmsi.Identity{
		Location: to.Ptr(location),
		Tags:     azureTags,
	}

	identity, err := client.CreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to create managed identity: %w", err)
	}

	return output.PrintJSON(cmd, identity)
}

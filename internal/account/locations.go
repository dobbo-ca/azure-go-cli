package account

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// LocationInfo is the trimmed view of an Azure location returned by list-locations.
type LocationInfo struct {
	Name                string `json:"name"`
	DisplayName         string `json:"displayName"`
	RegionalDisplayName string `json:"regionalDisplayName"`
	ID                  string `json:"id"`
}

// ListLocations lists the locations available to a subscription. It honors the
// global --subscription flag, falling back to the default subscription.
func ListLocations(ctx context.Context, cmd *cobra.Command) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	sub, _ := cmd.Flags().GetString("subscription")
	subscriptionID, err := config.GetSubscription(sub)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armsubscriptions.NewClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create subscriptions client: %w", err)
	}

	var locations []LocationInfo
	pager := client.NewListLocationsPager(subscriptionID, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list locations: %w", err)
		}
		for _, loc := range page.Value {
			locations = append(locations, LocationInfo{
				Name:                azure.GetStringValue(loc.Name),
				DisplayName:         azure.GetStringValue(loc.DisplayName),
				RegionalDisplayName: azure.GetStringValue(loc.RegionalDisplayName),
				ID:                  azure.GetStringValue(loc.ID),
			})
		}
	}

	format, _ := cmd.Flags().GetString("output")
	return output.PrintFormatted(cmd, locations, format)
}

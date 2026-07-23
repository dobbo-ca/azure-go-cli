package migration

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

func CheckNameAvailability(ctx context.Context, cmd *cobra.Command, name, location string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armpostgresqlflexibleservers.NewCheckNameAvailabilityWithLocationClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create check name availability client: %w", err)
	}

	request := armpostgresqlflexibleservers.CheckNameAvailabilityRequest{
		Name: to.Ptr(name),
		Type: to.Ptr("Microsoft.DBforPostgreSQL/flexibleServers"),
	}

	resp, err := client.Execute(ctx, location, request, nil)
	if err != nil {
		return fmt.Errorf("failed to check name availability: %w", err)
	}
	return output.PrintJSON(cmd, resp.NameAvailability)
}

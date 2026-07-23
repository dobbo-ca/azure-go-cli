package parameter

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

func Set(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, configurationName, value string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewConfigurationsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create configurations client: %w", err)
	}

	parameters := armpostgresqlflexibleservers.Configuration{
		Properties: &armpostgresqlflexibleservers.ConfigurationProperties{
			Value:  to.Ptr(value),
			Source: to.Ptr("user-override"),
		},
	}

	fmt.Printf("Setting parameter '%s' to '%s' on server '%s'...\n", configurationName, value, serverName)
	poller, err := client.BeginPut(ctx, resourceGroup, serverName, configurationName, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin parameter update: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "Parameter update started."})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("parameter update failed: %w", err)
	}

	return output.PrintJSON(cmd, resp.Configuration)
}

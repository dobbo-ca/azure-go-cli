package natgateway

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Update(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewNatGatewaysClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create nat gateways client: %w", err)
	}

	current, err := client.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get nat gateway: %w", err)
	}

	flags := cmd.Flags()

	if flags.Changed("idle-timeout") {
		v, _ := flags.GetInt32("idle-timeout")
		if current.Properties == nil {
			current.Properties = &armnetwork.NatGatewayPropertiesFormat{}
		}
		current.Properties.IdleTimeoutInMinutes = to.Ptr(v)
	}

	if flags.Changed("tags") {
		tags, _ := flags.GetStringToString("tags")
		azureTags := make(map[string]*string)
		for k, v := range tags {
			azureTags[k] = to.Ptr(v)
		}
		current.Tags = azureTags
	}

	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, current.NatGateway, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update NAT gateway: %w", err)
	}

	if noWait {
		fmt.Printf("Started update of NAT gateway '%s'\n", name)
		return nil
	}

	fmt.Printf("Updating NAT gateway '%s'...\n", name)
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update NAT gateway: %w", err)
	}

	return output.PrintJSON(cmd, result.NatGateway)
}

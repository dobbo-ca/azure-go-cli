package publicip

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

	client, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create public IP client: %w", err)
	}

	current, err := client.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get public IP: %w", err)
	}

	if current.Properties == nil {
		current.Properties = &armnetwork.PublicIPAddressPropertiesFormat{}
	}
	props := current.Properties

	flags := cmd.Flags()

	if flags.Changed("idle-timeout") {
		v, _ := flags.GetInt32("idle-timeout")
		props.IdleTimeoutInMinutes = to.Ptr(v)
	}

	if flags.Changed("allocation-method") {
		allocationMethod, _ := flags.GetString("allocation-method")
		var allocation armnetwork.IPAllocationMethod
		switch allocationMethod {
		case "Static":
			allocation = armnetwork.IPAllocationMethodStatic
		case "Dynamic":
			allocation = armnetwork.IPAllocationMethodDynamic
		default:
			return fmt.Errorf("invalid allocation method: %s (must be Static or Dynamic)", allocationMethod)
		}
		props.PublicIPAllocationMethod = to.Ptr(allocation)
	}

	if flags.Changed("tags") {
		tags, _ := flags.GetStringToString("tags")
		azureTags := make(map[string]*string)
		for k, v := range tags {
			azureTags[k] = to.Ptr(v)
		}
		current.Tags = azureTags
	}

	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, current.PublicIPAddress, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update public IP: %w", err)
	}

	if noWait {
		fmt.Printf("Started update of public IP address '%s'\n", name)
		return nil
	}

	fmt.Printf("Updating public IP address '%s'...\n", name)
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update public IP: %w", err)
	}

	return output.PrintJSON(cmd, result.PublicIPAddress)
}

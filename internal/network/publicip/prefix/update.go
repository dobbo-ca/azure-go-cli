package prefix

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

	client, err := armnetwork.NewPublicIPPrefixesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create public IP prefix client: %w", err)
	}

	current, err := client.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get public IP prefix: %w", err)
	}

	flags := cmd.Flags()

	if flags.Changed("tags") {
		tags, _ := flags.GetStringToString("tags")
		azureTags := make(map[string]*string)
		for k, v := range tags {
			azureTags[k] = to.Ptr(v)
		}
		current.Tags = azureTags
	}

	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, current.PublicIPPrefix, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update public IP prefix: %w", err)
	}

	if noWait {
		fmt.Printf("Started update of public IP prefix '%s'\n", name)
		return nil
	}

	fmt.Printf("Updating public IP prefix '%s'...\n", name)
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update public IP prefix: %w", err)
	}

	return output.PrintJSON(cmd, result.PublicIPPrefix)
}

package asg

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

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location string, tags map[string]string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewApplicationSecurityGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create ASG client: %w", err)
	}

	// Convert tags to Azure format
	azureTags := make(map[string]*string)
	for k, v := range tags {
		azureTags[k] = to.Ptr(v)
	}

	parameters := armnetwork.ApplicationSecurityGroup{
		Location:   to.Ptr(location),
		Tags:       azureTags,
		Properties: &armnetwork.ApplicationSecurityGroupPropertiesFormat{},
	}

	fmt.Printf("Creating application security group '%s'...\n", name)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to create ASG: %w", err)
	}

	if noWait, _ := cmd.Flags().GetBool("no-wait"); noWait {
		fmt.Printf("Create operation started for application security group '%s'\n", name)
		return nil
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete ASG creation: %w", err)
	}

	fmt.Printf("Created application security group '%s'\n", name)
	return output.PrintJSON(cmd, result.ApplicationSecurityGroup)
}

package nsg

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewSecurityGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create NSG client: %w", err)
	}

	var nsgs []*armnetwork.SecurityGroup
	if resourceGroup != "" {
		pager := client.NewListPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get next page: %w", err)
			}
			nsgs = append(nsgs, page.Value...)
		}
	} else {
		pager := client.NewListAllPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get next page: %w", err)
			}
			nsgs = append(nsgs, page.Value...)
		}
	}

	// nsg list historically defaulted to table output, unlike the global
	// default of json, so only honor an explicitly-passed format.
	format, _ := cmd.Flags().GetString("output")
	if !cmd.Flags().Changed("output") {
		format = "table"
	}
	if format != "table" {
		return output.PrintJSON(cmd, nsgs)
	}

	fmt.Printf("%-40s %-30s %-20s\n", "NAME", "LOCATION", "PROVISIONING STATE")
	fmt.Println("------------------------------------------------------------------------------------------------")
	for _, nsg := range nsgs {
		name := ""
		if nsg.Name != nil {
			name = *nsg.Name
		}

		location := ""
		if nsg.Location != nil {
			location = *nsg.Location
		}

		provisioningState := ""
		if nsg.Properties != nil && nsg.Properties.ProvisioningState != nil {
			provisioningState = string(*nsg.Properties.ProvisioningState)
		}

		fmt.Printf("%-40s %-30s %-20s\n", name, location, provisioningState)
	}

	return nil
}

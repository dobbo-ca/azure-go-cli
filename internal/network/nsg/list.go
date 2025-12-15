package nsg

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func List(ctx context.Context, resourceGroup string) error {
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

	fmt.Printf("%-40s %-30s %-20s\n", "NAME", "LOCATION", "PROVISIONING STATE")
	fmt.Println("------------------------------------------------------------------------------------------------")

	if resourceGroup != "" {
		pager := client.NewListPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get next page: %w", err)
			}

			for _, nsg := range page.Value {
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
		}
	} else {
		pager := client.NewListAllPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get next page: %w", err)
			}

			for _, nsg := range page.Value {
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
		}
	}

	return nil
}

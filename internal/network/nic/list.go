package nic

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

	client, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create NIC client: %w", err)
	}

	var nics []*armnetwork.Interface
	if resourceGroup != "" {
		pager := client.NewListPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get next page: %w", err)
			}
			nics = append(nics, page.Value...)
		}
	} else {
		pager := client.NewListAllPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get next page: %w", err)
			}
			nics = append(nics, page.Value...)
		}
	}

	// nic list historically defaulted to table output, unlike the global
	// default of json, so only honor an explicitly-passed format.
	format, _ := cmd.Flags().GetString("output")
	if !cmd.Flags().Changed("output") {
		format = "table"
	}
	if format != "table" {
		return output.PrintJSON(cmd, nics)
	}

	fmt.Printf("%-40s %-20s %-30s\n", "NAME", "LOCATION", "PRIVATE IP")
	fmt.Println("------------------------------------------------------------------------------------------------")
	for _, nic := range nics {
		printNIC(nic)
	}

	return nil
}

func printNIC(nic *armnetwork.Interface) {
	name := ""
	if nic.Name != nil {
		name = *nic.Name
	}

	location := ""
	if nic.Location != nil {
		location = *nic.Location
	}

	privateIP := ""
	if nic.Properties != nil && len(nic.Properties.IPConfigurations) > 0 {
		ipConfig := nic.Properties.IPConfigurations[0]
		if ipConfig.Properties != nil && ipConfig.Properties.PrivateIPAddress != nil {
			privateIP = *ipConfig.Properties.PrivateIPAddress
		}
	}

	fmt.Printf("%-40s %-20s %-30s\n", name, location, privateIP)
}

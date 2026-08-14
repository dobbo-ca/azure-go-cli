package publicip

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

	client, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create public IP client: %w", err)
	}

	var ips []*armnetwork.PublicIPAddress
	if resourceGroup != "" {
		pager := client.NewListPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get next page: %w", err)
			}
			ips = append(ips, page.Value...)
		}
	} else {
		pager := client.NewListAllPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get next page: %w", err)
			}
			ips = append(ips, page.Value...)
		}
	}

	// public-ip list historically defaulted to table output, unlike the
	// global default of json, so only honor an explicitly-passed format.
	format, _ := cmd.Flags().GetString("output")
	if !cmd.Flags().Changed("output") {
		format = "table"
	}
	if format != "table" {
		return output.PrintJSON(cmd, ips)
	}

	fmt.Printf("%-40s %-20s %-30s %-20s\n", "NAME", "LOCATION", "IP ADDRESS", "ALLOCATION")
	fmt.Println("--------------------------------------------------------------------------------------------------")
	for _, ip := range ips {
		printPublicIP(ip)
	}

	return nil
}

func printPublicIP(ip *armnetwork.PublicIPAddress) {
	name := ""
	if ip.Name != nil {
		name = *ip.Name
	}

	location := ""
	if ip.Location != nil {
		location = *ip.Location
	}

	ipAddress := ""
	allocation := ""
	if ip.Properties != nil {
		if ip.Properties.IPAddress != nil {
			ipAddress = *ip.Properties.IPAddress
		}
		if ip.Properties.PublicIPAllocationMethod != nil {
			allocation = string(*ip.Properties.PublicIPAllocationMethod)
		}
	}

	fmt.Printf("%-40s %-20s %-30s %-20s\n", name, location, ipAddress, allocation)
}

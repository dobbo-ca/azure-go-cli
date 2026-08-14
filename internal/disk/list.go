package disk

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
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

	client, err := armcompute.NewDisksClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create disk client: %w", err)
	}

	var disks []*armcompute.Disk
	if resourceGroup != "" {
		pager := client.NewListByResourceGroupPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get next page: %w", err)
			}
			disks = append(disks, page.Value...)
		}
	} else {
		pager := client.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get next page: %w", err)
			}
			disks = append(disks, page.Value...)
		}
	}

	// disk list historically defaulted to table output, unlike the global
	// default of json, so only honor an explicitly-passed format.
	format, _ := cmd.Flags().GetString("output")
	if !cmd.Flags().Changed("output") {
		format = "table"
	}
	if format != "table" {
		return output.PrintJSON(cmd, disks)
	}

	fmt.Printf("%-40s %-20s %-15s %-15s\n", "NAME", "LOCATION", "SIZE (GB)", "SKU")
	fmt.Println("--------------------------------------------------------------------------------------------")
	for _, disk := range disks {
		printDisk(disk)
	}

	return nil
}

func printDisk(disk *armcompute.Disk) {
	name := ""
	if disk.Name != nil {
		name = *disk.Name
	}

	location := ""
	if disk.Location != nil {
		location = *disk.Location
	}

	size := ""
	if disk.Properties != nil && disk.Properties.DiskSizeGB != nil {
		size = fmt.Sprintf("%d", *disk.Properties.DiskSizeGB)
	}

	sku := ""
	if disk.SKU != nil && disk.SKU.Name != nil {
		sku = string(*disk.SKU.Name)
	}

	fmt.Printf("%-40s %-20s %-15s %-15s\n", name, location, size, sku)
}

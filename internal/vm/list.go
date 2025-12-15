package vm

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
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

	client, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VM client: %w", err)
	}

	vms := make([]*armcompute.VirtualMachine, 0)

	if resourceGroup != "" {
		pager := client.NewListPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get next page: %w", err)
			}
			vms = append(vms, page.Value...)
		}
	} else {
		pager := client.NewListAllPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get next page: %w", err)
			}
			vms = append(vms, page.Value...)
		}
	}

	// Print simple table format
	fmt.Printf("%-40s %-30s %-15s %-20s\n", "NAME", "RESOURCE GROUP", "LOCATION", "VM SIZE")
	fmt.Println("-------------------------------------------------------------------------------------------------------------------")

	for _, vm := range vms {
		name := ""
		if vm.Name != nil {
			name = *vm.Name
		}

		location := ""
		if vm.Location != nil {
			location = *vm.Location
		}

		vmSize := ""
		if vm.Properties != nil && vm.Properties.HardwareProfile != nil && vm.Properties.HardwareProfile.VMSize != nil {
			vmSize = string(*vm.Properties.HardwareProfile.VMSize)
		}

		// Extract resource group from ID
		resourceGroup := ""
		if vm.ID != nil {
			// ID format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Compute/virtualMachines/{name}
			parts := parseResourceID(*vm.ID)
			if rg, ok := parts["resourceGroups"]; ok {
				resourceGroup = rg
			}
		}

		fmt.Printf("%-40s %-30s %-15s %-20s\n", name, resourceGroup, location, vmSize)
	}

	return nil
}

func parseResourceID(id string) map[string]string {
	parts := make(map[string]string)
	segments := splitResourceID(id)

	for i := 0; i < len(segments)-1; i += 2 {
		if i+1 < len(segments) {
			parts[segments[i]] = segments[i+1]
		}
	}

	return parts
}

func splitResourceID(id string) []string {
	var result []string
	current := ""

	for _, char := range id {
		if char == '/' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

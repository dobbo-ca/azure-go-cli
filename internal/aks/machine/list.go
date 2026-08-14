package machine

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, clusterName, nodepoolName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create machines client: %w", err)
	}

	pager := client.NewListPager(resourceGroup, clusterName, nodepoolName, nil)
	var machines []map[string]interface{}

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list machines: %w", err)
		}

		for _, machine := range page.Value {
			machines = append(machines, formatMachine(machine))
		}
	}

	return output.PrintJSON(cmd, machines)
}

func formatMachine(machine *armcontainerservice.Machine) map[string]interface{} {
	result := map[string]interface{}{
		"name": azure.GetStringValue(machine.Name),
	}

	if machine.Properties != nil {
		if machine.Properties.ResourceID != nil {
			result["resourceId"] = *machine.Properties.ResourceID
		}
		if machine.Properties.Network != nil {
			if machine.Properties.Network.IPAddresses != nil && len(machine.Properties.Network.IPAddresses) > 0 {
				ips := []string{}
				for _, ip := range machine.Properties.Network.IPAddresses {
					if ip.IP != nil {
						ips = append(ips, *ip.IP)
					}
				}
				result["ipAddresses"] = ips
			}
		}
	}

	return result
}

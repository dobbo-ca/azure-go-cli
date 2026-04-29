package subnet

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Update(ctx context.Context, cmd *cobra.Command, name, vnetName, resourceGroup string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewSubnetsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create subnets client: %w", err)
	}

	current, err := client.Get(ctx, resourceGroup, vnetName, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get subnet: %w", err)
	}

	if current.Properties == nil {
		current.Properties = &armnetwork.SubnetPropertiesFormat{}
	}
	props := current.Properties

	flags := cmd.Flags()

	if flags.Changed("network-security-group") {
		v, _ := flags.GetString("network-security-group")
		if v == "" {
			props.NetworkSecurityGroup = nil
		} else {
			props.NetworkSecurityGroup = &armnetwork.SecurityGroup{
				ID: to.Ptr(resolveNetworkResourceID(v, subscriptionID, resourceGroup, "networkSecurityGroups")),
			}
		}
	}

	if flags.Changed("route-table") {
		v, _ := flags.GetString("route-table")
		if v == "" {
			props.RouteTable = nil
		} else {
			props.RouteTable = &armnetwork.RouteTable{
				ID: to.Ptr(resolveNetworkResourceID(v, subscriptionID, resourceGroup, "routeTables")),
			}
		}
	}

	if flags.Changed("nat-gateway") {
		v, _ := flags.GetString("nat-gateway")
		if v == "" {
			props.NatGateway = nil
		} else {
			props.NatGateway = &armnetwork.SubResource{
				ID: to.Ptr(resolveNetworkResourceID(v, subscriptionID, resourceGroup, "natGateways")),
			}
		}
	}

	if flags.Changed("service-endpoints") {
		v, _ := flags.GetString("service-endpoints")
		if v == "" {
			props.ServiceEndpoints = nil
		} else {
			services := splitCSV(v)
			endpoints := make([]*armnetwork.ServiceEndpointPropertiesFormat, 0, len(services))
			for _, svc := range services {
				endpoints = append(endpoints, &armnetwork.ServiceEndpointPropertiesFormat{
					Service: to.Ptr(svc),
				})
			}
			props.ServiceEndpoints = endpoints
		}
	}

	if flags.Changed("delegations") {
		v, _ := flags.GetString("delegations")
		if v == "" {
			props.Delegations = nil
		} else {
			services := splitCSV(v)
			delegations := make([]*armnetwork.Delegation, 0, len(services))
			for _, svc := range services {
				delegations = append(delegations, &armnetwork.Delegation{
					Name: to.Ptr(svc),
					Properties: &armnetwork.ServiceDelegationPropertiesFormat{
						ServiceName: to.Ptr(svc),
					},
				})
			}
			props.Delegations = delegations
		}
	}

	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, vnetName, name, current.Subnet, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update subnet: %w", err)
	}

	if noWait {
		fmt.Printf("Started update of subnet '%s' in VNet '%s'\n", name, vnetName)
		return nil
	}

	fmt.Printf("Updating subnet '%s' in VNet '%s'...\n", name, vnetName)
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update subnet: %w", err)
	}

	return output.PrintJSON(cmd, result.Subnet)
}

// resolveNetworkResourceID returns v unchanged if it's already a full resource ID;
// otherwise constructs a Microsoft.Network resource ID using the given resource type
// (e.g., "networkSecurityGroups", "routeTables", "natGateways") in the same subscription
// and resource group as the subnet.
func resolveNetworkResourceID(v, subscriptionID, resourceGroup, resourceType string) string {
	if strings.HasPrefix(v, "/subscriptions/") {
		return v
	}
	return fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/%s/%s",
		subscriptionID, resourceGroup, resourceType, v,
	)
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

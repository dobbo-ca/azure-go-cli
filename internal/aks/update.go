package aks

import (
	"context"
	"fmt"
	"slices"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

type UpdateOptions struct {
	OutboundType       string
	ManagedOutboundIPs *int32
}

// applyUpdate mutates the cluster the way Python az aks update does.
func applyUpdate(mc *armcontainerservice.ManagedCluster, opts UpdateOptions) error {
	if opts.OutboundType == "" && opts.ManagedOutboundIPs == nil {
		return fmt.Errorf("please specify one or more of --outbound-type, --load-balancer-managed-outbound-ip-count")
	}
	if mc.Properties == nil {
		mc.Properties = &armcontainerservice.ManagedClusterProperties{}
	}
	if mc.Properties.NetworkProfile == nil {
		mc.Properties.NetworkProfile = &armcontainerservice.NetworkProfile{}
	}
	np := mc.Properties.NetworkProfile

	if opts.OutboundType != "" {
		t := armcontainerservice.OutboundType(opts.OutboundType)
		if !slices.Contains(armcontainerservice.PossibleOutboundTypeValues(), t) {
			return fmt.Errorf("invalid --outbound-type %q, allowed values: %v", opts.OutboundType, armcontainerservice.PossibleOutboundTypeValues())
		}
		np.OutboundType = &t
		// Python az drops profiles the new outbound type rejects.
		if t != armcontainerservice.OutboundTypeLoadBalancer {
			if opts.ManagedOutboundIPs != nil {
				return fmt.Errorf("--load-balancer-managed-outbound-ip-count needs --outbound-type loadBalancer")
			}
			np.LoadBalancerProfile = nil
		}
		if t != armcontainerservice.OutboundTypeManagedNATGateway {
			np.NatGatewayProfile = nil
		}
	}

	if opts.ManagedOutboundIPs != nil {
		count := *opts.ManagedOutboundIPs
		if count < 1 || count > 100 {
			return fmt.Errorf("--load-balancer-managed-outbound-ip-count must be between 1 and 100")
		}
		if np.LoadBalancerProfile == nil {
			np.LoadBalancerProfile = &armcontainerservice.ManagedClusterLoadBalancerProfile{}
		}
		lb := np.LoadBalancerProfile
		if lb.ManagedOutboundIPs == nil {
			lb.ManagedOutboundIPs = &armcontainerservice.ManagedClusterLoadBalancerProfileManagedOutboundIPs{}
		}
		lb.ManagedOutboundIPs.Count = to.Ptr(count)
		// Managed IPs replace any user-supplied IPs or prefixes.
		lb.OutboundIPs = nil
		lb.OutboundIPPrefixes = nil
	}
	return nil
}

func Update(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, opts UpdateOptions, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create AKS client: %w", err)
	}

	got, err := client.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	mc := got.ManagedCluster
	if err := applyUpdate(&mc, opts); err != nil {
		return err
	}

	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, mc, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update: %w", err)
	}

	if noWait {
		return nil
	}

	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update cluster: %w", err)
	}

	return output.PrintJSON(cmd, res.ManagedCluster)
}

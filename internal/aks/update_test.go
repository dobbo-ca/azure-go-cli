package aks

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
)

func TestApplyUpdate(t *testing.T) {
	mc := armcontainerservice.ManagedCluster{Properties: &armcontainerservice.ManagedClusterProperties{
		NetworkProfile: &armcontainerservice.NetworkProfile{
			OutboundType: to.Ptr(armcontainerservice.OutboundTypeUserDefinedRouting),
			LoadBalancerProfile: &armcontainerservice.ManagedClusterLoadBalancerProfile{
				OutboundIPs: &armcontainerservice.ManagedClusterLoadBalancerProfileOutboundIPs{},
			},
		},
	}}
	if err := applyUpdate(&mc, UpdateOptions{OutboundType: "loadBalancer", ManagedOutboundIPs: to.Ptr[int32](1)}); err != nil {
		t.Fatal(err)
	}
	np := mc.Properties.NetworkProfile
	if *np.OutboundType != armcontainerservice.OutboundTypeLoadBalancer {
		t.Errorf("outbound type = %v", *np.OutboundType)
	}
	if *np.LoadBalancerProfile.ManagedOutboundIPs.Count != 1 || np.LoadBalancerProfile.OutboundIPs != nil {
		t.Errorf("load balancer profile not updated: %+v", np.LoadBalancerProfile)
	}

	if err := applyUpdate(&mc, UpdateOptions{OutboundType: "userDefinedRouting"}); err != nil {
		t.Fatal(err)
	}
	if np.LoadBalancerProfile != nil {
		t.Errorf("load balancer profile kept for userDefinedRouting: %+v", np.LoadBalancerProfile)
	}

	for _, bad := range []UpdateOptions{{}, {OutboundType: "bogus"}, {ManagedOutboundIPs: to.Ptr[int32](0)}, {OutboundType: "userDefinedRouting", ManagedOutboundIPs: to.Ptr[int32](1)}} {
		if err := applyUpdate(&armcontainerservice.ManagedCluster{}, bad); err == nil {
			t.Errorf("expected error for %+v", bad)
		}
	}
}

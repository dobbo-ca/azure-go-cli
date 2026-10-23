package routetable

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/query"
)

func TestPyShapeQuery(t *testing.T) {
	tables := []*armnetwork.RouteTable{{
		ID:   to.Ptr("/subscriptions/s/resourceGroups/rg1/providers/Microsoft.Network/routeTables/vnet-rt"),
		Name: to.Ptr("vnet-rt"),
		Tags: map[string]*string{"env": to.Ptr("dev")},
		Properties: &armnetwork.RouteTablePropertiesFormat{
			DisableBgpRoutePropagation: to.Ptr(true),
			Routes: []*armnetwork.Route{{
				ID:   to.Ptr("/subscriptions/s/resourceGroups/rg1/providers/Microsoft.Network/routeTables/vnet-rt/routes/r1"),
				Name: to.Ptr("r1"),
				Properties: &armnetwork.RoutePropertiesFormat{
					AddressPrefix:    to.Ptr("0.0.0.0/0"),
					NextHopType:      to.Ptr(armnetwork.RouteNextHopTypeVirtualAppliance),
					NextHopIPAddress: to.Ptr("10.0.0.4"),
				},
			}},
		},
	}}
	shaped, err := pyShape(tables)
	if err != nil {
		t.Fatal(err)
	}
	got, err := query.ApplyJMESPath(shaped, "[?name=='vnet-rt'].{rg:resourceGroup, disableBgp:disableBgpRoutePropagation, routes:routes[].{p:addressPrefix,t:nextHopType,ip:nextHopIpAddress}, tags:tags}")
	if err != nil {
		t.Fatal(err)
	}
	row := got.([]interface{})[0].(map[string]interface{})
	route := row["routes"].([]interface{})[0].(map[string]interface{})
	if row["rg"] != "rg1" || row["disableBgp"] != true || route["p"] != "0.0.0.0/0" || route["t"] != "VirtualAppliance" || route["ip"] != "10.0.0.4" {
		t.Errorf("unexpected result: %#v", row)
	}
}

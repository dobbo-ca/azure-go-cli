package network

import (
  "github.com/cdobbyn/azure-go-cli/internal/network/bastion"
  "github.com/cdobbyn/azure-go-cli/internal/network/lb"
  "github.com/cdobbyn/azure-go-cli/internal/network/natgateway"
  "github.com/cdobbyn/azure-go-cli/internal/network/nsg"
  "github.com/cdobbyn/azure-go-cli/internal/network/peering"
  "github.com/cdobbyn/azure-go-cli/internal/network/privateendpoint"
  "github.com/cdobbyn/azure-go-cli/internal/network/subnet"
  "github.com/cdobbyn/azure-go-cli/internal/network/vnet"
  "github.com/cdobbyn/azure-go-cli/internal/network/vpngateway"
  "github.com/spf13/cobra"
)

func NewNetworkCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "network",
    Short: "Manage Azure network resources",
    Long:  "Commands to manage Azure network resources",
  }

  cmd.AddCommand(
    bastion.NewBastionCommand(),
    vnet.NewVNetCommand(),
    subnet.NewSubnetCommand(),
    peering.NewPeeringCommand(),
    natgateway.NewNatGatewayCommand(),
    vpngateway.NewVpnGatewayCommand(),
    lb.NewLoadBalancerCommand(),
    privateendpoint.NewPrivateEndpointCommand(),
    nsg.NewNsgCommand(),
  )
  return cmd
}

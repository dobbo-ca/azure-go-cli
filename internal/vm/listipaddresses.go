package vm

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

type ipInfo struct {
	PrivateIPAddress string `json:"privateIpAddress,omitempty"`
	PublicIPAddress  string `json:"publicIpAddress,omitempty"`
	PublicIPName     string `json:"publicIpName,omitempty"`
}

func ListIPAddresses(ctx context.Context, cmd *cobra.Command, resourceGroup, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VM client: %w", err)
	}
	nicClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create network interfaces client: %w", err)
	}
	pipClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create public IP client: %w", err)
	}

	vm, err := vmClient.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get virtual machine: %w", err)
	}

	result := []ipInfo{}
	if vm.Properties == nil || vm.Properties.NetworkProfile == nil {
		return output.PrintJSON(cmd, result)
	}

	for _, nicRef := range vm.Properties.NetworkProfile.NetworkInterfaces {
		if nicRef == nil || nicRef.ID == nil {
			continue
		}
		parts := parseResourceID(*nicRef.ID)
		nicRG := parts["resourceGroups"]
		nicName := parts["networkInterfaces"]
		if nicRG == "" || nicName == "" {
			continue
		}
		nic, err := nicClient.Get(ctx, nicRG, nicName, nil)
		if err != nil {
			return fmt.Errorf("failed to get network interface '%s': %w", nicName, err)
		}
		if nic.Properties == nil {
			continue
		}
		for _, ipCfg := range nic.Properties.IPConfigurations {
			if ipCfg == nil || ipCfg.Properties == nil {
				continue
			}
			info := ipInfo{}
			if ipCfg.Properties.PrivateIPAddress != nil {
				info.PrivateIPAddress = *ipCfg.Properties.PrivateIPAddress
			}
			if ipCfg.Properties.PublicIPAddress != nil && ipCfg.Properties.PublicIPAddress.ID != nil {
				pipParts := parseResourceID(*ipCfg.Properties.PublicIPAddress.ID)
				pipRG := pipParts["resourceGroups"]
				pipName := pipParts["publicIPAddresses"]
				info.PublicIPName = pipName
				if pipRG != "" && pipName != "" {
					pip, err := pipClient.Get(ctx, pipRG, pipName, nil)
					if err == nil && pip.Properties != nil && pip.Properties.IPAddress != nil {
						info.PublicIPAddress = *pip.Properties.IPAddress
					}
				}
			}
			result = append(result, info)
		}
	}
	return output.PrintJSON(cmd, result)
}

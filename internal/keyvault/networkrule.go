package keyvault

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// subnetID builds a subnet resource id from --subnet and --vnet-name, as
// validate_subnet does. A --subnet that is already an id passes through.
func subnetID(subnet, vnetName, resourceGroup string) (string, error) {
	if subnet == "" || strings.HasPrefix(subnet, "/subscriptions/") {
		return subnet, nil
	}
	if vnetName == "" {
		return "", fmt.Errorf("--vnet-name must be supplied when --subnet is a name")
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
		subscriptionID, resourceGroup, vnetName, subnet), nil
}

// prefixOf reads an IP rule value as a prefix. A bare address counts as a
// single-address prefix, which is how Python's ip_network reads it.
func prefixOf(value string) (netip.Prefix, error) {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix, nil
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("'%s' is not a valid IPv4 address or CIDR range", value)
	}
	return netip.PrefixFrom(address, address.BitLen()), nil
}

// AddNetworkRule adds IP and subnet rules to a vault's network ACLs
// (custom.py:861).
func AddNetworkRule(ctx context.Context, cmd *cobra.Command, vaultName, resourceGroup string, ipAddresses []string, subnet, vnetName string) error {
	subnet, err := subnetID(subnet, vnetName, resourceGroup)
	if err != nil {
		return err
	}
	client, err := vaultsClient()
	if err != nil {
		return err
	}
	vault, err := client.Get(ctx, resourceGroup, vaultName, nil)
	if err != nil {
		return fmt.Errorf("failed to get key vault: %w", err)
	}
	props := vault.Properties
	if props.NetworkACLs == nil {
		props.NetworkACLs = &armkeyvault.NetworkRuleSet{
			Bypass:        to.Ptr(armkeyvault.NetworkRuleBypassOptionsAzureServices),
			DefaultAction: to.Ptr(armkeyvault.NetworkRuleActionAllow),
		}
	}
	rules := props.NetworkACLs

	if subnet == "" && len(ipAddresses) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "No subnet or ip address supplied.")
	}

	changed := false
	if subnet != "" {
		found := false
		for _, rule := range rules.VirtualNetworkRules {
			if rule.ID != nil && strings.EqualFold(*rule.ID, subnet) {
				found = true
				break
			}
		}
		if !found {
			rules.VirtualNetworkRules = append(rules.VirtualNetworkRules, &armkeyvault.VirtualNetworkRule{ID: to.Ptr(subnet)})
			changed = true
		}
	}

	for _, address := range ipAddresses {
		candidate, err := prefixOf(address)
		if err != nil {
			return err
		}
		overlaps := false
		for _, rule := range rules.IPRules {
			if rule.Value == nil {
				continue
			}
			existing, err := prefixOf(*rule.Value)
			if err != nil {
				continue
			}
			// azure-cli skips an address that overlaps an existing rule
			// (custom.py:897).
			if existing.Overlaps(candidate) {
				fmt.Fprintf(cmd.ErrOrStderr(), "IP/CIDR %s overlaps with %s, which exists already. Not adding duplicates.\n", address, *rule.Value)
				overlaps = true
				break
			}
		}
		if !overlaps {
			rules.IPRules = append(rules.IPRules, &armkeyvault.IPRule{Value: to.Ptr(address)})
			changed = true
		}
	}

	if !changed {
		return output.PrintJSON(cmd, vault.Vault)
	}
	return updateVault(ctx, cmd, client, resourceGroup, vaultName, vault.Vault)
}

// RemoveNetworkRule removes IP and subnet rules from a vault's network ACLs
// (custom.py:968).
func RemoveNetworkRule(ctx context.Context, cmd *cobra.Command, vaultName, resourceGroup string, ipAddresses []string, subnet, vnetName string) error {
	subnet, err := subnetID(subnet, vnetName, resourceGroup)
	if err != nil {
		return err
	}
	client, err := vaultsClient()
	if err != nil {
		return err
	}
	vault, err := client.Get(ctx, resourceGroup, vaultName, nil)
	if err != nil {
		return fmt.Errorf("failed to get key vault: %w", err)
	}
	props := vault.Properties
	if props.NetworkACLs == nil {
		return output.PrintJSON(cmd, vault.Vault)
	}
	rules := props.NetworkACLs
	changed := false

	if subnet != "" && len(rules.VirtualNetworkRules) > 0 {
		kept := make([]*armkeyvault.VirtualNetworkRule, 0, len(rules.VirtualNetworkRules))
		for _, rule := range rules.VirtualNetworkRules {
			if rule.ID != nil && strings.EqualFold(*rule.ID, subnet) {
				continue
			}
			kept = append(kept, rule)
		}
		if len(kept) != len(rules.VirtualNetworkRules) {
			rules.VirtualNetworkRules = kept
			changed = true
		}
	}

	if len(ipAddresses) > 0 && len(rules.IPRules) > 0 {
		remove := make([]netip.Prefix, 0, len(ipAddresses))
		for _, address := range ipAddresses {
			prefix, err := prefixOf(address)
			if err != nil {
				return err
			}
			remove = append(remove, prefix)
		}
		kept := make([]*armkeyvault.IPRule, 0, len(rules.IPRules))
		for _, rule := range rules.IPRules {
			drop := false
			if rule.Value != nil {
				if existing, err := prefixOf(*rule.Value); err == nil {
					for _, prefix := range remove {
						// Removal matches exactly, unlike the overlap
						// test on add (custom.py:992).
						if existing == prefix {
							drop = true
							break
						}
					}
				}
			}
			if !drop {
				kept = append(kept, rule)
			}
		}
		if len(kept) != len(rules.IPRules) {
			rules.IPRules = kept
			changed = true
		}
	}

	if !changed {
		return output.PrintJSON(cmd, vault.Vault)
	}
	return updateVault(ctx, cmd, client, resourceGroup, vaultName, vault.Vault)
}

// ListNetworkRules prints the network ACLs of a vault (custom.py:1045).
func ListNetworkRules(ctx context.Context, cmd *cobra.Command, vaultName, resourceGroup string) error {
	client, err := vaultsClient()
	if err != nil {
		return err
	}
	vault, err := client.Get(ctx, resourceGroup, vaultName, nil)
	if err != nil {
		return fmt.Errorf("failed to get key vault: %w", err)
	}
	return output.PrintJSON(cmd, vault.Properties.NetworkACLs)
}

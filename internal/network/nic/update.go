package nic

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

func Update(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, dnsServers []string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create NIC client: %w", err)
	}

	current, err := client.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get NIC: %w", err)
	}

	if current.Properties == nil {
		current.Properties = &armnetwork.InterfacePropertiesFormat{}
	}
	props := current.Properties

	flags := cmd.Flags()

	if flags.Changed("ip-forwarding") {
		v, _ := flags.GetBool("ip-forwarding")
		props.EnableIPForwarding = to.Ptr(v)
	}

	if flags.Changed("dns-servers") {
		if props.DNSSettings == nil {
			props.DNSSettings = &armnetwork.InterfaceDNSSettings{}
		}
		servers := make([]*string, 0, len(dnsServers))
		for _, s := range dnsServers {
			servers = append(servers, to.Ptr(s))
		}
		props.DNSSettings.DNSServers = servers
	}

	if flags.Changed("tags") {
		tags, _ := flags.GetStringToString("tags")
		azureTags := make(map[string]*string)
		for k, v := range tags {
			azureTags[k] = to.Ptr(v)
		}
		current.Tags = azureTags
	}

	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, current.Interface, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update NIC: %w", err)
	}

	if noWait {
		fmt.Printf("Started update of network interface '%s'\n", name)
		return nil
	}

	fmt.Printf("Updating network interface '%s'...\n", name)
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update NIC: %w", err)
	}

	return output.PrintJSON(cmd, result.Interface)
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

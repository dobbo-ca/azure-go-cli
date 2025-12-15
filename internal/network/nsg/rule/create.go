package rule

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

type RuleParams struct {
	Priority                 int32
	Direction                string
	Access                   string
	Protocol                 string
	SourceAddressPrefix      string
	SourcePortRange          string
	DestinationAddressPrefix string
	DestinationPortRange     string
	Description              string
}

func Create(ctx context.Context, cmd *cobra.Command, name, nsgName, resourceGroup string, params RuleParams) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewSecurityRulesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create security rules client: %w", err)
	}

	// Parse direction
	var direction armnetwork.SecurityRuleDirection
	switch strings.ToLower(params.Direction) {
	case "inbound":
		direction = armnetwork.SecurityRuleDirectionInbound
	case "outbound":
		direction = armnetwork.SecurityRuleDirectionOutbound
	default:
		return fmt.Errorf("invalid direction: %s (must be Inbound or Outbound)", params.Direction)
	}

	// Parse access
	var access armnetwork.SecurityRuleAccess
	switch strings.ToLower(params.Access) {
	case "allow":
		access = armnetwork.SecurityRuleAccessAllow
	case "deny":
		access = armnetwork.SecurityRuleAccessDeny
	default:
		return fmt.Errorf("invalid access: %s (must be Allow or Deny)", params.Access)
	}

	// Parse protocol
	var protocol armnetwork.SecurityRuleProtocol
	switch strings.ToLower(params.Protocol) {
	case "tcp":
		protocol = armnetwork.SecurityRuleProtocolTCP
	case "udp":
		protocol = armnetwork.SecurityRuleProtocolUDP
	case "icmp":
		protocol = armnetwork.SecurityRuleProtocolIcmp
	case "*", "any":
		protocol = armnetwork.SecurityRuleProtocolAsterisk
	default:
		return fmt.Errorf("invalid protocol: %s (must be TCP, UDP, ICMP, or *)", params.Protocol)
	}

	rule := armnetwork.SecurityRule{
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Priority:                 to.Ptr(params.Priority),
			Direction:                to.Ptr(direction),
			Access:                   to.Ptr(access),
			Protocol:                 to.Ptr(protocol),
			SourceAddressPrefix:      to.Ptr(params.SourceAddressPrefix),
			SourcePortRange:          to.Ptr(params.SourcePortRange),
			DestinationAddressPrefix: to.Ptr(params.DestinationAddressPrefix),
			DestinationPortRange:     to.Ptr(params.DestinationPortRange),
		},
	}

	if params.Description != "" {
		rule.Properties.Description = to.Ptr(params.Description)
	}

	fmt.Printf("Creating security rule '%s'...\n", name)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, nsgName, name, rule, nil)
	if err != nil {
		return fmt.Errorf("failed to create security rule: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete security rule creation: %w", err)
	}

	fmt.Printf("Created security rule '%s'\n", name)
	return output.PrintJSON(cmd, result.SecurityRule)
}

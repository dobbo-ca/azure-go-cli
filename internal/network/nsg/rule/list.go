package rule

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, nsgName, resourceGroup string) error {
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

	pager := client.NewListPager(resourceGroup, nsgName, nil)

	var rules []*armnetwork.SecurityRule
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get next page: %w", err)
		}
		rules = append(rules, page.Value...)
	}

	// rule list historically defaulted to table output, unlike the global
	// default of json, so only honor an explicitly-passed format.
	format, _ := cmd.Flags().GetString("output")
	if !cmd.Flags().Changed("output") {
		format = "table"
	}
	if format != "table" {
		return output.PrintJSON(cmd, rules)
	}

	fmt.Printf("%-30s %-10s %-10s %-10s %-20s %-20s\n", "NAME", "PRIORITY", "DIRECTION", "ACCESS", "PROTOCOL", "SOURCE")
	fmt.Println("----------------------------------------------------------------------------------------------------------------")
	for _, rule := range rules {
		name := ""
		if rule.Name != nil {
			name = *rule.Name
		}

		priority := ""
		direction := ""
		access := ""
		protocol := ""
		source := ""

		if rule.Properties != nil {
			if rule.Properties.Priority != nil {
				priority = fmt.Sprintf("%d", *rule.Properties.Priority)
			}
			if rule.Properties.Direction != nil {
				direction = string(*rule.Properties.Direction)
			}
			if rule.Properties.Access != nil {
				access = string(*rule.Properties.Access)
			}
			if rule.Properties.Protocol != nil {
				protocol = string(*rule.Properties.Protocol)
			}
			if rule.Properties.SourceAddressPrefix != nil {
				source = *rule.Properties.SourceAddressPrefix
			} else if len(rule.Properties.SourceAddressPrefixes) > 0 {
				source = *rule.Properties.SourceAddressPrefixes[0] + "..."
			}
		}

		fmt.Printf("%-30s %-10s %-10s %-10s %-20s %-20s\n", name, priority, direction, access, protocol, source)
	}

	return nil
}

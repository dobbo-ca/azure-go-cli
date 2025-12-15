package rule

import (
	"context"

	"github.com/spf13/cobra"
)

func NewRuleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rule",
		Short: "Manage network security group rules",
		Long:  "Commands to manage rules in network security groups",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List security rules in a network security group",
		RunE: func(cmd *cobra.Command, args []string) error {
			nsgName, _ := cmd.Flags().GetString("nsg-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), nsgName, resourceGroup)
		},
	}
	listCmd.Flags().String("nsg-name", "", "Network security group name")
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.MarkFlagRequired("nsg-name")
	listCmd.MarkFlagRequired("resource-group")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a security rule",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			nsgName, _ := cmd.Flags().GetString("nsg-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, name, nsgName, resourceGroup)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Security rule name")
	showCmd.Flags().String("nsg-name", "", "Network security group name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("nsg-name")
	showCmd.MarkFlagRequired("resource-group")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a security rule",
		Long: `Create a security rule in a network security group.

Priority ranges from 100 to 4096 (lower numbers have higher priority).
Direction must be Inbound or Outbound.
Access must be Allow or Deny.
Protocol must be TCP, UDP, ICMP, or * (any).

Example:
  az network nsg rule create --name allow-ssh --nsg-name my-nsg --resource-group my-rg \
    --priority 1000 --direction Inbound --access Allow --protocol TCP \
    --source-address-prefix "*" --source-port-range "*" \
    --destination-address-prefix "*" --destination-port-range 22`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			nsgName, _ := cmd.Flags().GetString("nsg-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			priority, _ := cmd.Flags().GetInt32("priority")
			direction, _ := cmd.Flags().GetString("direction")
			access, _ := cmd.Flags().GetString("access")
			protocol, _ := cmd.Flags().GetString("protocol")
			sourceAddr, _ := cmd.Flags().GetString("source-address-prefix")
			sourcePort, _ := cmd.Flags().GetString("source-port-range")
			destAddr, _ := cmd.Flags().GetString("destination-address-prefix")
			destPort, _ := cmd.Flags().GetString("destination-port-range")
			description, _ := cmd.Flags().GetString("description")

			params := RuleParams{
				Priority:                 priority,
				Direction:                direction,
				Access:                   access,
				Protocol:                 protocol,
				SourceAddressPrefix:      sourceAddr,
				SourcePortRange:          sourcePort,
				DestinationAddressPrefix: destAddr,
				DestinationPortRange:     destPort,
				Description:              description,
			}

			return Create(context.Background(), cmd, name, nsgName, resourceGroup, params)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Security rule name")
	createCmd.Flags().String("nsg-name", "", "Network security group name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().Int32("priority", 0, "Priority (100-4096, lower numbers have higher priority)")
	createCmd.Flags().String("direction", "", "Direction: Inbound or Outbound")
	createCmd.Flags().String("access", "", "Access: Allow or Deny")
	createCmd.Flags().String("protocol", "", "Protocol: TCP, UDP, ICMP, or *")
	createCmd.Flags().String("source-address-prefix", "*", "Source address prefix (CIDR or * for any)")
	createCmd.Flags().String("source-port-range", "*", "Source port or range (* for any)")
	createCmd.Flags().String("destination-address-prefix", "*", "Destination address prefix (CIDR or * for any)")
	createCmd.Flags().String("destination-port-range", "*", "Destination port or range (* for any)")
	createCmd.Flags().String("description", "", "Rule description")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("nsg-name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("priority")
	createCmd.MarkFlagRequired("direction")
	createCmd.MarkFlagRequired("access")
	createCmd.MarkFlagRequired("protocol")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a security rule",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			nsgName, _ := cmd.Flags().GetString("nsg-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), name, nsgName, resourceGroup, noWait)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Security rule name")
	deleteCmd.Flags().String("nsg-name", "", "Network security group name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("nsg-name")
	deleteCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
	return cmd
}

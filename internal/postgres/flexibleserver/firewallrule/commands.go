package firewallrule

import (
	"context"

	"github.com/spf13/cobra"
)

func NewFirewallRuleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firewall-rule",
		Short: "Manage PostgreSQL flexible server firewall rules",
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a firewall rule",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			rule, _ := cmd.Flags().GetString("rule-name")
			startIP, _ := cmd.Flags().GetString("start-ip-address")
			endIP, _ := cmd.Flags().GetString("end-ip-address")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Create(context.Background(), cmd, rg, server, rule, startIP, endIP, noWait)
		},
	}
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().String("server-name", "", "Flexible server name")
	createCmd.Flags().StringP("rule-name", "n", "", "Firewall rule name")
	createCmd.Flags().String("start-ip-address", "", "Start IP address of the firewall rule")
	createCmd.Flags().String("end-ip-address", "", "End IP address of the firewall rule")
	createCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("server-name")
	createCmd.MarkFlagRequired("rule-name")
	createCmd.MarkFlagRequired("start-ip-address")
	createCmd.MarkFlagRequired("end-ip-address")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a firewall rule",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			rule, _ := cmd.Flags().GetString("rule-name")
			startIP, _ := cmd.Flags().GetString("start-ip-address")
			endIP, _ := cmd.Flags().GetString("end-ip-address")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Update(context.Background(), cmd, rg, server, rule, startIP, endIP, noWait)
		},
	}
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().String("server-name", "", "Flexible server name")
	updateCmd.Flags().StringP("rule-name", "n", "", "Firewall rule name")
	updateCmd.Flags().String("start-ip-address", "", "Start IP address of the firewall rule")
	updateCmd.Flags().String("end-ip-address", "", "End IP address of the firewall rule")
	updateCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	updateCmd.MarkFlagRequired("resource-group")
	updateCmd.MarkFlagRequired("server-name")
	updateCmd.MarkFlagRequired("rule-name")
	updateCmd.MarkFlagRequired("start-ip-address")
	updateCmd.MarkFlagRequired("end-ip-address")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a firewall rule",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			rule, _ := cmd.Flags().GetString("rule-name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), cmd, rg, server, rule, noWait)
		},
	}
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().String("server-name", "", "Flexible server name")
	deleteCmd.Flags().StringP("rule-name", "n", "", "Firewall rule name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("server-name")
	deleteCmd.MarkFlagRequired("rule-name")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List firewall rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			return List(context.Background(), cmd, rg, server)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.Flags().String("server-name", "", "Flexible server name")
	listCmd.MarkFlagRequired("resource-group")
	listCmd.MarkFlagRequired("server-name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a firewall rule",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			rule, _ := cmd.Flags().GetString("rule-name")
			return Show(context.Background(), cmd, rg, server, rule)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("server-name", "", "Flexible server name")
	showCmd.Flags().StringP("rule-name", "n", "", "Firewall rule name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("server-name")
	showCmd.MarkFlagRequired("rule-name")

	cmd.AddCommand(createCmd, updateCmd, deleteCmd, listCmd, showCmd)
	return cmd
}

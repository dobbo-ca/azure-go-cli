package nic

import (
	"context"

	"github.com/spf13/cobra"
)

func NewNicCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nic",
		Short: "Manage VMSS network interfaces",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all network interfaces in a virtual machine scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			return List(context.Background(), cmd, resourceGroup, vmssName)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.Flags().String("vmss-name", "", "Virtual machine scale set name")
	listCmd.MarkFlagRequired("resource-group")
	listCmd.MarkFlagRequired("vmss-name")

	listVMNicsCmd := &cobra.Command{
		Use:   "list-vm-nics",
		Short: "List all network interfaces of a VM instance in a virtual machine scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			instanceID, _ := cmd.Flags().GetString("instance-id")
			return ListVMNics(context.Background(), cmd, resourceGroup, vmssName, instanceID)
		},
	}
	listVMNicsCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listVMNicsCmd.Flags().String("vmss-name", "", "Virtual machine scale set name")
	listVMNicsCmd.Flags().String("instance-id", "", "Virtual machine instance ID")
	listVMNicsCmd.MarkFlagRequired("resource-group")
	listVMNicsCmd.MarkFlagRequired("vmss-name")
	listVMNicsCmd.MarkFlagRequired("instance-id")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a network interface in a virtual machine scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			instanceID, _ := cmd.Flags().GetString("instance-id")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, resourceGroup, vmssName, instanceID, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("vmss-name", "", "Virtual machine scale set name")
	showCmd.Flags().String("instance-id", "", "Virtual machine instance ID")
	showCmd.Flags().StringP("name", "n", "", "Network interface name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("vmss-name")
	showCmd.MarkFlagRequired("instance-id")
	showCmd.MarkFlagRequired("name")

	cmd.AddCommand(listCmd, listVMNicsCmd, showCmd)
	return cmd
}

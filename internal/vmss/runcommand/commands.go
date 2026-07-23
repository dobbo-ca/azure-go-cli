package runcommand

import (
	"context"

	"github.com/spf13/cobra"
)

func NewRunCommandCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run-command",
		Short: "Manage VMSS run commands",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List run commands on a VMSS instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			instanceID, _ := cmd.Flags().GetString("instance-id")
			return List(context.Background(), cmd, resourceGroup, vmssName, instanceID)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.Flags().String("vmss-name", "", "Virtual machine scale set name")
	listCmd.Flags().String("instance-id", "", "Instance ID of the scale set VM")
	listCmd.MarkFlagRequired("resource-group")
	listCmd.MarkFlagRequired("vmss-name")
	listCmd.MarkFlagRequired("instance-id")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a run command on a VMSS instance",
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
	showCmd.Flags().String("instance-id", "", "Instance ID of the scale set VM")
	showCmd.Flags().StringP("name", "n", "", "Run command name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("vmss-name")
	showCmd.MarkFlagRequired("instance-id")
	showCmd.MarkFlagRequired("name")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a run command on a VMSS instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			instanceID, _ := cmd.Flags().GetString("instance-id")
			name, _ := cmd.Flags().GetString("name")
			location, _ := cmd.Flags().GetString("location")
			script, _ := cmd.Flags().GetString("script")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Create(context.Background(), cmd, resourceGroup, vmssName, instanceID, name, location, script, noWait)
		},
	}
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().String("vmss-name", "", "Virtual machine scale set name")
	createCmd.Flags().String("instance-id", "", "Instance ID of the scale set VM")
	createCmd.Flags().StringP("name", "n", "", "Run command name")
	createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	createCmd.Flags().String("script", "", "Script content to execute on the VM")
	createCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("vmss-name")
	createCmd.MarkFlagRequired("instance-id")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("location")
	createCmd.MarkFlagRequired("script")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a run command on a VMSS instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			instanceID, _ := cmd.Flags().GetString("instance-id")
			name, _ := cmd.Flags().GetString("name")
			script, _ := cmd.Flags().GetString("script")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Update(context.Background(), cmd, resourceGroup, vmssName, instanceID, name, script, noWait)
		},
	}
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().String("vmss-name", "", "Virtual machine scale set name")
	updateCmd.Flags().String("instance-id", "", "Instance ID of the scale set VM")
	updateCmd.Flags().StringP("name", "n", "", "Run command name")
	updateCmd.Flags().String("script", "", "Script content to execute on the VM")
	updateCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	updateCmd.MarkFlagRequired("resource-group")
	updateCmd.MarkFlagRequired("vmss-name")
	updateCmd.MarkFlagRequired("instance-id")
	updateCmd.MarkFlagRequired("name")
	updateCmd.MarkFlagRequired("script")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a run command on a VMSS instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			instanceID, _ := cmd.Flags().GetString("instance-id")
			name, _ := cmd.Flags().GetString("name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), cmd, resourceGroup, vmssName, instanceID, name, noWait)
		},
	}
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().String("vmss-name", "", "Virtual machine scale set name")
	deleteCmd.Flags().String("instance-id", "", "Instance ID of the scale set VM")
	deleteCmd.Flags().StringP("name", "n", "", "Run command name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("vmss-name")
	deleteCmd.MarkFlagRequired("instance-id")
	deleteCmd.MarkFlagRequired("name")

	invokeCmd := &cobra.Command{
		Use:   "invoke",
		Short: "Invoke a run command on a VMSS instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			instanceID, _ := cmd.Flags().GetString("instance-id")
			commandID, _ := cmd.Flags().GetString("command-id")
			script, _ := cmd.Flags().GetString("script")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Invoke(context.Background(), cmd, resourceGroup, vmssName, instanceID, commandID, script, noWait)
		},
	}
	invokeCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	invokeCmd.Flags().String("vmss-name", "", "Virtual machine scale set name")
	invokeCmd.Flags().String("instance-id", "", "Instance ID of the scale set VM")
	invokeCmd.Flags().String("command-id", "", "Predefined run command ID (e.g., RunShellScript, RunPowerShellScript)")
	invokeCmd.Flags().String("script", "", "Script to execute, overriding the default script of the command")
	invokeCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	invokeCmd.MarkFlagRequired("resource-group")
	invokeCmd.MarkFlagRequired("vmss-name")
	invokeCmd.MarkFlagRequired("instance-id")
	invokeCmd.MarkFlagRequired("command-id")

	cmd.AddCommand(listCmd, showCmd, createCmd, updateCmd, deleteCmd, invokeCmd)

	return cmd
}

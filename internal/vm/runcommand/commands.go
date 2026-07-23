package runcommand

import (
	"context"

	"github.com/spf13/cobra"
)

func NewRunCommandCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run-command",
		Short: "Manage VM run commands",
	}

	invokeCmd := &cobra.Command{
		Use:   "invoke",
		Short: "Invoke a run command on a VM",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmName, _ := cmd.Flags().GetString("vm-name")
			commandID, _ := cmd.Flags().GetString("command-id")
			script, _ := cmd.Flags().GetString("script")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Invoke(context.Background(), cmd, resourceGroup, vmName, commandID, script, noWait)
		},
	}
	invokeCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	invokeCmd.Flags().String("vm-name", "", "VM name")
	invokeCmd.Flags().String("command-id", "", "Predefined run command ID (e.g., RunShellScript, RunPowerShellScript)")
	invokeCmd.Flags().String("script", "", "Script to execute")
	invokeCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	invokeCmd.MarkFlagRequired("resource-group")
	invokeCmd.MarkFlagRequired("vm-name")
	invokeCmd.MarkFlagRequired("command-id")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List run commands on a VM",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmName, _ := cmd.Flags().GetString("vm-name")
			return List(context.Background(), cmd, resourceGroup, vmName)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.Flags().String("vm-name", "", "VM name")
	listCmd.MarkFlagRequired("resource-group")
	listCmd.MarkFlagRequired("vm-name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a run command on a VM",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmName, _ := cmd.Flags().GetString("vm-name")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, resourceGroup, vmName, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("vm-name", "", "VM name")
	showCmd.Flags().StringP("name", "n", "", "Run command name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("vm-name")
	showCmd.MarkFlagRequired("name")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a run command on a VM",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmName, _ := cmd.Flags().GetString("vm-name")
			name, _ := cmd.Flags().GetString("name")
			location, _ := cmd.Flags().GetString("location")
			script, _ := cmd.Flags().GetString("script")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Create(context.Background(), cmd, resourceGroup, vmName, name, location, script, noWait)
		},
	}
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().String("vm-name", "", "VM name")
	createCmd.Flags().StringP("name", "n", "", "Run command name")
	createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	createCmd.Flags().String("script", "", "Script content to execute on the VM")
	createCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("vm-name")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("location")
	createCmd.MarkFlagRequired("script")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a run command on a VM",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmName, _ := cmd.Flags().GetString("vm-name")
			name, _ := cmd.Flags().GetString("name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), cmd, resourceGroup, vmName, name, noWait)
		},
	}
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().String("vm-name", "", "VM name")
	deleteCmd.Flags().StringP("name", "n", "", "Run command name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("vm-name")
	deleteCmd.MarkFlagRequired("name")

	cmd.AddCommand(invokeCmd, listCmd, showCmd, createCmd, deleteCmd)

	return cmd
}

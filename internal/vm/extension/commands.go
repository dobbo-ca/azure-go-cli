package extension

import (
	"context"

	"github.com/spf13/cobra"
)

func NewExtensionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extension",
		Short: "Manage VM extensions",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List extensions on a VM",
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
		Short: "Show details of a VM extension",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmName, _ := cmd.Flags().GetString("vm-name")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, resourceGroup, vmName, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("vm-name", "", "VM name")
	showCmd.Flags().StringP("name", "n", "", "Extension name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("vm-name")
	showCmd.MarkFlagRequired("name")

	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Create or update a VM extension",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmName, _ := cmd.Flags().GetString("vm-name")
			name, _ := cmd.Flags().GetString("name")
			publisher, _ := cmd.Flags().GetString("publisher")
			extType, _ := cmd.Flags().GetString("extension-type")
			version, _ := cmd.Flags().GetString("version")
			settings, _ := cmd.Flags().GetString("settings")
			location, _ := cmd.Flags().GetString("location")
			autoUpgradeMinor, _ := cmd.Flags().GetBool("auto-upgrade-minor-version")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Set(context.Background(), cmd, resourceGroup, vmName, name, publisher, extType, version, settings, location, autoUpgradeMinor, noWait)
		},
	}
	setCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	setCmd.Flags().String("vm-name", "", "VM name")
	setCmd.Flags().StringP("name", "n", "", "Extension name")
	setCmd.Flags().String("publisher", "", "Extension handler publisher (e.g., Microsoft.Azure.Extensions)")
	setCmd.Flags().String("extension-type", "", "Extension type (e.g., CustomScript)")
	setCmd.Flags().String("version", "", "Type handler version")
	setCmd.Flags().String("settings", "", "JSON formatted public settings for the extension")
	setCmd.Flags().String("location", "", "Resource location (defaults to the VM's location)")
	setCmd.Flags().Bool("auto-upgrade-minor-version", false, "Use a newer minor version if available at deployment time")
	setCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	setCmd.MarkFlagRequired("resource-group")
	setCmd.MarkFlagRequired("vm-name")
	setCmd.MarkFlagRequired("name")
	setCmd.MarkFlagRequired("publisher")
	setCmd.MarkFlagRequired("extension-type")
	setCmd.MarkFlagRequired("version")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a VM extension",
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
	deleteCmd.Flags().StringP("name", "n", "", "Extension name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("vm-name")
	deleteCmd.MarkFlagRequired("name")

	cmd.AddCommand(listCmd, showCmd, setCmd, deleteCmd)
	return cmd
}

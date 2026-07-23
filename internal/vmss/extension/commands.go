package extension

import (
	"context"

	"github.com/spf13/cobra"
)

func NewExtensionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extension",
		Short: "Manage VMSS extensions",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List extensions on a VMSS",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			return List(context.Background(), cmd, resourceGroup, vmssName)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.Flags().String("vmss-name", "", "VM scale set name")
	listCmd.MarkFlagRequired("resource-group")
	listCmd.MarkFlagRequired("vmss-name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a VMSS extension",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, resourceGroup, vmssName, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("vmss-name", "", "VM scale set name")
	showCmd.Flags().StringP("name", "n", "", "Extension name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("vmss-name")
	showCmd.MarkFlagRequired("name")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a VMSS extension",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			name, _ := cmd.Flags().GetString("name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), cmd, resourceGroup, vmssName, name, noWait)
		},
	}
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().String("vmss-name", "", "VM scale set name")
	deleteCmd.Flags().StringP("name", "n", "", "Extension name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("vmss-name")
	deleteCmd.MarkFlagRequired("name")

	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Create or update a VMSS extension",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			name, _ := cmd.Flags().GetString("name")
			publisher, _ := cmd.Flags().GetString("publisher")
			extType, _ := cmd.Flags().GetString("extension-type")
			version, _ := cmd.Flags().GetString("version")
			settings, _ := cmd.Flags().GetString("settings")
			autoUpgradeMinor, _ := cmd.Flags().GetBool("auto-upgrade-minor-version")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Set(context.Background(), cmd, resourceGroup, vmssName, name, publisher, extType, version, settings, autoUpgradeMinor, noWait)
		},
	}
	setCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	setCmd.Flags().String("vmss-name", "", "VM scale set name")
	setCmd.Flags().StringP("name", "n", "", "Extension name")
	setCmd.Flags().String("publisher", "", "Extension handler publisher (e.g., Microsoft.Azure.Extensions)")
	setCmd.Flags().String("extension-type", "", "Extension type (e.g., CustomScript)")
	setCmd.Flags().String("version", "", "Type handler version")
	setCmd.Flags().String("settings", "", "JSON formatted public settings for the extension")
	setCmd.Flags().Bool("auto-upgrade-minor-version", false, "Use a newer minor version if available at deployment time")
	setCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	setCmd.MarkFlagRequired("resource-group")
	setCmd.MarkFlagRequired("vmss-name")
	setCmd.MarkFlagRequired("name")
	setCmd.MarkFlagRequired("publisher")
	setCmd.MarkFlagRequired("extension-type")
	setCmd.MarkFlagRequired("version")

	upgradeCmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Start a rolling upgrade of all extensions to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Upgrade(context.Background(), cmd, resourceGroup, vmssName, noWait)
		},
	}
	upgradeCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	upgradeCmd.Flags().String("vmss-name", "", "VM scale set name")
	upgradeCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	upgradeCmd.MarkFlagRequired("resource-group")
	upgradeCmd.MarkFlagRequired("vmss-name")

	cmd.AddCommand(listCmd, showCmd, deleteCmd, setCmd, upgradeCmd)
	return cmd
}

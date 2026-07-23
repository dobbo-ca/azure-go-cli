package rollingupgrade

import (
	"context"

	"github.com/spf13/cobra"
)

func NewRollingUpgradeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rolling-upgrade",
		Short: "Manage VMSS rolling upgrades",
	}

	cancelCmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel the current VMSS rolling upgrade",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Cancel(context.Background(), cmd, resourceGroup, vmssName, noWait)
		},
	}
	cancelCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	cancelCmd.Flags().String("vmss-name", "", "VM scale set name")
	cancelCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	cancelCmd.MarkFlagRequired("resource-group")
	cancelCmd.MarkFlagRequired("vmss-name")

	getLatestCmd := &cobra.Command{
		Use:   "get-latest",
		Short: "Get the status of the latest VMSS rolling upgrade",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			return GetLatest(context.Background(), cmd, resourceGroup, vmssName)
		},
	}
	getLatestCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	getLatestCmd.Flags().String("vmss-name", "", "VM scale set name")
	getLatestCmd.MarkFlagRequired("resource-group")
	getLatestCmd.MarkFlagRequired("vmss-name")

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start an OS rolling upgrade for a VMSS",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			vmssName, _ := cmd.Flags().GetString("vmss-name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Start(context.Background(), cmd, resourceGroup, vmssName, noWait)
		},
	}
	startCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	startCmd.Flags().String("vmss-name", "", "VM scale set name")
	startCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	startCmd.MarkFlagRequired("resource-group")
	startCmd.MarkFlagRequired("vmss-name")

	cmd.AddCommand(cancelCmd, getLatestCmd, startCmd)
	return cmd
}

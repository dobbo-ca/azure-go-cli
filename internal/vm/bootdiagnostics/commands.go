package bootdiagnostics

import (
	"context"

	"github.com/spf13/cobra"
)

func NewBootDiagnosticsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "boot-diagnostics",
		Short: "Manage boot diagnostics for a virtual machine",
	}

	enableCmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable boot diagnostics on a virtual machine",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			storage, _ := cmd.Flags().GetString("storage")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Enable(context.Background(), cmd, rg, name, storage, noWait)
		},
	}
	enableCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	enableCmd.Flags().StringP("name", "n", "", "VM name")
	enableCmd.Flags().String("storage", "", "Storage account URI for boot diagnostics (managed storage if omitted)")
	enableCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	enableCmd.MarkFlagRequired("resource-group")
	enableCmd.MarkFlagRequired("name")

	disableCmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable boot diagnostics on a virtual machine",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Disable(context.Background(), cmd, rg, name, noWait)
		},
	}
	disableCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	disableCmd.Flags().StringP("name", "n", "", "VM name")
	disableCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	disableCmd.MarkFlagRequired("resource-group")
	disableCmd.MarkFlagRequired("name")

	getURIsCmd := &cobra.Command{
		Use:   "get-boot-log-uris",
		Short: "Get the boot diagnostics log SAS URIs for a virtual machine",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return GetBootLogURIs(context.Background(), cmd, rg, name)
		},
	}
	getURIsCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	getURIsCmd.Flags().StringP("name", "n", "", "VM name")
	getURIsCmd.MarkFlagRequired("resource-group")
	getURIsCmd.MarkFlagRequired("name")

	cmd.AddCommand(enableCmd, disableCmd, getURIsCmd)
	return cmd
}

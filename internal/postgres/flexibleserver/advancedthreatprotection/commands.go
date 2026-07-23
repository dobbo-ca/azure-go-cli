package advancedthreatprotection

import (
	"context"

	"github.com/spf13/cobra"
)

func NewAdvancedThreatProtectionSettingCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "advanced-threat-protection-setting",
		Short: "Manage advanced threat protection settings for a PostgreSQL flexible server",
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show advanced threat protection settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			return Show(context.Background(), cmd, rg, server)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("server-name", "", "Flexible server name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("server-name")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update advanced threat protection settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			state, _ := cmd.Flags().GetString("state")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Update(context.Background(), cmd, rg, server, state, noWait)
		},
	}
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().String("server-name", "", "Flexible server name")
	updateCmd.Flags().String("state", "", "Threat protection state (Enabled|Disabled)")
	updateCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	updateCmd.MarkFlagRequired("resource-group")
	updateCmd.MarkFlagRequired("server-name")
	updateCmd.MarkFlagRequired("state")

	cmd.AddCommand(showCmd, updateCmd)
	return cmd
}

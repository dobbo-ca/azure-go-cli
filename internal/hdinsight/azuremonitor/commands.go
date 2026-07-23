package azuremonitor

import (
	"context"

	"github.com/spf13/cobra"
)

func NewAzureMonitorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "azure-monitor",
		Short: "Manage the Azure Monitor integration of an HDInsight cluster",
	}

	enableCmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable the Azure Monitor integration on an HDInsight cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			workspaceID, _ := cmd.Flags().GetString("workspace-id")
			primaryKey, _ := cmd.Flags().GetString("primary-key")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Enable(context.Background(), cmd, rg, clusterName, workspaceID, primaryKey, noWait)
		},
	}
	enableCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	enableCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	enableCmd.Flags().String("workspace-id", "", "Log Analytics workspace ID")
	enableCmd.Flags().String("primary-key", "", "Log Analytics workspace key")
	enableCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	enableCmd.MarkFlagRequired("resource-group")
	enableCmd.MarkFlagRequired("cluster-name")
	enableCmd.MarkFlagRequired("workspace-id")

	disableCmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable the Azure Monitor integration on an HDInsight cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Disable(context.Background(), cmd, rg, clusterName, noWait)
		},
	}
	disableCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	disableCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	disableCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	disableCmd.MarkFlagRequired("resource-group")
	disableCmd.MarkFlagRequired("cluster-name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show the Azure Monitor integration status of an HDInsight cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			return Show(context.Background(), cmd, rg, clusterName)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("cluster-name")

	cmd.AddCommand(enableCmd, disableCmd, showCmd)
	return cmd
}

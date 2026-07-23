package credentials

import (
	"context"

	"github.com/spf13/cobra"
)

func NewCredentialsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credentials",
		Short: "Manage the gateway credentials of an HDInsight cluster",
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show gateway settings",
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

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update the gateway HTTP username/password",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			username, _ := cmd.Flags().GetString("http-username")
			password, _ := cmd.Flags().GetString("http-password")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Update(context.Background(), cmd, rg, clusterName, username, password, noWait)
		},
	}
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	updateCmd.Flags().String("http-username", "", "Gateway settings user name")
	updateCmd.Flags().String("http-password", "", "Gateway settings user password")
	updateCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	updateCmd.MarkFlagRequired("resource-group")
	updateCmd.MarkFlagRequired("cluster-name")
	updateCmd.MarkFlagRequired("http-username")
	updateCmd.MarkFlagRequired("http-password")

	waitCmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait for the cluster to reach a condition",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			deleted, _ := cmd.Flags().GetBool("deleted")
			exists, _ := cmd.Flags().GetBool("exists")
			interval, _ := cmd.Flags().GetInt("interval")
			timeout, _ := cmd.Flags().GetInt("timeout")
			return Wait(context.Background(), cmd, rg, clusterName, deleted, exists, interval, timeout)
		},
	}
	waitCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	waitCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	waitCmd.Flags().Bool("created", false, "Wait until the cluster is created (Succeeded)")
	waitCmd.Flags().Bool("deleted", false, "Wait until the cluster is deleted")
	waitCmd.Flags().Bool("exists", false, "Wait until the cluster exists")
	waitCmd.Flags().Int("interval", 30, "Polling interval in seconds")
	waitCmd.Flags().Int("timeout", 3600, "Maximum wait time in seconds")
	waitCmd.MarkFlagRequired("resource-group")
	waitCmd.MarkFlagRequired("cluster-name")

	cmd.AddCommand(showCmd, updateCmd, waitCmd)
	return cmd
}

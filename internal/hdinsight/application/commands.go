package application

import (
	"context"

	"github.com/spf13/cobra"
)

func NewApplicationCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "application",
		Short: "Manage applications on an HDInsight cluster",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List applications",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			cluster, _ := cmd.Flags().GetString("cluster-name")
			return List(context.Background(), cmd, rg, cluster)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	listCmd.MarkFlagRequired("resource-group")
	listCmd.MarkFlagRequired("cluster-name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show an application",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			cluster, _ := cmd.Flags().GetString("cluster-name")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, rg, cluster, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	showCmd.Flags().StringP("name", "n", "", "Application name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("cluster-name")
	showCmd.MarkFlagRequired("name")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an application",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			cluster, _ := cmd.Flags().GetString("cluster-name")
			name, _ := cmd.Flags().GetString("name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), cmd, rg, cluster, name, noWait)
		},
	}
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	deleteCmd.Flags().StringP("name", "n", "", "Application name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("cluster-name")
	deleteCmd.MarkFlagRequired("name")

	waitCmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait for an application to reach a condition",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			cluster, _ := cmd.Flags().GetString("cluster-name")
			name, _ := cmd.Flags().GetString("name")
			deleted, _ := cmd.Flags().GetBool("deleted")
			exists, _ := cmd.Flags().GetBool("exists")
			interval, _ := cmd.Flags().GetInt("interval")
			timeout, _ := cmd.Flags().GetInt("timeout")
			return Wait(context.Background(), cmd, rg, cluster, name, deleted, exists, interval, timeout)
		},
	}
	waitCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	waitCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	waitCmd.Flags().StringP("name", "n", "", "Application name")
	waitCmd.Flags().Bool("created", false, "Wait until created with terminal provisioning state")
	waitCmd.Flags().Bool("deleted", false, "Wait until deleted")
	waitCmd.Flags().Bool("exists", false, "Wait until the application exists")
	waitCmd.Flags().Int("interval", 30, "Polling interval in seconds")
	waitCmd.Flags().Int("timeout", 3600, "Maximum wait time in seconds")
	waitCmd.MarkFlagRequired("resource-group")
	waitCmd.MarkFlagRequired("cluster-name")
	waitCmd.MarkFlagRequired("name")

	cmd.AddCommand(listCmd, showCmd, deleteCmd, waitCmd)
	return cmd
}

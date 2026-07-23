package host

import (
	"context"

	"github.com/spf13/cobra"
)

func NewHostCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host",
		Short: "Manage the hosts of an HDInsight cluster",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List the hosts of a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			return List(context.Background(), cmd, rg, clusterName)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.Flags().String("cluster-name", "", "Cluster name")
	listCmd.MarkFlagRequired("resource-group")
	listCmd.MarkFlagRequired("cluster-name")

	restartCmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the specified hosts of a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			hostNames, _ := cmd.Flags().GetStringSlice("host-names")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Restart(context.Background(), cmd, rg, clusterName, hostNames, noWait)
		},
	}
	restartCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	restartCmd.Flags().String("cluster-name", "", "Cluster name")
	restartCmd.Flags().StringSlice("host-names", nil, "Names of hosts to restart")
	restartCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	restartCmd.MarkFlagRequired("resource-group")
	restartCmd.MarkFlagRequired("cluster-name")
	restartCmd.MarkFlagRequired("host-names")

	cmd.AddCommand(listCmd, restartCmd)
	return cmd
}

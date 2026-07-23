package scriptaction

import (
	"context"

	"github.com/spf13/cobra"
)

func NewScriptActionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "script-action",
		Short: "Manage script actions on an HDInsight cluster",
	}

	executeCmd := &cobra.Command{
		Use:   "execute",
		Short: "Execute a script action on a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			name, _ := cmd.Flags().GetString("name")
			scriptURI, _ := cmd.Flags().GetString("script-uri")
			roles, _ := cmd.Flags().GetStringSlice("roles")
			parameters, _ := cmd.Flags().GetString("parameters")
			persistOnSuccess, _ := cmd.Flags().GetBool("persist-on-success")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Execute(context.Background(), cmd, rg, clusterName, name, scriptURI, roles, parameters, persistOnSuccess, noWait)
		},
	}
	executeCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	executeCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	executeCmd.Flags().StringP("name", "n", "", "Script action name")
	executeCmd.Flags().String("script-uri", "", "The URI to the script")
	executeCmd.Flags().StringSlice("roles", nil, "The list of roles where the script will be executed")
	executeCmd.Flags().String("parameters", "", "The parameters for the script")
	executeCmd.Flags().Bool("persist-on-success", false, "Persist the script action on success")
	executeCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	executeCmd.MarkFlagRequired("cluster-name")
	executeCmd.MarkFlagRequired("resource-group")
	executeCmd.MarkFlagRequired("name")
	executeCmd.MarkFlagRequired("script-uri")
	executeCmd.MarkFlagRequired("roles")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a persisted script action",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			name, _ := cmd.Flags().GetString("name")
			return Delete(context.Background(), cmd, rg, clusterName, name)
		},
	}
	deleteCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().StringP("name", "n", "", "Persisted script name")
	deleteCmd.MarkFlagRequired("cluster-name")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("name")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List persisted script actions for a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			return List(context.Background(), cmd, rg, clusterName)
		},
	}
	listCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.MarkFlagRequired("cluster-name")
	listCmd.MarkFlagRequired("resource-group")

	listExecutionHistoryCmd := &cobra.Command{
		Use:   "list-execution-history",
		Short: "List script action execution history for a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			return ListExecutionHistory(context.Background(), cmd, rg, clusterName)
		},
	}
	listExecutionHistoryCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	listExecutionHistoryCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listExecutionHistoryCmd.MarkFlagRequired("cluster-name")
	listExecutionHistoryCmd.MarkFlagRequired("resource-group")

	promoteCmd := &cobra.Command{
		Use:   "promote",
		Short: "Promote a script action execution to a persisted script action",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			executionID, _ := cmd.Flags().GetString("execution-id")
			return Promote(context.Background(), cmd, rg, clusterName, executionID)
		},
	}
	promoteCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	promoteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	promoteCmd.Flags().String("execution-id", "", "The script action execution ID")
	promoteCmd.MarkFlagRequired("cluster-name")
	promoteCmd.MarkFlagRequired("resource-group")
	promoteCmd.MarkFlagRequired("execution-id")

	showExecutionDetailsCmd := &cobra.Command{
		Use:   "show-execution-details",
		Short: "Show the details of a script action execution",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			executionID, _ := cmd.Flags().GetString("execution-id")
			return ShowExecutionDetails(context.Background(), cmd, rg, clusterName, executionID)
		},
	}
	showExecutionDetailsCmd.Flags().String("cluster-name", "", "HDInsight cluster name")
	showExecutionDetailsCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showExecutionDetailsCmd.Flags().String("execution-id", "", "The script action execution ID")
	showExecutionDetailsCmd.MarkFlagRequired("cluster-name")
	showExecutionDetailsCmd.MarkFlagRequired("resource-group")
	showExecutionDetailsCmd.MarkFlagRequired("execution-id")

	cmd.AddCommand(executeCmd, deleteCmd, listCmd, listExecutionHistoryCmd, promoteCmd, showExecutionDetailsCmd)
	return cmd
}

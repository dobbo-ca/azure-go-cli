package operation

import (
	"context"

	"github.com/spf13/cobra"
)

func NewOperationCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "operation",
		Short: "Manage and view operations on AKS clusters",
		Long:  "Commands to view operation status for AKS clusters",
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details for a specific operation",
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			operationID, _ := cmd.Flags().GetString("operation-id")
			return Show(context.Background(), clusterName, resourceGroup, operationID)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "AKS cluster name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("operation-id", "", "Operation ID")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("operation-id")

	showLatestCmd := &cobra.Command{
		Use:   "show-latest",
		Short: "Show details for the latest operation",
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return ShowLatest(context.Background(), clusterName, resourceGroup)
		},
	}
	showLatestCmd.Flags().StringP("name", "n", "", "AKS cluster name")
	showLatestCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showLatestCmd.MarkFlagRequired("name")
	showLatestCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(showCmd, showLatestCmd)
	return cmd
}

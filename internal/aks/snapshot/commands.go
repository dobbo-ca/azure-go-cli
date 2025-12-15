package snapshot

import (
	"context"

	"github.com/spf13/cobra"
)

func NewSnapshotCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage AKS cluster snapshots",
		Long:  "Commands to manage snapshots of AKS node pools",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List cluster snapshots",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), resourceGroup)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a cluster snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			snapshotName, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), snapshotName, resourceGroup)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Snapshot name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(listCmd, showCmd)
	return cmd
}

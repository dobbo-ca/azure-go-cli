package group

import (
	"context"

	"github.com/spf13/cobra"
)

func NewGroupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage resource groups",
		Long:  "Commands to manage Azure resource groups",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List resource groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			return List(context.Background())
		},
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a resource group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), name)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Resource group name")
	showCmd.MarkFlagRequired("name")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a resource group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			location, _ := cmd.Flags().GetString("location")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Create(context.Background(), cmd, name, location, tags)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Resource group name")
	createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("location")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a resource group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), name, noWait)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Resource group name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	deleteCmd.Flags().BoolP("yes", "y", false, "Do not prompt for confirmation")
	deleteCmd.MarkFlagRequired("name")

	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
	return cmd
}

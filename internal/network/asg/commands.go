package asg

import (
	"context"

	"github.com/spf13/cobra"
)

func NewASGCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "asg",
		Short: "Manage application security groups",
		Long:  "Commands to manage Azure application security groups",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List application security groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, resourceGroup)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of an application security group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, name, resourceGroup)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "ASG name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create an application security group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Create(context.Background(), cmd, name, resourceGroup, location, tags)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "ASG name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	createCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("location")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an application security group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), name, resourceGroup, noWait)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "ASG name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("resource-group")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update an application security group (tags)",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Update(context.Background(), cmd, name, resourceGroup, noWait)
		},
	}
	updateCmd.Flags().StringP("name", "n", "", "ASG name")
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	updateCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	updateCmd.MarkFlagRequired("name")
	updateCmd.MarkFlagRequired("resource-group")

	waitCmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait until a condition of the application security group is met",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			deleted, _ := cmd.Flags().GetBool("deleted")
			exists, _ := cmd.Flags().GetBool("exists")
			interval, _ := cmd.Flags().GetInt("interval")
			timeout, _ := cmd.Flags().GetInt("timeout")
			return Wait(context.Background(), cmd, name, resourceGroup, deleted, exists, interval, timeout)
		},
	}
	waitCmd.Flags().StringP("name", "n", "", "ASG name")
	waitCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	waitCmd.Flags().Bool("deleted", false, "Wait until deleted")
	waitCmd.Flags().Bool("exists", false, "Wait until the resource exists")
	waitCmd.Flags().Int("interval", 30, "Polling interval in seconds")
	waitCmd.Flags().Int("timeout", 3600, "Maximum wait time in seconds")
	waitCmd.MarkFlagRequired("name")
	waitCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd, updateCmd, waitCmd)
	return cmd
}

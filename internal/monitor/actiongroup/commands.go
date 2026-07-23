package actiongroup

import (
	"context"

	"github.com/spf13/cobra"
)

func NewActionGroupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "action-group",
		Short: "Manage Azure Monitor action groups",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List action groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, resourceGroup)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of an action group",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, resourceGroup, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().StringP("name", "n", "", "Action group name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("name")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create or update an action group",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			shortName, _ := cmd.Flags().GetString("short-name")
			emails, _ := cmd.Flags().GetStringToString("email")
			webhooks, _ := cmd.Flags().GetStringToString("webhook")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Create(context.Background(), cmd, resourceGroup, name, shortName, emails, webhooks, tags)
		},
	}
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().StringP("name", "n", "", "Action group name")
	createCmd.Flags().String("short-name", "", "Short name of the action group (used in SMS messages)")
	createCmd.Flags().StringToString("email", nil, "Email receivers name=address")
	createCmd.Flags().StringToString("webhook", nil, "Webhook receivers name=uri")
	createCmd.Flags().StringToString("tags", nil, "Tags: key1=value1 key2=value2")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("short-name")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an action group",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return Delete(context.Background(), cmd, resourceGroup, name)
		},
	}
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().StringP("name", "n", "", "Action group name")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("name")

	enableReceiverCmd := &cobra.Command{
		Use:   "enable-receiver",
		Short: "Resubscribe (enable) a receiver in an action group",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			receiverName, _ := cmd.Flags().GetString("receiver-name")
			return EnableReceiver(context.Background(), cmd, resourceGroup, name, receiverName)
		},
	}
	enableReceiverCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	enableReceiverCmd.Flags().StringP("name", "n", "", "Action group name")
	enableReceiverCmd.Flags().String("receiver-name", "", "Name of the receiver to enable")
	enableReceiverCmd.MarkFlagRequired("resource-group")
	enableReceiverCmd.MarkFlagRequired("name")
	enableReceiverCmd.MarkFlagRequired("receiver-name")

	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd, enableReceiverCmd)

	return cmd
}

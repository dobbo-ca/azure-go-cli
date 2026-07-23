package managementgroup

import (
	"context"

	"github.com/spf13/cobra"
)

// NewManagementGroupCommand builds the `az account management-group` group
// (Microsoft.Management/managementGroups).
func NewManagementGroupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "management-group",
		Aliases: []string{"managementgroup"},
		Short:   "Manage Azure Management Groups",
		Long:    "Commands to manage Azure management groups, their subscriptions and settings",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List management groups for the current tenant",
		RunE: func(cmd *cobra.Command, args []string) error {
			return List(context.Background(), cmd)
		},
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a management group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, name)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Management group ID (name)")
	showCmd.MarkFlagRequired("name")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a management group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			displayName, _ := cmd.Flags().GetString("display-name")
			parent, _ := cmd.Flags().GetString("parent")
			return Create(context.Background(), name, displayName, parent)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Management group ID (name)")
	createCmd.Flags().String("display-name", "", "Friendly display name (defaults to the name)")
	createCmd.Flags().String("parent", "", "Parent management group name or full ID")
	createCmd.MarkFlagRequired("name")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a management group's display name or parent",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			displayName, _ := cmd.Flags().GetString("display-name")
			parent, _ := cmd.Flags().GetString("parent")
			return Update(context.Background(), cmd, name, displayName, parent)
		},
	}
	updateCmd.Flags().StringP("name", "n", "", "Management group ID (name)")
	updateCmd.Flags().String("display-name", "", "New friendly display name")
	updateCmd.Flags().String("parent", "", "New parent management group name or full ID")
	updateCmd.MarkFlagRequired("name")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a management group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			return Delete(context.Background(), name)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Management group ID (name)")
	deleteCmd.MarkFlagRequired("name")

	checkNameCmd := &cobra.Command{
		Use:   "check-name-availability",
		Short: "Check whether a management group name is available",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			return CheckNameAvailability(context.Background(), cmd, name)
		},
	}
	checkNameCmd.Flags().StringP("name", "n", "", "Management group name to check")
	checkNameCmd.MarkFlagRequired("name")

	entitiesCmd := &cobra.Command{
		Use:   "entities",
		Short: "List all entities (management groups and subscriptions) for the tenant",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ListEntities(context.Background(), cmd)
		},
	}

	hierarchySettingsCmd := &cobra.Command{
		Use:   "hierarchy-settings",
		Short: "Show the hierarchy settings for a management group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			return ShowHierarchySettings(context.Background(), cmd, name)
		},
	}
	hierarchySettingsCmd.Flags().StringP("name", "n", "", "Management group ID (name)")
	hierarchySettingsCmd.MarkFlagRequired("name")

	cmd.AddCommand(listCmd, showCmd, createCmd, updateCmd, deleteCmd, checkNameCmd, entitiesCmd, hierarchySettingsCmd)
	cmd.AddCommand(newSubscriptionCommand(), newTenantBackfillCommand())
	return cmd
}

// newSubscriptionCommand builds `management-group subscription add|remove`.
func newSubscriptionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscription",
		Short: "Associate or dissociate a subscription with a management group",
	}

	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a subscription to a management group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			subscription, _ := cmd.Flags().GetString("subscription")
			return AddSubscription(context.Background(), name, subscription)
		},
	}
	addCmd.Flags().StringP("name", "n", "", "Management group ID (name)")
	addCmd.Flags().String("subscription", "", "Subscription ID to add")
	addCmd.MarkFlagRequired("name")
	addCmd.MarkFlagRequired("subscription")

	removeCmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a subscription from a management group",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			subscription, _ := cmd.Flags().GetString("subscription")
			return RemoveSubscription(context.Background(), name, subscription)
		},
	}
	removeCmd.Flags().StringP("name", "n", "", "Management group ID (name)")
	removeCmd.Flags().String("subscription", "", "Subscription ID to remove")
	removeCmd.MarkFlagRequired("name")
	removeCmd.MarkFlagRequired("subscription")

	cmd.AddCommand(addCmd, removeCmd)
	return cmd
}

// newTenantBackfillCommand builds `management-group tenant-backfill get|start`.
func newTenantBackfillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant-backfill",
		Short: "Manage tenant backfill of subscriptions into management groups",
	}

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get the tenant backfill status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return GetTenantBackfill(context.Background(), cmd)
		},
	}

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start backfilling subscriptions for the tenant",
		RunE: func(cmd *cobra.Command, args []string) error {
			return StartTenantBackfill(context.Background(), cmd)
		},
	}

	cmd.AddCommand(getCmd, startCmd)
	return cmd
}

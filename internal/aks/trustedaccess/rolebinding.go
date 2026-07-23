package trustedaccess

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewTrustedAccessRoleBindingsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create trusted access role bindings client: %w", err)
	}

	var items []*armcontainerservice.TrustedAccessRoleBinding
	pager := client.NewListPager(resourceGroup, clusterName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list trusted access role bindings: %w", err)
		}
		items = append(items, page.Value...)
	}

	return output.PrintJSON(cmd, items)
}

func Show(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewTrustedAccessRoleBindingsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create trusted access role bindings client: %w", err)
	}

	resp, err := client.Get(ctx, resourceGroup, clusterName, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get trusted access role binding: %w", err)
	}

	return output.PrintJSON(cmd, resp.TrustedAccessRoleBinding)
}

func CreateOrUpdate(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName, name, sourceResourceID string, roles []string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewTrustedAccessRoleBindingsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create trusted access role bindings client: %w", err)
	}

	rolesPtr := make([]*string, 0, len(roles))
	for _, r := range roles {
		rolesPtr = append(rolesPtr, to.Ptr(r))
	}

	binding := armcontainerservice.TrustedAccessRoleBinding{
		Properties: &armcontainerservice.TrustedAccessRoleBindingProperties{
			SourceResourceID: to.Ptr(sourceResourceID),
			Roles:            rolesPtr,
		},
	}

	fmt.Printf("Creating trusted access role binding '%s'...\n", name)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, clusterName, name, binding, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create or update: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "create or update started"})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("create or update operation failed: %w", err)
	}

	return output.PrintJSON(cmd, resp.TrustedAccessRoleBinding)
}

func Delete(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName, name string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewTrustedAccessRoleBindingsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create trusted access role bindings client: %w", err)
	}

	fmt.Printf("Deleting trusted access role binding '%s'...\n", name)
	poller, err := client.BeginDelete(ctx, resourceGroup, clusterName, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "delete started"})
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete operation failed: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("'%s' deleted.", name)})
}

func newRoleBindingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rolebinding",
		Short: "Manage trusted access role bindings",
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a trusted access role binding",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			name, _ := cmd.Flags().GetString("name")
			sourceResourceID, _ := cmd.Flags().GetString("source-resource-id")
			roles, _ := cmd.Flags().GetStringSlice("roles")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return CreateOrUpdate(context.Background(), cmd, rg, clusterName, name, sourceResourceID, roles, noWait)
		},
	}
	createCmd.Flags().String("cluster-name", "", "Name of the managed cluster")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().StringP("name", "n", "", "Trusted access role binding name")
	createCmd.Flags().String("source-resource-id", "", "The ARM resource ID of source resource that trusted access is configured for")
	createCmd.Flags().StringSlice("roles", nil, "Space/comma separated role names e.g. Microsoft.MachineLearningServices/workspaces/mlworkload")
	createCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	createCmd.MarkFlagRequired("cluster-name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("source-resource-id")
	createCmd.MarkFlagRequired("roles")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a trusted access role binding",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			name, _ := cmd.Flags().GetString("name")
			sourceResourceID, _ := cmd.Flags().GetString("source-resource-id")
			roles, _ := cmd.Flags().GetStringSlice("roles")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return CreateOrUpdate(context.Background(), cmd, rg, clusterName, name, sourceResourceID, roles, noWait)
		},
	}
	updateCmd.Flags().String("cluster-name", "", "Name of the managed cluster")
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().StringP("name", "n", "", "Trusted access role binding name")
	updateCmd.Flags().String("source-resource-id", "", "The ARM resource ID of source resource that trusted access is configured for")
	updateCmd.Flags().StringSlice("roles", nil, "Space/comma separated role names e.g. Microsoft.MachineLearningServices/workspaces/mlworkload")
	updateCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	updateCmd.MarkFlagRequired("cluster-name")
	updateCmd.MarkFlagRequired("resource-group")
	updateCmd.MarkFlagRequired("name")
	updateCmd.MarkFlagRequired("source-resource-id")
	updateCmd.MarkFlagRequired("roles")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a trusted access role binding",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			name, _ := cmd.Flags().GetString("name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), cmd, rg, clusterName, name, noWait)
		},
	}
	deleteCmd.Flags().String("cluster-name", "", "Name of the managed cluster")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().StringP("name", "n", "", "Trusted access role binding name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("cluster-name")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("name")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List trusted access role bindings",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			return List(context.Background(), cmd, rg, clusterName)
		},
	}
	listCmd.Flags().String("cluster-name", "", "Name of the managed cluster")
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.MarkFlagRequired("cluster-name")
	listCmd.MarkFlagRequired("resource-group")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a trusted access role binding",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, rg, clusterName, name)
		},
	}
	showCmd.Flags().String("cluster-name", "", "Name of the managed cluster")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().StringP("name", "n", "", "Trusted access role binding name")
	showCmd.MarkFlagRequired("cluster-name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("name")

	cmd.AddCommand(createCmd, updateCmd, deleteCmd, listCmd, showCmd)
	return cmd
}

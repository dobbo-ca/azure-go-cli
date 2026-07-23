package trustedaccess

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func RoleList(ctx context.Context, cmd *cobra.Command, location string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewTrustedAccessRolesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create trusted access roles client: %w", err)
	}

	var items []*armcontainerservice.TrustedAccessRole
	pager := client.NewListPager(location, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list trusted access roles: %w", err)
		}
		items = append(items, page.Value...)
	}

	return output.PrintJSON(cmd, items)
}

func newRoleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "role",
		Short: "Manage trusted access roles",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List trusted access roles available in a location",
		RunE: func(cmd *cobra.Command, args []string) error {
			location, _ := cmd.Flags().GetString("location")
			return RoleList(context.Background(), cmd, location)
		},
	}
	listCmd.Flags().StringP("location", "l", "", "Location")
	listCmd.MarkFlagRequired("location")

	cmd.AddCommand(listCmd)
	return cmd
}

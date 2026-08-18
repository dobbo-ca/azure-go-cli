package encryptionset

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func newListAssociatedResourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-associated-resources",
		Short: "List resources encrypted with a disk encryption set",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return listAssociatedResources(context.Background(), cmd, name, resourceGroup)
		},
	}

	cmd.Flags().StringP("name", "n", "", "Disk encryption set name")
	cmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("resource-group")

	return cmd
}

func listAssociatedResources(ctx context.Context, cmd *cobra.Command, name, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armcompute.NewDiskEncryptionSetsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create disk encryption sets client: %w", err)
	}

	var resources []*string
	pager := client.NewListAssociatedResourcesPager(resourceGroup, name, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list associated resources: %w", err)
		}
		resources = append(resources, page.Value...)
	}

	return output.PrintJSON(cmd, resources)
}

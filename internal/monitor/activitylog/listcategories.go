package activitylog

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func ListCategories(ctx context.Context, cmd *cobra.Command) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewEventCategoriesClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create event categories client: %w", err)
	}

	var categories []*armmonitor.LocalizableString
	pager := client.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list event categories: %w", err)
		}
		categories = append(categories, page.Value...)
	}
	return output.PrintJSON(cmd, categories)
}

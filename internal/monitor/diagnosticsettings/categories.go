package diagnosticsettings

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func CategoriesList(ctx context.Context, cmd *cobra.Command, resource string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewDiagnosticSettingsCategoryClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create diagnostic settings category client: %w", err)
	}

	var items []*armmonitor.DiagnosticSettingsCategoryResource
	pager := client.NewListPager(resource, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list diagnostic setting categories: %w", err)
		}
		items = append(items, page.Value...)
	}
	return output.PrintJSON(cmd, items)
}

func CategoriesShow(ctx context.Context, cmd *cobra.Command, resource, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewDiagnosticSettingsCategoryClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create diagnostic settings category client: %w", err)
	}

	resp, err := client.Get(ctx, resource, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get diagnostic setting category: %w", err)
	}
	return output.PrintJSON(cmd, resp.DiagnosticSettingsCategoryResource)
}

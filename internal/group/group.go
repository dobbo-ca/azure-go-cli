package group

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource groups client: %w", err)
	}

	pager := client.NewListPager(nil)
	var groups []map[string]interface{}

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list resource groups: %w", err)
		}

		for _, rg := range page.Value {
			group := map[string]interface{}{
				"name":     azure.GetStringValue(rg.Name),
				"location": azure.GetStringValue(rg.Location),
			}
			if rg.Tags != nil {
				group["tags"] = rg.Tags
			}
			groups = append(groups, group)
		}
	}

	return output.PrintJSON(cmd, groups)
}

package sig

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command) error {
	rg, _ := cmd.Flags().GetString("resource-group")
	client, _, _, err := newClients(cmd)
	if err != nil {
		return err
	}

	galleries := []*armcompute.Gallery{}
	if rg != "" {
		pager := client.NewListByResourceGroupPager(rg, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list galleries: %w", err)
			}
			galleries = append(galleries, page.Value...)
		}
	} else {
		pager := client.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list galleries: %w", err)
			}
			galleries = append(galleries, page.Value...)
		}
	}
	return output.PrintJSON(cmd, galleries)
}

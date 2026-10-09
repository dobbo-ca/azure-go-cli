package provider

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, expand string) error {
	client, err := newClient(cmd)
	if err != nil {
		return err
	}

	providers := []*armresources.Provider{}
	pager := client.NewListPager(&armresources.ProvidersClientListOptions{Expand: expandOpt(expand)})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list providers: %w", err)
		}
		providers = append(providers, page.Value...)
	}

	return output.PrintJSON(cmd, providers)
}

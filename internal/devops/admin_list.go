package devops

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// adminBannerListCmd is "devops admin banner list" (banner.py:10-14).
func adminBannerListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List banners.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdminBannerList(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)

	return cmd
}

func runAdminBannerList(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return adminBannerListDo(ctx, cmd, client)
}

func adminBannerListDo(ctx context.Context, cmd *cobra.Command, client *ado.Client) error {
	entries, err := adminFetchBanners(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to list banners: %w", err)
	}
	return adminPrintBanners(cmd, entries)
}

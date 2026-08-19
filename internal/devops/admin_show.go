package devops

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// adminBannerShowCmd is "devops admin banner show" (banner.py:17-27).
func adminBannerShowCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details for a banner.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdminBannerShow(context.Background(), cmd, id)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Identifier for the banner.")
	_ = cmd.MarkFlagRequired("id")
	ado.AddOrgFlags(cmd)

	return cmd
}

func runAdminBannerShow(ctx context.Context, cmd *cobra.Command, id string) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return adminBannerShowDo(ctx, cmd, client, id)
}

// adminBannerShowDo is banner_show's whole body: there is no server-side
// "get single banner" endpoint, so show is always a full list fetch
// followed by a client-side lookup by exact key (banner.py:23-26) — do not
// "optimize" this into a query-parameterized single-item GET, it would
// change the error semantics (a full successful fetch happens even for a
// nonexistent id, before failing locally).
func adminBannerShowDo(ctx context.Context, cmd *cobra.Command, client *ado.Client, id string) error {
	entries, err := adminFetchBanners(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to show banner: %w", err)
	}

	entry, ok := entries[id]
	if !ok {
		return fmt.Errorf("The following banner was not found: %s", id)
	}

	return adminPrintBanners(cmd, map[string]map[string]any{id: entry})
}

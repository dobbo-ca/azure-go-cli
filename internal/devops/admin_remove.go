package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// adminBannerRemoveCmd is "devops admin banner remove" (banner.py:121-128).
// This is the one destructive command in the whole surface with no
// confirmation prompt (dev/admin/commands.py:22 has no confirmation= kwarg,
// unlike every delete/remove/uninstall/reset-all elsewhere) — do not add
// --yes/Confirm here to "match the pattern" of the other groups.
func adminBannerRemoveCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a banner.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdminBannerRemove(context.Background(), cmd, id)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "ID of the banner to remove.")
	_ = cmd.MarkFlagRequired("id")
	ado.AddOrgFlags(cmd)

	return cmd
}

func runAdminBannerRemove(ctx context.Context, cmd *cobra.Command, id string) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return adminBannerRemoveDo(ctx, client, id)
}

// adminBannerRemoveDo has no existence check first (unlike show/update) —
// removing a nonexistent id is whatever the Settings API does with a DELETE
// on a nonexistent key (banner.py:121-128 does not list-and-check first).
// banner_remove has no return statement, so there is nothing to print.
func adminBannerRemoveDo(ctx context.Context, client *ado.Client, id string) error {
	key := adminBannerKey + "/" + id
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodDelete,
		Path:       "Settings/Entries/host/" + url.PathEscape(key),
		APIVersion: adminSettingsAPIVersion,
	}, nil); err != nil {
		return fmt.Errorf("failed to remove banner: %w", err)
	}
	return nil
}

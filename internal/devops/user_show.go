package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// userNewShowCmd wires `az devops user show` (user.py:27 get_user_entitlement).
func userNewShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show user details.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return userRunShow(context.Background(), cmd)
		},
	}

	userAddUserFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func userRunShow(ctx context.Context, cmd *cobra.Command) error {
	user, _ := cmd.Flags().GetString("user")

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	id, err := userResolveID(ctx, client, user)
	if err != nil {
		return err
	}

	var entitlement map[string]any
	if err := client.Do(ctx, ado.Request{
		Host:       "vsaex",
		Path:       "userentitlements/" + url.PathEscape(id),
		APIVersion: "5.0-preview.2",
	}, &entitlement); err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	return ado.Print(cmd, entitlement, userColumns...)
}

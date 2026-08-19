package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// userNewRemoveCmd wires `az devops user remove` (user.py:41 delete_user_entitlement).
func userNewRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a user from the organization.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return userRunRemove(context.Background(), cmd)
		},
	}

	userAddUserFlag(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddYesFlag(cmd)

	return cmd
}

func userRunRemove(ctx context.Context, cmd *cobra.Command) error {
	user, _ := cmd.Flags().GetString("user")

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	if err := ado.Confirm(cmd, "Are you sure you want to remove this user?"); err != nil {
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

	// delete_user_entitlement has no explicit return statement (client.py's
	// generated wrapper deserializes nothing for this call) and no table
	// transformer is registered for it, so nothing is printed on success.
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodDelete,
		Host:       "vsaex",
		Path:       "userentitlements/" + url.PathEscape(id),
		APIVersion: "5.0-preview.2",
	}, nil); err != nil {
		return fmt.Errorf("failed to remove user: %w", err)
	}

	return nil
}

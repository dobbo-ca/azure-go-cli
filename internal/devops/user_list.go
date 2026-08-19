package devops

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// userNewListCmd wires `az devops user list` (user.py:15 get_user_entitlements).
func userNewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all the users along with their licenses and extensions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return userRunList(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("top", 100, "Maximum number of users to return.")
	cmd.Flags().Int("skip", 0, "Number of users to skip.")
	ado.AddOrgFlags(cmd)

	return cmd
}

func userRunList(ctx context.Context, cmd *cobra.Command) error {
	top, _ := cmd.Flags().GetInt("top")
	skip, _ := cmd.Flags().GetInt("skip")

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	query := url.Values{"top": {strconv.Itoa(top)}}
	if cmd.Flags().Changed("skip") {
		query.Set("skip", strconv.Itoa(skip))
	}

	// get_user_entitlements returns the whole PagedGraphMemberList wrapper
	// (member_entitlement_management/models.py:939-952 + its PagedList base,
	// :342-357 — continuation_token/items/total_count alongside members),
	// not just the bare members array; decoding into a map rather than a
	// members-only struct keeps whatever the server actually sends instead
	// of guessing field names. So this reads it with Do, not List. Python
	// performs a single call here too; --top/--skip are the pagination
	// controls, there is no auto-follow.
	var page map[string]any
	if err := client.Do(ctx, ado.Request{
		Host:       "vsaex",
		Path:       "userentitlements",
		APIVersion: "5.0-preview.2",
		Query:      query,
	}, &page); err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	// _format.py:331-336: transform_users_table_output reads result['members']
	// for the table's row source; other formats print the whole wrapper.
	if ado.TableMode(cmd) {
		members, _ := page["members"].([]any)
		rows := make([]map[string]any, 0, len(members))
		for _, m := range members {
			if row, ok := m.(map[string]any); ok {
				rows = append(rows, row)
			}
		}
		return ado.Print(cmd, rows, userColumns...)
	}

	return ado.Print(cmd, page)
}

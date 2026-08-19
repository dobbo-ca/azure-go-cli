package pipelines

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// agentpoolNewPoolShowCmd implements `az pipelines pool show` (show_pool,
// agent_pool_queue.py:29).
func agentpoolNewPoolShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get details of a specific agent pool",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return agentpoolRunPoolShow(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("pool-id", 0, "ID of the pool to show details for.")
	cmd.Flags().Int("id", 0, "Alias for --pool-id.")
	cmd.Flags().String("action", "", "Filter the list with user action permitted: use, manage, none.")
	ado.AddOrgFlags(cmd)

	return cmd
}

func agentpoolRunPoolShow(ctx context.Context, cmd *cobra.Command) error {
	poolID, err := agentpoolRequiredIntFlag(cmd, "pool-id", "id")
	if err != nil {
		return err
	}

	action, _ := cmd.Flags().GetString("action")
	action, err = agentpoolValidateChoice(action, "action", agentpoolActionChoices)
	if err != nil {
		return err
	}

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return agentpoolPoolShow(ctx, cmd, client, poolID, action)
}

// agentpoolPoolShow does the HTTP work, split out from agentpoolRunPoolShow
// so tests can drive it against an httptest server directly.
func agentpoolPoolShow(ctx context.Context, cmd *cobra.Command, client *ado.Client, poolID int, action string) error {
	q := url.Values{}
	if action != "" {
		q.Set("actionFilter", action)
	}

	var pool map[string]any
	if err := client.Do(ctx, ado.Request{
		Path:       fmt.Sprintf("distributedtask/pools/%d", poolID),
		APIVersion: "5.1",
		Query:      q,
	}, &pool); err != nil {
		return fmt.Errorf("failed to get agent pool: %w", err)
	}

	return ado.Print(cmd, pool, agentpoolPoolColumns...)
}

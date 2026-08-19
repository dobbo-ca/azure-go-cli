package pipelines

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// agentpoolPoolColumns is the table shape shared by pool list/show
// (_format.py:256-270 _transform_pipeline_pool_row).
var agentpoolPoolColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Field: "name"},
	{Header: "Is Hosted", Field: "isHosted"},
	{Header: "Pool Type", Field: "poolType"},
}

// agentpoolNewPoolListCmd implements `az pipelines pool list` (list_pools,
// agent_pool_queue.py:14).
func agentpoolNewPoolListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List agent pools",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return agentpoolRunPoolList(context.Background(), cmd)
		},
	}

	cmd.Flags().String("pool-name", "", "Filter the list with matching pool name.")
	cmd.Flags().String("pool-type", "", "Filter the list with type of pool: automation, deployment.")
	cmd.Flags().String("action", "", "Filter the list with user action permitted: use, manage, none.")
	ado.AddOrgFlags(cmd)

	return cmd
}

func agentpoolRunPoolList(ctx context.Context, cmd *cobra.Command) error {
	poolName, _ := cmd.Flags().GetString("pool-name")
	poolType, _ := cmd.Flags().GetString("pool-type")
	action, _ := cmd.Flags().GetString("action")

	var err error
	poolType, err = agentpoolValidateChoice(poolType, "pool-type", agentpoolPoolTypeChoices)
	if err != nil {
		return err
	}
	action, err = agentpoolValidateChoice(action, "action", agentpoolActionChoices)
	if err != nil {
		return err
	}

	// pool/agent are org-level, not project-scoped
	// (agent_pool_queue.py:23 resolve_instance, project is discarded) —
	// ado.Resolve, not ado.ResolveProject.
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return agentpoolPoolList(ctx, cmd, client, poolName, poolType, action)
}

// agentpoolPoolList does the HTTP work, split out from agentpoolRunPoolList
// so tests can drive it against an httptest server directly (ado.Resolve's
// org-URL check rejects a plain httptest URL).
func agentpoolPoolList(ctx context.Context, cmd *cobra.Command, client *ado.Client, poolName, poolType, action string) error {
	q := url.Values{}
	if poolName != "" {
		q.Set("poolName", poolName)
	}
	if poolType != "" {
		q.Set("poolType", poolType)
	}
	if action != "" {
		q.Set("actionFilter", action)
	}

	var pools []map[string]any
	if err := client.List(ctx, ado.Request{
		Path:       "distributedtask/pools",
		APIVersion: "5.1",
		Query:      q,
	}, &pools); err != nil {
		return fmt.Errorf("failed to list agent pools: %w", err)
	}

	return ado.Print(cmd, pools, agentpoolPoolColumns...)
}

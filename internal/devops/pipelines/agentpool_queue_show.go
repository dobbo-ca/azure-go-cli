package pipelines

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// agentpoolNewQueueShowCmd implements `az pipelines queue show` (show_queue,
// agent_pool_queue.py:102).
func agentpoolNewQueueShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get details of an agent queue",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return agentpoolRunQueueShow(context.Background(), cmd)
		},
	}

	cmd.Flags().String("queue-id", "", "ID of the agent queue to get information about.")
	cmd.Flags().String("id", "", "Alias for --queue-id.")
	cmd.Flags().String("action", "", "Filter by whether the calling user has use or manage permissions: use, manage, none.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func agentpoolRunQueueShow(ctx context.Context, cmd *cobra.Command) error {
	queueID, err := agentpoolRequiredStringFlag(cmd, "queue-id", "id")
	if err != nil {
		return err
	}

	action, _ := cmd.Flags().GetString("action")
	action, err = agentpoolValidateChoice(action, "action", agentpoolActionChoices)
	if err != nil {
		return err
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return agentpoolQueueShow(ctx, cmd, client, dctx.Project, queueID, action)
}

// agentpoolQueueShow does the HTTP work, split out from agentpoolRunQueueShow
// so tests can drive it against an httptest server directly.
func agentpoolQueueShow(ctx context.Context, cmd *cobra.Command, client *ado.Client, project, queueID, action string) error {
	q := url.Values{}
	if action != "" {
		q.Set("actionFilter", action)
	}

	var queue map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      project,
		Path:       "distributedtask/queues/" + url.PathEscape(queueID),
		APIVersion: "5.1-preview.1",
		Query:      q,
	}, &queue); err != nil {
		return fmt.Errorf("failed to get agent queue: %w", err)
	}

	return ado.Print(cmd, queue, agentpoolQueueColumns...)
}

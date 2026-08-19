package pipelines

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// agentpoolQueueColumns is the table shape shared by queue list/show
// (_format.py:299-317 _transform_pipeline_queue_row).
var agentpoolQueueColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Field: "name"},
	{Header: "Pool IsHosted", Field: "pool.isHosted"},
	{Header: "Pool Type", Field: "pool.poolType"},
}

// agentpoolNewQueueListCmd implements `az pipelines queue list`
// (list_queues, agent_pool_queue.py:89).
func agentpoolNewQueueListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List agent queues",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return agentpoolRunQueueList(context.Background(), cmd)
		},
	}

	cmd.Flags().String("queue-name", "", "Filter the list with matching queue name regex. e.g. *ubuntu* for queue with name 'Hosted Ubuntu 1604'.")
	cmd.Flags().String("action", "", "Filter by whether the calling user has use or manage permissions: use, manage, none.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func agentpoolRunQueueList(ctx context.Context, cmd *cobra.Command) error {
	queueName, _ := cmd.Flags().GetString("queue-name")
	action, _ := cmd.Flags().GetString("action")
	action, err := agentpoolValidateChoice(action, "action", agentpoolActionChoices)
	if err != nil {
		return err
	}

	// queue, unlike pool/agent, IS project-scoped
	// (agent_pool_queue.py:97 resolve_instance_and_project) — ado.ResolveProject.
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return agentpoolQueueList(ctx, cmd, client, dctx.Project, queueName, action)
}

// agentpoolQueueList does the HTTP work, split out from agentpoolRunQueueList
// so tests can drive it against an httptest server directly.
func agentpoolQueueList(ctx context.Context, cmd *cobra.Command, client *ado.Client, project, queueName, action string) error {
	q := url.Values{}
	if queueName != "" {
		q.Set("queueName", queueName)
	}
	if action != "" {
		q.Set("actionFilter", action)
	}

	var queues []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      project,
		Path:       "distributedtask/queues",
		APIVersion: "5.1-preview.1",
		Query:      q,
	}, &queues); err != nil {
		return fmt.Errorf("failed to list agent queues: %w", err)
	}

	return ado.Print(cmd, queues, agentpoolQueueColumns...)
}

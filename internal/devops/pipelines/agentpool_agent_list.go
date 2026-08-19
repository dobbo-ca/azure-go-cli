package pipelines

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// agentpoolAgentColumns is the table shape shared by agent list/show
// (_format.py:277-296 _transform_pipeline_agent_row).
var agentpoolAgentColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Field: "name"},
	{Header: "Is Enabled", Field: "enabled"},
	{Header: "Status", Field: "status"},
	{Header: "Version", Field: "version"},
}

// agentpoolNewAgentListCmd implements `az pipelines agent list` (list_agents,
// agent_pool_queue.py:41).
func agentpoolNewAgentListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Get a list of agents in a pool",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return agentpoolRunAgentList(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("pool-id", 0, "The agent pool containing the agents.")
	cmd.MarkFlagRequired("pool-id")
	cmd.Flags().String("agent-name", "", "Filter on agent name.")
	agentpoolAddThreeStateFlag(cmd, "include-capabilities", "Whether to include the agents' capabilities in the response.")
	agentpoolAddThreeStateFlag(cmd, "include-assigned-request", "Whether to include details about the agents' current work.")
	agentpoolAddThreeStateFlag(cmd, "include-last-completed-request", "Whether to include details about the agents' most recent completed work.")
	// Comma-separated string, not a repeatable flag: agent_pool_queue.py:59-60
	// splits on "," client-side (and the SDK immediately rejoins with ",").
	// --demands "a,b", not space-separated like --variables/--tags.
	cmd.Flags().String("demands", "", "Filter by demands the agents can satisfy. Comma separated list.")
	ado.AddOrgFlags(cmd)

	return cmd
}

func agentpoolRunAgentList(ctx context.Context, cmd *cobra.Command) error {
	poolID, _ := cmd.Flags().GetInt("pool-id")
	agentName, _ := cmd.Flags().GetString("agent-name")
	demands, _ := cmd.Flags().GetString("demands")

	includeCapabilities, err := agentpoolThreeState(cmd, "include-capabilities")
	if err != nil {
		return err
	}
	includeAssignedRequest, err := agentpoolThreeState(cmd, "include-assigned-request")
	if err != nil {
		return err
	}
	includeLastCompletedRequest, err := agentpoolThreeState(cmd, "include-last-completed-request")
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

	return agentpoolAgentList(ctx, cmd, client, poolID, agentName, demands,
		includeCapabilities, includeAssignedRequest, includeLastCompletedRequest)
}

// agentpoolAgentList does the HTTP work, split out from agentpoolRunAgentList
// so tests can drive it against an httptest server directly.
func agentpoolAgentList(ctx context.Context, cmd *cobra.Command, client *ado.Client, poolID int, agentName, demands string,
	includeCapabilities, includeAssignedRequest, includeLastCompletedRequest *bool) error {
	q := url.Values{}
	if agentName != "" {
		q.Set("agentName", agentName)
	}
	if includeCapabilities != nil {
		q.Set("includeCapabilities", strconv.FormatBool(*includeCapabilities))
	}
	if includeAssignedRequest != nil {
		q.Set("includeAssignedRequest", strconv.FormatBool(*includeAssignedRequest))
	}
	if includeLastCompletedRequest != nil {
		q.Set("includeLastCompletedRequest", strconv.FormatBool(*includeLastCompletedRequest))
	}
	if demands != "" {
		q.Set("demands", demands)
	}

	var agents []map[string]any
	if err := client.List(ctx, ado.Request{
		Path:       fmt.Sprintf("distributedtask/pools/%d/agents", poolID),
		APIVersion: "5.1",
		Query:      q,
	}, &agents); err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	return ado.Print(cmd, agents, agentpoolAgentColumns...)
}

package pipelines

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// agentpoolNewAgentShowCmd implements `az pipelines agent show` (show_agent,
// agent_pool_queue.py:67).
func agentpoolNewAgentShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get details about a specific agent",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return agentpoolRunAgentShow(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("pool-id", 0, "The agent pool containing the agent.")
	cmd.MarkFlagRequired("pool-id")
	// NOT aliased to --id: the --id<->--pool-id alias in arguments.py:91 is
	// scoped to `pipelines pool`, which does not apply to `pipelines agent`.
	cmd.Flags().String("agent-id", "", "The agent ID to get information about.")
	cmd.Flags().String("id", "", "Alias for --agent-id.")
	agentpoolAddThreeStateFlag(cmd, "include-capabilities", "Whether to include the agents' capabilities in the response.")
	agentpoolAddThreeStateFlag(cmd, "include-assigned-request", "Whether to include details about the agents' current work.")
	agentpoolAddThreeStateFlag(cmd, "include-last-completed-request", "Whether to include details about the agents' most recent completed work.")
	ado.AddOrgFlags(cmd)

	return cmd
}

func agentpoolRunAgentShow(ctx context.Context, cmd *cobra.Command) error {
	poolID, _ := cmd.Flags().GetInt("pool-id")
	agentID, err := agentpoolRequiredStringFlag(cmd, "agent-id", "id")
	if err != nil {
		return err
	}

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

	return agentpoolAgentShow(ctx, cmd, client, poolID, agentID,
		includeCapabilities, includeAssignedRequest, includeLastCompletedRequest)
}

// agentpoolAgentShow does the HTTP work, split out from agentpoolRunAgentShow
// so tests can drive it against an httptest server directly.
func agentpoolAgentShow(ctx context.Context, cmd *cobra.Command, client *ado.Client, poolID int, agentID string,
	includeCapabilities, includeAssignedRequest, includeLastCompletedRequest *bool) error {
	q := url.Values{}
	if includeCapabilities != nil {
		q.Set("includeCapabilities", strconv.FormatBool(*includeCapabilities))
	}
	if includeAssignedRequest != nil {
		q.Set("includeAssignedRequest", strconv.FormatBool(*includeAssignedRequest))
	}
	if includeLastCompletedRequest != nil {
		q.Set("includeLastCompletedRequest", strconv.FormatBool(*includeLastCompletedRequest))
	}

	var agent map[string]any
	if err := client.Do(ctx, ado.Request{
		Path:       fmt.Sprintf("distributedtask/pools/%d/agents/%s", poolID, url.PathEscape(agentID)),
		APIVersion: "5.1",
		Query:      q,
	}, &agent); err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	return ado.Print(cmd, agent, agentpoolAgentColumns...)
}

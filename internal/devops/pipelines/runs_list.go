package pipelines

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

var runsBuildReasonValues = []string{
	"all", "batchedCI", "buildCompletion", "checkInShelveset", "individualCI", "manual",
	"pullRequest", "schedule", "triggered", "userCreated", "validateShelveset",
}

var runsBuildResultValues = []string{"canceled", "failed", "none", "partiallySucceeded", "succeeded"}

var runsBuildStatusValues = []string{
	"all", "cancelling", "completed", "inProgress", "none", "notStarted", "postponed",
}

// runsQueryOrderValues is pipelines/arguments.py's _PIPELINES_RUNS_QUERY_ORDER
// — a different enum set from `pipelines list`'s --query-order (no "Name*"
// options; Start/Finish/Queue time variants instead).
var runsQueryOrderValues = []string{
	"FinishTimeAsc", "FinishTimeDesc", "StartTimeAsc", "StartTimeDesc", "QueueTimeAsc", "QueueTimeDesc",
}

// runsQueryOrderTarget maps the CLI-facing enum value (lowercased) to the
// server's queryOrder value, per pipeline_run.py:79-87
// (_resolve_runs_query_order's substring match, which — because the CLI
// argument is already restricted to runsQueryOrderValues by validation
// below — always resolves to exactly one of these).
var runsQueryOrderTarget = map[string]string{
	"finishtimeasc":  "finishTimeAscending",
	"finishtimedesc": "finishTimeDescending",
	"starttimeasc":   "startTimeAscending",
	"starttimedesc":  "startTimeDescending",
	"queuetimeasc":   "queueTimeAscending",
	"queuetimedesc":  "queueTimeDescending",
}

func newRunsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List runs for a pipeline.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunsList(context.Background(), cmd)
		},
	}

	// --pipeline-ids and --tags are nargs='*' in Python (space separated;
	// commas are ordinary characters in a tag value); StringArray, not
	// StringSlice, per foundation convention (StringSlice comma-splits and
	// would corrupt a tag containing a comma).
	cmd.Flags().StringArray("pipeline-ids", nil, "Space separated IDs of pipelines to list runs for. For multiple pipeline ids:  --pipeline-ids 1 2")
	cmd.Flags().String("branch", "", "Filter by builds for this branch.")
	cmd.Flags().Int("top", 0, "Maximum number of runs to list.")
	cmd.Flags().String("query-order", "", fmt.Sprintf("Order of pipeline runs. Allowed values: %s.", strings.Join(runsQueryOrderValues, ", ")))
	cmd.Flags().String("result", "", fmt.Sprintf("Limit to runs with this result. Allowed values: %s.", strings.Join(runsBuildResultValues, ", ")))
	cmd.Flags().String("status", "", fmt.Sprintf("Limit to runs with this status. Allowed values: %s.", strings.Join(runsBuildStatusValues, ", ")))
	cmd.Flags().String("reason", "", fmt.Sprintf("Limit to runs with this reason. Allowed values: %s.", strings.Join(runsBuildReasonValues, ", ")))
	cmd.Flags().StringArray("tags", nil, "Space separated tags, limit to runs with each of the specified tags.")
	cmd.Flags().String("requested-for", "", "Limit to runs requested for this user or group.")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func runRunsList(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	pipelineIDStrs, _ := cmd.Flags().GetStringArray("pipeline-ids")
	branch, _ := cmd.Flags().GetString("branch")
	top, _ := cmd.Flags().GetInt("top")
	queryOrder, _ := cmd.Flags().GetString("query-order")
	result, _ := cmd.Flags().GetString("result")
	status, _ := cmd.Flags().GetString("status")
	reason, _ := cmd.Flags().GetString("reason")
	tags, _ := cmd.Flags().GetStringArray("tags")
	requestedFor, _ := cmd.Flags().GetString("requested-for")

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	// pipeline_run.py:75 resolves the filter through the Identity REST API
	// first (resolve_identity_as_id) so "me", an email, or an alias resolve
	// to the id the server actually filters on.
	requestedForID, err := pipelinesResolveIdentityID(ctx, client, requestedFor)
	if err != nil {
		return err
	}

	q, err := runsListQuery(pipelineIDStrs, branch, top, queryOrder, result, status, reason, tags, requestedForID)
	if err != nil {
		return err
	}

	var runs []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "build/Builds",
		APIVersion: "5.0",
		Query:      q,
	}, &runs); err != nil {
		return fmt.Errorf("failed to list runs: %w", err)
	}

	return ado.Print(cmd, runs, runsColumns...)
}

// runsListQuery builds the `GET .../build/Builds` query parameters for
// `pipelines runs list` (pipeline_run.py:35-75), pulled out of runRunsList
// so it's directly unit-testable without an HTTP round trip.
func runsListQuery(pipelineIDStrs []string, branch string, top int, queryOrder, result, status, reason string, tags []string, requestedFor string) (url.Values, error) {
	if err := runsValidateChoice("result", result, runsBuildResultValues); err != nil {
		return nil, err
	}
	if err := runsValidateChoice("status", status, runsBuildStatusValues); err != nil {
		return nil, err
	}
	if err := runsValidateChoice("reason", reason, runsBuildReasonValues); err != nil {
		return nil, err
	}

	q := url.Values{}

	if len(pipelineIDStrs) > 0 {
		ids, err := runsDedupInts(pipelineIDStrs)
		if err != nil {
			return nil, fmt.Errorf("invalid --pipeline-ids: %w", err)
		}
		q.Set("definitions", strings.Join(ids, ","))
	}
	if len(tags) > 0 {
		q.Set("tagFilters", strings.Join(runsDedupStrings(tags), ","))
	}
	if branch != "" {
		q.Set("branchName", coreResolveGitRefHeads(branch))
	}
	if top > 0 {
		q.Set("$top", strconv.Itoa(top))
	}
	if result != "" {
		q.Set("resultFilter", result)
	}
	if status != "" {
		q.Set("statusFilter", status)
	}
	if reason != "" {
		q.Set("reasonFilter", reason)
	}
	if queryOrder != "" {
		target, ok := runsQueryOrderTarget[strings.ToLower(queryOrder)]
		if !ok {
			return nil, fmt.Errorf("--query-order must be one of %s", strings.Join(runsQueryOrderValues, ", "))
		}
		q.Set("queryOrder", target)
	}
	if requestedFor != "" {
		// requestedFor is already resolved to an identity id by the caller
		// (pipelinesResolveIdentityID, mirroring pipeline_run.py:75's
		// resolve_identity_as_id) before runsListQuery ever sees it.
		q.Set("requestedFor", requestedFor)
	}

	return q, nil
}

// runsValidateChoice matches enum_choice_list's case-insensitive validation
// (pipelines/arguments.py); empty is always allowed (the filter is optional).
func runsValidateChoice(flag, value string, choices []string) error {
	if value == "" {
		return nil
	}
	for _, c := range choices {
		if strings.EqualFold(c, value) {
			return nil
		}
	}
	return fmt.Errorf("--%s must be one of %s", flag, strings.Join(choices, ", "))
}

// runsDedupInts parses each string as an int and de-duplicates, preserving
// first-seen order (Python's list(set(pipeline_ids)) dedups too, just with
// unspecified order — see pipeline_run.py:63-64 and the surface doc's note
// that order is not a contract here).
func runsDedupInts(vals []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return nil, fmt.Errorf("%q is not a valid integer", v)
		}
		s := strconv.Itoa(n)
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out, nil
}

func runsDedupStrings(vals []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

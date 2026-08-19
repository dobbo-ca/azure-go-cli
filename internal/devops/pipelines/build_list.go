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

var buildResultChoices = []string{"canceled", "failed", "none", "partiallySucceeded", "succeeded"}
var buildStatusChoices = []string{"all", "cancelling", "completed", "inProgress", "none", "notStarted", "postponed"}
var buildReasonChoices = []string{
	"all", "batchedCI", "buildCompletion", "checkInShelveset", "individualCI", "manual",
	"pullRequest", "schedule", "triggered", "userCreated", "validateShelveset",
}

func buildChoiceOK(v string, choices []string) bool {
	if v == "" {
		return true
	}
	for _, c := range choices {
		if v == c {
			return true
		}
	}
	return false
}

// newBuildListCmd implements `az pipelines build list` (build_list,
// dev/pipelines/build.py:86).
func newBuildListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List build results",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildRunList(context.Background(), cmd)
		},
	}

	// StringArray, not StringSlice: Python's definition_ids/tags are
	// argparse nargs='*' (space separated; commas are ordinary characters
	// in a tag value).
	cmd.Flags().StringArray("definition-ids", nil, "IDs (space separated) of definitions to list builds for.")
	cmd.Flags().String("branch", "", "Filter by builds for this branch.")
	cmd.Flags().Int("top", 0, "Maximum number of builds to list.")
	cmd.Flags().String("result", "", "Limit to builds with this result: "+strings.Join(buildResultChoices, ", "))
	cmd.Flags().String("status", "", "Limit to builds with this status: "+strings.Join(buildStatusChoices, ", "))
	cmd.Flags().String("reason", "", "Limit to builds with this reason: "+strings.Join(buildReasonChoices, ", "))
	cmd.Flags().StringArray("tags", nil, "Limit to builds with each of the specified tags. Space separated.")
	cmd.Flags().String("requested-for", "", "Limit to builds requested for this user or group.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func buildRunList(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return buildList(ctx, cmd, client, dctx)
}

// buildDedup drops duplicates, keeping first-seen order. Python dedups via
// `list(set(...))`, which does not preserve input order at all — keeping
// first-seen order here is a deterministic, equally-valid reading of "make
// distinct" (build.py:107,109).
func buildDedup(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, v := range items {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// buildDedupInts ports arguments.py:34's `type=int` on --definition-ids
// (argparse rejects a non-integer value at parse time) plus build.py:107's
// dedup, mirroring runs_list.go's runsDedupInts for the sibling
// --pipeline-ids.
func buildDedupInts(vals []string) ([]string, error) {
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

func buildList(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	definitionIDs, _ := cmd.Flags().GetStringArray("definition-ids")
	branch, _ := cmd.Flags().GetString("branch")
	top, _ := cmd.Flags().GetInt("top")
	result, _ := cmd.Flags().GetString("result")
	status, _ := cmd.Flags().GetString("status")
	reason, _ := cmd.Flags().GetString("reason")
	tags, _ := cmd.Flags().GetStringArray("tags")
	requestedFor, _ := cmd.Flags().GetString("requested-for")

	if !buildChoiceOK(result, buildResultChoices) {
		return fmt.Errorf("--result must be one of %s", strings.Join(buildResultChoices, ", "))
	}
	if !buildChoiceOK(status, buildStatusChoices) {
		return fmt.Errorf("--status must be one of %s", strings.Join(buildStatusChoices, ", "))
	}
	if !buildChoiceOK(reason, buildReasonChoices) {
		return fmt.Errorf("--reason must be one of %s", strings.Join(buildReasonChoices, ", "))
	}

	q := url.Values{}
	if len(definitionIDs) > 0 {
		ids, err := buildDedupInts(definitionIDs)
		if err != nil {
			return fmt.Errorf("invalid --definition-ids: %w", err)
		}
		q.Set("definitions", strings.Join(ids, ","))
	}
	if branch := coreResolveGitRefHeads(branch); branch != "" {
		q.Set("branchName", branch)
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
	if len(tags) > 0 {
		q.Set("tagFilters", strings.Join(buildDedup(tags), ","))
	}
	if requestedFor != "" {
		// build.py:122 resolves the filter through the Identity REST API
		// first (resolve_identity_as_id, dev/common/identities.py) so "me",
		// an email, or an alias resolve to the id the server actually
		// filters on.
		id, err := pipelinesResolveIdentityID(ctx, client, requestedFor)
		if err != nil {
			return err
		}
		q.Set("requestedFor", id)
	}

	var builds []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "build/Builds",
		APIVersion: "5.0",
		Query:      q,
	}, &builds); err != nil {
		return fmt.Errorf("failed to list builds: %w", err)
	}

	return ado.Print(cmd, builds, buildColumns...)
}

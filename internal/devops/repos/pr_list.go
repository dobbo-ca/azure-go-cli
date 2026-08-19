package repos

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// prListStatusValues is _PR_STATUS_VALUES, dev/repos/arguments.py:12.
var prListStatusValues = []string{"all", "active", "completed", "abandoned"}

func newPRListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pull requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunList(context.Background(), cmd)
		},
	}

	ado.AddRepoFlag(cmd)
	cmd.Flags().String("creator", "", "Limit results to pull requests created by this user.")
	cmd.Flags().Bool("include-links", false, "Include _links for each pull request.")
	cmd.Flags().String("reviewer", "", "Limit results to pull requests where this user is a reviewer.")
	cmd.Flags().StringP("source-branch", "s", "", "Limit results to pull requests that originate from this source branch.")
	cmd.Flags().String("status", "", "Limit results to pull requests with this status. Allowed values: all, active, completed, abandoned.")
	cmd.Flags().StringP("target-branch", "t", "", "Limit results to pull requests that target this branch.")
	cmd.Flags().Int("skip", 0, "Number of pull requests to skip.")
	cmd.Flags().Int("top", 0, "Maximum number of pull requests to list.")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func prRunList(ctx context.Context, cmd *cobra.Command) error {
	client, dctx, err := prClientProject(ctx, cmd)
	if err != nil {
		return err
	}
	return prListExec(ctx, cmd, client, dctx)
}

// prListExec does the actual work given an already-resolved client, split
// out from prRunList so tests can exercise it against an httptest server
// without going through ado.ResolveProject's org validation.
func prListExec(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	creator, _ := cmd.Flags().GetString("creator")
	reviewer, _ := cmd.Flags().GetString("reviewer")
	includeLinks, _ := cmd.Flags().GetBool("include-links")
	sourceBranch, _ := cmd.Flags().GetString("source-branch")
	targetBranch, _ := cmd.Flags().GetString("target-branch")
	status, _ := cmd.Flags().GetString("status")

	creatorID, err := prResolveIdentity(ctx, client, creator)
	if err != nil {
		return err
	}
	reviewerID, err := prResolveIdentity(ctx, client, reviewer)
	if err != nil {
		return err
	}

	q := url.Values{}
	// search_criteria.include_links defaults to False, not None, in Python
	// (list_pull_requests(..., include_links=False, ...)), so it is always
	// sent, not conditionally omitted.
	q.Set("searchCriteria.includeLinks", strconv.FormatBool(includeLinks))
	if creatorID != "" {
		q.Set("searchCriteria.creatorId", creatorID)
	}
	if reviewerID != "" {
		q.Set("searchCriteria.reviewerId", reviewerID)
	}
	if status != "" {
		valid := false
		for _, v := range prListStatusValues {
			if status == v {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("--status must be one of: %s", "all, active, completed, abandoned")
		}
		q.Set("searchCriteria.status", status)
	}
	if sourceBranch != "" {
		q.Set("searchCriteria.sourceRefName", policyResolveRefHeads(sourceBranch))
	}
	if targetBranch != "" {
		q.Set("searchCriteria.targetRefName", policyResolveRefHeads(targetBranch))
	}
	if cmd.Flags().Changed("skip") {
		skip, _ := cmd.Flags().GetInt("skip")
		q.Set("$skip", strconv.Itoa(skip))
	}
	if cmd.Flags().Changed("top") {
		top, _ := cmd.Flags().GetInt("top")
		q.Set("$top", strconv.Itoa(top))
	}

	repo, _ := cmd.Flags().GetString("repository")
	if repo == "" {
		repo = dctx.Repo
	}

	path := "git/pullRequests"
	if repo != "" {
		path = "git/repositories/" + url.PathEscape(repo) + "/pullRequests"
	}

	var prs []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       path,
		APIVersion: "5.0",
		Query:      q,
	}, &prs); err != nil {
		return fmt.Errorf("failed to list pull requests: %w", err)
	}

	return ado.Print(cmd, prs, prColumns...)
}

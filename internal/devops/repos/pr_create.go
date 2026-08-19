package repos

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPRCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().StringP("source-branch", "s", "", "Name of the source branch. Example: \"dev\".")
	cmd.Flags().StringP("target-branch", "t", "", "Name of the target branch. If not specified, defaults to the default branch of the target repository.")
	cmd.Flags().String("title", "", "Title for the new pull request.")
	cmd.Flags().StringArrayP("description", "d", nil, "Description for the new pull request. Can include markdown. Each value sent to this arg will be a new line.")
	policyAddTriStateFlag(cmd, "draft", "Use this flag to create the pull request in draft/work in progress mode.")
	policyAddTriStateFlag(cmd, "auto-complete", "Set the pull request to complete automatically when all policies have passed and the source branch can be merged into the target branch.")
	policyAddTriStateFlag(cmd, "squash", "Squash the commits in the source branch when merging into the target branch.")
	policyAddTriStateFlag(cmd, "delete-source-branch", "Delete the source branch after the pull request has been completed and merged into the target branch.")
	policyAddTriStateFlag(cmd, "bypass-policy", "Bypass required policies (if any) and completes the pull request once it can be merged.")
	cmd.Flags().String("bypass-policy-reason", "", "Reason for bypassing the required policies.")
	cmd.Flags().String("merge-commit-message", "", "Message displayed when commits are merged.")
	cmd.Flags().StringArray("reviewers", nil, "Additional users or groups to include as optional reviewers on the new pull request. Space separated.")
	cmd.Flags().StringArray("optional-reviewers", nil, "Alias for --reviewers.")
	cmd.Flags().StringArray("required-reviewers", nil, "Additional users or groups to include as required reviewers on the new pull request. Space separated.")
	cmd.Flags().StringSlice("work-items", nil, "IDs of the work items to link to the new pull request. Space separated.")
	cmd.Flags().Bool("open", false, "Open the pull request in your web browser.")
	policyAddTriStateFlag(cmd, "transition-work-items", "Transition any work items linked to the pull request into the next logical state. (e.g. Active -> Resolved)")
	cmd.Flags().String("labels", "", "The labels associated with the pull request. Space separated.")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddRepoFlag(cmd)

	return cmd
}

// prCurrentBranchName ports get_current_branch_name, dev/common/git.py:45-56.
func prCurrentBranchName() (string, bool) {
	out, err := exec.Command("git", "symbolic-ref", "--short", "-q", "HEAD").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func prRunCreate(ctx context.Context, cmd *cobra.Command) error {
	// Deviation: create_pull_request's `repository` parameter is optional in
	// Python (resolve_instance_project_and_repo's default repo_required=False),
	// but the create call always needs a concrete repository id/name in its
	// URL — passing none is a broken call, not a documented "project-wide
	// create". ado.ResolveRepo requires --project and --repository up front
	// instead of letting that failure surface from the server.
	client, dctx, err := prClientRepo(ctx, cmd)
	if err != nil {
		return err
	}
	return prCreateExec(ctx, cmd, client, dctx)
}

// prCreateExec does the actual work given an already-resolved client and
// context, split out from prRunCreate so tests can exercise it against an
// httptest server without going through ado.ResolveRepo's org validation.
func prCreateExec(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	sourceBranch, _ := cmd.Flags().GetString("source-branch")
	targetBranch, _ := cmd.Flags().GetString("target-branch")
	title, _ := cmd.Flags().GetString("title")
	hasTitle := cmd.Flags().Changed("title")
	description, _ := cmd.Flags().GetStringArray("description")
	hasDescription := cmd.Flags().Changed("description")
	draft, err := policyTriState(cmd, "draft")
	if err != nil {
		return err
	}
	autoCompleteVal, err := policyTriState(cmd, "auto-complete")
	if err != nil {
		return err
	}
	autoComplete := autoCompleteVal != nil && *autoCompleteVal
	squashVal, err := policyTriState(cmd, "squash")
	if err != nil {
		return err
	}
	squash := squashVal != nil && *squashVal
	deleteSourceBranchVal, err := policyTriState(cmd, "delete-source-branch")
	if err != nil {
		return err
	}
	deleteSourceBranch := deleteSourceBranchVal != nil && *deleteSourceBranchVal
	bypassPolicyVal, err := policyTriState(cmd, "bypass-policy")
	if err != nil {
		return err
	}
	bypassPolicy := bypassPolicyVal != nil && *bypassPolicyVal
	bypassPolicyReason, _ := cmd.Flags().GetString("bypass-policy-reason")
	hasBypassReason := cmd.Flags().Changed("bypass-policy-reason")
	mergeCommitMessage, _ := cmd.Flags().GetString("merge-commit-message")
	hasMergeMessage := cmd.Flags().Changed("merge-commit-message")
	reviewers, _ := cmd.Flags().GetStringArray("reviewers")
	if len(reviewers) == 0 {
		// ponytail: --reviewers/--optional-reviewers are the same Python
		// argument under two option strings; when both are given on the
		// command line the last one wins there. cobra can't see argv order
		// across two independently-registered flags, so --reviewers wins
		// deterministically here — same tradeoff as ado.AddOrgFlags'
		// --organization/--org.
		reviewers, _ = cmd.Flags().GetStringArray("optional-reviewers")
	}
	requiredReviewers, _ := cmd.Flags().GetStringArray("required-reviewers")
	workItems, _ := cmd.Flags().GetStringSlice("work-items")
	open, _ := cmd.Flags().GetBool("open")
	transitionWorkItemsVal, err := policyTriState(cmd, "transition-work-items")
	if err != nil {
		return err
	}
	transitionWorkItems := transitionWorkItemsVal != nil && *transitionWorkItemsVal
	labels, _ := cmd.Flags().GetString("labels")

	detectFlag, _ := cmd.Flags().GetString("detect")
	detectOn := !strings.EqualFold(detectFlag, "false")

	if sourceBranch == "" {
		if detectOn {
			b, ok := prCurrentBranchName()
			if !ok {
				return fmt.Errorf("the source branch could not be detected, please provide the --source-branch argument")
			}
			sourceBranch = b
		} else {
			return fmt.Errorf("--source-branch is a required argument")
		}
	}

	if targetBranch == "" {
		repo, err := prGetRepo(ctx, client, dctx.Project, dctx.Repo)
		if err != nil {
			return err
		}
		targetBranch, _ = repo["defaultBranch"].(string)
		if targetBranch == "" {
			return fmt.Errorf("the target branch could not be detected, please provide the --target-branch argument")
		}
	}

	// pull_request.py:175-180: the default title is built from the raw
	// (unprefixed) branch names, computed before source/target are
	// ref-resolved below.
	prTitle := "Merge " + sourceBranch + " to " + targetBranch
	if hasTitle {
		prTitle = title
	}

	sourceRef := policyResolveRefHeads(sourceBranch)
	targetRef := policyResolveRefHeads(targetBranch)
	if sourceRef == targetRef {
		return fmt.Errorf("the source branch, %q, can not be the same as the target branch", sourceRef)
	}

	optLower := prLowerDedupe(reviewers)
	reqLower := prLowerDedupe(requiredReviewers)
	resolvedReviewers, err := prResolveReviewersAsRefs(ctx, client, optLower, reqLower)
	if err != nil {
		return err
	}

	body := map[string]any{
		"sourceRefName": sourceRef,
		"targetRefName": targetRef,
		"title":         prTitle,
		"reviewers":     resolvedReviewers,
	}
	if hasDescription {
		body["description"] = strings.Join(description, "\n")
	}
	if draft != nil {
		body["isDraft"] = *draft
	}
	if labels != "" {
		// labels.split(' '): a literal double space yields an empty label
		// entry — kept verbatim, matching pull_request.py:163.
		parts := strings.Split(labels, " ")
		tags := make([]map[string]string, len(parts))
		for i, p := range parts {
			tags[i] = map[string]string{"name": p}
		}
		body["labels"] = tags
	}
	if len(workItems) > 0 {
		refs := make([]map[string]string, len(workItems))
		for i, w := range workItems {
			refs[i] = map[string]string{"id": w}
		}
		body["workItemRefs"] = refs
	}

	var pr map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Scope:      dctx.Project,
		Path:       "git/repositories/" + url.PathEscape(dctx.Repo) + "/pullRequests",
		APIVersion: "5.0",
		Body:       body,
	}, &pr); err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}

	prID := prIDString(pr)

	var titleFromCommit, descFromCommit string
	if !hasTitle || !hasDescription {
		var commits []map[string]any
		if err := client.List(ctx, ado.Request{
			Scope:      dctx.Project,
			Path:       "git/repositories/" + url.PathEscape(dctx.Repo) + "/pullRequests/" + prID + "/commits",
			APIVersion: "5.0",
		}, &commits); err != nil {
			return fmt.Errorf("failed to get pull request commits: %w", err)
		}
		if len(commits) == 1 {
			commitID, _ := commits[0]["commitId"].(string)
			var commit map[string]any
			if err := client.Do(ctx, ado.Request{
				Scope:      dctx.Project,
				Path:       "git/repositories/" + url.PathEscape(dctx.Repo) + "/commits/" + url.PathEscape(commitID),
				APIVersion: "5.0",
				Query:      url.Values{"changeCount": {"0"}},
			}, &commit); err != nil {
				return fmt.Errorf("failed to get commit: %w", err)
			}
			if comment, _ := commit["comment"].(string); comment != "" {
				if !hasTitle {
					titleFromCommit = strings.SplitN(comment, "\n", 2)[0]
				}
				if !hasDescription {
					descFromCommit = comment
				}
			}
		}
	}

	setCompletionOptions := bypassPolicy || hasBypassReason || squash || hasMergeMessage ||
		deleteSourceBranch || transitionWorkItems

	if autoComplete || setCompletionOptions || titleFromCommit != "" || descFromCommit != "" {
		update := map[string]any{}
		if autoComplete {
			me, err := prCurrentIdentityID(ctx, client)
			if err != nil {
				return err
			}
			update["autoCompleteSetBy"] = map[string]string{"id": me}
		}
		if setCompletionOptions {
			completionOptions := map[string]any{
				"bypassPolicy":        bypassPolicy,
				"deleteSourceBranch":  deleteSourceBranch,
				"squashMerge":         squash,
				"transitionWorkItems": transitionWorkItems,
			}
			if hasBypassReason {
				completionOptions["bypassReason"] = bypassPolicyReason
			}
			if hasMergeMessage {
				completionOptions["mergeCommitMessage"] = mergeCommitMessage
			}
			update["completionOptions"] = completionOptions
		}
		if titleFromCommit != "" {
			update["title"] = titleFromCommit
		}
		if descFromCommit != "" {
			update["description"] = descFromCommit
		}

		repoID, projectID := prRepoProjectID(pr)
		if err := client.Do(ctx, ado.Request{
			Method:     http.MethodPatch,
			Scope:      projectID,
			Path:       "git/repositories/" + url.PathEscape(repoID) + "/pullRequests/" + prID,
			APIVersion: "5.0",
			Body:       update,
		}, &pr); err != nil {
			return fmt.Errorf("failed to update pull request: %w", err)
		}
	}

	if open {
		prOpenInBrowser(dctx.Org, pr)
	}

	return ado.Print(cmd, pr, prColumns...)
}

// prGetRepo fetches the repository to read its default branch when
// --target-branch is omitted (_get_default_branch, pull_request.py:687-690).
func prGetRepo(ctx context.Context, client *ado.Client, project, repo string) (map[string]any, error) {
	var out map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      project,
		Path:       "git/repositories/" + url.PathEscape(repo),
		APIVersion: "5.0",
	}, &out); err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	return out, nil
}

// prResolveReviewersAsRefs ports _resolve_reviewers_as_refs,
// pull_request.py:637-660. Faithful quirk kept: a required reviewer already
// present as optional gets the existing entry flagged required AND a second
// entry appended for it — Python always appends, it never skips the append
// after finding a match (pull_request.py:657-660's `continue` only exits the
// inner loop, not the outer one).
func prResolveReviewersAsRefs(ctx context.Context, client *ado.Client, optional, required []string) ([]map[string]any, error) {
	resolved := []map[string]any{}

	for _, r := range optional {
		id, err := prResolveIdentity(ctx, client, r)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, map[string]any{"id": id})
	}

	for _, r := range required {
		id, err := prResolveIdentity(ctx, client, r)
		if err != nil {
			return nil, err
		}
		for _, existing := range resolved {
			if existing["id"] == id {
				existing["isRequired"] = true
			}
		}
		resolved = append(resolved, map[string]any{"id": id, "isRequired": true})
	}

	return resolved, nil
}

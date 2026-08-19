package repos

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// prUpdateTargetStatusValues is _PR_TARGET_STATUS_VALUES, dev/repos/arguments.py:13.
var prUpdateTargetStatusValues = []string{"active", "completed", "abandoned"}

func newPRUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunUpdate(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the pull request.")
	cmd.MarkFlagRequired("id")
	cmd.Flags().String("title", "", "New title for the pull request.")
	cmd.Flags().StringArrayP("description", "d", nil, "New description for the pull request. Can include markdown. Each value sent to this arg will be a new line.")
	policyAddTriStateFlag(cmd, "auto-complete", "Set the pull request to complete automatically when all policies have passed and the source branch can be merged into the target branch.")
	policyAddTriStateFlag(cmd, "squash", "Squash the commits in the source branch when merging into the target branch.")
	policyAddTriStateFlag(cmd, "delete-source-branch", "Delete the source branch after the pull request has been completed and merged into the target branch.")
	policyAddTriStateFlag(cmd, "bypass-policy", "Bypass required policies (if any) and completes the pull request once it can be merged.")
	cmd.Flags().String("bypass-policy-reason", "", "Reason for bypassing the required policies.")
	cmd.Flags().String("merge-commit-message", "", "Message displayed when commits are merged.")
	policyAddTriStateFlag(cmd, "draft", "Publish the PR or convert to draft mode.")
	policyAddTriStateFlag(cmd, "transition-work-items", "Transition any work items linked to the pull request into the next logical state. (e.g. Active -> Resolved)")
	cmd.Flags().String("status", "", "Set the new state of pull request. Allowed values: active, completed, abandoned.")

	ado.AddOrgFlags(cmd)

	return cmd
}

func prRunUpdate(ctx context.Context, cmd *cobra.Command) error {
	client, _, err := prClientOrg(ctx, cmd)
	if err != nil {
		return err
	}
	return prUpdateExec(ctx, cmd, client)
}

// prUpdateExec does the actual work given an already-resolved client, split
// out from prRunUpdate so tests can exercise it against an httptest server
// without going through ado.Resolve's org validation. No dctx parameter:
// update_pull_request declares no --project/--repository (see
// newPRUpdateCmd) — project/repo are read from the PR itself, below.
func prUpdateExec(ctx context.Context, cmd *cobra.Command, client *ado.Client) error {
	id, _ := cmd.Flags().GetInt("id")
	idStr := strconv.Itoa(id)

	// arguments.py:144 enum_choice_list(_PR_TARGET_STATUS_VALUES) validates
	// --status at parse time, before any HTTP call; validate here, ahead of
	// the GET below, so a bad value doesn't cost a request first.
	status, _ := cmd.Flags().GetString("status")
	if status != "" {
		valid := false
		for _, v := range prUpdateTargetStatusValues {
			if status == v {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("--status must be one of: %s", strings.Join(prUpdateTargetStatusValues, ", "))
		}
	}

	existing, err := prGetByID(ctx, client, idStr)
	if err != nil {
		return err
	}

	body := map[string]any{}
	if cmd.Flags().Changed("title") {
		title, _ := cmd.Flags().GetString("title")
		body["title"] = title
	}
	if cmd.Flags().Changed("description") {
		desc, _ := cmd.Flags().GetStringArray("description")
		body["description"] = strings.Join(desc, "\n")
	}

	bypassPolicy, err := policyTriState(cmd, "bypass-policy")
	if err != nil {
		return err
	}
	squash, err := policyTriState(cmd, "squash")
	if err != nil {
		return err
	}
	deleteSourceBranch, err := policyTriState(cmd, "delete-source-branch")
	if err != nil {
		return err
	}
	transitionWorkItems, err := policyTriState(cmd, "transition-work-items")
	if err != nil {
		return err
	}
	bypassReasonChanged := cmd.Flags().Changed("bypass-policy-reason")
	bypassPolicyReason, _ := cmd.Flags().GetString("bypass-policy-reason")
	mergeMsgChanged := cmd.Flags().Changed("merge-commit-message")
	mergeCommitMessage, _ := cmd.Flags().GetString("merge-commit-message")

	if bypassPolicy != nil || bypassReasonChanged || squash != nil || mergeMsgChanged ||
		deleteSourceBranch != nil || transitionWorkItems != nil {
		completionOptions := map[string]any{}
		if co, ok := existing["completionOptions"].(map[string]any); ok {
			for k, v := range co {
				completionOptions[k] = v
			}
		}
		if bypassPolicy != nil {
			completionOptions["bypassPolicy"] = *bypassPolicy
		}
		if bypassReasonChanged {
			completionOptions["bypassReason"] = bypassPolicyReason
		}
		if deleteSourceBranch != nil {
			completionOptions["deleteSourceBranch"] = *deleteSourceBranch
		}
		if squash != nil {
			completionOptions["squashMerge"] = *squash
		}
		if mergeMsgChanged {
			completionOptions["mergeCommitMessage"] = mergeCommitMessage
		}
		if transitionWorkItems != nil {
			completionOptions["transitionWorkItems"] = *transitionWorkItems
		}
		body["completionOptions"] = completionOptions
	}

	autoComplete, err := policyTriState(cmd, "auto-complete")
	if err != nil {
		return err
	}
	if autoComplete != nil {
		if *autoComplete {
			me, err := prCurrentIdentityID(ctx, client)
			if err != nil {
				return err
			}
			body["autoCompleteSetBy"] = map[string]string{"id": me}
		} else {
			// EMPTY_UUID, dev/common/uuid.py:18.
			body["autoCompleteSetBy"] = map[string]string{"id": "00000000-0000-0000-0000-000000000000"}
		}
	}

	draft, err := policyTriState(cmd, "draft")
	if err != nil {
		return err
	}
	if draft != nil {
		body["isDraft"] = *draft
	}

	if status != "" {
		body["status"] = status
		if status == "completed" {
			if lm, ok := existing["lastMergeSourceCommit"]; ok {
				body["lastMergeSourceCommit"] = lm
			}
			if _, has := body["completionOptions"]; !has {
				if co, ok := existing["completionOptions"]; ok {
					body["completionOptions"] = co
				}
			}
		}
	}

	// update_pull_request PATCHes by repository/project *name*, not id
	// (pull_request.py:355-356) — unlike almost every other call in this
	// surface, which prefers GUIDs.
	repoName, projectName := prRepoNames(existing)

	var pr map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Scope:      projectName,
		Path:       "git/repositories/" + url.PathEscape(repoName) + "/pullRequests/" + idStr,
		APIVersion: "5.0",
		Body:       body,
	}, &pr); err != nil {
		return fmt.Errorf("failed to update pull request: %w", err)
	}

	return ado.Print(cmd, pr, prColumns...)
}

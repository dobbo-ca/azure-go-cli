package repos

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPRReviewerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reviewer",
		Short: "Manage reviewers of a pull request",
	}
	cmd.AddCommand(newPRReviewerAddCmd())
	cmd.AddCommand(newPRReviewerListCmd())
	cmd.AddCommand(newPRReviewerRemoveCmd())
	return cmd
}

func newPRReviewerAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add one or more reviewers to a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunReviewerAdd(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the pull request.")
	cmd.MarkFlagRequired("id")
	cmd.Flags().StringSlice("reviewers", nil, "Users or groups to include as reviewers on a pull request. Space separated.")
	cmd.MarkFlagRequired("reviewers")
	policyAddTriStateFlag(cmd, "required", "Make the reviewers required.")

	// create_pull_request_reviewers declares only organization/detect.
	ado.AddOrgFlags(cmd)

	return cmd
}

func prRunReviewerAdd(ctx context.Context, cmd *cobra.Command) error {
	client, _, err := prClientOrg(ctx, cmd)
	if err != nil {
		return err
	}
	return prReviewerAddExec(ctx, cmd, client)
}

// prReviewerAddExec does the actual work given an already-resolved client,
// split out from prRunReviewerAdd so tests can exercise it against an
// httptest server without going through ado.Resolve's org validation.
func prReviewerAddExec(ctx context.Context, cmd *cobra.Command, client *ado.Client) error {
	id, _ := cmd.Flags().GetInt("id")
	idStr := strconv.Itoa(id)
	reviewers, _ := cmd.Flags().GetStringSlice("reviewers")
	required, err := policyTriState(cmd, "required")
	if err != nil {
		return err
	}

	pr, err := prGetByID(ctx, client, idStr)
	if err != nil {
		return err
	}
	repoID, projectID := prRepoProjectID(pr)

	// create_pull_request_reviewers, unlike create_pull_request, does not
	// lower-case or dedupe --reviewers before resolving (pull_request.py:361-372).
	refs := make([]map[string]any, 0, len(reviewers))
	for _, r := range reviewers {
		rid, err := prResolveIdentity(ctx, client, r)
		if err != nil {
			return err
		}
		refs = append(refs, map[string]any{"id": rid})
	}
	if required != nil && *required {
		for _, ref := range refs {
			ref["isRequired"] = true
		}
	}

	var reviewersOut []map[string]any
	if err := client.List(ctx, ado.Request{
		Method:     http.MethodPost,
		Scope:      projectID,
		Path:       "git/repositories/" + url.PathEscape(repoID) + "/pullRequests/" + idStr + "/reviewers",
		APIVersion: "5.0",
		Body:       refs,
	}, &reviewersOut); err != nil {
		return fmt.Errorf("failed to add reviewers: %w", err)
	}

	prSortReviewersForTable(cmd, reviewersOut)
	return ado.Print(cmd, reviewersOut, prReviewerColumns...)
}

func newPRReviewerListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List reviewers of a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunReviewerList(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the pull request.")
	cmd.MarkFlagRequired("id")

	ado.AddOrgFlags(cmd)

	return cmd
}

func prRunReviewerList(ctx context.Context, cmd *cobra.Command) error {
	client, _, err := prClientOrg(ctx, cmd)
	if err != nil {
		return err
	}
	return prReviewerListExec(ctx, cmd, client)
}

// prReviewerListExec does the actual work given an already-resolved client,
// split out from prRunReviewerList so tests can exercise it against an
// httptest server without going through ado.Resolve's org validation.
func prReviewerListExec(ctx context.Context, cmd *cobra.Command, client *ado.Client) error {
	id, _ := cmd.Flags().GetInt("id")
	idStr := strconv.Itoa(id)

	pr, err := prGetByID(ctx, client, idStr)
	if err != nil {
		return err
	}
	repoID, projectID := prRepoProjectID(pr)

	reviewers, err := prListReviewers(ctx, client, projectID, repoID, idStr)
	if err != nil {
		return err
	}

	prSortReviewersForTable(cmd, reviewers)
	return ado.Print(cmd, reviewers, prReviewerColumns...)
}

func newPRReviewerRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove one or more reviewers from a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunReviewerRemove(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the pull request.")
	cmd.MarkFlagRequired("id")
	cmd.Flags().StringSlice("reviewers", nil, "Users or groups to remove as reviewers on a pull request. Space separated.")
	cmd.MarkFlagRequired("reviewers")

	ado.AddOrgFlags(cmd)

	return cmd
}

func prRunReviewerRemove(ctx context.Context, cmd *cobra.Command) error {
	client, _, err := prClientOrg(ctx, cmd)
	if err != nil {
		return err
	}
	return prReviewerRemoveExec(ctx, cmd, client)
}

// prReviewerRemoveExec does the actual work given an already-resolved
// client, split out from prRunReviewerRemove so tests can exercise it
// against an httptest server without going through ado.Resolve's org
// validation.
func prReviewerRemoveExec(ctx context.Context, cmd *cobra.Command, client *ado.Client) error {
	id, _ := cmd.Flags().GetInt("id")
	idStr := strconv.Itoa(id)
	reviewers, _ := cmd.Flags().GetStringSlice("reviewers")

	pr, err := prGetByID(ctx, client, idStr)
	if err != nil {
		return err
	}
	repoID, projectID := prRepoProjectID(pr)

	for _, r := range reviewers {
		rid, err := prResolveIdentity(ctx, client, r)
		if err != nil {
			return err
		}
		if err := client.Do(ctx, ado.Request{
			Method:     http.MethodDelete,
			Scope:      projectID,
			Path:       "git/repositories/" + url.PathEscape(repoID) + "/pullRequests/" + idStr + "/reviewers/" + url.PathEscape(rid),
			APIVersion: "5.0",
		}, nil); err != nil {
			return fmt.Errorf("failed to remove reviewer %s: %w", r, err)
		}
	}

	reviewersOut, err := prListReviewers(ctx, client, projectID, repoID, idStr)
	if err != nil {
		return err
	}

	prSortReviewersForTable(cmd, reviewersOut)
	return ado.Print(cmd, reviewersOut, prReviewerColumns...)
}

func prListReviewers(ctx context.Context, client *ado.Client, projectID, repoID, id string) ([]map[string]any, error) {
	var reviewers []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      projectID,
		Path:       "git/repositories/" + url.PathEscape(repoID) + "/pullRequests/" + id + "/reviewers",
		APIVersion: "5.0",
	}, &reviewers); err != nil {
		return nil, fmt.Errorf("failed to list reviewers: %w", err)
	}
	return reviewers, nil
}

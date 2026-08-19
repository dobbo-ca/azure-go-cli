package repos

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPRWorkItemCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work-item",
		Short: "Manage work items linked to a pull request",
	}
	cmd.AddCommand(newPRWorkItemAddCmd())
	cmd.AddCommand(newPRWorkItemListCmd())
	cmd.AddCommand(newPRWorkItemRemoveCmd())
	return cmd
}

func newPRWorkItemAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Link one or more work items to a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunWorkItemAdd(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the pull request.")
	cmd.MarkFlagRequired("id")
	cmd.Flags().StringSlice("work-items", nil, "IDs of the work items to link. Space separated.")
	cmd.MarkFlagRequired("work-items")

	ado.AddOrgFlags(cmd)

	return cmd
}

// prWorkItemArtifactURL builds the vstfs artifact URL used to link/unlink a
// work item to this PR (pull_request.py:439-440, 505-506). The "%2F"
// separators are literal text in the Python format string, not a
// double-encoding of "/".
func prWorkItemArtifactURL(projectID, repoID, prID string) string {
	return fmt.Sprintf("vstfs:///Git/PullRequestId/%s%%2F%s%%2F%s", projectID, repoID, prID)
}

func prRunWorkItemAdd(ctx context.Context, cmd *cobra.Command) error {
	client, _, err := prClientOrg(ctx, cmd)
	if err != nil {
		return err
	}
	return prWorkItemAddExec(ctx, cmd, client)
}

// prWorkItemAddExec does the actual work given an already-resolved client,
// split out from prRunWorkItemAdd so tests can exercise it against an
// httptest server without going through ado.Resolve's org validation.
func prWorkItemAddExec(ctx context.Context, cmd *cobra.Command, client *ado.Client) error {
	id, _ := cmd.Flags().GetInt("id")
	idStr := strconv.Itoa(id)
	workItems, _ := cmd.Flags().GetStringSlice("work-items")

	pr, err := prGetByID(ctx, client, idStr)
	if err != nil {
		return err
	}
	repoID, projectID := prRepoProjectID(pr)

	dedup := prDedupe(workItems)
	prURL := prWorkItemArtifactURL(projectID, repoID, idStr)

	for _, wi := range dedup {
		// "op": 0 is a literal JSON integer, not the string "add" —
		// JsonPatchOperation.op is untyped ('object') and msrest serializes
		// whatever Python value is assigned (pull_request.py:442). The
		// sibling `boards work-item relation add` command sends "add" as a
		// string for the same PATCH endpoint; this command does not.
		patch := []map[string]any{{
			"op":   0,
			"path": "/relations/-",
			"value": map[string]any{
				"rel":        "ArtifactLink",
				"url":        prURL,
				"attributes": map[string]any{"name": "Pull Request"},
			},
		}}
		if err := client.Do(ctx, ado.Request{
			Method:     http.MethodPatch,
			Path:       "wit/workItems/" + url.PathEscape(wi),
			APIVersion: "5.0",
			JSONPatch:  true,
			Body:       patch,
		}, nil); err != nil {
			var ae *ado.APIError
			if errors.As(err, &ae) && ae.Message == "Relation already exists." {
				continue
			}
			return fmt.Errorf("failed to link work item %s: %w", wi, err)
		}
	}

	items, err := prGetLinkedWorkItems(ctx, client, projectID, repoID, idStr)
	if err != nil {
		return err
	}

	return ado.Print(cmd, items, prWorkItemColumns...)
}

func newPRWorkItemListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List work items linked to a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunWorkItemList(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the pull request.")
	cmd.MarkFlagRequired("id")

	ado.AddOrgFlags(cmd)

	return cmd
}

func prRunWorkItemList(ctx context.Context, cmd *cobra.Command) error {
	client, _, err := prClientOrg(ctx, cmd)
	if err != nil {
		return err
	}
	return prWorkItemListExec(ctx, cmd, client)
}

// prWorkItemListExec does the actual work given an already-resolved client,
// split out from prRunWorkItemList so tests can exercise it against an
// httptest server without going through ado.Resolve's org validation.
func prWorkItemListExec(ctx context.Context, cmd *cobra.Command, client *ado.Client) error {
	id, _ := cmd.Flags().GetInt("id")
	idStr := strconv.Itoa(id)

	pr, err := prGetByID(ctx, client, idStr)
	if err != nil {
		return err
	}
	repoID, projectID := prRepoProjectID(pr)

	refs, err := prGetWorkItemRefs(ctx, client, projectID, repoID, idStr)
	if err != nil {
		return err
	}
	if len(refs) == 0 {
		// list_pull_request_work_items returns the raw (empty) refs list
		// directly when there are no linked work items, skipping the batch
		// work-item GET (pull_request.py:555-562) — same JSON shape ([]) as
		// the non-empty path either way.
		return ado.Print(cmd, refs, prWorkItemColumns...)
	}

	items, err := prBatchGetWorkItems(ctx, client, prRefIDs(refs), false)
	if err != nil {
		return err
	}

	return ado.Print(cmd, items, prWorkItemColumns...)
}

func newPRWorkItemRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Unlink one or more work items from a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunWorkItemRemove(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the pull request.")
	cmd.MarkFlagRequired("id")
	cmd.Flags().StringSlice("work-items", nil, "IDs of the work items to unlink. Space separated.")
	cmd.MarkFlagRequired("work-items")

	ado.AddOrgFlags(cmd)

	return cmd
}

func prRunWorkItemRemove(ctx context.Context, cmd *cobra.Command) error {
	client, _, err := prClientOrg(ctx, cmd)
	if err != nil {
		return err
	}
	return prWorkItemRemoveExec(ctx, cmd, client)
}

// prWorkItemRemoveExec does the actual work given an already-resolved
// client, split out from prRunWorkItemRemove so tests can exercise it
// against an httptest server without going through ado.Resolve's org
// validation.
func prWorkItemRemoveExec(ctx context.Context, cmd *cobra.Command, client *ado.Client) error {
	id, _ := cmd.Flags().GetInt("id")
	idStr := strconv.Itoa(id)
	workItems, _ := cmd.Flags().GetStringSlice("work-items")

	pr, err := prGetByID(ctx, client, idStr)
	if err != nil {
		return err
	}
	repoID, projectID := prRepoProjectID(pr)

	dedup := prDedupe(workItems)
	// remove_pull_request_work_items returns None (not []) when there is
	// nothing to remove or nothing ends up linked afterwards — a different
	// empty-state shape than `work-item list`'s [] (pull_request.py:503-540).
	if len(dedup) == 0 {
		return ado.Print(cmd, nil, prWorkItemColumns...)
	}

	full, err := prBatchGetWorkItems(ctx, client, dedup, true)
	if err != nil {
		return err
	}
	if len(full) == 0 {
		return ado.Print(cmd, nil, prWorkItemColumns...)
	}

	prURL := prWorkItemArtifactURL(projectID, repoID, idStr)

	for _, wi := range full {
		relations, _ := wi["relations"].([]any)
		if relations == nil {
			continue
		}
		wiID := fmt.Sprint(wi["id"])
		rev := wi["rev"]
		// index is only incremented on a non-matching relation — a Python
		// bug (pull_request.py:513-530) kept verbatim: correct for the
		// common case of at most one matching relation, but a second
		// matching relation would PATCH against a now-stale index.
		index := 0
		for _, relAny := range relations {
			rel, _ := relAny.(map[string]any)
			relURL, _ := rel["url"].(string)
			if relURL == prURL {
				patch := []map[string]any{
					{"op": "test", "path": "/rev", "value": rev},
					{"op": 1, "path": fmt.Sprintf("/relations/%d", index)},
				}
				if err := client.Do(ctx, ado.Request{
					Method:     http.MethodPatch,
					Path:       "wit/workItems/" + url.PathEscape(wiID),
					APIVersion: "5.0",
					JSONPatch:  true,
					Body:       patch,
				}, nil); err != nil {
					return fmt.Errorf("failed to unlink work item %s: %w", wiID, err)
				}
			} else {
				index++
			}
		}
	}

	refs, err := prGetWorkItemRefs(ctx, client, projectID, repoID, idStr)
	if err != nil {
		return err
	}
	ids := prRefIDs(refs)
	if len(ids) == 0 {
		return ado.Print(cmd, nil, prWorkItemColumns...)
	}

	items, err := prBatchGetWorkItems(ctx, client, ids, false)
	if err != nil {
		return err
	}

	return ado.Print(cmd, items, prWorkItemColumns...)
}

// prGetWorkItemRefs ports get_pull_request_work_item_refs,
// git_client_base.py:2464-2483.
func prGetWorkItemRefs(ctx context.Context, client *ado.Client, projectID, repoID, prID string) ([]map[string]any, error) {
	var refs []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      projectID,
		Path:       "git/repositories/" + url.PathEscape(repoID) + "/pullRequests/" + prID + "/workitemrefs",
		APIVersion: "5.0",
	}, &refs); err != nil {
		return nil, fmt.Errorf("failed to list work item refs: %w", err)
	}
	return refs, nil
}

// prBatchGetWorkItems ports get_work_items(ids=...) / get_work_items(ids=...,
// expand=1), work_item_tracking_client.py:1401-1433.
func prBatchGetWorkItems(ctx context.Context, client *ado.Client, ids []string, expandRelations bool) ([]map[string]any, error) {
	q := url.Values{"ids": {strings.Join(ids, ",")}}
	if expandRelations {
		q.Set("$expand", "1")
	}
	var items []map[string]any
	if err := client.List(ctx, ado.Request{
		Path:       "wit/workItems",
		APIVersion: "5.0",
		Query:      q,
	}, &items); err != nil {
		return nil, fmt.Errorf("failed to get work items: %w", err)
	}
	return items, nil
}

func prGetLinkedWorkItems(ctx context.Context, client *ado.Client, projectID, repoID, prID string) ([]map[string]any, error) {
	refs, err := prGetWorkItemRefs(ctx, client, projectID, repoID, prID)
	if err != nil {
		return nil, err
	}
	return prBatchGetWorkItems(ctx, client, prRefIDs(refs), false)
}

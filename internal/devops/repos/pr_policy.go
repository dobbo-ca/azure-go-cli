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

func newPRPolicyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage pull request policies",
	}
	cmd.AddCommand(newPRPolicyListCmd())
	cmd.AddCommand(newPRPolicyQueueCmd())
	return cmd
}

func newPRPolicyListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List policies for a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunPolicyList(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the pull request.")
	cmd.MarkFlagRequired("id")
	cmd.Flags().Int("top", 0, "Maximum number of policies to list.")
	cmd.Flags().Int("skip", 0, "Number of policies to skip.")

	ado.AddOrgFlags(cmd)

	return cmd
}

func prRunPolicyList(ctx context.Context, cmd *cobra.Command) error {
	client, _, err := prClientOrg(ctx, cmd)
	if err != nil {
		return err
	}
	return prPolicyListExec(ctx, cmd, client)
}

// prPolicyListExec does the actual work given an already-resolved client,
// split out from prRunPolicyList so tests can exercise it against an
// httptest server without going through ado.Resolve's org validation.
func prPolicyListExec(ctx context.Context, cmd *cobra.Command, client *ado.Client) error {
	id, _ := cmd.Flags().GetInt("id")
	idStr := strconv.Itoa(id)

	pr, err := prGetByID(ctx, client, idStr)
	if err != nil {
		return err
	}
	_, projectID := prRepoProjectID(pr)

	artifactID := fmt.Sprintf("vstfs:///CodeReview/CodeReviewId/%s/%s", projectID, idStr)

	q := url.Values{"artifactId": {artifactID}}
	if cmd.Flags().Changed("top") {
		top, _ := cmd.Flags().GetInt("top")
		q.Set("$top", strconv.Itoa(top))
	}
	if cmd.Flags().Changed("skip") {
		skip, _ := cmd.Flags().GetInt("skip")
		q.Set("$skip", strconv.Itoa(skip))
	}

	var evals []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      projectID,
		Path:       "policy/evaluations",
		APIVersion: "5.0-preview.1",
		Query:      q,
	}, &evals); err != nil {
		return fmt.Errorf("failed to list policies: %w", err)
	}

	prSortPolicyEvalsForTable(cmd, evals)
	return ado.Print(cmd, evals, prPolicyColumns...)
}

func newPRPolicyQueueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Queue an evaluation of a policy for a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunPolicyQueue(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the pull request.")
	cmd.MarkFlagRequired("id")
	cmd.Flags().StringP("evaluation-id", "e", "", "ID of the policy evaluation to queue.")
	cmd.MarkFlagRequired("evaluation-id")

	ado.AddOrgFlags(cmd)

	return cmd
}

func prRunPolicyQueue(ctx context.Context, cmd *cobra.Command) error {
	client, _, err := prClientOrg(ctx, cmd)
	if err != nil {
		return err
	}
	return prPolicyQueueExec(ctx, cmd, client)
}

// prPolicyQueueExec does the actual work given an already-resolved client,
// split out from prRunPolicyQueue so tests can exercise it against an
// httptest server without going through ado.Resolve's org validation.
func prPolicyQueueExec(ctx context.Context, cmd *cobra.Command, client *ado.Client) error {
	id, _ := cmd.Flags().GetInt("id")
	idStr := strconv.Itoa(id)
	evalID, _ := cmd.Flags().GetString("evaluation-id")

	pr, err := prGetByID(ctx, client, idStr)
	if err != nil {
		return err
	}
	_, projectID := prRepoProjectID(pr)

	var eval map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Scope:      projectID,
		Path:       "policy/evaluations/" + url.PathEscape(evalID),
		APIVersion: "5.0-preview.1",
	}, &eval); err != nil {
		return fmt.Errorf("failed to queue policy evaluation: %w", err)
	}

	return ado.Print(cmd, eval, prPolicyColumns...)
}

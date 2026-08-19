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

func newPRSetVoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-vote",
		Short: "Vote on a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunSetVote(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the pull request.")
	cmd.MarkFlagRequired("id")
	cmd.Flags().String("vote", "", "New vote value for the pull request. Allowed values: approve, approve-with-suggestions, reset, wait-for-author, reject.")
	cmd.MarkFlagRequired("vote")

	// vote_pull_request declares only organization/detect.
	ado.AddOrgFlags(cmd)

	return cmd
}

// prConvertVote ports _convert_vote_to_int, pull_request.py:586-597.
func prConvertVote(vote string) (int, error) {
	switch strings.ToLower(vote) {
	case "approve":
		return 10, nil
	case "approve-with-suggestions":
		return 5, nil
	case "reset":
		return 0, nil
	case "wait-for-author":
		return -5, nil
	case "reject":
		return -10, nil
	default:
		return 0, fmt.Errorf("%q is an invalid value for a pull request vote", vote)
	}
}

func prRunSetVote(ctx context.Context, cmd *cobra.Command) error {
	client, _, err := prClientOrg(ctx, cmd)
	if err != nil {
		return err
	}
	return prSetVoteExec(ctx, cmd, client)
}

// prSetVoteExec does the actual work given an already-resolved client, split
// out from prRunSetVote so tests can exercise it against an httptest server
// without going through ado.Resolve's org validation.
func prSetVoteExec(ctx context.Context, cmd *cobra.Command, client *ado.Client) error {
	id, _ := cmd.Flags().GetInt("id")
	idStr := strconv.Itoa(id)
	voteFlag, _ := cmd.Flags().GetString("vote")
	voteInt, err := prConvertVote(voteFlag)
	if err != nil {
		return err
	}

	pr, err := prGetByID(ctx, client, idStr)
	if err != nil {
		return err
	}
	repoID, projectID := prRepoProjectID(pr)

	// vote_pull_request always votes as the caller — reviewerId in the route
	// is the caller's own resolved identity id, never an arbitrary reviewer
	// (pull_request.py:568-580), even though the underlying PUT endpoint
	// supports voting on anyone's behalf.
	me, err := prCurrentIdentityID(ctx, client)
	if err != nil {
		return err
	}

	var reviewer map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPut,
		Scope:      projectID,
		Path:       "git/repositories/" + url.PathEscape(repoID) + "/pullRequests/" + idStr + "/reviewers/" + url.PathEscape(me),
		APIVersion: "5.0",
		Body:       map[string]any{"id": me, "vote": voteInt},
	}, &reviewer); err != nil {
		return fmt.Errorf("failed to set vote: %w", err)
	}

	return ado.Print(cmd, reviewer, prReviewerColumns...)
}

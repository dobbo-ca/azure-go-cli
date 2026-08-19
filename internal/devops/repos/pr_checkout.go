package repos

import (
	"context"
	"errors"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPRCheckoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkout",
		Short: "Checkout the pull request source branch locally, if no local changes are present",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRunCheckout(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the pull request.")
	cmd.MarkFlagRequired("id")
	cmd.Flags().String("remote-name", "origin", "Name of git remote against which PR is raised.")

	// checkout() declares no --organization/--project/--repository/--detect
	// at all (pull_request.py:465) — org comes exclusively from
	// get_vsts_info_from_current_remote_url() reading the current directory's
	// git remote.

	return cmd
}

func prRunCheckout(ctx context.Context, cmd *cobra.Command) error {
	// Deviation: since no org/detect flags are registered on this command,
	// ado.Resolve reads them back as "" and falls into its default-detect
	// path — reproducing Python's unconditional git-remote lookup. Where
	// this diverges: if no git remote is found, ado.Resolve falls back to
	// the configured default organization (`az devops configure --defaults
	// organization=...`) instead of Python's checkout-specific error "This
	// command should be used from a valid Azure DevOps git repository only"
	// (pull_request.py:474-475). Reproducing that exact error would require
	// calling the ado package's unexported git-remote detector directly,
	// which this file may not do (package ado is a separate, unmodifiable
	// dependency here).
	client, _, err := prClientOrg(ctx, cmd)
	if err != nil {
		return err
	}

	id, _ := cmd.Flags().GetInt("id")
	idStr := strconv.Itoa(id)
	remoteName, _ := cmd.Flags().GetString("remote-name")

	pr, err := prGetByID(ctx, client, idStr)
	if err != nil {
		return err
	}

	repo, _ := pr["repository"].(map[string]any)
	repoID, _ := repo["id"].(string)
	project, _ := repo["project"].(map[string]any)
	projectID, _ := project["id"].(string)
	sourceRef, _ := pr["sourceRefName"].(string)

	favorite := map[string]any{"name": sourceRef, "repositoryId": repoID, "type": 2}
	var fav map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Scope:      projectID,
		Path:       "git/favorites",
		APIVersion: "5.0-preview.1",
		Body:       favorite,
	}, &fav); err != nil {
		var ae *ado.APIError
		if !(errors.As(err, &ae) && strings.Contains(ae.Message, "is already a favorite for user")) {
			return err
		}
	}

	branch := strings.TrimPrefix(sourceRef, "refs/heads/")
	// fetch_remote_and_checkout, dev/common/git.py:39-42: all three run with
	// check=False — failures are silently swallowed, the command still
	// reports success even if the checkout actually failed.
	_ = exec.Command("git", "fetch", remoteName, sourceRef).Run()
	_ = exec.Command("git", "checkout", branch).Run()
	_ = exec.Command("git", "pull", remoteName, branch).Run()

	return nil
}

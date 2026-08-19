package repos

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// repoImportColumns is transform_repo_import_table_output's shape
// (_format.py:307-312) — a single flat row, not wrapped in a list.
var repoImportColumns = []ado.Column{
	{Header: "Name", Field: "repository.name"},
	{Header: "Project", Field: "repository.project.name"},
	{Header: "Import Status", Field: "status"},
}

// repoGitSourcePasswordEnvVar mirrors GIT_SOURCE_PASSWORD_OR_PAT
// (common/const.py:23): CLI_ENV_VARIABLE_PREFIX + "GIT_SOURCE_PASSWORD_OR_PAT".
const repoGitSourcePasswordEnvVar = "AZURE_DEVOPS_EXT_GIT_SOURCE_PASSWORD_OR_PAT"

// newRepoImportCreateCmd implements `az repos import create`
// (create_import_request, import_request.py:20).
func newRepoImportCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a git import request",
		RunE: func(cmd *cobra.Command, args []string) error {
			return repoRunImportCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("git-source-url", "", "Url of the source git repository.")
	cmd.Flags().String("git-url", "", "Alias for --git-source-url.")
	cmd.Flags().Bool("requires-authorization", false, "Flag to tell if source git repository is private.")
	cmd.Flags().String("user-name", "", "User name in case source git repository is private.")
	cmd.Flags().String("git-service-endpoint-id", "", "Service Endpoint for connection to external endpoint.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddRepoFlag(cmd)

	return cmd
}

func repoRunImportCreate(ctx context.Context, cmd *cobra.Command) error {
	// git_source_url has no default (import_request.py:20), so Python
	// requires it before resolving org/project/repo; --repository is
	// resolved from the git remote by ado.ResolveRepo below and is only
	// required if that resolution fails (import_request.py:37-42).
	gitSourceURL, _ := cmd.Flags().GetString("git-source-url")
	if gitSourceURL == "" {
		gitSourceURL, _ = cmd.Flags().GetString("git-url")
	}
	if gitSourceURL == "" {
		return fmt.Errorf("--git-source-url must be specified")
	}

	dctx, err := ado.ResolveRepo(cmd)
	if err != nil {
		return err
	}

	requiresAuth, _ := cmd.Flags().GetBool("requires-authorization")
	userName, _ := cmd.Flags().GetString("user-name")
	endpointID, _ := cmd.Flags().GetString("git-service-endpoint-id")

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return repoImportCreate(ctx, cmd, client, dctx, gitSourceURL, requiresAuth, userName, endpointID)
}

// repoImportCreate does the create-endpoint/create-request/poll HTTP work,
// split out from repoRunImportCreate so tests can drive it against an
// httptest server with a hand-built ado.Context.
func repoImportCreate(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context, gitSourceURL string, requiresAuth bool, userName, endpointID string) error {
	var err error

	// delete_se_after_import (import_request.py:44) is only true when this
	// command itself created the temp service endpoint; an explicitly
	// supplied --git-service-endpoint-id is never deleted by this command.
	deleteAfter := false
	if requiresAuth && endpointID == "" {
		deleteAfter = true

		password := os.Getenv(repoGitSourcePasswordEnvVar)
		if password == "" {
			// verify_is_a_tty_or_raise_error (import_request.py:57): a
			// non-interactive session without the env var is a hard error,
			// not a silent stdin read.
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return fmt.Errorf("Please specify target git password / PAT in %s environment variable in non-interactive mode.", repoGitSourcePasswordEnvVar)
			}
			password, err = repoPromptImportPassword("Git Password / PAT")
			if err != nil {
				return err
			}
		}

		var endpoint map[string]any
		if err := client.Do(ctx, ado.Request{
			Method:     "POST",
			Scope:      dctx.Project,
			Path:       "serviceendpoint/endpoints",
			APIVersion: "5.0-preview.2",
			Body: map[string]any{
				"authorization": map[string]any{
					"parameters": map[string]any{
						"password": password,
						"username": repoNilIfEmpty(userName),
					},
					"scheme": "UsernamePassword",
				},
				"name": repoRandomEndpointName(),
				"type": "git",
				"url":  gitSourceURL,
			},
		}, &endpoint); err != nil {
			return fmt.Errorf("failed to create service endpoint: %w", err)
		}
		endpointID, _ = endpoint["id"].(string)
	}

	var created map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     "POST",
		Scope:      dctx.Project,
		Path:       "git/repositories/" + url.PathEscape(dctx.Repo) + "/importRequests",
		APIVersion: "5.0-preview.1",
		Body: map[string]any{
			"parameters": map[string]any{
				"gitSource": map[string]any{
					"overwrite": false,
					"url":       gitSourceURL,
				},
				"serviceEndpointId":                      repoNilIfEmpty(endpointID),
				"deleteServiceEndpointAfterImportIsDone": deleteAfter,
				"tfvcSource":                             nil,
			},
		},
	}, &created); err != nil {
		return fmt.Errorf("failed to create import request: %w", err)
	}

	importRequestID, _ := created["importRequestId"].(float64)

	final, err := repoPollImportRequest(ctx, client, dctx.Project, dctx.Repo, importRequestID)
	if err != nil {
		return err
	}

	return ado.Print(cmd, final, repoImportColumns...)
}

// repoPollImportRequest polls the import request every 5s until it reaches a
// terminal state (completed/failed/abandoned), matching
// _wait_for_import_request (import_request.py:81-89) — no timeout, no
// attempt cap; a stuck import blocks indefinitely, same as Python.
func repoPollImportRequest(ctx context.Context, client *ado.Client, project, repo string, importRequestID float64) (map[string]any, error) {
	path := fmt.Sprintf("git/repositories/%s/importRequests/%d", url.PathEscape(repo), int64(importRequestID))

	for {
		var imp map[string]any
		if err := client.Do(ctx, ado.Request{
			Scope:      project,
			Path:       path,
			APIVersion: "5.0-preview.1",
		}, &imp); err != nil {
			return nil, fmt.Errorf("failed to poll import request: %w", err)
		}

		status, _ := imp["status"].(string)
		switch strings.ToLower(status) {
		case "completed", "failed", "abandoned":
			return imp, nil
		}

		time.Sleep(5 * time.Second)
	}
}

// repoRandomEndpointName builds a 10-char upper+digit name
// (”.join(random.choice(...) for _ in range(10)), import_request.py:65) —
// collision-avoidance only, not security-sensitive.
func repoRandomEndpointName() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 10)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// repoNilIfEmpty maps "" to a JSON null (Python's None) so an unset optional
// string round-trips the same as Python's default, instead of an empty
// string.
func repoNilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// repoPromptImportPassword mirrors knack's prompt_pass(label, confirm=True)
// (import_request.py:58): prompts twice and re-prompts until both entries
// match, with no minimum-length rule (unlike ado.PromptSecret, which is the
// PAT-only rule from `az devops login`, credentials.py:73-86).
func repoPromptImportPassword(label string) (string, error) {
	for {
		fmt.Printf("%s: ", label)
		first, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", label, err)
		}

		fmt.Printf("Confirm %s: ", label)
		second, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", label, err)
		}

		if string(first) != string(second) {
			fmt.Println("The two entered values do not match.")
			continue
		}
		return string(first), nil
	}
}

package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func wikiCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a wiki.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return wikiRunCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", "Name of the new wiki.")
	// --wiki-type/--type: two independent flag names for the same Python
	// param (options_list=('--wiki-type','--type'), arguments.py:184) —
	// mirrors context.go's --organization/--org dual-flag idiom rather than
	// inventing a new aliasing mechanism.
	cmd.Flags().String("wiki-type", "", "Type of wiki to create. Allowed values: projectwiki, codewiki. Default: projectwiki.")
	cmd.Flags().String("type", "", "Alias for --wiki-type.")
	cmd.Flags().String("mapped-path", "", "Mapped path of the new wiki e.g. '/' to publish from root of repository. [Required for codewiki type]")
	cmd.Flags().StringP("version", "v", "", "[Required for codewiki type] Repository branch name to publish the code wiki from.")

	ado.AddRepoFlag(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func wikiRunCreate(ctx context.Context, cmd *cobra.Command) error {
	name, _ := cmd.Flags().GetString("name")

	wikiType, _ := cmd.Flags().GetString("wiki-type")
	if wikiType == "" {
		wikiType, _ = cmd.Flags().GetString("type")
	}
	if wikiType == "" {
		wikiType = "projectwiki"
	}
	if wikiType != "projectwiki" && wikiType != "codewiki" {
		return fmt.Errorf("--wiki-type must be one of: projectwiki, codewiki")
	}

	// mapped-path/version are documented-but-unenforced-required for
	// codewiki in Python (wiki.py:61-62) — replicate the lack of
	// enforcement, don't add validation the original doesn't have.
	mappedPath, _ := cmd.Flags().GetString("mapped-path")
	version, _ := cmd.Flags().GetString("version")

	var dctx ado.Context
	var err error
	if wikiType == "codewiki" {
		if name == "" {
			return fmt.Errorf("--name is required for wiki type 'codewiki'")
		}
		dctx, err = ado.ResolveRepo(cmd)
	} else {
		dctx, err = ado.ResolveProject(cmd)
	}
	if err != nil {
		return err
	}

	return wikiCreate(ctx, cmd, dctx, wikiType, name, mappedPath, version)
}

// wikiCreate does the actual client call sequence, split out from
// wikiRunCreate so tests can supply a dctx pointing at an httptest server
// without going through org validation.
func wikiCreate(ctx context.Context, cmd *cobra.Command, dctx ado.Context, wikiType, name, mappedPath, version string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	// Call sequence matches wiki.py:38-58 exactly: repository lookup (codewiki
	// only) happens before the project-GUID lookup, which happens for both types.
	var repositoryID string
	if wikiType == "codewiki" {
		repositoryID, err = wikiRepositoryID(ctx, client, dctx.Project, dctx.Repo)
		if err != nil {
			return err
		}
	}

	projectID, err := wikiProjectID(ctx, client, dctx.Project)
	if err != nil {
		return err
	}

	body := map[string]any{
		"type":      wikiType,
		"projectId": projectID,
	}
	if name != "" {
		body["name"] = name
	}
	if repositoryID != "" {
		body["repositoryId"] = repositoryID
	}
	if mappedPath != "" {
		body["mappedPath"] = mappedPath
	}
	if version != "" {
		body["version"] = map[string]any{"version": version}
	}

	var wiki map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Scope:      dctx.Project,
		Path:       "wiki/wikis",
		APIVersion: "5.0",
		Body:       body,
	}, &wiki); err != nil {
		return fmt.Errorf("failed to create wiki: %w", err)
	}

	return ado.Print(cmd, wiki, wikiColumns...)
}

// wikiProjectID resolves project (a name or id) to its project GUID, mirroring
// get_project_id_from_name (dev/common/services.py:437-442): a value already
// shaped like a UUID is passed through unchanged, saving the lookup call.
func wikiProjectID(ctx context.Context, client *ado.Client, project string) (string, error) {
	if ado.IsUUID(project) {
		return project, nil
	}
	var p map[string]any
	if err := client.Do(ctx, ado.Request{
		Path:       "projects/" + url.PathEscape(project),
		APIVersion: "5.0",
	}, &p); err != nil {
		return "", fmt.Errorf("failed to get project: %w", err)
	}
	id, _ := p["id"].(string)
	return id, nil
}

// wikiRepositoryID resolves a repository name or id to its GUID via
// _get_repository_id_from_name (dev/team/wiki.py:228-231).
func wikiRepositoryID(ctx context.Context, client *ado.Client, project, repository string) (string, error) {
	var r map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      project,
		Path:       "git/repositories/" + url.PathEscape(repository),
		APIVersion: "5.0",
	}, &r); err != nil {
		return "", fmt.Errorf("failed to get repository: %w", err)
	}
	id, _ := r["id"].(string)
	return id, nil
}

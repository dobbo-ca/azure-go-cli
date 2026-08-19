package pipelines

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

var buildRepositoryTypeChoices = []string{
	"tfsversioncontrol", "tfsgit", "git", "github", "githubenterprise", "bitbucket", "svn",
}

// newBuildDefinitionListCmd implements `az pipelines build definition list`
// (build_definition_list, dev/pipelines/build_definition.py:18).
func newBuildDefinitionListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List build definitions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildRunDefinitionList(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", `Limit results to definitions with this name or starting with this name. Examples: "FabCI" or "Fab*"`)
	cmd.Flags().Int("top", 0, "Maximum number of definitions to list.")
	cmd.Flags().String("repository-type", "", "Limit results to definitions associated with this repository type: "+strings.Join(buildRepositoryTypeChoices, ", "))
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	// A plain --repository, not ado.AddRepoFlag: -r is registered by Python
	// only in the `devops`/`repos` argument contexts (team/arguments.py:56-57,
	// repos/arguments.py:18-19), not `pipelines` (team/arguments.py:180-181,
	// load_global_args only).
	cmd.Flags().String("repository", "", "Name or ID of the git repository.")

	return cmd
}

func buildRunDefinitionList(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return buildDefinitionList(ctx, cmd, client, dctx)
}

func buildDefinitionList(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	name, _ := cmd.Flags().GetString("name")
	top, _ := cmd.Flags().GetInt("top")
	repositoryType, _ := cmd.Flags().GetString("repository-type")
	// dctx.Repo already carries the --repository flag OR (when the flag is
	// empty) the git-detected repo from ado.ResolveProject
	// (resolve_instance_project_and_repo, build_definition.py:32-33) — read
	// it instead of the raw flag so `list` inside an Azure Repos checkout
	// with no --repository filters by the detected repo like Python does.
	repository := dctx.Repo

	if !buildChoiceOK(strings.ToLower(repositoryType), buildRepositoryTypeChoices) {
		return fmt.Errorf("--repository-type must be one of %s", strings.Join(buildRepositoryTypeChoices, ", "))
	}

	q := url.Values{}
	if name != "" {
		q.Set("name", name)
	}
	if repository != "" {
		// build_definition.py:36-40: repository_type is unconditionally
		// discarded and forced to 'TfsGit' whenever --repository is given —
		// --repository-type has no effect. Reproduced deliberately
		// (Python-bug policy: match observable quirks, not crashes); the
		// resulting always-TfsGit branch means the plain-repository-id path
		// (repository_type != 'tfsgit') is unreachable and is not ported.
		resolvedRepo, err := buildResolveRepositoryAsID(ctx, client, repository, dctx.Project)
		if err != nil {
			return err
		}
		if resolvedRepo == "" {
			return fmt.Errorf("Could not find a repository with name '%s', in project '%s'.", repository, dctx.Project)
		}
		q.Set("repositoryId", resolvedRepo)
		q.Set("repositoryType", "TfsGit")
	}
	if top > 0 {
		q.Set("$top", strconv.Itoa(top))
	}
	q.Set("queryOrder", "DefinitionNameAscending")

	var defs []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "build/Definitions",
		APIVersion: "5.0",
		Query:      q,
	}, &defs); err != nil {
		return fmt.Errorf("failed to list build definitions: %w", err)
	}

	return ado.Print(cmd, defs, buildDefinitionColumns(defs)...)
}

// buildResolveRepositoryAsID ports _resolve_repository_as_id
// (build_definition.py:117-126).
func buildResolveRepositoryAsID(ctx context.Context, client *ado.Client, repository, project string) (string, error) {
	if ado.IsUUID(repository) {
		return repository, nil
	}

	var repos []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      project,
		Path:       "git/repositories",
		APIVersion: "5.0",
		Query:      url.Values{"includeLinks": {"false"}, "includeAllUrls": {"false"}},
	}, &repos); err != nil {
		return "", fmt.Errorf("failed to list repositories: %w", err)
	}

	for _, r := range repos {
		if name, _ := r["name"].(string); strings.EqualFold(name, repository) {
			id, _ := r["id"].(string)
			return id, nil
		}
	}
	return "", nil
}

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

func coreNewListCmd() *cobra.Command {
	var name, queryOrder, repository, folderPath, repositoryType string
	var top int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pipelines",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return coreRunList(context.Background(), cmd, name, top, queryOrder, repository, folderPath, repositoryType)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	cmd.Flags().StringVar(&name, "name", "", `Limit results to pipelines with this name or starting with this name. Examples: "FabCI" or "Fab*"`)
	cmd.Flags().IntVar(&top, "top", 0, "Maximum number of pipelines to list.")
	cmd.Flags().StringVar(&queryOrder, "query-order", "", "Order of the results. Allowed values: NameAsc, NameDesc, ModifiedAsc, ModifiedDesc, None.")
	cmd.Flags().StringVar(&repository, "repository", "", "Limit results to pipelines associated with this repository.")
	cmd.Flags().StringVar(&folderPath, "folder-path", "", "If specified, filters to definitions under this folder.")
	cmd.Flags().StringVar(&repositoryType, "repository-type", "", "Limit results to pipelines associated with this repository type. Requires --repository. Allowed values: tfsversioncontrol, tfsgit, git, github, githubenterprise, bitbucket, svn.")

	return cmd
}

func coreRunList(ctx context.Context, cmd *cobra.Command, name string, top int, queryOrder, repository, folderPath, repositoryType string) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return coreList(ctx, cmd, dctx, name, top, queryOrder, repository, folderPath, repositoryType)
}

// coreList does the actual client call, split out from coreRunList so tests
// can supply a dctx pointing at an httptest server without going through org
// validation (same split as internal/devops/team.go's
// teamRunCreate/teamCreate).
func coreList(ctx context.Context, cmd *cobra.Command, dctx ado.Context, name string, top int, queryOrder, repository, folderPath, repositoryType string) error {
	queryOrder, err := coreValidateChoice(queryOrder, "query-order", coreQueryOrderChoices)
	if err != nil {
		return err
	}
	repositoryType, err = coreValidateChoice(repositoryType, "repository-type", buildRepositoryTypeChoices)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var repoID string
	if repository != "" {
		if repositoryType == "" {
			repositoryType = "TfsGit"
		}
		if strings.EqualFold(repositoryType, "tfsgit") {
			repoID, err = coreResolveRepositoryID(ctx, client, dctx.Project, repository)
			if err != nil {
				return err
			}
			if repoID == "" {
				return fmt.Errorf("could not find a repository with name '%s', in project '%s'", repository, dctx.Project)
			}
		} else {
			repoID = repository
		}
	}

	q := url.Values{}
	if name != "" {
		q.Set("name", name)
	}
	if repoID != "" {
		q.Set("repositoryId", repoID)
	}
	if repositoryType != "" {
		q.Set("repositoryType", repositoryType)
	}
	if top > 0 {
		q.Set("$top", strconv.Itoa(top))
	}
	if p := coreFixPathForAPI(folderPath); p != "" {
		q.Set("path", p)
	}
	q.Set("queryOrder", coreResolveQueryOrder(queryOrder))

	var defs []map[string]any
	if err := client.List(ctx, ado.Request{Scope: dctx.Project, Path: "build/Definitions", APIVersion: "5.0", Query: q}, &defs); err != nil {
		return fmt.Errorf("failed to list pipelines: %w", err)
	}

	return ado.Print(cmd, defs, coreDefinitionColumns(defs)...)
}

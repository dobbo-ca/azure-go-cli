package boards

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// areaiterationNewProjectListCmd is `boards iteration project list` /
// `boards area project list` (get_project_iterations, iteration.py:25 /
// get_project_areas, area.py:19) — identical shape save for structureGroup.
func areaiterationNewProjectListCmd(structureGroup string) *cobra.Command {
	iteration := structureGroup == areaiterationStructureGroupIteration

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List " + areaiterationNoun(structureGroup) + "s for a project.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunProjectList(context.Background(), cmd, structureGroup)
		},
	}

	if iteration {
		cmd.Flags().Int("depth", 1, "Depth of child nodes to be fetched. Example: --depth 3.")
		cmd.Flags().String("path", "", `Absolute path of an iteration. Example:\ProjectName\Iteration\IterationName`)
	} else {
		cmd.Flags().Int("depth", 1, "Depth of child nodes to be fetched. Example: --depth 3")
		cmd.Flags().String("path", "", `Absolute path of an area. Example:\ProjectName\Area\AreaName`)
	}
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunProjectList(ctx context.Context, cmd *cobra.Command, structureGroup string) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return areaiterationProjectList(ctx, cmd, client, dctx.Project, structureGroup)
}

// areaiterationProjectList does the actual client calls, split out so tests
// can supply a client pointing at an httptest server without going through
// org validation.
func areaiterationProjectList(ctx context.Context, cmd *cobra.Command, client *ado.Client, project, structureGroup string) error {
	depth, _ := cmd.Flags().GetInt("depth")
	path, _ := cmd.Flags().GetString("path")

	if path != "" {
		resolved, err := areaiterationResolveClassificationNodePath(ctx, client, project, structureGroup, path)
		if err != nil {
			return err
		}
		path = resolved
	}

	var node map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      project,
		Path:       areaiterationNodePath(structureGroup, path),
		APIVersion: "5.0",
		Query:      url.Values{"$depth": {fmt.Sprintf("%d", depth)}},
	}, &node); err != nil {
		return fmt.Errorf("failed to list %ss: %w", areaiterationNoun(structureGroup), err)
	}

	return areaiterationPrintClassificationTree(cmd, node)
}

// areaiterationNoun returns the singular display noun for a structure
// group ("iteration"/"area").
func areaiterationNoun(structureGroup string) string {
	if structureGroup == areaiterationStructureGroupArea {
		return "area"
	}
	return "iteration"
}

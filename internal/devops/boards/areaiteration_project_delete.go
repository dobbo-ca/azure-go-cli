package boards

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// areaiterationNewProjectDeleteCmd is `boards iteration project delete`
// (delete_project_iteration, iteration.py:88) / `boards area project
// delete` (delete_project_area, area.py:36). commands.py:87-88/98-99
// register these with the CLI's implicit confirmation flag (no custom
// help text) — ado.AddYesFlag matches that shape.
func areaiterationNewProjectDeleteCmd(structureGroup string) *cobra.Command {
	noun := areaiterationNoun(structureGroup)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete " + noun + ".",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunProjectDelete(context.Background(), cmd, structureGroup)
		},
	}

	if structureGroup == areaiterationStructureGroupIteration {
		cmd.Flags().String("path", "", `Absolute path of an iteration. Example:\ProjectName\Iteration\IterationName`)
	} else {
		cmd.Flags().String("path", "", `Absolute path of an area. Example:\ProjectName\Area\AreaName`)
	}
	_ = cmd.MarkFlagRequired("path")
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddYesFlag(cmd)

	return cmd
}

func areaiterationRunProjectDelete(ctx context.Context, cmd *cobra.Command, structureGroup string) error {
	noun := areaiterationNoun(structureGroup)
	if err := ado.Confirm(cmd, "Are you sure you want to delete this "+noun+"?"); err != nil {
		return err
	}

	path, _ := cmd.Flags().GetString("path")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	resolvedPath, err := areaiterationResolveClassificationNodePath(ctx, client, dctx.Project, structureGroup, path)
	if err != nil {
		return err
	}

	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodDelete,
		Scope:      dctx.Project,
		Path:       areaiterationNodePath(structureGroup, resolvedPath),
		APIVersion: "5.0",
	}, nil); err != nil {
		return fmt.Errorf("failed to delete %s: %w", noun, err)
	}

	// No table_transformer registered (commands.py:89-90/104-105 register
	// none) — falls back to JSON with no columns, matching
	// internal/devops/project_delete.go's precedent for the same shape.
	return ado.Print(cmd, nil)
}

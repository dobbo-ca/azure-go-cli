package boards

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// areaiterationNewProjectShowCmd is `boards iteration project show`
// (get_project_iteration, iteration.py:102) / `boards area project show`
// (get_project_area, area.py:70).
//
// Deviation: area project show's Python --id has no `type=int` override
// (only iteration's does, arguments.py:54-55) — a non-numeric --id there
// crashes with an unhandled ValueError from `int(id)` (area.py:75). Per the
// crash-fix policy this port declares --id as an int flag for both
// commands, trading Python's raw-traceback for a clean cobra parse error.
func areaiterationNewProjectShowCmd(structureGroup string) *cobra.Command {
	noun := areaiterationNoun(structureGroup)
	idHelp := "Iteration ID."
	if structureGroup == areaiterationStructureGroupArea {
		idHelp = "Area ID."
	}

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show " + noun + " details for a project.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunProjectShow(context.Background(), cmd, structureGroup)
		},
	}

	cmd.Flags().Int("id", 0, idHelp)
	_ = cmd.MarkFlagRequired("id")
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunProjectShow(ctx context.Context, cmd *cobra.Command, structureGroup string) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return areaiterationProjectShow(ctx, cmd, client, dctx.Project, structureGroup)
}

func areaiterationProjectShow(ctx context.Context, cmd *cobra.Command, client *ado.Client, project, structureGroup string) error {
	id, _ := cmd.Flags().GetInt("id")

	var nodes []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      project,
		Path:       "wit/classificationnodes",
		APIVersion: "5.0",
		Query:      url.Values{"ids": {strconv.Itoa(id)}},
	}, &nodes); err != nil {
		return fmt.Errorf("failed to show %s: %w", areaiterationNoun(structureGroup), err)
	}

	return ado.Print(cmd, nodes, areaiterationVisibleColumns(nodes, areaiterationClassificationColumns)...)
}

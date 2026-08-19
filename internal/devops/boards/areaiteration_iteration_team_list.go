package boards

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// areaiterationNewIterationTeamListCmd is `boards iteration team list`
// (get_team_iterations, iteration.py:152).
func areaiterationNewIterationTeamListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List iterations for a team.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunIterationTeamList(context.Background(), cmd)
		},
	}

	areaiterationAddTeamFlag(cmd)
	cmd.Flags().String("timeframe", "", "A filter for which iterations are returned based on relative time. Only Current is supported currently.")
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunIterationTeamList(ctx context.Context, cmd *cobra.Command) error {
	team, _ := cmd.Flags().GetString("team")
	timeframe, _ := cmd.Flags().GetString("timeframe")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	req := ado.Request{
		Scope:      areaiterationTeamScope(dctx.Project, team),
		Path:       "work/teamsettings/iterations",
		APIVersion: "5.0",
	}
	if timeframe != "" {
		req.Query = url.Values{"$timeframe": {timeframe}}
	}

	var iterations []map[string]any
	if err := client.List(ctx, req, &iterations); err != nil {
		return fmt.Errorf("failed to list iterations: %w", err)
	}

	// _get_team_iteration_key (_format.py:239-240): sorted by name.lower(),
	// table view only — JSON/tsv preserve server order (matches
	// internal/devops/repos/ref_list.go's precedent for the same shape).
	rows := iterations
	if ado.TableMode(cmd) {
		rows = append([]map[string]any(nil), iterations...)
		sort.Slice(rows, func(i, j int) bool {
			ni, _ := rows[i]["name"].(string)
			nj, _ := rows[j]["name"].(string)
			return strings.ToLower(ni) < strings.ToLower(nj)
		})
	}

	return ado.Print(cmd, rows, areaiterationVisibleColumns(rows, areaiterationTeamIterationColumns)...)
}

// areaiterationNewIterationTeamListWorkItemsCmd is `boards iteration team
// list-work-items` (list_iteration_work_items, iteration.py:205).
func areaiterationNewIterationTeamListWorkItemsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-work-items",
		Short: "List work-items for an iteration.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunIterationTeamListWorkItems(context.Background(), cmd)
		},
	}

	cmd.Flags().String("id", "", "Identifier of the iteration.")
	_ = cmd.MarkFlagRequired("id")
	areaiterationAddTeamFlag(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunIterationTeamListWorkItems(ctx context.Context, cmd *cobra.Command) error {
	id, _ := cmd.Flags().GetString("id")
	team, _ := cmd.Flags().GetString("team")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	result, err := areaiterationIterationTeamListWorkItems(ctx, client, dctx.Project, team, id)
	if err != nil {
		return err
	}

	return areaiterationPrintIterationWorkItems(cmd, result)
}

// areaiterationIterationTeamListWorkItems does the two-call sequence, split
// out from areaiterationRunIterationTeamListWorkItems so tests can supply a
// client pointing at an httptest server without going through org
// validation.
func areaiterationIterationTeamListWorkItems(ctx context.Context, client *ado.Client, project, team, id string) (map[string]any, error) {
	var result map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      areaiterationTeamScope(project, team),
		Path:       "work/teamsettings/iterations/" + url.PathEscape(id) + "/workitems",
		APIVersion: "5.0-preview.1",
	}, &result); err != nil {
		return nil, fmt.Errorf("failed to list iteration work items: %w", err)
	}

	// iteration.py:216-219: a second call resolves friendly relation-type
	// names, always made even when the first call returned zero relations
	// (iteration.py:287-288 early-returns on the substitution, but the GET
	// itself still happens).
	var relationTypes []map[string]any
	if err := client.List(ctx, ado.Request{
		Path:       "wit/workitemrelationtypes",
		APIVersion: "5.0",
	}, &relationTypes); err != nil {
		return nil, fmt.Errorf("failed to resolve relation type names: %w", err)
	}

	if relations, ok := result["workItemRelations"].([]any); ok {
		names := map[string]string{}
		for _, rt := range relationTypes {
			ref, _ := rt["referenceName"].(string)
			name, _ := rt["name"].(string)
			names[ref] = name
		}
		for _, r := range relations {
			rel, ok := r.(map[string]any)
			if !ok {
				continue
			}
			if refName, _ := rel["rel"].(string); refName != "" {
				if friendly, ok := names[refName]; ok {
					rel["rel"] = friendly
				}
			}
		}
	}

	return result, nil
}

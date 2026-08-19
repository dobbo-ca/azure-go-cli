package boards

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// areaiterationNewIterationProjectCreateCmd is `boards iteration project
// create` (create_project_iteration, iteration.py:117).
func areaiterationNewIterationProjectCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create iteration.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunIterationProjectCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", "Name of the iteration.")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().String("path", "", `Absolute path of an iteration. Creates an iteration at root level if --path is not specified. Example:\ProjectName\Iteration\IterationName.`)
	cmd.Flags().String("start-date", "", `Start date of the iteration. Example : "2019-06-03"`)
	cmd.Flags().String("finish-date", "", `Finish date of the iteration. Example : "2019-06-21"`)
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunIterationProjectCreate(ctx context.Context, cmd *cobra.Command) error {
	name, _ := cmd.Flags().GetString("name")
	path, _ := cmd.Flags().GetString("path")
	startDate, _ := cmd.Flags().GetString("start-date")
	finishDate, _ := cmd.Flags().GetString("finish-date")

	body, err := areaiterationIterationCreateBody(name, startDate, finishDate)
	if err != nil {
		return err
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return areaiterationCreateNode(ctx, cmd, client, dctx.Project, areaiterationStructureGroupIteration, path, body)
}

// areaiterationIterationCreateBody builds the create_project_iteration POST
// body (iteration.py:129-142), split out from areaiterationRunIterationProjectCreate
// so it's testable without an org to resolve.
func areaiterationIterationCreateBody(name, startDate, finishDate string) (map[string]any, error) {
	// iteration.py:132-133: unconditional (the freshly-built node always
	// has attributes=None here, so Python's attributes-None guard is
	// always true — simplified to an unconditional check, per
	// iteration.py:134's noted dead-code smell).
	if (startDate != "") != (finishDate != "") {
		return nil, errors.New("You must specify both start and finish dates or neither date")
	}

	// iteration.py:135-137: attributes is assigned {} unconditionally
	// (nested inside the always-true guard noted above), so a fresh node's
	// POST body always carries "attributes": {} even with no dates given.
	body := map[string]any{"name": name, "attributes": map[string]any{}}
	if startDate != "" && finishDate != "" {
		start, err := areaiterationParseDate(startDate, "start_date")
		if err != nil {
			return nil, err
		}
		finish, err := areaiterationParseDate(finishDate, "finish_date")
		if err != nil {
			return nil, err
		}
		body["attributes"] = map[string]any{"startDate": start, "finishDate": finish}
	}
	return body, nil
}

// areaiterationNewAreaProjectCreateCmd is `boards area project create`
// (create_project_area, area.py:50). Areas have no start/finish date
// concept, unlike iterations.
func areaiterationNewAreaProjectCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create area.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunAreaProjectCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", "Name of the area.")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().String("path", "", `Absolute path of an area. Creates an area at root level if --path is not specified. Example:\ProjectName\Area\AreaName.`)
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunAreaProjectCreate(ctx context.Context, cmd *cobra.Command) error {
	name, _ := cmd.Flags().GetString("name")
	path, _ := cmd.Flags().GetString("path")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	body := map[string]any{"name": name}
	return areaiterationCreateNode(ctx, cmd, client, dctx.Project, areaiterationStructureGroupArea, path, body)
}

// areaiterationCreateNode POSTs a new classification node, resolving --path
// first when one was given (create_or_update_classification_node,
// work_item_tracking_client.py:199-221).
func areaiterationCreateNode(ctx context.Context, cmd *cobra.Command, client *ado.Client, project, structureGroup, path string, body map[string]any) error {
	if path != "" {
		resolved, err := areaiterationResolveClassificationNodePath(ctx, client, project, structureGroup, path)
		if err != nil {
			return err
		}
		path = resolved
	}

	var node map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Scope:      project,
		Path:       areaiterationNodePath(structureGroup, path),
		APIVersion: "5.0",
		Body:       body,
	}, &node); err != nil {
		return fmt.Errorf("failed to create %s: %w", areaiterationNoun(structureGroup), err)
	}

	return areaiterationPrintClassificationTree(cmd, node)
}

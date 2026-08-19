package boards

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// areaiterationNewIterationProjectUpdateCmd is `boards iteration project
// update` (update_project_iteration, iteration.py:42).
func areaiterationNewIterationProjectUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update project iteration.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunIterationProjectUpdate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("path", "", `Absolute path of an iteration. Example:\ProjectName\Iteration\IterationName`)
	_ = cmd.MarkFlagRequired("path")
	cmd.Flags().Int("child-id", 0, "Move an existing iteration and add as child node for this iteration.")
	cmd.Flags().String("name", "", "New name of the iteration.")
	cmd.Flags().String("start-date", "", `Start date of the iteration. Example : "2019-06-03"`)
	cmd.Flags().String("finish-date", "", `Finish date of the iteration. Example : "2019-06-21"`)
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunIterationProjectUpdate(ctx context.Context, cmd *cobra.Command) error {
	path, _ := cmd.Flags().GetString("path")
	childID, _ := cmd.Flags().GetInt("child-id")
	nameChanged := cmd.Flags().Changed("name")
	startDate, _ := cmd.Flags().GetString("start-date")
	finishDate, _ := cmd.Flags().GetString("finish-date")

	// iteration.py:50-51.
	if startDate == "" && finishDate == "" && !nameChanged && !cmd.Flags().Changed("child-id") {
		return errors.New("At least one of --start-date , --finish-date , --child-id or --name arguments is required.")
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	resolvedPath, err := areaiterationResolveClassificationNodePath(ctx, client, dctx.Project, areaiterationStructureGroupIteration, path)
	if err != nil {
		return err
	}
	nodePath := areaiterationNodePath(areaiterationStructureGroupIteration, resolvedPath)

	if cmd.Flags().Changed("child-id") {
		// iteration.py:57-63: move response is fetched and discarded —
		// step below re-fetches the node's current state regardless.
		var moved map[string]any
		if err := client.Do(ctx, ado.Request{
			Method:     http.MethodPost,
			Scope:      dctx.Project,
			Path:       nodePath,
			APIVersion: "5.0",
			Body:       map[string]any{"id": childID},
		}, &moved); err != nil {
			return fmt.Errorf("failed to move iteration: %w", err)
		}
	}

	var node map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       nodePath,
		APIVersion: "5.0",
	}, &node); err != nil {
		return fmt.Errorf("failed to update iteration: %w", err)
	}

	attrs, hasAttrs := node["attributes"].(map[string]any)
	if !hasAttrs && ((startDate != "") != (finishDate != "")) {
		return errors.New("You must specify both start and finish dates or neither date")
	}
	if !hasAttrs {
		attrs = map[string]any{}
	}
	if startDate != "" {
		start, err := areaiterationParseDate(startDate, "start_date")
		if err != nil {
			return err
		}
		attrs["startDate"] = start
	}
	if finishDate != "" {
		finish, err := areaiterationParseDate(finishDate, "finish_date")
		if err != nil {
			return err
		}
		attrs["finishDate"] = finish
	}
	node["attributes"] = attrs
	if nameChanged {
		nameVal, _ := cmd.Flags().GetString("name")
		node["name"] = nameVal
	}

	var updated map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Scope:      dctx.Project,
		Path:       nodePath,
		APIVersion: "5.0",
		Body:       node,
	}, &updated); err != nil {
		return fmt.Errorf("failed to update iteration: %w", err)
	}

	return areaiterationPrintClassificationTree(cmd, updated)
}

// areaiterationNewAreaProjectUpdateCmd is `boards area project update`
// (update_project_area, area.py:85). Unlike iteration update, the final
// PATCH only runs when --name was given (area.py:109-115) — an
// update with only --child-id returns the move response, not a re-fetched
// node.
func areaiterationNewAreaProjectUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update area.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunAreaProjectUpdate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("path", "", `Absolute path of an area. Example:\ProjectName\Area\AreaName`)
	_ = cmd.MarkFlagRequired("path")
	cmd.Flags().String("name", "", "New name of the area.")
	cmd.Flags().Int("child-id", 0, "Move an existing area and add as child node for this area.")
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunAreaProjectUpdate(ctx context.Context, cmd *cobra.Command) error {
	path, _ := cmd.Flags().GetString("path")
	childID, _ := cmd.Flags().GetInt("child-id")
	nameChanged := cmd.Flags().Changed("name")
	childIDChanged := cmd.Flags().Changed("child-id")

	// area.py:92-93.
	if !nameChanged && !childIDChanged {
		return errors.New("Either --name or --child-id should be provided.")
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	resolvedPath, err := areaiterationResolveClassificationNodePath(ctx, client, dctx.Project, areaiterationStructureGroupArea, path)
	if err != nil {
		return err
	}
	nodePath := areaiterationNodePath(areaiterationStructureGroupArea, resolvedPath)

	var response map[string]any
	if childIDChanged {
		if err := client.Do(ctx, ado.Request{
			Method:     http.MethodPost,
			Scope:      dctx.Project,
			Path:       nodePath,
			APIVersion: "5.0",
			Body:       map[string]any{"id": childID},
		}, &response); err != nil {
			return fmt.Errorf("failed to move area: %w", err)
		}
	}

	var node map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       nodePath,
		APIVersion: "5.0",
	}, &node); err != nil {
		return fmt.Errorf("failed to update area: %w", err)
	}

	if nameChanged {
		name, _ := cmd.Flags().GetString("name")
		node["name"] = name
		var updated map[string]any
		if err := client.Do(ctx, ado.Request{
			Method:     http.MethodPatch,
			Scope:      dctx.Project,
			Path:       nodePath,
			APIVersion: "5.0",
			Body:       node,
		}, &updated); err != nil {
			return fmt.Errorf("failed to update area: %w", err)
		}
		response = updated
	}

	return areaiterationPrintClassificationTree(cmd, response)
}

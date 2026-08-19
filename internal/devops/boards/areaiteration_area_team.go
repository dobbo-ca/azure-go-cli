package boards

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// areaiterationNewAreaTeamListCmd is `boards area team list`
// (get_team_areas, area.py:118).
func areaiterationNewAreaTeamListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List areas for a team.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunAreaTeamList(context.Background(), cmd)
		},
	}

	areaiterationAddTeamFlag(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunAreaTeamList(ctx context.Context, cmd *cobra.Command) error {
	team, _ := cmd.Flags().GetString("team")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	result, err := areaiterationGetTeamFieldValues(ctx, client, dctx.Project, team)
	if err != nil {
		return err
	}
	return areaiterationPrintTeamAreas(cmd, result)
}

// areaiterationGetTeamFieldValues is get_team_field_values
// (work_client.py:1221-1247), shared by every `boards area team` command.
func areaiterationGetTeamFieldValues(ctx context.Context, client *ado.Client, project, team string) (map[string]any, error) {
	var result map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      areaiterationTeamScope(project, team),
		Path:       "work/teamsettings/teamfieldvalues",
		APIVersion: "5.0",
	}, &result); err != nil {
		return nil, fmt.Errorf("failed to get team field values: %w", err)
	}
	return result, nil
}

// areaiterationPatchTeamFieldValues is update_team_field_values
// (work_client.py:1250-1280).
func areaiterationPatchTeamFieldValues(ctx context.Context, client *ado.Client, project, team string, body map[string]any) (map[string]any, error) {
	var result map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Scope:      areaiterationTeamScope(project, team),
		Path:       "work/teamsettings/teamfieldvalues",
		APIVersion: "5.0",
		Body:       body,
	}, &result); err != nil {
		return nil, areaiterationHandleBoardsError(err)
	}
	return result, nil
}

// areaiterationParseTriState parses a get_three_state_flag() string value
// ("true"/"false", case-insensitive), matching context.go's parseDetectFlag
// idiom. Callers check cmd.Flags().Changed first to distinguish "unset"
// from an explicit value.
func areaiterationParseTriState(v string) (bool, error) {
	switch {
	case strings.EqualFold(v, "true"):
		return true, nil
	case strings.EqualFold(v, "false"):
		return false, nil
	default:
		return false, fmt.Errorf("invalid value %q; must be true or false", v)
	}
}

// areaiterationNewAreaTeamAddCmd is `boards area team add` (add_team_area,
// area.py:130).
func areaiterationNewAreaTeamAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add area to a team.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunAreaTeamAdd(context.Background(), cmd)
		},
	}

	cmd.Flags().String("path", "", `Area path. Example:\ProjectName\AreaName`)
	_ = cmd.MarkFlagRequired("path")
	areaiterationAddTeamFlag(cmd)
	cmd.Flags().Bool("set-as-default", false, "Set this area path as default area for this team. Default: False")
	// area.py:146/get_three_state_flag (parameters.py:187-191) accepts the
	// space-separated "--include-sub-areas false" form via argparse's
	// nargs='?' lookahead; pflag has no equivalent (NoOptDefVal always wins
	// over a following bare token, flag.go:1017-1019), so only "=false" is
	// supported here -- Args: cobra.NoArgs below turns the unsupported form
	// into a hard error instead of silently writing the inverse value.
	cmd.Flags().String("include-sub-areas", "", "Include child nodes of this area. Default false; pass --include-sub-areas=false explicitly to disable (the bare space-separated form is not supported).")
	cmd.Flags().Lookup("include-sub-areas").NoOptDefVal = "true"
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunAreaTeamAdd(ctx context.Context, cmd *cobra.Command) error {
	path, _ := cmd.Flags().GetString("path")
	team, _ := cmd.Flags().GetString("team")
	setAsDefault, _ := cmd.Flags().GetBool("set-as-default")

	// area.py:144-145: None (not passed) is coerced to False.
	includeSubAreas := false
	if cmd.Flags().Changed("include-sub-areas") {
		raw, _ := cmd.Flags().GetString("include-sub-areas")
		v, err := areaiterationParseTriState(raw)
		if err != nil {
			return err
		}
		includeSubAreas = v
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	result, err := areaiterationAreaTeamAdd(ctx, client, dctx.Project, team, path, includeSubAreas, setAsDefault)
	if err != nil {
		return err
	}
	return areaiterationPrintTeamAreas(cmd, result)
}

// areaiterationAreaTeamAdd does the GET-then-PATCH sequence (add_team_area,
// area.py:130-156), split out so tests can supply a client pointing at an
// httptest server without going through org validation.
func areaiterationAreaTeamAdd(ctx context.Context, client *ado.Client, project, team, path string, includeSubAreas, setAsDefault bool) (map[string]any, error) {
	current, err := areaiterationGetTeamFieldValues(ctx, client, project, team)
	if err != nil {
		return nil, err
	}
	values, _ := current["values"].([]any)
	values = append(values, map[string]any{"value": path, "includeChildren": includeSubAreas})

	defaultValue := current["defaultValue"]
	if setAsDefault {
		defaultValue = path
	}

	return areaiterationPatchTeamFieldValues(ctx, client, project, team, map[string]any{
		"values":       values,
		"defaultValue": defaultValue,
	})
}

// areaiterationNewAreaTeamRemoveCmd is `boards area team remove`
// (remove_team_area, area.py:159). No --yes/-y — commands.py:109-110
// registers no confirmation= kwarg.
func areaiterationNewAreaTeamRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove area from a team.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunAreaTeamRemove(context.Background(), cmd)
		},
	}

	cmd.Flags().String("path", "", `Area path. Example:\ProjectName\AreaName`)
	_ = cmd.MarkFlagRequired("path")
	areaiterationAddTeamFlag(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunAreaTeamRemove(ctx context.Context, cmd *cobra.Command) error {
	path, _ := cmd.Flags().GetString("path")
	team, _ := cmd.Flags().GetString("team")

	// area.py:167-168: `path[0] == '\\'` is evaluated unconditionally and
	// crashes with an unhandled IndexError on an empty --path. Per the
	// crash-fix policy this is a validation error instead.
	if path == "" {
		return errors.New("--path must not be empty")
	}
	if path[0] == '\\' {
		path = path[1:]
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	result, err := areaiterationAreaTeamRemove(ctx, client, dctx.Project, team, path)
	if err != nil {
		return err
	}
	return areaiterationPrintTeamAreas(cmd, result)
}

// areaiterationAreaTeamRemove does the GET-then-PATCH sequence
// (remove_team_area, area.py:159-187), split out so tests can supply a
// client pointing at an httptest server without going through org
// validation. path must already have any leading backslash stripped
// (area.py:167-168).
func areaiterationAreaTeamRemove(ctx context.Context, client *ado.Client, project, team, path string) (map[string]any, error) {
	current, err := areaiterationGetTeamFieldValues(ctx, client, project, team)
	if err != nil {
		return nil, err
	}

	defaultValue, _ := current["defaultValue"].(string)
	if defaultValue == path {
		return nil, errors.New("You are trying to remove the default area for this team. " +
			"Please change the default area node and then try this command again.")
	}

	values, _ := current["values"].([]any)
	var kept []any
	found := false
	for _, v := range values {
		entry, ok := v.(map[string]any)
		if !ok {
			kept = append(kept, v)
			continue
		}
		if val, _ := entry["value"].(string); val == path {
			// area.py:173-179: removes every matching entry (Python's
			// list.remove(entry) inside a for loop over the same list
			// mutates while iterating, which can skip a run of
			// consecutive duplicate matches — not worth reproducing that
			// mutate-during-iterate quirk; this filters all matches).
			found = true
			continue
		}
		kept = append(kept, v)
	}
	if !found {
		return nil, errors.New("Path is not added to team area list.")
	}

	return areaiterationPatchTeamFieldValues(ctx, client, project, team, map[string]any{
		"values":       kept,
		"defaultValue": current["defaultValue"],
	})
}

// areaiterationNewAreaTeamUpdateCmd is `boards area team update`
// (update_team_area, area.py:190).
func areaiterationNewAreaTeamUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update team area.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return areaiterationRunAreaTeamUpdate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("path", "", `Area path. Example:\ProjectName\AreaName`)
	_ = cmd.MarkFlagRequired("path")
	areaiterationAddTeamFlag(cmd)
	cmd.Flags().String("include-sub-areas", "", "Include child nodes of this area. Default false; pass --include-sub-areas=false explicitly to disable (the bare space-separated form is not supported).")
	cmd.Flags().Lookup("include-sub-areas").NoOptDefVal = "true"
	cmd.Flags().Bool("set-as-default", false, "Set as default team area path. Default: False")
	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func areaiterationRunAreaTeamUpdate(ctx context.Context, cmd *cobra.Command) error {
	path, _ := cmd.Flags().GetString("path")
	team, _ := cmd.Flags().GetString("team")
	setAsDefault, _ := cmd.Flags().GetBool("set-as-default")
	includeChanged := cmd.Flags().Changed("include-sub-areas")

	// area.py:196-197: `include_sub_areas is None and set_as_default is
	// False` — passing --include-sub-areas false explicitly still counts
	// as "provided".
	if !includeChanged && !setAsDefault {
		return errors.New("Either --set-as-default or --include-sub-areas parameter should be provided.")
	}

	var includeSubAreas bool
	if includeChanged {
		raw, _ := cmd.Flags().GetString("include-sub-areas")
		v, err := areaiterationParseTriState(raw)
		if err != nil {
			return err
		}
		includeSubAreas = v
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	current, err := areaiterationGetTeamFieldValues(ctx, client, dctx.Project, team)
	if err != nil {
		return err
	}

	values, _ := current["values"].([]any)
	found := false
	defaultValue := current["defaultValue"]
	for _, v := range values {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if val, _ := entry["value"].(string); val == path {
			found = true
			if includeChanged {
				entry["includeChildren"] = includeSubAreas
			}
			if setAsDefault {
				defaultValue = path
			}
		}
	}
	if !found {
		return errors.New("Path is not added to team area list.")
	}

	result, err := areaiterationPatchTeamFieldValues(ctx, client, dctx.Project, team, map[string]any{
		"values":       values,
		"defaultValue": defaultValue,
	})
	if err != nil {
		return err
	}
	return areaiterationPrintTeamAreas(cmd, result)
}

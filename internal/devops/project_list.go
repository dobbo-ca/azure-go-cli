package devops

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// projectStateFilterValues is _PROJECT_GET_STATE_VALUE_FILTER
// (team/arguments.py:18) — the actual choices wired to --state-filter (not
// the unrelated, differently-cased _STATE_VALUES list used elsewhere).
var projectStateFilterValues = []string{"all", "createPending", "deleted", "deleting", "new", "unchanged", "wellFormed"}

func newProjectListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List team projects.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectList(context.Background(), cmd, args)
		},
	}

	cmd.Flags().Int("top", 0, "Maximum number of results to list.")
	cmd.Flags().Int("skip", 0, "Number of results to skip.")
	cmd.Flags().String("state-filter", "all", "State filter.")
	cmd.Flags().String("continuation-token", "", "Continuation token. This can be retrieved from previous run of this command if more results are present.")
	// arguments.py:77-78 registers this with get_three_state_flag() (nargs='?'),
	// not a plain bool — a Bool flag here made "--get-default-team-image-url
	// false" send getDefaultTeamImageUrl=true (a bare bool's NoOptDefVal also
	// swallows the space-separated "false").
	cmd.Flags().String("get-default-team-image-url", "", "Whether to get default team image url or not. true or false.")
	cmd.Flags().Lookup("get-default-team-image-url").NoOptDefVal = "true"
	ado.AddOrgFlags(cmd)

	return cmd
}

// projectListParams is the flag-parsed input to projectList, kept separate
// from *cobra.Command for the same testability reason as projectCreateParams
// (see project_create.go).
type projectListParams struct {
	StateFilter         string
	Top                 int
	HasTop              bool
	Skip                int
	HasSkip             bool
	ContinuationToken   string
	GetDefaultImageURL  bool
	HasGetDefaultImgURL bool
}

// projectListParseFlags builds a projectListParams from cmd's flags plus any
// leftover positional args pflag couldn't attach to
// --get-default-team-image-url (see the flag's NoOptDefVal comment above).
func projectListParseFlags(cmd *cobra.Command, args []string) (projectListParams, error) {
	var p projectListParams
	raw, _ := cmd.Flags().GetString("state-filter")
	normalized, ok := projectNormalizeStateFilter(raw)
	if !ok {
		return p, fmt.Errorf("invalid value %q for --state-filter; must be one of %s", raw, strings.Join(projectStateFilterValues, ", "))
	}
	p.StateFilter = normalized
	if p.HasTop = cmd.Flags().Changed("top"); p.HasTop {
		p.Top, _ = cmd.Flags().GetInt("top")
	}
	if p.HasSkip = cmd.Flags().Changed("skip"); p.HasSkip {
		p.Skip, _ = cmd.Flags().GetInt("skip")
	}
	p.ContinuationToken, _ = cmd.Flags().GetString("continuation-token")
	if p.HasGetDefaultImgURL = cmd.Flags().Changed("get-default-team-image-url"); p.HasGetDefaultImgURL {
		raw, _ := cmd.Flags().GetString("get-default-team-image-url")
		// Space-separated "--get-default-team-image-url false" leaves
		// "false" as a stray positional (same pflag limitation as
		// --enable-for-all); pick it up the way core_invoke.go's nargs
		// handling does.
		if len(args) > 0 {
			raw = args[0]
		}
		var err error
		p.GetDefaultImageURL, err = extensionParseTriState(raw)
		if err != nil {
			return p, fmt.Errorf("--get-default-team-image-url: %w", err)
		}
	}
	return p, nil
}

func runProjectList(ctx context.Context, cmd *cobra.Command, args []string) error {
	p, err := projectListParseFlags(cmd, args)
	if err != nil {
		return err
	}

	// Resolve resolves only the org (list is org-level, no --project flag).
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	projects, err := projectList(ctx, client, p)
	if err != nil {
		return err
	}

	v := projectListOutput(projects, ado.TableMode(cmd))
	return ado.Print(cmd, v, projectListColumns...)
}

// projectListOutput builds the value ado.Print prints for `project list`.
//
// tableMode (-o table, no --query): _format.py:53-57 sorts by name.lower()
// for table rendering only, so this returns the plain, sorted slice.
//
// Otherwise: core_client.py:245-247's GetProjects returns
// GetProjectsResponseValue(value, continuation_token), which knack
// serializes as {"value": [...], "continuationToken": ...} — a bare array
// made the documented --continuation-token round trip unreachable.
// DEVIATION: continuationToken is always null here — ado.Client doesn't
// expose the X-MS-ContinuationToken response header to this package (same
// documented gap as core_invoke.go's continuation_token).
func projectListOutput(projects []map[string]any, tableMode bool) any {
	if !tableMode {
		return map[string]any{"value": projects, "continuationToken": nil}
	}
	rows := append([]map[string]any(nil), projects...)
	sort.SliceStable(rows, func(i, j int) bool {
		ni, _ := rows[i]["name"].(string)
		nj, _ := rows[j]["name"].(string)
		return strings.ToLower(ni) < strings.ToLower(nj)
	})
	return rows
}

// projectList ports list_projects's single GET (project.py:123-148,
// get_projects). Deliberately does not auto-page: unlike every other list
// command in this port, Python neither loops on the continuation token nor
// unwraps the collection through the paging client — --continuation-token
// is a manual pass-through the caller re-invokes with.
func projectList(ctx context.Context, client *ado.Client, p projectListParams) ([]map[string]any, error) {
	q := url.Values{"stateFilter": {p.StateFilter}}
	if p.HasTop {
		q.Set("$top", strconv.Itoa(p.Top))
	}
	if p.HasSkip {
		q.Set("$skip", strconv.Itoa(p.Skip))
	}
	if p.ContinuationToken != "" {
		q.Set("continuationToken", p.ContinuationToken)
	}
	if p.HasGetDefaultImgURL {
		q.Set("getDefaultTeamImageUrl", strconv.FormatBool(p.GetDefaultImageURL))
	}

	var projects []map[string]any
	if err := client.Do(ctx, ado.Request{
		Path:       "projects",
		APIVersion: "5.1", // list_projects uses get_core_client_v51, unlike every other project op (5.0)
		Query:      q,
	}, &struct {
		Value *[]map[string]any `json:"value"`
	}{&projects}); err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	return projects, nil
}

// projectNormalizeStateFilter matches v against projectStateFilterValues
// case-insensitively (arguments.py:72 registers state_filter via
// get_enum_type(), whose CaseInsensitiveList choices are matched
// case-insensitively, unlike the exact-match enum_choice_list used
// elsewhere in this file) and returns the canonical-cased value.
func projectNormalizeStateFilter(v string) (string, bool) {
	for _, c := range projectStateFilterValues {
		if strings.EqualFold(v, c) {
			return c, true
		}
	}
	return "", false
}

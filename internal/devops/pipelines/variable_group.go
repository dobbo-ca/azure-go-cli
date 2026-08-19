package pipelines

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// variableGroupBaseColumns are the columns shared by create/show/update
// (which fetch or set `authorized` themselves) and list (which does not) —
// _transform_pipeline_variable_group_row (_format.py:320-343).
var variableGroupBaseColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Field: "name"},
	{Header: "Type", Field: "type"},
	{Header: "Description", Field: "description"},
}

var variableGroupNumberOfVariablesColumn = ado.Column{Header: "Number of Variables", Value: func(row map[string]any) string {
	vars, _ := row["variables"].(map[string]any)
	return strconv.Itoa(len(vars))
}}

// variableGroupColumns is create/show/update's column set
// (transform_pipelines_variable_group_table_output, _format.py:320-330):
// unlike list, these look up `authorized` themselves
// (variableGroupAuthorizedResult), so the column is always meaningful.
var variableGroupColumns = append(append([]ado.Column{}, variableGroupBaseColumns...),
	ado.Column{Header: "Is Authorized", Field: "authorized"},
	variableGroupNumberOfVariablesColumn,
)

// variableGroupListColumns is list's column set
// (transform_pipelines_variable_groups_table_output, _format.py:337-341):
// "Is Authorized" is added only when a row's `authorized` field is present
// and non-nil. get_variable_groups never returns that field
// (variable_group.py:81-103 does no per-item authorization lookup), so this
// port's `list` rows never have it and the column never appears —
// equivalent to, but cheaper than, checking presence per row.
var variableGroupListColumns = append(append([]ado.Column{}, variableGroupBaseColumns...),
	variableGroupNumberOfVariablesColumn,
)

// variableNewGroupCmd wires `az pipelines variable-group`
// (variable_group.py) plus its nested `variable-group variable` subgroup
// (variable_group_variable.go).
func variableNewGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variable-group",
		Short: "Manage variable groups.",
	}
	cmd.AddCommand(variableNewGroupCreateCmd())
	cmd.AddCommand(variableNewGroupShowCmd())
	cmd.AddCommand(variableNewGroupListCmd())
	cmd.AddCommand(variableNewGroupUpdateCmd())
	cmd.AddCommand(variableNewGroupDeleteCmd())
	cmd.AddCommand(variableNewGroupVariableCmd())
	return cmd
}

// variableGroupFetch is the GET .../distributedtask/variablegroups/{id}
// shared by every variable-group and variable-group-variable command
// (get_variable_group, task_agent_client.py:469-485).
func variableGroupFetch(ctx context.Context, client *ado.Client, project string, groupID int) (map[string]any, error) {
	var group map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      project,
		Path:       "distributedtask/variablegroups/" + strconv.Itoa(groupID),
		APIVersion: "5.0-preview.1",
	}, &group); err != nil {
		return nil, fmt.Errorf("failed to get variable group: %w", err)
	}
	return group, nil
}

// variableGroupPut is the PUT .../distributedtask/variablegroups/{id}
// shared by update and every variable-group-variable mutation
// (update_variable_group, task_agent_client.py:540-559). Recording-verified
// (test_variable_group.yaml) that Python round-trips the FULL fetched
// object back on the wire, unfiltered — it does not narrow the body to the
// VariableGroupParameters shape despite the SDK method's declared param
// type, so body is sent as-is.
func variableGroupPut(ctx context.Context, client *ado.Client, project string, groupID int, body map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPut,
		Scope:      project,
		Path:       "distributedtask/variablegroups/" + strconv.Itoa(groupID),
		APIVersion: "5.0-preview.1",
		Body:       body,
	}, &out); err != nil {
		return nil, fmt.Errorf("failed to update variable group: %w", err)
	}
	return out, nil
}

// variableGroupGetAuthorized is pipeline_utils.get_authorize_resource
// (pipeline_utils.py:24-35): GET the build client's authorizedresources for
// this group, returning the authorized value of the first match, or nil if
// none.
func variableGroupGetAuthorized(ctx context.Context, client *ado.Client, project string, groupID int) (any, error) {
	var wrap struct {
		Value []map[string]any `json:"value"`
	}
	q := url.Values{"type": {"variablegroup"}, "id": {strconv.Itoa(groupID)}}
	if err := client.Do(ctx, ado.Request{
		Scope:      project,
		Path:       "build/authorizedresources",
		APIVersion: "5.0-preview.1",
		Query:      q,
	}, &wrap); err != nil {
		return nil, fmt.Errorf("failed to get variable group authorization: %w", err)
	}
	if len(wrap.Value) > 0 {
		return wrap.Value[0]["authorized"], nil
	}
	return nil, nil
}

// variableGroupSetAuthorized is pipeline_utils.set_authorize_resource
// (pipeline_utils.py:10-21): PATCH the build client's authorizedresources
// for this group. Note "id" travels as a string on the wire
// (recording-verified: `"id": "47"`), not a number.
func variableGroupSetAuthorized(ctx context.Context, client *ado.Client, project string, groupID int, name string, authorized bool) error {
	body := []map[string]any{{
		"authorized": authorized,
		"id":         strconv.Itoa(groupID),
		"name":       name,
		"type":       "variablegroup",
	}}
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Scope:      project,
		Path:       "build/authorizedresources",
		APIVersion: "5.0-preview.1",
		Body:       body,
	}, nil); err != nil {
		return fmt.Errorf("failed to set variable group authorization: %w", err)
	}
	return nil
}

// variableGroupAuthorizedResult builds the VariableGroupAuthorized shape
// (variable_group.py:14-27): a copy of id/name/type/description/variables
// (+ providerData if present) off group, plus "authorized" — which Python
// always coerces a nil/None authorized value to false for, on every code
// path (create/show/update alike), rather than passing None through.
func variableGroupAuthorizedResult(group map[string]any, authorized any) map[string]any {
	if authorized == nil {
		authorized = false
	}
	result := map[string]any{
		"id":          group["id"],
		"name":        group["name"],
		"type":        group["type"],
		"description": group["description"],
		"variables":   group["variables"],
		"authorized":  authorized,
	}
	if pd, ok := group["providerData"]; ok {
		result["providerData"] = pd
	}
	return result
}

// --- create ---------------------------------------------------------------

func variableNewGroupCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a variable group.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return variableRunGroupCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", "Name of the variable group.")
	cmd.MarkFlagRequired("name")
	cmd.Flags().StringSlice("variables", nil, "Space separated \"key=value\" pairs for the non-secret variables in the group.")
	// variable_group.py:31: variable_group_create(name, variables, ...) has
	// no default for `variables`, so azure-cli's signature binding marks it
	// required exactly like `name`.
	cmd.MarkFlagRequired("variables")
	cmd.Flags().String("description", "", "Description of the variable group.")
	agentpoolAddThreeStateFlag(cmd, "authorize", "Whether the variable group should be accessible by all pipelines.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func variableRunGroupCreate(ctx context.Context, cmd *cobra.Command) error {
	name, _ := cmd.Flags().GetString("name")
	rawVariables, _ := cmd.Flags().GetStringSlice("variables")
	variables, err := variableParseKeyValues(rawVariables)
	if err != nil {
		return err
	}
	description, _ := cmd.Flags().GetString("description")
	authorizePtr, err := agentpoolThreeState(cmd, "authorize")
	if err != nil {
		return err
	}
	authorizeSet := authorizePtr != nil
	authorize := authorizeSet && *authorizePtr

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	body := map[string]any{"name": name, "type": "Vsts", "variables": variables}
	if cmd.Flags().Changed("description") {
		body["description"] = description
	}

	var group map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Scope:      dctx.Project,
		Path:       "distributedtask/variablegroups",
		APIVersion: "5.0-preview.1",
		Body:       body,
	}, &group); err != nil {
		return fmt.Errorf("failed to create variable group: %w", err)
	}

	if authorizeSet {
		groupName, _ := group["name"].(string)
		if err := variableGroupSetAuthorized(ctx, client, dctx.Project, variableIntField(group, "id"), groupName, authorize); err != nil {
			return err
		}
	}

	var authorizedVal any
	if authorizeSet {
		authorizedVal = authorize
	}
	return ado.Print(cmd, variableGroupAuthorizedResult(group, authorizedVal), variableGroupColumns...)
}

// --- show -------------------------------------------------------------

func variableNewGroupShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show variable group details.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return variableRunGroupShow(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("group-id", 0, "Id of the variable group.")
	cmd.Flags().Int("id", 0, "Alias for --group-id.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func variableRunGroupShow(ctx context.Context, cmd *cobra.Command) error {
	groupID, err := variableRequiredIntFlag(cmd, "group-id", "id")
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

	group, err := variableGroupFetch(ctx, client, dctx.Project, groupID)
	if err != nil {
		return err
	}
	authorized, err := variableGroupGetAuthorized(ctx, client, dctx.Project, groupID)
	if err != nil {
		return err
	}

	return ado.Print(cmd, variableGroupAuthorizedResult(group, authorized), variableGroupColumns...)
}

// --- list ---------------------------------------------------------------

func variableNewGroupListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List variable groups.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return variableRunGroupList(context.Background(), cmd)
		},
	}

	cmd.Flags().String("group-name", "", "Name of the variable group. Wildcards are accepted, e.g. var_group*")
	cmd.Flags().String("action-filter", "", "Action filter for the variable group. One of use, manage, none.")
	cmd.Flags().String("action", "", "Alias for --action-filter.")
	cmd.Flags().String("top", "", "Number of variable groups to get.")
	cmd.Flags().String("continuation-token", "", "Gets the variable groups after the continuation token provided.")
	cmd.Flags().String("query-order", "Desc", "Gets the results in the defined order. One of Asc, Desc.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func variableRunGroupList(ctx context.Context, cmd *cobra.Command) error {
	groupName, _ := cmd.Flags().GetString("group-name")
	actionFilter, _ := cmd.Flags().GetString("action-filter")
	if actionFilter == "" {
		actionFilter, _ = cmd.Flags().GetString("action")
	}
	if actionFilter != "" && !strings.EqualFold(actionFilter, "use") && !strings.EqualFold(actionFilter, "manage") && !strings.EqualFold(actionFilter, "none") {
		return fmt.Errorf("--action-filter must be one of use, manage, none")
	}
	top, _ := cmd.Flags().GetString("top")
	continuationToken, _ := cmd.Flags().GetString("continuation-token")
	queryOrder, _ := cmd.Flags().GetString("query-order")
	if !strings.EqualFold(queryOrder, "asc") && !strings.EqualFold(queryOrder, "desc") {
		return fmt.Errorf("--query-order must be one of Asc, Desc")
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	// variable_group.py:96-98: query_order defaults to 'Desc' client-side
	// (not left unset), so a bare `variable-group list` always explicitly
	// requests idDescending.
	q := url.Values{}
	if groupName != "" {
		q.Set("groupName", groupName)
	}
	if actionFilter != "" {
		q.Set("actionFilter", actionFilter)
	}
	if top != "" {
		q.Set("$top", top)
	}
	if continuationToken != "" {
		q.Set("continuationToken", continuationToken)
	}
	if strings.EqualFold(queryOrder, "desc") {
		q.Set("queryOrder", "idDescending")
	} else {
		q.Set("queryOrder", "idAscending")
	}

	// No auto-paging here (unlike most list commands elsewhere in this
	// port): Python's variable_group_list returns exactly one page and
	// exposes --continuation-token as a manual pass-through, same precedent
	// as internal/devops/project_list.go's runProjectList/projectList.
	var wrap struct {
		Value []map[string]any `json:"value"`
	}
	if err := client.Do(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "distributedtask/variablegroups",
		APIVersion: "5.0-preview.1",
		Query:      q,
	}, &wrap); err != nil {
		return fmt.Errorf("failed to list variable groups: %w", err)
	}

	return ado.Print(cmd, wrap.Value, variableGroupListColumns...)
}

// --- update ---------------------------------------------------------------

func variableNewGroupUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a variable group.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return variableRunGroupUpdate(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("group-id", 0, "Id of the variable group.")
	cmd.Flags().Int("id", 0, "Alias for --group-id.")
	cmd.Flags().String("name", "", "New name of the variable group.")
	cmd.Flags().String("description", "", "New description of the variable group.")
	agentpoolAddThreeStateFlag(cmd, "authorize", "Whether the variable group should be accessible by all pipelines.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func variableRunGroupUpdate(ctx context.Context, cmd *cobra.Command) error {
	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")
	authorizePtr, err := agentpoolThreeState(cmd, "authorize")
	if err != nil {
		return err
	}
	authorizeSet := authorizePtr != nil
	authorize := authorizeSet && *authorizePtr

	// variable_group.py:131-132: this check runs before org/project
	// resolution and before the --group-id requiredness check.
	if name == "" && description == "" && !authorizeSet {
		return fmt.Errorf("Either --name, --description or --authorize must be specified for update.")
	}

	groupID, err := variableRequiredIntFlag(cmd, "group-id", "id")
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

	group, err := variableGroupFetch(ctx, client, dctx.Project, groupID)
	if err != nil {
		return err
	}

	updated := false
	if name != "" {
		group["name"] = name
		updated = true
	}
	if description != "" {
		group["description"] = description
		updated = true
	}
	if updated {
		group, err = variableGroupPut(ctx, client, dctx.Project, groupID, group)
		if err != nil {
			return err
		}
	}

	var authorizedVal any
	if authorizeSet {
		groupName, _ := group["name"].(string)
		if err := variableGroupSetAuthorized(ctx, client, dctx.Project, groupID, groupName, authorize); err != nil {
			return err
		}
		authorizedVal = authorize
	} else {
		authorizedVal, err = variableGroupGetAuthorized(ctx, client, dctx.Project, groupID)
		if err != nil {
			return err
		}
	}

	return ado.Print(cmd, variableGroupAuthorizedResult(group, authorizedVal), variableGroupColumns...)
}

// --- delete ---------------------------------------------------------------

func variableNewGroupDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a variable group.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return variableRunGroupDelete(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("group-id", 0, "Id of the variable group.")
	cmd.Flags().Int("id", 0, "Alias for --group-id.")
	ado.AddYesFlag(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func variableRunGroupDelete(ctx context.Context, cmd *cobra.Command) error {
	if err := ado.Confirm(cmd, "Are you sure you want to delete this variable group?"); err != nil {
		return err
	}

	groupID, err := variableRequiredIntFlag(cmd, "group-id", "id")
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

	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodDelete,
		Scope:      dctx.Project,
		Path:       "distributedtask/variablegroups/" + strconv.Itoa(groupID),
		APIVersion: "5.0-preview.1",
	}, nil); err != nil {
		return fmt.Errorf("failed to delete variable group: %w", err)
	}

	// variable_group.py:115: printed unconditionally, independent of
	// -o/--output, same idiom as internal/devops/repos/repo_delete.go.
	fmt.Println("Deleted variable group successfully.")

	return ado.Print(cmd, nil)
}

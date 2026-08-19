package pipelines

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// variableColumns is transform_pipelines_variables_table_output /
// _transform_pipeline_variable_row (_format.py:344-361).
var variableColumns = []ado.Column{
	{Header: "Name", Field: "name"},
	{Header: "Allow Override", Field: "allowOverride"},
	{Header: "Is Secret", Field: "isSecret"},
	{Header: "Value", Value: func(row map[string]any) string { return variableTruncate(row["value"]) }},
}

// variableNewPipelineCmd wires `az pipelines variable`
// (pipeline_variables.py) — variables on a build pipeline definition, as
// opposed to a variable group.
func variableNewPipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variable",
		Short: "Manage variables of pipelines.",
	}
	cmd.AddCommand(variableNewPipelineCreateCmd())
	cmd.AddCommand(variableNewPipelineUpdateCmd())
	cmd.AddCommand(variableNewPipelineListCmd())
	cmd.AddCommand(variableNewPipelineDeleteCmd())
	return cmd
}

// variableResolvePipelineID is get_definition_id_from_name
// (build_definition.py:103-115) as used by pipeline_variables.py: a plain
// name-filtered list (no isExactNameMatch flag, unlike the release-side
// resolver), requiring exactly one match.
func variableResolvePipelineID(ctx context.Context, client *ado.Client, project, name string) (int, error) {
	var wrap struct {
		Value []map[string]any `json:"value"`
	}
	if err := client.Do(ctx, ado.Request{
		Scope:      project,
		Path:       "build/Definitions",
		APIVersion: "5.0",
		Query:      url.Values{"name": {name}},
	}, &wrap); err != nil {
		return 0, fmt.Errorf("failed to look up pipeline %q: %w", name, err)
	}
	switch len(wrap.Value) {
	case 1:
		return variableIntField(wrap.Value[0], "id"), nil
	case 0:
		return 0, fmt.Errorf("There were no build definitions matching name %q in project %q.", name, project)
	default:
		// build_definition.py:106-111: when --project is a GUID, the
		// "multiple matches" error substitutes the resolved project *name*
		// off the first match instead of echoing the raw GUID back.
		proj := project
		if ado.IsUUID(project) {
			if p, ok := wrap.Value[0]["project"].(map[string]any); ok {
				if n, ok := p["name"].(string); ok {
					proj = n
				}
			}
		}
		return 0, fmt.Errorf("Multiple definitions were found matching name %q in project %q. Try supplying the definition ID or folder path to differentiate.", name, proj)
	}
}

// variablePipelineFetch is GET .../build/Definitions/{id} (get_definition,
// build_client.py:648-678). Casing of "Definitions" is capitalised
// (recording-verified: test_pipeline_create_and_variables_test.yaml), unlike
// the lowercase "distributedtask/variablegroups" route.
func variablePipelineFetch(ctx context.Context, client *ado.Client, project string, id int) (map[string]any, error) {
	var definition map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      project,
		Path:       "build/Definitions/" + strconv.Itoa(id),
		APIVersion: "5.0",
	}, &definition); err != nil {
		return nil, fmt.Errorf("failed to get pipeline definition: %w", err)
	}
	return definition, nil
}

// variablePipelinePut is PUT .../build/Definitions/{id} (update_definition,
// build_client.py:770-797). Recording-verified: the full fetched definition
// object is round-tripped back on the wire, not a narrowed shape.
func variablePipelinePut(ctx context.Context, client *ado.Client, project string, id int, body map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPut,
		Scope:      project,
		Path:       "build/Definitions/" + strconv.Itoa(id),
		APIVersion: "5.0",
		Body:       body,
	}, &out); err != nil {
		return nil, fmt.Errorf("failed to update pipeline definition: %w", err)
	}
	return out, nil
}

// variablePipelineIDFlags reads --pipeline-id/--pipeline-name off cmd. It
// does not itself validate that one was given — callers must do that at the
// point in their control flow Python does (the ordering relative to
// org/project resolution and the "at least one field" checks differs by
// command, see each RunE below).
func variablePipelineIDFlags(cmd *cobra.Command) (id int, idSet bool, name string) {
	id, _ = cmd.Flags().GetInt("pipeline-id")
	idSet = cmd.Flags().Changed("pipeline-id")
	name, _ = cmd.Flags().GetString("pipeline-name")
	return id, idSet, name
}

const variableNoPipelineErr = "Either the --pipeline-id or --pipeline-name argument must be supplied for this command."

// --- create -----------------------------------------------------------

func variableNewPipelineCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Add a variable to a pipeline.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return variableRunPipelineCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", "Name of the variable.")
	cmd.MarkFlagRequired("name")
	cmd.Flags().Int("pipeline-id", 0, "Id of the pipeline.")
	cmd.Flags().String("pipeline-name", "", "Name of the pipeline. Ignored if --pipeline-id is supplied.")
	cmd.Flags().String("value", "", "Value of the variable. For secret variables, if omitted it is read from AZURE_DEVOPS_EXT_PIPELINE_VAR_<name> or prompted for.")
	agentpoolAddThreeStateFlag(cmd, "allow-override", "Whether the value can be set at queue time.")
	agentpoolAddThreeStateFlag(cmd, "secret", "Whether the variable's value is a secret.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// variablePipelineResolve does the org/project resolution, --pipeline-id /
// --pipeline-name validation and name->id lookup shared by all four
// `pipelines variable` subcommands.
//
// pipeline_variables.py:36-39: the pipeline-selector check runs AFTER
// org/project resolution (unlike the variable-group-variable analogue's "at
// least one field" checks, which run before).
//
// resolveByVariableName reproduces pipeline_variables.py:178 (confirmed bug,
// deliberately preserved): `delete` resolves the pipeline by --name -- the
// *variable* name -- instead of --pipeline-name, so
// `az pipelines variable delete --name foo --pipeline-name bar` looks up a
// pipeline named "foo". Wrong behaviour, not a crash, so per the port's bug
// policy it is replicated rather than fixed.
func variablePipelineResolve(ctx context.Context, cmd *cobra.Command, resolveByVariableName bool) (ado.Context, *ado.Client, int, error) {
	pipelineID, pipelineIDSet, pipelineName := variablePipelineIDFlags(cmd)

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return dctx, nil, 0, err
	}

	if !pipelineIDSet && pipelineName == "" {
		return dctx, nil, 0, fmt.Errorf(variableNoPipelineErr)
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return dctx, nil, 0, err
	}

	if !pipelineIDSet {
		lookup := pipelineName
		if resolveByVariableName {
			lookup, _ = cmd.Flags().GetString("name")
		}
		pipelineID, err = variableResolvePipelineID(ctx, client, dctx.Project, lookup)
		if err != nil {
			return dctx, nil, 0, err
		}
	}
	return dctx, client, pipelineID, nil
}

func variableRunPipelineCreate(ctx context.Context, cmd *cobra.Command) error {
	name, _ := cmd.Flags().GetString("name")
	dctx, client, pipelineID, err := variablePipelineResolve(ctx, cmd, false)
	if err != nil {
		return err
	}

	definition, err := variablePipelineFetch(ctx, client, dctx.Project, pipelineID)
	if err != nil {
		return err
	}
	variables, _ := definition["variables"].(map[string]any)
	if variables == nil {
		variables = map[string]any{}
	}

	if existingKey, _, found := variableCaseInsensitiveGet(variables, name); found {
		return fmt.Errorf("Variable '%s' already exists. Use `az pipelines variable update` command to update the key/value.", existingKey)
	}

	value, _ := cmd.Flags().GetString("value")
	secretPtr, err := agentpoolThreeState(cmd, "secret")
	if err != nil {
		return err
	}
	secretSet := secretPtr != nil
	secret := secretSet && *secretPtr
	allowOverridePtr, err := agentpoolThreeState(cmd, "allow-override")
	if err != nil {
		return err
	}
	allowOverrideSet := allowOverridePtr != nil
	allowOverride := allowOverrideSet && *allowOverridePtr

	if value == "" && secretSet && secret {
		value, err = variableValueOrPrompt(name)
		if err != nil {
			return err
		}
	}

	entry := map[string]any{"value": value}
	if allowOverrideSet {
		entry["allowOverride"] = allowOverride
	}
	if secretSet {
		entry["isSecret"] = secret
	}
	variables[name] = entry
	definition["variables"] = variables

	updated, err := variablePipelinePut(ctx, client, dctx.Project, pipelineID, definition)
	if err != nil {
		return err
	}

	updatedVars, _ := updated["variables"].(map[string]any)
	key, val, _ := variableCaseInsensitiveGet(updatedVars, name)
	result := map[string]any{key: val}

	return variablePrintMap(cmd, result, variableColumns)
}

// --- update -----------------------------------------------------------

func variableNewPipelineUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a variable in a pipeline.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return variableRunPipelineUpdate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", "Name of the variable to update.")
	cmd.MarkFlagRequired("name")
	cmd.Flags().Int("pipeline-id", 0, "Id of the pipeline.")
	cmd.Flags().String("pipeline-name", "", "Name of the pipeline. Ignored if --pipeline-id is supplied.")
	cmd.Flags().String("new-name", "", "New name of the variable.")
	cmd.Flags().String("value", "", "New value of the variable.")
	agentpoolAddThreeStateFlag(cmd, "allow-override", "Whether the value can be set at queue time.")
	agentpoolAddThreeStateFlag(cmd, "secret", "Whether the variable's value is a secret.")
	agentpoolAddThreeStateFlag(cmd, "prompt-value", "Prompt (or read AZURE_DEVOPS_EXT_PIPELINE_VAR_<name>) for the value of a secret variable.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func variableRunPipelineUpdate(ctx context.Context, cmd *cobra.Command) error {
	newName, _ := cmd.Flags().GetString("new-name")
	value, _ := cmd.Flags().GetString("value")
	valueSet := cmd.Flags().Changed("value")
	secretPtr, err := agentpoolThreeState(cmd, "secret")
	if err != nil {
		return err
	}
	secretSet := secretPtr != nil
	secret := secretSet && *secretPtr
	allowOverridePtr, err := agentpoolThreeState(cmd, "allow-override")
	if err != nil {
		return err
	}
	allowOverrideSet := allowOverridePtr != nil
	allowOverride := allowOverrideSet && *allowOverridePtr
	promptValuePtr, err := agentpoolThreeState(cmd, "prompt-value")
	if err != nil {
		return err
	}
	promptValue := promptValuePtr != nil && *promptValuePtr

	// pipeline_variables.py:100-108: the "at least one field" check runs
	// BEFORE org/project resolution, which itself runs before the
	// pipeline-id/name check — the opposite order from pipeline_variable_add
	// above. Preserved exactly (see the surface doc's note on this file).
	// `not value` (falsy), not `value is None` — unlike the write below,
	// --value "" counts the same as --value omitted for this guard.
	if newName == "" && value == "" && !secretSet && !allowOverrideSet && !promptValue {
		return fmt.Errorf("Atleast one of --new-name, --value, --is-secret, --prompt-value or --allow-override must be specified for update.")
	}

	name, _ := cmd.Flags().GetString("name")
	dctx, client, pipelineID, err := variablePipelineResolve(ctx, cmd, false)
	if err != nil {
		return err
	}

	definition, err := variablePipelineFetch(ctx, client, dctx.Project, pipelineID)
	if err != nil {
		return err
	}
	variables, _ := definition["variables"].(map[string]any)
	if variables == nil {
		variables = map[string]any{}
	}

	oldKey, oldVal, found := variableCaseInsensitiveGet(variables, name)
	if !found {
		return fmt.Errorf("Variable '%s' does not exist.", name)
	}

	newKey := oldKey
	if newName != "" {
		newKey = newName
	}

	// pipeline_variables.py:121-122,131-134: is_secret/allow_override
	// inherit the old value when the flag is unset, and can stay None
	// (server omits them); variableInheritedBool tracks that so the PUT
	// omits the key entirely instead of sending an incorrect explicit false.
	effectiveSecret := variableInheritedBool(secretSet, secret, oldVal, "isSecret")
	effectiveAllowOverride := variableInheritedBool(allowOverrideSet, allowOverride, oldVal, "allowOverride")

	// pipeline_variables.py:123: `not value` (falsy) again, same
	// distinction as the guard above.
	if value == "" && effectiveSecret != nil && *effectiveSecret && promptValue {
		value, err = variableValueOrPrompt(newKey)
		if err != nil {
			return err
		}
		valueSet = true
	}

	if oldKey != newKey {
		if existingKey, _, exists := variableCaseInsensitiveGet(variables, newKey); exists {
			return fmt.Errorf("Variable '%s' already exists.", existingKey)
		}
		delete(variables, oldKey)
	}

	entry := map[string]any{}
	if effectiveSecret != nil {
		entry["isSecret"] = *effectiveSecret
	}
	if effectiveAllowOverride != nil {
		entry["allowOverride"] = *effectiveAllowOverride
	}
	if valueSet {
		entry["value"] = value
	} else {
		entry["value"] = oldVal["value"]
	}
	variables[newKey] = entry
	definition["variables"] = variables

	updated, err := variablePipelinePut(ctx, client, dctx.Project, pipelineID, definition)
	if err != nil {
		return err
	}

	updatedVars, _ := updated["variables"].(map[string]any)
	key, val, _ := variableCaseInsensitiveGet(updatedVars, newKey)
	result := map[string]any{key: val}

	return variablePrintMap(cmd, result, variableColumns)
}

// --- list -------------------------------------------------------------

func variableNewPipelineListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the variables in a pipeline.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return variableRunPipelineList(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("pipeline-id", 0, "Id of the pipeline.")
	cmd.Flags().String("pipeline-name", "", "Name of the pipeline. Ignored if --pipeline-id is supplied.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func variableRunPipelineList(ctx context.Context, cmd *cobra.Command) error {
	dctx, client, pipelineID, err := variablePipelineResolve(ctx, cmd, false)
	if err != nil {
		return err
	}

	definition, err := variablePipelineFetch(ctx, client, dctx.Project, pipelineID)
	if err != nil {
		return err
	}
	variables, _ := definition["variables"].(map[string]any)

	return variablePrintMap(cmd, variables, variableColumns)
}

// --- delete -----------------------------------------------------------

func variableNewPipelineDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a variable from a pipeline.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return variableRunPipelineDelete(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", "Name of the variable to delete.")
	cmd.MarkFlagRequired("name")
	cmd.Flags().Int("pipeline-id", 0, "Id of the pipeline.")
	cmd.Flags().String("pipeline-name", "", "Name of the pipeline. Ignored if --pipeline-id is supplied.")
	ado.AddYesFlag(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func variableRunPipelineDelete(ctx context.Context, cmd *cobra.Command) error {
	if err := ado.Confirm(cmd, "Are you sure you want to delete this variable?"); err != nil {
		return err
	}

	name, _ := cmd.Flags().GetString("name")
	dctx, client, pipelineID, err := variablePipelineResolve(ctx, cmd, true)
	if err != nil {
		return err
	}

	definition, err := variablePipelineFetch(ctx, client, dctx.Project, pipelineID)
	if err != nil {
		return err
	}
	variables, _ := definition["variables"].(map[string]any)

	key, _, found := variableCaseInsensitiveGet(variables, name)
	if !found {
		return fmt.Errorf("Variable '%s' does not exist. ", name)
	}
	delete(variables, key)
	definition["variables"] = variables

	if _, err := variablePipelinePut(ctx, client, dctx.Project, pipelineID, definition); err != nil {
		return err
	}

	fmt.Printf("Deleted variable '%s' successfully.\n", key)

	return ado.Print(cmd, nil)
}

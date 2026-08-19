package pipelines

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// variableGroupVariableColumns is
// transform_pipelines_var_group_variables_table_output /
// _transform_pipeline_var_group_variable_row (_format.py:363-379).
var variableGroupVariableColumns = []ado.Column{
	{Header: "Name", Field: "name"},
	{Header: "Is Secret", Field: "isSecret"},
	{Header: "Value", Value: func(row map[string]any) string { return variableTruncate(row["value"]) }},
}

// variableNewGroupVariableCmd wires `az pipelines variable-group variable`
// (variable_group.py's variable_group_variable_* functions).
func variableNewGroupVariableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variable",
		Short: "Manage variables in a variable group.",
	}
	cmd.AddCommand(variableNewGroupVariableCreateCmd())
	cmd.AddCommand(variableNewGroupVariableListCmd())
	cmd.AddCommand(variableNewGroupVariableUpdateCmd())
	cmd.AddCommand(variableNewGroupVariableDeleteCmd())
	return cmd
}

// --- create -----------------------------------------------------------

func variableNewGroupVariableCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Add a variable to a variable group.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return variableRunGroupVariableCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("group-id", 0, "Id of the variable group.")
	cmd.Flags().Int("id", 0, "Alias for --group-id.")
	cmd.Flags().String("name", "", "Name of the variable.")
	cmd.MarkFlagRequired("name")
	cmd.Flags().String("value", "", "Value of the variable. For secret variables, if omitted it is read from AZURE_DEVOPS_EXT_PIPELINE_VAR_<name> or prompted for.")
	agentpoolAddThreeStateFlag(cmd, "secret", "If the value of the variable is a secret.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func variableRunGroupVariableCreate(ctx context.Context, cmd *cobra.Command) error {
	groupID, err := variableRequiredIntFlag(cmd, "group-id", "id")
	if err != nil {
		return err
	}
	name, _ := cmd.Flags().GetString("name")
	value, _ := cmd.Flags().GetString("value")
	secretPtr, err := agentpoolThreeState(cmd, "secret")
	if err != nil {
		return err
	}
	secretSet := secretPtr != nil
	secret := secretSet && *secretPtr

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
	variables, _ := group["variables"].(map[string]any)
	if variables == nil {
		variables = map[string]any{}
	}

	if existingKey, _, found := variableCaseInsensitiveGet(variables, name); found {
		return fmt.Errorf("Variable '%s' already exists. Use `az pipelines variable-group variable update` command to update the key/value.", existingKey)
	}

	if value == "" && secretSet && secret {
		value, err = variableValueOrPrompt(name)
		if err != nil {
			return err
		}
	}

	// variable_group.py:196-197: is_secret is only ever set on the wire when
	// --secret was actually passed (a None is_secret is omitted by msrest,
	// recording-verified); value is always sent, defaulting to "".
	entry := map[string]any{"value": value}
	if secretSet {
		entry["isSecret"] = secret
	}
	variables[name] = entry
	group["variables"] = variables

	updated, err := variableGroupPut(ctx, client, dctx.Project, groupID, group)
	if err != nil {
		return err
	}

	updatedVars, _ := updated["variables"].(map[string]any)
	key, val, _ := variableCaseInsensitiveGet(updatedVars, name)
	result := map[string]any{key: val}

	return variablePrintMap(cmd, result, variableGroupVariableColumns)
}

// --- list -------------------------------------------------------------

func variableNewGroupVariableListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the variables in a variable group.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return variableRunGroupVariableList(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("group-id", 0, "Id of the variable group.")
	cmd.Flags().Int("id", 0, "Alias for --group-id.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func variableRunGroupVariableList(ctx context.Context, cmd *cobra.Command) error {
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
	variables, _ := group["variables"].(map[string]any)

	return variablePrintMap(cmd, variables, variableGroupVariableColumns)
}

// --- update -----------------------------------------------------------

func variableNewGroupVariableUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a variable in a variable group.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return variableRunGroupVariableUpdate(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("group-id", 0, "Id of the variable group.")
	cmd.Flags().Int("id", 0, "Alias for --group-id.")
	cmd.Flags().String("name", "", "Name of the variable to update.")
	cmd.MarkFlagRequired("name")
	cmd.Flags().String("new-name", "", "New name of the variable.")
	cmd.Flags().String("value", "", "New value of the variable.")
	agentpoolAddThreeStateFlag(cmd, "secret", "If the value of the variable is a secret.")
	agentpoolAddThreeStateFlag(cmd, "prompt-value", "Prompt (or read AZURE_DEVOPS_EXT_PIPELINE_VAR_<name>) for the value of a secret variable.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func variableRunGroupVariableUpdate(ctx context.Context, cmd *cobra.Command) error {
	newName, _ := cmd.Flags().GetString("new-name")
	value, _ := cmd.Flags().GetString("value")
	valueSet := cmd.Flags().Changed("value")
	secretPtr, err := agentpoolThreeState(cmd, "secret")
	if err != nil {
		return err
	}
	secretSet := secretPtr != nil
	secret := secretSet && *secretPtr
	promptValuePtr, err := agentpoolThreeState(cmd, "prompt-value")
	if err != nil {
		return err
	}
	promptValue := promptValuePtr != nil && *promptValuePtr

	// variable_group.py:222-224: `not value` (falsy), not `value is None` —
	// unlike the write below, --value "" counts the same as --value
	// omitted for this guard. Runs before org/project resolution and
	// before the --group-id requiredness check. The message text keeps
	// Python's own "--is-secret" wording even though the actual flag is
	// --secret — a verbatim quirk, not a typo to silently fix.
	if newName == "" && value == "" && !secretSet && !promptValue {
		return fmt.Errorf("Atleast one of --new-name, --value or --is-secret, --prompt-value must be specified for update.")
	}

	groupID, err := variableRequiredIntFlag(cmd, "group-id", "id")
	if err != nil {
		return err
	}
	name, _ := cmd.Flags().GetString("name")

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
	variables, _ := group["variables"].(map[string]any)
	if variables == nil {
		variables = map[string]any{}
	}

	oldKey, oldVal, found := variableCaseInsensitiveGet(variables, name)
	if !found {
		return fmt.Errorf("Variable '%s' does not exist. ", name)
	}

	newKey := oldKey
	if newName != "" {
		newKey = newName
	}

	effectiveSecret := variableInheritedBool(secretSet, secret, oldVal, "isSecret")

	// variable_group.py:236: `not value` (falsy) again, same distinction as
	// the guard above.
	if value == "" && effectiveSecret != nil && *effectiveSecret && promptValue {
		value, err = variableValueOrPrompt(newKey)
		if err != nil {
			return err
		}
		valueSet = true
	}

	// variable_group.py:243-246: this lookup deliberately runs BEFORE
	// popping oldKey, so renaming a variable to a different case of its own
	// existing name (e.g. "Foo" -> "FOO") self-collides and errors — a
	// faithfully reproduced Python quirk, not a bug to route around.
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
	if valueSet {
		entry["value"] = value
	} else {
		entry["value"] = oldVal["value"]
	}
	variables[newKey] = entry
	group["variables"] = variables

	updated, err := variableGroupPut(ctx, client, dctx.Project, groupID, group)
	if err != nil {
		return err
	}

	updatedVars, _ := updated["variables"].(map[string]any)
	key, val, _ := variableCaseInsensitiveGet(updatedVars, newKey)
	result := map[string]any{key: val}

	return variablePrintMap(cmd, result, variableGroupVariableColumns)
}

// --- delete -----------------------------------------------------------

func variableNewGroupVariableDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a variable from a variable group.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return variableRunGroupVariableDelete(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("group-id", 0, "Id of the variable group.")
	cmd.Flags().Int("id", 0, "Alias for --group-id.")
	cmd.Flags().String("name", "", "Name of the variable.")
	cmd.MarkFlagRequired("name")
	ado.AddYesFlag(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func variableRunGroupVariableDelete(ctx context.Context, cmd *cobra.Command) error {
	if err := ado.Confirm(cmd, "Are you sure you want to delete this variable?"); err != nil {
		return err
	}

	groupID, err := variableRequiredIntFlag(cmd, "group-id", "id")
	if err != nil {
		return err
	}
	name, _ := cmd.Flags().GetString("name")

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
	variables, _ := group["variables"].(map[string]any)

	key, _, found := variableCaseInsensitiveGet(variables, name)
	if !found {
		return fmt.Errorf("Variable '%s' does not exist. ", name)
	}
	delete(variables, key)
	group["variables"] = variables

	if _, err := variableGroupPut(ctx, client, dctx.Project, groupID, group); err != nil {
		return err
	}

	fmt.Printf("Deleted variable '%s' successfully.\n", key)

	return ado.Print(cmd, nil)
}

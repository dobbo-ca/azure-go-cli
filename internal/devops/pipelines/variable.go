// Package pipelines implements `az pipelines`.
//
// This file and every other "variable*"-prefixed file in this package are
// owned by the variable-group/variable-group-variable/variable phase of the
// port. See variable_group.go, variable_group_variable.go and
// variable_pipeline.go for the command implementations; this file holds only
// the top-level constructor and the handful of helpers shared across all
// three.
package pipelines

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// newVariableCommands wires the two independent top-level `az pipelines`
// command groups owned by this phase: `variable-group` (with its nested
// `variable-group variable` subgroup, dev/pipelines/variable_group.py) and
// `variable` (pipeline/build-definition variables,
// dev/pipelines/pipeline_variables.py). They share no cobra parent of their
// own, so this returns both rather than a single wrapping command.
func newVariableCommands() []*cobra.Command {
	return []*cobra.Command{
		variableNewGroupCmd(),
		variableNewPipelineCmd(),
	}
}

const variableValueTruncationLength = 80

// variableTruncate mirrors _format.py's _VALUE_TRUNCATION_LENGTH handling
// (`dev/pipelines/_format.py:9,357-358,375-376`): nil/non-string -> "",
// otherwise cut to 80 runes (code points, matching Python str slicing — a
// byte-slice would cut a multi-byte rune in half and render U+FFFD) with a
// trailing "...". Python's row builder assigns table_row['Value']
// unconditionally (falling back to an empty string for None, never omitting
// the key), so an empty result here renders as ado.Column's " " (blank, keep
// the column) rather than "" (drop the column) — otherwise a variable list
// where every row's value happens to be empty would lose the Value column
// entirely.
func variableTruncate(v any) string {
	s, _ := v.(string)
	t := coreTruncate(s, variableValueTruncationLength, "...")
	if t == "" {
		return " "
	}
	return t
}

// variableParseKeyValues parses `--variables key=value ...` pairs for
// `variable-group create`, splitting on the first '=' only
// (variable_group.py:52: `variable.split('=', 1)`). Python lets a malformed
// entry (no '=') crash with an unpacking ValueError; that's a crash, not a
// controlled CLIError, so per the port's bug policy this is fixed to a plain
// error instead of reproduced.
func variableParseKeyValues(pairs []string) (map[string]any, error) {
	out := map[string]any{}
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return nil, fmt.Errorf("--variables entries must be in the form key=value, got %q", p)
		}
		out[k] = map[string]any{"isSecret": false, "value": v}
	}
	return out, nil
}

// variableCaseInsensitiveGet is _case_insensitive_get
// (pipeline_variables.py:217-222, shared by both variable-group and pipeline
// variable commands): case-insensitive key lookup that returns the ACTUAL
// stored key (original casing) and its value.
func variableCaseInsensitiveGet(m map[string]any, search string) (key string, value map[string]any, found bool) {
	search = strings.ToLower(search)
	for k, v := range m {
		if strings.ToLower(k) == search {
			mv, _ := v.(map[string]any)
			return k, mv, true
		}
	}
	return "", nil, false
}

// variableValueOrPrompt resolves --value for a secret variable that omitted
// it: env var AZURE_DEVOPS_EXT_PIPELINE_VAR_<name> first
// (common/const.py's AZ_DEVOPS_PIPELINES_VARIABLES_KEY_PREFIX), else a
// masked stdin prompt (pipeline_variables.py:197-208
// _get_value_from_env_or_stdin). Deliberately does NOT delegate to the
// foundation's ado.PromptSecret: that helper's non-TTY behaviour (read one
// stdin line) is correct for its own caller, `az devops login`'s
// `echo $PAT | az devops login`, but wrong here — Python's
// verify_is_a_tty_or_raise_error requires a real TTY and errors otherwise,
// so a non-interactive secret-variable prompt must fail loudly instead of
// silently consuming an unrelated stdin line as the secret value. Its
// interactive loop text ("Please provide a PAT token.") is also
// login-specific, not appropriate for a variable-value prompt.
func variableValueOrPrompt(name string) (string, error) {
	envVar := "AZURE_DEVOPS_EXT_PIPELINE_VAR_" + name
	if v := os.Getenv(envVar); v != "" {
		return v, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("For non-interactive consoles set environment variable %s, "+
			"or pipe the value of variable into the command.", envVar)
	}

	fmt.Printf("%s: ", name)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", name, err)
	}
	return string(b), nil
}

// variableInheritedBool resolves an update flag that inherits from the old
// stored value when unset (variable_group.py:237's `secret = old_value.is_
// secret if secret is None else secret`, and pipeline_variables.py's
// analogous is_secret/allow_override handling): flagSet true returns
// flagVal; otherwise the old entry's key if present; otherwise nil (both
// unset), matching Python's is_secret staying None and msrest dropping it
// from the wire rather than sending an incorrect explicit false.
func variableInheritedBool(flagSet, flagVal bool, oldVal map[string]any, key string) *bool {
	if flagSet {
		return &flagVal
	}
	if b, ok := oldVal[key].(bool); ok {
		return &b
	}
	return nil
}

// variableIntField reads an int-typed field out of a JSON-decoded
// map[string]any, where numbers always decode as float64.
func variableIntField(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// variableRequiredIntFlag reads an int flag together with its --id alias
// (the `--group-id --id` / `--pipeline-id`-style pairing arguments.py wires
// per subcommand group) and errors if neither was supplied. Mirrors the
// --organization/--org alias idiom (ado/context.go): two independent flag
// registrations, read and OR'd together at the call site, rather than one
// flag bound under two names.
func variableRequiredIntFlag(cmd *cobra.Command, primary, alias string) (int, error) {
	v, _ := cmd.Flags().GetInt(primary)
	if v == 0 {
		v, _ = cmd.Flags().GetInt(alias)
	}
	if v == 0 {
		return 0, fmt.Errorf("required flag(s) %q not set", primary)
	}
	return v, nil
}

// variablePrintMap renders a {name: {...}} map the way Python's
// transform_pipelines_var*_variables_table_output does: JSON/tsv/--query see
// the map verbatim (this is exactly what `pipeline_variable_list`/
// `variable_group_variable_list` etc. return), while -o table flattens it
// into one row per key (name injected as a field) so it can go through
// ado.Print's Column machinery.
//
// ponytail: Table row order is sorted by name, NOT the server's wire order
// (Python's transform_pipelines_variables_table_output,
// dev/pipelines/_format.py:344-348, just iterates the dict and keeps
// insertion == wire order — confirmed by reading that function). Wire order
// is unrecoverable here: it dies before this function ever runs, at the
// variablePipelineFetch/variableGroupFetch `client.Do(ctx, req, &definition)`
// call, whose out is map[string]any — encoding/json has already discarded
// key order by the time `m` reaches variablePrintMap. Preserving it needs a
// second decode target (`client.Do(ctx, req, &raw)` into a json.RawMessage,
// which works today with no ado.Client change since json.RawMessage
// satisfies json.Unmarshaler) plus a json.Decoder token-stream walk of the
// "variables" sub-object to capture key order, threaded through both fetch
// functions (2 call sites each already touch 6 callers combined) and an
// order-aware variant of the row-building loop below — north of 30 lines
// once the plumbing and a helper are counted, so per the task's own line
// ceiling this stays sorted. Upgrade path: add a variableFetchWithOrder
// helper that does the raw decode + token-stream walk described above, and
// use it only in the two `... list` RunE functions (the only callers where
// row order is user-visible; create/update build a single-key result map).
func variablePrintMap(cmd *cobra.Command, m map[string]any, cols []ado.Column) error {
	if !ado.TableMode(cmd) {
		return ado.Print(cmd, m)
	}
	return ado.Print(cmd, variableSortedRows(m), cols...)
}

// variableSortedRows builds variablePrintMap's table rows (one per key,
// name injected as a field), sorted alphabetically by name — see the
// ponytail note on variablePrintMap for why sorted rather than wire order.
// Split out so the ordering is unit-testable without going through
// ado.Print's table rendering.
func variableSortedRows(m map[string]any) []map[string]any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	rows := make([]map[string]any, 0, len(m))
	for _, k := range keys {
		row := map[string]any{"name": k}
		if v, ok := m[k].(map[string]any); ok {
			for kk, vv := range v {
				row[kk] = vv
			}
		}
		rows = append(rows, row)
	}
	return rows
}

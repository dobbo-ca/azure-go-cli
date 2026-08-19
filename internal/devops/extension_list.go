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

func newExtensionListCmd() *cobra.Command {
	var includeBuiltIn, includeDisabled string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Get a list of extensions installed in an organization.",
		RunE: func(cmd *cobra.Command, args []string) error {
			builtIn, disabled := extensionAppendNargsStar(includeBuiltIn, includeDisabled,
				cmd.Flags().Changed("include-built-in"), cmd.Flags().Changed("include-disabled"), args)
			return runExtensionList(context.Background(), cmd, builtIn, disabled)
		},
	}

	ado.AddOrgFlags(cmd)
	cmd.Flags().StringVar(&includeBuiltIn, "include-built-in", "", "Include built in extensions.")
	cmd.Flags().Lookup("include-built-in").NoOptDefVal = "true"
	cmd.Flags().StringVar(&includeDisabled, "include-disabled", "", "Include disabled extensions.")
	cmd.Flags().Lookup("include-disabled").NoOptDefVal = "true"

	return cmd
}

// extensionParseTriState turns a raw --include-built-in/--include-disabled
// flag value into a bool, matching get_three_state_flag() where an unset
// value defaults to true here (extension.py:97-100: the CLI resolves
// Python's None default to True before calling the SDK).
func extensionParseTriState(v string) (bool, error) {
	switch {
	case v == "":
		return true, nil
	case strings.EqualFold(v, "true"):
		return true, nil
	case strings.EqualFold(v, "false"):
		return false, nil
	default:
		return false, fmt.Errorf("invalid value %q; must be true or false", v)
	}
}

// extensionAppendNargsStar picks up the stray positional pflag leaves behind
// for a space-separated "--include-built-in false"/"--include-disabled
// false" (a string flag's value is never looked ahead for — see
// core_invoke.go's nargs handling). Unambiguous when only one of the two
// flags was given; when both were given there is no way to tell, from
// cobra's post-parse Args(), which leftover belongs to which flag, so this
// leaves both as pflag parsed them rather than risk assigning the wrong one.
func extensionAppendNargsStar(includeBuiltIn, includeDisabled string, builtInChanged, disabledChanged bool, leftover []string) (string, string) {
	if len(leftover) == 0 {
		return includeBuiltIn, includeDisabled
	}
	switch {
	case builtInChanged && !disabledChanged:
		return leftover[0], includeDisabled
	case disabledChanged && !builtInChanged:
		return includeBuiltIn, leftover[0]
	default:
		return includeBuiltIn, includeDisabled
	}
}

func runExtensionList(ctx context.Context, cmd *cobra.Command, includeBuiltInFlag, includeDisabledFlag string) error {
	includeBuiltIn, err := extensionParseTriState(includeBuiltInFlag)
	if err != nil {
		return fmt.Errorf("--include-built-in: %w", err)
	}
	includeDisabled, err := extensionParseTriState(includeDisabledFlag)
	if err != nil {
		return fmt.Errorf("--include-disabled: %w", err)
	}

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := extensionNewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var extensions []map[string]any
	if err := client.List(ctx, ado.Request{
		Host:       "extmgmt",
		Path:       "extensionmanagement/installedextensions",
		APIVersion: "5.0-preview.1",
		Query:      url.Values{"includeDisabledExtensions": {strconv.FormatBool(includeDisabled)}},
	}, &extensions); err != nil {
		return fmt.Errorf("failed to list extensions: %w", err)
	}

	if !includeBuiltIn {
		// extension.py:105-111: substring match on the serialized flags
		// enum, not a structured field check — keep it that naive.
		filtered := make([]map[string]any, 0, len(extensions))
		for _, e := range extensions {
			if !strings.Contains(fmt.Sprint(e["flags"]), "builtIn") {
				filtered = append(filtered, e)
			}
		}
		extensions = filtered
	}

	// The extensionName sort (dev/team/_format.py:355-356) is a table_transformer
	// effect only — azure-cli applies it exclusively to -o table rendering, not
	// to JSON. Only sort when we're actually about to render a table.
	rows := extensions
	if ado.TableMode(cmd) {
		rows = extensionSortByName(extensions)
	}

	return ado.Print(cmd, rows, extensionColumns...)
}

func extensionSortByName(extensions []map[string]any) []map[string]any {
	sorted := make([]map[string]any, len(extensions))
	copy(sorted, extensions)
	sort.SliceStable(sorted, func(i, j int) bool {
		return strings.ToLower(devopsStr(sorted[i]["extensionName"])) < strings.ToLower(devopsStr(sorted[j]["extensionName"]))
	})
	return sorted
}

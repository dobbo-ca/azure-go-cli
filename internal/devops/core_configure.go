package devops

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// coreGitAliases mirrors git_aliases (git_alias.py:39-40).
var coreGitAliases = map[string]string{
	"pr":   "repos pr",
	"repo": "repos",
}

func newCoreConfigureCmd() *cobra.Command {
	var defaults []string
	var listConfig bool

	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Configure the Azure DevOps CLI or view your configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			// arguments.py:52 registers --defaults with nargs='*': a single
			// "--defaults organization=X project=Y" call is meant to
			// capture both tokens, but pflag only ever consumes one token
			// per flag occurrence — "project=Y" is left as a stray
			// positional in args, folded back in here.
			return runCoreConfigure(cmd, append(append([]string(nil), defaults...), args...), listConfig)
		},
	}

	// StringArrayVarP (not StringSliceVarP) so a single value isn't
	// comma-split — configure.py's --defaults takes each token verbatim.
	cmd.Flags().StringArrayVarP(&defaults, "defaults", "d", nil,
		"Space separated 'name=value' pairs for common argument defaults, e.g. 'organization=ORG_URL project=NAME'.")
	cmd.Flags().String("use-git-aliases", "", "Configure Git aliases (to enable commands like \"git pr list\"). true or false.")
	cmd.Flags().Lookup("use-git-aliases").NoOptDefVal = "true"
	cmd.Flags().BoolVarP(&listConfig, "list", "l", false, "List all configuration values.")

	return cmd
}

func runCoreConfigure(cmd *cobra.Command, defaults []string, listConfig bool) error {
	useGitAliasesGiven := cmd.Flags().Changed("use-git-aliases")
	useGitAliasesRaw, _ := cmd.Flags().GetString("use-git-aliases")

	if len(defaults) == 0 && !useGitAliasesGiven && !listConfig {
		return errors.New("usage error: atleast one of the options must be specified.For list of supported options see help using -h flag.")
	}

	if len(defaults) > 0 {
		if err := coreApplyDefaults(defaults); err != nil {
			return err
		}
	}

	if useGitAliasesGiven {
		on, err := coreParseThreeState(useGitAliasesRaw)
		if err != nil {
			return err
		}
		if on {
			if err := coreSetupGitAliases(); err != nil {
				return err
			}
		} else {
			coreClearGitAliases()
		}
	}

	if listConfig {
		if err := coreListConfig(); err != nil {
			return err
		}
	}

	return nil
}

// coreApplyDefaults ports configure.py:42-51. Each pair is validated then
// written immediately, one at a time — not validated as a batch and written
// once — so an earlier valid pair in the same --defaults invocation is
// already persisted if a later pair fails validation. That is the shipped
// Python behaviour (configure.py's loop calls set_global_config_value inside
// the same iteration as the validation), reproduced here deliberately.
func coreApplyDefaults(defaults []string) error {
	org, project, err := ado.ConfigDefaults()
	if err != nil {
		return err
	}

	for _, d := range defaults {
		parts := strings.SplitN(d, "=", 2)
		if len(parts) != 2 {
			return errors.New("usage error: --defaults STRING=STRING STRING=STRING ...")
		}
		key, value := parts[0], parts[1]

		switch key {
		case "organization":
			// value can be '' or a valid URL (configure.py:83-88); unlike
			// context.go's validateOrg this does NOT require a
			// dev.azure.com/visualstudio.com host, only scheme+host.
			if value != "" {
				u, err := url.Parse(value)
				if err != nil || u.Scheme == "" || u.Host == "" {
					return errors.New("Organization should be a valid Azure DevOps repository url. See command help for details.")
				}
			}
			org = value
		case "project":
			// project values are never validated (configure.py has no
			// branch for it in _validate_configuration).
			project = value
		default:
			return errors.New("usage error: invalid default value setup. Supported values are ['organization', 'project'].")
		}

		if err := ado.SetConfigDefaults(org, project); err != nil {
			return err
		}
	}

	return nil
}

// coreParseThreeState mirrors get_three_state_flag() as used by --detect in
// ado/context.go: unset/bare means true, an explicit value must be "true" or
// "false" (case-insensitive).
func coreParseThreeState(v string) (bool, error) {
	switch {
	case v == "" || strings.EqualFold(v, "true"):
		return true, nil
	case strings.EqualFold(v, "false"):
		return false, nil
	default:
		return false, fmt.Errorf("invalid value %q for --use-git-aliases; must be true or false", v)
	}
}

// coreListConfig ports print_current_configuration (configure.py:62-80).
// The devops config file, in this port, only ever has one section written
// by SetConfigDefaults — [defaults] with organization/project — so this
// reads that pair rather than walking an arbitrary INI structure.
func coreListConfig() error {
	org, project, _ := ado.ConfigDefaults()
	if org != "" || project != "" {
		fmt.Println()
		fmt.Println("[defaults]")
		if org != "" {
			fmt.Printf("organization = %s\n", org)
		}
		if project != "" {
			fmt.Printf("project = %s\n", project)
		}
	}

	configured, err := coreGitAliasesConfigured()
	if err != nil {
		return err
	}
	aliasSetup := "No"
	if configured {
		aliasSetup = "Yes"
	}
	fmt.Printf("\nUse git alias = %s\n", aliasSetup)

	var envVars []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "AZURE_DEVOPS_EXT_") {
			envVars = append(envVars, strings.SplitN(e, "=", 2)[0])
		}
	}
	if len(envVars) > 0 {
		fmt.Println("\nEnvironment variables:")
		for _, v := range envVars {
			fmt.Println(v)
		}
	}
	return nil
}

// coreGitAliasValue mirrors _get_alias_value (git.py:192-194).
func coreGitAliasValue(command string) string {
	mime := ""
	if runtime.GOOS == "windows" {
		mime = ".cmd"
	}
	return `!f() { exec az` + mime + ` ` + command + ` "$@"; }; f`
}

// coreSetupGitAliases mirrors setup_git_aliases (git_alias.py:9-15), calling
// `git config --global` — the one external binary this port shells out to,
// same as ado/git.go's --detect support.
func coreSetupGitAliases() error {
	for alias, command := range coreGitAliases {
		cmd := exec.Command("git", "config", "--global", "alias."+alias, coreGitAliasValue(command))
		if err := cmd.Run(); err != nil {
			return errors.New("Setting the git alias failed. Ensure git is installed and in your path.")
		}
	}
	return nil
}

// coreClearGitAliases mirrors clear_git_aliases (git_alias.py:18-25): it
// only unsets an alias that is currently set to exactly our value.
func coreClearGitAliases() {
	for alias, command := range coreGitAliases {
		out, err := exec.Command("git", "config", "--global", "alias."+alias).Output()
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(out)) == coreGitAliasValue(command) {
			_ = exec.Command("git", "config", "--global", "--unset", "alias."+alias).Run()
		}
	}
}

// coreGitAliasesConfigured mirrors are_git_aliases_setup (git_alias.py:28-36),
// backed by is_git_alias_setup (git.py:179-186): a non-zero git exit
// (CalledProcessError — e.g. the alias isn't set) means "not configured",
// but git being missing entirely (OSError) is a CLIError, not a silent "No".
func coreGitAliasesConfigured() (bool, error) {
	for alias, command := range coreGitAliases {
		out, err := exec.Command("git", "config", "--global", "alias."+alias).Output()
		if err != nil {
			var execErr *exec.Error
			if errors.As(err, &execErr) {
				return false, errors.New("Checking the git config values failed. Ensure git is installed and in your path.")
			}
			return false, nil
		}
		if strings.TrimSpace(string(out)) != coreGitAliasValue(command) {
			return false, nil
		}
	}
	return true, nil
}

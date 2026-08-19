package ado

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

// Context is the resolved organization/project/repository for one command.
type Context struct {
	Org     string // no trailing slash, e.g. "https://dev.azure.com/myorg"
	Project string // name or id; "" when not required
	Repo    string // name or id; "" when not required
}

// AddOrgFlags registers --organization/--org and --detect. Every command that
// talks to Azure DevOps calls this. --organization has no -o short flag —
// -o is the root --output flag.
//
// --detect is a string, not a bool, because it is tri-state in Python
// (get_three_state_flag(), team/arguments.py:32): unset or a bare --detect
// both mean detect (default "true"), an explicit value must be "true" or
// "false" (case-insensitive) — anything else is an error, matching
// argparse's choices=['true','false'] validation. NoOptDefVal lets --detect
// be passed bare; an explicit value still needs "=", e.g. --detect=false —
// cobra's String flags, like pflag's Bool flags, can't disambiguate a
// space-separated "--detect false" from the literal argument "false".
func AddOrgFlags(cmd *cobra.Command) {
	cmd.Flags().String("organization", "", "Azure DevOps organization URL, e.g. https://dev.azure.com/MyOrg.")
	// ponytail: when both --organization and --org are given, --organization
	// always wins here; Python's single argument with options_list takes
	// whichever appeared last on the command line (team/arguments.py:29).
	// cobra's Flags().Visit doesn't preserve argv order, so replicating
	// last-wins is not a cheap fix — not worth it for an alias pair no one
	// passes twice in practice.
	cmd.Flags().String("org", "", "Alias for --organization.")
	cmd.Flags().String("detect", "", "Automatically detect the organization/project/repository from the current git repo. Default true; pass --detect=false to disable.")
	cmd.Flags().Lookup("detect").NoOptDefVal = "true"
}

// AddProjectFlag registers --project/-p.
func AddProjectFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("project", "p", "", "Name or ID of the Azure DevOps project.")
}

// AddRepoFlag registers --repository/-r.
func AddRepoFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("repository", "r", "", "Name or ID of the git repository.")
}

const errNoOrg = "--organization must be specified. The value should be the URI of your Azure DevOps " +
	"organization, for example: https://dev.azure.com/MyOrganization/. " +
	"You can set a default value by running: az devops configure --defaults " +
	"organization=https://dev.azure.com/MyOrganization/. For auto detection to work " +
	"(--detect true), you must be in a local Git directory that has a \"remote\" referencing an " +
	"Azure DevOps repository."

const errNoProject = "--project must be specified. The value should be the ID or name of a team project. " +
	"You can set a default value by running: az devops configure --defaults project=<ProjectName>."

const errNoRepo = "--repository must be specified"

const errOnPrem = "The Azure DevOps CLI extension works only with Azure DevOps Services (cloud). " +
	"It doesn't support Azure DevOps Server (on-premises)."

// Resolve resolves the organization only. Project/Repo may still be populated
// if git detection supplied them. Mirrors services.resolve_instance.
func Resolve(cmd *cobra.Command) (Context, error) {
	return resolve(cmd, false, false)
}

// ResolveProject additionally requires a project.
// Mirrors services.resolve_instance_and_project.
func ResolveProject(cmd *cobra.Command) (Context, error) {
	return resolve(cmd, true, false)
}

// ResolveRepo additionally requires a project and a repository.
// Mirrors services.resolve_instance_project_and_repo(repo_required=True).
func ResolveRepo(cmd *cobra.Command) (Context, error) {
	return resolve(cmd, true, true)
}

// resolve is a port of services.py:337-372. Faithful quirk, kept
// deliberately: the whole detect+config-defaults block only runs when
// --organization is empty, so passing --organization explicitly also
// disables the configured project default and --detect for that invocation
// — the user must then pass --project too. This looks like a bug but is the
// shipped Python behaviour.
func resolve(cmd *cobra.Command, projectRequired, repoRequired bool) (Context, error) {
	org, _ := cmd.Flags().GetString("organization")
	if org == "" {
		org, _ = cmd.Flags().GetString("org")
	}
	project, _ := cmd.Flags().GetString("project")
	repo, _ := cmd.Flags().GetString("repository")
	detectFlag, _ := cmd.Flags().GetString("detect")
	detect, err := parseDetectFlag(detectFlag)
	if err != nil {
		return Context{}, err
	}

	if org == "" {
		if detect {
			if info, ok := detectFromGitRemote(); ok {
				org = info.Org
				if project == "" {
					project = info.Project
					if repo == "" {
						repo = info.Repo
					}
				}
			} else {
				logger.Warning("Auto-detect was enabled but no Azure DevOps remote was found in the " +
					"current git repository. Ensure your git remote points to an Azure DevOps URL " +
					"(e.g., https://dev.azure.com/MyOrganization/...).")
			}
		}

		if org == "" || project == "" {
			cfgOrg, cfgProject, _ := ConfigDefaults()
			if org == "" {
				org = cfgOrg
			}
			if project == "" {
				project = cfgProject
			}
		}
	}

	// services.py:360-361 resolves/validates project before validating org
	// (services.py:366-370), so a missing --project reports errNoProject even
	// when org is also invalid or on-prem.
	if projectRequired && project == "" {
		return Context{}, errors.New(errNoProject)
	}

	org, err = validateOrg(org)
	if err != nil {
		return Context{}, err
	}

	if repoRequired && repo == "" {
		return Context{}, errors.New(errNoRepo)
	}

	return Context{Org: org, Project: project, Repo: repo}, nil
}

// parseDetectFlag turns the raw --detect string into a bool, matching
// get_three_state_flag() (team/arguments.py:32): unset/bare means detect
// (true is the default label); an explicit value must be "true" or "false"
// (case-insensitive, argparse's choices=['true','false']) — anything else is
// a hard error rather than silently enabling or disabling detection.
func parseDetectFlag(v string) (bool, error) {
	switch {
	case v == "":
		return true, nil
	case strings.EqualFold(v, "true"):
		return true, nil
	case strings.EqualFold(v, "false"):
		return false, nil
	default:
		return false, fmt.Errorf("invalid value %q for --detect; must be true or false", v)
	}
}

// validateOrg checks org per services.py:445-448 (check_organization_in_azure)
// and returns it with any trailing slash stripped.
func validateOrg(org string) (string, error) {
	if org == "" {
		return "", errors.New(errNoOrg)
	}

	u, err := url.Parse(org)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New(errNoOrg)
	}

	// services.py:446-447: startswith is checked against the raw value;
	// endswith strips ALL trailing slashes first, not just one.
	trimmed := strings.TrimRight(org, "/")
	startsWith := strings.HasPrefix(org, "https://dev.azure.com/")
	endsWith := strings.HasSuffix(trimmed, ".visualstudio.com")
	if startsWith || endsWith {
		// Context.Org promises no trailing slash (see the struct doc
		// comment); trimmed strips all of them, unlike the raw value
		// Python's resolve_instance* actually returns.
		return trimmed, nil
	}

	return "", errors.New(errOnPrem)
}

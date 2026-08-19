// Package repos implements the `az repos` command group. This file and the
// other repo_*.go files implement the core repository commands
// (create/delete/list/show/update) and the `az repos import` subgroup
// (create). Other repos subgroups (policy, pr, ref) are implemented in
// sibling files by other contributors.
package repos

import (
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

// repoColumns is the table shape shared by repos create/show/update/list
// (_format.py:302-325, _transform_repo_row via transform_repo_table_output /
// transform_repos_table_output).
var repoColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Field: "name"},
	{Header: "Default Branch", Value: repoDefaultBranchCell},
	{Header: "Project", Field: "project.name"},
}

// repoDefaultBranchCell strips the "refs/heads/" prefix from defaultBranch
// (get_branch_name_from_ref, common/git.py:155-163); a repo with no default
// branch yet (e.g. right after create) renders a single space, matching
// Python's fallback (_format.py:320-323).
func repoDefaultBranchCell(row map[string]any) string {
	branch, _ := row["defaultBranch"].(string)
	if branch == "" {
		return " "
	}
	return strings.TrimPrefix(branch, "refs/heads/")
}

// repoOpenInBrowser opens repo's web page: {org}/{project}/_git/{repo},
// using the server-returned project/repo names, not the input strings
// (_open_repository, repository.py:119-127). Errors are warned, never
// fatal, matching foundation-spec.md §7.
func repoOpenInBrowser(org string, repo map[string]any) {
	project, _ := repo["project"].(map[string]any)
	projectName, _ := project["name"].(string)
	name, _ := repo["name"].(string)
	webURL := strings.TrimRight(org, "/") + "/" + url.PathEscape(projectName) + "/_git/" + url.PathEscape(name)
	if err := ado.OpenBrowser(webURL); err != nil {
		logger.Warning("failed to open browser: %v", err)
	}
}

// newRepoCommands wires the `az repos` command group's core repository
// commands (create/delete/list/show/update) and the `az repos import`
// subgroup (create). Sibling contributors add the policy/pr/ref subgroups
// to the returned command.
func newRepoCommands() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repos",
		Short: "Manage Azure Repos",
		Long:  "Manage Azure Repos repositories, pull requests, policies and refs",
	}

	cmd.AddCommand(newRepoCreateCmd())
	cmd.AddCommand(newRepoDeleteCmd())
	cmd.AddCommand(newRepoListCmd())
	cmd.AddCommand(newRepoShowCmd())
	cmd.AddCommand(newRepoUpdateCmd())
	cmd.AddCommand(newRepoImportGroupCmd())

	return cmd
}

func newRepoImportGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Manage Git repositories import",
	}
	cmd.AddCommand(newRepoImportCreateCmd())
	return cmd
}

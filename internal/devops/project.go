package devops

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

// projectColumns is transform_project_table_output / _transform_project_row
// (team/_format.py:60-85): ID, Name, Visibility always; Process and Source
// Control only when the row's capabilities carry them (list rows never do,
// since list doesn't request capabilities).
var projectColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Field: "name"},
	{Header: "Visibility", Value: func(row map[string]any) string {
		// row['visibility'].capitalize() (_format.py:72): uppercase the
		// first rune, lowercase the rest.
		v, _ := row["visibility"].(string)
		if v == "" {
			return ""
		}
		return strings.ToUpper(v[:1]) + strings.ToLower(v[1:])
	}},
	{Header: "Process", Field: "capabilities.processTemplate.templateName"},
	{Header: "Source Control", Field: "capabilities.versioncontrol.sourceControlType"},
}

// projectListColumns is projectColumns without Process/Source Control:
// _format.py:74-83 only adds those two columns "if 'capabilities' in row",
// and list_projects (project.py:123-148) never requests capabilities, so
// `project list -o table` never has them (contrast project_show.go, which
// passes includeCapabilities=true).
var projectListColumns = projectColumns[:3]

// newProjectCommand wires `az devops project` (create, delete, show, list).
func newProjectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage Azure DevOps projects.",
	}
	cmd.AddCommand(newProjectCreateCmd())
	cmd.AddCommand(newProjectDeleteCmd())
	cmd.AddCommand(newProjectShowCmd())
	cmd.AddCommand(newProjectListCmd())
	return cmd
}

// projectWaitForOperation polls GET operations/{id} once, then every second,
// until the operation's status is succeeded/failed/cancelled
// (common/operations.py:11-22). No timeout — matches Python's unbounded
// polling loop exactly (project.py:78-81, 96-99).
func projectWaitForOperation(ctx context.Context, client *ado.Client, id string) (map[string]any, error) {
	for {
		var op map[string]any
		if err := client.Do(ctx, ado.Request{
			Path:       "operations/" + url.PathEscape(id),
			APIVersion: "5.0",
		}, &op); err != nil {
			return nil, err
		}

		status, _ := op["status"].(string)
		switch strings.ToLower(status) {
		case "succeeded", "failed", "cancelled":
			return op, nil
		}
		time.Sleep(time.Second)
	}
}

// projectOpen implements the --open browser launch shared by create/show
// (project.py:151-161, _open_project): cut project.url at "/_apis/" and
// append the URL-escaped project name.
func projectOpen(project map[string]any) error {
	rawURL, _ := project["url"].(string)
	const marker = "/_apis/"
	pos := strings.Index(rawURL, marker)
	if pos < 0 {
		return errors.New("Failed to open web browser, due to unrecognized url in response.")
	}
	name, _ := project["name"].(string)
	target := rawURL[:pos+1] + url.PathEscape(name)
	if err := ado.OpenBrowser(target); err != nil {
		logger.Warning("failed to open browser: %v", err)
	}
	return nil
}

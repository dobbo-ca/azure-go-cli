package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newProjectCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", "Name of the new project.")
	cmd.Flags().StringP("process", "p", "", "Process to use. Default if not specified.")
	cmd.Flags().StringP("source-control", "s", "git", "Source control type of the initial code repository created.")
	cmd.Flags().StringP("description", "d", "", "Description for the new project.")
	cmd.Flags().String("visibility", "private", "Project visibility.")
	cmd.Flags().Bool("open", false, "Open the team project in the default web browser.")
	ado.AddOrgFlags(cmd)
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

// projectCreateParams is the flag-parsed input to projectCreate, kept
// separate from *cobra.Command so the HTTP call sequence is unit-testable
// against an httptest server: ado.Resolve validates --organization as a
// real Azure DevOps Services host, but ado.NewClient does not (see
// client.go's NewClient doc comment), so tests build the Client directly
// and call projectCreate without going through Resolve.
type projectCreateParams struct {
	Name          string
	Process       string
	SourceControl string
	Description   string
	Visibility    string
}

func runProjectCreate(ctx context.Context, cmd *cobra.Command) error {
	var p projectCreateParams
	p.Name, _ = cmd.Flags().GetString("name")
	p.Process, _ = cmd.Flags().GetString("process")
	p.SourceControl, _ = cmd.Flags().GetString("source-control")
	if p.SourceControl != "git" && p.SourceControl != "tfvc" {
		return fmt.Errorf("invalid value %q for --source-control; must be git or tfvc", p.SourceControl)
	}
	p.Description, _ = cmd.Flags().GetString("description")
	p.Visibility, _ = cmd.Flags().GetString("visibility")
	if p.Visibility != "private" && p.Visibility != "public" {
		return fmt.Errorf("invalid value %q for --visibility; must be private or public", p.Visibility)
	}
	open, _ := cmd.Flags().GetBool("open")

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	project, err := projectCreate(ctx, client, p)
	if err != nil {
		return err
	}

	if open {
		if err := projectOpen(project); err != nil {
			return err
		}
	}

	return ado.Print(cmd, project, projectColumns...)
}

// projectCreate ports create_project's HTTP sequence (project.py:21-85):
// list processes, resolve the process id, queue creation, poll to
// completion, then re-fetch the project by name with capabilities.
func projectCreate(ctx context.Context, client *ado.Client, p projectCreateParams) (map[string]any, error) {
	var processes []map[string]any
	if err := client.List(ctx, ado.Request{
		Path:       "process/processes",
		APIVersion: "5.0",
	}, &processes); err != nil {
		return nil, fmt.Errorf("failed to list process templates: %w", err)
	}

	processID, err := projectResolveProcessID(processes, p.Process, p.Name)
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"name":       p.Name,
		"visibility": p.Visibility,
		"capabilities": map[string]any{
			"versioncontrol":  map[string]any{"sourceControlType": p.SourceControl},
			"processTemplate": map[string]any{"templateTypeId": processID},
		},
	}
	if p.Description != "" {
		body["description"] = p.Description
	} else {
		body["description"] = nil
	}

	var ref map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Path:       "projects",
		APIVersion: "5.0",
		Body:       body,
	}, &ref); err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	opID, _ := ref["id"].(string)
	op, err := projectWaitForOperation(ctx, client, opID)
	if err != nil {
		return nil, err
	}
	if status, _ := op["status"].(string); strings.EqualFold(status, "failed") {
		return nil, fmt.Errorf("Project creation failed.")
	} else if strings.EqualFold(status, "cancelled") {
		return nil, fmt.Errorf("Project creation was cancelled.")
	}

	var project map[string]any
	if err := client.Do(ctx, ado.Request{
		Path:       "projects/" + url.PathEscape(p.Name),
		APIVersion: "5.0",
		Query:      map[string][]string{"includeCapabilities": {"true"}},
	}, &project); err != nil {
		return nil, fmt.Errorf("failed to fetch created project: %w", err)
	}

	return project, nil
}

// projectResolveProcessID ports project.py:52-66. Both error messages use
// the project *name*, not the process name — a Python quirk (project.py:59,
// 64) preserved deliberately, not a copy-paste mistake on our side.
func projectResolveProcessID(processes []map[string]any, process, projectName string) (string, error) {
	if process != "" {
		lower := strings.ToLower(process)
		for _, proc := range processes {
			if pname, _ := proc["name"].(string); strings.ToLower(pname) == lower {
				id, _ := proc["id"].(string)
				return id, nil
			}
		}
		return "", fmt.Errorf("Could not find a process template with name: %q", projectName)
	}

	for _, proc := range processes {
		if isDefault, _ := proc["isDefault"].(bool); isDefault {
			id, _ := proc["id"].(string)
			return id, nil
		}
	}
	return "", fmt.Errorf("Could not find a default process template: %q", projectName)
}

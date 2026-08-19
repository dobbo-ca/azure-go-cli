package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newProjectShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show a project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectShow(context.Background(), cmd)
		},
	}

	ado.AddProjectFlag(cmd)
	_ = cmd.MarkFlagRequired("project")
	cmd.Flags().Bool("open", false, "Open the team project in the default web browser.")
	ado.AddOrgFlags(cmd)

	return cmd
}

func runProjectShow(ctx context.Context, cmd *cobra.Command) error {
	open, _ := cmd.Flags().GetBool("open")
	projectID, _ := cmd.Flags().GetString("project")

	// project.py:107,116: show_project(project, ...) has no default for
	// `project` (required), and resolves only the organization
	// (resolve_instance) — no git-detected/config-default project fallback.
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var project map[string]any
	if err := client.Do(ctx, ado.Request{
		Path:       "projects/" + url.PathEscape(projectID),
		APIVersion: "5.0",
		Query:      map[string][]string{"includeCapabilities": {"true"}},
	}, &project); err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	if open {
		if err := projectOpen(project); err != nil {
			return err
		}
	}

	return ado.Print(cmd, project, projectColumns...)
}

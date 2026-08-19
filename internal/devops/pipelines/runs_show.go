package pipelines

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

// newRunsShowCmd is `az pipelines runs show`. Uniquely among the `runs`/
// `runs tag`/`runs artifact` commands, Python names its id argument `--id`,
// not `--run-id` (pipeline_run.py:14 — the function parameter is literally
// `id`) — do not "fix" this to match its siblings.
func newRunsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get the details of a run.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunsShow(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the pipeline run.")
	cmd.MarkFlagRequired("id")
	cmd.Flags().Bool("open", false, "Open the build results page in your web browser.")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func runRunsShow(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	id, _ := cmd.Flags().GetInt("id")
	open, _ := cmd.Flags().GetBool("open")

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var run map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "build/Builds/" + strconv.Itoa(id),
		APIVersion: "5.0",
	}, &run); err != nil {
		return fmt.Errorf("failed to get run: %w", err)
	}

	if open {
		// pipeline_run.py:31 -> _open_pipeline_run (pipeline_run.py:142-155):
		// {org}/{project}/_build/results?buildid={id}, where project is
		// read off the response (run.project.name), not the --project flag.
		project := buildProjectName(run)
		if project == "" {
			project = dctx.Project
		}
		webURL := strings.TrimRight(dctx.Org, "/") + "/" + url.PathEscape(project) +
			"/_build/results?buildid=" + strconv.Itoa(id)
		if err := ado.OpenBrowser(webURL); err != nil {
			logger.Warning("failed to open web browser: %v", err)
		}
	}

	return ado.Print(cmd, run, runsColumns...)
}

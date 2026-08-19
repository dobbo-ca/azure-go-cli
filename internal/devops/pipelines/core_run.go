package pipelines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

func coreNewRunCmd() *cobra.Command {
	var id int
	var name, branch, folderPath, commitID string
	var variables, parameters []string
	var openFlag bool

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Queue (run) a pipeline",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return coreRunRun(context.Background(), cmd, id, name, branch, folderPath, commitID, variables, parameters, openFlag)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	cmd.Flags().IntVar(&id, "id", 0, "ID of the pipeline to queue. Required if --name is not supplied.")
	cmd.Flags().StringVar(&name, "name", "", "Name of the pipeline to queue. Ignored if --id is supplied.")
	cmd.Flags().StringVar(&branch, "branch", "", `Name of the branch on which the pipeline run is to be queued. Example: refs/heads/master or master or refs/pull/1/merge or refs/tags/tag`)
	cmd.Flags().StringVar(&folderPath, "folder-path", "", "Folder path of pipeline. Default is root level folder.")
	// ponytail: Python's --variables/--parameters are argparse nargs='*' (one
	// flag, space-separated values). cobra has no equivalent; StringArrayVar
	// (repeatable flag) is the standard Go substitute — scriptable the same
	// way, just `--variables a=1 --variables b=2` instead of one flag with two
	// values.
	cmd.Flags().StringArrayVar(&variables, "variables", nil, `"name=value" pairs for the variables you would like to set. Repeat the flag for more than one.`)
	cmd.Flags().StringArrayVar(&parameters, "parameters", nil, `"name=value" pairs for the parameters you would like to set. Repeat the flag for more than one. Presence of this flag switches to the pipelines-run API.`)
	cmd.Flags().StringVar(&commitID, "commit-id", "", "Commit-id on which the pipeline run is to be queued.")
	cmd.Flags().BoolVar(&openFlag, "open", false, "Open the pipeline results page in your web browser.")

	return cmd
}

// coreRunRun ports pipeline.py:120-186 pipeline_run. Presence of
// --parameters switches to an entirely different client/API version/body
// shape (path A, v6.0 pipelines run) instead of the classic build-queue path
// (path B, v5.0) — this is the single most consequential quirk in this
// command and is replicated exactly, including that path A's table render
// (coreRunOrDefinitionColumns) is known to mis-render since a v6.0 Run object
// has no buildNumber/queueTime/definition fields (_format.py:213-221).
func coreRunRun(ctx context.Context, cmd *cobra.Command, id int, name, branch, folderPath, commitID string, variables, parameters []string, openFlag bool) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	if id == 0 && name == "" {
		return errors.New("either the --id argument or the --name argument must be supplied for this command")
	}

	return coreRunPipeline(ctx, cmd, dctx, id, name, branch, folderPath, commitID, variables, parameters, openFlag)
}

// coreRunPipeline does the actual client calls, split out from coreRunRun
// for testability (see coreList's doc comment).
func coreRunPipeline(ctx context.Context, cmd *cobra.Command, dctx ado.Context, id int, name, branch, folderPath, commitID string, variables, parameters []string, openFlag bool) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	defID := id
	if defID == 0 {
		defID, err = coreDefinitionIDByName(ctx, client, dctx.Project, name, folderPath, "5.0")
		if err != nil {
			return err
		}
	}

	if len(parameters) > 0 {
		return coreRunPipelineWithParameters(ctx, cmd, client, dctx, defID, branch, commitID, variables, parameters, openFlag)
	}
	return coreQueueBuild(ctx, cmd, client, dctx, defID, branch, commitID, variables, openFlag)
}

// coreRunPipelineWithParameters is path A: POST pipelines/{id}/runs, v6.0
// pipelines client, api-version 6.0-preview.1 (pipeline.py:150-166).
func coreRunPipelineWithParameters(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context, defID int, branch, commitID string, variables, parameters []string, openFlag bool) error {
	templateParameters, err := coreParseNameValuePairs(parameters, "parameters", false)
	if err != nil {
		return err
	}
	varParameters, err := coreParseNameValuePairs(variables, "variables", true)
	if err != nil {
		return err
	}

	self := map[string]any{}
	// pipeline.py:157: branch/commit_id are sent raw here, NOT normalised
	// through resolve_git_ref_heads — unlike path B below.
	if branch != "" {
		self["refName"] = branch
	}
	if commitID != "" {
		self["version"] = commitID
	}

	body := map[string]any{
		"resources": map[string]any{
			"repositories": map[string]any{"self": self},
		},
	}
	if varParameters != nil {
		body["variables"] = varParameters
	}
	if templateParameters != nil {
		body["templateParameters"] = templateParameters
	}

	var run map[string]any
	if err := client.Do(ctx, ado.Request{Method: http.MethodPost, Scope: dctx.Project, Path: fmt.Sprintf("pipelines/%d/runs", defID), APIVersion: "6.0-preview.1", Body: body}, &run); err != nil {
		return fmt.Errorf("failed to run pipeline: %w", err)
	}

	if openFlag {
		coreOpenRunResults(dctx.Org, dctx.Project, run)
	}

	return ado.Print(cmd, run, coreRunOrDefinitionColumns(run)...)
}

// coreQueueBuild is path B: POST build/Builds, v5.0 build client
// (pipeline.py:168-181).
func coreQueueBuild(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context, defID int, branch, commitID string, variables []string, openFlag bool) error {
	body := map[string]any{
		"definition": map[string]any{"id": defID},
	}
	if resolved := coreResolveGitRefHeads(branch); resolved != "" {
		body["sourceBranch"] = resolved
	}
	if commitID != "" {
		body["sourceVersion"] = commitID
	}

	varParameters, err := coreParseNameValuePairs(variables, "variables", false)
	if err != nil {
		return err
	}
	if varParameters != nil {
		// Build.parameters is a JSON-encoded string field
		// (v5_1/build/models.py:439 `'parameters': {'key': 'parameters', 'type': 'str'}`),
		// not a raw object.
		b, err := json.Marshal(varParameters)
		if err != nil {
			return fmt.Errorf("failed to encode --variables: %w", err)
		}
		body["parameters"] = string(b)
	}

	var build map[string]any
	if err := client.Do(ctx, ado.Request{Method: http.MethodPost, Scope: dctx.Project, Path: "build/Builds", APIVersion: "5.0", Body: body}, &build); err != nil {
		return fmt.Errorf("failed to queue build: %w", err)
	}

	if openFlag {
		// pipeline_run.py:180 -> _open_pipeline_run (pipeline_run.py:142-155)
		// reads queued_build.project.name from the response, unlike the
		// v6.0-Pipelines-API path above which is handed --project directly
		// (_open_pipeline_run6_0, pipeline.py:167).
		project := buildProjectName(build)
		if project == "" {
			project = dctx.Project
		}
		coreOpenRunResults(dctx.Org, project, build)
	}

	return ado.Print(cmd, build, coreRunOrDefinitionColumns(build)...)
}

// coreOpenRunResults ports pipeline_run.py's _open_pipeline_run /
// _open_pipeline_run6_0 — both build the same URL shape.
func coreOpenRunResults(org, project string, run map[string]any) {
	u := strings.TrimRight(org, "/") + "/" + url.PathEscape(project) + "/_build/results?buildid=" + url.PathEscape(coreStr(run["id"]))
	if err := ado.OpenBrowser(u); err != nil {
		logger.Warning("failed to open web browser: %v", err)
	}
}

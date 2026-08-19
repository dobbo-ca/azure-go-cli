package pipelines

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func coreNewUpdateCmd() *cobra.Command {
	var id int
	var description, newName, branch, ymlPath, queueID, newFolderPath string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a pipeline",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return coreRunUpdate(context.Background(), cmd, id, description, newName, branch, ymlPath, queueID, newFolderPath)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	cmd.Flags().IntVar(&id, "id", 0, "Id of the pipeline to update.")
	cmd.Flags().StringVar(&description, "description", "", "New description for the pipeline.")
	cmd.Flags().StringVar(&newName, "new-name", "", "New updated name of the pipeline.")
	cmd.Flags().StringVar(&branch, "branch", "", "Branch name for which the pipeline will be configured.")
	cmd.Flags().StringVar(&ymlPath, "yml-path", "", "Path of the pipelines yaml file in the repo.")
	cmd.Flags().StringVar(&ymlPath, "yaml-path", "", "Alias for --yml-path.")
	cmd.Flags().StringVar(&queueID, "queue-id", "", "Queue id of the agent pool where the pipeline needs to run.")
	cmd.Flags().StringVar(&newFolderPath, "new-folder-path", "", `New full path of the folder to move the pipeline to, e.g. "user1/production_pipelines".`)
	cmd.MarkFlagRequired("id")

	return cmd
}

// coreRunUpdate ports pipeline_create.py:158-196 pipeline_update: a
// read-modify-write. Every "set" below is gated on the flag being non-empty,
// matching Python's truthiness checks — an empty string is indistinguishable
// from omitting the flag, kept as a faithful (if surprising) quirk.
// --new-folder-path deliberately skips coreFixPathForAPI (unlike
// create/list/show) — same asymmetry as pipeline_create.py:183-196.
func coreRunUpdate(ctx context.Context, cmd *cobra.Command, id int, description, newName, branch, ymlPath, queueID, newFolderPath string) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return coreUpdate(ctx, cmd, dctx, id, description, newName, branch, ymlPath, queueID, newFolderPath)
}

// coreUpdate does the actual client calls, split out from coreRunUpdate for
// testability (see coreList's doc comment).
func coreUpdate(ctx context.Context, cmd *cobra.Command, dctx ado.Context, id int, description, newName, branch, ymlPath, queueID, newFolderPath string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("build/Definitions/%d", id)

	var def map[string]any
	if err := client.Do(ctx, ado.Request{Scope: dctx.Project, Path: path, APIVersion: "5.1"}, &def); err != nil {
		return fmt.Errorf("failed to get pipeline: %w", err)
	}

	if newName != "" {
		def["name"] = newName
	}
	if description != "" {
		def["description"] = description
	}
	if branch != "" {
		if repo, ok := def["repository"].(map[string]any); ok {
			repo["defaultBranch"] = branch
		}
	}
	if queueID != "" {
		q := map[string]any{}
		if n, err := strconv.Atoi(queueID); err == nil {
			q["id"] = n
		} else {
			q["id"] = queueID
		}
		def["queue"] = q
	}
	if ymlPath != "" {
		def["process"] = map[string]any{"yamlFilename": ymlPath, "type": 2}
	}
	if newFolderPath != "" {
		def["path"] = newFolderPath
	}

	var updated map[string]any
	if err := client.Do(ctx, ado.Request{Method: http.MethodPut, Scope: dctx.Project, Path: path, APIVersion: "5.1", Body: def}, &updated); err != nil {
		return fmt.Errorf("failed to update pipeline: %w", err)
	}

	return ado.Print(cmd, updated, coreDefinitionColumns([]map[string]any{updated})...)
}

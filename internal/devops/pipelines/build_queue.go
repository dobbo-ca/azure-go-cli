package pipelines

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newBuildQueueCmd implements `az pipelines build queue` (build_queue,
// dev/pipelines/build.py:20).
func newBuildQueueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Request (queue) a build",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildRunQueue(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("definition-id", 0, "ID of the definition to queue. Required if --definition-name is not supplied.")
	cmd.Flags().String("definition-name", "", "Name of the definition to queue. Ignored if --definition-id is supplied.")
	cmd.Flags().String("branch", "", "Branch to build. Required if there is not a default branch set up on the definition.")
	// StringArray, not StringSlice: Python's --variables is argparse
	// nargs='*' (space separated, commas are ordinary characters in a
	// value); StringSlice comma-splits, so `--variables "msg=a,b"` would
	// wrongly become ["msg=a","b"] and fail the "name=value" check below.
	cmd.Flags().StringArray("variables", nil, `Space separated "name=value" pairs for the variables you would like to set.`)
	cmd.Flags().Bool("open", false, "Open the build results page in your web browser.")
	cmd.Flags().String("commit-id", "", "Commit ID of the branch to build.")
	cmd.Flags().String("queue-id", "", "Queue Id of the pool that will be used to queue the build.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func buildRunQueue(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return buildQueue(ctx, cmd, client, dctx)
}

func buildQueue(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	definitionID, _ := cmd.Flags().GetInt("definition-id")
	definitionName, _ := cmd.Flags().GetString("definition-name")
	branch, _ := cmd.Flags().GetString("branch")
	variables, _ := cmd.Flags().GetStringArray("variables")
	open, _ := cmd.Flags().GetBool("open")
	commitID, _ := cmd.Flags().GetString("commit-id")
	queueID, _ := cmd.Flags().GetString("queue-id")

	if definitionID == 0 && definitionName == "" {
		return fmt.Errorf("Either the --definition-id argument or the --definition-name argument must be supplied for this command.")
	}
	if definitionID == 0 {
		id, err := buildDefinitionIDFromName(ctx, client, dctx.Project, definitionName)
		if err != nil {
			return err
		}
		definitionID = id
	}

	body := map[string]any{"definition": map[string]any{"id": definitionID}}
	if ref := coreResolveGitRefHeads(branch); ref != "" {
		body["sourceBranch"] = ref
	}
	if commitID != "" {
		body["sourceVersion"] = commitID
	}
	if queueID != "" {
		// build.py:51-53 assigns the --queue-id string straight onto
		// AgentPoolQueue.id, whose wire type is int (models.py); msrest
		// would coerce or fail there. This port parses it explicitly and
		// fails fast on a non-numeric value instead of sending a malformed
		// request (Python-bug policy: fix crashes, don't reproduce them).
		id, err := strconv.Atoi(queueID)
		if err != nil {
			return fmt.Errorf("--queue-id must be a valid integer: %w", err)
		}
		body["queue"] = map[string]any{"id": id}
	}
	if len(variables) > 0 {
		params := map[string]string{}
		for _, v := range variables {
			i := strings.Index(v, "=")
			if i < 0 {
				return fmt.Errorf(`The --variables argument should consist of space separated "name=value" pairs.`)
			}
			params[v[:i]] = v[i+1:]
		}
		// Build.parameters is documented on the wire as a JSON-encoded
		// string (models.py declares it type 'str'), even though build.py
		// assigns it a raw dict — encode it here to match the actual REST
		// contract.
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to encode --variables: %w", err)
		}
		body["parameters"] = string(b)
	}

	var build map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     "POST",
		Scope:      dctx.Project,
		Path:       "build/Builds",
		APIVersion: "5.0",
		Body:       body,
	}, &build); err != nil {
		return fmt.Errorf("failed to queue build: %w", err)
	}

	if open {
		buildOpenInBrowser(buildBuildURL(dctx.Org, build))
	}

	return ado.Print(cmd, build, buildColumns...)
}

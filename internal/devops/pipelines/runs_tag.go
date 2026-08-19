package pipelines

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// runsTagColumns is transform_build_tags_output (_format.py:51-55): one
// "Tags" column, one row per tag string. Shared with `pipelines build tag`,
// which Python renders through the same transformer.
var runsTagColumns = []ado.Column{
	{Header: "Tags", Field: "tag"},
}

func newRunsTagCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage tags for pipeline runs.",
	}
	cmd.AddCommand(newRunsTagAddCmd())
	cmd.AddCommand(newRunsTagListCmd())
	cmd.AddCommand(newRunsTagDeleteCmd())
	return cmd
}

func newRunsTagAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a tag for a pipeline run.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunsTagAdd(context.Background(), cmd)
		},
	}
	cmd.Flags().Int("run-id", 0, "ID of the pipeline run.")
	cmd.MarkFlagRequired("run-id")
	cmd.Flags().String("tags", "", "Tag(s) to add. Comma separated values.")
	cmd.MarkFlagRequired("tags")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	return cmd
}

func runRunsTagAdd(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	runID, _ := cmd.Flags().GetInt("run-id")
	tagsRaw, _ := cmd.Flags().GetString("tags")

	// pipeline_run.py:102: no trim, no dedup — matches the CLI's actual
	// quirky behaviour.
	tags := strings.Split(tagsRaw, ",")

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var result []string
	base := "build/builds/" + strconv.Itoa(runID) + "/tags"
	if len(tags) == 1 {
		if err := client.List(ctx, ado.Request{
			Method:     "PUT",
			Scope:      dctx.Project,
			Path:       base + "/" + url.PathEscape(tags[0]),
			APIVersion: "5.0",
		}, &result); err != nil {
			return fmt.Errorf("failed to add tag: %w", err)
		}
	} else {
		if err := client.List(ctx, ado.Request{
			Method:     "POST",
			Scope:      dctx.Project,
			Path:       base,
			APIVersion: "5.0",
			Body:       tags,
		}, &result); err != nil {
			return fmt.Errorf("failed to add tags: %w", err)
		}
	}

	return runsPrintTags(cmd, result)
}

func newRunsTagListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Get tags for a pipeline run.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunsTagList(context.Background(), cmd)
		},
	}
	cmd.Flags().Int("run-id", 0, "ID of the pipeline run.")
	cmd.MarkFlagRequired("run-id")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	return cmd
}

func runRunsTagList(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	runID, _ := cmd.Flags().GetInt("run-id")

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var tags []string
	if err := client.List(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "build/builds/" + strconv.Itoa(runID) + "/tags",
		APIVersion: "5.0",
	}, &tags); err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}

	return runsPrintTags(cmd, tags)
}

func newRunsTagDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a pipeline run tag.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunsTagDelete(context.Background(), cmd)
		},
	}
	cmd.Flags().Int("run-id", 0, "ID of the pipeline run.")
	cmd.MarkFlagRequired("run-id")
	cmd.Flags().String("tag", "", "Tag to delete.")
	cmd.MarkFlagRequired("tag")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	// commands.py:151 registers no confirmation for this command, unlike
	// most other delete commands in the surface — no --yes flag here.
	return cmd
}

func runRunsTagDelete(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	runID, _ := cmd.Flags().GetInt("run-id")
	tag, _ := cmd.Flags().GetString("tag")

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var result []string
	if err := client.List(ctx, ado.Request{
		Method:     "DELETE",
		Scope:      dctx.Project,
		Path:       "build/builds/" + strconv.Itoa(runID) + "/tags/" + url.PathEscape(tag),
		APIVersion: "5.0",
	}, &result); err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}

	return runsPrintTags(cmd, result)
}

// runsPrintTags routes a []string tag list through ado.Print, for both
// `pipelines runs tag` and `pipelines build tag`. Deviation:
// ado.Print's table path (tableRows in print.go) only extracts
// map[string]any elements from a JSON-round-tripped slice — a bare
// []string collapses to zero rows for -o table, which is a gap in the
// foundation we are not permitted to edit (only files prefixed "runs" are
// ours to touch). For table mode only, wrap each tag as {"tag": t}; every
// other format (json/yaml/tsv, or any --query) gets the untouched []string,
// matching Python's untransformed list-of-str output.
func runsPrintTags(cmd *cobra.Command, tags []string) error {
	if ado.TableMode(cmd) {
		rows := make([]map[string]any, len(tags))
		for i, t := range tags {
			rows[i] = map[string]any{"tag": t}
		}
		return ado.Print(cmd, rows, runsTagColumns...)
	}
	return ado.Print(cmd, tags, runsTagColumns...)
}

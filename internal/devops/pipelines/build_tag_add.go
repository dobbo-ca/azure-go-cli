package pipelines

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newBuildTagAddCmd implements `az pipelines build tag add` (add_build_tags,
// dev/pipelines/build.py:144).
func newBuildTagAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add tag(s) for a build",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildRunTagAdd(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("build-id", 0, "ID of the build.")
	cmd.MarkFlagRequired("build-id")
	cmd.Flags().String("tags", "", "Tag(s) to be added to the build. [Comma separated values]")
	cmd.MarkFlagRequired("tags")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func buildRunTagAdd(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return buildTagAdd(ctx, cmd, client, dctx)
}

func buildTagAdd(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	buildID, _ := cmd.Flags().GetInt("build-id")
	tagsRaw, _ := cmd.Flags().GetString("tags")

	// build.py:156: tags.split(',') with no whitespace trim and no dedup —
	// "a, b" yields tags "a" and " b" (leading space kept). Replicated
	// deliberately for parity.
	tags := strings.Split(tagsRaw, ",")

	var result []string
	if len(tags) == 1 {
		if err := client.List(ctx, ado.Request{
			Method:     "PUT",
			Scope:      dctx.Project,
			Path:       fmt.Sprintf("build/builds/%d/tags/%s", buildID, url.PathEscape(tags[0])),
			APIVersion: "5.0",
		}, &result); err != nil {
			return fmt.Errorf("failed to add build tag: %w", err)
		}
	} else {
		if err := client.List(ctx, ado.Request{
			Method:     "POST",
			Scope:      dctx.Project,
			Path:       fmt.Sprintf("build/builds/%d/tags", buildID),
			APIVersion: "5.0",
			Body:       tags,
		}, &result); err != nil {
			return fmt.Errorf("failed to add build tags: %w", err)
		}
	}

	return runsPrintTags(cmd, result)
}

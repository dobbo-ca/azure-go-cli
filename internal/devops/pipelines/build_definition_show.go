package pipelines

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newBuildDefinitionShowCmd implements `az pipelines build definition show`
// (build_definition_show, dev/pipelines/build_definition.py:55).
func newBuildDefinitionShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get the details of a build definition",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildRunDefinitionShow(context.Background(), cmd)
		},
	}

	cmd.Flags().Int("id", 0, "ID of the definition. Required if --name is not supplied.")
	cmd.Flags().String("name", "", "Name of the definition. Ignored if --id is supplied.")
	cmd.Flags().Bool("open", false, "Open the definition summary page in your web browser.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func buildRunDefinitionShow(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return buildDefinitionShow(ctx, cmd, client, dctx)
}

func buildDefinitionShow(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context) error {
	id, _ := cmd.Flags().GetInt("id")
	name, _ := cmd.Flags().GetString("name")
	open, _ := cmd.Flags().GetBool("open")

	if id == 0 {
		if name == "" {
			return fmt.Errorf("Either the --id argument or the --name argument must be supplied for this command.")
		}
		resolvedID, err := buildDefinitionIDFromName(ctx, client, dctx.Project, name)
		if err != nil {
			return err
		}
		id = resolvedID
	}

	var def map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       fmt.Sprintf("build/Definitions/%d", id),
		APIVersion: "5.0",
	}, &def); err != nil {
		return fmt.Errorf("failed to get build definition: %w", err)
	}

	if open {
		buildOpenInBrowser(buildDefinitionURL(dctx.Org, def))
	}

	return ado.Print(cmd, def, buildDefinitionColumns([]map[string]any{def})...)
}

package pipelines

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func releaseDefinitionNewShowCmd() *cobra.Command {
	var id int
	var name string
	var open bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get the details of a release definition.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return releaseDefinitionRunShow(context.Background(), cmd, id, name, open)
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "ID of the definition.")
	cmd.Flags().StringVar(&name, "name", "", "Name of the definition. Ignored if --id is supplied.")
	cmd.Flags().BoolVar(&open, "open", false, "Open the definition summary page in your web browser.")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func releaseDefinitionRunShow(ctx context.Context, cmd *cobra.Command, id int, name string, open bool) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	if id == 0 {
		if name == "" {
			return fmt.Errorf("Either the --id argument or the --name argument must be supplied for this command.")
		}
		resolved, err := releaseResolveDefinitionID(ctx, client, dctx.Project, name)
		if err != nil {
			return err
		}
		id = resolved
	}

	var def map[string]any
	if err := client.Do(ctx, ado.Request{
		Host:       releaseHost,
		Scope:      dctx.Project,
		Path:       fmt.Sprintf("release/definitions/%d", id),
		APIVersion: "5.0",
	}, &def); err != nil {
		return fmt.Errorf("failed to get release definition: %w", err)
	}

	if open {
		if u := releaseWebURL(def); u != "" {
			if err := ado.OpenBrowser(u); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to open browser: %v\n", err)
			}
		}
	}

	return ado.Print(cmd, def, releaseDefinitionColumns...)
}

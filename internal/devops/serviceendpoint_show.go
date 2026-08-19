package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func serviceendpointNewShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get the details of a service endpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serviceendpointRunShow(context.Background(), cmd)
		},
	}

	cmd.Flags().String("id", "", "ID of the service endpoint.")
	_ = cmd.MarkFlagRequired("id")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func serviceendpointRunShow(ctx context.Context, cmd *cobra.Command) error {
	id, _ := cmd.Flags().GetString("id")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var endpoint map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "serviceendpoint/endpoints/" + url.PathEscape(id),
		APIVersion: "5.0-preview.2",
	}, &endpoint); err != nil {
		return fmt.Errorf("failed to get service endpoint: %w", err)
	}

	// No table transformer (commands.py:113: "no table transform because
	// type is not well defined" — data/authorization.parameters vary per
	// endpoint type). ado.Print falls back to JSON for -o table with no
	// columns.
	return ado.Print(cmd, endpoint)
}

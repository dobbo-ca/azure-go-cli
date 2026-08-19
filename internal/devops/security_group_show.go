package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

var securityGroupShowColumns = []ado.Column{
	{Header: "Name", Field: "principalName"},
	{Header: "Description", Field: "description"},
}

func securityGroupShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show group details.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityGroupShowRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	cmd.Flags().String("id", "", "Descriptor of the group.")
	cmd.MarkFlagRequired("id")

	return cmd
}

func securityGroupShowRun(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := securityNewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	id, _ := cmd.Flags().GetString("id")

	var group map[string]any
	if err := client.Do(ctx, ado.Request{
		Host:       "vssps",
		Path:       "Graph/Groups/" + url.PathEscape(id),
		APIVersion: "5.0-preview.1",
	}, &group); err != nil {
		return fmt.Errorf("failed to show group: %w", err)
	}

	return ado.Print(cmd, group, securityGroupShowColumns...)
}

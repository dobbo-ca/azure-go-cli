package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

var securityNamespaceListColumns = []ado.Column{
	{Header: "Id", Field: "namespaceId"},
	{Header: "Name", Field: "name"},
}

func securityPermissionNamespaceListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available namespaces for an organization.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityPermissionNamespaceListRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	cmd.Flags().Bool("local-only", false, "If true, retrieve only local security namespaces.")

	return cmd
}

func securityPermissionNamespaceListRun(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := securityNewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	localOnly, _ := cmd.Flags().GetBool("local-only")

	var namespaces []map[string]any
	if err := client.List(ctx, ado.Request{
		Path:       "SecurityNamespaces",
		APIVersion: "5.0",
		Query:      url.Values{"localOnly": {fmt.Sprint(localOnly)}},
	}, &namespaces); err != nil {
		return fmt.Errorf("failed to list security namespaces: %w", err)
	}

	return ado.Print(cmd, namespaces, securityNamespaceListColumns...)
}

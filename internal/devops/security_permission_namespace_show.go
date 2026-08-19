package devops

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

var securityNamespaceShowColumns = []ado.Column{
	{Header: "Name", Field: "name"},
	{Header: "Permission Description", Field: "displayName"},
	{Header: "Permission Bit", Field: "bit"},
}

func securityPermissionNamespaceShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of permissions available in each namespace.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityPermissionNamespaceShowRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	securityAddNamespaceIDFlag(cmd)

	return cmd
}

func securityPermissionNamespaceShowRun(ctx context.Context, cmd *cobra.Command) error {
	namespaceID, err := securityNamespaceID(cmd)
	if err != nil {
		return err
	}

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := securityNewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	// security_permission.py:30 (show_namespace) returns _get_permission_types
	// verbatim — the full [SecurityNamespaceDescription] list — not just its
	// actions; transform_namespace_table_output (_format.py:202-206) only
	// digs into result[0]['actions'] for table rendering.
	namespaces, err := securityQuerySecurityNamespaceList(ctx, client, namespaceID)
	if err != nil {
		return fmt.Errorf("failed to show security namespace: %w", err)
	}

	if ado.TableMode(cmd) {
		if len(namespaces) == 0 {
			return fmt.Errorf("security namespace not found: %s", namespaceID)
		}
		actions, _ := namespaces[0]["actions"].([]any)
		rows := make([]map[string]any, 0, len(actions))
		for _, a := range actions {
			if m, ok := a.(map[string]any); ok {
				rows = append(rows, m)
			}
		}
		return ado.Print(cmd, rows, securityNamespaceShowColumns...)
	}

	return ado.Print(cmd, namespaces, securityNamespaceShowColumns...)
}

package devops

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

var serviceendpointListColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Field: "name"},
	{Header: "Type", Field: "type"},
	{Header: "Is Ready", Field: "isReady"},
	{Header: "Created By", Field: "createdBy.displayName"},
}

func serviceendpointNewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List service endpoints in a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serviceendpointRunList(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func serviceendpointRunList(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var endpoints []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "serviceendpoint/endpoints",
		APIVersion: "5.0-preview.2",
	}, &endpoints); err != nil {
		return fmt.Errorf("failed to list service endpoints: %w", err)
	}

	// _format.py:88-92: the table transformer sorts by name.lower() before
	// rendering; JSON/tsv keep the server's original order.
	if ado.TableMode(cmd) {
		sort.Slice(endpoints, func(i, j int) bool {
			return strings.ToLower(fmt.Sprint(endpoints[i]["name"])) < strings.ToLower(fmt.Sprint(endpoints[j]["name"]))
		})
	}

	return ado.Print(cmd, endpoints, serviceendpointListColumns...)
}

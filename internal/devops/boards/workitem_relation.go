package boards

import (
	"context"
	"fmt"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// workitemRelationCommand wires the `az boards work-item relation`
// subgroup: list-type, add, remove, show. Registered under the
// `boards work-item` command group in Python (commands.py:57-63), not a
// separate command_group -- there is no dedicated relations.py group, so
// this is nested under work-item here too.
func workitemRelationCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relation",
		Short: "Manage work item relations.",
	}

	cmd.AddCommand(workitemRelationListTypeCmd())
	cmd.AddCommand(workitemRelationAddCmd())
	cmd.AddCommand(workitemRelationRemoveCmd())
	cmd.AddCommand(workitemRelationShowCmd())

	return cmd
}

// workitemRelationTypeColumns is transform_work_item_relation_type_table_output's
// row shape (_format.py:13-23).
var workitemRelationTypeColumns = []ado.Column{
	{Header: "Name", Field: "name"},
	{Header: "ReferenceName", Field: "referenceName"},
	{Header: "Enabled", Field: "attributes.enabled"},
	{Header: "Usage", Field: "attributes.usage"},
}

// workitemGetRelationTypes is get_relation_types (relations.py:16-21):
// a single organization-scoped GET, no project.
func workitemGetRelationTypes(ctx context.Context, client *ado.Client) ([]map[string]any, error) {
	var relationTypes []map[string]any
	if err := client.List(ctx, ado.Request{
		Path:       "wit/workitemrelationtypes",
		APIVersion: "5.0",
	}, &relationTypes); err != nil {
		return nil, fmt.Errorf("failed to get work item relation types: %w", err)
	}
	return relationTypes, nil
}

// workitemSystemRelationName is get_system_relation_name (relations.py:132-138):
// a case-insensitive friendly-name lookup against the server's relation
// types, e.g. "parent" -> "System.LinkTypes.Hierarchy-Reverse".
func workitemSystemRelationName(relationTypes []map[string]any, relationType string) (string, error) {
	for _, rt := range relationTypes {
		if name, _ := rt["name"].(string); strings.EqualFold(name, relationType) {
			ref, _ := rt["referenceName"].(string)
			return ref, nil
		}
	}
	return "", fmt.Errorf(`--relation-type is not valid. Use "az boards work-item relation list-type" command to list possible relation types in your project`)
}

// workitemRelationListTypeCmd is `az boards work-item relation list-type`,
// port of get_relation_types_show (relations.py:16).
func workitemRelationListTypeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-type",
		Short: "List work item relations supported in the organization.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return workitemRunRelationListType(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)

	return cmd
}

func workitemRunRelationListType(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}
	return workitemRelationListType(ctx, cmd, dctx)
}

// workitemRelationListType does the actual client call, split out from
// workitemRunRelationListType so tests can supply a dctx pointing at an
// httptest server without going through org validation.
func workitemRelationListType(ctx context.Context, cmd *cobra.Command, dctx ado.Context) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	relationTypes, err := workitemGetRelationTypes(ctx, client)
	if err != nil {
		return err
	}

	return ado.Print(cmd, relationTypes, workitemRelationTypeColumns...)
}

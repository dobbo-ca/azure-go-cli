package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// securityMembershipName mirrors _transform_membership_row's Name logic:
// displayName for a user, principalName otherwise.
func securityMembershipName(row map[string]any) string {
	if kind, _ := row["subjectKind"].(string); kind == "user" {
		return ado.TSVScalar(row["displayName"])
	}
	return ado.TSVScalar(row["principalName"])
}

var securityMembershipListColumns = []ado.Column{
	{Header: "Name", Value: securityMembershipName},
	{Header: "Type", Field: "subjectKind"},
	{Header: "Email", Field: "mailAddress"},
	{Header: "Descriptor", Field: "descriptor"},
}

func securityGroupMembershipListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List memberships for a group or user.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityGroupMembershipListRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	cmd.Flags().String("id", "", "Group descriptor or User Email whose membership details are required.")
	cmd.Flags().String("relationship", "members", "Get member of/members for this group. Allowed values: members, memberof.")
	cmd.MarkFlagRequired("id")

	return cmd
}

func securityGroupMembershipListRun(ctx context.Context, cmd *cobra.Command) error {
	relationship, _ := cmd.Flags().GetString("relationship")
	if relationship != "members" && relationship != "memberof" {
		return fmt.Errorf("--relationship must be one of members, memberof")
	}

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := securityNewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	id, _ := cmd.Flags().GetString("id")
	subjectDescriptor, err := securityResolveMemberDescriptor(ctx, client, id)
	if err != nil {
		return fmt.Errorf("failed to resolve identity: %w", err)
	}

	direction := "down"
	if relationship == "memberof" {
		direction = "up"
	}

	var memberships []map[string]any
	if err := client.List(ctx, ado.Request{
		Host:       "vssps",
		Path:       "Graph/Memberships/" + url.PathEscape(subjectDescriptor),
		APIVersion: "5.0-preview.1",
		Query:      url.Values{"direction": {direction}},
	}, &memberships); err != nil {
		return fmt.Errorf("failed to list memberships: %w", err)
	}

	// security_group.py:163-169: always looks up the *other party* in each
	// edge - the member descriptor when listing down, the container
	// descriptor when listing up.
	lookupKeys := make([]map[string]string, 0, len(memberships))
	for _, m := range memberships {
		key := "memberDescriptor"
		if relationship == "memberof" {
			key = "containerDescriptor"
		}
		if d, ok := m[key].(string); ok {
			lookupKeys = append(lookupKeys, map[string]string{"descriptor": d})
		}
	}

	subjects, err := securityLookupSubjects(ctx, client, lookupKeys)
	if err != nil {
		return fmt.Errorf("failed to resolve members: %w", err)
	}

	// security_group.py:171-172: list_membership returns lookup_subjects'
	// descriptor-keyed dict verbatim (rtype {GraphSubject}), not an array —
	// print that for JSON/tsv/--query. transform_memberships_table_output
	// (_format.py:153-158) iterates in membership-edge (lookupKeys) order
	// for the table only; ranging the Go map instead would randomize -o
	// table row order run to run.
	if ado.TableMode(cmd) {
		rows := make([]map[string]any, 0, len(lookupKeys))
		for _, key := range lookupKeys {
			if s, ok := subjects[key["descriptor"]]; ok {
				rows = append(rows, s)
			}
		}
		return ado.Print(cmd, rows, securityMembershipListColumns...)
	}

	return ado.Print(cmd, subjects, securityMembershipListColumns...)
}

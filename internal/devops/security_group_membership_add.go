package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// securityMembershipAddColumns is transform_membership_table_output: same
// Name/Type/Email cells as list, but no Descriptor column
// (dev/team/_format.py:161-172).
var securityMembershipAddColumns = []ado.Column{
	{Header: "Name", Value: securityMembershipName},
	{Header: "Type", Field: "subjectKind"},
	{Header: "Email", Field: "mailAddress"},
}

func securityGroupMembershipAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add membership.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityGroupMembershipAddRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	cmd.Flags().String("member-id", "", "Descriptor of the group or Email Id of the user to be added. User should already be a part of the organization.")
	cmd.Flags().String("group-id", "", "Descriptor of the group to which member needs to be added.")
	cmd.MarkFlagRequired("member-id")
	cmd.MarkFlagRequired("group-id")

	return cmd
}

func securityGroupMembershipAddRun(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := securityNewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	memberID, _ := cmd.Flags().GetString("member-id")
	groupID, _ := cmd.Flags().GetString("group-id")

	subjectDescriptor, err := securityResolveMemberDescriptor(ctx, client, memberID)
	if err != nil {
		return fmt.Errorf("failed to resolve identity: %w", err)
	}

	var membership map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     "PUT",
		Host:       "vssps",
		Path:       "Graph/Memberships/" + url.PathEscape(subjectDescriptor) + "/" + url.PathEscape(groupID),
		APIVersion: "5.0-preview.1",
	}, &membership); err != nil {
		return fmt.Errorf("failed to add membership: %w", err)
	}

	container, _ := membership["containerDescriptor"].(string)
	member, _ := membership["memberDescriptor"].(string)

	// security_group.py:192-198: hydrates both parties, container first.
	subjects, err := securityLookupSubjects(ctx, client, []map[string]string{
		{"descriptor": container},
		{"descriptor": member},
	})
	if err != nil {
		return fmt.Errorf("failed to resolve members: %w", err)
	}

	// security_group.py:198 returns lookup_subjects' descriptor-keyed dict
	// verbatim — print that for JSON/tsv/--query.
	// transform_membership_table_output (_format.py:161-165) iterates in
	// container-then-member order for the table only.
	if ado.TableMode(cmd) {
		rows := make([]map[string]any, 0, 2)
		if s, ok := subjects[container]; ok {
			rows = append(rows, s)
		}
		if s, ok := subjects[member]; ok {
			rows = append(rows, s)
		}
		return ado.Print(cmd, rows, securityMembershipAddColumns...)
	}

	return ado.Print(cmd, subjects, securityMembershipAddColumns...)
}

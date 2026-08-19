package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func securityGroupMembershipRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove membership.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityGroupMembershipRemoveRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddYesFlag(cmd)
	cmd.Flags().String("member-id", "", "Descriptor of the group or Email Id of the user to be removed.")
	cmd.Flags().String("group-id", "", "Descriptor of the group from which member needs to be removed.")
	cmd.MarkFlagRequired("member-id")
	cmd.MarkFlagRequired("group-id")

	return cmd
}

func securityGroupMembershipRemoveRun(ctx context.Context, cmd *cobra.Command) error {
	if err := ado.Confirm(cmd, "Are you sure you want to delete this relationship?"); err != nil {
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

	memberID, _ := cmd.Flags().GetString("member-id")
	groupID, _ := cmd.Flags().GetString("group-id")

	subjectDescriptor, err := securityResolveMemberDescriptor(ctx, client, memberID)
	if err != nil {
		return fmt.Errorf("failed to resolve identity: %w", err)
	}

	path := "Graph/Memberships/" + url.PathEscape(subjectDescriptor) + "/" + url.PathEscape(groupID)

	// security_group.py:215-222: a HEAD existence check turns a raw SDK
	// error into this friendlier (grammatically off, preserved verbatim)
	// message, and the DELETE is never issued when it doesn't exist.
	if err := client.Do(ctx, ado.Request{
		Method:     "HEAD",
		Host:       "vssps",
		Path:       path,
		APIVersion: "5.0-preview.1",
	}, nil); err != nil {
		return fmt.Errorf("Membership doesn't exists.")
	}

	if err := client.Do(ctx, ado.Request{
		Method:     "DELETE",
		Host:       "vssps",
		Path:       path,
		APIVersion: "5.0-preview.1",
	}, nil); err != nil {
		return fmt.Errorf("failed to remove membership: %w", err)
	}

	return ado.Print(cmd, nil)
}

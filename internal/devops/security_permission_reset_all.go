package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func securityPermissionResetAllCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset-all",
		Short: "Clear all permissions of this token for a user/group.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityPermissionResetAllRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddYesFlag(cmd)
	securityAddNamespaceIDFlag(cmd)
	cmd.Flags().String("subject", "", "User Email ID or Group descriptor")
	cmd.Flags().String("token", "", "Security token.")
	cmd.MarkFlagRequired("subject")
	cmd.MarkFlagRequired("token")

	return cmd
}

func securityPermissionResetAllRun(ctx context.Context, cmd *cobra.Command) error {
	namespaceID, err := securityNamespaceID(cmd)
	if err != nil {
		return err
	}

	if err := ado.Confirm(cmd, "Are you sure you want to reset all explicit permissions for this user/group and token?"); err != nil {
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

	subject, _ := cmd.Flags().GetString("subject")
	token, _ := cmd.Flags().GetString("token")

	subjectDescriptor, err := securityResolveSubjectAsIdentityDescriptor(ctx, client, subject)
	if err != nil {
		return fmt.Errorf("failed to resolve subject: %w", err)
	}

	q := url.Values{"token": {token}, "descriptors": {subjectDescriptor}}

	// dev/team/commands.py:164-165: no table_transformer for this command.
	var result bool
	if err := client.Do(ctx, ado.Request{
		Method:     "DELETE",
		Path:       "AccessControlEntries/" + url.PathEscape(namespaceID),
		APIVersion: "5.0",
		Query:      q,
	}, &result); err != nil {
		return fmt.Errorf("failed to reset permissions: %w", err)
	}

	return ado.Print(cmd, result)
}

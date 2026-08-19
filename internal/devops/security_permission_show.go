package devops

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func securityPermissionShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show permissions for given token, namespace and user/group.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityPermissionShowRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	securityAddNamespaceIDFlag(cmd)
	cmd.Flags().String("subject", "", "User Email ID or Group descriptor")
	cmd.Flags().String("token", "", "Security token.")
	cmd.MarkFlagRequired("subject")
	cmd.MarkFlagRequired("token")

	return cmd
}

func securityPermissionShowRun(ctx context.Context, cmd *cobra.Command) error {
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

	subject, _ := cmd.Flags().GetString("subject")
	token, _ := cmd.Flags().GetString("token")

	subjectDescriptor, err := securityResolveSubjectAsIdentityDescriptor(ctx, client, subject)
	if err != nil {
		return fmt.Errorf("failed to resolve subject: %w", err)
	}

	// changedBits=0: display every permission the namespace defines
	// (security_permission.py:58).
	result, err := securityResolvedPermissions(ctx, cmd, client, namespaceID, subjectDescriptor, token, 0)
	if err != nil {
		return fmt.Errorf("failed to show permissions: %w", err)
	}

	return ado.Print(cmd, result, securityPermissionBitsColumns...)
}

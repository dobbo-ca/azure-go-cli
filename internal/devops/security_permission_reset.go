package devops

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func securityPermissionResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset permission for given permission bit(s).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityPermissionResetRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	securityAddNamespaceIDFlag(cmd)
	cmd.Flags().Int("permission-bit", 0, "Permission bit or addition of permission bits which needs to be reset for given user/group and token.")
	cmd.Flags().String("subject", "", "User Email ID or Group descriptor")
	cmd.Flags().String("token", "", "Security token.")
	cmd.MarkFlagRequired("permission-bit")
	cmd.MarkFlagRequired("subject")
	cmd.MarkFlagRequired("token")

	return cmd
}

func securityPermissionResetRun(ctx context.Context, cmd *cobra.Command) error {
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

	permissionBit, _ := cmd.Flags().GetInt("permission-bit")
	subject, _ := cmd.Flags().GetString("subject")
	token, _ := cmd.Flags().GetString("token")

	subjectDescriptor, err := securityResolveSubjectAsIdentityDescriptor(ctx, client, subject)
	if err != nil {
		return fmt.Errorf("failed to resolve subject: %w", err)
	}

	// security_client.remove_permission: `permissions` is a route segment
	// here, not a query param - a different shape from AccessControlEntries.
	path := "Permissions/" + url.PathEscape(namespaceID) + "/" + strconv.Itoa(permissionBit)
	q := url.Values{"descriptor": {subjectDescriptor}, "token": {token}}
	if err := client.Do(ctx, ado.Request{
		Method:     "DELETE",
		Path:       path,
		APIVersion: "5.0",
		Query:      q,
	}, nil); err != nil {
		return fmt.Errorf("failed to reset permission: %w", err)
	}

	result, err := securityResolvedPermissions(ctx, cmd, client, namespaceID, subjectDescriptor, token, permissionBit)
	if err != nil {
		return fmt.Errorf("failed to show updated permissions: %w", err)
	}

	return ado.Print(cmd, result, securityPermissionBitsColumns...)
}

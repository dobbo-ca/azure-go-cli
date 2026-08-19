package devops

import (
	"context"
	"fmt"
	"sort"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// securityFirstACE picks a value out of an ACL's acesDictionary.
// dev/team/_format.py:224-226 (_transform_acl_details_row) raises CLIError
// when there is more than one entry; this port takes a value instead of
// replicating that render-time-only guard, but picks the lowest descriptor
// key rather than ranging the Go map — map iteration order is randomized
// per run, which made Effective Allow/Deny vary between identical runs for
// a multi-ACE ACL.
func securityFirstACE(row map[string]any) map[string]any {
	acesDict, _ := row["acesDictionary"].(map[string]any)
	if len(acesDict) == 0 {
		return nil
	}
	keys := make([]string, 0, len(acesDict))
	for k := range acesDict {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	m, _ := acesDict[keys[0]].(map[string]any)
	return m
}

func securityEffectiveBit(row map[string]any, key string) string {
	ace := securityFirstACE(row)
	if ace == nil {
		return "0"
	}
	ext, _ := ace["extendedInfo"].(map[string]any)
	if ext == nil {
		return "0"
	}
	v, ok := ext[key]
	if !ok || v == nil {
		return "0"
	}
	return ado.TSVScalar(v)
}

var securityACLColumns = []ado.Column{
	{Header: "token", Field: "token"},
	{Header: "Effective Allow", Value: func(row map[string]any) string { return securityEffectiveBit(row, "effectiveAllow") }},
	{Header: "Effective Deny", Value: func(row map[string]any) string { return securityEffectiveBit(row, "effectiveDeny") }},
}

func securityPermissionListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tokens for given user/group and namespace.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityPermissionListRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	securityAddNamespaceIDFlag(cmd)
	cmd.Flags().String("subject", "", "User Email ID or Group descriptor")
	cmd.Flags().String("token", "", "Security token.")
	cmd.Flags().Bool("recurse", false, "If true and this is a hierarchical namespace, return child ACLs of the specified token.")
	cmd.MarkFlagRequired("subject")

	return cmd
}

func securityPermissionListRun(ctx context.Context, cmd *cobra.Command) error {
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
	recurse, _ := cmd.Flags().GetBool("recurse")

	subjectDescriptor, err := securityResolveSubjectAsIdentityDescriptor(ctx, client, subject)
	if err != nil {
		return fmt.Errorf("failed to resolve subject: %w", err)
	}

	acls, err := securityQueryACL(ctx, client, namespaceID, subjectDescriptor, token, recurse)
	if err != nil {
		return fmt.Errorf("failed to list permissions: %w", err)
	}

	return ado.Print(cmd, acls, securityACLColumns...)
}

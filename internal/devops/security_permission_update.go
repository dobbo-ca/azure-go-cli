package devops

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func securityPermissionUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Assign allow or deny permission to given user/group.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityPermissionUpdateRun(context.Background(), cmd, args)
		},
	}

	ado.AddOrgFlags(cmd)
	securityAddNamespaceIDFlag(cmd)
	cmd.Flags().String("subject", "", "User Email ID or Group descriptor")
	cmd.Flags().String("token", "", "Security token.")
	// Three-state, matching context.go's --detect idiom: unset means true.
	cmd.Flags().String("merge", "true", "If set, the existing ACE has its allow and deny merged with the incoming ACE's allow and deny. If unset, the existing ACE is displaced.")
	cmd.Flags().Lookup("merge").NoOptDefVal = "true"
	cmd.Flags().Int("allow-bit", 0, "Allow bit or addition of bits. Required if --deny-bit is missing.")
	cmd.Flags().Int("deny-bit", 0, "Deny bit or addition of bits. Required if --allow-bit is missing.")
	cmd.MarkFlagRequired("subject")
	cmd.MarkFlagRequired("token")

	return cmd
}

func securityPermissionUpdateRun(ctx context.Context, cmd *cobra.Command, args []string) error {
	namespaceID, err := securityNamespaceID(cmd)
	if err != nil {
		return err
	}

	allowBit, _ := cmd.Flags().GetInt("allow-bit")
	denyBit, _ := cmd.Flags().GetInt("deny-bit")
	if allowBit == 0 && denyBit == 0 {
		return fmt.Errorf("Either --allow-bit or --deny-bit parameter should be provided.")
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
	mergeStr, _ := cmd.Flags().GetString("merge")
	// Space-separated "--merge false" leaves "false" as a stray positional
	// (pflag has no lookahead for a string flag's value); pick it up the
	// way core_invoke.go's nargs handling does.
	if len(args) > 0 {
		mergeStr = args[0]
	}
	merge := !strings.EqualFold(mergeStr, "false")

	subjectDescriptor, err := securityResolveSubjectAsIdentityDescriptor(ctx, client, subject)
	if err != nil {
		return fmt.Errorf("failed to resolve subject: %w", err)
	}

	body := map[string]any{
		"token": token,
		"merge": merge,
		"accessControlEntries": []map[string]any{
			{"allow": allowBit, "deny": denyBit, "descriptor": subjectDescriptor},
		},
	}
	if err := client.Do(ctx, ado.Request{
		Method:     "POST",
		Path:       "AccessControlEntries/" + url.PathEscape(namespaceID),
		APIVersion: "5.0",
		Body:       body,
	}, nil); err != nil {
		return fmt.Errorf("failed to update permissions: %w", err)
	}

	// security_permission.py:113-114: deny wins for any bit set in both,
	// but only for the locally-computed display mask - the raw bits above
	// were already POSTed as-is.
	changedBits := (allowBit &^ denyBit) + denyBit

	result, err := securityResolvedPermissions(ctx, cmd, client, namespaceID, subjectDescriptor, token, changedBits)
	if err != nil {
		return fmt.Errorf("failed to show updated permissions: %w", err)
	}

	return ado.Print(cmd, result, securityPermissionBitsColumns...)
}

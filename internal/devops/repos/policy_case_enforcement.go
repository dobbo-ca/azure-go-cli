package repos

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPolicyCaseEnforcementCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "case-enforcement",
		Short: "Manage case enforcement policies.",
	}
	cmd.AddCommand(newPolicyCaseEnforcementCreateCmd())
	cmd.AddCommand(newPolicyCaseEnforcementUpdateCmd())
	return cmd
}

func newPolicyCaseEnforcementCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create case enforcement policy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunCaseEnforcementCreate(context.Background(), cmd)
		},
	}

	// No branch, no path-filter, no other settings flags at all - repo-wide,
	// policy.py:573-583.
	cmd.Flags().String("repository-id", "", "Id of the repository on which to apply the policy")
	policyAddTriStateFlag(cmd, "blocking", "Whether the policy should be blocking or not")
	policyAddTriStateFlag(cmd, "enabled", "Whether the policy is enabled or not")
	_ = cmd.MarkFlagRequired("repository-id")
	_ = cmd.MarkFlagRequired("blocking")
	_ = cmd.MarkFlagRequired("enabled")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunCaseEnforcementCreate ports create_policy_case_enforcement,
// policy.py:573-585. The settings value is the literal string "true", not a
// JSON boolean - reproduced byte-for-byte since it is not user-configurable
// (there is no flag for it at all).
func policyRunCaseEnforcementCreate(ctx context.Context, cmd *cobra.Command) error {
	repositoryID, _ := cmd.Flags().GetString("repository-id")

	blocking, err := policyTriState(cmd, "blocking")
	if err != nil {
		return err
	}
	enabled, err := policyTriState(cmd, "enabled")
	if err != nil {
		return err
	}

	settings := map[string]any{"enforceConsistentCase": "true"}
	cfg := policyBuildRepoWide(repositoryID, *blocking, *enabled, policyTypeCaseEnforcement, settings)

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	result, err := policyCreate(ctx, dctx, cfg)
	if err != nil {
		return err
	}
	return ado.Print(cmd, result, policyColumns...)
}

func newPolicyCaseEnforcementUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update case enforcement policy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunCaseEnforcementUpdate(context.Background(), cmd)
		},
	}

	policyAddIDFlags(cmd)
	cmd.Flags().String("repository-id", "", "Id of the repository on which to apply the policy")
	policyAddTriStateFlag(cmd, "blocking", "Whether the policy should be blocking or not")
	policyAddTriStateFlag(cmd, "enabled", "Whether the policy is enabled or not")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunCaseEnforcementUpdate ports update_policy_case_enforcement,
// policy.py:588-614.
func policyRunCaseEnforcementUpdate(ctx context.Context, cmd *cobra.Command) error {
	policyID, err := policyIDValue(cmd)
	if err != nil {
		return err
	}
	repositoryID, _ := cmd.Flags().GetString("repository-id")

	blocking, err := policyTriState(cmd, "blocking")
	if err != nil {
		return err
	}
	enabled, err := policyTriState(cmd, "enabled")
	if err != nil {
		return err
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	result, err := policyDoUpdate(ctx, dctx, policyID, func(cur policyCurrent) (policyConfig, error) {
		repo := repositoryID
		if repo == "" {
			repo = cur.scopeStr("repositoryId")
		}
		blk := cur.IsBlocking
		if blocking != nil {
			blk = *blocking
		}
		en := cur.IsEnabled
		if enabled != nil {
			en = *enabled
		}
		settings := map[string]any{"enforceConsistentCase": "true"}
		return policyBuildRepoWide(repo, blk, en, policyTypeCaseEnforcement, settings), nil
	})
	if err != nil {
		return err
	}
	return ado.Print(cmd, result, policyColumns...)
}

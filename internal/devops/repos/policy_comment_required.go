package repos

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// policyNoSettingsCreateCmd builds the create command shared by
// comment-required and work-item-linking, which take no type-specific
// settings (policy.py:481-539) - only scope.
func policyNoSettingsCreateCmd(use, short string, run func(ctx context.Context, cmd *cobra.Command) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(context.Background(), cmd)
		},
	}

	cmd.Flags().String("repository-id", "", "Id of the repository on which to apply the policy")
	cmd.Flags().String("branch", "", "Branch on which this policy should be applied. For example: master")
	cmd.Flags().String("branch-match-type", "exact", "Determines how the branch argument is used to apply a policy.")
	policyAddTriStateFlag(cmd, "blocking", "Whether the policy should be blocking or not")
	policyAddTriStateFlag(cmd, "enabled", "Whether the policy is enabled or not")
	_ = cmd.MarkFlagRequired("repository-id")
	_ = cmd.MarkFlagRequired("branch")
	_ = cmd.MarkFlagRequired("blocking")
	_ = cmd.MarkFlagRequired("enabled")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyNoSettingsUpdateCmd builds the shared update command shape.
func policyNoSettingsUpdateCmd(use, short string, run func(ctx context.Context, cmd *cobra.Command) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(context.Background(), cmd)
		},
	}

	policyAddIDFlags(cmd)
	cmd.Flags().String("repository-id", "", "Id of the repository on which to apply the policy")
	cmd.Flags().String("branch", "", "Branch on which this policy should be applied. For example: master")
	cmd.Flags().String("branch-match-type", "", "Determines how the branch argument is used to apply a policy.")
	policyAddTriStateFlag(cmd, "blocking", "Whether the policy should be blocking or not")
	policyAddTriStateFlag(cmd, "enabled", "Whether the policy is enabled or not")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunNoSettingsCreate is shared by create_policy_comment_required
// (policy.py:481-493) and create_policy_work_item_linking (policy.py:527-539) -
// identical bodies other than the type GUID.
func policyRunNoSettingsCreate(ctx context.Context, cmd *cobra.Command, typeID string) error {
	repositoryID, _ := cmd.Flags().GetString("repository-id")
	branch, _ := cmd.Flags().GetString("branch")
	matchType, _ := cmd.Flags().GetString("branch-match-type")
	matchType, err := policyValidateMatchType(matchType)
	if err != nil {
		return err
	}
	blocking, err := policyTriState(cmd, "blocking")
	if err != nil {
		return err
	}
	enabled, err := policyTriState(cmd, "enabled")
	if err != nil {
		return err
	}

	cfg := policyBuildScoped(repositoryID, branch, matchType, *blocking, *enabled, typeID, map[string]any{})

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

// policyRunNoSettingsUpdate is shared by update_policy_comment_required
// (policy.py:496-524) and update_policy_work_item_linking (policy.py:542-570).
func policyRunNoSettingsUpdate(ctx context.Context, cmd *cobra.Command, typeID string) error {
	policyID, err := policyIDValue(cmd)
	if err != nil {
		return err
	}
	repositoryID, _ := cmd.Flags().GetString("repository-id")
	branch, _ := cmd.Flags().GetString("branch")
	matchType, _ := cmd.Flags().GetString("branch-match-type")
	if matchType != "" {
		if matchType, err = policyValidateMatchType(matchType); err != nil {
			return err
		}
	}
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
		b := branch
		if b == "" {
			b = cur.scopeStr("refName")
		}
		mt := matchType
		if mt == "" {
			mt = cur.scopeStr("matchKind")
		}
		blk := cur.IsBlocking
		if blocking != nil {
			blk = *blocking
		}
		en := cur.IsEnabled
		if enabled != nil {
			en = *enabled
		}
		return policyBuildScoped(repo, b, mt, blk, en, typeID, map[string]any{}), nil
	})
	if err != nil {
		return err
	}
	return ado.Print(cmd, result, policyColumns...)
}

func newPolicyCommentRequiredCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment-required",
		Short: "Manage comment resolution required policies.",
	}
	cmd.AddCommand(policyNoSettingsCreateCmd("create", "Create comment resolution required policy.", func(ctx context.Context, cmd *cobra.Command) error {
		return policyRunNoSettingsCreate(ctx, cmd, policyTypeCommentRequired)
	}))
	cmd.AddCommand(policyNoSettingsUpdateCmd("update", "Update comment resolution required policy.", func(ctx context.Context, cmd *cobra.Command) error {
		return policyRunNoSettingsUpdate(ctx, cmd, policyTypeCommentRequired)
	}))
	return cmd
}

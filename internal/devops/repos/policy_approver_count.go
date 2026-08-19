package repos

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPolicyApproverCountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approver-count",
		Short: "Manage approver count policies.",
	}
	cmd.AddCommand(newPolicyApproverCountCreateCmd())
	cmd.AddCommand(newPolicyApproverCountUpdateCmd())
	return cmd
}

func newPolicyApproverCountCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create approver count policy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunApproverCountCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("repository-id", "", "Id of the repository on which to apply the policy")
	cmd.Flags().String("branch", "", "Branch on which this policy should be applied. For example: master")
	cmd.Flags().String("branch-match-type", "exact", "Determines how the branch argument is used to apply a policy.")
	cmd.Flags().String("minimum-approver-count", "", "Minimum number of approvers required. For example: 2")
	policyAddTriStateFlag(cmd, "blocking", "Whether the policy should be blocking or not")
	policyAddTriStateFlag(cmd, "enabled", "Whether the policy is enabled or not")
	policyAddTriStateFlag(cmd, "creator-vote-counts", "Whether the creator's vote counts or not.")
	policyAddTriStateFlag(cmd, "allow-downvotes", "Whether to allow downvotes or not.")
	policyAddTriStateFlag(cmd, "reset-on-source-push", "Whether to reset source on push.")
	_ = cmd.MarkFlagRequired("repository-id")
	_ = cmd.MarkFlagRequired("branch")
	_ = cmd.MarkFlagRequired("blocking")
	_ = cmd.MarkFlagRequired("enabled")
	_ = cmd.MarkFlagRequired("minimum-approver-count")
	_ = cmd.MarkFlagRequired("creator-vote-counts")
	_ = cmd.MarkFlagRequired("allow-downvotes")
	_ = cmd.MarkFlagRequired("reset-on-source-push")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunApproverCountCreate ports create_policy_approver_count, policy.py:96-111.
func policyRunApproverCountCreate(ctx context.Context, cmd *cobra.Command) error {
	repositoryID, _ := cmd.Flags().GetString("repository-id")
	branch, _ := cmd.Flags().GetString("branch")
	matchType, _ := cmd.Flags().GetString("branch-match-type")
	minApproverCount, _ := cmd.Flags().GetString("minimum-approver-count")

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
	creatorVoteCounts, err := policyTriState(cmd, "creator-vote-counts")
	if err != nil {
		return err
	}
	allowDownvotes, err := policyTriState(cmd, "allow-downvotes")
	if err != nil {
		return err
	}
	resetOnSourcePush, err := policyTriState(cmd, "reset-on-source-push")
	if err != nil {
		return err
	}

	settings := map[string]any{
		"minimumApproverCount": minApproverCount,
		"creatorVoteCounts":    *creatorVoteCounts,
		"allowDownvotes":       *allowDownvotes,
		"resetOnSourcePush":    *resetOnSourcePush,
	}
	cfg := policyBuildScoped(repositoryID, branch, matchType, *blocking, *enabled, policyTypeApproverCount, settings)

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

func newPolicyApproverCountUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update approver count policy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunApproverCountUpdate(context.Background(), cmd)
		},
	}

	policyAddIDFlags(cmd)
	cmd.Flags().String("repository-id", "", "Id of the repository on which to apply the policy")
	cmd.Flags().String("branch", "", "Branch on which this policy should be applied. For example: master")
	cmd.Flags().String("branch-match-type", "", "Determines how the branch argument is used to apply a policy.")
	cmd.Flags().String("minimum-approver-count", "", "Minimum number of approvers required. For example: 2")
	policyAddTriStateFlag(cmd, "blocking", "Whether the policy should be blocking or not")
	policyAddTriStateFlag(cmd, "enabled", "Whether the policy is enabled or not")
	policyAddTriStateFlag(cmd, "creator-vote-counts", "Whether the creator's vote counts or not.")
	policyAddTriStateFlag(cmd, "allow-downvotes", "Whether to allow downvotes or not.")
	policyAddTriStateFlag(cmd, "reset-on-source-push", "Whether to reset source on push.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunApproverCountUpdate ports update_policy_approver_count, policy.py:114-151.
func policyRunApproverCountUpdate(ctx context.Context, cmd *cobra.Command) error {
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
	minApproverCount, _ := cmd.Flags().GetString("minimum-approver-count")
	blocking, err := policyTriState(cmd, "blocking")
	if err != nil {
		return err
	}
	enabled, err := policyTriState(cmd, "enabled")
	if err != nil {
		return err
	}
	creatorVoteCounts, err := policyTriState(cmd, "creator-vote-counts")
	if err != nil {
		return err
	}
	allowDownvotes, err := policyTriState(cmd, "allow-downvotes")
	if err != nil {
		return err
	}
	resetOnSourcePush, err := policyTriState(cmd, "reset-on-source-push")
	if err != nil {
		return err
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	result, err := policyDoUpdate(ctx, dctx, policyID, func(cur policyCurrent) (policyConfig, error) {
		settings := map[string]any{
			"minimumApproverCount": policyPickStr(minApproverCount, cur.Settings["minimumApproverCount"]),
			"creatorVoteCounts":    policyCoalesce(policyBoolIface(creatorVoteCounts), cur.Settings["creatorVoteCounts"]),
			"allowDownvotes":       policyCoalesce(policyBoolIface(allowDownvotes), cur.Settings["allowDownvotes"]),
			"resetOnSourcePush":    policyCoalesce(policyBoolIface(resetOnSourcePush), cur.Settings["resetOnSourcePush"]),
		}
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
		return policyBuildScoped(repo, b, mt, blk, en, policyTypeApproverCount, settings), nil
	})
	if err != nil {
		return err
	}
	return ado.Print(cmd, result, policyColumns...)
}

// policyPickStr matches Python's `v or fallback` idiom for optional string
// settings (build.py, approver-count etc.): an empty flag value means "not
// given", falling back to whatever the server already has.
func policyPickStr(v string, fallback any) any {
	if v != "" {
		return v
	}
	return fallback
}

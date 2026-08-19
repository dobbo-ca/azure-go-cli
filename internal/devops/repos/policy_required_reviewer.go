package repos

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPolicyRequiredReviewerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "required-reviewer",
		Short: "Manage required reviewer policies.",
	}
	cmd.AddCommand(newPolicyRequiredReviewerCreateCmd())
	cmd.AddCommand(newPolicyRequiredReviewerUpdateCmd())
	return cmd
}

func newPolicyRequiredReviewerCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create required reviewer policy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunRequiredReviewerCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("repository-id", "", "Id of the repository on which to apply the policy")
	cmd.Flags().String("branch", "", "Branch on which this policy should be applied. For example: master")
	cmd.Flags().String("branch-match-type", "exact", "Determines how the branch argument is used to apply a policy.")
	cmd.Flags().String("required-reviewer-ids", "", "Required reviewers email addresses separated by ';'. For example: john@contoso.com;alice@contoso.com")
	cmd.Flags().String("message", "", "Message.")
	cmd.Flags().String("path-filter", "", "Filter path(s) on which the policy is applied. Supports absolute paths, wildcards and multiple paths separated by ';'.")
	policyAddTriStateFlag(cmd, "blocking", "Whether the policy should be blocking or not")
	policyAddTriStateFlag(cmd, "enabled", "Whether the policy is enabled or not")
	_ = cmd.MarkFlagRequired("repository-id")
	_ = cmd.MarkFlagRequired("branch")
	_ = cmd.MarkFlagRequired("blocking")
	_ = cmd.MarkFlagRequired("enabled")
	_ = cmd.MarkFlagRequired("required-reviewer-ids")
	// policy.py:154-156: message has no default, so it's required (unlike
	// update, policy.py:178, which defaults it to None).
	_ = cmd.MarkFlagRequired("message")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunRequiredReviewerCreate ports create_policy_required_reviewer,
// policy.py:154-172.
func policyRunRequiredReviewerCreate(ctx context.Context, cmd *cobra.Command) error {
	repositoryID, _ := cmd.Flags().GetString("repository-id")
	branch, _ := cmd.Flags().GetString("branch")
	matchType, _ := cmd.Flags().GetString("branch-match-type")
	matchType, err := policyValidateMatchType(matchType)
	if err != nil {
		return err
	}
	requiredReviewerIDs, _ := cmd.Flags().GetString("required-reviewer-ids")
	message, _ := cmd.Flags().GetString("message")
	pathFilter, _ := cmd.Flags().GetString("path-filter")

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

	// Unlike update, create sends resolveIdentityMailsToIds' result
	// straight through with no `or` fallback (policy.py:163-166): a
	// whitespace-only --required-reviewer-ids resolves to nil here, which
	// is sent as JSON null, matching Python's None.
	reviewerIDs, err := policyResolveReviewerIDs(ctx, dctx.Org, requiredReviewerIDs)
	if err != nil {
		return err
	}

	settings := map[string]any{
		"requiredReviewerIds": reviewerIDs,
		"message":             message,
		"filenamePatterns":    policyFileNamePatterns(pathFilter),
	}
	cfg := policyBuildScoped(repositoryID, branch, matchType, *blocking, *enabled, policyTypeRequiredReviewer, settings)

	result, err := policyCreate(ctx, dctx, cfg)
	if err != nil {
		return err
	}
	return ado.Print(cmd, result, policyColumns...)
}

func newPolicyRequiredReviewerUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update required reviewer policy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunRequiredReviewerUpdate(context.Background(), cmd)
		},
	}

	policyAddIDFlags(cmd)
	cmd.Flags().String("repository-id", "", "Id of the repository on which to apply the policy")
	cmd.Flags().String("branch", "", "Branch on which this policy should be applied. For example: master")
	cmd.Flags().String("branch-match-type", "", "Determines how the branch argument is used to apply a policy.")
	cmd.Flags().String("required-reviewer-ids", "", "Required reviewers email addresses separated by ';'. For example: john@contoso.com;alice@contoso.com")
	cmd.Flags().String("message", "", "Message.")
	cmd.Flags().String("path-filter", "", "Filter path(s) on which the policy is applied.")
	policyAddTriStateFlag(cmd, "blocking", "Whether the policy should be blocking or not")
	policyAddTriStateFlag(cmd, "enabled", "Whether the policy is enabled or not")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunRequiredReviewerUpdate ports update_policy_required_reviewer,
// policy.py:175-214.
func policyRunRequiredReviewerUpdate(ctx context.Context, cmd *cobra.Command) error {
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
	requiredReviewerIDs, _ := cmd.Flags().GetString("required-reviewer-ids")
	message, _ := cmd.Flags().GetString("message")
	pathFilter, _ := cmd.Flags().GetString("path-filter")

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

	// resolveIdentityMailsToIds is called unconditionally in Python
	// (policy.py:188), before the current policy is even fetched; its
	// result is nil for both an omitted flag and a whitespace-only one, in
	// which case the `or` below falls back to the server's current value.
	reviewerIDs, err := policyResolveReviewerIDs(ctx, dctx.Org, requiredReviewerIDs)
	if err != nil {
		return err
	}

	result, err := policyDoUpdate(ctx, dctx, policyID, func(cur policyCurrent) (policyConfig, error) {
		var reviewerSetting any
		if len(reviewerIDs) > 0 {
			reviewerSetting = reviewerIDs
		} else {
			reviewerSetting = cur.Settings["requiredReviewerIds"]
		}
		settings := map[string]any{
			"requiredReviewerIds": reviewerSetting,
			"message":             policyPickStr(message, cur.Settings["message"]),
			"filenamePatterns":    policyPickPatterns(pathFilter, cur.Settings["filenamePatterns"]),
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
		return policyBuildScoped(repo, b, mt, blk, en, policyTypeRequiredReviewer, settings), nil
	})
	if err != nil {
		return err
	}
	return ado.Print(cmd, result, policyColumns...)
}

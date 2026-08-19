package repos

import (
	"context"
	"errors"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPolicyMergeStrategyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge-strategy",
		Short: "Manage merge strategy policies.",
	}
	cmd.AddCommand(newPolicyMergeStrategyCreateCmd())
	cmd.AddCommand(newPolicyMergeStrategyUpdateCmd())
	return cmd
}

func policyAddMergeStrategyFlags(cmd *cobra.Command) {
	cmd.Flags().String("repository-id", "", "Id of the repository on which to apply the policy")
	cmd.Flags().String("branch", "", "Branch on which this policy should be applied. For example: master")
	// arguments.py:66-71: use_squash_merge wraps context.deprecate(
	// redirect='--allow-squash', hide=True) — hidden from help, still
	// functional, warns on use. MarkDeprecated is pflag's exact equivalent.
	policyAddTriStateFlag(cmd, "use-squash-merge", "Deprecated. Use --allow-squash instead.")
	_ = cmd.Flags().MarkDeprecated("use-squash-merge", "use --allow-squash instead")
	policyAddTriStateFlag(cmd, "allow-squash", "Squash merge - Creates a linear history by condensing the source branch commits into a single new commit on the target branch.")
	policyAddTriStateFlag(cmd, "allow-rebase", "Rebase and fast-forward - Creates a linear history by replaying the source branch commits onto the target without a merge commit.")
	policyAddTriStateFlag(cmd, "allow-no-fast-forward", "Basic merge (no fast-forward) - Preserves nonlinear history exactly as it happened during development.")
	policyAddTriStateFlag(cmd, "allow-rebase-merge", "Rebase with merge commit - Creates a semi-linear history by replaying the source branch commits onto the target and then creating a merge commit.")
}

func newPolicyMergeStrategyCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create merge strategy policy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunMergeStrategyCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("branch-match-type", "exact", "Determines how the branch argument is used to apply a policy.")
	policyAddMergeStrategyFlags(cmd)
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

// policyMergeStrategyCreateFlags reads the 5 merge-type tri-states shared by
// create and update.
func policyMergeStrategyCreateFlags(cmd *cobra.Command) (useSquashMerge, allowSquash, allowRebase, allowRebaseMerge, allowNoFastForward *bool, err error) {
	if useSquashMerge, err = policyTriState(cmd, "use-squash-merge"); err != nil {
		return
	}
	if allowSquash, err = policyTriState(cmd, "allow-squash"); err != nil {
		return
	}
	if allowRebase, err = policyTriState(cmd, "allow-rebase"); err != nil {
		return
	}
	if allowRebaseMerge, err = policyTriState(cmd, "allow-rebase-merge"); err != nil {
		return
	}
	if allowNoFastForward, err = policyTriState(cmd, "allow-no-fast-forward"); err != nil {
		return
	}
	return
}

// policyRunMergeStrategyCreate ports create_policy_merge_strategy, policy.py:222-253.
func policyRunMergeStrategyCreate(ctx context.Context, cmd *cobra.Command) error {
	repositoryID, _ := cmd.Flags().GetString("repository-id")
	branch, _ := cmd.Flags().GetString("branch")
	matchType, _ := cmd.Flags().GetString("branch-match-type")
	matchType, err := policyValidateMatchType(matchType)
	if err != nil {
		return err
	}

	useSquashMerge, allowSquash, allowRebase, allowRebaseMerge, allowNoFastForward, err := policyMergeStrategyCreateFlags(cmd)
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

	if useSquashMerge == nil && !policyFalseIfNil(policyBoolIface(allowSquash)) && !policyFalseIfNil(policyBoolIface(allowRebaseMerge)) &&
		!policyFalseIfNil(policyBoolIface(allowRebase)) && !policyFalseIfNil(policyBoolIface(allowNoFastForward)) {
		return errors.New("Atleast one merge type must be enabled.")
	}

	settings := map[string]any{}
	if useSquashMerge == nil {
		settings["allowSquash"] = policyFalseIfNil(policyBoolIface(allowSquash))
		settings["allowRebase"] = policyFalseIfNil(policyBoolIface(allowRebase))
		settings["allowRebaseMerge"] = policyFalseIfNil(policyBoolIface(allowRebaseMerge))
		settings["allowNoFastForward"] = policyFalseIfNil(policyBoolIface(allowNoFastForward))
	} else {
		if policyFalseIfNil(policyBoolIface(allowRebase)) || policyFalseIfNil(policyBoolIface(allowNoFastForward)) ||
			policyFalseIfNil(policyBoolIface(allowRebaseMerge)) || policyFalseIfNil(policyBoolIface(allowSquash)) {
			return errors.New(policyMergeDeprecatedError)
		}
		settings["useSquashMerge"] = *useSquashMerge
	}

	cfg := policyBuildScoped(repositoryID, branch, matchType, *blocking, *enabled, policyTypeMergeStrategy, settings)

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

func newPolicyMergeStrategyUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update merge strategy policy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunMergeStrategyUpdate(context.Background(), cmd)
		},
	}

	policyAddIDFlags(cmd)
	cmd.Flags().String("branch-match-type", "", "Determines how the branch argument is used to apply a policy.")
	policyAddMergeStrategyFlags(cmd)
	policyAddTriStateFlag(cmd, "blocking", "Whether the policy should be blocking or not")
	policyAddTriStateFlag(cmd, "enabled", "Whether the policy is enabled or not")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunMergeStrategyUpdate ports update_policy_merge_strategy,
// policy.py:256-343 - the hardest state machine in this surface. Read it
// alongside the Python before touching this function.
func policyRunMergeStrategyUpdate(ctx context.Context, cmd *cobra.Command) error {
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
	useSquashMerge, allowSquash, allowRebase, allowRebaseMerge, allowNoFastForward, err := policyMergeStrategyCreateFlags(cmd)
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

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	result, err := policyDoUpdate(ctx, dctx, policyID, func(cur policyCurrent) (policyConfig, error) {
		isNewMergeTypeUpdate := allowSquash != nil || allowNoFastForward != nil || allowRebase != nil || allowRebaseMerge != nil

		var settings map[string]any
		switch {
		case isNewMergeTypeUpdate:
			if useSquashMerge != nil {
				return policyConfig{}, errors.New(policyMergeDeprecatedError)
			}
			allowSquashVal := policyCoalesce(policyBoolIface(allowSquash), cur.Settings["allowSquash"], cur.Settings["useSquashMerge"])
			allowRebaseVal := policyCoalesce(policyBoolIface(allowRebase), cur.Settings["allowRebase"])
			allowRebaseMergeVal := policyCoalesce(policyBoolIface(allowRebaseMerge), cur.Settings["allowRebaseMerge"])
			allowNoFastForwardVal := policyCoalesce(policyBoolIface(allowNoFastForward), cur.Settings["allowNoFastForward"])

			settings = map[string]any{
				"allowSquash":        policyFalseIfNil(allowSquashVal),
				"allowRebase":        policyFalseIfNil(allowRebaseVal),
				"allowRebaseMerge":   policyFalseIfNil(allowRebaseMergeVal),
				"allowNoFastForward": policyFalseIfNil(allowNoFastForwardVal),
			}
			if !settings["allowSquash"].(bool) && !settings["allowRebase"].(bool) &&
				!settings["allowRebaseMerge"].(bool) && !settings["allowNoFastForward"].(bool) {
				return policyConfig{}, errors.New("Atleast one merge type must be enabled.")
			}

		case useSquashMerge != nil:
			if cur.Settings["allowSquash"] != nil || cur.Settings["allowRebase"] != nil ||
				cur.Settings["allowRebaseMerge"] != nil || cur.Settings["allowNoFastForward"] != nil {
				return policyConfig{}, errors.New(policyMergeDeprecatedError)
			}
			settings = map[string]any{"useSquashMerge": *useSquashMerge}

		default:
			if cur.Settings["allowSquash"] != nil || cur.Settings["allowRebase"] != nil ||
				cur.Settings["allowRebaseMerge"] != nil || cur.Settings["allowNoFastForward"] != nil {
				settings = map[string]any{
					"allowSquash":        policyFalseIfNil(cur.Settings["allowSquash"]),
					"allowRebase":        policyFalseIfNil(cur.Settings["allowRebase"]),
					"allowRebaseMerge":   policyFalseIfNil(cur.Settings["allowRebaseMerge"]),
					"allowNoFastForward": policyFalseIfNil(cur.Settings["allowNoFastForward"]),
				}
			} else {
				v := cur.Settings["useSquashMerge"]
				if v == nil {
					v = false
				}
				settings = map[string]any{"useSquashMerge": v}
			}
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
		return policyBuildScoped(repo, b, mt, blk, en, policyTypeMergeStrategy, settings), nil
	})
	if err != nil {
		return err
	}
	return ado.Print(cmd, result, policyColumns...)
}

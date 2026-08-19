package repos

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPolicyBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Manage build policies.",
	}
	cmd.AddCommand(newPolicyBuildCreateCmd())
	cmd.AddCommand(newPolicyBuildUpdateCmd())
	return cmd
}

func newPolicyBuildCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create build policy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunBuildCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("repository-id", "", "Id of the repository on which to apply the policy")
	cmd.Flags().String("branch", "", "Branch on which this policy should be applied. For example: master")
	cmd.Flags().String("branch-match-type", "exact", "Determines how the branch argument is used to apply a policy.")
	cmd.Flags().String("build-definition-id", "", "Build Definition Id.")
	cmd.Flags().String("display-name", "", "Display name for this build policy to identify the policy. For example: 'Manual queue policy'")
	cmd.Flags().String("valid-duration", "", "Policy validity duration (in minutes).")
	cmd.Flags().String("path-filter", "", "Filter path(s) on which the policy is applied. Supports absolute paths, wildcards and multiple paths separated by ';'.")
	policyAddTriStateFlag(cmd, "blocking", "Whether the policy should be blocking or not")
	policyAddTriStateFlag(cmd, "enabled", "Whether the policy is enabled or not")
	policyAddTriStateFlag(cmd, "queue-on-source-update-only", "Queue Only on source update.")
	policyAddTriStateFlag(cmd, "manual-queue-only", "Whether to allow only manual queue of builds.")
	_ = cmd.MarkFlagRequired("repository-id")
	_ = cmd.MarkFlagRequired("branch")
	_ = cmd.MarkFlagRequired("blocking")
	_ = cmd.MarkFlagRequired("enabled")
	_ = cmd.MarkFlagRequired("build-definition-id")
	_ = cmd.MarkFlagRequired("queue-on-source-update-only")
	_ = cmd.MarkFlagRequired("manual-queue-only")
	_ = cmd.MarkFlagRequired("display-name")
	_ = cmd.MarkFlagRequired("valid-duration")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunBuildCreate ports create_policy_build, policy.py:346-375.
func policyRunBuildCreate(ctx context.Context, cmd *cobra.Command) error {
	repositoryID, _ := cmd.Flags().GetString("repository-id")
	branch, _ := cmd.Flags().GetString("branch")
	matchType, _ := cmd.Flags().GetString("branch-match-type")
	matchType, err := policyValidateMatchType(matchType)
	if err != nil {
		return err
	}
	buildDefinitionID, _ := cmd.Flags().GetString("build-definition-id")
	displayName, _ := cmd.Flags().GetString("display-name")
	validDuration, _ := cmd.Flags().GetString("valid-duration")
	pathFilter, _ := cmd.Flags().GetString("path-filter")

	blocking, err := policyTriState(cmd, "blocking")
	if err != nil {
		return err
	}
	enabled, err := policyTriState(cmd, "enabled")
	if err != nil {
		return err
	}
	queueOnSourceUpdateOnly, err := policyTriState(cmd, "queue-on-source-update-only")
	if err != nil {
		return err
	}
	manualQueueOnly, err := policyTriState(cmd, "manual-queue-only")
	if err != nil {
		return err
	}

	settings := map[string]any{
		"buildDefinitionId":       buildDefinitionID,
		"queueOnSourceUpdateOnly": *queueOnSourceUpdateOnly,
		"manualQueueOnly":         *manualQueueOnly,
		"displayName":             displayName,
		"validDuration":           validDuration,
		"filenamePatterns":        policyFileNamePatterns(pathFilter),
	}
	cfg := policyBuildScoped(repositoryID, branch, matchType, *blocking, *enabled, policyTypeBuild, settings)

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

func newPolicyBuildUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update build policy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunBuildUpdate(context.Background(), cmd)
		},
	}

	policyAddIDFlags(cmd)
	cmd.Flags().String("repository-id", "", "Id of the repository on which to apply the policy")
	cmd.Flags().String("branch", "", "Branch on which this policy should be applied. For example: master")
	cmd.Flags().String("branch-match-type", "", "Determines how the branch argument is used to apply a policy.")
	cmd.Flags().String("build-definition-id", "", "Build Definition Id.")
	cmd.Flags().String("display-name", "", "Display name for this build policy to identify the policy. For example: 'Manual queue policy'")
	cmd.Flags().String("valid-duration", "", "Policy validity duration (in minutes).")
	cmd.Flags().String("path-filter", "", "Filter path(s) on which the policy is applied. Supports absolute paths, wildcards and multiple paths separated by ';'.")
	policyAddTriStateFlag(cmd, "blocking", "Whether the policy should be blocking or not")
	policyAddTriStateFlag(cmd, "enabled", "Whether the policy is enabled or not")
	policyAddTriStateFlag(cmd, "queue-on-source-update-only", "Queue Only on source update.")
	policyAddTriStateFlag(cmd, "manual-queue-only", "Whether to allow only manual queue of builds.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunBuildUpdate ports update_policy_build, policy.py:378-424.
func policyRunBuildUpdate(ctx context.Context, cmd *cobra.Command) error {
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
	buildDefinitionID, _ := cmd.Flags().GetString("build-definition-id")
	displayName, _ := cmd.Flags().GetString("display-name")
	validDuration, _ := cmd.Flags().GetString("valid-duration")
	pathFilter, _ := cmd.Flags().GetString("path-filter")

	blocking, err := policyTriState(cmd, "blocking")
	if err != nil {
		return err
	}
	enabled, err := policyTriState(cmd, "enabled")
	if err != nil {
		return err
	}
	queueOnSourceUpdateOnly, err := policyTriState(cmd, "queue-on-source-update-only")
	if err != nil {
		return err
	}
	manualQueueOnly, err := policyTriState(cmd, "manual-queue-only")
	if err != nil {
		return err
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	result, err := policyDoUpdate(ctx, dctx, policyID, func(cur policyCurrent) (policyConfig, error) {
		settings := map[string]any{
			"buildDefinitionId":       policyPickStr(buildDefinitionID, cur.Settings["buildDefinitionId"]),
			"queueOnSourceUpdateOnly": policyCoalesce(policyBoolIface(queueOnSourceUpdateOnly), cur.Settings["queueOnSourceUpdateOnly"]),
			"manualQueueOnly":         policyCoalesce(policyBoolIface(manualQueueOnly), cur.Settings["manualQueueOnly"]),
			"displayName":             policyPickStr(displayName, cur.Settings["displayName"]),
			"validDuration":           policyPickStr(validDuration, cur.Settings["validDuration"]),
			"filenamePatterns":        policyPickPatterns(pathFilter, cur.Settings["filenamePatterns"]),
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
		return policyBuildScoped(repo, b, mt, blk, en, policyTypeBuild, settings), nil
	})
	if err != nil {
		return err
	}
	return ado.Print(cmd, result, policyColumns...)
}

// policyPickPatterns matches `createFileNamePatterns(path_filter) or
// current_setting.get('filenamePatterns', None)` (policy.py:406): an omitted
// --path-filter yields the falsy empty list, which "or" turns into a
// fall-back to whatever the server already has.
func policyPickPatterns(pathFilter string, fallback any) any {
	if pathFilter != "" {
		return policyFileNamePatterns(pathFilter)
	}
	return fallback
}

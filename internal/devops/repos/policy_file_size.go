package repos

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newPolicyFileSizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file-size",
		Short: "Manage file size policies.",
	}
	cmd.AddCommand(newPolicyFileSizeCreateCmd())
	cmd.AddCommand(newPolicyFileSizeUpdateCmd())
	return cmd
}

func newPolicyFileSizeCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create file size policy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunFileSizeCreate(context.Background(), cmd)
		},
	}

	// No --branch/--branch-match-type at all: file-size scope is always
	// repo-wide, policy.py:437-438.
	cmd.Flags().String("repository-id", "", "Id of the repository on which to apply the policy")
	cmd.Flags().String("maximum-git-blob-size", "", "Maximum git blob size in bytes. For example, to specify a 10byte limit, --maximum-git-blob-size 10.")
	policyAddTriStateFlag(cmd, "blocking", "Whether the policy should be blocking or not")
	policyAddTriStateFlag(cmd, "enabled", "Whether the policy is enabled or not")
	policyAddTriStateFlag(cmd, "use-uncompressed-size", "Whether to use uncompressed size.")
	_ = cmd.MarkFlagRequired("repository-id")
	_ = cmd.MarkFlagRequired("blocking")
	_ = cmd.MarkFlagRequired("enabled")
	_ = cmd.MarkFlagRequired("maximum-git-blob-size")
	_ = cmd.MarkFlagRequired("use-uncompressed-size")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunFileSizeCreate ports create_policy_file_size, policy.py:427-441.
func policyRunFileSizeCreate(ctx context.Context, cmd *cobra.Command) error {
	repositoryID, _ := cmd.Flags().GetString("repository-id")
	maxBlobSize, _ := cmd.Flags().GetString("maximum-git-blob-size")

	blocking, err := policyTriState(cmd, "blocking")
	if err != nil {
		return err
	}
	enabled, err := policyTriState(cmd, "enabled")
	if err != nil {
		return err
	}
	useUncompressedSize, err := policyTriState(cmd, "use-uncompressed-size")
	if err != nil {
		return err
	}

	settings := map[string]any{
		"maximumGitBlobSizeInBytes": maxBlobSize,
		"useUncompressedSize":       *useUncompressedSize,
	}
	cfg := policyBuildRepoWide(repositoryID, *blocking, *enabled, policyTypeFileSize, settings)

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

func newPolicyFileSizeUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update file size policy.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyRunFileSizeUpdate(context.Background(), cmd)
		},
	}

	policyAddIDFlags(cmd)
	cmd.Flags().String("repository-id", "", "Id of the repository on which to apply the policy")
	cmd.Flags().String("maximum-git-blob-size", "", "Maximum git blob size in bytes.")
	policyAddTriStateFlag(cmd, "blocking", "Whether the policy should be blocking or not")
	policyAddTriStateFlag(cmd, "enabled", "Whether the policy is enabled or not")
	policyAddTriStateFlag(cmd, "use-uncompressed-size", "Whether to use uncompressed size.")
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// policyRunFileSizeUpdate ports update_policy_file_size, policy.py:444-478.
func policyRunFileSizeUpdate(ctx context.Context, cmd *cobra.Command) error {
	policyID, err := policyIDValue(cmd)
	if err != nil {
		return err
	}
	repositoryID, _ := cmd.Flags().GetString("repository-id")
	maxBlobSize, _ := cmd.Flags().GetString("maximum-git-blob-size")

	blocking, err := policyTriState(cmd, "blocking")
	if err != nil {
		return err
	}
	enabled, err := policyTriState(cmd, "enabled")
	if err != nil {
		return err
	}
	useUncompressedSize, err := policyTriState(cmd, "use-uncompressed-size")
	if err != nil {
		return err
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	result, err := policyDoUpdate(ctx, dctx, policyID, func(cur policyCurrent) (policyConfig, error) {
		settings := map[string]any{
			"maximumGitBlobSizeInBytes": policyPickStr(maxBlobSize, cur.Settings["maximumGitBlobSizeInBytes"]),
			"useUncompressedSize":       policyCoalesce(policyBoolIface(useUncompressedSize), cur.Settings["useUncompressedSize"]),
		}
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
		return policyBuildRepoWide(repo, blk, en, policyTypeFileSize, settings), nil
	})
	if err != nil {
		return err
	}
	return ado.Print(cmd, result, policyColumns...)
}

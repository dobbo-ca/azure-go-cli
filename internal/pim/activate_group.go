package pim

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	pimvendor "github.com/cdobbyn/azure-go-cli/internal/pim/vendor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

type activateGroupArgs struct {
	Name          string
	Justification string
	Duration      int
	NoInput       bool
	Output        string
}

func newActivateGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Activate an eligible Entra ID group membership",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := activateGroupArgs{}
			a.Name, _ = cmd.Flags().GetString("name")
			a.Justification, _ = cmd.Flags().GetString("justification")
			a.Duration, _ = cmd.Flags().GetInt("duration")
			a.NoInput, _ = cmd.Flags().GetBool("no-input")
			a.Output, _ = cmd.Flags().GetString("output")
			return runActivateGroup(a, cmd.OutOrStdout())
		},
	}
	cmd.Flags().String("name", "", "group display name")
	cmd.Flags().String("justification", "", "reason for activation")
	cmd.Flags().Int("duration", 0, "activation duration in minutes")
	cmd.Flags().Bool("no-input", false, "disable interactive prompts")
	cmd.Flags().String("output", "json", "output format: json or table")
	return cmd
}

func validateActivateGroupArgs(a activateGroupArgs, noInput bool) error {
	missing := []string{}
	if a.Name == "" {
		missing = append(missing, "--name")
	}
	if a.Justification == "" {
		missing = append(missing, "--justification")
	}
	if a.Duration <= 0 {
		missing = append(missing, "--duration")
	}
	if len(missing) == 0 {
		return nil
	}
	if noInput {
		return fmt.Errorf("%w: %s", errMissingFlag, strings.Join(missing, ", "))
	}
	return nil
}

func runActivateGroup(a activateGroupArgs, w io.Writer) error {
	prompter := NewPrompter(a.NoInput)
	if a.Name == "" {
		v, err := prompter.PromptString("Group name")
		if err != nil {
			return err
		}
		a.Name = v
	}
	if a.Justification == "" {
		v, err := prompter.PromptString("Justification")
		if err != nil {
			return err
		}
		a.Justification = v
	}
	if a.Duration <= 0 {
		v, err := prompter.PromptString("Duration (minutes)")
		if err != nil {
			return err
		}
		d, perr := strconv.Atoi(strings.TrimSpace(v))
		if perr != nil || d <= 0 {
			return fmt.Errorf("invalid duration %q", v)
		}
		a.Duration = d
	}
	if err := validateActivateGroupArgs(a, true); err != nil {
		return err
	}

	cred, err := azure.GetCredential()
	if err != nil {
		return fmt.Errorf("get credential: %w", err)
	}
	ts := NewTokenSource(cred)
	graphToken, err := ts.GetAccessToken("https://graph.microsoft.com/.default")
	if err != nil {
		return err
	}

	client := pimvendor.AzureClient{ARMBaseURL: pimvendor.ARM_GLOBAL_BASE_URL}
	info, err := pimvendor.GetUserInfo(graphToken)
	if err != nil {
		return err
	}

	eligible, err := client.GetEligibleGovernanceRoleAssignments(pimvendor.ROLE_TYPE_AAD_GROUPS, info.ObjectId, graphToken)
	if err != nil {
		return err
	}

	matches := []pimvendor.GovernanceRoleAssignment{}
	for _, e := range eligible.Value {
		if e.RoleDefinition != nil && e.RoleDefinition.DisplayName == a.Name {
			matches = append(matches, e)
		}
	}
	switch len(matches) {
	case 0:
		return fmt.Errorf("no eligible group named %q", a.Name)
	case 1:
		// proceed
	default:
		var ids []string
		for _, m := range matches {
			if m.RoleDefinition != nil && m.RoleDefinition.Resource != nil {
				ids = append(ids, m.RoleDefinition.Resource.Id)
			}
		}
		return fmt.Errorf("ambiguous group name %q; candidates by id: %s", a.Name, strings.Join(ids, ", "))
	}

	roleType, req, err := pimvendor.CreateGovernanceRoleAssignmentRequest(
		info.ObjectId, pimvendor.ROLE_TYPE_AAD_GROUPS, &matches[0],
		a.Duration, "", "", a.Justification, "", "")
	if err != nil {
		return err
	}

	ok, err := client.ValidateGovernanceRoleAssignmentRequest(roleType, req, graphToken)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Azure rejected the group activation request during validation; check group name and duration")
	}

	resp, err := client.RequestGovernanceRoleAssignment(roleType, req, graphToken)
	if err != nil {
		return err
	}

	subStatus := ""
	if resp.Status != nil {
		subStatus = resp.Status.SubStatus
	}
	return renderActivationResult(w, a.Output, subStatus, "—", a.Name, resp.RoleAssignmentEndDateTime, resp.Id)
}

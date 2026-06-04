package pim

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	pimvendor "github.com/cdobbyn/azure-go-cli/internal/pim/vendor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

var errMissingFlag = errors.New("missing required flag")

type activateResourceArgs struct {
	Role            string
	Scope           string
	Ticket          string
	Justification   string
	Duration        int
	SetSubscription bool
	NoInput         bool
	Output          string
}

func newActivateResourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Activate an eligible Azure resource role assignment",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := activateResourceArgs{}
			a.Role, _ = cmd.Flags().GetString("role")
			a.Scope, _ = cmd.Flags().GetString("scope")
			a.Ticket, _ = cmd.Flags().GetString("ticket")
			a.Justification, _ = cmd.Flags().GetString("justification")
			a.Duration, _ = cmd.Flags().GetInt("duration")
			a.SetSubscription, _ = cmd.Flags().GetBool("set-subscription")
			a.NoInput, _ = cmd.Flags().GetBool("no-input")
			a.Output, _ = cmd.Flags().GetString("output")
			return runActivateResource(a, cmd.OutOrStdout())
		},
	}
	cmd.Flags().String("role", "", "role display name (e.g. Contributor)")
	cmd.Flags().String("scope", "", "subscription scope: ARM path, UUID, tenant/sub, or sub name")
	cmd.Flags().String("ticket", "", "ticket reference: SYSTEM:NUMBER (e.g. Jira:TEC-1234)")
	cmd.Flags().String("justification", "", "reason for activation")
	cmd.Flags().Int("duration", 0, "activation duration in minutes")
	cmd.Flags().Bool("set-subscription", false, "set the activated subscription as the default after activation")
	cmd.Flags().Bool("no-input", false, "disable interactive prompts; missing required flags become errors")
	cmd.Flags().String("output", "json", "output format: json or table")
	return cmd
}

func validateActivateResourceArgs(a activateResourceArgs, noInput bool) error {
	missing := []string{}
	if a.Role == "" {
		missing = append(missing, "--role")
	}
	if a.Ticket == "" {
		missing = append(missing, "--ticket")
	}
	if a.Justification == "" {
		missing = append(missing, "--justification")
	}
	if a.Duration <= 0 {
		missing = append(missing, "--duration")
	}
	// --scope is optional; resolveResourceScope handles unambiguous omission.
	if len(missing) == 0 {
		return nil
	}
	if noInput {
		return fmt.Errorf("%w: %s", errMissingFlag, strings.Join(missing, ", "))
	}
	// In interactive mode, missing flags get prompted later — validator passes.
	return nil
}

func runActivateResource(a activateResourceArgs, w io.Writer) error {
	prompter := NewPrompter(a.NoInput)
	if err := promptForMissingResourceArgs(prompter, &a); err != nil {
		return err
	}
	if err := validateActivateResourceArgs(a, true /* enforce */); err != nil {
		return err
	}

	cred, err := azure.GetCredential()
	if err != nil {
		return fmt.Errorf("get credential: %w", err)
	}
	ts := NewTokenSource(cred)
	token, err := ts.GetAccessToken("https://management.azure.com/.default")
	if err != nil {
		return err
	}

	client := pimvendor.AzureClient{ARMBaseURL: pimvendor.ARM_GLOBAL_BASE_URL}

	eligible, err := client.GetEligibleResourceAssignments(token)
	if err != nil {
		return err
	}

	profile, _ := config.Load()
	index := buildScopeIndex(eligible, profile)
	scope, err := resolveResourceScope(a, eligible, index)
	if err != nil {
		return err
	}

	assignment, err := findAssignment(eligible, a.Role, scope)
	if err != nil {
		return err
	}

	info, err := pimvendor.GetUserInfo(token)
	if err != nil {
		return err
	}

	system, number := ParseTicket(a.Ticket)
	// vendorScope is the resolver's scope minus the leading "/" — the form the
	// vendor's URL builder expects (sprintf inserts its own slash before it).
	vendorScope, req, err := pimvendor.CreateResourceAssignmentRequest(info.ObjectId, &assignment, a.Duration, "", "", a.Justification, system, number)
	if err != nil {
		return err
	}

	// Validate first. The vendor's validate path mutates
	// Properties.IsValidationOnly via the aliased pointer, which would
	// otherwise poison the subsequent activation request. Properties is a
	// struct value, so a one-level copy of req is enough to isolate it.
	validationReq := *req
	ok, err := client.ValidateResourceAssignmentRequest(vendorScope, &validationReq, token)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Azure rejected the activation request during validation; check role, scope, ticket, and duration")
	}

	resp, err := client.RequestResourceAssignment(vendorScope, req, token)
	if err != nil {
		return err
	}

	if a.SetSubscription {
		subID := extractSubscriptionID(scope)
		if subID != "" && profile != nil {
			if err := SetDefaultSubscription(profile, subID); err != nil {
				return err
			}
			if err := config.Save(profile); err != nil {
				return err
			}
		}
	}

	status := ""
	if resp.Properties != nil {
		status = resp.Properties.Status
	}
	return renderActivationResult(w, a.Output, status, scope, a.Role, "", resp.Id)
}

func promptForMissingResourceArgs(p *Prompter, a *activateResourceArgs) error {
	if a.Role == "" {
		v, err := p.PromptString("Role")
		if err != nil {
			return err
		}
		a.Role = v
	}
	if a.Ticket == "" {
		v, err := p.PromptString("Ticket (SYSTEM:NUMBER)")
		if err != nil {
			return err
		}
		a.Ticket = v
	}
	if a.Justification == "" {
		v, err := p.PromptString("Justification")
		if err != nil {
			return err
		}
		a.Justification = v
	}
	if a.Duration <= 0 {
		v, err := p.PromptString("Duration (minutes)")
		if err != nil {
			return err
		}
		d, perr := strconv.Atoi(strings.TrimSpace(v))
		if perr != nil || d <= 0 {
			return fmt.Errorf("invalid duration %q", v)
		}
		a.Duration = d
	}
	return nil
}

func resolveResourceScope(a activateResourceArgs, eligible *pimvendor.ResourceAssignmentResponse, index []ScopeIndexEntry) (string, error) {
	if a.Scope != "" {
		return ResolveScope(a.Scope, index)
	}
	// Unambiguous match by role only.
	var candidates []pimvendor.ResourceAssignment
	for _, e := range eligible.Value {
		if e.Properties == nil || e.Properties.ExpandedProperties == nil ||
			e.Properties.ExpandedProperties.RoleDefinition == nil {
			continue
		}
		if e.Properties.ExpandedProperties.RoleDefinition.DisplayName == a.Role {
			candidates = append(candidates, e)
		}
	}
	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("no eligible assignment for role %q", a.Role)
	case 1:
		return candidates[0].Properties.Scope, nil
	default:
		var names []string
		for _, c := range candidates {
			names = append(names, c.Properties.Scope)
		}
		return "", fmt.Errorf("ambiguous: role %q is eligible on multiple scopes; pass --scope; candidates: %s",
			a.Role, strings.Join(names, ", "))
	}
}

func findAssignment(eligible *pimvendor.ResourceAssignmentResponse, role, scope string) (pimvendor.ResourceAssignment, error) {
	for _, e := range eligible.Value {
		if e.Properties == nil || e.Properties.Scope != scope {
			continue
		}
		if e.Properties.ExpandedProperties == nil ||
			e.Properties.ExpandedProperties.RoleDefinition == nil {
			continue
		}
		if e.Properties.ExpandedProperties.RoleDefinition.DisplayName == role {
			return e, nil
		}
	}
	return pimvendor.ResourceAssignment{}, fmt.Errorf("no eligible assignment for role %q at scope %s", role, scope)
}

func buildScopeIndex(eligible *pimvendor.ResourceAssignmentResponse, p *config.Profile) []ScopeIndexEntry {
	var index []ScopeIndexEntry
	for _, e := range eligible.Value {
		if e.Properties == nil {
			continue
		}
		armPath := e.Properties.Scope
		subID := extractSubscriptionID(armPath)
		entry := ScopeIndexEntry{
			ArmPath:        armPath,
			SubscriptionID: subID,
		}
		if e.Properties.ExpandedProperties != nil && e.Properties.ExpandedProperties.Scope != nil {
			entry.SubscriptionName = e.Properties.ExpandedProperties.Scope.DisplayName
		}
		if p != nil {
			for _, s := range p.Subscriptions {
				if s.ID == subID {
					entry.TenantID = s.TenantID
					entry.TenantDisplayName = s.TenantID // best-effort; profile has no display name
				}
			}
		}
		index = append(index, entry)
	}
	return index
}

func renderActivationResult(w io.Writer, outFmt, status, scope, role, expires, requestID string) error {
	switch strings.ToLower(outFmt) {
	case "json", "":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]string{
			"status":    status,
			"scope":     scope,
			"role":      role,
			"expiresAt": expires,
			"requestId": requestID,
		})
	case "table":
		if strings.HasPrefix(strings.ToLower(status), "pending") {
			fmt.Fprintf(w, "Pending approval; request %s\n", requestID)
			return nil
		}
		if expires == "" {
			fmt.Fprintf(w, "Activated %s on %s\n", role, scope)
		} else {
			fmt.Fprintf(w, "Activated %s on %s; expires %s\n", role, scope, expires)
		}
		return nil
	default:
		return fmt.Errorf("unknown --output %q", outFmt)
	}
}

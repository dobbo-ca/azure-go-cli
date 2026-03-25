package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/manifoldco/promptui"
)

const subscriptionThreshold = 10

func Login(ctx context.Context, forceTenantSelection bool) error {
	// Clear any existing profile to ensure fresh login
	// This prevents old subscription data from persisting if the new login fails
	if err := config.Delete(); err != nil {
		// Ignore errors if profile doesn't exist
		_ = err
	}

	logger.Println("A web browser has been opened at https://login.microsoftonline.com/organizations/oauth2/v2.0/authorize.")
	logger.Println("Please continue the login in the web browser.")
	logger.Println("If no web browser is available or if the web browser fails to open, use device code flow with `az login --use-device-code`.")
	logger.Println("")

	// Use interactive browser credential (matches official Azure CLI behavior)
	cred, err := azure.GetInteractiveBrowserCredentialWithCache()
	if err != nil {
		return fmt.Errorf("failed to create credential: %w", err)
	}

	// Discover subscriptions across all tenants
	// This will:
	// 1. Trigger interactive browser authentication (ONE TIME)
	// 2. List all tenants (home + guest)
	// 3. For each tenant, get subscriptions silently using cached tokens
	logger.Info("Retrieving tenants and subscriptions for the selection...")
	tenantInfos, authRecord, err := azure.DiscoverAllSubscriptionsWithAuth(ctx, cred)
	if err != nil {
		return fmt.Errorf("failed to discover subscriptions: %w", err)
	}

	// Separate tenants with subscriptions from MFA-blocked tenants
	var activeTenants []azure.TenantInfo
	var mfaTenants []azure.TenantInfo
	for _, t := range tenantInfos {
		if t.NeedsMFA {
			mfaTenants = append(mfaTenants, t)
		} else if len(t.Subscriptions) > 0 {
			activeTenants = append(activeTenants, t)
		}
	}

	if len(activeTenants) == 0 && len(mfaTenants) == 0 {
		return fmt.Errorf("no tenants with subscriptions found")
	}

	// Get all subscriptions for counting
	allSubscriptions := azure.GetAllSubscriptions(activeTenants)
	if len(allSubscriptions) == 0 && len(mfaTenants) == 0 {
		return fmt.Errorf("no subscriptions found")
	}

	var selectedSub *config.Subscription

	// Decide whether to use two-step or flat selection
	useTenantSelection := forceTenantSelection || len(allSubscriptions) > subscriptionThreshold

	if useTenantSelection {
		// Two-step: tenant first, then subscription
		logger.Debug("Using two-step selection (tenant then subscription)")
		selectedTenant, err := promptForTenant(tenantInfos)
		if err != nil {
			return fmt.Errorf("failed to select tenant: %w", err)
		}

		// If the selected tenant needs MFA, authenticate interactively
		if selectedTenant.NeedsMFA {
			selectedTenant, err = authenticateMFATenant(ctx, selectedTenant)
			if err != nil {
				return fmt.Errorf("failed to authenticate for tenant '%s': %w", selectedTenant.DisplayName, err)
			}
			// Add newly discovered subscriptions to allSubscriptions
			allSubscriptions = append(allSubscriptions, selectedTenant.Subscriptions...)
		}

		selectedSub, err = promptForSubscriptionInTenant(selectedTenant)
		if err != nil {
			return fmt.Errorf("failed to select subscription: %w", err)
		}
	} else {
		// Single-step: flat list of all subscriptions + MFA tenants
		logger.Debug("Using flat selection (%d subscriptions <= threshold of %d)", len(allSubscriptions), subscriptionThreshold)
		selectedSub, err = promptForSubscriptionFlatWithMFA(ctx, activeTenants, mfaTenants)
		if err != nil {
			return fmt.Errorf("failed to select subscription: %w", err)
		}
		// Refresh allSubscriptions in case MFA tenant was selected
		allSubscriptions = azure.GetAllSubscriptions(activeTenants)
		// Ensure the selected sub is in the list
		found := false
		for _, s := range allSubscriptions {
			if s.ID == selectedSub.ID {
				found = true
				break
			}
		}
		if !found {
			allSubscriptions = append(allSubscriptions, *selectedSub)
		}
	}

	// Mark the selected subscription as default
	for i := range allSubscriptions {
		allSubscriptions[i].IsDefault = (allSubscriptions[i].ID == selectedSub.ID)
	}

	// Save profile with authentication record and subscriptions
	profile := config.Profile{
		Subscriptions:        allSubscriptions,
		AuthenticationRecord: &authRecord,
	}

	if err := config.Save(&profile); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	logger.Println("")
	logger.Println("Tenant: %s", selectedSub.TenantID)
	logger.Println("Subscription: %s (%s)", selectedSub.Name, selectedSub.ID)
	logger.Println("")
	logger.Info("You have successfully logged in")

	return nil
}

func promptForTenant(tenantInfos []azure.TenantInfo) (*azure.TenantInfo, error) {
	logger.Println("")
	logger.Println("[Tenant selection]")
	logger.Println("")

	templates := &promptui.SelectTemplates{
		Label: "{{ . }}",
		Active: `{{ if .NeedsMFA }}` + "\U0001F449 \U0001F510 {{ .DisplayName | yellow }} (requires sign-in)" + `{{ else }}` + "\U0001F449 {{ .DisplayName | cyan }} ({{ .Subscriptions | len }} subscriptions)" + `{{ end }}`,
		Inactive: `{{ if .NeedsMFA }}  ` + "\U0001F510 {{ .DisplayName | yellow }} (requires sign-in)" + `{{ else }}  {{ .DisplayName | cyan }} ({{ .Subscriptions | len }} subscriptions){{ end }}`,
		Selected: "\U0001F449 {{ .DisplayName | green }}",
		Details: `{{ if .NeedsMFA }}
--------- Tenant Details ----------
{{ "Name:" | faint }}	{{ .DisplayName }}
{{ "Domain:" | faint }}	{{ .DefaultDomain }}
{{ "ID:" | faint }}	{{ .TenantID }}
{{ "Status:" | faint }}	Requires additional authentication (MFA){{ else }}
--------- Tenant Details ----------
{{ "Name:" | faint }}	{{ .DisplayName }}
{{ "Domain:" | faint }}	{{ .DefaultDomain }}
{{ "ID:" | faint }}	{{ .TenantID }}
{{ "Subscriptions:" | faint }}	{{ .Subscriptions | len }}{{ end }}`,
	}

	// Create display items with proper tenant names
	type tenantDisplay struct {
		azure.TenantInfo
		DisplayName string
	}

	var items []tenantDisplay
	for _, t := range tenantInfos {
		name := t.DisplayName
		if name == "" {
			name = t.DefaultDomain
		}
		if name == "" {
			name = t.TenantID
		}
		items = append(items, tenantDisplay{
			TenantInfo:  t,
			DisplayName: name,
		})
	}

	prompt := promptui.Select{
		Label:     "Select Tenant",
		Items:     items,
		Templates: templates,
		Size:      10,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	return &tenantInfos[idx], nil
}

func promptForSubscriptionInTenant(tenant *azure.TenantInfo) (*config.Subscription, error) {
	if len(tenant.Subscriptions) == 0 {
		return nil, fmt.Errorf("no subscriptions found in tenant")
	}

	tenantName := tenant.DisplayName
	if tenantName == "" {
		tenantName = tenant.DefaultDomain
	}
	if tenantName == "" {
		tenantName = tenant.TenantID
	}

	logger.Println("")
	logger.Println("[Subscription selection]")
	logger.Println("Tenant: %s", tenantName)
	logger.Println("")

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "\U0001F449 {{ .Name | cyan }}",
		Inactive: "  {{ .Name }}",
		Selected: "\U0001F449 {{ .Name | green }}",
		Details: `
--------- Subscription Details ----------
{{ "Name:" | faint }}	{{ .Name }}
{{ "ID:" | faint }}	{{ .ID }}
{{ "State:" | faint }}	{{ .State }}
{{ "Tenant:" | faint }}	{{ .TenantID }}`,
	}

	prompt := promptui.Select{
		Label:     "Select Subscription",
		Items:     tenant.Subscriptions,
		Templates: templates,
		Size:      10,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	return &tenant.Subscriptions[idx], nil
}

func promptForSubscriptionFlat(tenantInfos []azure.TenantInfo) (*config.Subscription, error) {
	logger.Println("")
	logger.Println("[Subscription selection]")
	logger.Println("")

	// Build flat list with tenant info
	type subscriptionWithTenant struct {
		config.Subscription
		TenantName string
	}

	var items []subscriptionWithTenant
	for _, tenant := range tenantInfos {
		tenantName := tenant.DisplayName
		if tenantName == "" {
			tenantName = tenant.DefaultDomain
		}
		if tenantName == "" {
			tenantName = tenant.TenantID
		}

		for _, sub := range tenant.Subscriptions {
			items = append(items, subscriptionWithTenant{
				Subscription: sub,
				TenantName:   tenantName,
			})
		}
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "\U0001F449 {{ .Name | cyan }} ({{ .TenantName | faint }})",
		Inactive: "  {{ .Name }} ({{ .TenantName | faint }})",
		Selected: "\U0001F449 {{ .Name | green }}",
		Details: `
--------- Subscription Details ----------
{{ "Name:" | faint }}	{{ .Name }}
{{ "ID:" | faint }}	{{ .ID }}
{{ "State:" | faint }}	{{ .State }}
{{ "Tenant:" | faint }}	{{ .TenantName }}
{{ "Tenant ID:" | faint }}	{{ .TenantID }}`,
	}

	searcher := func(input string, index int) bool {
		item := items[index]
		name := strings.Replace(strings.ToLower(item.Name), " ", "", -1)
		tenant := strings.Replace(strings.ToLower(item.TenantName), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)

		return strings.Contains(name, input) || strings.Contains(tenant, input)
	}

	prompt := promptui.Select{
		Label:     "Select Subscription",
		Items:     items,
		Templates: templates,
		Size:      10,
		Searcher:  searcher,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	result := items[idx].Subscription
	return &result, nil
}

// authenticateMFATenant performs interactive auth for an MFA-blocked tenant and returns updated TenantInfo
func authenticateMFATenant(ctx context.Context, tenant *azure.TenantInfo) (*azure.TenantInfo, error) {
	tenantName := tenant.DisplayName
	if tenantName == "" {
		tenantName = tenant.TenantID
	}

	logger.Println("")
	logger.Println("Tenant '%s' requires additional authentication (MFA).", tenantName)
	logger.Println("Opening browser for authentication...")
	logger.Println("")

	cred, err := azure.AuthenticateForTenant(ctx, tenant.TenantID)
	if err != nil {
		return nil, err
	}

	subs, err := azure.DiscoverTenantSubscriptions(ctx, tenant.TenantID, cred)
	if err != nil {
		return nil, err
	}

	logger.Info("Found %d subscription(s) in tenant '%s'", len(subs), tenantName)

	return &azure.TenantInfo{
		TenantID:      tenant.TenantID,
		DisplayName:   tenant.DisplayName,
		DefaultDomain: tenant.DefaultDomain,
		Subscriptions: subs,
		NeedsMFA:      false,
	}, nil
}

// promptForSubscriptionFlatWithMFA shows a flat subscription list with MFA-blocked tenants at the bottom
func promptForSubscriptionFlatWithMFA(ctx context.Context, activeTenants []azure.TenantInfo, mfaTenants []azure.TenantInfo) (*config.Subscription, error) {
	logger.Println("")
	logger.Println("[Subscription selection]")
	logger.Println("")

	type selectableItem struct {
		Name       string
		ID         string
		State      string
		TenantID   string
		TenantName string
		IsMFA      bool // True for MFA tenant placeholder items
	}

	var items []selectableItem

	// Add regular subscriptions
	for _, tenant := range activeTenants {
		tenantName := tenant.DisplayName
		if tenantName == "" {
			tenantName = tenant.DefaultDomain
		}
		if tenantName == "" {
			tenantName = tenant.TenantID
		}

		for _, sub := range tenant.Subscriptions {
			items = append(items, selectableItem{
				Name:       sub.Name,
				ID:         sub.ID,
				State:      sub.State,
				TenantID:   sub.TenantID,
				TenantName: tenantName,
				IsMFA:      false,
			})
		}
	}

	// Add MFA tenant placeholders
	for _, tenant := range mfaTenants {
		tenantName := tenant.DisplayName
		if tenantName == "" {
			tenantName = tenant.DefaultDomain
		}
		if tenantName == "" {
			tenantName = tenant.TenantID
		}

		items = append(items, selectableItem{
			Name:       tenantName + " (requires sign-in to view subscriptions)",
			ID:         "",
			TenantID:   tenant.TenantID,
			TenantName: tenantName,
			IsMFA:      true,
		})
	}

	templates := &promptui.SelectTemplates{
		Label:  "{{ . }}",
		Active: `{{ if .IsMFA }}` + "\U0001F449 \U0001F510 {{ .Name | yellow }}" + `{{ else }}` + "\U0001F449 {{ .Name | cyan }} ({{ .TenantName | faint }})" + `{{ end }}`,
		Inactive: `{{ if .IsMFA }}  ` + "\U0001F510 {{ .Name | yellow }}" + `{{ else }}  {{ .Name }} ({{ .TenantName | faint }}){{ end }}`,
		Selected: "\U0001F449 {{ .Name | green }}",
		Details: `{{ if .IsMFA }}
--------- Tenant Details ----------
{{ "Tenant:" | faint }}	{{ .TenantName }}
{{ "Tenant ID:" | faint }}	{{ .TenantID }}
{{ "Status:" | faint }}	Requires additional authentication (MFA)
{{ "Action:" | faint }}	Select to sign in and view subscriptions{{ else }}
--------- Subscription Details ----------
{{ "Name:" | faint }}	{{ .Name }}
{{ "ID:" | faint }}	{{ .ID }}
{{ "State:" | faint }}	{{ .State }}
{{ "Tenant:" | faint }}	{{ .TenantName }}
{{ "Tenant ID:" | faint }}	{{ .TenantID }}{{ end }}`,
	}

	searcher := func(input string, index int) bool {
		item := items[index]
		name := strings.Replace(strings.ToLower(item.Name), " ", "", -1)
		tenant := strings.Replace(strings.ToLower(item.TenantName), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)

		return strings.Contains(name, input) || strings.Contains(tenant, input)
	}

	prompt := promptui.Select{
		Label:     "Select Subscription",
		Items:     items,
		Templates: templates,
		Size:      10,
		Searcher:  searcher,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	selected := items[idx]

	// If the user selected an MFA tenant, authenticate and show its subscriptions
	if selected.IsMFA {
		mfaTenant := findMFATenant(mfaTenants, selected.TenantID)
		if mfaTenant == nil {
			return nil, fmt.Errorf("tenant %s not found", selected.TenantID)
		}

		resolvedTenant, err := authenticateMFATenant(ctx, mfaTenant)
		if err != nil {
			return nil, err
		}

		if len(resolvedTenant.Subscriptions) == 0 {
			return nil, fmt.Errorf("no subscriptions found in tenant '%s' after authentication", selected.TenantName)
		}

		// Let the user pick from the newly discovered subscriptions
		sub, err := promptForSubscriptionInTenant(resolvedTenant)
		if err != nil {
			return nil, err
		}
		return sub, nil
	}

	// Regular subscription selected
	result := config.Subscription{
		ID:              selected.ID,
		Name:            selected.Name,
		State:           selected.State,
		TenantID:        selected.TenantID,
		EnvironmentName: "AzureCloud",
		IsDefault:       false,
	}
	return &result, nil
}

func findMFATenant(mfaTenants []azure.TenantInfo, tenantID string) *azure.TenantInfo {
	for i := range mfaTenants {
		if mfaTenants[i].TenantID == tenantID {
			return &mfaTenants[i]
		}
	}
	return nil
}

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

	if len(tenantInfos) == 0 {
		return fmt.Errorf("no tenants with subscriptions found")
	}

	// Get all subscriptions for counting
	allSubscriptions := azure.GetAllSubscriptions(tenantInfos)
	if len(allSubscriptions) == 0 {
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

		selectedSub, err = promptForSubscriptionInTenant(selectedTenant)
		if err != nil {
			return fmt.Errorf("failed to select subscription: %w", err)
		}
	} else {
		// Single-step: flat list of all subscriptions
		logger.Debug("Using flat selection (%d subscriptions <= threshold of %d)", len(allSubscriptions), subscriptionThreshold)
		selectedSub, err = promptForSubscriptionFlat(tenantInfos)
		if err != nil {
			return fmt.Errorf("failed to select subscription: %w", err)
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
		Label:    "{{ . }}",
		Active:   "\U0001F449 {{ .DisplayName | cyan }} ({{ .Subscriptions | len }} subscriptions)",
		Inactive: "  {{ .DisplayName | cyan }} ({{ .Subscriptions | len }} subscriptions)",
		Selected: "\U0001F449 {{ .DisplayName | green }}",
		Details: `
--------- Tenant Details ----------
{{ "Name:" | faint }}	{{ .DisplayName }}
{{ "Domain:" | faint }}	{{ .DefaultDomain }}
{{ "ID:" | faint }}	{{ .TenantID }}
{{ "Subscriptions:" | faint }}	{{ .Subscriptions | len }}`,
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

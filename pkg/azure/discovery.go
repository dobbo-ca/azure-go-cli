package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
)

// TenantInfo represents a tenant with its subscriptions
type TenantInfo struct {
	TenantID      string
	DisplayName   string
	DefaultDomain string
	Subscriptions []config.Subscription
	NeedsMFA      bool // True if tenant requires interactive auth (e.g., Conditional Access MFA)
}

// DiscoverAllSubscriptionsWithAuth discovers subscriptions across all tenants
// This matches the official Azure CLI flow:
// 1. Authenticate once with "organizations" endpoint (triggers browser)
// 2. List all tenants (includes home + guest tenants)
// 3. For each tenant, create tenant-specific credential and get subscriptions silently
func DiscoverAllSubscriptionsWithAuth(ctx context.Context, baseCred azcore.TokenCredential) ([]TenantInfo, azidentity.AuthenticationRecord, error) {
	// Step 1: Get authentication record from the base credential
	// This will trigger interactive authentication ONCE
	var authRecord azidentity.AuthenticationRecord
	var err error

	// First, trigger authentication by calling Authenticate if available
	// This gets us the authentication record AND authenticates the user
	// Only the azure-cli mode credential has no explicit Authenticate step: it
	// borrows an already-authenticated Python Azure CLI session, so authRecord
	// stays zero-value. Every other mode must still authenticate here.
	if authCred, ok := baseCred.(interface {
		Authenticate(context.Context) (azidentity.AuthenticationRecord, error)
	}); ok {
		authRecord, err = authCred.Authenticate(ctx)
		if err != nil {
			return nil, azidentity.AuthenticationRecord{}, fmt.Errorf("failed to authenticate: %w", err)
		}
	} else if authMode() != AuthModeAzureCLI {
		return nil, azidentity.AuthenticationRecord{}, fmt.Errorf("credential does not support Authenticate method")
	}

	// Step 2: List ALL tenants using the base credential
	tenantsClient, err := armsubscriptions.NewTenantsClient(baseCred, nil)
	if err != nil {
		return nil, authRecord, fmt.Errorf("failed to create tenants client: %w", err)
	}

	var allTenants []*armsubscriptions.TenantIDDescription
	tenantPager := tenantsClient.NewListPager(nil)
	for tenantPager.More() {
		tenantPage, err := tenantPager.NextPage(ctx)
		if err != nil {
			return nil, authRecord, fmt.Errorf("failed to list tenants: %w", err)
		}
		allTenants = append(allTenants, tenantPage.Value...)
	}

	logger.Info("Found %d accessible tenant(s) (including guest tenants)", len(allTenants))

	// Show which tenants were discovered (debug level)
	for _, t := range allTenants {
		displayName := GetStringValue(t.DisplayName)
		if displayName == "" {
			displayName = GetStringValue(t.DefaultDomain)
		}
		if displayName == "" {
			displayName = *t.TenantID
		}
		logger.Debug("  - %s (Tenant ID: %s)", displayName, *t.TenantID)
	}

	// Step 3: For each tenant, create tenant-specific credential and get subscriptions
	// This matches Python CLI: create new credential per tenant with shared cache
	tenantInfos := []TenantInfo{}
	seenSubscriptions := make(map[string]bool) // For deduplication

	for _, tenant := range allTenants {
		if tenant.TenantID == nil {
			continue
		}

		tenantID := *tenant.TenantID
		tenantDisplay := GetStringValue(tenant.DisplayName)
		if tenantDisplay == "" {
			tenantDisplay = GetStringValue(tenant.DefaultDomain)
		}
		if tenantDisplay == "" {
			tenantDisplay = tenantID
		}

		logger.Debug("Getting subscriptions for tenant: %s", tenantDisplay)

		// Create a tenant-specific credential
		// This uses the authentication record from initial auth
		tenantCred, err := createTenantCredential(tenantID, authRecord)
		if err != nil {
			logger.Warning("Failed to create credential for tenant '%s' (%s): %v", tenantDisplay, tenantID, err)
			logger.Info("Skipping tenant '%s' - you may not have necessary permissions", tenantDisplay)
			continue
		}

		// Create subscriptions client with tenant-specific credential
		subsClient, err := armsubscriptions.NewClient(tenantCred, nil)
		if err != nil {
			logger.Warning("Failed to create subscriptions client for tenant '%s' (%s): %v", tenantDisplay, tenantID, err)
			continue
		}

		// List subscriptions in this tenant
		var tenantSubs []config.Subscription
		mfaRequired := false
		subsPager := subsClient.NewListPager(nil)
		for subsPager.More() {
			subsPage, err := subsPager.NextPage(ctx)
			if err != nil {
				// Check if interactive sign-in can fix this (MFA AADSTS50076 or
				// expired refresh token AADSTS70043)
				if needsInteractiveAuth(err) {
					logger.Debug("Tenant '%s' needs interactive sign-in - will offer re-auth if selected", tenantDisplay)
					mfaRequired = true
				} else {
					logger.Warning("Failed to list subscriptions for tenant '%s' (%s): %v", tenantDisplay, tenantID, err)
					logger.Info("You may not have sufficient permissions in this tenant")
				}
				break
			}

			for _, sub := range subsPage.Value {
				if sub.SubscriptionID == nil || sub.DisplayName == nil {
					continue
				}

				subID := *sub.SubscriptionID

				// De-duplicate: if we've seen this subscription, skip it
				if seenSubscriptions[subID] {
					continue
				}
				seenSubscriptions[subID] = true

				subscription := config.Subscription{
					ID:              subID,
					Name:            *sub.DisplayName,
					State:           string(*sub.State),
					TenantID:        tenantID, // Use the tenant we're querying from (token tenant)
					EnvironmentName: "AzureCloud",
					IsDefault:       false,
				}

				tenantSubs = append(tenantSubs, subscription)
			}
		}

		if mfaRequired {
			// Include MFA-blocked tenant so the user can choose to authenticate
			tenantInfos = append(tenantInfos, TenantInfo{
				TenantID:      tenantID,
				DisplayName:   tenantDisplay,
				DefaultDomain: GetStringValue(tenant.DefaultDomain),
				NeedsMFA:      true,
			})
		} else if len(tenantSubs) > 0 {
			logger.Debug("Found %d subscription(s) in tenant '%s'", len(tenantSubs), tenantDisplay)
			tenantInfos = append(tenantInfos, TenantInfo{
				TenantID:      tenantID,
				DisplayName:   tenantDisplay,
				DefaultDomain: GetStringValue(tenant.DefaultDomain),
				Subscriptions: tenantSubs,
			})
		} else {
			logger.Debug("No subscriptions found in tenant '%s'", tenantDisplay)
			logger.Info("You may not have any subscriptions or sufficient permissions in tenant '%s'", tenantDisplay)
		}
	}

	totalSubs := 0
	for _, ti := range tenantInfos {
		totalSubs += len(ti.Subscriptions)
	}
	logger.Info("Discovery complete: Found %d subscription(s) across %d tenant(s)", totalSubs, len(tenantInfos))

	return tenantInfos, authRecord, nil
}

// createTenantCredential creates a tenant-specific credential for silent token acquisition
// This matches Python CLI's approach of creating tenant-specific PublicClientApplication instances
func createTenantCredential(tenantID string, authRecord azidentity.AuthenticationRecord) (azcore.TokenCredential, error) {
	// NO user interaction - uses cached refresh tokens only (or, in azure-cli
	// mode, the Python Azure CLI's own cached token for this tenant)
	return TenantCredential(tenantID, authRecord)
}

// GetAllSubscriptions flattens all subscriptions from all tenants
func GetAllSubscriptions(tenantInfos []TenantInfo) []config.Subscription {
	var allSubs []config.Subscription
	for _, tenant := range tenantInfos {
		allSubs = append(allSubs, tenant.Subscriptions...)
	}
	return allSubs
}

// DiscoverAllSubscriptionsFromCache discovers subscriptions using cached credentials
// This is used for commands like 'account list' that don't need interactive auth
func DiscoverAllSubscriptionsFromCache(ctx context.Context) ([]TenantInfo, error) {
	// Load profile to get auth record
	profile, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("not authenticated. Please run 'az login' first: %w", err)
	}

	if profile.AuthMode != AuthModeAzureCLI && profile.AuthenticationRecord == nil {
		return nil, fmt.Errorf("no authentication record found. Please run 'az login'")
	}

	// Create base credential for listing tenants (use organizations)
	baseCred, err := BaseCredential()
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	// List ALL tenants
	tenantsClient, err := armsubscriptions.NewTenantsClient(baseCred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenants client: %w", err)
	}

	var allTenants []*armsubscriptions.TenantIDDescription
	tenantPager := tenantsClient.NewListPager(nil)
	for tenantPager.More() {
		tenantPage, err := tenantPager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list tenants: %w", err)
		}
		allTenants = append(allTenants, tenantPage.Value...)
	}

	// For each tenant, get subscriptions using cached credentials
	tenantInfos := []TenantInfo{}
	seenSubscriptions := make(map[string]bool)

	for _, tenant := range allTenants {
		if tenant.TenantID == nil {
			continue
		}

		tenantID := *tenant.TenantID
		tenantDisplay := GetStringValue(tenant.DisplayName)
		if tenantDisplay == "" {
			tenantDisplay = GetStringValue(tenant.DefaultDomain)
		}
		if tenantDisplay == "" {
			tenantDisplay = tenantID
		}

		// Create tenant-specific cached credential (silent - no user interaction)
		var authRecord azidentity.AuthenticationRecord
		if profile.AuthenticationRecord != nil {
			authRecord = *profile.AuthenticationRecord
		}
		tenantCred, err := TenantCredential(tenantID, authRecord)
		if err != nil {
			continue
		}

		// Create subscriptions client
		subsClient, err := armsubscriptions.NewClient(tenantCred, nil)
		if err != nil {
			continue
		}

		// List subscriptions in this tenant
		var tenantSubs []config.Subscription
		subsPager := subsClient.NewListPager(nil)
		for subsPager.More() {
			subsPage, err := subsPager.NextPage(ctx)
			if err != nil {
				break
			}

			for _, sub := range subsPage.Value {
				if sub.SubscriptionID == nil || sub.DisplayName == nil {
					continue
				}

				subID := *sub.SubscriptionID
				if seenSubscriptions[subID] {
					continue
				}
				seenSubscriptions[subID] = true

				subscription := config.Subscription{
					ID:              subID,
					Name:            *sub.DisplayName,
					State:           string(*sub.State),
					TenantID:        tenantID,
					EnvironmentName: "AzureCloud",
					IsDefault:       false,
				}

				tenantSubs = append(tenantSubs, subscription)
			}
		}

		if len(tenantSubs) > 0 {
			tenantInfos = append(tenantInfos, TenantInfo{
				TenantID:      tenantID,
				DisplayName:   tenantDisplay,
				DefaultDomain: GetStringValue(tenant.DefaultDomain),
				Subscriptions: tenantSubs,
			})
		}
	}

	return tenantInfos, nil
}

// needsInteractiveAuth reports whether a silent token acquisition failed for a
// reason that interactive sign-in can fix: an MFA / Conditional Access challenge
// (AADSTS50076), or an expired refresh token from Conditional Access sign-in
// frequency policy (AADSTS70043). Both are resolved by re-authenticating the
// tenant interactively, so the tenant is offered as "requires sign-in".
func needsInteractiveAuth(err error) bool {
	if err == nil {
		return false
	}
	// The broker reports the same condition as a status rather than an AADSTS code.
	if interactionRequired(err) {
		return true
	}
	errStr := err.Error()
	return strings.Contains(errStr, "AADSTS50076") || // MFA required
		strings.Contains(errStr, "AADSTS70043") // refresh token expired (sign-in frequency)
}

// AuthenticateForTenant performs interactive browser authentication targeting a specific tenant
// Used when a tenant requires MFA that wasn't satisfied by the initial login
func AuthenticateForTenant(ctx context.Context, tenantID string) (azcore.TokenCredential, error) {
	// In azure-cli mode there's no MSAL browser flow to fall back to - MFA is
	// handled by the Python Azure CLI's own login. Retry through the same
	// tenant-scoped Azure CLI credential; if the tenant still needs sign-in,
	// the caller surfaces that error telling the user to `az login --tenant`.
	if authMode() == AuthModeAzureCLI {
		return TenantCredential(tenantID, azidentity.AuthenticationRecord{})
	}

	// In broker mode the broker owns the interactive sign-in, scoped to this
	// tenant so it can satisfy the tenant's MFA policy.
	if authMode() == AuthModeBroker {
		return NewBrokerCredential(tenantID, true)
	}

	authority := fmt.Sprintf("https://login.microsoftonline.com/%s", tenantID)

	cacheAccessor, err := GetSharedMSALCache()
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	client, err := public.New(
		"04b07795-8ddb-461a-bbee-02f9e1bf7b46",
		public.WithAuthority(authority),
		public.WithCache(cacheAccessor),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create MSAL client for tenant: %w", err)
	}

	scopes := []string{"https://management.azure.com/.default"}

	// Interactive auth targeting this specific tenant — will trigger MFA
	result, err := client.AcquireTokenInteractive(ctx, scopes, interactiveOpts()...)
	if err != nil {
		return nil, fmt.Errorf("interactive authentication failed for tenant %s: %w", tenantID, err)
	}

	// Create a silent credential from the now-cached token
	account := result.Account
	authRecord := azidentity.AuthenticationRecord{
		Authority:     account.Environment,
		HomeAccountID: account.HomeAccountID,
		TenantID:      account.Realm,
		Username:      account.PreferredUsername,
		ClientID:      "04b07795-8ddb-461a-bbee-02f9e1bf7b46",
		Version:       "1.0",
	}

	return NewMSALSilentCredential(tenantID, authRecord)
}

// DiscoverTenantSubscriptions lists subscriptions for a single tenant using the provided credential
func DiscoverTenantSubscriptions(ctx context.Context, tenantID string, cred azcore.TokenCredential) ([]config.Subscription, error) {
	subsClient, err := armsubscriptions.NewClient(cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscriptions client: %w", err)
	}

	var subs []config.Subscription
	pager := subsClient.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list subscriptions: %w", err)
		}

		for _, sub := range page.Value {
			if sub.SubscriptionID == nil || sub.DisplayName == nil {
				continue
			}

			subs = append(subs, config.Subscription{
				ID:              *sub.SubscriptionID,
				Name:            *sub.DisplayName,
				State:           string(*sub.State),
				TenantID:        tenantID,
				EnvironmentName: "AzureCloud",
				IsDefault:       false,
			})
		}
	}

	return subs, nil
}

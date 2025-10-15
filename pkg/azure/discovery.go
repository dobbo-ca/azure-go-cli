package azure

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore"
  "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

// TenantInfo represents a tenant with its subscriptions
type TenantInfo struct {
  TenantID      string
  DisplayName   string
  DefaultDomain string
  Subscriptions []config.Subscription
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
  if authCred, ok := baseCred.(interface {
    Authenticate(context.Context) (azidentity.AuthenticationRecord, error)
  }); ok {
    authRecord, err = authCred.Authenticate(ctx)
    if err != nil {
      return nil, azidentity.AuthenticationRecord{}, fmt.Errorf("failed to authenticate: %w", err)
    }
  } else {
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

  fmt.Printf("Found %d accessible tenant(s) (including guest tenants).\n", len(allTenants))

  // Debug: Show which tenants were discovered
  for _, t := range allTenants {
    displayName := GetStringValue(t.DisplayName)
    if displayName == "" {
      displayName = GetStringValue(t.DefaultDomain)
    }
    fmt.Printf("  - Tenant: %s (%s)\n", displayName, *t.TenantID)
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

    fmt.Printf("  Getting subscriptions for tenant: %s\n", tenantDisplay)

    // Create a tenant-specific credential
    // This uses the authentication record from initial auth
    tenantCred, err := createTenantCredential(tenantID, authRecord)
    if err != nil {
      fmt.Printf("  Warning: failed to create credential for tenant %s: %v\n", tenantID, err)
      continue
    }

    // Create subscriptions client with tenant-specific credential
    subsClient, err := armsubscriptions.NewClient(tenantCred, nil)
    if err != nil {
      fmt.Printf("  Warning: failed to create subscriptions client for tenant %s: %v\n", tenantID, err)
      continue
    }

    // List subscriptions in this tenant
    var tenantSubs []config.Subscription
    subsPager := subsClient.NewListPager(nil)
    for subsPager.More() {
      subsPage, err := subsPager.NextPage(ctx)
      if err != nil {
        // Don't fail entire operation - just log and continue
        fmt.Printf("  Warning: failed to list subscriptions for tenant %s: %v\n", tenantID, err)
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

    if len(tenantSubs) > 0 {
      fmt.Printf("  Found %d subscription(s) in tenant %s\n", len(tenantSubs), tenantDisplay)
      tenantInfos = append(tenantInfos, TenantInfo{
        TenantID:      tenantID,
        DisplayName:   tenantDisplay,
        DefaultDomain: GetStringValue(tenant.DefaultDomain),
        Subscriptions: tenantSubs,
      })
    } else {
      fmt.Printf("  No subscriptions found in tenant %s\n", tenantDisplay)
    }
  }

  totalSubs := 0
  for _, ti := range tenantInfos {
    totalSubs += len(ti.Subscriptions)
  }
  fmt.Printf("Found %d subscription(s) across all tenants.\n", totalSubs)

  return tenantInfos, authRecord, nil
}

// createTenantCredential creates a tenant-specific credential for silent token acquisition
// This matches Python CLI's approach of creating tenant-specific PublicClientApplication instances
func createTenantCredential(tenantID string, authRecord azidentity.AuthenticationRecord) (azcore.TokenCredential, error) {
  // Use custom MSAL credential that calls AcquireTokenSilent
  // This is exactly what Python CLI does: acquire_token_silent_with_error
  // NO user interaction - uses cached refresh tokens only
  return NewMSALSilentCredential(tenantID, authRecord)
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

  if profile.AuthenticationRecord == nil {
    return nil, fmt.Errorf("no authentication record found. Please run 'az login'")
  }

  // Create base credential for listing tenants (use organizations)
  baseCred, err := NewMSALInteractiveCredential()
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
    tenantCred, err := NewMSALSilentCredential(tenantID, *profile.AuthenticationRecord)
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

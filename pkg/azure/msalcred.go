package azure

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore"
  "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
  "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
  "github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
)

// MSALSilentCredential wraps MSAL PublicClientApplication for silent token acquisition
// This matches the Python CLI's approach: create tenant-specific MSAL apps that acquire tokens silently
type MSALSilentCredential struct {
  client  public.Client
  account public.Account
  scopes  []string
}

// NewMSALSilentCredential creates a credential that uses MSAL's AcquireTokenSilent
// This allows acquiring tokens from cache/refresh token without user interaction
func NewMSALSilentCredential(tenantID string, authRecord azidentity.AuthenticationRecord) (*MSALSilentCredential, error) {
  // Create MSAL PublicClientApplication for this specific tenant
  // This matches Python CLI: new PublicClientApplication per tenant with tenant-specific authority
  authority := fmt.Sprintf("https://login.microsoftonline.com/%s", tenantID)

  // Get shared cache - this is critical for silent token acquisition
  cacheAccessor, err := GetSharedMSALCache()
  if err != nil {
    return nil, fmt.Errorf("failed to create cache: %w", err)
  }

  client, err := public.New(
    "04b07795-8ddb-461a-bbee-02f9e1bf7b46",
    public.WithAuthority(authority),
    public.WithCache(cacheAccessor), // Use shared cache!
  )
  if err != nil {
    return nil, fmt.Errorf("failed to create MSAL client: %w", err)
  }

  // Convert AuthenticationRecord to MSAL Account
  // The account is used for silent token acquisition
  account := public.Account{
    HomeAccountID:  authRecord.HomeAccountID,
    Environment:    authRecord.Authority,
    Realm:          authRecord.TenantID,
    LocalAccountID: authRecord.HomeAccountID,
    PreferredUsername: authRecord.Username,
    AuthorityType:  "MSSTS",
  }

  return &MSALSilentCredential{
    client:  client,
    account: account,
    scopes:  []string{"https://management.azure.com/.default"},
  }, nil
}

// GetToken implements azcore.TokenCredential interface
func (m *MSALSilentCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
  // Use AcquireTokenSilent - this ONLY uses cache and refresh tokens (no user interaction)
  // This is exactly what Python CLI does: acquire_token_silent_with_error
  result, err := m.client.AcquireTokenSilent(ctx, m.scopes, public.WithSilentAccount(m.account))
  if err != nil {
    return azcore.AccessToken{}, fmt.Errorf("failed to acquire token silently: %w", err)
  }

  return azcore.AccessToken{
    Token:     result.AccessToken,
    ExpiresOn: result.ExpiresOn,
  }, nil
}

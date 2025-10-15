package azure

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore"
  "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
  "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
  "github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
)

// MSALInteractiveCredential uses MSAL's AcquireTokenInteractive for initial auth
// Then all subsequent operations use AcquireTokenSilent with the shared cache
type MSALInteractiveCredential struct {
  client public.Client
  scopes []string
}

// NewMSALInteractiveCredential creates the base credential for "organizations" authentication
func NewMSALInteractiveCredential() (*MSALInteractiveCredential, error) {
  // Create MSAL PublicClientApplication with "organizations" authority
  // This is the same as Python CLI's initial authentication
  authority := "https://login.microsoftonline.com/organizations"

  // Get shared cache
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
    return nil, fmt.Errorf("failed to create MSAL client: %w", err)
  }

  return &MSALInteractiveCredential{
    client: client,
    scopes: []string{"https://management.azure.com/.default"},
  }, nil
}

// Authenticate performs interactive browser authentication and returns the auth record
func (m *MSALInteractiveCredential) Authenticate(ctx context.Context) (azidentity.AuthenticationRecord, error) {
  // AcquireTokenInteractive opens browser for user authentication
  // This is equivalent to Python CLI's acquire_token_interactive
  result, err := m.client.AcquireTokenInteractive(ctx, m.scopes)
  if err != nil {
    return azidentity.AuthenticationRecord{}, fmt.Errorf("interactive authentication failed: %w", err)
  }

  // Convert MSAL account to AuthenticationRecord for compatibility
  account := result.Account
  authRecord := azidentity.AuthenticationRecord{
    Authority:      account.Environment,
    HomeAccountID:  account.HomeAccountID,
    TenantID:       account.Realm,
    Username:       account.PreferredUsername,
    ClientID:       "04b07795-8ddb-461a-bbee-02f9e1bf7b46",
    Version:        "1.0", // Required by Azure SDK
  }

  return authRecord, nil
}

// GetToken implements azcore.TokenCredential interface
func (m *MSALInteractiveCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
  // Try silent acquisition first
  accounts, err := m.client.Accounts(ctx)
  if err == nil && len(accounts) > 0 {
    result, err := m.client.AcquireTokenSilent(ctx, m.scopes, public.WithSilentAccount(accounts[0]))
    if err == nil {
      return azcore.AccessToken{
        Token:     result.AccessToken,
        ExpiresOn: result.ExpiresOn,
      }, nil
    }
  }

  // If silent fails, fall back to interactive
  result, err := m.client.AcquireTokenInteractive(ctx, m.scopes)
  if err != nil {
    return azcore.AccessToken{}, fmt.Errorf("failed to acquire token: %w", err)
  }

  return azcore.AccessToken{
    Token:     result.AccessToken,
    ExpiresOn: result.ExpiresOn,
  }, nil
}

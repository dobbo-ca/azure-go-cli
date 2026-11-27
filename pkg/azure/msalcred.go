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
	client   public.Client
	account  public.Account
	scopes   []string
	tenantID string
}

// NewMSALSilentCredential creates a credential that uses MSAL's AcquireTokenSilent
// This allows acquiring tokens from cache/refresh token without user interaction
func NewMSALSilentCredential(tenantID string, authRecord azidentity.AuthenticationRecord) (*MSALSilentCredential, error) {
	// Get shared cache - this is critical for silent token acquisition
	cacheAccessor, err := GetSharedMSALCache()
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}
	return newMSALSilentCredential(tenantID, authRecord, public.WithCache(cacheAccessor))
}

// newMSALSilentCredential builds the credential with extra MSAL options (the
// cache, and in tests an HTTP client).
func newMSALSilentCredential(tenantID string, authRecord azidentity.AuthenticationRecord, opts ...public.Option) (*MSALSilentCredential, error) {
	// Create MSAL PublicClientApplication for this specific tenant
	// This matches Python CLI: new PublicClientApplication per tenant with tenant-specific authority
	authority := fmt.Sprintf("https://login.microsoftonline.com/%s", tenantID)

	client, err := public.New(
		"04b07795-8ddb-461a-bbee-02f9e1bf7b46",
		append([]public.Option{public.WithAuthority(authority)}, opts...)...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create MSAL client: %w", err)
	}

	// Convert AuthenticationRecord to MSAL Account
	// The account is used for silent token acquisition
	account := public.Account{
		HomeAccountID:     authRecord.HomeAccountID,
		Environment:       authRecord.Authority,
		Realm:             authRecord.TenantID,
		LocalAccountID:    authRecord.HomeAccountID,
		PreferredUsername: authRecord.Username,
		AuthorityType:     "MSSTS",
	}

	return &MSALSilentCredential{
		client:   client,
		account:  account,
		scopes:   []string{}, // Don't hardcode scopes - will use opts.Scopes in GetToken
		tenantID: tenantID,
	}, nil
}

// GetToken implements azcore.TokenCredential interface
func (m *MSALSilentCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	// Use the scopes from opts if provided, otherwise fall back to management API
	scopes := opts.Scopes
	if len(scopes) == 0 {
		scopes = []string{"https://management.azure.com/.default"}
	}

	// Use AcquireTokenSilent - this ONLY uses cache and refresh tokens (no user interaction)
	// This is exactly what Python CLI does: acquire_token_silent_with_error
	result, err := m.client.AcquireTokenSilent(ctx, scopes, public.WithSilentAccount(m.account))
	if err != nil {
		if NeedsInteractiveAuth(err) {
			return azcore.AccessToken{}, &LoginRequiredError{TenantID: m.tenantID, Err: err}
		}
		return azcore.AccessToken{}, fmt.Errorf("failed to acquire token silently: %w", err)
	}

	return azcore.AccessToken{
		Token:     result.AccessToken,
		ExpiresOn: result.ExpiresOn,
	}, nil
}

// LoginRequiredError is a silent token failure that only a new sign-in can
// fix: a revoked or expired refresh token (AADSTS50173, AADSTS70043,
// AADSTS700082) or an MFA challenge (AADSTS50076). See NeedsInteractiveAuth. Like
// the Python Azure CLI, commands fail with the AAD reason and an "az login"
// hint instead of opening a browser mid-command.
type LoginRequiredError struct {
	TenantID string
	Err      error
}

func (e *LoginRequiredError) Error() string {
	reason := e.Err.Error()
	if desc := aadErrorDescription(e.Err); desc != "" {
		reason = desc
	}
	return fmt.Sprintf("failed to acquire token silently for tenant %s: %s\nInteractive authentication is needed. Please run 'az login' (or 'az login --tenant %s' to sign in to only this tenant).", e.TenantID, reason, e.TenantID)
}

func (e *LoginRequiredError) Unwrap() error { return e.Err }

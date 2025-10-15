package azure

import (
  "context"

  "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// GetInteractiveBrowserCredentialWithCache creates an MSAL-based credential
// This uses the same flow as the official Azure CLI - opens browser for authentication
// Uses shared MSAL file cache so tenant-specific credentials can access the same tokens
func GetInteractiveBrowserCredentialWithCache() (*MSALInteractiveCredential, error) {
  // Use MSAL directly to ensure cache sharing
  return NewMSALInteractiveCredential()
}

// GetDeviceCodeCredentialWithCache creates a DeviceCodeCredential with file-based caching
// This is used for --use-device-code flag (alternative flow)
func GetDeviceCodeCredentialWithCache(userPrompt func(context.Context, azidentity.DeviceCodeMessage) error) (*FileCachedCredential, error) {
  // Create device code credential without SDK cache (which uses keychain)
  opts := &azidentity.DeviceCodeCredentialOptions{
    ClientID: "04b07795-8ddb-461a-bbee-02f9e1bf7b46", // Azure CLI public client ID
    TenantID: "organizations",                        // Support multi-tenant scenarios
  }

  if userPrompt != nil {
    opts.UserPrompt = userPrompt
  }

  innerCred, err := azidentity.NewDeviceCodeCredential(opts)
  if err != nil {
    return nil, err
  }

  // Wrap with file-based cache
  return NewFileCachedCredential(innerCred)
}

// GetDeviceCodeCredentialWithCacheAndTenant creates a DeviceCodeCredential for a specific tenant
func GetDeviceCodeCredentialWithCacheAndTenant(tenantID string, userPrompt func(context.Context, azidentity.DeviceCodeMessage) error) (*FileCachedCredential, error) {
  // Create device code credential without SDK cache
  opts := &azidentity.DeviceCodeCredentialOptions{
    ClientID: "04b07795-8ddb-461a-bbee-02f9e1bf7b46",
    TenantID: tenantID,
  }

  if userPrompt != nil {
    opts.UserPrompt = userPrompt
  }

  innerCred, err := azidentity.NewDeviceCodeCredential(opts)
  if err != nil {
    return nil, err
  }

  // Wrap with file-based cache
  return NewFileCachedCredential(innerCred)
}

// GetDeviceCodeCredentialWithAuthRecord creates a DeviceCodeCredential using a saved authentication record
// This allows the credential to use cached tokens without prompting the user
func GetDeviceCodeCredentialWithAuthRecord(authRecord azidentity.AuthenticationRecord) (*FileCachedCredential, error) {
  // Use the tenant ID from the authentication record (the home tenant)
  // but allow access to all tenants via AdditionallyAllowedTenants
  tenantID := authRecord.TenantID
  if tenantID == "" {
    tenantID = "organizations"
  }

  opts := &azidentity.DeviceCodeCredentialOptions{
    ClientID:             "04b07795-8ddb-461a-bbee-02f9e1bf7b46",
    TenantID:             tenantID,
    AuthenticationRecord: authRecord,
    // Allow credential to get tokens for any tenant
    // This is crucial for guest scenarios where subscriptions are in different tenants
    AdditionallyAllowedTenants:     []string{"*"},
    DisableAutomaticAuthentication: false,
  }

  innerCred, err := azidentity.NewDeviceCodeCredential(opts)
  if err != nil {
    return nil, err
  }

  // Wrap with file-based cache
  return NewFileCachedCredential(innerCred)
}

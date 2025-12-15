package azure

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

// GetCredentialWithTenantSupport returns a credential that properly handles multi-tenant scenarios
// This uses the MSAL cache from login to provide silent authentication
func GetCredentialWithTenantSupport() (azcore.TokenCredential, error) {
	profile, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("not authenticated. Please run 'az login' first: %w", err)
	}

	if profile.AuthenticationRecord == nil {
		return nil, fmt.Errorf("no authentication record found. Please run 'az login'")
	}

	// Get the tenant ID from the default subscription
	var tenantID string
	for _, sub := range profile.Subscriptions {
		if sub.IsDefault {
			tenantID = sub.TenantID
			break
		}
	}

	if tenantID == "" && len(profile.Subscriptions) > 0 {
		tenantID = profile.Subscriptions[0].TenantID
	}

	if tenantID == "" {
		return nil, fmt.Errorf("no tenant ID found in subscriptions")
	}

	// Create MSAL silent credential for the default subscription's tenant
	// This will use the cached tokens from login - no user interaction
	return NewMSALSilentCredential(tenantID, *profile.AuthenticationRecord)
}

// GetCredentialForTenant returns a credential for a specific tenant
// This is useful when working with resources in a different tenant than the default
func GetCredentialForTenant(tenantID string) (azcore.TokenCredential, error) {
	profile, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("not authenticated. Please run 'az login' first: %w", err)
	}

	if profile.AuthenticationRecord == nil {
		return nil, fmt.Errorf("no authentication record found. Please run 'az login'")
	}

	// Create MSAL silent credential for the specified tenant
	return NewMSALSilentCredential(tenantID, *profile.AuthenticationRecord)
}

package azure

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
)

// GetCredential returns a credential with MSAL persistent token caching
// It loads the authentication record from the last login and uses it to access cached tokens
func GetCredential() (azcore.TokenCredential, error) {
	// Use the new multi-tenant aware credential
	// This handles guest user scenarios where the user's home tenant
	// differs from the tenant containing the Azure resources
	return GetCredentialWithTenantSupport()
}

// GetCredentialWithTenantSupport is implemented in multicred.go
// It handles multi-tenant scenarios including guest user access

func GetStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func GetOSType(osType string) *armcontainerservice.OSType {
	t := armcontainerservice.OSType(osType)
	return &t
}

func GetAgentPoolMode(mode string) *armcontainerservice.AgentPoolMode {
	m := armcontainerservice.AgentPoolMode(mode)
	return &m
}

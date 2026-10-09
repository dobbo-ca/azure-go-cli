package account

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// TokenResponse matches the format expected by kubelogin
type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	ExpiresOn    string `json:"expiresOn"`
	Subscription string `json:"subscription"`
	Tenant       string `json:"tenant"`
	TokenType    string `json:"tokenType"`
}

// GetAccessToken retrieves an access token for a specific resource or scope
func GetAccessToken(cmd *cobra.Command, resource string, scopes []string, subscriptionID, tenantID string) error {
	ctx := context.Background()

	if err := checkTenantFlags(subscriptionID, tenantID); err != nil {
		return err
	}

	logger.Debug("get-access-token called")
	logger.Debug("  resource: %s", resource)
	logger.Debug("  scopes: %v", scopes)
	logger.Debug("  subscription: %s", subscriptionID)
	logger.Debug("  tenant: %s", tenantID)

	// Get credentials
	var cred azcore.TokenCredential
	var err error
	if tenantID != "" {
		cred, err = tenantCredential(tenantID)
	} else {
		cred, err = azure.GetCredential()
	}
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}
	logger.Debug("Successfully obtained credentials")

	// Get subscription if not provided
	if subscriptionID == "" && tenantID == "" {
		subscriptionID, err = config.GetSubscription("")
		if err != nil {
			return fmt.Errorf("failed to get subscription: %w", err)
		}
		logger.Debug("Using subscription: %s", subscriptionID)
	}

	// Resolve scopes: --scope takes precedence, then --resource, then default ARM
	var tokenScopes []string
	if len(scopes) > 0 {
		tokenScopes = scopes
	} else if resource != "" {
		scope := resource + "/.default"
		tokenScopes = []string{scope}
	} else {
		tokenScopes = []string{"https://management.azure.com/.default"}
	}
	logger.Debug("Requesting token with scopes: %v", tokenScopes)

	// Get access token
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: tokenScopes,
	})
	if err != nil {
		logger.Debug("Failed to get token: %v", err)
		return fmt.Errorf("failed to get token: %w", err)
	}
	logger.Debug("Token acquired successfully")
	logger.Debug("  Token length: %d", len(token.Token))
	logger.Debug("  Expires: %s", token.ExpiresOn.Format("2006-01-02 15:04:05"))

	// Format expiry time in the format expected by Azure SDK
	// Format: "2006-01-02 15:04:05.999999"
	expiresOn := token.ExpiresOn.Format("2006-01-02 15:04:05.000000")

	if tenantID != "" {
		// Python omits subscription when --tenant is given
		return output.PrintJSON(cmd, struct {
			AccessToken string `json:"accessToken"`
			ExpiresOn   string `json:"expiresOn"`
			Tenant      string `json:"tenant"`
			TokenType   string `json:"tokenType"`
		}{token.Token, expiresOn, tenantID, "Bearer"})
	}

	response := TokenResponse{
		AccessToken:  token.Token,
		ExpiresOn:    expiresOn,
		Subscription: subscriptionID,
		Tenant:       "", // We don't have tenant ID readily available
		TokenType:    "Bearer",
	}

	return output.PrintJSON(cmd, response)
}

func checkTenantFlags(subscriptionID, tenantID string) error {
	if subscriptionID != "" && tenantID != "" {
		return fmt.Errorf("please specify only one of subscription and tenant, not both")
	}
	return nil
}

// tenantCredential builds a credential bound to tenantID; the shared one is bound to the default subscription's tenant.
func tenantCredential(tenantID string) (azcore.TokenCredential, error) {
	profile, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("not authenticated. Please run 'az login' first: %w", err)
	}
	var rec azidentity.AuthenticationRecord
	if profile.AuthenticationRecord != nil {
		rec = *profile.AuthenticationRecord
	}
	return azure.TenantCredential(tenantID, rec)
}

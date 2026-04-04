package account

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
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
func GetAccessToken(resource string, scopes []string, subscriptionID string) error {
	ctx := context.Background()

	logger.Debug("get-access-token called")
	logger.Debug("  resource: %s", resource)
	logger.Debug("  scopes: %v", scopes)
	logger.Debug("  subscription: %s", subscriptionID)

	// Get credentials
	cred, err := azure.GetCredential()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}
	logger.Debug("Successfully obtained credentials")

	// Get subscription if not provided
	if subscriptionID == "" {
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

	response := TokenResponse{
		AccessToken:  token.Token,
		ExpiresOn:    expiresOn,
		Subscription: subscriptionID,
		Tenant:       "", // We don't have tenant ID readily available
		TokenType:    "Bearer",
	}

	// Output JSON
	output, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

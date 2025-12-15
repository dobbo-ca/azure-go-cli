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

// GetAccessToken retrieves an access token for a specific resource
func GetAccessToken(resource, subscriptionID string) error {
	ctx := context.Background()

	logger.Debug("get-access-token called")
	logger.Debug("  resource: %s", resource)
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

	// Construct scope from resource
	// Azure resource format: https://management.azure.com/.default
	// K8s server ID: 6dae42f8-4368-4678-94ff-3960e28e3630
	scope := resource
	if resource != "" && resource[len(resource)-1] != '/' {
		scope = resource + "/.default"
	}
	logger.Debug("Requesting token with scope: %s", scope)

	// Get access token
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{scope},
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

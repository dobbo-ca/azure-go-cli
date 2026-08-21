package keyvault

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

// resolveObjectID turns --object-id, --spn or --upn into a directory object
// id, as _object_id_args_helper does (custom.py:775).
func resolveObjectID(ctx context.Context, objectID, spn, upn string) (string, error) {
	if objectID != "" {
		return objectID, nil
	}
	if spn == "" && upn == "" {
		return "", fmt.Errorf("one of --object-id, --spn or --upn is required")
	}

	path := fmt.Sprintf("/v1.0/users/%s?$select=id", url.PathEscape(upn))
	if spn != "" {
		filter := url.QueryEscape(fmt.Sprintf("servicePrincipalNames/any(c:c eq '%s')", spn))
		path = fmt.Sprintf("/v1.0/servicePrincipals?$filter=%s&$select=id", filter)
	}

	body, err := graphGet(ctx, path)
	if err != nil {
		return "", err
	}
	var result struct {
		ID    string `json:"id"`
		Value []struct {
			ID string `json:"id"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to read the Microsoft Graph reply: %w", err)
	}
	id := result.ID
	if id == "" && len(result.Value) > 0 {
		id = result.Value[0].ID
	}
	if id == "" {
		return "", fmt.Errorf("unable to get object id from principal name")
	}
	return id, nil
}

// graphGet calls Microsoft Graph with the signed-in credential.
func graphGet(ctx context.Context, path string) ([]byte, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, err
	}
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://graph.microsoft.com/.default"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get a Microsoft Graph token: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://graph.microsoft.com"+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Microsoft Graph: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("microsoft graph returned %s: %s", resp.Status, body)
	}
	return body, nil
}

// distinct drops repeated permissions, as _permissions_distinct does
// (custom.py:784).
func distinct(values []string) []string {
	if len(values) == 0 {
		return values
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

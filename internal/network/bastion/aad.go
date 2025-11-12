package bastion

import (
  "context"
  "encoding/base64"
  "encoding/json"
  "fmt"
  "io"
  "net/http"
  "net/url"
  "os"
  "path/filepath"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore"
  "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
  "github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
  "github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
  "github.com/cdobbyn/azure-go-cli/internal/network/bastion/sshkeys"
  "github.com/cdobbyn/azure-go-cli/pkg/logger"
)

// Cloud-specific AAD scopes for SSH certificate requests
var cloudScopes = map[string]string{
  "azurecloud":        "https://pas.windows.net/CheckMyAccess/Linux/.default",
  "azurechinacloud":   "https://pas.chinacloudapi.cn/CheckMyAccess/Linux/.default",
  "azureusgovernment": "https://pasff.usgovcloudapi.net/CheckMyAccess/Linux/.default",
}

// min returns the minimum of two integers
func min(a, b int) int {
  if a < b {
    return a
  }
  return b
}

// GetAADSSHCertificate requests an SSH certificate from Azure AD
func GetAADSSHCertificate(ctx context.Context, cred azcore.TokenCredential, keyPair *sshkeys.KeyPair, cloudName string) (string, error) {
  // Get scope for cloud
  scope, ok := cloudScopes[cloudName]
  if !ok {
    return "", fmt.Errorf("unsupported cloud: %s (supported: azurecloud, azurechinacloud, azureusgovernment)", cloudName)
  }

  logger.Debug("Requesting SSH certificate from AAD for cloud: %s", cloudName)
  logger.Debug("Using scope: %s", scope)

  // Create certificate request data
  certReq, err := sshkeys.CreateCertificateRequest(keyPair)
  if err != nil {
    return "", fmt.Errorf("failed to create certificate request: %w", err)
  }

  logger.Debug("Certificate request key_id: %s", certReq.KeyID)

  // First get a regular access token to use for the SSH cert request
  token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
    Scopes: []string{scope},
  })
  if err != nil {
    return "", fmt.Errorf("failed to get access token: %w", err)
  }

  // The token.Token should actually be the SSH certificate if the request
  // was made with the SSH cert scope and the credential supports it
  // However, the standard Azure SDK doesn't pass the custom data
  // So we need a workaround: use the token as-is if it's an SSH cert format
  // or make a custom request

  // Check if token looks like an SSH certificate (starts with "ssh-rsa-cert")
  if strings.HasPrefix(token.Token, "ssh-rsa-cert") || strings.Contains(token.Token, "ssh-rsa-cert") {
    logger.Debug("Received SSH certificate from token request")
    return token.Token, nil
  }

  logger.Debug("Standard token received, attempting custom SSH cert request...")

  // Fall back to custom HTTP request with certificate request data
  // This matches what the Azure CLI Python does with profile.get_msal_token(scopes, data)
  return requestSSHCertWithToken(ctx, token.Token, scope, certReq, cloudName)
}

// requestSSHCertWithToken uses MSAL to request an SSH certificate with JWK claims
func requestSSHCertWithToken(ctx context.Context, bearerToken, scope string, certReq *sshkeys.CertificateRequest, cloudName string) (string, error) {
  logger.Debug("Requesting SSH certificate using MSAL with claims")

  // Try to use MSAL with cached credentials from Azure CLI
  // Azure CLI stores tokens in ~/.azure/msal_token_cache.json
  homeDir, err := os.UserHomeDir()
  if err != nil {
    return "", fmt.Errorf("failed to get home directory: %w", err)
  }

  azureDir := filepath.Join(homeDir, ".azure")
  cacheFile := filepath.Join(azureDir, "msal_token_cache.json")

  logger.Debug("Looking for MSAL cache at: %s", cacheFile)

  // Check if cache exists
  if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
    return "", fmt.Errorf(`no Azure CLI login found. Please login first with:
    az login

Then try again. The SSH certificate request requires access to your cached Azure credentials.`)
  }

  // Create MSAL public client
  // The client ID for Azure CLI is a well-known constant
  azureCliClientID := "04b07795-8ddb-461a-bbee-02f9e1bf7b46"

  client, err := public.New(azureCliClientID,
    public.WithAuthority("https://login.microsoftonline.com/common"),
    public.WithCache(&fileCache{path: cacheFile}))
  if err != nil {
    return "", fmt.Errorf("failed to create MSAL client: %w", err)
  }

  // Create claims JSON with certificate request data
  // Based on Azure SSH cert protocol, we need req_cnf in the access_token
  // The JWK needs to be embedded as a JSON object
  claims := map[string]interface{}{
    "access_token": map[string]interface{}{
      "req_cnf": json.RawMessage(certReq.ReqCnf),
    },
  }

  claimsBytes, err := json.Marshal(claims)
  if err != nil {
    return "", fmt.Errorf("failed to marshal claims: %w", err)
  }
  claimsJSON := string(claimsBytes)

  logger.Debug("Claims JSON: %s", claimsJSON)

  // Get accounts
  accounts, err := client.Accounts(ctx)
  if err != nil || len(accounts) == 0 {
    return "", fmt.Errorf("no cached accounts found. Please run 'az login' first")
  }

  logger.Debug("Found %d cached accounts, using first account", len(accounts))

  // Python MSAL passes SSH cert data as form parameters in the token request
  // We replicate this by making a custom token request
  certificate, err := requestSSHCertViaTokenEndpoint(ctx, cacheFile, accounts[0], scope, certReq, cloudName)
  if err != nil {
    return "", fmt.Errorf("failed to get SSH certificate: %w", err)
  }

  return certificate, nil
}

// requestSSHCertFromAzure calls Azure's certificate signing API
func requestSSHCertFromAzure(ctx context.Context, accessToken string, certReq *sshkeys.CertificateRequest, cloudName string) (string, error) {
  // Azure's certificate signing endpoint - it's a REST API endpoint
  // The scope we requested was for CheckMyAccess/Linux which is the SSH cert service
  var certEndpoint string
  switch strings.ToLower(cloudName) {
  case "azurecloud":
    certEndpoint = "https://pas.windows.net/api/sshcertificate"
  case "azurechinacloud":
    certEndpoint = "https://pas.chinacloudapi.cn/api/sshcertificate"
  case "azureusgovernment":
    certEndpoint = "https://pasff.usgovcloudapi.net/api/sshcertificate"
  default:
    return "", fmt.Errorf("unsupported cloud: %s", cloudName)
  }

  logger.Debug("Requesting SSH certificate from: %s", certEndpoint)

  // Parse the JWK JSON string back to an object
  var jwk map[string]interface{}
  if err := json.Unmarshal([]byte(certReq.ReqCnf), &jwk); err != nil {
    return "", fmt.Errorf("failed to parse JWK: %w", err)
  }

  // Create request with the actual data Azure expects
  reqData := map[string]interface{}{
    "token_type": "ssh-cert",
    "req_cnf":    jwk,
    "key_id":     certReq.KeyID,
  }

  reqBody, err := json.Marshal(reqData)
  if err != nil {
    return "", fmt.Errorf("failed to marshal request: %w", err)
  }

  logger.Debug("Request body: %s", string(reqBody))

  req, err := http.NewRequestWithContext(ctx, "POST", certEndpoint, strings.NewReader(string(reqBody)))
  if err != nil {
    return "", fmt.Errorf("failed to create request: %w", err)
  }

  req.Header.Set("Authorization", "Bearer "+accessToken)
  req.Header.Set("Content-Type", "application/json")
  req.Header.Set("Accept", "application/json")

  resp, err := http.DefaultClient.Do(req)
  if err != nil {
    return "", fmt.Errorf("failed to make request: %w", err)
  }
  defer resp.Body.Close()

  respBody, err := io.ReadAll(resp.Body)
  if err != nil {
    return "", fmt.Errorf("failed to read response: %w", err)
  }

  logger.Debug("Certificate response status: %d", resp.StatusCode)
  logger.Debug("Certificate response body: %s", string(respBody))

  if resp.StatusCode != http.StatusOK {
    return "", fmt.Errorf("certificate request failed with status %d: %s", resp.StatusCode, string(respBody))
  }

  // Parse response
  var certResp map[string]interface{}
  if err := json.Unmarshal(respBody, &certResp); err != nil {
    // Maybe it's just the certificate as plain text
    cert := string(respBody)
    if strings.Contains(cert, "ssh-rsa-cert-v01@openssh.com") {
      return cert, nil
    }
    return "", fmt.Errorf("failed to parse response: %w", err)
  }

  // Look for certificate in response
  if cert, ok := certResp["certificate"].(string); ok && cert != "" {
    return cert, nil
  }
  if cert, ok := certResp["cert"].(string); ok && cert != "" {
    return cert, nil
  }
  if cert, ok := certResp["signedCert"].(string); ok && cert != "" {
    return cert, nil
  }

  logger.Debug("Certificate response keys: %v", getMapKeys(certResp))
  return "", fmt.Errorf("no certificate found in response")
}

// extractCertFromJWT decodes a JWT and extracts the SSH certificate from the claims
func extractCertFromJWT(jwt string) (string, error) {
  // JWT format: header.payload.signature
  parts := strings.Split(jwt, ".")
  if len(parts) != 3 {
    return "", fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
  }

  // Decode the payload (second part)
  payload, err := base64.RawURLEncoding.DecodeString(parts[1])
  if err != nil {
    return "", fmt.Errorf("failed to decode JWT payload: %w", err)
  }

  // Parse the JSON payload
  var claims map[string]interface{}
  if err := json.Unmarshal(payload, &claims); err != nil {
    return "", fmt.Errorf("failed to parse JWT claims: %w", err)
  }

  logger.Debug("JWT claims keys: %v", getMapKeys(claims))

  // Look for SSH certificate in various possible claim names
  possibleKeys := []string{"cert", "certificate", "ssh_cert", "x5c", "cnf"}

  for _, key := range possibleKeys {
    if val, ok := claims[key]; ok {
      // Try as string
      if certStr, ok := val.(string); ok && certStr != "" {
        logger.Debug("Found SSH certificate in claim '%s'", key)
        return certStr, nil
      }

      // Try as array (like x5c)
      if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
        if certStr, ok := arr[0].(string); ok && certStr != "" {
          logger.Debug("Found SSH certificate in claim '%s' array", key)
          return certStr, nil
        }
      }

      // Try as nested object (like cnf.kid or cnf.jkt)
      if obj, ok := val.(map[string]interface{}); ok {
        logger.Debug("Claim '%s' is an object with keys: %v", key, getMapKeys(obj))
        // Check for common sub-keys
        for _, subKey := range []string{"cert", "ssh_cert", "kid", "jkt"} {
          if subVal, ok := obj[subKey].(string); ok && subVal != "" {
            logger.Debug("Found value in claim '%s.%s'", key, subKey)
            // The kid/jkt might be a reference, not the cert itself
            // Continue looking for actual cert
          }
        }
      }
    }
  }

  return "", fmt.Errorf("no SSH certificate found in JWT claims. Available claims: %v", getMapKeys(claims))
}

// getMapKeys returns the keys of a map for debugging
func getMapKeys(m map[string]interface{}) []string {
  keys := make([]string, 0, len(m))
  for k := range m {
    keys = append(keys, k)
  }
  return keys
}

// requestSSHCertificate makes an API call to get the SSH certificate using the access token
func requestSSHCertificate(ctx context.Context, accessToken, scope string, certReq *sshkeys.CertificateRequest) (string, error) {
  // Extract the base URL from the scope
  // scope is like "https://pas.windows.net/CheckMyAccess/Linux/.default"
  // The SSH cert endpoint is the scope without .default
  certURL := strings.TrimSuffix(scope, "/.default")

  logger.Debug("Requesting SSH certificate from: %s", certURL)

  // Azure CLI Python code shows the token with req_cnf is enough
  // We make a simple GET request with the bearer token
  req, err := http.NewRequestWithContext(ctx, "GET", certURL, nil)
  if err != nil {
    return "", fmt.Errorf("failed to create request: %w", err)
  }

  req.Header.Set("Authorization", "Bearer "+accessToken)

  resp, err := http.DefaultClient.Do(req)
  if err != nil {
    return "", fmt.Errorf("failed to make request: %w", err)
  }
  defer resp.Body.Close()

  respBody, err := io.ReadAll(resp.Body)
  if err != nil {
    return "", fmt.Errorf("failed to read response: %w", err)
  }

  if resp.StatusCode != http.StatusOK {
    logger.Debug("Certificate request failed. Status: %d, Response: %s", resp.StatusCode, string(respBody))
    return "", fmt.Errorf("certificate request failed with status %d: %s", resp.StatusCode, string(respBody))
  }

  // Parse response to extract certificate
  var certResp map[string]interface{}
  if err := json.Unmarshal(respBody, &certResp); err != nil {
    // Maybe the response is directly the certificate
    certStr := string(respBody)
    if strings.HasPrefix(certStr, "ssh-rsa-cert") {
      return certStr, nil
    }
    return "", fmt.Errorf("failed to parse certificate response: %w", err)
  }

  // Look for certificate in response
  if cert, ok := certResp["cert"].(string); ok && cert != "" {
    return cert, nil
  }
  if cert, ok := certResp["certificate"].(string); ok && cert != "" {
    return cert, nil
  }
  if cert, ok := certResp["ssh_cert"].(string); ok && cert != "" {
    return cert, nil
  }

  logger.Debug("Certificate response keys: %v", getMapKeys(certResp))
  return "", fmt.Errorf("no certificate found in response")
}

// findSSHCertInClaims searches for SSH certificate in ID token claims
func findSSHCertInClaims(claims map[string]interface{}) string {
  // Check for 'cnf' (confirmation) claim with SSH cert
  if cnf, ok := claims["cnf"].(map[string]interface{}); ok {
    logger.Debug("Found 'cnf' claim, inspecting...")

    // Check for ssh_cert in cnf
    if sshCert, ok := cnf["ssh_cert"].(string); ok && sshCert != "" {
      return sshCert
    }

    // Check for jkt (JWK thumbprint)
    if jkt, ok := cnf["jkt"].(string); ok {
      logger.Debug("Found jkt in cnf: %s", jkt)
    }

    // Check for kid
    if kid, ok := cnf["kid"].(string); ok {
      logger.Debug("Found kid in cnf: %s", kid)
    }

    // Log all cnf keys
    cnfKeys := make([]string, 0, len(cnf))
    for k := range cnf {
      cnfKeys = append(cnfKeys, k)
    }
    logger.Debug("cnf claim keys: %v", cnfKeys)
  }

  // Check for direct 'ssh_cert' claim
  if sshCert, ok := claims["ssh_cert"].(string); ok && sshCert != "" {
    return sshCert
  }

  // Check for 'x5c' (X.509 certificate chain)
  if x5c, ok := claims["x5c"].([]interface{}); ok && len(x5c) > 0 {
    if cert, ok := x5c[0].(string); ok {
      return cert
    }
  }

  return ""
}

// getIDTokenClaimKeys returns the keys from ID token claims for debugging
func getIDTokenClaimKeys(claims map[string]interface{}) []string {
  keys := make([]string, 0, len(claims))
  for k := range claims {
    keys = append(keys, k)
  }
  return keys
}

// getRefreshTokenFromCache reads the MSAL cache and extracts the refresh token for the account
func getRefreshTokenFromCache(cacheFile string, account public.Account) (string, error) {
  logger.Debug("Reading MSAL cache from: %s", cacheFile)

  data, err := os.ReadFile(cacheFile)
  if err != nil {
    return "", fmt.Errorf("failed to read cache file: %w", err)
  }

  // MSAL cache structure
  var cache struct {
    RefreshToken map[string]struct {
      Secret          string `json:"secret"`
      HomeAccountID   string `json:"home_account_id"`
      Environment     string `json:"environment"`
      ClientID        string `json:"client_id"`
      CredentialType  string `json:"credential_type"`
    } `json:"RefreshToken"`
  }

  if err := json.Unmarshal(data, &cache); err != nil {
    return "", fmt.Errorf("failed to parse cache file: %w", err)
  }

  logger.Debug("Found %d refresh tokens in cache", len(cache.RefreshToken))

  // Find refresh token matching the account's home_account_id
  accountID := account.HomeAccountID
  for key, rt := range cache.RefreshToken {
    if rt.HomeAccountID == accountID {
      logger.Debug("Found matching refresh token for account: %s", accountID)
      logger.Debug("Refresh token key: %s", key)
      return rt.Secret, nil
    }
  }

  // If no exact match, try to find any refresh token for the Azure CLI client
  azureCliClientID := "04b07795-8ddb-461a-bbee-02f9e1bf7b46"
  for key, rt := range cache.RefreshToken {
    if rt.ClientID == azureCliClientID {
      logger.Debug("Found refresh token for Azure CLI client: %s", key)
      return rt.Secret, nil
    }
  }

  return "", fmt.Errorf("no refresh token found in cache for account %s", accountID)
}

// requestSSHCertViaTokenEndpoint makes a direct HTTP request to Azure's token endpoint
// with SSH certificate data as form parameters. This replicates what Python MSAL does.
func requestSSHCertViaTokenEndpoint(ctx context.Context, cacheFile string, account public.Account, scope string, certReq *sshkeys.CertificateRequest, cloudName string) (string, error) {
  logger.Debug("Making custom token request with SSH cert data as form parameters")

  // Get the refresh token from the cache
  refreshToken, err := getRefreshTokenFromCache(cacheFile, account)
  if err != nil {
    return "", fmt.Errorf("failed to get refresh token: %w", err)
  }

  logger.Debug("Successfully extracted refresh token from cache")

  // Token endpoint
  tokenEndpoint := "https://login.microsoftonline.com/common/oauth2/v2.0/token"

  // Build form data with SSH certificate parameters
  formData := url.Values{}
  formData.Set("client_id", "04b07795-8ddb-461a-bbee-02f9e1bf7b46") // Azure CLI client ID
  formData.Set("scope", scope)
  formData.Set("refresh_token", refreshToken)
  formData.Set("grant_type", "refresh_token")

  // Add SSH cert data as form parameters (this is what Python MSAL does)
  formData.Set("token_type", "ssh-cert")
  formData.Set("req_cnf", certReq.ReqCnf) // JWK as JSON string
  formData.Set("key_id", certReq.KeyID)

  logger.Debug("Token endpoint: %s", tokenEndpoint)
  logger.Debug("Form data keys: %v", formData.Encode()[:min(200, len(formData.Encode()))])

  req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(formData.Encode()))
  if err != nil {
    return "", fmt.Errorf("failed to create request: %w", err)
  }

  req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

  resp, err := http.DefaultClient.Do(req)
  if err != nil {
    return "", fmt.Errorf("failed to make token request: %w", err)
  }
  defer resp.Body.Close()

  respBody, err := io.ReadAll(resp.Body)
  if err != nil {
    return "", fmt.Errorf("failed to read response: %w", err)
  }

  logger.Debug("Token response status: %d", resp.StatusCode)

  if resp.StatusCode != http.StatusOK {
    logger.Debug("Token response body: %s", string(respBody))
    return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(respBody))
  }

  // Parse response
  var tokenResp map[string]interface{}
  if err := json.Unmarshal(respBody, &tokenResp); err != nil {
    return "", fmt.Errorf("failed to parse token response: %w", err)
  }

  // The access_token field should contain the SSH certificate
  if accessToken, ok := tokenResp["access_token"].(string); ok && accessToken != "" {
    // Check if it's an SSH certificate
    // SSH certificates in base64 format start with "AAAA" (the base64-encoded certificate header)
    // They contain "ssh-rsa-cert-v01@openssh.com" when decoded, but Azure returns them base64-encoded
    if strings.HasPrefix(accessToken, "AAAA") || strings.Contains(accessToken, "ssh-rsa-cert-v01@openssh.com") {
      logger.Debug("Received SSH certificate from token endpoint!")
      return accessToken, nil
    }
    logger.Debug("Received access token (length: %d) but it's not an SSH certificate", len(accessToken))
    logger.Debug("Token starts with: %s...", accessToken[:min(100, len(accessToken))])
  }

  logger.Debug("Token response keys: %v", getMapKeys(tokenResp))
  return "", fmt.Errorf("no SSH certificate found in token response")
}

// fileCache implements the MSAL cache.ExportReplace interface for file-based caching
type fileCache struct {
  path string
}

func (f *fileCache) Export(ctx context.Context, m cache.Marshaler, h cache.ExportHints) error {
  data, err := m.Marshal()
  if err != nil {
    return err
  }
  return os.WriteFile(f.path, data, 0600)
}

func (f *fileCache) Replace(ctx context.Context, u cache.Unmarshaler, h cache.ReplaceHints) error {
  data, err := os.ReadFile(f.path)
  if err != nil {
    if os.IsNotExist(err) {
      return nil // Cache doesn't exist yet, that's okay
    }
    return err
  }
  return u.Unmarshal(data)
}

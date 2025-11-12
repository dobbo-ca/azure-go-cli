package bastion

import (
  "context"
  "crypto/tls"
  "encoding/json"
  "fmt"
  "io"
  "net"
  "net/http"
  "net/url"
  "strings"
  "time"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore"
  "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/cdobbyn/azure-go-cli/pkg/logger"
  "github.com/gorilla/websocket"
)

// TokenResponse represents the response from the token exchange API
type TokenResponse struct {
  AuthToken      string `json:"authToken"`
  NodeID         string `json:"nodeId"`
  WebSocketToken string `json:"websocketToken"`
}

// Tunnel opens a tunnel to a target resource through Azure Bastion
func Tunnel(ctx context.Context, bastionName, resourceGroup, targetResourceID string, resourcePort, localPort int) error {
  fmt.Printf("Opening tunnel through Bastion %s...\n", bastionName)
  fmt.Printf("Local port: %d\n", localPort)
  fmt.Printf("Target: %s:%d\n", targetResourceID, resourcePort)

  // Get Azure credentials
  logger.Debug("Getting Azure credentials...")
  cred, err := azure.GetCredential()
  if err != nil {
    return fmt.Errorf("failed to get credentials: %w", err)
  }
  logger.Debug("Successfully obtained credentials")

  // Get subscription ID
  logger.Debug("Getting subscription ID...")
  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }
  logger.Debug("Subscription ID: %s", subscriptionID)

  // Get Bastion details
  logger.Debug("Creating Bastion client for resource group: %s", resourceGroup)
  client, err := armnetwork.NewBastionHostsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create bastion client: %w", err)
  }

  logger.Debug("Retrieving Bastion host details: %s", bastionName)
  bastion, err := client.Get(ctx, resourceGroup, bastionName, nil)
  if err != nil {
    return fmt.Errorf("failed to get bastion host: %w", err)
  }

  if bastion.Properties == nil || bastion.Properties.DNSName == nil {
    return fmt.Errorf("bastion DNS name not found")
  }

  bastionEndpoint := *bastion.Properties.DNSName
  fmt.Printf("Bastion endpoint: %s\n", bastionEndpoint)
  logger.Debug("Bastion DNS name: %s", bastionEndpoint)

  // Get access token
  logger.Debug("Acquiring Azure AD access token...")
  token, err := getAccessToken(ctx, cred)
  if err != nil {
    return fmt.Errorf("failed to get access token: %w", err)
  }
  logger.Debug("Access token acquired (length: %d)", len(token))

  // Start local TCP listener
  logger.Debug("Starting TCP listener on 127.0.0.1:%d", localPort)
  listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
  if err != nil {
    return fmt.Errorf("failed to start local listener: %w", err)
  }
  defer listener.Close()

  fmt.Printf("Tunnel ready at 127.0.0.1:%d\n", localPort)
  fmt.Println("Press Ctrl+C to close the tunnel")
  logger.Debug("Tunnel listener ready, waiting for connections...")

  // Track WebSocket token for reuse in subsequent connections
  // First connection uses empty token, subsequent connections pass previous token
  var currentWSToken string

  // Accept connections and forward them through WebSocket
  for {
    conn, err := listener.Accept()
    if err != nil {
      if ctx.Err() != nil {
        logger.Debug("Context cancelled, stopping listener")
        return nil
      }
      logger.Debug("Error accepting connection: %v", err)
      fmt.Printf("Error accepting connection: %v\n", err)
      continue
    }

    logger.Debug("Accepted new connection from %s", conn.RemoteAddr())

    // Get fresh WebSocket token for this connection
    // Pass previous token (empty for first connection)
    logger.Debug("Exchanging token for WebSocket tunnel token...")
    wsToken, nodeID, err := exchangeTokenWithPrevious(bastionEndpoint, token, targetResourceID, resourcePort, currentWSToken)
    if err != nil {
      logger.Debug("Failed to exchange token: %v", err)
      conn.Close()
      continue
    }
    logger.Debug("WebSocket token acquired (length: %d)", len(wsToken))
    logger.Debug("Node ID: %s", nodeID)

    // Update current token for next connection
    currentWSToken = wsToken

    go handleConnection(ctx, conn, bastionEndpoint, wsToken, nodeID)
  }
}

// getAccessToken retrieves an Azure AD access token
func getAccessToken(ctx context.Context, cred azcore.TokenCredential) (string, error) {
  // Get token for Azure Resource Manager
  token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
    Scopes: []string{"https://management.azure.com/.default"},
  })
  if err != nil {
    return "", err
  }
  return token.Token, nil
}

// exchangeTokenWithPrevious exchanges an Azure AD token for a WebSocket token and node ID
// For the first connection, pass empty string as previousWSToken
// For subsequent connections, pass the previous WebSocket token
func exchangeTokenWithPrevious(bastionEndpoint, accessToken, targetResourceID string, resourcePort int, previousWSToken string) (string, string, error) {
  // Prepare form data payload (not JSON!)
  // Azure Bastion API expects application/x-www-form-urlencoded
  formData := url.Values{}
  formData.Set("resourceId", targetResourceID)
  formData.Set("protocol", "tcptunnel")
  formData.Set("workloadHostPort", fmt.Sprintf("%d", resourcePort))
  formData.Set("aztoken", accessToken)
  formData.Set("token", previousWSToken) // Empty for first connection, previous token for subsequent

  logger.Debug("Token exchange request payload:")
  logger.Debug("  resourceId: %s", targetResourceID)
  logger.Debug("  protocol: tcptunnel")
  logger.Debug("  workloadHostPort: %d", resourcePort)
  logger.Debug("  aztoken: <redacted> (length: %d)", len(accessToken))
  if previousWSToken == "" {
    logger.Debug("  token: <empty - first connection>")
  } else {
    logger.Debug("  token: <previous token> (length: %d)", len(previousWSToken))
  }

  // Create HTTPS request
  tokenURL := fmt.Sprintf("https://%s/api/tokens", bastionEndpoint)
  logger.Debug("Token exchange URL: %s", tokenURL)
  req, err := http.NewRequest("POST", tokenURL, strings.NewReader(formData.Encode()))
  if err != nil {
    return "", "", fmt.Errorf("failed to create request: %w", err)
  }

  // Set form content type (not JSON)
  req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

  // Send request
  logger.Debug("Sending token exchange request...")
  client := &http.Client{
    Timeout: 30 * time.Second,
  }
  resp, err := client.Do(req)
  if err != nil {
    return "", "", fmt.Errorf("failed to send request: %w", err)
  }
  defer resp.Body.Close()

  logger.Debug("Token exchange response status: %d", resp.StatusCode)

  if resp.StatusCode != http.StatusOK {
    body, _ := io.ReadAll(resp.Body)
    logger.Debug("Token exchange error response: %s", string(body))
    return "", "", fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
  }

  // Parse response
  var tokenResp TokenResponse
  if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
    return "", "", fmt.Errorf("failed to decode response: %w", err)
  }

  logger.Debug("Token exchange successful")
  logger.Debug("  authToken length: %d", len(tokenResp.AuthToken))
  logger.Debug("  nodeId: %s", tokenResp.NodeID)
  logger.Debug("  websocketToken length: %d", len(tokenResp.WebSocketToken))

  // Return both websocketToken and nodeID
  return tokenResp.WebSocketToken, tokenResp.NodeID, nil
}

// handleConnection handles a single TCP connection by forwarding it through WebSocket
func handleConnection(ctx context.Context, tcpConn net.Conn, bastionEndpoint, wsToken, nodeID string) {
  defer tcpConn.Close()

  logger.Debug("Starting WebSocket connection for %s", tcpConn.RemoteAddr())

  // Establish WebSocket connection
  // For Standard/Premium SKU: wss://{bastion}/webtunnelv2/{wsToken}?X-Node-Id={nodeID}
  wsURL := fmt.Sprintf("wss://%s/webtunnelv2/%s?X-Node-Id=%s", bastionEndpoint, wsToken, nodeID)
  logger.Debug("WebSocket URL: wss://%s/webtunnelv2/<redacted>?X-Node-Id=%s", bastionEndpoint, nodeID)

  dialer := websocket.Dialer{
    TLSClientConfig: &tls.Config{
      MinVersion: tls.VersionTLS12,
    },
    HandshakeTimeout: 30 * time.Second,
  }

  logger.Debug("Dialing WebSocket...")
  wsConn, resp, err := dialer.DialContext(ctx, wsURL, nil)
  if err != nil {
    logger.Debug("WebSocket connection failed: %v", err)
    if resp != nil {
      logger.Debug("HTTP Response Status: %d %s", resp.StatusCode, resp.Status)
      body, _ := io.ReadAll(resp.Body)
      logger.Debug("HTTP Response Body: %s", string(body))
      resp.Body.Close()
    }
    fmt.Printf("Failed to establish WebSocket connection: %v\n", err)
    return
  }
  defer wsConn.Close()
  logger.Debug("WebSocket connection established")

  // Create channels for errors
  errChan := make(chan error, 2)

  // TCP -> WebSocket
  go func() {
    buf := make([]byte, 32*1024)
    for {
      n, err := tcpConn.Read(buf)
      if err != nil {
        if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
          logger.Debug("TCP read error: %v", err)
          errChan <- fmt.Errorf("TCP read error: %w", err)
        }
        return
      }

      logger.Debug("Read %d bytes from TCP, forwarding to WebSocket", n)
      if err := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
        logger.Debug("WebSocket write error: %v", err)
        errChan <- fmt.Errorf("WebSocket write error: %w", err)
        return
      }
      logger.Debug("Wrote %d bytes to WebSocket", n)
    }
  }()

  // WebSocket -> TCP
  go func() {
    for {
      _, message, err := wsConn.ReadMessage()
      if err != nil {
        if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
          logger.Debug("WebSocket read error: %v", err)
          errChan <- fmt.Errorf("WebSocket read error: %w", err)
        }
        return
      }

      logger.Debug("Read %d bytes from WebSocket, forwarding to TCP", len(message))
      if _, err := tcpConn.Write(message); err != nil {
        logger.Debug("TCP write error: %v", err)
        errChan <- fmt.Errorf("TCP write error: %w", err)
        return
      }
      logger.Debug("Wrote %d bytes to TCP", len(message))
    }
  }()

  // Wait for either goroutine to finish or context to be cancelled
  select {
  case err := <-errChan:
    if err != nil {
      fmt.Printf("Connection error: %v\n", err)
    }
  case <-ctx.Done():
    return
  }
}

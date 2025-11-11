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
  cred, err := azure.GetCredential()
  if err != nil {
    return fmt.Errorf("failed to get credentials: %w", err)
  }

  // Get subscription ID
  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  // Get Bastion details
  client, err := armnetwork.NewBastionHostsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create bastion client: %w", err)
  }

  bastion, err := client.Get(ctx, resourceGroup, bastionName, nil)
  if err != nil {
    return fmt.Errorf("failed to get bastion host: %w", err)
  }

  if bastion.Properties == nil || bastion.Properties.DNSName == nil {
    return fmt.Errorf("bastion DNS name not found")
  }

  bastionEndpoint := *bastion.Properties.DNSName
  fmt.Printf("Bastion endpoint: %s\n", bastionEndpoint)

  // Get access token
  token, err := getAccessToken(ctx, cred)
  if err != nil {
    return fmt.Errorf("failed to get access token: %w", err)
  }

  // Exchange token for WebSocket token
  wsToken, err := exchangeToken(bastionEndpoint, token, targetResourceID, resourcePort)
  if err != nil {
    return fmt.Errorf("failed to exchange token: %w", err)
  }

  // Start local TCP listener
  listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
  if err != nil {
    return fmt.Errorf("failed to start local listener: %w", err)
  }
  defer listener.Close()

  fmt.Printf("Tunnel ready at 127.0.0.1:%d\n", localPort)
  fmt.Println("Press Ctrl+C to close the tunnel")

  // Accept connections and forward them through WebSocket
  for {
    conn, err := listener.Accept()
    if err != nil {
      if ctx.Err() != nil {
        return nil
      }
      fmt.Printf("Error accepting connection: %v\n", err)
      continue
    }

    go handleConnection(ctx, conn, bastionEndpoint, wsToken)
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

// exchangeToken exchanges an Azure AD token for a WebSocket token
func exchangeToken(bastionEndpoint, accessToken, targetResourceID string, resourcePort int) (string, error) {
  // Prepare form data payload (not JSON!)
  // Azure Bastion API expects application/x-www-form-urlencoded
  formData := url.Values{}
  formData.Set("resourceId", targetResourceID)
  formData.Set("protocol", "tcptunnel")
  formData.Set("workloadHostPort", fmt.Sprintf("%d", resourcePort))
  formData.Set("aztoken", accessToken)
  formData.Set("token", "") // Empty for first connection

  // Create HTTPS request
  tokenURL := fmt.Sprintf("https://%s/api/tokens", bastionEndpoint)
  req, err := http.NewRequest("POST", tokenURL, strings.NewReader(formData.Encode()))
  if err != nil {
    return "", fmt.Errorf("failed to create request: %w", err)
  }

  // Set form content type (not JSON)
  req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

  // Send request
  client := &http.Client{
    Timeout: 30 * time.Second,
  }
  resp, err := client.Do(req)
  if err != nil {
    return "", fmt.Errorf("failed to send request: %w", err)
  }
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusOK {
    body, _ := io.ReadAll(resp.Body)
    return "", fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
  }

  // Parse response
  var tokenResp TokenResponse
  if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
    return "", fmt.Errorf("failed to decode response: %w", err)
  }

  // Return the websocketToken field
  return tokenResp.WebSocketToken, nil
}

// handleConnection handles a single TCP connection by forwarding it through WebSocket
func handleConnection(ctx context.Context, tcpConn net.Conn, bastionEndpoint, wsToken string) {
  defer tcpConn.Close()

  // Establish WebSocket connection
  wsURL := fmt.Sprintf("wss://%s/api/tunnel?token=%s", bastionEndpoint, wsToken)

  dialer := websocket.Dialer{
    TLSClientConfig: &tls.Config{
      MinVersion: tls.VersionTLS12,
    },
    HandshakeTimeout: 30 * time.Second,
  }

  wsConn, _, err := dialer.DialContext(ctx, wsURL, nil)
  if err != nil {
    fmt.Printf("Failed to establish WebSocket connection: %v\n", err)
    return
  }
  defer wsConn.Close()

  // Create channels for errors
  errChan := make(chan error, 2)

  // TCP -> WebSocket
  go func() {
    buf := make([]byte, 32*1024)
    for {
      n, err := tcpConn.Read(buf)
      if err != nil {
        if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
          errChan <- fmt.Errorf("TCP read error: %w", err)
        }
        return
      }

      if err := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
        errChan <- fmt.Errorf("WebSocket write error: %w", err)
        return
      }
    }
  }()

  // WebSocket -> TCP
  go func() {
    for {
      _, message, err := wsConn.ReadMessage()
      if err != nil {
        if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
          errChan <- fmt.Errorf("WebSocket read error: %w", err)
        }
        return
      }

      if _, err := tcpConn.Write(message); err != nil {
        errChan <- fmt.Errorf("TCP write error: %w", err)
        return
      }
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

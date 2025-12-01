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

// BufferConfig contains buffer size configuration for WebSocket tunnels
type BufferConfig struct {
  // Connection-level buffer sizes (for underlying network I/O)
  ConnReadBufferSize  int // Default: 8KB
  ConnWriteBufferSize int // Default: 8KB
  // Streaming chunk buffer sizes (for application-level read/write operations)
  ChunkReadBufferSize  int // Default: 4KB
  ChunkWriteBufferSize int // Default: 4KB
}

// DefaultBufferConfig returns conservative defaults that work with Azure Bastion
func DefaultBufferConfig() BufferConfig {
  return BufferConfig{
    ConnReadBufferSize:   32 * 1024,  // 32KB
    ConnWriteBufferSize:  32 * 1024,  // 32KB
    ChunkReadBufferSize:  8 * 1024,   // 8KB
    ChunkWriteBufferSize: 8 * 1024,   // 8KB
  }
}

// TunnelSSH opens an SSH tunnel with optional username for AAD authentication
func TunnelSSH(ctx context.Context, bastionName, resourceGroup, targetResourceID string, localPort int, username string, bufferConfig BufferConfig) error {
  return tunnelWithProtocol(ctx, bastionName, resourceGroup, targetResourceID, 22, localPort, "tcptunnel", username, bufferConfig)
}

// Tunnel opens a tunnel to a target resource through Azure Bastion
func Tunnel(ctx context.Context, bastionName, resourceGroup, targetResourceID string, resourcePort, localPort int, bufferConfig BufferConfig) error {
  return tunnelWithProtocol(ctx, bastionName, resourceGroup, targetResourceID, resourcePort, localPort, "tcptunnel", "", bufferConfig)
}

// tunnelWithProtocol opens a tunnel with specific protocol and optional username
func tunnelWithProtocol(ctx context.Context, bastionName, resourceGroup, targetResourceID string, resourcePort, localPort int, protocol, username string, bufferConfig BufferConfig) error {
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

  // Only show tunnel ready messages in debug mode
  // In SSH mode, these messages are suppressed to avoid interfering with SSH I/O
  logger.Debug("Tunnel ready at 127.0.0.1:%d", localPort)
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
      continue
    }

    logger.Debug("Accepted new connection from %s", conn.RemoteAddr())

    // Get fresh WebSocket token for this connection
    // Pass previous token (empty for first connection)
    logger.Debug("Exchanging token for WebSocket tunnel token...")
    wsToken, nodeID, err := exchangeTokenWithProtocol(bastionEndpoint, token, targetResourceID, resourcePort, currentWSToken, protocol, username)
    if err != nil {
      logger.Debug("Failed to exchange token: %v", err)
      conn.Close()
      continue
    }
    logger.Debug("WebSocket token acquired (length: %d)", len(wsToken))
    logger.Debug("Node ID: %s", nodeID)

    // Update current token for next connection
    currentWSToken = wsToken

    go handleConnection(ctx, conn, bastionEndpoint, wsToken, nodeID, bufferConfig)
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
  return exchangeTokenWithProtocol(bastionEndpoint, accessToken, targetResourceID, resourcePort, previousWSToken, "tcptunnel", "")
}

// exchangeTokenWithProtocol exchanges tokens with specific protocol and optional username
func exchangeTokenWithProtocol(bastionEndpoint, accessToken, targetResourceID string, resourcePort int, previousWSToken, protocol, username string) (string, string, error) {
  // Prepare form data payload (not JSON!)
  // Azure Bastion API expects application/x-www-form-urlencoded
  formData := url.Values{}
  formData.Set("resourceId", targetResourceID)
  formData.Set("protocol", protocol)
  formData.Set("workloadHostPort", fmt.Sprintf("%d", resourcePort))
  formData.Set("aztoken", accessToken)
  formData.Set("token", previousWSToken) // Empty for first connection, previous token for subsequent

  logger.Debug("Token exchange request payload:")
  logger.Debug("  resourceId: %s", targetResourceID)
  logger.Debug("  protocol: %s", protocol)
  logger.Debug("  workloadHostPort: %d", resourcePort)
  logger.Debug("  aztoken: <redacted> (length: %d)", len(accessToken))
  if hostname := formData.Get("hostname"); hostname != "" {
    logger.Debug("  hostname: %s", hostname)
  }
  if username != "" {
    logger.Debug("  workloadUsername: %s", username)
  }
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
func handleConnection(ctx context.Context, tcpConn net.Conn, bastionEndpoint, wsToken, nodeID string, bufferConfig BufferConfig) {
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
    ReadBufferSize:   bufferConfig.ConnReadBufferSize,
    WriteBufferSize:  bufferConfig.ConnWriteBufferSize,
  }

  logger.Debug("WebSocket Dialer config: ReadBuffer=%d, WriteBuffer=%d",
    bufferConfig.ConnReadBufferSize, bufferConfig.ConnWriteBufferSize)

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

  // Set up WebSocket keepalive with ping/pong
  // Azure Bastion requires periodic activity to keep connections alive
  // Send ping every 30 seconds to prevent 2-hour idle timeout
  keepaliveCtx, cancelKeepalive := context.WithCancel(ctx)
  defer cancelKeepalive()

  // Configure pong handler to reset read deadline
  wsConn.SetPongHandler(func(appData string) error {
    logger.Debug("Received pong from server")
    // Reset read deadline on pong receipt - gives us another interval
    wsConn.SetReadDeadline(time.Now().Add(90 * time.Second))
    return nil
  })

  // Start keepalive goroutine
  go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
      select {
      case <-keepaliveCtx.Done():
        logger.Debug("Keepalive goroutine stopped")
        return
      case <-ticker.C:
        logger.Debug("Sending WebSocket ping keepalive")
        // Set write deadline for ping
        if err := wsConn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
          logger.Debug("Failed to set write deadline for ping: %v", err)
          return
        }
        if err := wsConn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
          logger.Debug("Failed to send ping: %v", err)
          return
        }
        logger.Debug("Ping sent successfully")
      }
    }
  }()

  // Set initial read deadline
  wsConn.SetReadDeadline(time.Now().Add(90 * time.Second))

  // Create channels for errors and error tracking
  errChan := make(chan error, 2)
  var lastError error
  var errorCount int
  errorThreshold := 3 // Only display errors after they occur 3 times consecutively

  // TCP -> WebSocket (streaming with NextWriter, chunked for message boundaries)
  go func() {
    buf := make([]byte, bufferConfig.ChunkWriteBufferSize)
    logger.Debug("TCP->WebSocket: chunk write buffer=%d bytes, conn write buffer=%d bytes",
      bufferConfig.ChunkWriteBufferSize, bufferConfig.ConnWriteBufferSize)
    for {
      n, err := tcpConn.Read(buf)
      if err != nil {
        if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
          logger.Debug("TCP read error: %v", err)
          errChan <- fmt.Errorf("TCP read error: %w", err)
        }
        return
      }

      logger.Debug("Read %d bytes from TCP, streaming to WebSocket", n)

      // Use NextWriter for each chunk to create message boundaries for HTTP/2
      writer, err := wsConn.NextWriter(websocket.BinaryMessage)
      if err != nil {
        logger.Debug("WebSocket NextWriter error: %v", err)
        errChan <- fmt.Errorf("WebSocket write error: %w", err)
        return
      }

      if _, err := writer.Write(buf[:n]); err != nil {
        logger.Debug("WebSocket writer.Write error: %v", err)
        writer.Close()
        errChan <- fmt.Errorf("WebSocket write error: %w", err)
        return
      }

      if err := writer.Close(); err != nil {
        logger.Debug("WebSocket writer.Close error: %v", err)
        errChan <- fmt.Errorf("WebSocket write error: %w", err)
        return
      }

      logger.Debug("Streamed %d bytes to WebSocket", n)
    }
  }()

  // WebSocket -> TCP (streaming with NextReader, chunked for message boundaries)
  go func() {
    buf := make([]byte, bufferConfig.ChunkReadBufferSize)
    logger.Debug("WebSocket->TCP: chunk read buffer=%d bytes, conn read buffer=%d bytes",
      bufferConfig.ChunkReadBufferSize, bufferConfig.ConnReadBufferSize)
    for {
      // Use NextReader for streaming reads that support message fragmentation
      messageType, reader, err := wsConn.NextReader()
      if err != nil {
        if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
          logger.Debug("WebSocket NextReader error: %v", err)
          errChan <- fmt.Errorf("WebSocket read error: %w", err)
        }
        return
      }

      if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
        logger.Debug("Ignoring non-data message type: %d", messageType)
        continue
      }

      // Stream data from WebSocket to TCP in chunks
      totalBytes := 0
      for {
        n, err := reader.Read(buf)
        if n > 0 {
          logger.Debug("Read %d bytes from WebSocket reader, forwarding to TCP", n)
          if _, writeErr := tcpConn.Write(buf[:n]); writeErr != nil {
            logger.Debug("TCP write error: %v", writeErr)
            errChan <- fmt.Errorf("TCP write error: %w", writeErr)
            return
          }
          totalBytes += n
        }

        if err == io.EOF {
          // End of message
          logger.Debug("Completed streaming %d total bytes from WebSocket to TCP", totalBytes)
          break
        }

        if err != nil {
          logger.Debug("WebSocket reader error: %v", err)
          errChan <- fmt.Errorf("WebSocket read error: %w", err)
          return
        }
      }
    }
  }()

  // Wait for either goroutine to finish or context to be cancelled
  for {
    select {
    case err := <-errChan:
      if err != nil {
        // Check if this is the same error as before
        errMsg := err.Error()
        if lastError != nil && lastError.Error() == errMsg {
          errorCount++
        } else {
          // New error type, reset counter
          lastError = err
          errorCount = 1
        }

        // Only display error if it's persistent (occurred multiple times)
        if errorCount >= errorThreshold {
          fmt.Printf("Persistent connection error: %v\n", err)
          return
        }
        // For transient errors, just log debug and continue
        logger.Debug("Transient error (count: %d/%d): %v", errorCount, errorThreshold, err)
      }
    case <-ctx.Done():
      return
    }
  }
}

// extractHostnameFromResourceID extracts the VM/resource name from an Azure resource ID
// Example: /subscriptions/.../resourceGroups/.../providers/Microsoft.Compute/virtualMachines/myvm
// Returns: myvm
func extractHostnameFromResourceID(resourceID string) string {
  parts := strings.Split(resourceID, "/")
  if len(parts) > 0 {
    // The last part is the resource name
    return parts[len(parts)-1]
  }
  return ""
}

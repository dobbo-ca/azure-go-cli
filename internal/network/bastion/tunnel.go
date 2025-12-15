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
	"sync"
	"syscall"
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
		ConnReadBufferSize:   32 * 1024, // 32KB
		ConnWriteBufferSize:  32 * 1024, // 32KB
		ChunkReadBufferSize:  8 * 1024,  // 8KB
		ChunkWriteBufferSize: 8 * 1024,  // 8KB
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

		// Handle each connection in its own goroutine for true concurrency
		go func(tcpConn net.Conn) {
			// Get fresh WebSocket token for this connection
			// IMPORTANT: Always pass empty string to get independent tokens
			// Passing a previous token causes Azure to link/reuse sessions
			logger.Debug("Exchanging token for WebSocket tunnel token...")

			wsToken, nodeID, err := exchangeTokenWithProtocol(bastionEndpoint, token, targetResourceID, resourcePort, "", protocol, username)
			if err != nil {
				logger.Debug("Failed to exchange token: %v", err)
				tcpConn.Close()
				return
			}
			logger.Debug("WebSocket token acquired (length: %d)", len(wsToken))
			logger.Debug("Node ID: %s", nodeID)

			handleConnection(ctx, tcpConn, bastionEndpoint, wsToken, nodeID, bufferConfig)
		}(conn)
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

	// Set TCP_NODELAY on the underlying connection (disable Nagle's algorithm)
	// This matches Azure CLI behavior for lower latency
	netDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			var setErr error
			err := c.Control(func(fd uintptr) {
				// Set TCP_NODELAY (platform-specific implementation)
				setErr = setTCPNoDelay(fd)
			})
			if err != nil {
				return err
			}
			return setErr
		},
	}

	dialer := websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			return netDialer.Dial(network, addr)
		},
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

	// NOTE: Azure CLI does NOT implement WebSocket ping/pong keepalive
	// They rely on the SSH protocol's own traffic to keep the connection alive
	// Adding explicit pings can interfere with data flow and cause connection resets

	// Create write mutex to protect WebSocket writes
	// gorilla/websocket requires that only one goroutine writes at a time
	var writeMu sync.Mutex

	// Create channels for error and completion signaling
	errChan := make(chan error, 2)
	doneChan := make(chan struct{}, 2) // Signals when a goroutine completes

	// TCP -> WebSocket (streaming with NextWriter, chunked for message boundaries)
	go func() {
		defer func() {
			doneChan <- struct{}{}
		}()

		buf := make([]byte, bufferConfig.ChunkWriteBufferSize)
		logger.Debug("TCP->WebSocket: chunk write buffer=%d bytes, conn write buffer=%d bytes",
			bufferConfig.ChunkWriteBufferSize, bufferConfig.ConnWriteBufferSize)
		for {
			n, err := tcpConn.Read(buf)
			if err != nil {
				if err == io.EOF || strings.Contains(err.Error(), "use of closed network connection") {
					logger.Debug("TCP connection closed normally")
				} else {
					logger.Debug("TCP read error: %v", err)
					errChan <- fmt.Errorf("TCP read error: %w", err)
				}
				return
			}

			logger.Debug("Read %d bytes from TCP, streaming to WebSocket", n)

			// Lock before writing to WebSocket (required by gorilla/websocket)
			// Use WriteMessage instead of NextWriter for better performance
			writeMu.Lock()
			err = wsConn.WriteMessage(websocket.BinaryMessage, buf[:n])
			writeMu.Unlock()

			if err != nil {
				logger.Debug("WebSocket write error: %v", err)
				errChan <- fmt.Errorf("WebSocket write error: %w", err)
				return
			}

			logger.Debug("Streamed %d bytes to WebSocket", n)
		}
	}()

	// WebSocket -> TCP (streaming with NextReader, chunked for message boundaries)
	go func() {
		defer func() {
			doneChan <- struct{}{}
		}()

		buf := make([]byte, bufferConfig.ChunkReadBufferSize)
		logger.Debug("WebSocket->TCP: chunk read buffer=%d bytes, conn read buffer=%d bytes",
			bufferConfig.ChunkReadBufferSize, bufferConfig.ConnReadBufferSize)
		for {
			// Use NextReader for streaming reads that support message fragmentation
			messageType, reader, err := wsConn.NextReader()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					logger.Debug("WebSocket closed normally")
				} else {
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

	// Wait for either:
	// 1. An error from either goroutine
	// 2. Both goroutines to complete
	// 3. Context to be cancelled
	completedGoroutines := 0
	for {
		select {
		case err := <-errChan:
			// Fatal error occurred, close connection and return
			logger.Debug("Connection error, closing: %v", err)
			logger.Info("Connection closed: %v", err)
			return
		case <-doneChan:
			completedGoroutines++
			logger.Debug("Goroutine completed (%d/2)", completedGoroutines)
			if completedGoroutines >= 2 {
				// Both goroutines finished, connection is done
				logger.Debug("Both goroutines completed, connection closed")
				return
			}
		case <-ctx.Done():
			logger.Debug("Context cancelled, closing connection")
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

package bastion

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
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

	// Verify credentials work before starting listener
	logger.Debug("Verifying Azure AD credentials...")
	if _, err := getAccessToken(ctx, cred); err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}
	logger.Debug("Credentials verified")

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

	// Start health check goroutine to detect network failures even when tunnel is idle
	healthCheckInterval := 5 * time.Second
	healthCheckTimeout := 3 * time.Second
	healthErrCh := make(chan error, 1)

	go func() {
		ticker := time.NewTicker(healthCheckInterval)
		defer ticker.Stop()

		consecutiveFailures := 0
		maxFailures := 2 // Fail after 2 consecutive health check failures (~11 seconds)

		for {
			select {
			case <-ticker.C:
				// Health check: try to reach the bastion endpoint via HTTPS
				logger.Debug("Running health check to bastion endpoint...")
				healthURL := fmt.Sprintf("https://%s/api/health", bastionEndpoint)

				client := &http.Client{
					Timeout: healthCheckTimeout,
				}
				resp, err := client.Get(healthURL)
				if err != nil {
					consecutiveFailures++
					logger.Debug("Health check failed (%d/%d): %v", consecutiveFailures, maxFailures, err)
					if consecutiveFailures >= maxFailures {
						healthErrCh <- fmt.Errorf("network connectivity lost - unable to reach Azure Bastion after %d attempts", consecutiveFailures)
						return
					}
				} else {
					resp.Body.Close()
					if consecutiveFailures > 0 {
						logger.Debug("Health check succeeded after %d previous failure(s)", consecutiveFailures)
					} else {
						logger.Debug("Health check succeeded")
					}
					consecutiveFailures = 0
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	// Accept connections and forward them through WebSocket
	connAcceptCh := make(chan net.Conn, 10)
	connErrCh := make(chan error, 1)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					logger.Debug("Context cancelled, stopping listener")
					return
				}
				logger.Debug("Error accepting connection: %v", err)
				connErrCh <- err
				return
			}
			connAcceptCh <- conn
		}
	}()

	// Main loop: handle connections and health checks
	for {
		select {
		case err := <-healthErrCh:
			logger.Debug("Health check failure: %v", err)
			return fmt.Errorf("connection to Azure Bastion lost - network disconnected")

		case err := <-connErrCh:
			return err

		case conn := <-connAcceptCh:
			logger.Debug("Accepted new connection from %s", conn.RemoteAddr())

			// Handle each connection in its own goroutine for true concurrency
			go func(tcpConn net.Conn) {
				// Get fresh access token for each connection to handle token expiry
				// Azure AD tokens expire after ~1 hour; the SDK credential handles
				// caching and refresh automatically
				logger.Debug("Getting fresh access token for connection...")
				accessToken, err := getAccessToken(ctx, cred)
				if err != nil {
					logger.Debug("Failed to get access token: %v", err)
					tcpConn.Close()
					return
				}

				// Exchange for WebSocket tunnel token with retry
				// IMPORTANT: Always pass empty string to get independent tokens
				// Passing a previous token causes Azure to link/reuse sessions
				logger.Debug("Exchanging token for WebSocket tunnel token...")

				wsToken, nodeID, err := exchangeTokenWithRetry(bastionEndpoint, accessToken, targetResourceID, resourcePort, protocol, username)
				if err != nil {
					logger.Debug("Failed to exchange token after retries: %v", err)
					tcpConn.Close()
					return
				}
				logger.Debug("WebSocket token acquired (length: %d)", len(wsToken))
				logger.Debug("Node ID: %s", nodeID)

				handleConnection(ctx, tcpConn, bastionEndpoint, wsToken, nodeID, bufferConfig)
			}(conn)
		}
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

// exchangeTokenWithRetry wraps exchangeTokenWithProtocol with exponential backoff retry.
// This handles transient Azure rate limiting and network blips that would otherwise
// cause "connection reset by peer" errors in tools like k9s.
func exchangeTokenWithRetry(bastionEndpoint, accessToken, targetResourceID string, resourcePort int, protocol, username string) (string, string, error) {
	maxAttempts := 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		wsToken, nodeID, err := exchangeTokenWithProtocol(bastionEndpoint, accessToken, targetResourceID, resourcePort, "", protocol, username)
		if err == nil {
			if attempt > 1 {
				logger.Debug("Token exchange succeeded on attempt %d", attempt)
			}
			return wsToken, nodeID, nil
		}

		lastErr = err
		if attempt < maxAttempts {
			backoff := time.Duration(attempt) * time.Second
			logger.Debug("Token exchange attempt %d/%d failed: %v (retrying in %v)", attempt, maxAttempts, err, backoff)
			time.Sleep(backoff)
		}
	}

	return "", "", fmt.Errorf("token exchange failed after %d attempts: %w", maxAttempts, lastErr)
}

// isBrokenPipe returns true if the error is a broken pipe (EPIPE) or connection reset,
// which are expected during normal tunnel operation and should not be displayed to the user.
func isBrokenPipe(err error) bool {
	if errors.Is(err, syscall.EPIPE) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "broken pipe") || strings.Contains(msg, "connection reset by peer")
}

// handleConnection handles a single TCP connection by forwarding it through WebSocket
func handleConnection(ctx context.Context, tcpConn net.Conn, bastionEndpoint, wsToken, nodeID string, bufferConfig BufferConfig) {
	defer tcpConn.Close()

	logger.Debug("Starting WebSocket connection for %s", tcpConn.RemoteAddr())

	// Establish WebSocket connection
	// For Standard/Premium SKU: wss://{bastion}/webtunnelv2/{wsToken}?X-Node-Id={nodeID}
	wsURL := fmt.Sprintf("wss://%s/webtunnelv2/%s?X-Node-Id=%s", bastionEndpoint, wsToken, nodeID)
	logger.Debug("WebSocket URL: wss://%s/webtunnelv2/<redacted>?X-Node-Id=%s", bastionEndpoint, nodeID)

	// Set TCP_NODELAY on the underlying connection
	netDialer := &net.Dialer{
		Timeout: 30 * time.Second,
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
		logger.Debug("Failed to establish WebSocket connection: %v", err)
		return
	}
	defer wsConn.Close()
	logger.Debug("WebSocket connection established")

	// Create write mutex to protect WebSocket writes
	// gorilla/websocket requires that only one goroutine writes at a time
	var writeMu sync.Mutex

	// Create channels for error and completion signaling
	errChan := make(chan error, 4)
	doneChan := make(chan struct{}, 3) // Signals when a goroutine completes (2 data + 1 keepalive)

	// WebSocket keepalive: send pings every 30s to keep connection alive through
	// Azure load balancers and detect dead connections via pong timeout.
	// Without this, idle WebSocket connections get silently dropped by Azure
	// infrastructure, causing k9s and other tools with background polling to hang.
	pingInterval := 30 * time.Second
	pongTimeout := 60 * time.Second
	lastPong := time.Now()
	var pongMu sync.Mutex

	wsConn.SetPongHandler(func(appData string) error {
		pongMu.Lock()
		lastPong = time.Now()
		pongMu.Unlock()
		logger.Debug("Received pong from server")
		return nil
	})

	// Keepalive goroutine: sends pings and checks for pong responses
	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()

	go func() {
		defer func() {
			doneChan <- struct{}{}
		}()

		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check if we've received a pong recently
				pongMu.Lock()
				sincePong := time.Since(lastPong)
				pongMu.Unlock()

				if sincePong > pongTimeout {
					logger.Debug("WebSocket dead: no pong received in %v", sincePong)
					errChan <- fmt.Errorf("WebSocket keepalive timeout: no pong in %v", sincePong)
					return
				}

				// Send ping - WriteControl is safe to call concurrently with other writes
				err := wsConn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
				if err != nil {
					if isBrokenPipe(err) {
						logger.Debug("WebSocket ping failed (broken pipe)")
					} else {
						logger.Debug("WebSocket ping failed: %v", err)
						errChan <- fmt.Errorf("WebSocket ping error: %w", err)
					}
					return
				}
				logger.Debug("Sent ping to server")

			case <-connCtx.Done():
				return
			}
		}
	}()

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
				if err == io.EOF || strings.Contains(err.Error(), "use of closed network connection") || isBrokenPipe(err) {
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
				if isBrokenPipe(err) {
					logger.Debug("WebSocket write closed (broken pipe)")
				} else {
					logger.Debug("WebSocket write error: %v", err)
					errChan <- fmt.Errorf("WebSocket write error: %w", err)
				}
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
						if isBrokenPipe(writeErr) {
							logger.Debug("TCP write closed (broken pipe)")
						} else {
							logger.Debug("TCP write error: %v", writeErr)
							errChan <- fmt.Errorf("TCP write error: %w", writeErr)
						}
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
	// 1. An error from any goroutine
	// 2. Both data goroutines to complete (keepalive stops via connCancel)
	// 3. Context to be cancelled
	dataGoroutinesDone := 0
	for {
		select {
		case err := <-errChan:
			// Connection error occurred, close and return
			// Demoted to Debug to avoid corrupting TUI output (e.g., k9s)
			logger.Debug("Connection error, closing: %v", err)
			return
		case <-doneChan:
			dataGoroutinesDone++
			logger.Debug("Goroutine completed (%d/3)", dataGoroutinesDone)
			if dataGoroutinesDone >= 2 {
				// Both data goroutines finished, connection is done
				// Keepalive goroutine will stop via deferred connCancel()
				logger.Debug("Data goroutines completed, connection closed")
				return
			}
		case <-connCtx.Done():
			logger.Debug("Connection context cancelled, closing")
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

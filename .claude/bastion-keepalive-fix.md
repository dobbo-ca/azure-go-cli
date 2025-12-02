# Bastion Connection Timeout Fix

## Problem

Bastion tunnel connections were timing out after approximately 2 hours, while the official Azure CLI did not experience this issue.

## Root Cause

The issue was **not** related to Azure AD token expiration, but rather to **WebSocket connection keepalive**.

The WebSocket connections established to Azure Bastion were not sending any keepalive (ping/pong) messages. Without periodic activity:
- Network equipment (proxies, load balancers, NAT devices) may drop idle connections
- Azure Bastion's infrastructure may terminate inactive WebSocket connections
- The default timeout appears to be around 2 hours

## Solution

Implemented WebSocket keepalive using the standard ping/pong mechanism:

### Changes Made to `internal/network/bastion/tunnel.go`

1. **Ping Interval**: Send WebSocket ping frames every 30 seconds
2. **Pong Handler**: Configured to acknowledge pong responses from server
3. **No Deadlines**: Deliberately avoid read/write deadlines to prevent interference with data flow
4. **Graceful Shutdown**: Keepalive goroutine properly cancels when connection closes

### Implementation Details

```go
// Keepalive goroutine sends ping every 30 seconds
ticker := time.NewTicker(30 * time.Second)

// Pong handler (simple acknowledgment, no deadline manipulation)
wsConn.SetPongHandler(func(appData string) error {
    logger.Debug("Received pong from server")
    return nil
})

// Send ping using WriteControl (doesn't interfere with data flow)
wsConn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
```

**Important**: We do NOT set read deadlines on the WebSocket connection. Setting deadlines would interfere with normal data flow and cause operations (like k9s actions) to hang. The ping/pong mechanism alone is sufficient for keepalive.

### Why This Works

1. **Active Connection**: Ping/pong frames prove the connection is active
2. **Bidirectional**: Server responses (pongs) confirm server is reachable
3. **Network Traversal**: Keeps NAT entries and proxy connections alive
4. **Standard Protocol**: Uses WebSocket RFC 6455 control frames
5. **Minimal Overhead**: Ping frames are tiny (typically 2-6 bytes)

## Testing Recommendations

To verify the fix works for connections longer than 2 hours:

```bash
# Start a bastion SSH tunnel
./bin/az/az network bastion ssh \
  --name <bastion-name> \
  --resource-group <resource-group> \
  --target-resource-id <vm-resource-id> \
  --auth-type password

# Leave the connection idle for 2+ hours
# Connection should remain active with ping/pong keepalive messages visible in debug logs
```

Enable debug logging to see keepalive activity:
```bash
export AZURE_CLI_DEBUG=1
```

## Comparison with Official Azure CLI

The official Azure CLI (Python) includes WebSocket keepalive by default in its WebSocket library (`websockets` or similar). Our Go implementation now matches this behavior using the `gorilla/websocket` library's built-in ping/pong support.

## Related Files

- `internal/network/bastion/tunnel.go` - Main implementation
- `internal/network/bastion/ssh.go` - SSH wrapper (uses tunnel.go)

## Additional Notes

- Keepalive applies to **all existing connections**, not just new ones
- Works for both SSH tunnels and raw TCP tunnels
- Minimal performance impact (ping every 30s is negligible)
- Properly cleaned up when connection closes (no goroutine leaks)

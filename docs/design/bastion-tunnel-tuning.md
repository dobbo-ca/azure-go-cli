# Bastion tunnel engine: buffer flags, keepalive and retry

## Purpose

`az network bastion tunnel`, `az network bastion ssh` and `az aks bastion` all
run the same in-process WebSocket tunnel (`internal/network/bastion/tunnel.go`).
It adds three things the Python azure-cli bastion extension does not have:

- Four buffer-size flags.
- A per-connection WebSocket ping/pong keepalive.
- Retry of the token exchange.

The Python extension (`azext_bastion`) `tunnel` takes `--name`, `--resource-group`,
`--target-resource-id`/`--target-ip-address`, `--resource-port`, `--port` and
`--timeout`, with no buffer options (`_params.py`). The Go tunnel has no
`--timeout` or `--target-ip-address`. Python's `tunnel.py` has no ping,
keepalive or retry logic, only a plain `ws_socket.recv()`. Buffer tuning is
therefore Go-only; the keepalive and retry exist to keep long-lived interactive
tools (k9s, kubectl watches) working through Azure's load balancers.

## Flags

Defined per command, same names and defaults on each:

| Flag | Default | Unit |
|------|---------|------|
| `--conn-read-buffer` | 32 | KB |
| `--conn-write-buffer` | 32 | KB |
| `--chunk-read-buffer` | 8 | KB |
| `--chunk-write-buffer` | 8 | KB |

- `internal/network/bastion/commands.go`: `tunnel` (flags at lines 46-49) and `ssh` (91-94).
- `internal/aks/commands.go`: `aks bastion` (155-158).

Each RunE multiplies the KB value by 1024 into `bastion.BufferConfig`
(`tunnel.go`) and passes it down. `BufferConfig` carries the values as bytes.

## Data flow

```
az network bastion tunnel/ssh, az aks bastion
   |  BufferConfig
   v
tunnelWithProtocol (tunnel.go)
   |- BastionHosts.Get -> DNSName
   |- getAccessToken (verify creds before listening)
   |- net.Listen 127.0.0.1:<port>
   |- health goroutine: GET https://<bastion>/api/health every 5s
   v
accept loop -> one goroutine per TCP connection
   |- getAccessToken (fresh ARM token per connection)
   |- exchangeTokenWithRetry -> POST https://<bastion>/api/tokens
   v
handleConnection
   |- dial wss://<bastion>/webtunnelv2/<wsToken>?X-Node-Id=<nodeID>
   |- keepalive goroutine  (ping 30s, pong deadline 60s)
   |- TCP -> WS goroutine  (buffer = ChunkWriteBufferSize)
   `- WS -> TCP goroutine  (buffer = ChunkReadBufferSize)
```

Callers: `Tunnel` (tunnel command, `aks bastion` with port 443 and the cluster ID)
and `TunnelSSH` (ssh command, resource port 22, ephemeral local port 49152-65535
from `ssh.go`). Both go through `tunnelWithProtocol`.

## Buffer flags

- `ConnReadBufferSize` / `ConnWriteBufferSize` are set on the gorilla
  `websocket.Dialer` (`ReadBufferSize` / `WriteBufferSize`). They size the
  connection's buffered-I/O read/write buffers (not OS socket buffers).
- `ChunkReadBufferSize` / `ChunkWriteBufferSize` size the `[]byte` used per
  connection in the WS->TCP (`NextReader` + `Read`) and TCP->WS (`Read` +
  `WriteMessage`) loops. The chunk write size is therefore also the maximum
  WebSocket message size sent upstream.
- `TCP_NODELAY` is set on the dialed socket via the `Dialer.Control` hook
  (`setTCPNoDelay` in `tunnel_unix.go` / `tunnel_windows.go`).
- The `BufferConfig` struct comments still say "Default: 8KB / 4KB"; the real
  defaults are the flag defaults above. The struct comments are stale.

## Token exchange and retry

- A new ARM access token and a new WebSocket token are obtained for every
  accepted TCP connection (`tunnelWithProtocol`, connection goroutine), so
  tokens expiring after ~1 hour do not break a long-lived listener.
- Python also exchanges a new token per accepted client. The difference: the Go
  exchange always sends an empty `token` form field and no `X-Node-Id` header,
  so sessions are not linked. Python sends the previous token and node ID. The
  code comment records that passing a previous token makes Azure link or reuse
  sessions.
- `exchangeTokenWithRetry` makes up to 3 attempts. It sleeps `attempt` seconds
  between them (1s, then 2s), a linear backoff, not a doubling one. Every
  failure is retried, including non-transient ones (e.g. 401); the HTTP client
  timeout is 30s per attempt. After the last failure it returns
  `token exchange failed after 3 attempts: ...`.
- On failure after retries, the TCP connection is closed silently (only logged
  at debug level), so the client sees a reset.

## Keepalive

In `handleConnection`, one goroutine per connection:

- Ticker every 30s (`pingInterval`).
- On each tick: if `time.Since(lastPong) > 60s` (`pongTimeout`), it sends an
  error to `errChan` and the connection is torn down. Otherwise it sends a ping
  with `WriteControl` (10s deadline), which is safe alongside the data writer.
- The pong handler updates `lastPong` under a mutex. `lastPong` starts at the
  connection start time.
- Pong is only processed while the WS->TCP goroutine is calling `NextReader`,
  which it always is, so no separate read loop is needed.
- A broken-pipe ping error ends the keepalive goroutine without reporting an error.

Reason: idle WebSocket connections were being silently dropped by Azure
infrastructure, hanging tools with background polling (comment in `tunnel.go`).

Related, separate mechanism: the listener-level health goroutine polls
`https://<bastion>/api/health` every 5s (3s timeout). Two consecutive failures
end the whole tunnel with "connection to Azure Bastion lost - network
disconnected".

## Teardown

`handleConnection` returns on the first error in `errChan`, when both data
goroutines finish, or on context cancel. Deferred `connCancel`, `wsConn.Close`
and `tcpConn.Close` stop the others. All connection errors are logged at debug
level only, to avoid corrupting TUI output (k9s, ssh).

## Edge cases and limitations

- Flag values are not validated. A negative chunk size reaches
  `make([]byte, n)` and panics; zero makes both copy loops spin on 0-byte reads
  (the connection hangs and burns CPU). Conn size 0 uses gorilla's 4096
  default. A negative conn read size is raised to 125 bytes, and a negative
  conn write size falls back to 4096.
- The pong timeout is 60s with a 30s tick, so a dead connection is detected
  after 60-90s.
- Keepalive and retry values are constants, not flags.
- No automatic WebSocket reconnect: when a WebSocket dies, the local TCP
  connection is closed. A new token is exchanged only when the client opens a
  new connection (kubectl does this per request; an ssh session just ends).
- Retry covers only the token exchange, not the WebSocket dial.
- Same engine on Windows and Unix; only `setTCPNoDelay` differs by build tag.

## Tests

There are no unit tests for the tunnel engine, flags, keepalive or retry.
The only test in the package is `internal/network/bastion/aad_test.go` (AAD
helpers). Verify manually with `--debug`-level logging (`logger.Debug` output
shows buffer sizes, ping/pong and retry attempts; not available for
`az network bastion ssh`, which turns off debug logging once ssh starts).

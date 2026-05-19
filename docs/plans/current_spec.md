# Orthrus Agent Docker Proxy Listener — Feature Spec (PR 5)

**Branch**: `feature/hecate`
**PR**: #5 (Orthrus Docker Proxy — server-side listener)
**Date**: 2026-05-18
**Status**: Ready for implementation

---

## 1. Introduction

### Overview

Orthrus agents connect to Charon over a persistent WebSocket multiplexed with yamux. The agent binary (`agent/leash/leash.go`) already handles an incoming yamux stream whose first byte is `0x01` (`streamTypeDocker`) by connecting to its local `/var/run/docker.sock` and bidirectionally copying. The server-side half of this tunnel was deferred with the comment "PR 5".

This spec covers the complete implementation:

1. **Server-side proxy listener** — when an agent session is registered, allocate an ephemeral `127.0.0.1:0` TCP listener; for each accepted connection, open a yamux stream to the agent, write `0x01`, then bidirectionally copy.
2. **`DockerHandler` integration** — when a `RemoteServer` has `connection_type == "orthrus"`, resolve the agent's proxy address and use `tcp://127.0.0.1:<port>` as the Docker host instead of the server's `Host:Port` fields.

### Objectives

1. `GetProxyAddr()` returns a non-empty address for every live agent session.
2. Docker container listing via `GET /api/v1/docker/containers?server_id=<uuid>` works for Orthrus-backed remote servers.
3. Clear 502/503 errors when the agent is offline or the orthrus subsystem is unavailable.
4. All changes are covered by unit tests; an integration test stub is provided.

---

## 2. Research Findings

### 2.1 `backend/internal/orthrus/session.go`

**`AgentSession` struct** (relevant fields):

```go
type AgentSession struct {
    agentUUID string
    agentName string
    conn      *websocket.Conn
    session   *yamux.Session
    cancel    context.CancelFunc
    proxyPort int        // 0 means no proxy listener yet (allocated in PR 5)
    mu        sync.Mutex
}
```

**`GetProxyAddr()`** already returns the correct format, updated to use `s.listener != nil` as the sentinel for consistency with the idempotency guard:

```go
func (s *AgentSession) GetProxyAddr() string {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.listener == nil {
        return ""
    }
    return fmt.Sprintf("127.0.0.1:%d", s.proxyPort)
}
```

An existing coverage test (`session_coverage_test.go`) manually sets `sess.proxyPort = 8080` and asserts `GetProxyAddr()` returns `"127.0.0.1:8080"` — the integer field remains the source of truth for the port value; `listener` is the guard that determines whether a proxy is active.

**`Close()`** cancels the context and closes the yamux session (which also closes the underlying WebSocket). The listener allocated in PR 5 must be closed here.

**`IsAlive()`** returns `!s.session.IsClosed()`.

### 2.2 `backend/internal/orthrus/server.go`

**`OrthrusServer`** stores sessions in a `sync.Map` keyed by `agentUUID → *AgentSession`.

**`HandleWebSocket()`** — the connection lifecycle:
1. Authenticates the bearer token (bcrypt compare against all agents).
2. Calls `NewAgentSession(uuid, name, conn)` to create the yamux server session.
3. Stores in `sessions`.
4. Updates the agent's `status = online` in the DB.
5. Launches `watchHeartbeat` goroutine.

`StartDockerProxy()` (to be added) must be called between steps 2 and 3.

**`watchHeartbeat()`** polls `sess.IsAlive()` on a ticker. When false, it calls `markOffline` and `s.sessions.Delete(agentUUID)` — but does **not** call `sess.Close()`. This means the listener goroutine would outlive the session. The fix: call `sess.Close()` in `watchHeartbeat` when `!sess.IsAlive()`.

**`GetProxyAddr(agentUUID string) (string, bool)`** and **`GetSession(agentUUID string) (*AgentSession, bool)`** are already implemented as stubs and will work correctly once `proxyPort` is set.

**`Stop()`** already calls `sess.Close()` for all sessions — listener cleanup will be covered once `Close()` is updated.

### 2.3 `backend/internal/api/handlers/docker_handler.go`

**Current struct**:

```go
type DockerHandler struct {
    dockerService       dockerContainerLister
    remoteServerService remoteServerGetter
}
```

**Current `ListContainers` flow for `server_id`**:

```go
server, err := h.remoteServerService.GetByUUID(serverID)
// ...
host = fmt.Sprintf("tcp://%s:%d", server.Host, server.Port)
```

No awareness of `ConnectionType` or `OrthrusAgentUUID`. No reference to `OrthrusServer`.

### 2.4 `backend/internal/api/routes/routes.go`

**Critical wiring gap**: `orthrusServer` is created inside `if strings.TrimSpace(os.Getenv("CHARON_ENCRYPTION_KEY")) != "" { ... }`. The `dockerHandler` is created **after** this block closes:

```go
if encryptionKey != "" {
    // ...
    orthrusServer, _ = orthrus.NewOrthrusServer(db, orthrusCA)
    // orthrusServer registered with api and caddyManager here
}
// dockerHandler created here — orthrusServer is OUT OF SCOPE
dockerService := services.NewDockerService()
dockerHandler := handlers.NewDockerHandler(dockerService, remoteServerService)
dockerHandler.RegisterRoutes(management)
```

Fix: hoist `var orthrusServer *orthrus.OrthrusServer` declaration above the `if` block, then call `dockerHandler.SetOrthrusResolver(orthrusServer)` (may be nil) after the handler is created.

### 2.5 `backend/internal/services/docker_service.go`

**`ListContainers(ctx, host string)`**: when `host` is neither empty nor `"local"`, creates a new `*client.Client` with `client.WithHost(host)`. A `tcp://127.0.0.1:<port>` value is a valid host string — no changes needed to `docker_service.go`.

### 2.6 `backend/internal/models/remote_server.go`

```go
type ConnectionType string

const (
    ConnectionTypeDirect     ConnectionType = "direct"
    ConnectionTypeOrthrus    ConnectionType = "orthrus"
    ConnectionTypeCloudflare ConnectionType = "cloudflare"
    ConnectionTypeTailscale  ConnectionType = "tailscale"
    ConnectionTypeNetbird    ConnectionType = "netbird"
    ConnectionTypeZerotier   ConnectionType = "zerotier"
)

type RemoteServer struct {
    // ...
    ConnectionType   ConnectionType `json:"connection_type" gorm:"default:'direct';index"`
    OrthrusAgentUUID *string        `json:"orthrus_agent_uuid,omitempty" gorm:"index"`
    // ...
}
```

All six values are enumerated here for completeness. Only `orthrus` uses the proxy path; the `default:` branch in `ListContainers` handles all remaining types (`direct`, `cloudflare`, `tailscale`, `netbird`, `zerotier`) correctly by falling through to the `tcp://host:port` construction.

### 2.7 `agent/leash/leash.go` — Agent-side protocol (already implemented)

```go
const (
    streamTypeDocker      = byte(0x01)
    streamTypePortForward = byte(0x02)
)
```

**`handleStream(stream *yamux.Stream)`**: reads 1 type byte, dispatches to `handleDockerStream` for `0x01`.

**`handleDockerStream(stream *yamux.Stream)`**: calls `l.filter.ServeProxy(l.dockerSock, stream, stream)` — the `muzzle.Filter` proxies the stream to the local Docker socket, applying the allowlist filter.

**Server-side requirement**: open a yamux stream (`s.session.Open()`), write `[]byte{0x01}`, then bidirectionally copy between the TCP connection and the yamux stream.

### 2.8 Existing Tests

- `session_coverage_test.go` directly accesses `sess.proxyPort` (unexported). The new `StartDockerProxy()` sets this field — existing test remains valid.
- `docker_handler_test.go` uses `fakeDockerService` and `fakeRemoteServerService`. New tests will add a `fakeOrthrusResolver`.
- `server_test.go` uses a real SQLite DB and real WebSocket pairs. New tests will exercise the proxy listener start/stop lifecycle.

---

## 3. Technical Specifications

### 3.1 New Constant

**File**: `backend/internal/orthrus/session.go`

```go
// streamTypeDocker is the yamux stream type byte for Docker socket proxy traffic.
// Must match streamTypeDocker in the Orthrus agent (agent/leash/leash.go).
const streamTypeDocker = byte(0x01)
```

### 3.2 `AgentSession` Struct Changes

**File**: `backend/internal/orthrus/session.go`

Add one field to `AgentSession`:

```go
type AgentSession struct {
    agentUUID string
    agentName string
    conn      *websocket.Conn
    session   *yamux.Session
    cancel    context.CancelFunc
    proxyPort int          // ephemeral port; 0 until StartDockerProxy succeeds
    listener  net.Listener // nil until StartDockerProxy succeeds
    mu        sync.Mutex
}
```

**`GetProxyAddr()` sentinel updated** — the nil-check changes from `s.proxyPort == 0` to `s.listener == nil` for consistency with the idempotency guard introduced in `StartDockerProxy()`. The integer `s.proxyPort` field continues to hold the ephemeral port value and is still set atomically alongside `s.listener`.

### 3.3 `StartDockerProxy()` Method

**File**: `backend/internal/orthrus/session.go`

```go
// StartDockerProxy allocates an ephemeral TCP listener on localhost and starts
// accepting connections. Each accepted connection is proxied to the agent's
// Docker socket via a new yamux stream with stream type 0x01.
// Returns an error if the listener cannot be bound or if the proxy has already
// been started for this session (idempotency guard).
func (s *AgentSession) StartDockerProxy() error {
    s.mu.Lock()
    if s.listener != nil {
        s.mu.Unlock()
        return fmt.Errorf("orthrus: docker proxy already started for session %s", s.agentUUID)
    }
    s.mu.Unlock()

    ln, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        return fmt.Errorf("orthrus: allocate proxy listener: %w", err)
    }

    port := ln.Addr().(*net.TCPAddr).Port

    s.mu.Lock()
    s.listener = ln
    s.proxyPort = port
    s.mu.Unlock()

    go s.runProxyListener(ln)
    return nil
}
```

### 3.4 `runProxyListener()` and `proxyConn()` Methods

**File**: `backend/internal/orthrus/session.go`

```go
// runProxyListener accepts TCP connections and proxies each to the agent's
// Docker socket via a new yamux stream. Exits when the listener is closed.
func (s *AgentSession) runProxyListener(ln net.Listener) {
    for {
        conn, err := ln.Accept()
        if err != nil {
            // Listener closed — normal shutdown.
            return
        }
        go s.proxyConn(conn)
    }
}

// proxyConn opens a yamux stream for Docker proxy traffic, writes the type byte,
// then bidirectionally copies until either side closes.
func (s *AgentSession) proxyConn(conn net.Conn) {
    defer conn.Close()

    stream, err := s.session.Open()
    if err != nil {
        // yamux session already closed; nothing to proxy.
        return
    }
    defer stream.Close()

    if _, err := stream.Write([]byte{streamTypeDocker}); err != nil {
        return
    }

    var wg sync.WaitGroup
    wg.Add(2)
    go func() {
        defer wg.Done()
        defer stream.Close()
        io.Copy(stream, conn) //nolint:errcheck
    }()
    go func() {
        defer wg.Done()
        defer conn.Close()
        io.Copy(conn, stream) //nolint:errcheck
    }()
    wg.Wait()
}
```

### 3.5 `Close()` Changes

**File**: `backend/internal/orthrus/session.go`

Close the listener in addition to the yamux session:

```go
func (s *AgentSession) Close() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.cancel()

    if s.listener != nil {
        _ = s.listener.Close()
        s.listener = nil
    }

    return s.session.Close()
}
```

### 3.6 `server.go` — `HandleWebSocket` Changes

**File**: `backend/internal/orthrus/server.go`

After creating the session and before storing it, start the proxy listener:

```go
session, err := NewAgentSession(agent.UUID, agent.Name, conn)
if err != nil {
    logger.Log().WithError(err).Error("orthrus: create agent session failed")
    _ = conn.Close()
    return
}

if err := session.StartDockerProxy(); err != nil {
    logger.Log().WithField("uuid", util.SanitizeForLog(agent.UUID)).
        WithError(err).Warn("orthrus: failed to start docker proxy listener — Docker tunneling unavailable for this session")
    // Non-fatal: session still registered; GetProxyAddr() returns "" -> Docker handler returns 502
}

s.sessions.Store(agent.UUID, session)
```

### 3.7 `server.go` — `watchHeartbeat` Changes

**File**: `backend/internal/orthrus/server.go`

Call `sess.Close()` when the yamux session is found dead to ensure the listener goroutine exits:

```go
if !sess.IsAlive() {
    _ = sess.Close() // closes listener goroutine; idempotent
    s.markOffline(agentUUID)
    s.sessions.Delete(agentUUID)
    return
}
```

### 3.8 `docker_handler.go` — New Interface and Field

**File**: `backend/internal/api/handlers/docker_handler.go`

Add interface and field:

```go
// orthrusProxyResolver resolves the local TCP proxy address for a connected Orthrus agent.
// The address is in "host:port" form, suitable for use as tcp://host:port with Docker.
type orthrusProxyResolver interface {
    GetProxyAddr(agentUUID string) (string, bool)
}

type DockerHandler struct {
    dockerService       dockerContainerLister
    remoteServerService remoteServerGetter
    orthrusResolver     orthrusProxyResolver // nil when Orthrus subsystem is unavailable
}
```

**`NewDockerHandler` signature is unchanged.** Add a setter following the existing `caddyManager.SetOrthrusServer` pattern:

```go
// SetOrthrusResolver configures the Orthrus proxy resolver for Docker tunneling.
// If never called (or called with nil), requests for Orthrus-backed servers
// return 503 Service Unavailable.
func (h *DockerHandler) SetOrthrusResolver(r orthrusProxyResolver) {
    h.orthrusResolver = r
}
```

### 3.9 `docker_handler.go` — `ListContainers` Logic

**File**: `backend/internal/api/handlers/docker_handler.go`

Replace the single `host = fmt.Sprintf(...)` line in the `serverID != ""` branch with connection-type-aware logic:

```go
if serverID != "" {
    server, err := h.remoteServerService.GetByUUID(serverID)
    if err != nil {
        log.WithFields(map[string]any{"server_id": util.SanitizeForLog(serverID)}).Warn("remote server not found")
        c.JSON(http.StatusNotFound, gin.H{"error": "Remote server not found"})
        return
    }

    switch server.ConnectionType {
    case models.ConnectionTypeOrthrus:
        if h.orthrusResolver == nil {
            c.JSON(http.StatusServiceUnavailable, gin.H{
                "error":   "Orthrus subsystem unavailable",
                "details": "The Orthrus agent tunnel service is not running. Ensure CHARON_ENCRYPTION_KEY is set.",
            })
            return
        }
        if server.OrthrusAgentUUID == nil || *server.OrthrusAgentUUID == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Remote server has no linked Orthrus agent"})
            return
        }
        addr, ok := h.orthrusResolver.GetProxyAddr(*server.OrthrusAgentUUID)
        if !ok {
            log.WithFields(map[string]any{"agent_uuid": util.SanitizeForLog(*server.OrthrusAgentUUID)}).Warn("orthrus agent not connected")
            c.JSON(http.StatusBadGateway, gin.H{
                "error":   "Orthrus agent is not currently connected",
                "details": "The Orthrus agent for this server is offline. Please ensure the agent is running and connected to Charon.",
            })
            return
        }
        host = "tcp://" + addr // e.g. tcp://127.0.0.1:54321

    default:
        // Direct, Tailscale, NetBird, ZeroTier, Cloudflare — use explicit host/port
        host = fmt.Sprintf("tcp://%s:%d", server.Host, server.Port)
    }
}
```

### 3.10 `routes.go` — Hoist `orthrusServer` and Wire Resolver

**File**: `backend/internal/api/routes/routes.go`

1. Hoist the variable declaration before the encryption-key `if` block:

```go
// Declared here so dockerHandler (created below the if-block) can access it.
var orthrusServer *orthrus.OrthrusServer
```

2. Remove the `var orthrusServer *orthrus.OrthrusServer` declaration inside the `if` block (it becomes an assignment only).

3. After `dockerHandler` is created (outside the block), wire the resolver:

```go
dockerService := services.NewDockerService()
dockerHandler := handlers.NewDockerHandler(dockerService, remoteServerService)
if orthrusServer != nil {
    dockerHandler.SetOrthrusResolver(orthrusServer)
}
dockerHandler.RegisterRoutes(management)
```

---

## 4. Data Flow

### 4.1 Happy Path — Orthrus Agent Connected

```
Client
  → GET /api/v1/docker/containers?server_id=<uuid>
  → DockerHandler.ListContainers
      → remoteServerService.GetByUUID(uuid)
          → RemoteServer{connection_type: "orthrus", orthrus_agent_uuid: "agent-abc"}
      → h.orthrusResolver.GetProxyAddr("agent-abc")
          → AgentSession.proxyPort = 54321
          → returns ("127.0.0.1:54321", true)
      → host = "tcp://127.0.0.1:54321"
      → dockerService.ListContainers(ctx, "tcp://127.0.0.1:54321")
          → Docker client dials 127.0.0.1:54321
          → TCP connection accepted by AgentSession.runProxyListener
          → AgentSession.proxyConn:
              → session.Open() → new yamux stream
              → stream.Write([]byte{0x01})
              → io.Copy(stream, conn) / io.Copy(conn, stream)
          → yamux stream arrives at agent
          → agent.handleDockerStream:
              → filter.ServeProxy("/var/run/docker.sock", stream, stream)
              → HTTP request forwarded to local Docker API
          → Docker API response returns through tunnel
      → []DockerContainer returned
  → 200 OK JSON
```

### 4.2 Agent Disconnected — 502

```
GET /api/v1/docker/containers?server_id=<uuid>
→ server.ConnectionType == "orthrus"
→ h.orthrusResolver.GetProxyAddr("agent-abc") → ("", false)
→ 502 Bad Gateway {"error": "Orthrus agent is not currently connected", ...}
```

### 4.3 Orthrus Subsystem Unavailable (no encryption key) — 503

```
GET /api/v1/docker/containers?server_id=<uuid>
→ server.ConnectionType == "orthrus"
→ h.orthrusResolver == nil
→ 503 Service Unavailable {"error": "Orthrus subsystem unavailable", ...}
```

### 4.4 Session Disconnect — Listener Cleanup

```
watchHeartbeat: sess.IsAlive() == false
  → sess.Close()
      → s.listener.Close()       ← runProxyListener exits Accept() loop
      → s.session.Close()
  → markOffline(agentUUID)
  → s.sessions.Delete(agentUUID)
```

> **Async goroutine completion**: `proxyConn` goroutines launched from `runProxyListener` complete asynchronously after `Close()` returns. They are bounded by the time to flush in-flight `io.Copy` calls, which is safe because closing the yamux session terminates all streams, causing the in-flight `io.Copy` on the stream side to return promptly.

---

## 5. Error Handling and Edge Cases

| Scenario | Where Detected | Response |
|----------|---------------|----------|
| Agent not connected | `GetProxyAddr` returns `("", false)` | `ListContainers` → 502 Bad Gateway |
| Orthrus subsystem unavailable | `h.orthrusResolver == nil` | `ListContainers` → 503 Service Unavailable |
| `OrthrusAgentUUID` is nil/empty on the server record | `server.OrthrusAgentUUID == nil` check | `ListContainers` → 400 Bad Request |
| Port allocation failure on connect | `net.Listen` returns error | `HandleWebSocket` logs warning; session registered with `proxyPort == 0`; Docker returns 502 |
| Agent disconnects mid-request | `io.Copy` returns error | `proxyConn` exits; Docker client gets connection reset; `ListContainers` returns error → 503 |
| yamux stream open failure | `session.Open()` returns error | `proxyConn` returns; TCP conn closed; Docker client retries or fails |
| Concurrent `Close()` calls | `s.mu.Lock()` in `Close()`, `listener = nil` after close | Idempotent; second call is a no-op for the listener |
| `StartDockerProxy` called a second time | `s.listener != nil` guard under mutex | Returns error; caller logs and skips; no new listener |

---

## 6. Files Changed

| File | Change Type | Summary |
|------|-------------|---------|
| `backend/internal/orthrus/session.go` | Modify | Add `streamTypeDocker` constant, `listener net.Listener` field, `StartDockerProxy()`, `runProxyListener()`, `proxyConn()`, update `Close()` |
| `backend/internal/orthrus/server.go` | Modify | Call `session.StartDockerProxy()` in `HandleWebSocket`; call `sess.Close()` in `watchHeartbeat` cleanup |
| `backend/internal/api/handlers/docker_handler.go` | Modify | Add `orthrusProxyResolver` interface, `orthrusResolver` field, `SetOrthrusResolver()`, update `ListContainers` switch on `ConnectionType` |
| `backend/internal/api/routes/routes.go` | Modify | Hoist `var orthrusServer` declaration; call `dockerHandler.SetOrthrusResolver(orthrusServer)` |
| `backend/internal/orthrus/session_test.go` | Modify | Add proxy lifecycle tests |
| `backend/internal/orthrus/server_test.go` | Modify | Add test for proxy start on session connect |
| `backend/internal/api/handlers/docker_handler_test.go` | Modify | Add orthrus resolver tests |
| `backend/internal/orthrus/proxy_integration_test.go` | Create | Integration test stub (`//go:build integration`) |

---

## 7. Implementation Plan

### Phase 1 — Playwright Tests (UI/UX Specification)

Write Playwright tests that define expected behavior before implementation. These will fail until the backend is wired up.

**File**: `tests/docker-orthrus-proxy.spec.ts`

Tests:
1. **Agent offline** — with a remote server that has `connection_type: "orthrus"` and an agent that is offline, the Docker containers panel shows a "Orthrus agent is not currently connected" error state.
2. **No `server_id`** — local Docker containers still load normally (no regression).

These tests validate the UI error handling introduced by the new 502/503 responses.

### Phase 2 — Backend: Session Proxy Listener

**Files**: `session.go`, `server.go`

- [ ] Add `streamTypeDocker = byte(0x01)` constant to `session.go`
- [ ] Add `listener net.Listener` field to `AgentSession`
- [ ] Implement `StartDockerProxy() error`
- [ ] Implement `runProxyListener(ln net.Listener)`
- [ ] Implement `proxyConn(conn net.Conn)`
- [ ] Update `Close()` to close `s.listener` (under mutex, set to nil)
- [ ] In `HandleWebSocket`: call `session.StartDockerProxy()`, log warning on failure (non-fatal)
- [ ] In `watchHeartbeat`: call `sess.Close()` before `markOffline` when session is dead
- [ ] Unit tests:
  - `TestAgentSession_StartDockerProxy_SetsProxyAddr` — start proxy, assert `GetProxyAddr()` non-empty
  - `TestAgentSession_StartDockerProxy_AcceptsConnection` — dial the proxy addr, verify type byte written to yamux stream
  - `TestAgentSession_Close_StopsProxyListener` — start proxy, close session, verify listener no longer accepts
  - `TestAgentSession_StartDockerProxy_CalledTwice` — call twice; second call returns error containing "already started"; `GetProxyAddr()` returns same address as first call; no additional listener port allocated
  - `TestOrthrusServer_HandleWebSocket_StartsProxy` — full WebSocket handshake, assert session has non-empty proxy addr

**Validation gate**: `go test -race ./backend/internal/orthrus/...` passes.

### Phase 3 — Backend: DockerHandler Integration

**Files**: `docker_handler.go`, `routes.go`, `docker_handler_test.go`

- [ ] Add `orthrusProxyResolver` interface to `docker_handler.go`
- [ ] Add `orthrusResolver orthrusProxyResolver` field to `DockerHandler`
- [ ] Implement `SetOrthrusResolver(r orthrusProxyResolver)`
- [ ] Update `ListContainers`: replace single `fmt.Sprintf` with `switch server.ConnectionType`
- [ ] In `routes.go`: hoist `var orthrusServer` declaration; call `dockerHandler.SetOrthrusResolver(orthrusServer)`
- [ ] Unit tests:
  - `TestDockerHandler_ListContainers_OrthrusAgentConnected` — resolver returns `("127.0.0.1:54321", true)`; verify `dockerSvc.host == "tcp://127.0.0.1:54321"`
  - `TestDockerHandler_ListContainers_OrthrusAgentOffline` — resolver returns `("", false)`; verify 502
  - `TestDockerHandler_ListContainers_OrthrusSubsystemUnavailable` — `orthrusResolver == nil`; verify 503
  - `TestDockerHandler_ListContainers_OrthrusMissingAgentUUID` — server has `OrthrusAgentUUID == nil`; verify 400
  - `TestDockerHandler_SetOrthrusResolver_Nil` — explicit nil does not panic on request

**Validation gate**: `go test -race ./backend/internal/api/...` passes; no regression in existing Docker handler tests.

### Phase 4 — Integration Test Stub

**File**: `backend/internal/orthrus/proxy_integration_test.go`

```go
//go:build integration

package orthrus_test

import (
    "testing"
)

// TestDockerProxyIntegration_FullTunnel exercises the complete path:
//   TCP connection → local proxy listener → yamux stream → agent → Docker socket
// Requires a running Orthrus agent with Docker socket accessible.
func TestDockerProxyIntegration_FullTunnel(t *testing.T) {
    t.Skip("requires running Orthrus agent with /var/run/docker.sock")
}
```

**Validation gate**: `go test -tags integration ./backend/internal/orthrus/...` — skips cleanly.

### Phase 5 — Documentation

- [ ] Update `ARCHITECTURE.md` component table: add row for "Orthrus Docker Proxy Listener"
- [ ] Confirm no OpenAPI spec changes needed (existing `GET /docker/containers` endpoint; response schema unchanged)

---

## 8. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| AC-1 | `AgentSession.GetProxyAddr()` returns a non-empty `127.0.0.1:PORT` address after `StartDockerProxy()` succeeds | Unit test `TestAgentSession_StartDockerProxy_SetsProxyAddr` |
| AC-2 | A TCP connection to the proxy address results in `0x01` being written to the agent's yamux stream | Unit test `TestAgentSession_StartDockerProxy_AcceptsConnection` |
| AC-3 | Closing an `AgentSession` causes `ln.Accept()` to return an error and `runProxyListener` to exit | Unit test `TestAgentSession_Close_StopsProxyListener` |
| AC-4 | `HandleWebSocket` registers a session with a non-zero `proxyPort` | Unit test `TestOrthrusServer_HandleWebSocket_StartsProxy` |
| AC-5 | `GET /docker/containers?server_id=<orthrus-uuid>` with a connected agent passes `tcp://127.0.0.1:PORT` to the Docker client | Unit test `TestDockerHandler_ListContainers_OrthrusAgentConnected` |
| AC-6 | Same request with a disconnected agent returns HTTP 502 with `"Orthrus agent is not currently connected"` | Unit test `TestDockerHandler_ListContainers_OrthrusAgentOffline` |
| AC-7 | Same request when Orthrus subsystem is unavailable returns HTTP 503 | Unit test `TestDockerHandler_ListContainers_OrthrusSubsystemUnavailable` |
| AC-8 | `RemoteServer` with `connection_type == "orthrus"` and nil `OrthrusAgentUUID` returns HTTP 400 | Unit test `TestDockerHandler_ListContainers_OrthrusMissingAgentUUID` |
| AC-9 | No regression in existing Docker handler tests for direct connection type | `go test ./backend/internal/api/handlers/...` |
| AC-10 | No regression in existing Orthrus session/server unit tests | `go test ./backend/internal/orthrus/...` |
| AC-11 | `go test -race ./backend/...` passes with no race conditions | CI |
| AC-12 | A second call to `StartDockerProxy()` on an already-started session returns a non-nil error containing `"already started"`. `GetProxyAddr()` still returns the address from the first successful call. No additional listener port is allocated. | `TestAgentSession_StartDockerProxy_CalledTwice` |

---

## 9. Commit Slicing Strategy

**Decision**: Single PR with 3 ordered logical commits. Each commit is independently compilable and testable.

**Trigger reasons**: Cross-domain change (orthrus package + handler package + routes wiring); clear phase boundary between the tunnel mechanics (Commit 1) and the HTTP handler integration (Commit 2).

---

### Commit 1 — `feat(orthrus): implement server-side Docker proxy listener`

**Scope**: Session proxy lifecycle only. No HTTP handler changes.

**Files**:
- `backend/internal/orthrus/session.go` — constant, new field, `StartDockerProxy()`, `runProxyListener()`, `proxyConn()`, updated `Close()`
- `backend/internal/orthrus/server.go` — `StartDockerProxy()` call in `HandleWebSocket`; `sess.Close()` in `watchHeartbeat`
- `backend/internal/orthrus/session_test.go` — 3 new tests
- `backend/internal/orthrus/server_test.go` — 1 new test

**Dependencies**: None (self-contained change to orthrus package).

**Validation gate**: `go test -race ./backend/internal/orthrus/...` — all pass.

**Rollback**: Revert this commit. No other code depends on `StartDockerProxy()` until Commit 2.

---

### Commit 2 — `feat(docker): route orthrus-backed servers through local proxy`

**Scope**: `DockerHandler` orthrus awareness and routes wiring.

**Files**:
- `backend/internal/api/handlers/docker_handler.go` — interface, field, setter, updated `ListContainers`
- `backend/internal/api/routes/routes.go` — hoist var, call `SetOrthrusResolver`
- `backend/internal/api/handlers/docker_handler_test.go` — 5 new tests

**Dependencies**: Commit 1 (`AgentSession.GetProxyAddr()` must return a real address).

**Validation gate**: `go test -race ./backend/internal/api/...` — all pass; no regression in existing handler tests.

**Rollback**: Revert Commit 2. Proxy listener still runs (from Commit 1) but is never used by the Docker handler.

---

### Commit 3 — `test(orthrus): add integration test stub for Docker proxy tunnel`

**Scope**: Integration test placeholder only.

**Files**:
- `backend/internal/orthrus/proxy_integration_test.go` — `//go:build integration` stub

**Dependencies**: Commits 1 and 2.

**Validation gate**: `go test -tags integration ./backend/internal/orthrus/...` — skips cleanly.

**Rollback**: Delete the file. No functional impact.

---

## 10. Security Considerations

- **Loopback only**: The proxy listener binds `127.0.0.1:0` (never `0.0.0.0`). Only Charon's own process can dial it.
- **No port reuse**: Ephemeral OS-assigned port; no hardcoded port that could be predicted or hijacked.
- **Muzzle filter on the agent**: The agent's `handleDockerStream` applies `muzzle.Filter` (read-only Docker endpoint allowlist). Charon cannot trigger destructive Docker operations through the tunnel.
- **Session ownership**: The `agentUUID` is authenticated by bcrypt bearer token; only a legitimate agent creates a session with its UUID.
- **Listener closed on disconnect**: `Close()` closes the listener synchronously under the mutex. No dangling goroutine after session cleanup.
- **No secrets in logs**: `proxyPort` (integer) and `127.0.0.1:PORT` are safe to log. No external-facing addresses exposed.

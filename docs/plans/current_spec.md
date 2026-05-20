# Orthrus External Docker Proxy — Feature Spec

**Branch**: `feature/orthrus-external-proxy`
**Date**: 2026-05-20
**Status**: Ready for implementation
**Author**: Principal Architect

---

## 1. Introduction

### Overview

PR #1031 implemented a server-side Docker proxy listener for the Orthrus subsystem. When an agent connects over WebSocket (yamux-multiplexed), Charon binds an ephemeral `127.0.0.1:N` TCP listener and proxies Docker API traffic through the yamux session to the agent's `/var/run/docker.sock`. This loopback proxy allows Charon itself to discover remote Docker containers.

However, **other containers on the Docker network** (e.g., Dockhand) that previously pointed at a dedicated `socat`-based TCP proxy cannot reach `127.0.0.1:N` — loopback is not routable across Docker bridge networks.

This spec covers the **External Docker Proxy** extension: an optional HTTP-aware, muzzle-filtered TCP listener on `0.0.0.0:PORT` that bridges the Docker network to the Orthrus agent's Docker socket. Containers on the same Docker network reach the remote daemon via `charon:PORT` without any host port publishing.

### Objectives

1. Per-agent configurable `external_proxy_port` (0 = disabled, 1024–65535 = enabled).
2. When an agent with `external_proxy_port > 0` connects, bind `0.0.0.0:PORT` and serve HTTP-filtered (muzzle) Docker API traffic through the loopback proxy → yamux → agent.
3. API to read and write `external_proxy_port` per agent.
4. UI to configure the port and display the connection string (`tcp://charon:PORT`).
5. Graceful degradation: bind failure keeps the loopback proxy intact; agent disconnect closes the external listener immediately.
6. Full unit test coverage of new code paths.

---

## 2. Research Findings

### 2.1 Current loopback proxy (`session.go`)

`AgentSession` holds one `net.Listener` (`listener`) bound to `127.0.0.1:0`. `runProxyListener` accepts TCP connections; `proxyConn` opens a yamux stream, writes `0x01` (`streamTypeDocker`), then bidirectionally copies raw bytes. `Close()` closes the listener, cancels the context, and closes the yamux session.

**Key insight**: `proxyConn` is a raw byte forwarder — there is no HTTP parsing in the current path. `Muzzle` is an `http.Handler` wrapper that cannot intercept raw TCP bytes.

### 2.2 `OrthrusServer` (`server.go`)

`HandleWebSocket` calls `session.StartDockerProxy()` (the loopback proxy) immediately after creating the session and before storing it in `sessions`. `watchHeartbeat` polls `IsAlive()` and calls `markOffline` + `sessions.Delete` on disconnect. `watchHeartbeat` already calls `_ = sess.Close()` before `markOffline` (added in PR #1031). The `Close()` method must be **extended** in this PR to also stop the external proxy `http.Server` — this is new work, not a bug fix.

### 2.3 `Muzzle` (`muzzle.go`)

Allowlist: `GET /containers/json`, `GET /images/json`, `GET /info`, `GET /version` (after stripping `/vN.NN` prefix). All other methods and paths return `403 Forbidden`. It is an `http.Handler` and cannot intercept raw TCP.

### 2.4 `OrthrusAgent` model

No `external_proxy_port` field currently exists. The model resides in `backend/internal/models/orthrus_agent.go`. GORM AutoMigrate handles schema additions.

### 2.5 `OrthrusService.Patch`

Accepts `name, hecateTunnelUUID, deviceID, resolvedAddress *string`. The `patchAgentRequest` in the handler mirrors this. Both must be extended to carry `externalProxyPort *int`.

### 2.6 Docker bridge networking constraint

`0.0.0.0:PORT` inside the Charon container is reachable from other containers on the **same Docker network** by name (`charon:PORT`) without any `-p HOST:CONTAINER` port publishing. This is the desired behavior. Publishing the port to the host is an explicit user action with security implications (documented in §9).

---

## 3. Requirements (EARS Notation)

### 3.1 Configuration

- **R1**: WHEN an admin sends `PATCH /api/v1/orthrus/agents/:uuid` with `{"external_proxy_port": N}` where `N ∈ {0} ∪ [1024, 65535]`, THE SYSTEM SHALL persist the value to the `orthrus_agents` row.
- **R2**: WHEN `N` is outside the valid range (1–1023 or > 65535) or negative, THE SYSTEM SHALL return HTTP 400 with a descriptive error message.
- **R3**: THE SYSTEM SHALL default `external_proxy_port` to `0` for all new and migrated agents.

### 3.2 Proxy Lifecycle

- **R4**: WHEN an agent whose `external_proxy_port > 0` establishes a WebSocket connection, THE SYSTEM SHALL bind a TCP listener on `0.0.0.0:<external_proxy_port>` after the loopback proxy listener is successfully started.
- **R5**: WHEN the external proxy port bind fails (e.g., port already in use), THE SYSTEM SHALL log a warning, NOT start the external listener, AND NOT prevent the loopback proxy from operating.
- **R6**: WHEN an agent session is closed (disconnect, heartbeat timeout, or revocation), THE SYSTEM SHALL close the external proxy listener and its `http.Server` within the same `Close()` call.
- **R7**: WHEN the agent is disconnected and a client attempts to connect to `0.0.0.0:<PORT>`, THE SYSTEM SHALL refuse the TCP connection (port not listening).
- **R8**: WHEN an agent with `external_proxy_port = 0` connects, THE SYSTEM SHALL NOT bind any external listener.

### 3.3 Request Filtering

- **R9**: WHEN a client connects to the external proxy port and sends a Docker API request, THE SYSTEM SHALL apply muzzle filtering before forwarding any bytes to the agent.
- **R10**: WHEN a client sends a request to a non-allowlisted path or uses a non-GET method, THE SYSTEM SHALL return HTTP 403 and SHALL NOT open a yamux stream.
- **R11**: WHEN a client sends a valid muzzle-allowlisted GET request, THE SYSTEM SHALL forward it through the loopback proxy → yamux → agent's Docker socket and return the response to the client.

### 3.4 Concurrency

- **R12**: WHEN multiple clients connect simultaneously to the external proxy port, THE SYSTEM SHALL handle each connection concurrently in independent goroutines.
- **R13**: WHEN an agent reconnects after disconnect, THE SYSTEM SHALL successfully re-bind the same `external_proxy_port` (because the old listener was closed by `Close()`).

### 3.5 Status API

- **R14**: WHEN an admin calls `GET /api/v1/orthrus/agents/:uuid/proxy-status`, THE SYSTEM SHALL return the live external proxy state: configured port, bound address (if active), and whether the listener is currently active.

### 3.6 UI

- **R15**: WHEN viewing an Orthrus agent in the management UI, THE SYSTEM SHALL display an "External Docker Proxy" section with a port input field and a status indicator.
- **R16**: WHEN the external proxy is active, THE SYSTEM SHALL prominently display the connection string `tcp://charon:<PORT>` for copy-paste use.

---

## 4. Architecture Decision Records

### ADR-01: Port Assignment Strategy

**Decision**: Store `external_proxy_port int` on the `OrthrusAgent` model. Port `0` means disabled. Users set any value in `[1024, 65535]`. Ephemeral assignment is explicitly rejected.

**Context**: Dockhand and similar tools must know the port in advance to configure their Docker host. Ephemeral ports change on every Charon restart and every agent reconnect, making them unworkable for static container configuration.

**Options**:

- A. Ephemeral-and-stored: Assign a random port on first agent connect, persist it. Complex — requires write-back from runtime to DB; race conditions during initial connect.
- B. User-configured fixed port: User chooses the port, stored in DB. Simplest, most operationally predictable. **SELECTED**.
- C. Range-allocated pool: Charon manages a pool. Unnecessary complexity.

**Rationale**: Option B aligns with how operators configure Docker proxy endpoints (`socat` to a fixed port, `dockerd --host tcp://0.0.0.0:2375`). The value persists across Charon restarts. The agent reconnect scenario is clean: the port was freed when the old session closed, so rebinding succeeds.

**Impact**: DB migration adds one `int` column. No schema complexities.

---

### ADR-02: Bind Address

**Decision**: Always bind `0.0.0.0:<PORT>` for the external proxy. Not configurable in this release.

**Context**: The entire motivation is that containers on the Docker bridge network need to reach `charon:PORT`. A loopback-only bind defeats this purpose entirely.

**Options**:

- A. Loopback `127.0.0.1`: Useless for the stated goal.
- B. `0.0.0.0` (all interfaces): Reachable from Docker network peers. Also reachable from the Docker host and potentially external networks if the port is published. **SELECTED**.
- C. Docker bridge interface IP (e.g., `172.17.0.X`): Restricts to Docker network only but requires runtime inspection of container network config — unacceptable complexity.

**Rationale**: Option B is simple and correct. The Docker bridge network is an internal network. Exposure to external networks only occurs via explicit user `-p PORT:PORT` publishing.

**Security Mitigations**: Muzzle filtering (R9–R10); UI warning; §9 documentation.

---

### ADR-03: Muzzle Integration Architecture

**Decision**: The external proxy is an `http.Server` fronted by `Muzzle`, reverse-proxying to the loopback `127.0.0.1:<loopback_port>`. The raw TCP tunnel layer (`proxyConn`) remains unchanged.

**Proxy chain**:

```
Docker client → net/http.Server (0.0.0.0:PORT)
  → Muzzle (http.Handler — allowlist gate)
    → httputil.ReverseProxy (Director rewrites Host to 127.0.0.1:loopbackPort)
      → runProxyListener (existing loopback)
        → proxyConn (yamux stream + streamTypeDocker byte)
          → agent /var/run/docker.sock
```

**Why not raw TCP forwarding?** Muzzle cannot filter raw TCP bytes; HTTP parsing is mandatory for allowlist enforcement.

**Why not parse HTTP in `proxyConn`?** That would muzzle Charon's own loopback Docker access, adding complexity to a path that works correctly today.

**Streaming support**: `httputil.ReverseProxy.FlushInterval = -1` (flush immediately) handles Docker log/event streaming. `http.Server.WriteTimeout = 0` prevents premature termination of long-lived streaming responses. `ReadHeaderTimeout = 10s` guards against Slowloris.

**Double-filtering**: Muzzle applied at Charon (external entry) AND at agent (existing leash code). Defense in depth.

---

### ADR-04: Bind Timing

**Decision**: Bind eagerly on agent connect, same pattern as the loopback proxy.

**Rationale**: Consistent observable state — the port is either listening or not; there is no transient window. If bind fails, the error is stored and surfaced via the status endpoint.

---

### ADR-05: Port Change During Active Session

**Decision**: Port changes take effect on the **next agent connection**. Running sessions are not dynamically reconfigured.

**Rationale**: Dynamic reconfiguration requires stopping the `http.Server` while HTTP connections may be in flight, adding locking complexity. Administrative port changes are rare.

**UI signal**: Display live active port (from `/proxy-status`) alongside configured port. When they differ: *"Changes will take effect on next agent reconnect."*

---

### ADR-06: `external_proxy_port` Model Location

**Decision**: Store on `OrthrusAgent`, not `RemoteServer`.

**Rationale**: The external proxy is a property of the agent's Docker socket exposure. Multiple `RemoteServer` records can reference the same `OrthrusAgent`; the proxy port should be configured once. Storing it on `OrthrusAgent` maintains cohesion between the port and the session lifecycle that owns the listener.

---

### ADR-07: `Close()` Must Cover the External Proxy Server

**Decision**: Extend `AgentSession.Close()` to shut down the `extServer` (`http.Server`) and its `extListener` in addition to the existing loopback listener teardown.

**Context**: `watchHeartbeat` already calls `sess.Close()` (PR #1031). When the external proxy is added, `Close()` must also call `extServer.Close()` to stop the HTTP server and free the external listener port. Without this, the port would remain bound and refuse re-binding on agent reconnect (R13).

**No change required to `watchHeartbeat` itself** — it already calls `sess.Close()` before `markOffline`.

---

## 5. Database Schema

### 5.1 Change: `orthrus_agents` table

Add one column:

| Column | Type | Default | Notes |
|--------|------|---------|-------|
| `external_proxy_port` | `INTEGER` | `0` | `0` = disabled; `1024`–`65535` = external proxy port |

**GORM field**:

```go
// ExternalProxyPort is the TCP port bound on 0.0.0.0 for inter-container Docker API access.
// 0 = disabled. Valid range: 1024–65535.
ExternalProxyPort int `json:"external_proxy_port" gorm:"default:0"`
```

**Migration**: GORM AutoMigrate adds the column automatically at startup. Existing rows default to `0` (disabled). No explicit migration script required.

### 5.2 No new tables

---

## 6. Backend Implementation Plan

### 6.1 `backend/internal/models/orthrus_agent.go`

Add `ExternalProxyPort` field after `ResolvedAddress`:

```go
// ExternalProxyPort is the TCP port bound on 0.0.0.0 for inter-container Docker API access.
// 0 = disabled. Valid values: 1024–65535.
ExternalProxyPort int `json:"external_proxy_port" gorm:"default:0"`
```

---

### 6.2 `backend/internal/orthrus/session.go`

#### New fields on `AgentSession`

```go
extServer    *http.Server  // nil if external proxy disabled/not started
extListener  net.Listener  // nil until StartExternalProxy succeeds
extProxyPort int           // the port arg passed to StartExternalProxy; 0 if not started
extErr       error         // last bind error; nil on success
```

New imports required: `"net/http"`, `"net/http/httputil"`, `"net/url"`, `"time"`, `"errors"`.

#### New method: `StartExternalProxy(port int) error`

Signature:

```go
func (s *AgentSession) StartExternalProxy(port int) error
```

Behaviour:

1. `port == 0` → return nil immediately (no-op; R8).
2. Lock `s.mu`. If `s.proxyPort == 0` → return "loopback proxy not started" error.
3. If `s.extListener != nil` → return "already started" error.
4. If `s.session.IsClosed()` → return "closed session" error.
5. Read `loopbackPort = s.proxyPort`. Unlock.
6. `net.Listen("tcp", "0.0.0.0:PORT")`. On failure: lock, store `extErr`, unlock, return error (R5).
7. Create `httputil.ReverseProxy` targeting `http://127.0.0.1:loopbackPort`; `FlushInterval = -1`.
8. Create `http.Server{Handler: NewMuzzle(rp), ReadHeaderTimeout: 10s}`.
9. Lock. Double-check `s.extListener == nil` (concurrent call guard). Store `extListener`, `extServer`, `extProxyPort`. Unlock.
10. `go srv.Serve(ln)`.
11. Log at INFO: "orthrus: external docker proxy started".
12. Return nil.

#### New method: `GetExternalProxyStatus() ExternalProxyStatus`

Returns a snapshot of `extProxyPort`, `extListener.Addr()`, `extListener != nil`, `extErr`.

New exported type `ExternalProxyStatus`:

```go
type ExternalProxyStatus struct {
    ConfiguredPort int    `json:"configured_port"` // value passed to StartExternalProxy
    ActivePort     int    `json:"active_port"`     // 0 if not active
    BindAddress    string `json:"bind_address"`    // "0.0.0.0:PORT" or ""
    Active         bool   `json:"active"`
    Error          string `json:"error,omitempty"` // bind error text, if any
}
```

#### Updated `Close()`

Add before existing `s.cancel()` call:

```go
if s.extServer != nil {
    _ = s.extServer.Close() // closes extListener internally
    s.extServer = nil
}
if s.extListener != nil { // defensive; http.Server.Close covers this
    _ = s.extListener.Close()
    s.extListener = nil
}
```

---

### 6.3 `backend/internal/orthrus/server.go`

#### `HandleWebSocket` — call `StartExternalProxy` after `StartDockerProxy`

```go
if err := session.StartDockerProxy(); err != nil {
    logger.Log().WithField("uuid", util.SanitizeForLog(agent.UUID)).
        WithError(err).Warn("orthrus: failed to start loopback docker proxy")
}

// agent.ExternalProxyPort == 0 is a no-op in StartExternalProxy.
if err := session.StartExternalProxy(agent.ExternalProxyPort); err != nil {
    logger.Log().WithField("uuid", util.SanitizeForLog(agent.UUID)).
        WithField("port", agent.ExternalProxyPort).
        WithError(err).Warn("orthrus: failed to start external docker proxy")
}
```

#### `watchHeartbeat` — fix goroutine leak (ADR-07)

Add `_ = sess.Close()` before `s.markOffline(agentUUID)` and `s.sessions.Delete(agentUUID)`.

#### New method: `GetExternalProxyStatus(agentUUID string) (ExternalProxyStatus, bool)`

```go
func (s *OrthrusServer) GetExternalProxyStatus(agentUUID string) (ExternalProxyStatus, bool) {
    raw, ok := s.sessions.Load(agentUUID)
    if !ok {
        return ExternalProxyStatus{}, false
    }
    return raw.(*AgentSession).GetExternalProxyStatus(), true
}
```

---

### 6.4 `backend/internal/services/orthrus_service.go`

#### Updated `Patch` signature

Add `externalProxyPort *int` parameter.

#### Validation

```go
if externalProxyPort != nil {
    port := *externalProxyPort
    if port != 0 && (port < 1024 || port > 65535) {
        return nil, fmt.Errorf("orthrus: external_proxy_port must be 0 (disabled) or in range [1024, 65535], got %d", port)
    }
    updates["external_proxy_port"] = port
}
```

---

### 6.5 `backend/internal/api/handlers/orthrus_handler.go`

#### Updated `patchAgentRequest`

Add:

```go
ExternalProxyPort *int `json:"external_proxy_port"` // 0=disabled, 1024-65535=enabled
```

#### Updated `Patch` handler call

Pass `req.ExternalProxyPort` as the new final argument to `h.svc.Patch(...)`.

#### New interface + field on `OrthrusHandler`

```go
type orthrusProxyStatusResolver interface {
    GetExternalProxyStatus(agentUUID string) (orthrus.ExternalProxyStatus, bool)
}

type OrthrusHandler struct {
    svc           *services.OrthrusService
    proxyResolver orthrusProxyStatusResolver // nil when orthrus server unavailable
}

// SetProxyResolver wires the live proxy status source.
// Uses reflect nil-guard (same pattern as DockerHandler.SetOrthrusResolver).
func (h *OrthrusHandler) SetProxyResolver(r orthrusProxyStatusResolver) { ... }
```

#### New handler: `GetProxyStatus`

Route: `GET /orthrus/agents/:uuid/proxy-status` (added in `RegisterRoutes`).

Logic:

1. `h.svc.Get(uuid)` → 404 if not found.
2. Build base response with `configured_port` and `agent_online`.
3. If `h.proxyResolver == nil` → return with `active: false`, `error: "orthrus subsystem unavailable"`.
4. `h.proxyResolver.GetExternalProxyStatus(uuid)` → if not connected, `active: false`.
5. If connected and active: populate `active_port`, `bind_address`, `connection_string: "tcp://charon:PORT"`.
6. If connected but error: populate `error` field.

---

### 6.6 `backend/internal/api/routes/routes.go`

After `orthrusHandler.RegisterRoutes(management)`, add:

```go
if orthrusServer != nil {
    orthrusHandler.SetProxyResolver(orthrusServer)
}
```

No AutoMigrate change required (`OrthrusAgent` already listed).

---

## 7. API Endpoints

### 7.1 Modified: `PATCH /api/v1/orthrus/agents/:uuid`

**Auth**: Bearer (management access required)

**New optional field in request body**:

```json
{ "external_proxy_port": 2375 }
```

**Validation**: `external_proxy_port` ∈ `{0} ∪ [1024, 65535]`. Values 1–1023 or > 65535 → `400 Bad Request`.

**Response** `200 OK`:

```json
{
  "uuid": "abc-123",
  "name": "my-agent",
  "status": "online",
  "external_proxy_port": 2375,
  "created_at": "...",
  "updated_at": "..."
}
```

---

### 7.2 New: `GET /api/v1/orthrus/agents/:uuid/proxy-status`

**Auth**: Bearer (management access required)

**Response** `200 OK` — agent offline, proxy disabled:

```json
{
  "agent_uuid": "abc-123",
  "configured_port": 0,
  "agent_online": false,
  "active": false,
  "active_port": 0
}
```

**Response** `200 OK` — agent online, proxy active:

```json
{
  "agent_uuid": "abc-123",
  "configured_port": 2375,
  "agent_online": true,
  "active": true,
  "active_port": 2375,
  "bind_address": "0.0.0.0:2375",
  "connection_string": "tcp://charon:2375"
}
```

**Response** `200 OK` — bind failed (port conflict):

```json
{
  "agent_uuid": "abc-123",
  "configured_port": 2375,
  "agent_online": true,
  "active": false,
  "active_port": 0,
  "error": "bind tcp 0.0.0.0:2375: bind: address already in use"
}
```

**Error**: `404` — agent not found.

---

## 8. Frontend Changes

### 8.1 Orthrus Agent Edit Modal — New Section

**Location**: Identify the agent edit modal/panel in `frontend/src/components/orthrus/` by searching for the existing `PATCH /orthrus/agents/:uuid` call site.

**New "External Docker Proxy" section**:

```
┌─ External Docker Proxy ─────────────────────────────────────────────┐
│                                                                       │
│  Proxy Port  [ 2375 ]   (0 = disabled · 1024–65535)                  │
│                                                                       │
│  ⚠ Accessible from all containers on the same Docker network.       │
│     Do not publish this port to the host unless intended.             │
│                                                                       │
│  Status: ● Active   tcp://charon:2375   [Copy]                       │
│  or:     ○ Inactive (agent offline or port=0)                        │
│  or:     ⚠ Error: bind: address already in use                       │
│                                                                       │
│  ℹ Port changes take effect on next agent reconnect.                 │
└──────────────────────────────────────────────────────────────────────┘
```

**State management**:

1. On modal open: `GET /orthrus/agents/:uuid/proxy-status` → display live status.
2. Port input: `<input type="number" min="0" max="65535" step="1">`. Client-side validation: `value === 0 || (value >= 1024 && value <= 65535)`.
3. On save: `PATCH /orthrus/agents/:uuid` with `{ external_proxy_port: N }`.
4. After save: re-fetch proxy status.

**TypeScript type**:

```typescript
interface ExternalProxyStatus {
  agent_uuid: string;
  configured_port: number;
  agent_online: boolean;
  active: boolean;
  active_port: number;
  bind_address?: string;
  connection_string?: string;
  error?: string;
}
```

**Accessibility** (per a11y instructions):

- `<label for="external-proxy-port">` with `aria-describedby` pointing to help text.
- `role="status"` + `aria-live="polite"` on status div.
- Copy button: `aria-label="Copy Docker connection string to clipboard"`.
- Bind errors: `role="alert"`.
- Never use colour as the sole status differentiator; use icon + text.

### 8.2 Agent List — Optional PROXY Badge

When `external_proxy_port > 0` and agent is online: show a `PROXY` chip/badge. Clicking copies the connection string to clipboard.

### 8.3 API Client

Add to `frontend/src/api/orthrus.ts` (or equivalent):

```typescript
export async function getAgentProxyStatus(uuid: string): Promise<ExternalProxyStatus> {
  const res = await api.get<ExternalProxyStatus>(`/orthrus/agents/${uuid}/proxy-status`);
  return res.data;
}
```

This is a 5-line change to one file. No scaffolding, no test setup, no
multi-phase implementation. The entire work is one atomic commit.

## 9. Security Analysis

### 9.1 Threat Model

| Threat | Vector | Mitigation |
|--------|--------|------------|
| Destructive Docker control from Docker network | `POST /containers/{id}/kill` | Muzzle: only GET allowlisted paths. POST → 403. |
| Docker API path traversal | `GET /containers/../../../etc/passwd` | Muzzle path-maps against fixed allowlist after stripping version prefix. Unmatched → 403. |
| Log injection via path | `GET /containers%0A%0Ainjected` | `sanitizePath()` (existing in `muzzle.go`) strips `\n`/`\r`. |
| Slowloris connection hold | Client sends partial headers | `http.Server.ReadHeaderTimeout = 10s`. |
| SSRF via `Host` header | Attacker rewrites `Host` to internal service | `Director` unconditionally sets `req.URL.Host = "127.0.0.1:loopbackPort"`. |
| Port pre-occupation DoS | Another process holds the port | Bind failure logged and surfaced via `/proxy-status`. Loopback proxy unaffected. No crash. |
| Port published to Docker host | User adds `-p PORT:PORT` to Charon container | User-controlled; out of scope. UI warning displayed. |

### 9.2 Residual Read Exposure

Allowlisted endpoints (`/containers/json`, `/images/json`, `/info`, `/version`) reveal container names, image tags, environment variables, and network topology. Operators running sensitive workloads must evaluate whether enabling the external proxy is appropriate for their threat model.

### 9.3 Defence in Depth

Muzzle applied at:

1. **Charon side** (new) — at the external HTTP entry point.
2. **Agent side** (existing in `agent/leash/leash.go`) — before bytes reach the Docker socket.

A bug in Charon's muzzle is not catastrophic; the agent's muzzle is a second gate.

---

## 10. Test Plan

### 10.1 Unit Tests: `backend/internal/orthrus/session_external_proxy_test.go` (new)

| ID | Description | Pass Condition |
|----|-------------|----------------|
| U-EXT-01 | `StartExternalProxy(0)` is a no-op | Returns nil; `GetExternalProxyStatus().Active == false` |
| U-EXT-02 | `StartExternalProxy(PORT)` binds `0.0.0.0:PORT` | Status: `Active=true`, `BindAddress="0.0.0.0:PORT"` |
| U-EXT-03 | Double call to `StartExternalProxy` | Second call returns error containing "already started" |
| U-EXT-04 | `StartExternalProxy` before `StartDockerProxy` | Error contains "loopback proxy not started" |
| U-EXT-05 | `StartExternalProxy` on closed session | Error contains "closed session" |
| U-EXT-06 | `Close()` terminates external listener | Subsequent TCP dial refused |
| U-EXT-07 | GET `/containers/json` through external proxy | HTTP 200 proxied through mock loopback |
| U-EXT-08 | POST request to external proxy | HTTP 403, no yamux stream opened |
| U-EXT-09 | GET to non-allowlisted path | HTTP 403 for `GET /exec/abc/start` |
| U-EXT-10 | `ReadHeaderTimeout` set on http.Server | `srv.ReadHeaderTimeout == 10 * time.Second` |

**Test helper**: Use `testWSPairBoth` (existing in `session_proxy_test.go`). Allocate free port with `net.Listen("tcp", "127.0.0.1:0")`, extract port, close listener, pass that port to `StartExternalProxy`.

### 10.2 Unit Tests: `backend/internal/orthrus/server_test.go` (additions)

| ID | Description |
|----|-------------|
| U-SRV-01 | `GetExternalProxyStatus` returns `(_, false)` for unknown UUID |
| U-SRV-02 | `Close()` on session with active external proxy shuts down `http.Server` and frees the port |

### 10.3 Unit Tests: `backend/internal/services/orthrus_service_test.go` (additions)

| ID | Description |
|----|-------------|
| U-SVC-01 | `Patch` with `externalProxyPort = 0` persists successfully |
| U-SVC-02 | `Patch` with `externalProxyPort = 2375` persists successfully |
| U-SVC-03 | `Patch` with `externalProxyPort = 80` returns validation error |
| U-SVC-04 | `Patch` with `externalProxyPort = 99999` returns validation error |
| U-SVC-05 | `Patch` with `externalProxyPort = nil` leaves existing value unchanged |

### 10.4 Unit Tests: `backend/internal/api/handlers/orthrus_handler_test.go` (additions)

| ID | Description |
|----|-------------|
| U-HDL-01 | `PATCH` with `external_proxy_port: 2375` → updated agent with port |
| U-HDL-02 | `PATCH` with `external_proxy_port: 100` → 400 |
| U-HDL-03 | `GET /proxy-status` unknown agent → 404 |
| U-HDL-04 | `GET /proxy-status` resolver nil → `active: false`, `error` present |
| U-HDL-05 | `GET /proxy-status` resolver returns active status → `connection_string` present |
| U-HDL-06 | `GET /proxy-status` resolver returns inactive status → `active: false` |
| U-HDL-07 | `GET /proxy-status` resolver returns bind error → `error` field present |

### 10.5 Playwright E2E: `tests/orthrus-external-proxy.spec.ts` (new)

| Test | Steps | Expected |
|------|-------|----------|
| Configure proxy port | Open agent edit modal, enter 2376, save | Port saved; success toast; `/proxy-status` returns `configured_port: 2376` |
| Disable proxy | Set port to 0, save | Status shows disabled |
| Invalid port rejected | Enter 80, attempt save | Client-side validation error; no API call made |
| Connection string displayed | Agent online with active proxy | `tcp://charon:2376` shown with copy button |
| Reconnect notice | Configured port differs from active port | "Changes will take effect on next agent reconnect" message shown |

### 10.6 Integration Test Stub

File: `backend/internal/orthrus/external_proxy_integration_test.go`
Build tag: `//go:build integration`

Test: Full agent WebSocket connect → `ExternalProxyPort` configured → Docker client dials external port → container list returned.

---

## 11. Commit Slicing Strategy

**Decision**: Single PR, 4 ordered logical commits. Each commit is independently buildable and passes `go test ./...`.

---

### Commit 1 — `feat(orthrus): add external_proxy_port field to OrthrusAgent model`

**Scope**: Model only. Zero runtime behaviour change.

**Files**:

- `backend/internal/models/orthrus_agent.go` — add `ExternalProxyPort int` field

**Dependencies**: None.

**Validation gate**:

```bash
cd backend && go build ./... && go test ./internal/models/...
```

---

### Commit 2 — `feat(orthrus): implement external docker proxy listener with muzzle filtering`

**Scope**: Session and server layer. New external proxy + updated `Close()` + unit tests.

**Files**:

- `backend/internal/orthrus/session.go` — `StartExternalProxy`, `GetExternalProxyStatus`, `ExternalProxyStatus` type, updated `Close()` (add `extServer.Close()`)
- `backend/internal/orthrus/server.go` — wire `StartExternalProxy` in `HandleWebSocket`, add `GetExternalProxyStatus` method (no `watchHeartbeat` change needed — already correct)
- `backend/internal/orthrus/session_external_proxy_test.go` — new file: U-EXT-01..10
- `backend/internal/orthrus/server_test.go` — add U-SRV-01, U-SRV-02

**Dependencies**: Commit 1 (`agent.ExternalProxyPort` field must exist for `HandleWebSocket` to read).

**Validation gate**:

```bash
cd backend && go test ./internal/orthrus/...
```

---

### Commit 3 — `feat(orthrus): API endpoints for external proxy configuration and status`

**Scope**: Service + handler + route wiring.

**Files**:

- `backend/internal/services/orthrus_service.go` — extend `Patch`, add validation
- `backend/internal/services/orthrus_service_test.go` — add U-SVC-01..05
- `backend/internal/api/handlers/orthrus_handler.go` — `patchAgentRequest`, `GetProxyStatus`, `orthrusProxyStatusResolver`, `SetProxyResolver`
- `backend/internal/api/handlers/orthrus_handler_test.go` — add U-HDL-01..07
- `backend/internal/api/routes/routes.go` — wire `SetProxyResolver`

**Dependencies**: Commit 2 (`GetExternalProxyStatus` on `OrthrusServer`).

**Validation gate**:

```bash
cd backend && go test ./...
```

---

### Commit 4 — `feat(orthrus/ui): external docker proxy configuration and status UI`

**Scope**: Frontend only.

**Files**:

- `frontend/src/api/orthrus.ts` — add `getAgentProxyStatus`
- `frontend/src/components/orthrus/<AgentEditModal>.tsx` — new External Docker Proxy section
- `frontend/src/components/orthrus/<AgentList>.tsx` — optional PROXY badge
- `tests/orthrus-external-proxy.spec.ts` — new Playwright E2E tests

**Dependencies**: Commit 3 (API endpoints must exist).

**Validation gate**:

```bash
cd frontend && npm test
# E2E (requires Docker E2E environment rebuild):
npx playwright test tests/orthrus-external-proxy.spec.ts --project=firefox
```

---

### Rollback Notes

Each commit is independently revertable:

- Commit 1: DB column drop is safe (SQLite stores it; old code ignores unknown columns on read).
- Commits 2–3: New code paths only activate when `ExternalProxyPort > 0` or the new endpoint is called. Reverting has no impact on existing agent operation.
- Commit 4: Frontend-only; reverting does not affect backend.

---

## 12. Migration Strategy

### 12.1 Existing agents

GORM AutoMigrate at startup adds `external_proxy_port INTEGER DEFAULT 0`. All existing agents receive `0` (disabled). No proxy is started. No operator action required unless the feature is desired.

### 12.2 Downgrade

Old Charon without this feature ignores the unknown DB column on read. No functional breakage. The column can be dropped manually if desired.

### 12.3 `socat`-based deployments (Dockhand users)

Migration path:

1. Identify the Orthrus agent for the target host.
2. Set `external_proxy_port` to the port previously served by `socat` (e.g., 2375).
3. Update Dockhand Docker host config: `tcp://socat-container:2375` → `tcp://charon:2375`.
4. Decommission the `socat` container.

The UI displays `tcp://charon:PORT` prominently once the proxy is active.

### 12.4 Multi-agent port uniqueness

No DB uniqueness constraint on `external_proxy_port` (excluding 0) is enforced in this PR. Operators must assign distinct ports per agent. Conflicts surface as bind errors in the `/proxy-status` endpoint. A uniqueness constraint is a future improvement.

---

## Appendix A: Type Placement

`ExternalProxyStatus` is defined in `backend/internal/orthrus/session.go` alongside `AgentSession`. The handler imports it as `orthrus.ExternalProxyStatus` via the `orthrusProxyStatusResolver` interface. No circular imports.

## Appendix B: File Change Summary

| File | Change | Commit |
|------|--------|--------|
| `backend/internal/models/orthrus_agent.go` | Add `ExternalProxyPort int` | 1 |
| `backend/internal/orthrus/session.go` | `StartExternalProxy`, `GetExternalProxyStatus`, `Close` update, `ExternalProxyStatus` type | 2 |
| `backend/internal/orthrus/server.go` | Wire `StartExternalProxy`, fix `watchHeartbeat`, `GetExternalProxyStatus` | 2 |
| `backend/internal/orthrus/session_external_proxy_test.go` | New: U-EXT-01..10 | 2 |
| `backend/internal/orthrus/server_test.go` | Add U-SRV-01..02 | 2 |
| `backend/internal/services/orthrus_service.go` | Extend `Patch`, add port validation | 3 |
| `backend/internal/services/orthrus_service_test.go` | Add U-SVC-01..05 | 3 |
| `backend/internal/api/handlers/orthrus_handler.go` | `patchAgentRequest`, `GetProxyStatus`, interface, `SetProxyResolver` | 3 |
| `backend/internal/api/handlers/orthrus_handler_test.go` | Add U-HDL-01..07 | 3 |
| `backend/internal/api/routes/routes.go` | Wire `SetProxyResolver` | 3 |
| `frontend/src/api/orthrus.ts` | Add `getAgentProxyStatus` | 4 |
| `frontend/src/components/orthrus/<modal>.tsx` | External proxy section | 4 |
| `frontend/src/components/orthrus/<list>.tsx` | PROXY badge | 4 |
| `tests/orthrus-external-proxy.spec.ts` | New E2E tests | 4 |

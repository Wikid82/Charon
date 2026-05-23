# Uptime Monitoring Bugs: Orthrus-Managed Remote Servers Always Reported DOWN

**Status**: Active
**Target**: New PR (`development` → `main`)

---

## 1. Introduction

### Overview

Two interrelated bugs cause Charon's uptime monitoring subsystem to permanently report
Orthrus-managed remote servers as DOWN, even when the Orthrus WebSocket tunnel is alive
and healthy.

**Bug 1 — Orthrus host-check dials the wrong port on the wrong machine.**
`SyncMonitors()` creates a TCP monitor using `server.Host:server.Port` for every
`RemoteServer` regardless of `ConnectionType`. For `ConnectionTypeOrthrus`, `server.Port`
is the `ExternalProxyPort` (e.g. 2375) that Orthrus binds on `0.0.0.0` of the **Charon
server** — not on the remote machine. The host-level pre-check in `checkHost()` therefore
dials `<remote_tailscale_ip>:2375`, which is not open on the remote machine, and marks
the `UptimeHost` as DOWN after `FailureThreshold` failures.

**Bug 2 — Downstream cascade makes every TCP monitor for the same IP report DOWN.**
Once an `UptimeHost` is marked DOWN, `CheckAll()` short-circuits all TCP monitors
associated with that host without running them. Any additional TCP monitors for services
at the same Tailscale IP (e.g. a port-3001 Dockhand monitor) are therefore always
reported DOWN even if the service is reachable, because they ride the same `UptimeHost`
that was incorrectly marked DOWN by Bug 1.

### Objectives

1. Fix `SyncMonitors()` to build correct, connection-type-aware monitor records for
   Orthrus remote servers.
2. Fix `checkHost()` to skip raw TCP dials for Orthrus-only hosts and report their
   reachability via Orthrus session state instead.
3. Fix `checkMonitor()` to handle `Type = "orthrus"` monitors by querying the live
   Orthrus session rather than dialling a TCP port.
4. Fix `CheckAll()` to never short-circuit `"orthrus"` monitors on a host-DOWN event
   (analogous to the existing HTTP monitor exception).
5. Wire the Orthrus resolver into `UptimeService` without creating a circular dependency.
6. Preserve all existing behaviour for non-Orthrus remote servers and all proxy host monitors.

---

## 2. Research Findings

### 2.1 Architecture Summary

| Component | Location | Role |
|-----------|----------|------|
| `UptimeService` | `backend/internal/services/uptime_service.go` | Polling, monitor CRUD, host status |
| `SyncMonitors()` | `uptime_service.go:153` | Creates/updates monitor records from ProxyHost + RemoteServer |
| `CheckAll()` | `uptime_service.go:354` | Orchestrates host pre-checks then individual monitor checks |
| `checkAllHosts()` | `uptime_service.go:415` | Runs `checkHost()` for every `UptimeHost` |
| `checkHost()` | `uptime_service.go:457` | TCP pre-check; determines `UptimeHost.Status` |
| `checkMonitor()` | `uptime_service.go:731` | Per-monitor HTTP / TCP / (new) orthrus check |
| `extractPort()` | `uptime_service.go:81` | Extracts port string from URL or `host:port` |
| `OrthrusServer` | `backend/internal/orthrus/server.go` | WS endpoint; tracks live `AgentSession` objects |
| `AgentSession` | `backend/internal/orthrus/session.go` | Per-agent state; starts Docker proxy listeners |
| `RemoteServer` | `backend/internal/models/remote_server.go` | User record; holds `ConnectionType`, `OrthrusAgentUUID`, `Host`, `Port` |
| `UptimeMonitor` | `backend/internal/models/uptime.go` | Per-monitor record; `Type` is free-form string |
| `UptimeHost` | `backend/internal/models/uptime_host.go` | Per-IP grouping record; `Status` drives short-circuit |
| `routes.go` | `backend/internal/api/routes/routes.go:282` | DI wiring; creates `UptimeService` |

### 2.2 Critical Code Paths

#### SyncMonitors() — Remote server block (lines 260–335)

```go
// No ConnectionType check exists. For all RemoteServers:
targetType := "tcp"
targetURL  := fmt.Sprintf("%s:%d", server.Host, server.Port)
// For Orthrus: server.Port = ExternalProxyPort (e.g. 2375) bound on CHARON server
// Result: targetURL = "100.99.23.57:2375" — port doesn't exist on the remote machine
```

#### checkHost() — Port extraction (lines 513–525)

```go
// monitor.ProxyHost == nil for RemoteServer monitors, so fallback branch runs:
port = extractPort(monitor.URL)          // returns "2375" from "100.99.23.57:2375"
addr = net.JoinHostPort(host.Host, port) // "100.99.23.57:2375"
conn, err = dialer.DialContext(ctx, "tcp", addr)
// FAILS — port 2375 is bound on Charon (0.0.0.0:2375), not on the remote machine
```

#### StartExternalProxy() — Binds on Charon host (session.go ~260)

```go
// Binds on THE CHARON SERVER, not on the remote machine.
ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
connStr = fmt.Sprintf("tcp://charon:%d", activePort)
```

Port 2375 is bound on the Charon server's network interface. The remote machine at
`100.99.23.57` never listens on this port.

#### CheckAll() — Short-circuit logic (lines 390–412)

```go
if uptimeHost.Status == "down" {
    // TCP monitors are short-circuited (never checked):
    s.markHostMonitorsDown(tcpMonitors, &uptimeHost)
    // HTTP/HTTPS monitors still run (nonTCPMonitors).
    for _, monitor := range nonTCPMonitors { go s.checkMonitor(monitor) }
    continue
}
```

Once Bug 1 marks the host DOWN, every TCP monitor for the same IP is permanently
skipped, including unrelated services that may be fully reachable.

#### GetProxyAddr() — Session liveness signal (server.go:142–155)

```go
func (s *OrthrusServer) GetProxyAddr(agentUUID string) (string, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    sess, ok := s.sessions[agentUUID]
    if !ok { return "", false }
    addr := sess.GetDockerProxyAddr()
    return addr, addr != ""  // ok == true ↔ agent connected and proxy active
}
```

`GetProxyAddr` returning `ok = true` is the authoritative signal that an Orthrus agent
has an active WebSocket session. No network I/O — it's an in-memory map lookup.

#### DI wiring — routes.go (lines 282 and 511–512)

```go
// Line 282 — UptimeService created BEFORE orthrusServer exists:
uptimeService := services.NewUptimeService(db, notificationService)

// Lines 471–484 — orthrusServer conditionally created inside feature block.

// Lines 511–512 — existing post-conditional setter pattern (model to follow):
if orthrusServer != nil {
    dockerHandler.SetOrthrusResolver(orthrusServer)
}
```

The `SetOrthrusResolver` setter pattern is already established. `UptimeService` needs
the same setter called in the same location.

#### Existing orthrusProxyResolver interface (docker_handler.go:26–27)

```go
type orthrusProxyResolver interface {
    GetProxyAddr(agentUUID string) (string, bool)
}
```

`OrthrusServer.GetProxyAddr` already satisfies this signature. No new methods needed
on `OrthrusServer`.

---

## 3. Root Cause Analysis

### Bug 1 — Complete Causal Chain

```
User creates RemoteServer:
  Host = "100.99.23.57"  ← Tailscale IP of remote machine
  Port = 2375            ← ExternalProxyPort (bound on CHARON at 0.0.0.0:2375)
  ConnectionType = "orthrus"
  OrthrusAgentUUID = "<uuid>"

SyncMonitors() [uptime_service.go:260–335]:
  ↓ No ConnectionType check — all RemoteServers treated identically
  ↓ targetURL = "100.99.23.57:2375"
  ↓ targetType = "tcp"
  Creates UptimeMonitor { Type: "tcp", URL: "100.99.23.57:2375" }
  Creates UptimeHost    { Host: "100.99.23.57" }

checkAllHosts() → checkHost() [uptime_service.go:513–525]:
  ↓ monitor.ProxyHost == nil (RemoteServer monitor, not ProxyHost monitor)
  ↓ extractPort("100.99.23.57:2375") = "2375"
  ↓ addr = "100.99.23.57:2375"
  net.DialContext("tcp", "100.99.23.57:2375")
  → ECONNREFUSED — port 2375 not open on remote machine
  host.FailureCount++ → reaches FailureThreshold (default: 2)
  host.Status = "down"   ← FALSE NEGATIVE
```

### Bug 2 — Cascade Chain

```
UptimeHost { Host: "100.99.23.57", Status: "down" }  (set by Bug 1)

User has additional monitor at 100.99.23.57:3001 (e.g. Dockhand)

CheckAll() [uptime_service.go:390–412]:
  ↓ uptimeHost.Status == "down"
  ↓ monitor { Type: "tcp", URL: "100.99.23.57:3001" } → tcpMonitors list
  markHostMonitorsDown([monitor@3001], host)
  monitor.Status = "down"  ← NEVER ACTUALLY CHECKED — FALSE NEGATIVE
```

### Common Root

Both bugs share the same root: `SyncMonitors()` and `checkHost()` have zero awareness of
`RemoteServer.ConnectionType`. For `ConnectionTypeOrthrus`:

- `server.Port` stores the `ExternalProxyPort` — a port bound on the Charon server, not
  the remote machine. Using it as the TCP target for the remote IP is semantically wrong.
- Orthrus connectivity is gauged by **WebSocket session liveness**, not TCP reachability
  of any port on the remote machine.

The contrast is instructive: `docker_handler.go` already handles `ConnectionTypeOrthrus`
correctly — it calls `orthrusResolver.GetProxyAddr(agentUUID)` instead of dialling the
remote IP. The uptime service needs the same awareness.

---

## 4. Technical Specification

### 4.1 Design Decision

**Chosen approach: Orthrus-aware monitor type `"orthrus"`**

Introduce a first-class monitor type `"orthrus"` handled at every level of the uptime stack:

| Layer | Change |
|-------|--------|
| `SyncMonitors()` | Detect `ConnectionTypeOrthrus`; create monitor with `Type="orthrus"`, `URL=agentUUID` |
| `checkHost()` | Skip TCP dial for `"orthrus"` monitors; skip host pre-check entirely if all monitors are orthrus-only |
| `checkMonitor()` | Add `case "orthrus":` — query `orthrusResolver.GetProxyAddr(agentUUID)` |
| `CheckAll()` | `"orthrus"` is not `"tcp"` so it already falls into `nonTCPMonitors`; add defensive comment |
| DI | Add `SetOrthrusResolver()` to `UptimeService`; call from `routes.go` |

**Why not a minimal skip-in-checkHost approach**: It would require loading `RemoteServer`
in `checkHost()` (extra DB join), still produces false-DOWN on individual TCP monitor
checks, and doesn't give correct per-monitor status. Option A is cleaner and correct.

### 4.2 Interface Definition

Add a private interface to `uptime_service.go`. This avoids importing the `orthrus`
package and follows the established pattern in `docker_handler.go`:

```go
// orthrusStatusChecker allows UptimeService to query Orthrus session liveness
// without a direct dependency on the orthrus package.
type orthrusStatusChecker interface {
    GetProxyAddr(agentUUID string) (string, bool)
}
```

`OrthrusServer.GetProxyAddr` already satisfies this interface.

### 4.3 UptimeService Struct Changes

```go
type UptimeService struct {
    DB                  *gorm.DB
    NotificationService *NotificationService
    orthrusResolver     orthrusStatusChecker  // nil when Orthrus feature is disabled
    // ... all existing fields unchanged ...
}

// SetOrthrusResolver injects the Orthrus session resolver.
// Uses the typed-nil guard pattern established in DockerHandler.
func (s *UptimeService) SetOrthrusResolver(r orthrusStatusChecker) {
    if r == nil {
        s.orthrusResolver = nil
        return
    }
    s.orthrusResolver = r
}
```

### 4.4 SyncMonitors() — Orthrus Branch

Insert before the existing `switch err` (create-vs-update) block in the remote server loop:

```go
// Orthrus-managed servers: connectivity is measured by session liveness, not TCP.
if server.ConnectionType == models.ConnectionTypeOrthrus {
    if server.OrthrusAgentUUID == nil || *server.OrthrusAgentUUID == "" {
        continue // No agent linked — cannot create a meaningful monitor
    }
    targetType = "orthrus"
    targetURL  = *server.OrthrusAgentUUID  // Agent UUID as the monitor identifier
    // upstreamHost remains server.Host (Tailscale IP) — correct for grouping/display
}
```

The existing `switch err` block then stores or updates the monitor with `Type="orthrus"`
and `URL="<agentUUID>"`. The `monitor.Type != targetType` update guard migrates any
existing stale TCP records on the next `SyncMonitors()` call — no manual migration
needed.

### 4.5 checkHost() — Skip Orthrus Monitors

In the port-extraction loop, skip monitors with `Type == "orthrus"` and track whether
any dialable ports were found:

```go
attempted := false
for _, monitor := range monitors {
    if strings.ToLower(monitor.Type) == "orthrus" {
        continue  // Orthrus liveness checked per-monitor, not via TCP pre-check
    }
    var port string
    if monitor.ProxyHost != nil {
        port = fmt.Sprintf("%d", monitor.ProxyHost.ForwardPort)
    } else {
        port = extractPort(monitor.URL)
    }
    if port == "" {
        continue
    }
    attempted = true
    // ... existing dial logic unchanged ...
}

// If every monitor for this host is Orthrus-type, there are no dialable ports.
// Skip the TCP pre-check; individual checkMonitor() calls determine status.
if !attempted {
    return
}
```

This ensures `checkHost()` never marks an Orthrus-only `UptimeHost` as DOWN via TCP.

### 4.6 checkMonitor() — Orthrus Case

Add to the `switch monitor.Type` block:

```go
case "orthrus":
    agentUUID := monitor.URL  // Agent UUID stored in the URL field
    if s.orthrusResolver == nil {
        msg = "Orthrus subsystem unavailable"
        break
    }
    if agentUUID == "" {
        msg = "Monitor missing agent UUID"
        break
    }
    _, ok := s.orthrusResolver.GetProxyAddr(agentUUID)
    if ok {
        success = true
        msg = "Orthrus session active"
    } else {
        msg = "Orthrus agent not connected"
    }
```

No network I/O. `GetProxyAddr` is an in-memory map lookup (~0 µs). Latency recorded
by the outer checkMonitor logic will be near zero and is not meaningful for this type.

### 4.7 CheckAll() — Short-Circuit Invariant

`"orthrus"` is not `"tcp"`, so it already falls into `nonTCPMonitors` (always checked)
in the existing short-circuit logic. However, with the `checkHost()` fix in place,
Orthrus-only hosts will never enter the `Status == "down"` branch at all. Add a comment
documenting the invariant for clarity; no code change is required.

### 4.8 DI Wiring — routes.go

After the existing `if orthrusServer != nil { dockerHandler.SetOrthrusResolver(...) }`
block at lines 511–512, add:

```go
// Inject Orthrus session resolver into uptime service (mirrors dockerHandler pattern).
if orthrusServer != nil {
    uptimeService.SetOrthrusResolver(orthrusServer)
}
```

### 4.9 Database Migration

No schema changes. `UptimeMonitor.Type` is a free-form string column. Existing "tcp"
records for Orthrus servers are migrated in-place by `SyncMonitors()` on the next
scheduled run — no GORM `AutoMigrate` update required.

### 4.10 Edge Cases

| Scenario | Expected Behaviour |
|----------|--------------------|
| Orthrus feature disabled (`orthrusServer == nil`) | `SetOrthrusResolver` not called; `s.orthrusResolver == nil`; `checkMonitor` returns `success=false`, `msg="Orthrus subsystem unavailable"` — monitor stays DOWN. Correct: without the feature the tunnel cannot be active. |
| Agent configured but never connected | `GetProxyAddr` returns `("", false)`; monitor DOWN. Correct. |
| Agent disconnects mid-poll | Next `checkMonitor` run reports DOWN within one poll interval. |
| Agent reconnects | Next `checkMonitor` run reports UP; failure count resets; recovery notification fires. |
| `ConnectionTypeOrthrus` but `OrthrusAgentUUID == nil` | `SyncMonitors()` skips monitor creation (`continue`). Stale pre-fix TCP records for this server remain unchanged (no regression; they will be cleaned up manually or by a future task). |
| Mixed host: Orthrus monitor + direct HTTP monitor at same IP | `checkHost()` dials the HTTP monitor's port; host UP/DOWN reflects HTTP TCP reachability. Orthrus monitor is checked independently via session state. Both reported accurately. |
| Custom `ExternalProxyPort` (not 2375) | Fix is identical — the bug is in the `Type/URL` construction, not the specific port value. |
| Orthrus-only `UptimeHost` (no co-located TCP services) | `UptimeHost.Status` remains `"pending"` — the aggregate host tile never reflects UP/DOWN. Individual monitor tiles are accurate. Known limitation; cosmetic only. |

---

## 5. Implementation Plan

### Phase 1 — Playwright E2E Tests (Written First, TDD)

**File**: `tests/uptime-orthrus.spec.ts`

Write failing tests that define the observable contract before implementing the backend
fix. Use `page.route()` to intercept and mock the uptime monitors API response — no live
Orthrus agent required in the E2E container.

**Tests to write:**

| # | Test | Assertion |
|---|------|-----------|
| 1 | `Uptime dashboard — Orthrus monitor shows "up" badge when API reports connected` | Green UP badge visible for Orthrus-linked RemoteServer monitor |
| 2 | `Uptime dashboard — Orthrus monitor shows "down" when API reports disconnected` | Red DOWN badge; message "Orthrus agent not connected" visible |
| 3 | `Uptime monitor target column — type "orthrus" does not show a raw IP:port` | Target/URL column does not contain `:2375` or similar port string |
| 4 | `Uptime dashboard — non-Orthrus monitor at same IP is checked independently` | A TCP monitor at the same Tailscale IP retains its own correct status |

Run target: `npx playwright test tests/uptime-orthrus.spec.ts --project=firefox`

### Phase 2 — Backend Changes

#### 2a. `backend/internal/services/uptime_service.go`

| Change | Location | Description |
|--------|----------|-------------|
| Add interface | Top of file, after imports | `orthrusStatusChecker` with `GetProxyAddr(agentUUID string) (string, bool)` |
| Add field | `UptimeService` struct | `orthrusResolver orthrusStatusChecker` |
| Add setter | New exported method | `SetOrthrusResolver(r orthrusStatusChecker)` — typed-nil guard |
| Orthrus branch | `SyncMonitors()` — remote server loop | `targetType="orthrus"`, `targetURL=agentUUID` when `ConnectionTypeOrthrus` |
| Skip orthrus dials | `checkHost()` — port-extraction loop | Skip `"orthrus"` monitors; return early if `!attempted` |
| Orthrus case | `checkMonitor()` — switch block | `case "orthrus":` calls `orthrusResolver.GetProxyAddr(agentUUID)` |
| Comment | `CheckAll()` | Document `"orthrus"` ∈ `nonTCPMonitors` invariant |

#### 2b. `backend/internal/api/routes/routes.go`

After the existing `dockerHandler.SetOrthrusResolver` block:

```go
if orthrusServer != nil {
    uptimeService.SetOrthrusResolver(orthrusServer)
}
```

### Phase 3 — Frontend

No changes required for the core bug fix. The uptime dashboard already renders
`monitor.type` and `monitor.url` from the API response. The `"orthrus"` type and UUID
URL render as-is.

**Deferred (separate PR)**: Display a friendly label such as "Orthrus Tunnel" in the
target column instead of the raw agent UUID. This is cosmetic and not part of this fix.

### Phase 4 — Unit Tests

**File**: `backend/internal/services/uptime_service_test.go`

| Test | Description |
|------|-------------|
| `TestSyncMonitors_OrthrusRemoteServer_CreatesOrthrusMonitor` | Orthrus RS → `Type="orthrus"`, `URL=agentUUID`; no TCP record with port `:2375` |
| `TestSyncMonitors_OrthrusRemoteServer_NoUUID_SkipsMonitor` | Orthrus RS with nil UUID → no monitor created |
| `TestSyncMonitors_OrthrusRemoteServer_MigratesExistingTCPMonitor` | Stale `Type="tcp", URL="10.0.0.1:2375"` → updated to `Type="orthrus", URL=uuid` |
| `TestCheckHost_OrthrusOnlyHost_SkipsTCPDial` | Host with only `"orthrus"` monitors → zero TCP dials; host status unchanged from "pending" |
| `TestCheckMonitor_OrthrusType_AgentConnected_ReturnsUp` | `GetProxyAddr` returns `("127.0.0.1:54321", true)` → `success=true`, `msg="Orthrus session active"` |
| `TestCheckMonitor_OrthrusType_AgentDisconnected_ReturnsDown` | `GetProxyAddr` returns `("", false)` → `success=false`, `msg="Orthrus agent not connected"` |
| `TestCheckMonitor_OrthrusType_NilResolver_ReturnsDown` | `s.orthrusResolver == nil` → `success=false`, `msg` contains "Orthrus subsystem unavailable" |
| `TestCheckAll_OrthrusMonitor_NotShortCircuitedWhenHostDown` | `UptimeHost{Status:"down"}` with one `"orthrus"` monitor → `checkMonitor()` is called, `markHostMonitorsDown()` is NOT called for it |

### Phase 5 — Documentation

- `ARCHITECTURE.md` § Uptime Monitoring: document `"orthrus"` as a supported monitor
  type and explain that Orthrus monitors use WebSocket session liveness instead of TCP
  connectivity checks. Document that the `"orthrus"` monitor type reports UP only when
  both the WebSocket tunnel AND the loopback Docker proxy listener are operational.
  Document that Orthrus-only hosts (no co-located TCP services) will show
  `status=pending` as a known cosmetic limitation.
- `CHANGELOG.md`: add entry under Unreleased section.

---

## 6. Commit Slicing Strategy

**Decision**: Single PR, four ordered logical commits.

**Rationale**: The change is isolated to two files (`uptime_service.go` + `routes.go`)
plus tests and docs. Four commits align with four separable concerns, allowing reviewers
to validate each independently and enabling targeted revert if needed.

---

### Commit 1 — `fix(uptime): add OrthrusStatusChecker interface and DI wiring`

**Scope**: Interface definition, struct field, setter, routes.go injection
**Files**:
- `backend/internal/services/uptime_service.go`
- `backend/internal/api/routes/routes.go`

**Description**: Add `orthrusStatusChecker` interface, `orthrusResolver` field and
`SetOrthrusResolver()` setter (with typed-nil guard) to `UptimeService`. Call
`uptimeService.SetOrthrusResolver(orthrusServer)` in `routes.go` after the existing
`dockerHandler.SetOrthrusResolver` call. No behaviour change — resolver is wired but
not yet consulted by any check logic.

**Validation gate**: `go build ./...` passes; all existing tests pass; `uptime_service.go`
imports no packages from `backend/internal/orthrus`.

**Dependencies**: None.

---

### Commit 2 — `fix(uptime): create orthrus-type monitors in SyncMonitors`

**Scope**: `SyncMonitors()` Orthrus branch + SyncMonitors unit tests
**Files**:
- `backend/internal/services/uptime_service.go`
- `backend/internal/services/uptime_service_test.go`

**Description**: Add `ConnectionTypeOrthrus` guard in the remote server loop so that
Orthrus servers produce `Type="orthrus"` monitors with the agent UUID as the URL. The
existing update-branch guard migrates stale TCP records automatically.

**Validation gate**: `TestSyncMonitors_OrthrusRemoteServer_*` tests pass; non-Orthrus
sync tests unchanged.

**Dependencies**: Commit 1 (struct field must exist for mock resolver setup in tests).

> ⚠️ Intermediate state only — `checkMonitor()` case for `"orthrus"` type is added in Commit 3. This commit must not be deployed or merged independently.

---

### Commit 3 — `fix(uptime): skip TCP dials for Orthrus hosts; add orthrus checkMonitor case`

**Scope**: `checkHost()` + `checkMonitor()` + host/monitor unit tests
**Files**:
- `backend/internal/services/uptime_service.go`
- `backend/internal/services/uptime_service_test.go`

**Description**: Skip `"orthrus"` monitors in `checkHost()` port-extraction loop; return
early when no dialable ports remain. Add `case "orthrus":` to `checkMonitor()` switch
using `orthrusResolver.GetProxyAddr(agentUUID)` as the liveness signal.

**Validation gate**: `TestCheckHost_*` and `TestCheckMonitor_*` tests pass;
`TestCheckAll_OrthrusMonitor_NotShortCircuitedWhenHostDown` passes.

**Dependencies**: Commits 1 + 2.

---

### Commit 4 — `test(uptime): add Playwright E2E tests; update architecture docs`

**Scope**: E2E spec + documentation
**Files**:
- `tests/uptime-orthrus.spec.ts`
- `ARCHITECTURE.md`
- `CHANGELOG.md`

**Description**: Add Playwright E2E tests using API mocking. Update ARCHITECTURE.md to
document the `"orthrus"` monitor type. Add CHANGELOG entry.

**Validation gate**: `npx playwright test tests/uptime-orthrus.spec.ts --project=firefox`
passes.

**Dependencies**: Commits 1–3 (backend must be correct for tests to be meaningful).

---

### Rollback

If the PR needs to be reverted, `git revert` the merge commit. The database will contain
`Type="orthrus"` monitor records for any Orthrus servers synced since the PR merged.
These records are benign on the reverted code — they will simply never be checked
(no `case "orthrus"` branch in reverted code, so monitors silently remain in last-known
state). A `SyncMonitors()` call against the reverted code will re-create TCP monitors
for those servers (the update guard fires because `monitor.Type != "tcp"`), restoring
pre-fix behaviour. No manual SQL cleanup required.

---

## 7. Acceptance Criteria

### Functional

| ID | Criterion | Verification |
|----|-----------|-------------|
| AC-1 | Orthrus-managed RemoteServer uptime monitor shows `status=up` when agent WebSocket session is alive **and local Docker proxy listener is operational** | Unit test: `TestCheckMonitor_OrthrusType_AgentConnected_ReturnsUp`; Playwright: test #1 |
| AC-2 | Monitor transitions to `status=down` within one poll interval after agent disconnects | Unit test: `TestCheckMonitor_OrthrusType_AgentDisconnected_ReturnsDown` |
| AC-3 | `UptimeHost` for an Orthrus-only remote server is never marked DOWN due to TCP dial failure | Unit test: `TestCheckHost_OrthrusOnlyHost_SkipsTCPDial` |
| AC-4 | TCP service monitors at the same Tailscale IP as an Orthrus server are checked independently and not cascaded | Unit test: `TestCheckAll_OrthrusMonitor_NotShortCircuitedWhenHostDown`; Playwright: test #4 |
| AC-5 | `SyncMonitors()` creates no TCP monitor with `URL` containing the ExternalProxyPort for an Orthrus server | Unit test: `TestSyncMonitors_OrthrusRemoteServer_CreatesOrthrusMonitor` |
| AC-6 | `SyncMonitors()` creates a monitor with `type="orthrus"` and `url=<agentUUID>` for an Orthrus server | Unit test: same |
| AC-7 | Existing stale TCP monitors for Orthrus servers are migrated to `type="orthrus"` on next `SyncMonitors()` | Unit test: `TestSyncMonitors_OrthrusRemoteServer_MigratesExistingTCPMonitor` |
| AC-8 | When Orthrus feature is disabled (`orthrusServer == nil`), Orthrus monitors report DOWN with message "Orthrus subsystem unavailable" | Unit test: `TestCheckMonitor_OrthrusType_NilResolver_ReturnsDown` |

### Non-Regression

| ID | Criterion |
|----|-----------|
| NR-1 | All existing `uptime_service_test.go` tests pass unchanged |
| NR-2 | Non-Orthrus RemoteServer monitors (direct, tailscale, cloudflare, etc.) continue to use TCP/HTTP checks with `server.Host:server.Port` |
| NR-3 | ProxyHost monitors are unaffected by all changes |
| NR-4 | `GET /api/uptime/monitors` returns all monitor types including `"orthrus"` with correct JSON |

### Definition of Done

- [ ] All unit tests pass: `go test ./backend/...`
- [ ] All Playwright E2E tests pass (Firefox, headless): `npx playwright test tests/uptime-orthrus.spec.ts --project=firefox`
- [ ] `go vet ./...` and `golangci-lint run` pass with zero new findings
- [ ] GORM Security Scanner passes: no model-layer changes, run as precaution
- [ ] `ARCHITECTURE.md` updated to document `"orthrus"` monitor type
- [ ] `CHANGELOG.md` entry added under Unreleased
- [ ] PR description references this spec

---

## 8. Key File Reference

| File | Change Type | Purpose |
|------|-------------|---------|
| `backend/internal/services/uptime_service.go` | Modify | Primary fix: interface, SyncMonitors, checkHost, checkMonitor |
| `backend/internal/api/routes/routes.go` | Modify | DI: inject OrthrusServer into UptimeService |
| `backend/internal/services/uptime_service_test.go` | Modify | Unit tests for all new code paths |
| `tests/uptime-orthrus.spec.ts` | Create | Playwright E2E tests (API-mocked) |
| `ARCHITECTURE.md` | Modify | Document "orthrus" monitor type |
| `CHANGELOG.md` | Modify | Unreleased section entry |

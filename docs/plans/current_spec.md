# Uptime Monitoring Bug Triage & Fix Plan

## 1. Introduction

### Overview

Uptime Monitoring in Charon uses a two-level check system: host-level TCP pre-checks followed by per-monitor HTTP/TCP checks. Newly added proxy hosts (specifically Wizarr and Charon itself) display as "DOWN" in the UI even though the underlying services are fully accessible. Manual refresh via the health check button on the Uptime page correctly shows "UP", but the automated background checker fails to produce the same result.

### Objectives

1. Eliminate false "DOWN" status for newly added proxy hosts
2. Ensure the background checker produces consistent results with manual health checks
3. Improve the initial monitor lifecycle (creation → first check → display)
4. Address the dual `UptimeService` instance functional inconsistency
5. Evaluate whether a "custom health endpoint URL" feature is warranted

### Scope

- **Backend**: `backend/internal/services/uptime_service.go`, `backend/internal/api/routes/routes.go`, `backend/internal/api/handlers/proxy_host_handler.go`
- **Frontend**: `frontend/src/pages/Uptime.tsx`, `frontend/src/api/uptime.ts`
- **Models**: `backend/internal/models/uptime.go`, `backend/internal/models/uptime_host.go`
- **Tests**: `backend/internal/services/uptime_service_test.go` (1519 LOC), `uptime_service_unit_test.go` (257 LOC), `uptime_service_race_test.go` (402 LOC), `tests/monitoring/uptime-monitoring.spec.ts` (E2E)

---

## 2. Research Findings

### 2.1 Root Cause #1: Port Mismatch in Host-Level TCP Check (FIXED)

**Status**: Fixed in commit `209b2fc8`, refactored in `bfc19ef3`.

The `checkHost()` function extracted the port from the monitor's public URL (e.g., 443 for HTTPS) instead of using `ProxyHost.ForwardPort` (e.g., 5690 for Wizarr). This caused TCP checks to fail, marking the host as `down`, which then skipped individual HTTP monitor checks.

**Fix applied**: Added `Preload("ProxyHost")` and prioritized `monitor.ProxyHost.ForwardPort` over `extractPort(monitor.URL)`.

**Evidence**: Archived in `docs/plans/archive/uptime_monitoring_diagnosis.md` and `docs/implementation/uptime_monitoring_port_fix_COMPLETE.md`.

**Remaining risk**: If this fix has not been deployed to production, this remains the primary cause. If deployed, residual elevated `failure_count` values in the DB may need to be reset.

### 2.2 Root Cause #2: Dual UptimeService Instance (OPEN — Functional Inconsistency)

**File**: `backend/internal/api/routes/routes.go`

Two separate `UptimeService` instances are created:

| Instance | Line | Scope |
|----------|------|-------|
| `uptimeService` | 226 | Background ticker goroutine, `ProxyHostHandler`, `/system/uptime/check` endpoint |
| `uptimeSvc` | 414 | Uptime API handler routes (List, Create, Update, Delete, Check, Sync) |

Both share the same `*gorm.DB` (so data consistency via DB is maintained), but each has **independent in-memory state**:

- `pendingNotifications` map (notification batching)
- `hostMutexes` map (per-host mutex for concurrent writes)
- `batchWindow` timers

**Impact**: This is a **functional inconsistency that can cause race conditions between ProxyHostHandler operations and Uptime API operations**. Specifically:

- `ProxyHostHandler.Create()` uses instance #1 (`uptimeService`) for `SyncAndCheckForHost`
- Uptime API queries (List, GetHistory) use instance #2 (`uptimeSvc`)
- In-memory state (host mutexes, pending notifications) is **invisible between instances**

This creates a functional bug path because:

- When a user triggers a manual check via `POST /api/v1/uptime/monitors/:id/check`, the handler uses `uptimeSvc.CheckMonitor()`. If the monitor transitions to "down", the notification is queued in `uptimeSvc`'s `pendingNotifications` map. Meanwhile, the background checker uses `uptimeService`, which has a separate `pendingNotifications` map.
- Duplicate or missed notifications
- Independent failure debouncing state
- Mutex contention issues between the two instances

While NOT the direct cause of the "DOWN" display bug, this is a functional inconsistency — not merely a code smell — that can produce observable bugs in notification delivery and state synchronization.

### 2.3 Root Cause #3: No Immediate Monitor Creation on Proxy Host Create (OPEN)

> **Note — Create ↔ Update asymmetry**: `ProxyHostHandler.Update()` already calls `SyncMonitorForHost` (established pattern). The fix for `Create` should follow the same pattern for consistency.

When a user creates a new proxy host:

1. The proxy host is saved to DB
2. **No uptime monitor is created** — there is no hook in `ProxyHostHandler.Create()` to trigger `SyncMonitors()` or create a monitor
3. `SyncMonitorForHost()` (called on proxy host update) only updates existing monitors — it does NOT create new ones
4. The background ticker must fire (up to 1 minute) for `SyncMonitors()` to create the monitor

**Timeline for a new proxy host to show status**:

- T+0s: Proxy host created via API
- T+0s to T+60s: No uptime monitor exists — Uptime page shows nothing for this host
- T+60s: Background ticker fires, `SyncMonitors()` creates monitor with `status: "pending"`
- T+60s: `CheckAll()` runs, attempts host check + individual check
- T+62s: If checks succeed, monitor `status: "up"` is saved to DB
- T+90s (worst case): Frontend polls monitors and picks up the update

This is a poor UX experience. Users expect to see their new host on the Uptime page immediately.

### 2.4 Root Cause #4: "pending" Status Displayed as DOWN (OPEN)

**File**: `frontend/src/pages/Uptime.tsx`, MonitorCard component

```tsx
const isUp = latestBeat ? latestBeat.status === 'up' : monitor.status === 'up';
```

When a new monitor has `status: "pending"` and no heartbeat history:

- `latestBeat` = `null` (no history yet)
- Falls back to `monitor.status === 'up'`
- `"pending" === "up"` → `false`
- **Displayed with red DOWN styling**

The UI has no dedicated "pending" or "unknown" state. Between creation and first check, every monitor appears DOWN.

### 2.5 Root Cause #5: No Initial CheckAll After Server Start Sync (OPEN)

**File**: `backend/internal/api/routes/routes.go`, lines 455-490

The background goroutine flow on server start:

1. Sleep 30 seconds
2. Call `SyncMonitors()` — creates monitors for all proxy hosts
3. **Does NOT call `CheckAll()`**
4. Start 1-minute ticker
5. First `CheckAll()` runs on first tick (~90 seconds after server start)

This means after every server restart, all monitors sit in "pending" (displayed as DOWN) for up to 90 seconds.

### 2.6 Concern #6: Self-Referencing Check (Charon Pinging Itself)

If Charon has a proxy host pointing to itself (e.g., `charon.example.com` → `localhost:8080`):

**TCP host check**: Connects to `localhost:8080` → succeeds (Gin server is running locally).

**HTTP monitor check**: Sends GET to `https://charon.example.com` → requires DNS resolution from inside the Docker container. This may fail due to:

- **Docker hairpin NAT**: Containers cannot reach their own published ports via the host's external IP by default
- **Split-horizon DNS**: The domain may resolve to a public IP that isn't routable from within the container
- **Caddy certificate validation**: The HTTP client might reject a self-signed or incorrectly configured cert

When the user clicks manual refresh, the same `checkMonitor()` function runs with the same options (`WithAllowLocalhost()`, `WithMaxRedirects(0)`). If manual check succeeds but background check fails, the difference is likely **timing-dependent** — the alternating "up"/"down" pattern observed in the archived diagnosis (heartbeat records alternating between `up|HTTP 200` and `down|Host unreachable`) supports this hypothesis.

### 2.7 Feature Gap: No Custom Health Endpoint URL

The `UptimeMonitor` model has no `health_endpoint` or `custom_url` field. All monitors check the public root URL (`/`). This is problematic because:

- Some services redirect root → `/login` → 302 → tracked inconsistently
- Services with dedicated health endpoints (`/health`, `/api/health`) provide more reliable status
- Self-referencing checks (Charon) could use `http://localhost:8080/api/v1/health` instead of routing through DNS/Caddy

### 2.8 Existing Test Coverage

| File | LOC | Focus |
|------|-----|-------|
| `uptime_service_test.go` | 1519 | Integration tests with SQLite DB |
| `uptime_service_unit_test.go` | 257 | Unit tests for service methods |
| `uptime_service_race_test.go` | 402 | Concurrency/race condition tests |
| `uptime_service_notification_test.go` | — | Notification batching tests |
| `uptime_handler_test.go` | — | Handler HTTP endpoint tests |
| `uptime_monitor_initial_state_test.go` | — | Initial state tests |
| `uptime-monitoring.spec.ts` | — | Playwright E2E (22 scenarios) |

---

## 3. Technical Specifications

### 3.1 Consolidate UptimeService Singleton

**Current**: Two instances (`uptimeService` line 226, `uptimeSvc` line 414) in `routes.go`.

**Target**: Single instance passed to both the background goroutine AND the API handlers.

```go
// routes.go — BEFORE (two instances)
uptimeService := services.NewUptimeService(db, notificationService)  // line 226
uptimeSvc := services.NewUptimeService(db, notificationService)      // line 414

// routes.go — AFTER (single instance)
uptimeService := services.NewUptimeService(db, notificationService)  // line 226
// line 414: reuse uptimeService for handler registration
uptimeHandler := handlers.NewUptimeHandler(uptimeService)
```

**Impact**: All in-memory state (mutexes, notification batching, pending notifications) is shared. The single instance must remain thread-safe (it already is — methods use `sync.Mutex`).

### 3.2 Trigger Monitor Creation + Immediate Check on Proxy Host Create

**File**: `backend/internal/api/handlers/proxy_host_handler.go`

After successfully creating a proxy host, call `SyncMonitors()` (or a targeted sync) and trigger an immediate check:

```go
// In Create handler, after host is saved:
if h.uptimeService != nil {
    _ = h.uptimeService.SyncMonitors()
    // Trigger immediate check for the new monitor
    var monitor models.UptimeMonitor
    if err := h.uptimeService.DB.Where("proxy_host_id = ?", host.ID).First(&monitor).Error; err == nil {
        go h.uptimeService.CheckMonitor(monitor)
    }
}
```

**Alternative (lighter-weight)**: Add a `SyncAndCheckForHost(hostID uint)` method that creates the monitor if needed and immediately checks it.

### 3.3 Add "pending" UI State

**File**: `frontend/src/pages/Uptime.tsx`

Add dedicated handling for `"pending"` status:

```tsx
const isPending = monitor.status === 'pending' && (!history || history.length === 0);
const isUp = latestBeat ? latestBeat.status === 'up' : monitor.status === 'up';
const isPaused = monitor.enabled === false;
```

Visual treatment for pending state:

- Yellow/gray pulsing indicator (distinct from DOWN red and UP green)
- Badge text: "CHECKING..." or "PENDING"
- Heartbeat bar: show empty placeholder bars with a spinner or pulse animation

### 3.4 Run CheckAll After Initial SyncMonitors

**File**: `backend/internal/api/routes/routes.go`

```go
// AFTER initial sync
if enabled {
    if err := uptimeService.SyncMonitors(); err != nil {
        logger.Log().WithError(err).Error("Failed to sync monitors")
    }
    // Run initial check immediately
    uptimeService.CheckAll()
}
```

### 3.5 Add Optional `check_url` Field to UptimeMonitor (Enhancement)

**Model change** (`backend/internal/models/uptime.go`):

```go
type UptimeMonitor struct {
    // ... existing fields
    CheckURL string `json:"check_url,omitempty" gorm:"default:null"`
}
```

**Service behavior** (`uptime_service.go` `checkMonitor()`):

- If `monitor.CheckURL` is set and non-empty, use it instead of `monitor.URL` for the HTTP check
- This allows users to configure `/health` or `http://localhost:8080/api/v1/health` for self-referencing

**Frontend**: Add an optional "Health Check URL" field in the edit monitor modal.

**Auto-migration**: GORM handles adding the column. Existing monitors keep `CheckURL = ""` (uses default URL behavior).

#### 3.5.1 SSRF Protection for CheckURL

The `CheckURL` field accepts user-controlled URLs that the server will fetch. This requires layered SSRF defenses:

**Write-time validation** (on Create/Update API):

- Validate `CheckURL` before saving to DB
- **Scheme restriction**: Only `http://` and `https://` allowed. Block `file://`, `ftp://`, `gopher://`, and all other schemes
- **Max URL length**: 2048 characters
- Reject URLs that fail `url.Parse()` or have empty host components

**Check-time validation** (before each HTTP request):

- Re-validate the URL against the deny list before every check execution (defense-in-depth — the stored URL could have been valid at write time but conditions may change)
- **Localhost handling**: Allow loopback addresses (`127.0.0.1`, `::1`, `localhost`) since self-referencing checks are a valid use case. Block cloud metadata IPs:
  - `169.254.169.254` (AWS/GCP/Azure instance metadata)
  - `fd00::/8` (unique local addresses)
  - `100.100.100.200` (Alibaba Cloud metadata)
  - `169.254.0.0/16` link-local range (except loopback)
- **DNS rebinding protection**: Resolve the hostname at request time, pin the resolved IP, and validate the resolved IP against the deny list before establishing a connection. Use a custom `net.Dialer` or `http.Transport.DialContext` to enforce this
- **Redirect validation**: If `CheckURL` follows HTTP redirects (3xx), validate each redirect target URL against the same deny list (scheme, host, resolved IP). Use a `CheckRedirect` function on the `http.Client` to intercept and validate each hop

**Implementation pattern**:

```go
func validateCheckURL(rawURL string) error {
    if len(rawURL) > 2048 {
        return ErrURLTooLong
    }
    parsed, err := url.Parse(rawURL)
    if err != nil {
        return ErrInvalidURL
    }
    if parsed.Scheme != "http" && parsed.Scheme != "https" {
        return ErrDisallowedScheme
    }
    if parsed.Host == "" {
        return ErrEmptyHost
    }
    return nil
}

func validateResolvedIP(ip net.IP) error {
    // Allow loopback
    if ip.IsLoopback() {
        return nil
    }
    // Block cloud metadata and link-local
    if isCloudMetadataIP(ip) || ip.IsLinkLocalUnicast() {
        return ErrDeniedIP
    }
    return nil
}
```

### 3.6 Data Cleanup: Reset Stale Failure Counts

After deploying the port fix (if not already deployed), run a one-time DB cleanup:

```sql
-- Reset failure counts for hosts/monitors stuck from the port mismatch era
-- Only reset monitors with elevated failure counts AND no recent successful heartbeat
UPDATE uptime_hosts SET failure_count = 0, status = 'pending' WHERE status = 'down';
UPDATE uptime_monitors SET failure_count = 0, status = 'pending'
WHERE status = 'down'
  AND failure_count > 5
  AND id NOT IN (
    SELECT DISTINCT monitor_id FROM uptime_heartbeats
    WHERE status = 'up' AND created_at > datetime('now', '-24 hours')
  );
```

This could be automated in `SyncMonitors()` or done via a migration.

---

## 4. Data Flow Diagrams

### Current Flow (Buggy)

```
[Proxy Host Created] → (no uptime action)
  → [Wait up to 60s for ticker]
  → SyncMonitors() creates monitor (status: "pending")
  → CheckAll() runs:
      → checkAllHosts() TCP to ForwardHost:ForwardPort
      → If host up → checkMonitor() HTTP to public URL
      → DB updated
  → [Wait up to 30s for frontend poll]
  → Frontend displays status
```

### Proposed Flow (Fixed)

```
[Proxy Host Created]
  → SyncMonitors() or SyncAndCheckForHost() immediately
  → Monitor created (status: "pending")
  → Frontend shows "PENDING" (yellow indicator)
  → Immediate checkMonitor() in background goroutine
  → DB updated (status: "up" or "down")
  → Frontend polls in 30s → shows actual status
```

---

## 5. Implementation Plan

### Phase 1: Playwright E2E Tests (Behavior Specification)

Define expected behavior before implementation:

| Test | Description |
|------|-------------|
| New proxy host monitor appears immediately | After creating a proxy host, navigate to Uptime page, verify the monitor card exists |
| New monitor shows pending state | Verify "PENDING" badge before first check completes |
| Monitor status updates after check | Trigger manual check, verify status changes from pending/down to up |
| Verify no false DOWN on first load | Create host, wait for background check, verify status is UP (not DOWN) |

**Files**: `tests/monitoring/uptime-monitoring.spec.ts` (extend existing suite)

### Phase 2: Backend — Consolidate UptimeService Instance

1. Remove second `NewUptimeService` call at `routes.go` line 414
2. Pass `uptimeService` (line 226) to `NewUptimeHandler()`
3. Verify all handler operations use the shared instance
4. Update existing tests that may create multiple instances

**Files**: `backend/internal/api/routes/routes.go`

### Phase 3: Backend — Immediate Monitor Lifecycle

1. In `ProxyHostHandler.Create()`, after saving host: call `SyncMonitors()` or create a targeted `SyncAndCheckForHost()` method
2. Add `CheckAll()` call after initial `SyncMonitors()` in the background goroutine
3. Consider adding a `SyncAndCheckForHost(hostID uint)` method to `UptimeService` that:
   - Finds or creates the monitor for the given proxy host
   - Immediately runs `checkMonitor()` in a goroutine
   - Returns the monitor ID for the caller

**Files**: `backend/internal/services/uptime_service.go`, `backend/internal/api/handlers/proxy_host_handler.go`, `backend/internal/api/routes/routes.go`

### Phase 4: Frontend — Pending State Display

1. Add `isPending` check in `MonitorCard` component
2. Add yellow/gray styling for pending state
3. Add pulsing animation for pending badge
4. Add i18n key `uptime.pending` → "CHECKING..." for **all 5 supported languages** (not just the default locale)
5. Ensure heartbeat bar handles zero-length history gracefully

**Files**: `frontend/src/pages/Uptime.tsx`, `frontend/src/i18n/` locale files

### Phase 5: Backend — Optional `check_url` Field (Enhancement)

1. Add `CheckURL` field to `UptimeMonitor` model
2. Update `checkMonitor()` to use `CheckURL` if set
3. Update `SyncMonitors()` — do NOT overwrite user-configured `CheckURL`
4. Update API DTOs for create/update

**Files**: `backend/internal/models/uptime.go`, `backend/internal/services/uptime_service.go`, `backend/internal/api/handlers/uptime_handler.go`

### Phase 6: Frontend — Health Check URL in Edit Modal

1. Add optional "Health Check URL" field to `EditMonitorModal` and `CreateMonitorModal`
2. Show placeholder text: "Leave empty to use monitor URL"
3. Validate URL format on frontend

**Files**: `frontend/src/pages/Uptime.tsx`

### Phase 7: Testing & Validation

1. Run existing backend test suites (2178 LOC across 3 files)
2. Add tests for:
   - Single `UptimeService` instance behavior
   - Immediate monitor creation on proxy host create
   - `CheckURL` fallback logic
   - "pending" → "up" transition
3. Add edge case tests:
   - **Rapid Create-Delete**: Proxy host created and immediately deleted before `SyncAndCheckForHost` goroutine completes — goroutine should handle non-existent proxy host gracefully (no panic, no orphaned monitor)
   - **Concurrent Creates**: Multiple proxy hosts created simultaneously — verify `SyncMonitors()` from Create handlers doesn't conflict with background ticker's `SyncMonitors()` (no duplicate monitors, no data races)
   - **Feature Flag Toggle**: If `feature.uptime.enabled` is toggled to `false` while immediate check goroutine is running — goroutine should exit cleanly without writing stale results
   - **CheckURL with redirects**: `CheckURL` that 302-redirects to a private IP — redirect target must be validated against the deny list (SSRF redirect chain)
4. Run Playwright E2E suite with Docker rebuild
5. Verify coverage thresholds

### Phase 8: Data Cleanup Migration

1. Add one-time migration or startup hook to reset stale `failure_count` and `status` on hosts/monitors that were stuck from the port mismatch era
2. Log the cleanup action

---

## 6. EARS Requirements

1. WHEN a new proxy host is created, THE SYSTEM SHALL create a corresponding uptime monitor within 5 seconds (not waiting for the 1-minute ticker)
2. WHEN a new uptime monitor is created, THE SYSTEM SHALL immediately trigger a health check in a background goroutine
3. WHEN a monitor has status "pending" and no heartbeat history, THE SYSTEM SHALL display a distinct visual indicator (not DOWN red)
4. WHEN the server starts, THE SYSTEM SHALL run `CheckAll()` immediately after `SyncMonitors()` (not wait for first tick)
5. THE SYSTEM SHALL use a single `UptimeService` instance for both background checks and API handlers
6. WHERE a monitor has a `check_url` configured, THE SYSTEM SHALL use it for health checks instead of the monitor URL
7. WHEN a monitor's host-level TCP check succeeds but HTTP check fails, THE SYSTEM SHALL record the specific failure reason in the heartbeat message
8. IF the uptime feature flag is disabled, THEN THE SYSTEM SHALL skip all monitor sync and check operations

---

## 7. Acceptance Criteria

### Must Have

- [ ] WHEN a new proxy host is created, a corresponding uptime monitor exists within 5 seconds
- [ ] WHEN a new uptime monitor is created, an immediate health check runs
- [ ] WHEN a monitor has status "pending", a distinct yellow/gray visual indicator is shown (not red DOWN)
- [ ] WHEN the server starts, `CheckAll()` runs immediately after `SyncMonitors()`
- [ ] Only one `UptimeService` instance exists at runtime

### Should Have

- [ ] WHEN a monitor has a `check_url` configured, it is used for health checks
- [ ] WHEN a monitor's host-level TCP check succeeds but HTTP check fails, the heartbeat message contains the failure reason
- [ ] Stale `failure_count` values from the port mismatch era are reset on deployment

### Nice to Have

- [ ] Dedicated UI indicator for "first check in progress" (animated pulse)
- [ ] Automatic detection of health endpoints (try `/health` first, fall back to `/`)

---

## 8. PR Slicing Strategy

### Decision: 3 PRs

**Trigger reasons**: Cross-domain changes (backend + frontend + model), independent concerns (UX fix vs backend architecture vs new feature), review size management.

### PR-1: Backend Bug Fixes (Architecture + Lifecycle)

**Scope**: Phases 2, 3, and initial CheckAll (Section 3.4)

**Files**:

- `backend/internal/api/routes/routes.go` — consolidate to single UptimeService instance, add CheckAll after initial sync
- `backend/internal/services/uptime_service.go` — add `SyncAndCheckForHost()` method
- `backend/internal/api/handlers/proxy_host_handler.go` — call SyncAndCheckForHost on Create
- Backend test files — update for single instance, add new lifecycle tests
- Data cleanup migration
- `ARCHITECTURE.md` — update to reflect the UptimeService singleton consolidation (architecture change)

**Dependencies**: None (independent of frontend changes)

**Validation**: All backend tests pass, no duplicate UptimeService instantiation, new proxy hosts get immediate monitors, ARCHITECTURE.md reflects current design

**Rollback**: Revert commit; behavior returns to previous (ticker-based) lifecycle

### PR-2: Frontend Pending State

**Scope**: Phase 4

**Files**:

- `frontend/src/pages/Uptime.tsx` — add pending state handling
- `frontend/src/i18n/` locale files — add `uptime.pending` key
- `frontend/src/pages/__tests__/Uptime.spec.tsx` — update tests

**Dependencies**: Works independently of PR-1 (pending state display improves UX regardless of backend fix timing)

**Validation**: Playwright E2E tests pass, pending monitors show yellow indicator

**Rollback**: Revert commit; pending monitors display as DOWN (existing behavior)

### PR-3: Custom Health Check URL (Enhancement)

**Scope**: Phases 5, 6

**Files**:

- `backend/internal/models/uptime.go` — add CheckURL field
- `backend/internal/services/uptime_service.go` — use CheckURL in checkMonitor
- `backend/internal/api/handlers/uptime_handler.go` — update DTOs
- `frontend/src/pages/Uptime.tsx` — add form field
- Test files — add coverage for CheckURL logic

**Dependencies**: PR-1 should be merged first (shared instance simplifies testing)

**Validation**: Create monitor with custom health URL, verify check uses it

**Rollback**: Revert commit; GORM auto-migration adds the column but it remains unused

---

## 9. Risk Assessment

| Risk | Severity | Likelihood | Mitigation |
|------|----------|------------|------------|
| Consolidating UptimeService instance introduces race conditions | High | Low | Existing mutex protections are designed for shared use; run race tests with `-race` flag |
| Immediate SyncMonitors on proxy host create adds latency to API response | Medium | Medium | Run SyncAndCheckForHost in a goroutine; return HTTP 201 immediately |
| "pending" UI state confuses users who expect UP/DOWN binary | Low | Low | Clear tooltip/label: "Initial health check in progress..." |
| CheckURL allows SSRF if user provides malicious URL | High | Low | Layered SSRF defense (see Section 3.5.1): write-time validation (scheme, length, parse), check-time re-validation, DNS rebinding protection (pin resolved IP against deny list), redirect chain validation. Allow loopback for self-referencing checks; block cloud metadata IPs (`169.254.169.254`, `fd00::`, etc.) |
| Data cleanup migration resets legitimate DOWN status | Medium | Medium | Only reset monitors with elevated failure counts AND no recent successful heartbeat |
| Self-referencing check (Charon) still fails due to Docker DNS | Medium | High | **PR-3 scope**: When `SyncMonitors()` creates a monitor, if `ForwardHost` resolves to loopback (`localhost`, `127.0.0.1`, or the container's own hostname), automatically set `CheckURL` to `http://{ForwardHost}:{ForwardPort}/` to bypass the DNS/Caddy round-trip. Tracked as technical debt if deferred beyond PR-3 |

---

## 10. Validation Plan (Mandatory Sequence)

0. **E2E environment prerequisite**
   - Determine rebuild necessity per testing policy: if application/runtime or Docker input changes are present, rebuild is required.
   - If rebuild is required or the container is unhealthy, run `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`.
   - Record container health outcome before executing tests.

1. **Playwright first**
   - Run targeted uptime monitoring E2E scenarios.

2. **Local patch coverage preflight**
   - Generate `test-results/local-patch-report.md` and `test-results/local-patch-report.json`.

3. **Unit and coverage**
   - Backend coverage run (threshold >= 85%).
   - Frontend coverage run (threshold >= 85%).

4. **Race condition tests**
   - Run `go test -race ./backend/internal/services/...` to verify single-instance thread safety.

5. **Type checks**
   - Frontend TypeScript check.

6. **Pre-commit**
   - `pre-commit run --all-files` with zero blocking failures.

7. **Security scans**
   - CodeQL Go + JS (security-and-quality).
   - GORM security scan (model changes in PR-3).
   - Trivy scan.

8. **Build verification**
   - Backend build + frontend build pass.

---

## 11. Architecture Reference

### Two-Level Check System

```
Level 1: Host-Level TCP Pre-Check
├── Purpose: Quickly determine if backend host/container is reachable
├── Method: TCP connection to ForwardHost:ForwardPort
├── Runs: Once per unique UptimeHost
├── If DOWN → Skip all Level 2 checks, mark all monitors DOWN
└── If UP → Proceed to Level 2

Level 2: Service-Level HTTP/TCP Check
├── Purpose: Verify specific service is responding correctly
├── Method: HTTP GET to monitor URL (or CheckURL if set)
├── Runs: Per-monitor (in parallel goroutines)
└── Accepts: 2xx, 3xx, 401, 403 as "up"
```

### Background Ticker Flow

```
Server Start → Sleep 30s → SyncMonitors()
  → [PROPOSED] CheckAll()
  → Start 1-minute ticker
  → Each tick: SyncMonitors() → CheckAll()
      → checkAllHosts() [parallel, staggered]
      → Group monitors by host
      → For each host:
          If down → markHostMonitorsDown()
          If up → checkMonitor() per monitor [parallel goroutines]
```

### Key Configuration Values

| Setting | Value | Source |
|---------|-------|--------|
| `batchWindow` | 30s | `NewUptimeService()` |
| `TCPTimeout` | 10s | `NewUptimeService()` |
| `MaxRetries` (host) | 2 | `NewUptimeService()` |
| `FailureThreshold` (host) | 2 | `NewUptimeService()` |
| `CheckTimeout` | 60s | `NewUptimeService()` |
| `StaggerDelay` | 100ms | `NewUptimeService()` |
| `MaxRetries` (monitor) | 3 | `UptimeMonitor.MaxRetries` default |
| Ticker interval | 1 min | `routes.go` ticker |
| Frontend poll interval | 30s | `Uptime.tsx` refetchInterval |
| History poll interval | 60s | `MonitorCard` refetchInterval |

---

## 12. Rollback and Contingency

1. **PR-1**: If consolidating UptimeService causes regressions → revert commit; background checker and API revert to two separate instances (existing behavior).
2. **PR-2**: If pending state display causes confusion → revert commit; monitors display DOWN for pending (existing behavior).
3. **PR-3**: If CheckURL introduces SSRF or regressions → revert commit; column stays in DB but is unused.
4. **Data cleanup**: If migration resets legitimate DOWN hosts → restore from SQLite backup (standard Charon backup flow).

Post-rollback smoke checks:
- Verify background ticker creates monitors for all proxy hosts
- Verify manual health check button produces correct status
- Verify notification batching works correctly

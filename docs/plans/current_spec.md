# Orthrus Tunnel Diagnosis & Fix Plan

**Feature Branch:** `feature/hecate`
**Date:** 2026-05-23
**Status:** Research complete — ready for implementation

---

## 1. Introduction

### Overview

Dockhand (a Docker management UI running on the VPS at `0.0.0.0:3001`) is configured to
reach HomeLab Docker via Charon's Orthrus reverse-WebSocket tunnel at `http://charon:3000`
(internal Docker network). The HomeLab Orthrus agent has been **offline for approximately
four hours** as of the time of writing, and the connection is actively unstable due to a
session race condition in the Charon server code.

Even when the connection was briefly stable this morning, the Server Muzzle HTTP allowlist
was blocking `/system/df` (Dockhand's disk-usage check), and the Agent Muzzle on HomeLab
(running old pre-fix code) would have blocked `/_ping`, volumes, and networks entirely.

### Goals

1. Restore a stable Orthrus tunnel between VPS Charon and the HomeLab agent.
2. Eliminate all confirmed API-path 403 blocks so Dockhand can list containers, volumes,
   networks, and inspect disk usage.
3. Optionally enable container action endpoints (start/stop/restart/kill) through the
   tunnel for full Dockhand functionality.
4. Document the exact HomeLab agent rebuild and redeploy steps.

### Non-Goals

- Changes to Hawser protocol (Dockhand uses `connection_type=direct`, not Hawser).
- Rate limiting or audit logging for the Docker proxy.
- Multi-agent support improvements beyond what is needed for this fix.

---

## 2. Research Findings

### 2.1 Request Flow

```
Dockhand UI (VPS, port 3001)
  │  (internal vps Docker network, host=charon)
  ▼
Charon container, port 3000
  └─ Server Muzzle (HTTP allowlist filter)
     └─ httputil.ReverseProxy → loopback 127.0.0.1:<ephemeral>
        └─ yamux stream (WebSocket to HomeLab 100.99.23.57)
           └─ HomeLab Orthrus Agent
              └─ Agent Muzzle (TCP-level HTTP filter)
                 └─ /var/run/docker.sock
```

Dockhand is configured in its SQLite database (`dockhand.db`) as:

| Field | Value |
|-------|-------|
| `name` | `HomeLab` |
| `connection_type` | `direct` |
| `host` | `charon` |
| `port` | `3000` |
| `protocol` | `http` |
| `hawser_*` | (all empty) |

Dockhand does **not** use the Hawser token protocol. It speaks plain Docker API HTTP to
`http://charon:3000`.

### 2.2 Server-Side Code (VPS — HEAD `b6ff258c`, current)

**File:** `backend/internal/orthrus/muzzle.go`

The Server Muzzle allowlist as of HEAD:

```go
var allowedDockerPaths = map[string]struct{}{
    "/_ping":            {},
    "/containers/json":  {},
    "/images/json":      {},
    "/info":             {},
    "/version":          {},
    "/events":           {},
    "/volumes":          {},
    "/networks":         {},
}

var allowedDockerPatterns = []string{
    "/containers/*/json",
    "/volumes/*",
    "/networks/*",
}
```

The `HEAD` method is only allowed for `/_ping`; all other methods except `GET` return 403.
Version prefix stripping (`/v1.44/...` → `/...`) is correctly implemented.

**Missing from Server Muzzle allowlist:** `/system/df` (confirmed blocked in live logs).

### 2.3 Agent-Side Code (HomeLab — OLD pre-fix code, `b6ff258c^`)

**File:** `agent/muzzle/muzzle.go` (as running on HomeLab)

```go
var allowedPatterns = []string{
    "/v*/containers/json",
    "/v*/containers/*/json",
    "/v*/info",
    "/v*/images/json",
    "/v*/version",
    "/v*/events",
}
```

No `/_ping` entry. No volumes or networks entries.

**Structural bug (present in both old AND new versions):**

```go
func (f *Filter) Allow(method, path string) bool {
    if !strings.EqualFold(method, http.MethodGet) {
        return false  // blocks HEAD before reaching path patterns
    }
    // ... pattern matching
}
```

This unconditionally rejects `HEAD` before any path check is reached. The fix commit
`b6ff258c` added `/_ping` and `/v*/_ping` to the patterns but did **not** fix this method
check. `HEAD /_ping` will still be blocked even after deploying `b6ff258c`.

### 2.4 Agent Deployment State

Charon database (`orthrus_agents` table, queried via `docker exec`):

| Field | Value |
|-------|-------|
| `name` | `HomeLab` |
| `status` | `online` (stale — DB not updated on disconnect) |
| `external_proxy_port` | `3000` |
| `resolved_address` | `100.99.23.57` |
| `last_seen` | `2026-05-23 06:24:26 -04:00` |
| `last_heartbeat` | (empty) |

The agent is **actually offline** as of 06:41:16 today. The `status=online` in the DB is a
stale value from the last successful connection before the race condition struck.

### 2.5 Live Evidence from Charon Logs

Orthrus log events today (filtered from Charon container logs):

| Time (EDT) | Event |
|------------|-------|
| `06:23:46` | Agent marked offline (Charon was rebuilding/restarting) |
| `06:23:56` | Agent reconnected, external proxy started on port 3000 |
| `06:24:16` | Agent marked offline (stale heartbeat killed 20s-old session) |
| `06:24:26` | Agent reconnected again, external proxy started on port 3000 |
| `06:24:55` | **Server Muzzle blocked `GET /system/df`** |
| `06:28:03` | **Server Muzzle blocked `GET /system/df`** (5-min interval) |
| `06:33:03` | **Server Muzzle blocked `GET /system/df`** |
| `06:38:03` | **Server Muzzle blocked `GET /system/df`** |
| `06:41:13` | Agent reconnected; `StartExternalProxy` FAILED: "address already in use" |
| `06:41:13` | Agent session stored in sessions map |
| `06:41:16` | **Agent marked offline** (3 seconds after connecting) |
| *(silence)* | No further Orthrus events — agent offline ~4 hours |

The `/system/df` blocks occur every 5 minutes, confirming Dockhand polls this endpoint on
a recurring schedule. The path is confirmed absent from the Server Muzzle allowlist.

### 2.6 Session Race Condition (Root Cause of 4-Hour Outage)

Reading `backend/internal/orthrus/server.go`:

**`HandleWebSocket`** stores the new session and starts a `watchHeartbeat` goroutine:
```go
s.sessions.Store(agent.UUID, session)  // replaces old session in map
go s.watchHeartbeat(agent.UUID, session)
```

**`watchHeartbeat`** (old goroutine, still running with reference to `oldSess`):
```go
func (s *OrthrusServer) watchHeartbeat(agentUUID string, sess *AgentSession) {
    ticker := time.NewTicker(s.heartbeatTimeout)  // 10s
    for {
        case <-ticker.C:
            if !sess.IsAlive() {
                _ = sess.Close()         // closes old TCP listeners (correct)
                s.markOffline(agentUUID) // marks agent offline in DB (kills new session DB state)
                s.sessions.Delete(agentUUID) // DELETES THE NEW SESSION from map
                return
            }
    }
}
```

**Race sequence:**

```
t=0:    oldSess active in map, oldWatchHB goroutine running
t=0:    oldSess yamux closes (agent disconnected)
t=3s:   newSess connects (HandleWebSocket called)
         → sessions.Store(uuid, newSess)  ← map now has newSess
         → newWatchHB goroutine started for newSess
         → StartExternalProxy(3000) FAILS ("address already in use")
           because oldSess.extListener still bound (oldWatchHB hasn't fired yet)
t=10s:  oldWatchHB ticker fires
         → oldSess.IsAlive() = false
         → oldSess.Close() ← frees port 3000 (good)
         → markOffline(uuid) ← writes "offline" to DB (corrupts newSess DB state)
         → sessions.Delete(uuid) ← REMOVES newSess from map (bad)
         newSess is orphaned: alive yamux, not in map, DB says offline
t=~40s: newSess yamux keepalive times out (no streams opened, default 30s)
t=~40s: agent reconnects (session3)
         → sessions.Store(uuid, session3)  ← map now has session3
         → StartExternalProxy(3000) succeeds (port freed at t=10s)
         → newWatchHB (from t=3s) still running, has reference to newSess
t=~50s: newWatchHB fires for dead newSess
         → markOffline(uuid) ← kills session3 DB state
         → sessions.Delete(uuid) ← removes session3 from map
         Infinite kill cycle: every new session deleted ~40s after connecting
```

This is why the agent has been offline for 4 hours — the kill cycle prevents stable reconnection.

### 2.7 Agent Reconnect Logic

`agent/leash/leash.go` `Run()` method:

```go
const (
    reconnectDelayDefault = 5 * time.Second
    reconnectDelayMax     = 60 * time.Second
    reconnectResetAfter   = 60 * time.Second
)
```

- Backoff is reset to 5s when a connection was stable for >= 60s.
- The 06:24:26 session lasted ~17 minutes, so the delay reset to 5s at 06:41:16.
- After each kill-cycle iteration (~40s), delay doubles: 5s → 10s → 20s → 40s → 60s.
- At 60s max backoff the agent retries every ~100s, but each connection is killed within 40s.
- This creates an infinite retry-and-kill loop, explaining the 4-hour outage.

### 2.8 Dockhand Docker API Paths

Based on confirmed behavior in Charon logs and standard Docker management UI patterns:

| Category | Method | Path |
|----------|--------|------|
| Health/ping | GET, HEAD | `/_ping`, `/v*/version` |
| System | GET | `/v*/info`, `/v*/events`, `/v*/system/df` |
| Containers (read) | GET | `/v*/containers/json`, `/v*/containers/{id}/json` |
| Container streams | GET | `/v*/containers/{id}/logs`, `/v*/containers/{id}/stats`, `/v*/containers/{id}/top` |
| Container actions | POST | `/v*/containers/{id}/start`, `/v*/containers/{id}/stop`, `/v*/containers/{id}/restart`, `/v*/containers/{id}/kill` |
| Images | GET | `/v*/images/json`, `/v*/images/{id}/json` |
| Volumes | GET | `/v*/volumes`, `/v*/volumes/{id}` |
| Networks | GET | `/v*/networks`, `/v*/networks/{id}` |

---

## 3. Confirmed Root Causes

### RC1 — Agent Muzzle missing `/_ping` (old code on HomeLab)
**Evidence:** `git show b6ff258c^:agent/muzzle/muzzle.go` — 6 patterns only, no `/_ping` entry.
**Impact:** `GET /v*/ping` → 403 at Agent Muzzle. Dockhand cannot confirm connectivity.
**Fix:** Deploy fix commit `b6ff258c` to HomeLab (adds `/_ping` and `/v*/_ping`).

### RC2 — Agent Muzzle unconditionally blocks HEAD method (both old and new code)
**Evidence:**
```go
// agent/muzzle/muzzle.go (both b6ff258c^ and b6ff258c)
if !strings.EqualFold(method, http.MethodGet) {
    return false  // HEAD never reaches pattern matching
}
```
**Impact:** `HEAD /_ping` → 403 even after deploying `b6ff258c`. Separate bug, not fixed in current commit.
**Fix:** Add a HEAD-specific guard before the GET check (see Fix B in Section 4).

### RC3 — Agent Muzzle missing volumes and networks paths (old code)
**Evidence:** Pre-fix `allowedPatterns` has no `/v*/volumes`, `/v*/networks` entries.
**Impact:** `GET /v*/volumes` and `GET /v*/networks` → 403 at Agent Muzzle.
**Fix:** Covered by deploying `b6ff258c` (adds these patterns).

### RC4 — Fix commit `b6ff258c` not deployed to HomeLab agent
**Evidence:** Agent last reconnected at 06:24:26 running old code. No automated deployment
pipeline for agent to HomeLab. Agent is a scratch image requiring full rebuild from source.
**Impact:** RC1 and RC3 remain active on HomeLab.
**Fix:** Rebuild agent from HEAD, deploy to HomeLab.

### RC5 — Server Muzzle missing `/system/df`
**Evidence:** Charon logs confirm four consecutive `"muzzle blocked disallowed Docker path","path":"/system/df"` entries at 5-minute intervals (06:24:55 to 06:38:03).
**Impact:** Dockhand disk-usage dashboard errors every 5 minutes.
**Fix:** Add `/system/df` to `allowedDockerPaths` in `backend/internal/orthrus/muzzle.go`.

### RC6 — Session race condition: stale watchHeartbeat goroutine deletes new session
**Evidence:**
- 06:41:13: "orthrus: agent connected" (new session stored in map)
- 06:41:16: "orthrus: agent marked offline" (3 seconds later — old watchHeartbeat fired)
- No reconnection for 4+ hours (infinite kill cycle, see Section 2.6)

**Impact:** Agent cannot maintain stable connection after any rapid reconnect. **Primary reason agent has been offline 4 hours.**
**Fix:** In `HandleWebSocket`, explicitly close and remove the old session before storing the new one, AND use `CompareAndDelete` in `watchHeartbeat` (see Fix D in Section 4).

### RC7 — External proxy port leak (consequence of RC6)
**Evidence:** 06:41:13 log: `"error":"listen tcp 0.0.0.0:3000: bind: address already in use"`.
**Root cause:** `StartExternalProxy` called on new session while old session's `extListener` still bound (old `watchHeartbeat` hasn't fired yet to call `sess.Close()`).
**Impact:** External proxy fails to start; Dockhand gets connection refused on port 3000.
**Fix:** Resolved as side-effect of Fix D (explicitly closing old session before starting new).

---

## 4. Technical Specifications

### Fix A — Server Muzzle: add `/system/df` and container read paths

**File:** `backend/internal/orthrus/muzzle.go`

Add to `allowedDockerPaths`:
```go
"/system/df":   {},
```

Add to `allowedDockerPatterns` for container streaming endpoints:
```go
"/containers/*/logs",
"/containers/*/stats",
"/containers/*/top",
```

The version-prefix stripping logic correctly handles `/v1.47/system/df` → `/system/df`.

### Fix B — Agent Muzzle: allow HEAD for `/_ping`

**File:** `agent/muzzle/muzzle.go`

Current `Allow()` method (both old and new code):
```go
func (f *Filter) Allow(method, path string) bool {
    if !strings.EqualFold(method, http.MethodGet) {
        return false
    }
    cleanPath := cleanDockerPath(path)
    for _, pattern := range f.patterns {
        if matchGlob(pattern, cleanPath) {
            return true
        }
    }
    return false
}
```

Fixed `Allow()` method (using the existing code structure — `allowedPatterns` package-level slice, `path.Match`, `path.Clean`):
```go
func (f *Filter) Allow(method, reqPath string) bool {
    // HEAD is permitted only for /_ping (Docker SDK connectivity check).
    // Check against the two versioned ping patterns already in allowedPatterns.
    if strings.EqualFold(method, http.MethodHead) {
        cleanPath := path.Clean(reqPath)
        for _, p := range []string{"/_ping", "/v*/_ping"} {
            if matched, _ := path.Match(p, cleanPath); matched {
                return true
            }
        }
        return false
    }

    if !strings.EqualFold(method, http.MethodGet) {
        return false
    }

    cleanPath := path.Clean(reqPath)
    for _, pattern := range allowedPatterns {
        if matched, err := path.Match(pattern, cleanPath); err == nil && matched {
            return true
        }
    }
    return false
}
```

Note: the agent muzzle does NOT strip version prefixes — it matches against versioned patterns (`/v*/_ping`, etc.) using `path.Match`. The `/_ping` and `/v*/_ping` entries added in commit `b6ff258c` are already in `allowedPatterns`, so the HEAD branch only needs to check those two patterns.

### Fix C — Agent Muzzle: add POST for container actions (optional, policy decision required)

**File:** `agent/muzzle/muzzle.go`

```go
var allowedPostPatterns = []string{
    "/v*/containers/*/start",
    "/v*/containers/*/stop",
    "/v*/containers/*/restart",
    "/v*/containers/*/kill",
}

// In Allow(), before the GET check:
if strings.EqualFold(method, http.MethodPost) {
    for _, pattern := range allowedPostPatterns {
        if matchGlob(pattern, path) {
            return true
        }
    }
    return false
}
```

The Server Muzzle also needs corresponding changes to allow POST for these paths (currently
all non-GET/non-HEAD-ping return 403).

**Confirm with operator before implementing** — enables Dockhand to stop/kill HomeLab
containers remotely.

### Fix D — Server: fix watchHeartbeat session race condition

**File:** `backend/internal/orthrus/server.go`

**Change 1: Displace old session BEFORE binding ports in `HandleWebSocket`**

The displacement must happen immediately after `NewAgentSession` succeeds and **before** `StartDockerProxy` / `StartExternalProxy`, so the old session's external proxy port (3000) is freed before the new session tries to bind it. Placing it just before `sessions.Store` is too late — `StartExternalProxy` would already have failed with "address already in use".

```go
session, err := NewAgentSession(agent.UUID, agent.Name, conn)
// ... error check ...

// Displace prior session BEFORE binding the external proxy port so the old
// session's extListener is closed and port 3000 is freed in time.
if old, loaded := s.sessions.LoadAndDelete(agent.UUID); loaded {
    if oldSess, ok := old.(*AgentSession); ok {
        _ = oldSess.Close()
    }
}

if err := session.StartDockerProxy(); err != nil { ... }
if agent.ExternalProxyPort > 0 {
    if err := session.StartExternalProxy(agent.ExternalProxyPort); err != nil { ... }
}
s.sessions.Store(agent.UUID, session)
```

**Change 2: Use generation-aware delete in `watchHeartbeat`**

Replace `s.sessions.Delete(agentUUID)` with `s.sessions.CompareAndDelete(agentUUID, sess)`.
`sync.Map.CompareAndDelete` (Go 1.20+) only removes the entry when the stored value is
still the same pointer as `sess`. A stale goroutine holding a reference to the old session
will find the entry has changed and the delete is a no-op.

```go
func (s *OrthrusServer) watchHeartbeat(agentUUID string, sess *AgentSession) {
    ticker := time.NewTicker(s.heartbeatTimeout)
    defer ticker.Stop()

    for {
        select {
        case <-s.ctx.Done():
            return
        case <-ticker.C:
            if !sess.IsAlive() {
                _ = sess.Close()
                // Only delete and mark offline if this session is still the current one.
                if s.sessions.CompareAndDelete(agentUUID, sess) {
                    s.markOffline(agentUUID)
                }
                return
            }
        }
    }
}
```

### Fix E — Agent rebuild and redeploy to HomeLab

The HomeLab agent is a scratch Docker image built via `agent/Dockerfile` and must be
rebuilt from source to pick up any code changes.

**Build:**
```bash
cd /projects/Charon
docker buildx build \
  --platform linux/amd64 \
  --file agent/Dockerfile \
  --tag orthrus-agent:latest \
  --load \
  .
```

**Export and transfer:**
```bash
docker save orthrus-agent:latest | gzip > /tmp/orthrus-agent.tar.gz
scp /tmp/orthrus-agent.tar.gz homelab:/tmp/
```

**On HomeLab (100.99.23.57) — mechanism to be confirmed:**
```bash
docker load < /tmp/orthrus-agent.tar.gz
# Restart the agent container using the local compose file
```

**Agent environment variables required:**
```
ORTHRUS_SERVER_URL=wss://charon.hatfieldhosted.com/api/v1/ws/orthrus/connect
ORTHRUS_AUTH_KEY=<token from Charon provisioning>
ORTHRUS_AGENT_ID=6f446cca-792f-4631-a4b8-8c3406cbc10c
ORTHRUS_DOCKER_SOCKET=/var/run/docker.sock
```

**ACTION REQUIRED:** SSH to HomeLab and determine current agent deployment mechanism
(systemd unit, Docker Compose file path, or manual binary) before implementing Phase 4.

### Fix F — Emergency: restart Charon to clear stuck state (no code change)

```bash
docker compose -f /home/jeremy/docker/containers/charon/docker-compose.yml restart charon
```

This clears all orphaned session goroutines and forces the HomeLab agent to reconnect.
Does not fix the underlying race condition — it will recur on the next rapid reconnect.

---

## 5. Implementation Plan

### Phase 0: Emergency Recovery (no code change, 15 min)

| Task | Action |
|------|--------|
| 0.1 | Restart Charon container on VPS to clear orphaned session state |
| 0.2 | Verify HomeLab agent reconnects within 60s (watch `docker logs charon -f`) |
| 0.3 | Confirm `/system/df` blocks still appear (connection alive, muzzle issue remains) |
| 0.4 | Confirm Dockhand HomeLab environment shows containers in UI |

Expected: Dockhand shows containers but disk-usage widget errors every 5 min.

---

### Phase 1: Playwright Tests

| Task | File | Test Description |
|------|------|-----------------|
| 1.1 | `tests/orthrus-muzzle.spec.ts` | Server Muzzle: `GET /_ping` returns 200 through tunnel |
| 1.2 | `tests/orthrus-muzzle.spec.ts` | Server Muzzle: `HEAD /_ping` returns 200 through tunnel |
| 1.3 | `tests/orthrus-muzzle.spec.ts` | `GET /system/df` returns 200 (not 403) |
| 1.4 | `tests/orthrus-muzzle.spec.ts` | `GET /containers/json` returns container list |
| 1.5 | `tests/orthrus-session.spec.ts` | Rapid reconnect does not kill new session |

---

### Phase 2: Backend — Server Muzzle and Session Race Fix

| Task | File | Change |
|------|------|--------|
| 2.1 | `backend/internal/orthrus/muzzle.go` | Add `/system/df` to `allowedDockerPaths` |
| 2.2 | `backend/internal/orthrus/muzzle.go` | Add container stream paths (logs, stats, top) |
| 2.3 | `backend/internal/orthrus/server.go` | `HandleWebSocket`: `LoadAndDelete` + `Close()` old session |
| 2.4 | `backend/internal/orthrus/server.go` | `watchHeartbeat`: use `CompareAndDelete` |
| 2.5 | `backend/internal/orthrus/server.go` | `watchHeartbeat`: only call `markOffline` when `CompareAndDelete` returns true |
| 2.6 | `backend/internal/orthrus/server_test.go` | Unit test: rapid reconnect does not corrupt session map |
| 2.7 | `backend/internal/orthrus/muzzle_test.go` | Unit test: `/system/df` passes Server Muzzle |

---

### Phase 3: Agent — Muzzle HEAD fix

| Task | File | Change |
|------|------|--------|
| 3.1 | `agent/muzzle/muzzle.go` | Fix `Allow()`: HEAD guard for `/_ping` before GET check |
| 3.2 | `agent/muzzle/muzzle.go` | Verify `/v*/_ping`, volumes, networks patterns from `b6ff258c` are present |
| 3.3 | `agent/muzzle/muzzle_test.go` | `HEAD /_ping` → allowed; `HEAD /containers/json` → blocked |
| 3.4 | `agent/muzzle/muzzle_test.go` | `GET /v1.47/_ping` → allowed |
| 3.5 | `agent/muzzle/muzzle_test.go` | `GET /v1.47/volumes` → allowed |

---

### Phase 4: Integration and Agent Deployment

| Task | Action |
|------|--------|
| 4.1 | SSH to HomeLab, determine agent deployment mechanism and compose file location |
| 4.2 | Build agent Docker image from HEAD of `feature/hecate` |
| 4.3 | Transfer image to HomeLab and restart agent container |
| 4.4 | Verify `HEAD /_ping` no longer blocked (no muzzle-blocked log entries) |
| 4.5 | Verify all Dockhand features: containers, volumes, networks, disk usage |

---

### Phase 5: Optional — Container Actions (policy confirmation required)

| Task | File | Change |
|------|------|--------|
| 5.1 | `backend/internal/orthrus/muzzle.go` | Allow POST for container action paths |
| 5.2 | `agent/muzzle/muzzle.go` | Allow POST for container action paths |
| 5.3 | Tests | `POST /containers/{id}/start` passes through tunnel |

---

## 6. Commit Slicing Strategy

All work ships in a **single PR on `feature/hecate`** with ordered logical commits.
One feature = one PR. Commits are ordered so each is independently reviewable and verifiable.

| Commit | Message | Files | Dependencies | Validation Gate |
|--------|---------|-------|-------------|----------------|
| **1** | `fix(orthrus): add /system/df and container stream paths to Server Muzzle allowlist` | `backend/internal/orthrus/muzzle.go`, `backend/internal/orthrus/muzzle_test.go` | none | Unit tests pass; no `/system/df` block in logs after Charon restart |
| **2** | `fix(orthrus): prevent stale heartbeat goroutine from killing reconnected session` | `backend/internal/orthrus/server.go`, `backend/internal/orthrus/server_test.go` | Commit 1 merged | Unit test: rapid reconnect keeps session alive; agent stays online after restart |
| **3** | `fix(agent): allow HEAD /_ping through agent muzzle filter` | `agent/muzzle/muzzle.go`, `agent/muzzle/muzzle_test.go` | Commit 2 merged | Unit tests pass; `HEAD /_ping` allowed, `HEAD /containers/json` blocked |
| **4** | `docs: add HomeLab Orthrus agent rebuild and redeploy guide` | `docs/implementation/orthrus_agent_deploy.md` | Commit 3 merged, Phase 4 complete | Deploy steps verified on HomeLab 100.99.23.57 |

**Optional Commit 5 (after policy confirmation):**
`feat(orthrus): allow container action POST requests through Orthrus tunnel`
Files: `backend/internal/orthrus/muzzle.go`, `agent/muzzle/muzzle.go`, tests.

### Rollback Notes

- Commits 1–3 are independently safe to revert: no DB schema changes, no API contract changes.
- The session race fix (Commit 2) changes behavior: old session is explicitly closed on
  reconnect rather than reaped up to 10s later by the heartbeat timer. This is strictly an
  improvement but worth calling out in the PR description.
- Commit 3 requires agent rebuild + redeploy on HomeLab. Rollback requires redeploying the
  old agent binary.
- No persistent state changes (DB migrations) in this plan.

---

## 7. Acceptance Criteria

| ID | Criterion | Verification |
|----|-----------|-------------|
| AC1 | `HEAD /_ping` returns 200 through full Orthrus tunnel | `curl -I http://charon:3000/_ping` from Dockhand container → HTTP 200 |
| AC2 | `GET /_ping` returns 200 through tunnel | `curl http://charon:3000/_ping` → `{"OK":true}` |
| AC3 | `GET /v*/containers/json` returns container list | Dockhand HomeLab UI shows container list |
| AC4 | `GET /v*/volumes` and `GET /v*/networks` return data | Dockhand volumes/networks views populate |
| AC5 | `GET /system/df` returns 200 (not 403) | No more `muzzle blocked /system/df` entries in Charon logs |
| AC6 | Agent stays connected 60+ minutes without being killed | No `agent marked offline` entries except on natural disconnect |
| AC7 | Rapid reconnect does not trigger kill cycle | Manual test: `docker restart charon` → agent reconnects and stays online; unit test passes |
| AC8 | All Agent Muzzle unit tests pass | `go test ./agent/muzzle/...` green |
| AC9 | All Server Muzzle and Orthrus server unit tests pass | `go test ./backend/internal/orthrus/...` green |
| AC10 | Dockhand HomeLab environment shows connected status | Dockhand UI at VPS:3001 — HomeLab environment green |

---

## 8. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| HomeLab agent deployment mechanism unknown | High | Medium | SSH to HomeLab and inspect before Phase 4 |
| `sync.Map.CompareAndDelete` unavailable (Go < 1.20) | Low | Low | Verify `go.mod` Go version; fallback: mutex-protected generation counter |
| Closing old session in `HandleWebSocket` interrupts active yamux streams | Low | Low | Only fires on reconnect — the old connection was already broken from agent side |
| Container actions (Phase 5) enabling destructive operations on HomeLab | Medium | High | Require explicit operator confirmation before Commit 5 |
| Restarting Charon (Phase 0) briefly drops all proxied services | Certain | Low | Schedule during low-traffic window; typically completes in <5s |

---

*Plan authored: 2026-05-23 | Branch: `feature/hecate` | Status: Ready for implementation*

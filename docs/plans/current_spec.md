# Technical Spec — Uptime Monitoring at Scale

**Status:** Draft for review
**Branch:** `feat/uptime-monitoring-scale`
**Delivery model:** One feature = one PR, sliced into ordered logical commits (see [Commit Slicing Strategy](#commit-slicing-strategy)).
**Author:** Planning (Principal Architect)
**Date:** 2026-08-27

---

## 1. Introduction

### 1.1 Overview

Charon's uptime subsystem was built for a handful of monitors. It degrades non-linearly as monitor count grows: the user runs ~100 monitors today and wants comfortable headroom to **500**. Under load, every monitor's latency number inflates together (checks queue behind each other and behind the single DB connection), and the Uptime page is slow to load because each monitor card issues its own history request.

This spec replaces the uptime **execution model** (global ticker → per-monitor scheduler + bounded worker pool), the **write model** (synchronous per-check DB writes → a buffered ingester mirroring `StatsIngester`), and the **read model** (N per-card history queries → one cached batch endpoint). It adds **heartbeat retention** (unbounded growth → hourly chunked pruner) and a small **admin config surface**.

### 1.2 Objectives & Goals (ranked — locked with user)

1. **Uptime page + heartbeat history UI loads fast at 100+ monitors.** Target: `GET /api/v1/uptime/monitors/summary` p95 < 300 ms at 500 monitors; the page issues **one** history request regardless of monitor count.
2. **Faster detection via per-monitor configurable intervals.** The stored `Interval` field becomes authoritative, with a **30 s hard floor** and an admin global default for new/legacy monitors.
3. **Latency numbers stay stable under load.** A check's measured latency reflects the target's real response time, not scheduler/queue backlog. Achieved via a bounded worker pool, a shared keep-alive HTTP client, and moving DB writes off the check's critical path.
4. **Throughput (supporting requirement):** all due checks for 500 monitors complete within their interval under normal conditions, and degrade *gracefully* (some checks delayed, metric incremented) rather than collapsing when many targets are slow/down.

### 1.3 Non-goals

- Scaling beyond 500 monitors, or moving off SQLite.
- Heartbeat downsampling / rollup tables (retention is hard-delete only).
- Changing the notification *content* or provider routing (only *when* transition detection fires relative to buffered writes).
- Distributed / multi-node checking.
- Touching `SetMaxOpenConns(1)` or the main GORM pool (explicitly out of scope — see §2.2).

---

## 2. Research Findings

### 2.1 Existing architecture (as-is)

| Concern | Current implementation | File / line |
|---|---|---|
| Scheduler | One global `time.NewTicker(1 * time.Minute)` in a `go func()` that also `time.Sleep(30s)` on boot; calls `SyncMonitors()` + `CheckAll()` every tick. `for range ticker.C` — **ignores `ctx`**, no graceful shutdown. | `backend/internal/api/routes/routes.go` ~652–690 |
| Per-monitor `Interval` | Stored on `models.UptimeMonitor.Interval` (seconds), default 60. **Read nowhere in the check path** — every monitor checks every 60 s. | `backend/internal/models/uptime.go:20` |
| Check fan-out | `CheckAll()` groups monitors by `UptimeHost`, then `go s.checkMonitor(m)` per monitor — **unbounded goroutines**, no pool. | `uptime_service.go:419–479` |
| Host TCP pre-check | `checkHost()` retries a dial `MaxRetries` (2) times with `time.Sleep(2 * time.Second)` between attempts — **blocks its goroutine up to ~4 s** on a down host, redundant with the `FailureThreshold` (2) cross-cycle debounce. | `uptime_service.go:524–645` |
| HTTP client | `network.NewSafeHTTPClient(...)` constructed **fresh per check** (`uptime_service.go:850`). `DisableKeepAlives: true`, `MaxIdleConns: 1` — full DNS + TCP + TLS every check. | `network/safeclient.go:410` |
| URL validation | `security.ValidateExternalURL()` does its own `net.Resolver{}.LookupIP` (Layer 1); `safeDialer` re-resolves at connect time (Layer 2, DNS-rebinding guard). Two lookups per HTTP check. | `security/url_validator.go:190,270` |
| DB | `SetMaxOpenConns(1)` — the whole backend is serialized through one SQLite connection (WAL, `busy_timeout=5000`, `synchronous=NORMAL`, `cache_size=-64000`). Every check does `s.DB.Create(&heartbeat)` + `s.DB.Save(&monitor)` synchronously, contending with all API traffic. | `database/database.go:140–145` |
| Heartbeats | `models.UptimeHeartbeat` — one row per monitor per check. Indexes on `MonitorID`, `Status`, `CreatedAt`, plus composite `idx_heartbeat_lookup (monitor_id, status, created_at)`. **No retention/pruning** — rows deleted only in `DeleteMonitor()` (`uptime_service.go:1319`). Unbounded growth. | `models/uptime.go:38–45` |
| History UI (N+1) | `frontend/src/pages/Uptime.tsx:24` — each `MonitorCard` runs its own `useQuery(['uptimeHistory', id])` → `GET /uptime/monitors/:id/history?limit=60` with `refetchInterval: 60000`. The monitor list refetches every 30 s. N monitors ⇒ N history requests/min, each a separate query through the one connection. `GetHistory` handler applies **no cap** to `limit`. | `Uptime.tsx:24`, `uptime_handler.go:58–69` |

### 2.2 Reference precedent — the Stats subsystem (mirror this)

`ARCHITECTURE.md` §"Stats Subsystem" (~lines 356–362) and §"Database (SQLite + GORM)" (~line 596) establish the pattern this feature must copy:

- **`StatsIngester`** (`backend/internal/services/stats_ingester.go`, 157 lines):
  - Non-blocking buffered channel `ingestCh` (`channelBufferSize = 1000`).
  - `Send()` does a `select { case ch <- e: default: droppedCount.Add(1); log.Warn(...) }` — **drop-on-full**, counter exposed.
  - `Run(ctx)` batches: flush every `flushInterval = 500ms` **or** when `batchSize = 100` rows accumulate, via `db.CreateInBatches(batch, batchSize)`.
  - On `ctx.Done()` it **drains** the channel then flushes (crash/shutdown safety); `Stop()` is a second drain for post-`Run` cleanup.
- **`StatsService`** (`stats_service.go`, 303 lines): read-side aggregation with a `summaryCache` struct — `sync.Mutex` + `cachedValue` + `expiresAt`, `summaryCacheTTL = 30 * time.Second`. `GetSummary` checks cache first, runs grouped/windowed SQL, sets cache.
- **Health metric:** `StatsHandler.GetStatsHealth` → `GET /api/stats/health` → `gin.H{"dropped_count": n}` (`stats_handler.go:164`).
- **Wiring:** `routes.go` ~801–808 — `statsIngester := services.NewStatsIngester(db); go statsIngester.Run(ctx)` using the `ctx` from `Register(ctx, ...)`.

The uptime feature adds a directly analogous `UptimeIngester`, an `UptimeSummaryService` with a 30 s TTL cache, and a `GET /api/v1/uptime/health` endpoint.

### 2.3 Runtime & infra facts confirmed

- SQLite driver: `github.com/glebarez/sqlite` (pure-Go, wraps `modernc.org/sqlite`, SQLite 3.4x). **Window functions supported.** `DELETE ... LIMIT` is **not** compiled in — the pruner must use the `WHERE id IN (SELECT id ... LIMIT n)` subquery form.
- API base path: `/api/v1`; uptime routes live in the `management` group (`routes.go` ~610). E2E specs address them as `**/api/v1/uptime/...`.
- `Register(ctx context.Context, router, db, cfg)` (`routes.go:72`) — `ctx` is already threaded and used for `go statsIngester.Run(ctx)` / `go statsWSHub.Run(ctx)`. Uptime background goroutines will use the same `ctx`.
- Config: generic key/value `models.Setting` (`Key`, `Value`, `Type`, `Category`). `GET /api/v1/settings` returns a `map[key]value`; `POST /api/v1/settings` (`SettingsHandler.UpdateSetting`, admin-gated) writes one key. New `uptime.*` keys work through this endpoint with no new route; key-specific validation is added in `UpdateSetting` (precedent: `backup.*` rejection, `security.admin_whitelist` validation at `settings_handler.go:143–156`).
- Feature flag `feature.uptime.enabled` (bool Setting) already gates the subsystem and is `FirstOrCreate`d in `routes.go:641`.
- E2E: `tests/monitoring/uptime-monitoring.spec.ts` and `tests/a11y/uptime.a11y.spec.ts` use **`page.route` interception** (mock JSON), not a live backend. New E2E specs follow the same mock-response style.
- Frontend consumers of uptime data: `frontend/src/pages/Uptime.tsx` (list + per-card history), `frontend/src/components/UptimeWidget.tsx` (list only — `getMonitors`, no history; unaffected by the N+1 fix but may adopt the summary endpoint opportunistically).

### 2.4 External dependencies

None added. All work is in existing packages (`net/http`, `context`, `time`, `sync`, GORM). One small **additive** option is added to `backend/internal/network/safeclient.go` (`WithKeepAlive(...)`) — no new module.

---

## 3. Technical Specifications

### 3.0 Component map

```
                         ┌────────────────────────────────────────────────────────┐
                         │  Register(ctx, ...)  — routes.go                        │
                         │  (replaces the global 60s ticker go-func)               │
                         └───────┬───────────────┬───────────────┬────────────────┘
             go .Run(ctx)        │               │               │
                 ┌───────────────▼──┐   ┌────────▼────────┐  ┌───▼──────────────┐
                 │ UptimeScheduler  │   │ UptimeSyncLoop  │  │ UptimePruner     │
                 │ tick ~5s         │   │ tick ~5m + event│  │ tick 1h          │
                 │ monSchedule map  │   │ SyncMonitors()  │  │ chunked DELETE   │
                 │ hostSchedule map │   └─────────────────┘  │ + deferred INDEX │
                 │ reads hostState ◄─────────────┐           └──────────────────┘
                 └───────┬──────────┘            │ (RLock read: "is this host down?")
                  enqueue│ UptimeJob{Kind: Monitor|Host}   (non-blocking, bounded chan cap 512)
                 ┌───────▼────────────────────────────────────────┐
                 │ UptimeWorkerPool (fixed N, default 30)          │
                 │  - shared SSRF-safe keep-alive *http.Client     │  ── HTTP/TCP/orthrus monitor check
                 │  - single-dial host TCP check (Kind==Host)      │
                 │  - monState map  {status,failCount,lastChange,  │  ── AUTHORITATIVE debounce state
                 │       lastNotifiedDown}  (seeded from DB, the    │     (NOT the scheduler DB snapshot)
                 │       source of truth for transition detection) │
                 │  - hostState map {status,failCount,lastChange}  │  ── written here, read by scheduler
                 │  - transition detection + notifications  (SYNC, on the worker, before enqueue)
                 │  - host→down: synthesize child `down` CheckResults for the host's TCP monitors (SYNC)
                 └───────┬────────────────────────────────────────┘
                  result │ CheckResult / HostCheckResult   (non-blocking chan, drop-on-full)
                         │  pool is the SOLE sender → pool closes this channel at shutdown
                 ┌───────▼──────────────────────────────┐
                 │ UptimeIngester  (dumb writer only)   │
                 │  - batch INSERT uptime_heartbeats    │  500ms / 100 rows
                 │  - coalesced UPDATE uptime_monitors  │  (status,latency,last_check,failure_count,…)
                 │  - coalesced UPDATE uptime_hosts     │  — pure column copy from pre-computed result;
                 │  - DroppedCount() metric             │    NO transition logic, NO fan-out
                 └──────────────────────────────────────┘

 Read path:  GET /api/v1/uptime/monitors/summary  ──►  UptimeSummaryService (30s TTL cache)
             GET /api/v1/uptime/monitors/:id/history (paginated, capped) ──► UptimeService.GetMonitorHistory
             GET /api/v1/uptime/health  ──►  UptimeIngester.DroppedCount() + pool queue depths + pool size
```

Two in-memory maps live on `UptimeWorkerPool` and are the crux of B2/B3:

- **`monState`** — authoritative per-monitor debounce state (`status`, `failureCount`, `lastStatusChange`, `lastNotifiedDown`). Seeded from `uptime_monitors` at pool start, then read-modify-written **synchronously by the worker** on every check result. Transition detection and notification dispatch read/write this map, **never** the scheduler's DB snapshot. The ingester's DB write of `status`/`failure_count` is a best-effort persistence *mirror* (used only to reseed on restart) — a dropped `CheckResult` cannot suppress a transition.
- **`hostState`** — authoritative per-host connectivity state (`status`, `failureCount`, `lastStatusChange`). Written synchronously by the worker running a `Kind==Host` job; read (RLock) by the scheduler to decide whether to skip enqueueing a down host's TCP monitors. The ingester's `uptime_hosts` write is likewise a mirror.

New files (all `backend/internal/services/` unless noted):

| File | Contents |
|---|---|
| `uptime_scheduler.go` | `UptimeScheduler` — due-selection loop over **two** in-memory maps (`monSchedule`, `hostSchedule`), jittered cold-start backfill, batched `next_check_at` write-back (monitors only), host-down short-circuit consult against `pool.hostState`, `Rehydrate()` for post-restore resync. |
| `uptime_worker_pool.go` | `UptimeWorkerPool` — fixed worker set, bounded job channel, shared keep-alive client, `Kind`-discriminated jobs (monitor / host), the authoritative `monState` + `hostState` maps (seeded from DB), synchronous transition detection + notification + host-down child fan-out, `Enqueue`/`TryEnqueue`, shutdown WaitGroup + closes `results`. |
| `uptime_ingester.go` | `UptimeIngester` — buffered heartbeat writes + coalesced monitor/host column updates (mirrors `StatsIngester`). **Dumb writer**: pure column copy from pre-computed results, no transition logic, no fan-out. Receives on a channel the pool owns and closes. |
| `uptime_pruner.go` | `UptimePruner` — hourly chunked retention delete (wider first-pass pause) + periodic `PRAGMA optimize` + deferred `CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created`, retried at the end of every clean+caught-up pass until it lands. |
| `uptime_summary_service.go` | `UptimeSummaryService` — one windowed query for recent beats + monitor metadata, 30 s TTL cache. |
| `uptime_check.go` | Extracted pure check logic: `runCheck(job, client) rawResult` and `runHostCheck(job, dialer) rawResult` (no DB writes, no client construction, no state-map access) — refactored out of `uptime_service.go:checkMonitor` / `checkHost`. Debounce + transition + fan-out live in the worker (`uptime_worker_pool.go`), which calls these. |
| `backend/internal/network/safeclient.go` | **edit** — add `WithKeepAlive(maxIdle, perHost int, idleTimeout time.Duration) Option`. |
| `backend/internal/models/uptime.go` | **edit** — add `NextCheckAt` field (+ index) to `UptimeMonitor` **only**. `UptimeHeartbeat` tags unchanged (index created lazily by the pruner). |
| `backend/internal/services/uptime_service.go` | **edit** — `SyncAndCheckForRemoteServer` / `SyncMonitorForRemoteServer`; check path routed through ingester; `checkHost` de-blocked; `GetMonitorHistory` gains `before` + cap. |
| `backend/internal/api/handlers/uptime_handler.go` | **edit** — `Summary`, `Health` handlers; interval-floor validation in `Create`/`Update`; cap + `before` on `GetHistory`; `CheckMonitor` enqueues via pool. |
| `backend/internal/api/handlers/remote_server_handler.go` | **edit** — `NewRemoteServerHandler` gains nil-guarded `*services.UptimeService`; create/update/delete drive targeted monitor sync + cleanup. |
| `backend/internal/services/backup_service.go` | **edit** — `RestoreBackupSafe` reconcile step calls `UptimeScheduler.Rehydrate()` after a live DB restore (§3.9 / S6). |
| `backend/cmd/api/main.go` | **edit** — `migrate` CLI: warning log + eager unconditional `CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created` (S7). |
| `backend/internal/api/routes/routes.go` | **edit** — replace ticker go-func with the background components (scheduler, sync loop, worker pool, ingester, pruner); register `/uptime/monitors/summary` + `/uptime/health`; seed 3 `uptime.*` Settings defaults; pass `uptimeService` into `NewRemoteServerHandler`. |
| `frontend/src/api/uptime.ts` | **edit** — `getMonitorsSummary(beats = 30)`, `MonitorSummary`/`BeatDTO` types, `before` on history; `interval` already present. |
| `frontend/src/pages/Uptime.tsx` | **edit** — list page owns one summary query; `MonitorCard` reads from props; `BEAT_BAR_SLOTS = 30`; interval field in create/edit forms with 30 s floor. |
| `frontend/src/pages/SystemSettings.tsx` | **edit** — new admin "Uptime Monitoring" card (3 `uptime.*` fields, bounds validation, restart note, feature-flag gated). |
| `frontend/src/hooks/useUptimeSummary.ts` | **new** — React Query hook wrapping `getMonitorsSummary(30)`. |

---

### 3.1 Component A — Real per-monitor scheduler

#### 3.1.1 Model change

`models.UptimeMonitor` gains:

```go
// NextCheckAt is the wall-clock time this monitor is next due for a check.
// Zero value ⇒ "due now" (legacy rows and freshly-created monitors).
NextCheckAt time.Time `json:"next_check_at" gorm:"index"`
```

Migration: `&models.UptimeMonitor{}` is already in the `db.AutoMigrate(...)` list (`routes.go:118`). GORM adds the nullable column + index automatically. `uptime_monitors` is small (≤ 500 rows) so this is sub-millisecond. No data backfill needed — a zero `NextCheckAt` is treated as "due".

#### 3.1.2 `UptimeScheduler`

```go
type UptimeScheduler struct {
    db           *gorm.DB
    pool         *UptimeWorkerPool
    cfg          *uptimeConfig            // hot-reloading snapshot (see §3.6)
    tick         time.Duration            // default 5s
    monSchedule  map[string]time.Time     // monitorID -> next due (in-memory source of truth)
    hostSchedule map[string]time.Time     // hostID    -> next due (in-memory only; NOT persisted)
    hostMinInt   map[string]int           // hostID    -> min(enabled child monitor intervals), clamped
    writeback    map[string]time.Time     // pending uptime_monitors.next_check_at persists
    known        map[string]struct{}      // monitorIDs already hydrated (for the new-monitor re-scan)
    mu           sync.Mutex
    now          func() time.Time         // injectable clock for tests
}

func (s *UptimeScheduler) Run(ctx context.Context)   // launched as `go s.Run(ctx)`
func (s *UptimeScheduler) Rehydrate()                // re-runs cold-start hydration; called after a live DB restore (§3.9)
```

**Cold-start hydration (`hydrate()`, run once at `Run` entry and again on `Rehydrate()`):**

1. **Monitors** — `SELECT id, interval, enabled, next_check_at, uptime_host_id FROM uptime_monitors WHERE enabled = true`.
   - For each: `effInterval = clampInterval(interval, cfg)` (§3.6.2); assign `monSchedule[id]` with **jittered backfill**:
     - `next_check_at` in the future → keep it.
     - past or zero → `due = now + rand(0s, min(effInterval, backfillWindow))`, `backfillWindow = 60s`.
   - This spreads past-due monitors uniformly over the first 60 s after boot (≈ `monitors/60` enqueues/s) instead of a 500-wide first tick.
   - Persist the backfilled monitor due-times in one batched write (see write-back below).
2. **Hosts** — `SELECT uptime_host_id AS id, MIN(interval) AS min_interval FROM uptime_monitors WHERE enabled = true AND uptime_host_id IS NOT NULL GROUP BY uptime_host_id`.
   - `hostMinInt[id] = clampInterval(min_interval, cfg)`.
   - `hostSchedule[id] = now + rand(0s, min(hostMinInt[id], backfillWindow))`.
   - Host due-times are **in-memory only** — no column is added to `uptime_hosts`, no write-back. Hosts are few (one per distinct upstream), so a cold-start host-check wave is trivially small; jitter still applies for tidiness.
3. `known` = the set of hydrated monitor IDs.

`Rehydrate()` re-runs `hydrate()` under `s.mu`, discarding stale in-memory schedule entries and rebuilding from the (restored) DB. It also calls `pool.ReseedState()` (§3.2.1) so the debounce maps re-sync. See §3.9.

**Per-tick loop (`ticker := time.NewTicker(s.tick)`):**

```
select {
case <-ctx.Done():
    flushWriteback()          // best-effort final persist
    return                    // STOP enqueuing — first link of the teardown chain (§3.1.4)
case <-ticker.C:
    if !featureEnabled() { continue }        // feature.uptime.enabled, cached 60s

    // (a) HOST pass — connectivity pre-checks
    hostDue := hostIDs where hostSchedule[id] <= now()   (cap 200/tick)
    for _, hid := range hostDue:
        host := loadHostSnapshot(hid)                     // batched SELECT, one query
        if s.pool.TryEnqueue(UptimeJob{Kind: JobHostCheck, Host: host}):
            hostSchedule[hid] = now() + durSecs(hostMinInt[hid])
        // else: leave due, retried next tick

    // (b) MONITOR pass
    monDue := monitorIDs where monSchedule[id] <= now()  (sorted by due asc, cap 200/tick)
    snaps := loadJobSnapshots(monDue)                     // one batched SELECT WHERE id IN (...)
    for _, job := range snaps:
        // host-down short-circuit: skip TCP-type monitors whose host is known-down.
        // hostState is written by the host-check worker; scheduler only reads (RLock).
        if job.Monitor.Type == "tcp" && job.Monitor.UptimeHostID != nil {
            if st, ok := s.pool.HostState(*job.Monitor.UptimeHostID); ok && st.Status == "down" {
                next := now() + durSecs(clampInterval(job.Monitor.Interval, cfg))
                monSchedule[job.Monitor.ID] = next; writeback[job.Monitor.ID] = next
                continue          // no enqueue; the host check drives recovery. Synthetic
                                  // `down` heartbeat was already written at the transition.
            }
        }
        if s.pool.TryEnqueue(UptimeJob{Kind: JobMonitorCheck, Monitor: job.Monitor}):
            next := now() + durSecs(clampInterval(job.Monitor.Interval, cfg))
            monSchedule[job.Monitor.ID] = next; writeback[job.Monitor.ID] = next
        // else: leave due, retried next tick; pool.EnqueueDropped()++

    // (c) new-monitor / disabled reconcile — every 6th tick (~30s)
    if tickCount%6 == 0 { rescan() }

    flushWriteback()          // one batched UPDATE per tick (grouped by value)
}
```

- **`loadJobSnapshots`**: one `SELECT ... FROM uptime_monitors WHERE id IN (<due ids>)` per tick. The snapshot supplies the worker's **static** fields (`Type`, `URL`, `MaxRetries`, `UptimeHostID`, `UpstreamHost`, `Interval`, `Enabled`). It also carries the dynamic columns (`Status`, `FailureCount`, `LastStatusChange`, `LastNotifiedDown`) but the worker **does not** use them for debounce — those come from the pool's authoritative `monState` map (§3.2.1 / §3.3.3). The snapshot's dynamic columns are only a diagnostic breadcrumb.
- **`loadHostSnapshot`**: one batched `SELECT ... FROM uptime_hosts WHERE id IN (<due host ids>)` per tick; supplies the host's identity and the ports its monitors dial (via a joined `uptime_monitors`/`proxy_hosts` lookup, exactly as `checkHost` does today).
- **Write-back batching** (monitors only): `flushWriteback()` groups `writeback` entries by identical `next` value and emits `UPDATE uptime_monitors SET next_check_at = ? WHERE id IN (?)` per group in one transaction — 1–3 statements per 5 s tick regardless of monitor count. On failure it logs and retries next tick (in-memory `monSchedule` is the runtime truth; a lost write-back risks one duplicate check after a crash, absorbed by cold-start jitter).
- **Manual check** (`POST /uptime/monitors/:id/check`) bypasses the schedule: handler calls `pool.Enqueue(ctx, job)` (blocking, 2 s timeout → 503) directly, does not touch `next_check_at`. It goes through the same `monState` lock as scheduled checks (§3.3.3), so a manual-vs-scheduled race no longer double-counts or under-counts the failure streak — whichever worker takes the lock first increments; the second sees the updated count.
- **New / re-enabled monitor**: `CreateMonitor` and `UpdateMonitor(enabled=true)` set `NextCheckAt = now()`; `rescan()` (every ~30 s) picks up `WHERE enabled = true AND id NOT IN (known)`, hydrates them (jittered), recomputes affected `hostMinInt`, and calls `pool.EnsureMonitorState(id)` to seed a `monState` entry.
- **Disabled monitor**: dropped from `monSchedule` / `known` on `rescan()`; the per-tick due scan is in-memory so it simply stops being enqueued. The worker also re-checks `job.Monitor.Enabled` and emits nothing if false (guards the ≤ 30 s race window).

#### 3.1.3 `SyncMonitors` off the hot path — `UptimeSyncLoop`

- Its own goroutine, `time.NewTicker(5 * time.Minute)`, `ctx`-aware.
- Also invoked opportunistically on mutation:
  - **Proxy hosts** (existing): `ProxyHostHandler` calls `go uptimeService.SyncAndCheckForHost(hostID)` on create, `SyncMonitorForHost(hostID)` on update, and iterates `WHERE proxy_host_id = ?` → `DeleteMonitor` on delete.
  - **Remote servers** (new, this PR): mirror the proxy-host pattern exactly. `UptimeService` gains three methods, analogous to the existing `SyncAndCheckForHost` / `SyncMonitorForHost` / delete cleanup:
    - `SyncAndCheckForRemoteServer(remoteServerID uint)` — ensure a monitor exists for the remote server (create if missing, using the same target-type/URL derivation `SyncMonitors` already does for `RemoteServer` rows — `tcp` host:port, or `http(s)://` / `orthrus` per `ConnectionType`), then run an immediate check. Per-server mutex via the existing `hostMutexes` map (key `remote-<id>`). Feature-flag gated like `SyncAndCheckForHost`.
      - **Orthrus remote servers with a not-yet-bound agent UUID:** when `ConnectionType == ConnectionTypeOrthrus` and `OrthrusAgentUUID` is `nil`/empty at create time, `SyncAndCheckForRemoteServer` **returns silently — no error, no monitor row created** (mirrors `SyncMonitors`'s existing `continue` at `uptime_service.go:300`). The `UptimeSyncLoop` (below) creates the monitor on a later pass once the agent connects and the UUID is persisted. This is the decided behavior, not a placeholder.
    - `SyncMonitorForRemoteServer(remoteServerID uint) error` — update the linked monitor's `Name`/`Type`/`URL`/`Enabled`/`UpstreamHost` from current `RemoteServer` fields; no-op (nil) if no monitor exists.
    - Auto-created monitors (proxy-host and remote-server alike) are created with the interval resolved from `uptime.default_interval_seconds` at write time, **not** a hardcoded 60 — see §3.6.3 (S3).
    - Delete cleanup runs inline in the handler (mirrors `ProxyHostHandler.Delete` at `proxy_host_handler.go:755-761`): `uptimeService.DB.Where("remote_server_id = ?", id).Find(&monitors)` → `DeleteMonitor(m.ID)` for each.
  - **Wiring:** `RemoteServerHandler` currently has no `UptimeService` reference. `NewRemoteServerHandler(service, ns)` (`remote_server_handler.go:24`) gains a third param `uptimeService *services.UptimeService` (nil-guarded, same as `ProxyHostHandler`). Call sites: `routes.go:897`. `RemoteServerHandler.Create` → `go h.uptimeService.SyncAndCheckForRemoteServer(server.ID)`; `.Update` → `go h.uptimeService.SyncMonitorForRemoteServer(server.ID)` (log on error); `.Delete` → inline monitor cleanup before `h.service.Delete(...)`.
  - The 5-minute `UptimeSyncLoop` remains the backstop for any mutation path that misses the targeted hook (e.g. direct DB edits, Orthrus agent-UUID late-binding).
- `CleanupStaleFailureCounts()` runs once at boot (kept, via the existing `runInitialUptimeBootstrap` path, minus `CheckAll()`).

#### 3.1.4 Graceful shutdown — explicit teardown handshake

Sharing `ctx` is not enough — an in-flight worker that `emit`s a `CheckResult` **after** the ingester has already returned on `ctx.Done()` loses that result's persistence (and if it was a transition, the in-memory `monState` has it but the DB never will, so a later restart reseeds slightly stale — bounded, see §3.9). The teardown is therefore an **ordered chain enforced by channel ownership**, not five components independently reacting to `ctx`:

1. **`UptimeScheduler`** sees `ctx.Done()` first thing in its select, does a final `flushWriteback()`, and returns. **No further enqueues happen after this.**
2. **`UptimeSyncLoop`** sees `ctx.Done()`, returns. (Independent; nothing depends on its ordering.)
3. **`UptimeWorkerPool.Run`** sees `ctx.Done()`: stops pulling from `jobs`, then `workerWG.Wait()` blocks until every worker goroutine has finished the check it was mid-flight on. Each check is bounded by the per-check hard deadline (`hardCap`, default 20 s — see §3.2.1), and the worker still `emit`s that final result. When `workerWG` is drained the pool **closes `results`** (the pool is the *sole* sender, so closing is safe) and returns.
3a. In-flight results emitted during step 3 land in `results` (or drop-on-full → `DroppedCount`, same as steady state — acceptable).
4. **`UptimeIngester.Run`** is structured as `for r := range results { buffer; flush on tick-or-count }`. It does **not** terminate on `ctx.Done()` — only on `results` being **closed** by the pool. `ctx.Done()` only stops its periodic flush *ticker* early (so the loop tightens to drain-and-final-flush). When `results` closes, it does one final `flush()` and returns. This guarantees every result emitted in step 3 is persisted.
5. **`UptimePruner`** sees `ctx.Done()` via its `ctx.Err()` check between chunks, aborts the current pass, returns. Independent.

**Grace-period requirement:** the process-level shutdown timeout (in `server.Run(ctx)` / the `http.Server.Shutdown` path) must be **≥ `hardCap` (20 s) + ~2 s** for the final ingester flush. **Verify** the existing server shutdown grace during implementation — if it is shorter than ~25 s, either raise it for this path or lower the uptime `hardCap`. (Documented as a C5 implementation check.)

**Test (C5):** start the full pipeline; enqueue a monitor check whose mock target blocks ~2 s; cancel `ctx` immediately; assert the resulting heartbeat row **is** written (no result loss for an in-flight check) and all goroutines exit.

#### 3.1.5 Removed / retired

- The `go func(){ time.Sleep(30s); ...; ticker := time.NewTicker(1*time.Minute); for range ticker.C {...} }()` block in `routes.go`.
- `UptimeService.CheckAll()` and `checkAllHosts()` **as the scheduling mechanism** — host connectivity checks are now scheduled by `UptimeScheduler`'s per-tick **host pass** (§3.1.2 step (a)), enqueued as `UptimeJob{Kind: JobHostCheck}` on the same bounded queue with the same drop-on-full semantics as monitor jobs. `CheckAll()` is **kept as an exported method** (used by `POST /api/v1/system/uptime/check` and tests) but re-implemented to *enqueue every enabled host + monitor into the pool* and return `(enqueued, dropped int)` (see §3.7 / N5) rather than spawning goroutines directly.
- `runInitialUptimeBootstrap` loses its `CheckAll()` call (the scheduler's jittered backfill covers the "no blind window on boot" goal; backfill window 60 s < old 90 s blind window).

---

### 3.2 Component B — Bounded worker pool + shared HTTP client

#### 3.2.1 `UptimeWorkerPool`

```go
type UptimeJobKind uint8
const (
    JobMonitorCheck UptimeJobKind = iota
    JobHostCheck
)

type UptimeJob struct {
    Kind    UptimeJobKind
    Monitor models.UptimeMonitor   // populated for JobMonitorCheck
    Host    models.UptimeHost      // populated for JobHostCheck
    Manual  bool                   // true for POST /:id/check
}

// monStateEntry / hostStateEntry are the AUTHORITATIVE debounce state (B2/B3).
type monStateEntry struct {
    status           string
    failureCount     int
    lastStatusChange time.Time
    lastNotifiedDown time.Time
}
type hostStateEntry struct {
    status           string
    failureCount     int
    lastStatusChange time.Time
}

type UptimeWorkerPool struct {
    db          *gorm.DB
    jobs        chan UptimeJob      // bounded, cap = queueCapacity (default 512)
    results     chan any            // CheckResult | HostCheckResult; cap = 2048; pool is sole sender & closes it
    ingester    *UptimeIngester     // Send target
    size        int                 // worker count (default 30)
    httpClient  *http.Client        // shared, keep-alive, SSRF-safe
    hostDialer  *net.Dialer         // shared, 3s timeout, for JobHostCheck + TCP monitors
    notifier    *UptimeService      // for queueDownNotification / sendRecoveryNotification (SYNC)

    monMu    sync.Mutex                       // guards monState (short RMW critical sections; see note)
    monState map[string]monStateEntry
    hostMu   sync.RWMutex                     // guards hostState (scheduler reads via RLock)
    hostState map[string]hostStateEntry

    workerWG    sync.WaitGroup
    enqDropped  atomic.Int64
}

func (p *UptimeWorkerPool) SeedState(ctx context.Context) error   // one-time DB→map seed; called before Run
func (p *UptimeWorkerPool) ReseedState() error                    // re-seed after a live DB restore (§3.9)
func (p *UptimeWorkerPool) EnsureMonitorState(id string)          // add a zero entry for a newly-created monitor
func (p *UptimeWorkerPool) Run(ctx context.Context)               // seeds (if not seeded), spawns p.size workers, owns teardown
func (p *UptimeWorkerPool) TryEnqueue(j UptimeJob) bool           // non-blocking; false + metric on full
func (p *UptimeWorkerPool) Enqueue(ctx, j UptimeJob) error        // blocking with 2s timeout (manual checks)
func (p *UptimeWorkerPool) QueueDepth() int                       // len(p.jobs)
func (p *UptimeWorkerPool) EnqueueDropped() int64
func (p *UptimeWorkerPool) HostState(hostID string) (hostStateEntry, bool)  // RLock; used by the scheduler
```

- **State seeding (`SeedState`, once before `Run`):**
  - `monState`: `SELECT id, status, failure_count, last_status_change, last_notified_down FROM uptime_monitors WHERE enabled = true` → one entry per monitor.
  - `hostState`: `SELECT id, status, failure_count, last_status_change FROM uptime_hosts` → one entry per host.
  - This is the debounce **source of truth** for the process lifetime. The ingester's later DB writes of these same columns are a persistence *mirror* consulted only by the next process's `SeedState`.
- **Worker loop:** `for j := range p.jobs { p.handle(ctx, j) }`, wrapped in `p.workerWG`. `handle` dispatches on `j.Kind`:
  - **`JobMonitorCheck`** → `raw := runCheck(ctx, j, p.httpClient)` (pure: HTTP/TCP/orthrus probe, no state) → **worker** takes `p.monMu`, reads `monState[id]`, applies the existing debounce (`success ⇒ up + failCount=0`; `fail ⇒ failCount++`, `down` at `failCount >= MaxRetries`), computes `StatusChanged`, writes the entry back, releases `monMu` → if `StatusChanged`, dispatch notification **synchronously** (§3.3.3) → `p.emit(CheckResult{...pre-computed...})`.
  - **`JobHostCheck`** → `raw := runHostCheck(ctx, j, p.hostDialer)` (pure: single TCP dial to any child-monitor port, 3 s) → **worker** takes `p.hostMu` (write), reads `hostState[hostID]`, applies the `FailureThreshold = 2` debounce, computes host `StatusChanged`, writes back, releases → **if host → `down`** (transition): for each of that host's `tcp`-type child monitors whose `monState` is not already `down`, the worker synthesizes a `CheckResult{HeartbeatStatus:"down", Latency:0, Message:"Host unreachable"}`, runs it through the same `monMu` debounce (so the child's `failureCount` maxes and `StatusChanged` is computed per child), fires the **consolidated** down-notification once via `notifier.queueDownNotification(...)` (the existing 30 s batch window coalesces the fan-out into one alert), and `p.emit`s each child result → **if host → `up`**: just update `hostState`; child TCP monitors resume on their next scheduler tick → `p.emit(HostCheckResult{...})` for the `uptime_hosts` row.
- **`p.emit`** is a non-blocking send to `p.results`; on full, `p.ingester.noteDropped()` increments the drop counter (§3.3). The ingester never distinguishes synthetic from real results — all are pre-computed column copies.
- **`monMu` contention:** a single `sync.Mutex` is adequate — the scheduler enqueues at most one job per monitor per cycle (and advances `next_check_at`), so per-monitor RMW is effectively serial; the only real concurrency is *different* monitors' workers contending for the map lock, and each critical section is ~5 field assignments. If profiling ever shows contention, shard by `fnv(monitorID) % 64` — a mechanical change, not a design one. Noted, not pre-optimized.
- **Queue capacity 512:** headroom for a cold-start thundering herd (500 monitors + their hosts) without unbounded memory; each `UptimeJob` ≈ 400 B ⇒ ~200 KB worst case. When full, `TryEnqueue` returns false and the scheduler retries that monitor/host next tick (graceful degradation: check delayed by ≤ 5 s per retry, not lost silently — `enqDropped` is exposed at `/uptime/health`).
- **Shutdown:** `Run` owns steps 3–3a of §3.1.4 — on `ctx.Done()` it stops pulling from `jobs`, `workerWG.Wait()`s, then `close(p.results)`.
- **Worker count default 30**, admin-configurable via `uptime.worker_pool_size` (§3.6). **Sizing guidance** (documented in `docs/features/uptime-monitoring.md`):
  `poolSize ≳ ceil( monitors / minIntervalSeconds × worstCaseCheckSeconds )`.
  For 500 monitors @ 30 s floor with a 5 s worst-case failing check ⇒ ≈ 83 — but that is the pathological "every target slow-failing simultaneously" case. Normal steady state (checks ≈ 50–300 ms) needs < 10 workers for 500 monitors. Default **30** covers normal operation with margin; 500-monitor deployments with many chronically-down targets should raise to **60–90**. Restart required to apply (pool sized at construction).
- **Per-check deadline:** `ctx, cancel := context.WithTimeout(parent, checkBudget)` where `checkBudget = min(clampInterval(interval), hardCap)`, `hardCap` default **20 s**. Connect timeout **3 s**; TLS handshake 10 s (unchanged); response-header timeout = remaining budget.

#### 3.2.2 Shared SSRF-safe HTTP client

Add to `backend/internal/network/safeclient.go` (additive, no behavior change to existing callers):

```go
// WithKeepAlive enables connection pooling on the SSRF-safe client.
// maxIdle: total idle conns kept; perHost: idle conns per host; idleTimeout: how long to keep them.
// The safeDialer, redirect policy, and all timeouts are unchanged — only
// DisableKeepAlives/MaxIdleConns/MaxIdleConnsPerHost/IdleConnTimeout are overridden.
func WithKeepAlive(maxIdle, perHost int, idleTimeout time.Duration) Option
```

Implementation: sets `cfg.keepAlive = true` and the three ints; in `NewSafeHTTPClient` the `http.Transport` fields become conditional on `cfg.keepAlive` (`DisableKeepAlives: !cfg.keepAlive`, `MaxIdleConns: cfg.maxIdle`, `MaxIdleConnsPerHost: cfg.perHost`, `IdleConnTimeout: cfg.idleTimeout`). Default (option not passed) is byte-for-byte the current behavior.

The pool constructs **one** shared client at startup:

```go
p.httpClient = network.NewSafeHTTPClient(
    network.WithTimeout(20*time.Second),            // hard ceiling; per-request ctx is tighter
    network.WithDialTimeout(3*time.Second),
    network.WithMaxRedirects(0),
    network.WithAllowLocalhost(),                   // parity with today's per-check client
    network.WithAllowRFC1918(),                     // parity with today's per-check client
    network.WithKeepAlive(100, 4, 30*time.Second),  // idleTimeout 30s — see below
)
```

- Security parity: today **every** uptime HTTP check already passes `WithAllowLocalhost()` + `WithAllowRFC1918()` and `WithMaxRedirects(0)`, so a single shared client with the same options is not a regression. `safeDialer` still validates the resolved IP at connect time on every **new** connection (DNS-rebinding / TOCTOU guard preserved). Link-local (169.254/16), cloud-metadata, and other reserved ranges remain blocked at both layers.
- **`idleTimeout` = 30 s** (not 90 s): a pooled idle connection skips Layer-2 re-resolution for its lifetime, so bounding that window to 30 s bounds the staleness (an *established* TCP connection cannot be re-bound to a new IP, so there is no actual SSRF here — R11 stays Low — but 30 s is the tighter, defensible choice). `safeclient_test.go` gains a case asserting a connection older than `idleTimeout` is **not** reused.
- **`MaxIdleConnsPerHost: 4` / `MaxIdleConns: 100`** assume meaningful per-host reuse. With ~500 distinct target hosts the idle pool churns and the keep-alive win shrinks — but repeat checks of the *same* host within 30 s still reuse the connection (the common case: each monitor re-checks its one host every ≥ 30 s), so it stays net-positive. Not tuned further; noted.
- `security.ValidateExternalURL(...)` is still called per HTTP check (Layer 1) with the same options as today.
- Keep-alive win: repeat checks of the same host reuse the TCP + TLS connection — the dominant cost at scale — so measured latency drops to the target's actual response time and stops absorbing handshake variance.

#### 3.2.3 Host TCP pre-check — scheduling + de-blocking

The `checkHost()` inner `for retry := 0; retry <= MaxRetries; retry++ { ... time.Sleep(2*time.Second) ... }` loop is **removed**, and host checks become first-class scheduled jobs.

**Scheduling (B1).** `UptimeScheduler` maintains `hostSchedule` alongside `monSchedule` (§3.1.2): every `UptimeHost` row is hydrated at cold start with an in-memory due-time (**not persisted** — no column added to `uptime_hosts`), `due = min(clamped intervals of its enabled child monitors)`, jittered over the first 60 s. The per-tick **host pass** (§3.1.2 step (a)) selects due hosts and enqueues `UptimeJob{Kind: JobHostCheck, Host: <snapshot>}` on the same bounded queue as monitor jobs, with the same `TryEnqueue` drop-on-full semantics. `hostMinInt` is recomputed on the ~30 s `rescan()` when child monitors are added/removed/re-intervalled.

**De-blocking.** `runHostCheck` does **a single TCP dial** (connect timeout 3 s, via the pool's shared `hostDialer`) to any one child-monitor port — no `time.Sleep` retry. The consecutive-failure debounce is **unchanged in outcome**: the worker applies `FailureThreshold = 2` against the authoritative `hostState` entry, so the host flips to `down` only after 2 consecutive failed host-check cycles. Dropping the sleep-retry removes up to ~4 s of blocked worker time per down host with no change to detection semantics.

**Host-down short-circuit — single owner: the worker (B2).** There is **no** ingester back-channel. When `runHostCheck` + debounce produces a host `up→down` transition, the **worker** (synchronously, exactly like a monitor transition in §3.3.3):
1. writes the new `down` state into the pool's `hostState` map;
2. for each of that host's `tcp`-type child monitors whose `monState` is not already `down`, synthesizes a `CheckResult{HeartbeatStatus:"down", Latency:0, Message:"Host unreachable"}`, runs it through the normal `monMu` debounce (child `failureCount` → max, per-child `StatusChanged` computed), and `emit`s it into the normal result stream — so the ingester writes those synthetic `down` heartbeats + column updates as ordinary dumb column copies;
3. fires **one** consolidated down-notification via `notifier.queueDownNotification(...)` — the existing 30 s batch window coalesces the fan-out into a single "N services down on host X" alert (unchanged behavior from `markHostMonitorsDown` today).

While the host stays `down`, the **scheduler** reads `pool.HostState(hostID)` (RLock) in its monitor pass and **skips enqueueing** that host's TCP monitors (advancing their `next_check_at` so they resume cleanly on recovery) — no new heartbeats are written for them, which is correct (nothing changed). On the host `down→up` transition the worker clears the `hostState` entry to `up`; the scheduler stops skipping and the TCP monitors resume normal checks on their next due tick, each re-evaluating its own status from its first real result.

HTTP / HTTPS / orthrus monitors are **never** short-circuited (URL-truth authoritative — unchanged).

The ingester remains a dumb writer throughout: it copies pre-computed `status` / `failure_count` / `last_status_change` / heartbeat rows for both real and synthetic results and never inspects a transition.

#### 3.2.4 Redundant second DNS lookup

**Accepted as-is, documented.** `ValidateExternalURL`'s `LookupIP` (Layer 1) and `safeDialer`'s connect-time resolution (Layer 2) are *deliberately* independent — collapsing them re-opens the DNS-rebinding TOCTOU window that Layer 2 exists to close. The cost is one extra `getaddrinfo` per HTTP check; with the OS resolver cache and (post-change) keep-alive amortizing connection setup, this is negligible relative to the check itself. A shared in-process DNS cache was considered and **deferred** (adds rebinding risk for marginal gain). Note added to `docs/features/uptime-monitoring.md` and code comment at the call site.

---

### 3.3 Component C — Heartbeat ingester (mirror `StatsIngester`)

#### 3.3.1 `CheckResult` (worker → ingester)

```go
type CheckResult struct {
    MonitorID        string
    HostID           string          // UptimeHostID, "" if none
    HeartbeatStatus  string          // "up" | "down" (raw check outcome)
    Latency          int64           // ms
    Message          string
    CheckedAt        time.Time

    // Pre-computed by the worker against the authoritative monState map
    // (§3.2.1 / §3.3.3), NOT the scheduler's DB snapshot — so the ingester only writes:
    NewMonitorStatus string          // resolved status after debounce ("up"|"down"|unchanged)
    FailureCount     int             // post-check failure counter (from monState, authoritative)
    StatusChanged    bool
    StatusChangedAt  time.Time       // set iff StatusChanged
    Synthetic        bool            // true for host-down child fan-out results (§3.2.3) — informational only
}

type HostCheckResult struct {
    HostID           string
    Status           string          // resolved after FailureThreshold debounce
    FailureCount     int
    Latency          int64
    Message          string
    CheckedAt        time.Time
    StatusChanged    bool
    StatusChangedAt  time.Time
}
```

The pool sends both types on one `chan any`; the ingester type-switches to route each to the `uptime_monitors` or `uptime_hosts` coalescing map. Neither carries any instruction the ingester acts on beyond "copy these columns".

#### 3.3.2 `UptimeIngester`

Structure mirrors `stats_ingester.go` almost line-for-line:

```go
const (
    uptimeChannelBufferSize = 2048     // 500 monitors * ~4 in-flight cycles
    uptimeBatchSize         = 100
    uptimeFlushInterval     = 500 * time.Millisecond
)

type UptimeIngester struct {
    db           *gorm.DB
    results      <-chan any        // CheckResult | HostCheckResult; created in routes.go, OWNED & CLOSED by the pool
    droppedCount atomic.Int64
}

func NewUptimeIngester(db *gorm.DB, results <-chan any) *UptimeIngester
func (i *UptimeIngester) noteDropped()                // called by the pool's emit() when the channel is full
func (i *UptimeIngester) DroppedCount() int64
func (i *UptimeIngester) Run(ctx context.Context)     // for r := range results { ... }; returns when results is CLOSED
func (i *UptimeIngester) Stop()                       // test-only: drain + flush for an instance whose Run isn't driven
```

**Channel ownership (differs from `StatsIngester`).** `StatsIngester` owns `ingestCh`; here the `results` channel is created in `routes.go`, given to the pool as `chan<- any` and to the ingester as `<-chan any`. The **pool is the sole sender and closes it** at shutdown (§3.1.4 step 3). `Run` is `for r := range results { buffer; flush on 500 ms-tick or 100-count }` and returns **only when `results` is closed** — `ctx.Done()` merely tightens the flush ticker so the tail drains fast; it does not end the loop. This is what guarantees no in-flight result is lost at shutdown.

**Flush (on 500 ms tick or 100 buffered results):**

1. **Heartbeat inserts** — `[]models.UptimeHeartbeat` from every buffered `CheckResult` (real *and* synthetic host-down children), `db.CreateInBatches(rows, uptimeBatchSize)`.
2. **Coalesced monitor updates** — `map[string]CheckResult` keyed by `MonitorID`, latest wins. Grouped `UPDATE uptime_monitors SET status=?, last_check=?, latency=?, failure_count=?, last_status_change=COALESCE(?, last_status_change) WHERE id=?` — one per distinct monitor. `next_check_at` untouched (scheduler owns it).
3. **Coalesced host updates** — `map[string]HostCheckResult` for `uptime_hosts` (`status`, `last_check`, `latency`, `failure_count`, `last_status_change`).

All inside **one** `db.Transaction(...)` per flush ⇒ ~2–4 write statements / 500 ms for the whole subsystem, vs today's 2 writes *per check*.

**These DB writes are a persistence MIRROR, not the source of truth.** Authoritative `status` / `failure_count` / debounce state lives in the pool's `monState` / `hostState` maps (§3.2.1). The ingester keeps the DB roughly current so the summary endpoint/UI have fresh data and the *next* process's `SeedState` has a good baseline. A dropped write costs at most one stale row until the next flushed check for that monitor — it **cannot** suppress or delay a transition (B3), because runtime detection never reads these columns.

**Drop-on-full metric:** `droppedCount` at `GET /api/v1/uptime/health`. Logged `Warn` (rate-limited) like `StatsIngester`.

**Shutdown semantics:** see §3.1.4 — the pool closing `results` (after its worker `WaitGroup` drains) is what ends `Run`, after one final `flush()`. A hard crash loses at most `uptimeFlushInterval` (500 ms) of un-flushed heartbeats — acceptable for monitoring data; the debounce maps are unaffected by the loss.

#### 3.3.3 Debounce + transition detection are authoritative in memory (B3)

**The failure-debounce counter must never depend on a droppable async DB round-trip.** Under sustained ingester saturation — the exact overload this feature targets — a design that recomputed `FailureCount` from the last-persisted row would drop successive failing-check results, never persist the increment, keep reading a stale-low count, never reach `maxRetries`, and **never fire the down alert**. So the counter is owned in memory:

1. The pool's `monState` map (`{status, failureCount, lastStatusChange, lastNotifiedDown}`) is **seeded once from the DB at pool start** (`SeedState`, §3.2.1) and is the debounce source of truth for the whole process lifetime.
2. On every check result the **worker**, holding `monMu`:
   - reads `monState[monitorID]`;
   - applies the existing debounce (`uptime_service.go` `checkMonitor` logic, ~lines 928–952): success ⇒ `status="up"`, `failureCount=0`; failure ⇒ `failureCount++`, `status="down"` once `failureCount >= job.Monitor.MaxRetries` (MaxRetries is a *static* config field — safe to read from the scheduler snapshot);
   - computes `StatusChanged = old.status != new.status && old.status != "pending"`;
   - writes the updated entry back (including `lastNotifiedDown` if it dispatches below); releases `monMu`.
3. If `StatusChanged`, the worker **synchronously** (before emitting the result) invokes the existing notification path:
   - `down` ⇒ `notifier.queueDownNotification(monitor, msg, durationStr)` (30 s batch window unchanged — coalesces multi-service outages);
   - `up` ⇒ `notifier.sendRecoveryNotification(monitor, durationStr)`.
4. The `CheckResult` carries the already-resolved `NewMonitorStatus` / `FailureCount` / `StatusChanged` / `StatusChangedAt`; the ingester copies them (mirror only).

Result: alerts fire on the worker goroutine that observed the transition, with **zero** buffering latency, and **a dropped `CheckResult` cannot delay or suppress a subsequent transition** — the next check reads the still-correct in-memory `monState`.

**Manual `POST /:id/check`** uses the same `monMu` + `monState`. A manual check racing a scheduled check of the same monitor no longer under-counts the failure streak (the old "both read N, both write N+1" hazard): the lock serializes the two RMWs. A double-*notify* in that narrow window is still possible and still deduped by `NotificationService` + `lastNotifiedDown` (5-min host-down guard / 30 s monitor-down batch) — documented, not fixed (unchanged).

**Restart reseed staleness (ties to §3.1.4 / §3.9):** after a hard crash mid-saturation, `SeedState` reads whatever the ingester last flushed — `failureCount` may be stale-low by a few. A monitor one failed check from `down` then needs 1–2 extra cycles post-restart to re-accumulate and fire. Bounded (≤ 2 intervals ≈ 60 s), self-correcting, alert still fires — just slightly later. Acceptable.

**Test (C5):** saturate the ingester (`results` full, every send dropping); feed a monitor `maxRetries` consecutive `down` results through the worker; assert the `down` transition **is** detected and `queueDownNotification` **is** called despite every `CheckResult` being dropped.

---

### 3.4 Component D — Retention pruner

#### 3.4.1 `UptimePruner`

```go
const (
    prunerInterval   = 1 * time.Hour
    pruneChunkSize   = 5000
    pruneChunkPause  = 50 * time.Millisecond     // steady-state: yield the single connection between chunks
    firstPassChunkPause = 250 * time.Millisecond // first (cold, huge) pass: yield longer — see §3.4.2 / N1
    walCheckpointRowThreshold = 50_000           // TRUNCATE checkpoint after a big prune
)

type UptimePruner struct {
    db       *gorm.DB
    cfg      *uptimeConfig     // reads uptime.heartbeat_retention_days (hot)
    now      func() time.Time
    firstPassDone atomic.Bool  // widens the chunk pause until the first clean pass completes
}

func (p *UptimePruner) Run(ctx context.Context)   // go p.Run(ctx); first pass ~30s after boot, then hourly
func (p *UptimePruner) pruneOnce(ctx) (deleted int64, err error)
```

**`pruneOnce`:**

```
cutoff := now().Add(-retentionDays * 24h)
pause  := pruneChunkPause; if !p.firstPassDone.Load() { pause = firstPassChunkPause }
total  := 0
for {
    if ctx.Err() != nil { return total, ctx.Err() }
    res := db.Exec(`
        DELETE FROM uptime_heartbeats
        WHERE id IN (
            SELECT id FROM uptime_heartbeats
            WHERE created_at < ?
            ORDER BY id
            LIMIT ?
        )`, cutoff, pruneChunkSize)
    total += res.RowsAffected
    if res.Error != nil { return total, res.Error }
    if res.RowsAffected < pruneChunkSize { break }   // caught up
    time.Sleep(pause)                                // release connection to API/ingester
}
if total >= walCheckpointRowThreshold {
    db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)       // reclaim WAL growth from a large prune
}
```

- Subquery form (not `DELETE ... LIMIT`) — required for the `modernc.org/sqlite` driver.
- `ORDER BY id` makes each chunk delete the oldest rows first and keeps the plan index-friendly (`id` is the PK).
- **Per-chunk latency, honest range:** on a warm table a 5 000-row chunk delete is ~10–30 ms. On a **cold, huge table (first pass)** each chunk can be **100–500 ms** — with `SetMaxOpenConns(1)` + `busy_timeout=5000` the ingester flush and API writes block for that window. So the first pass uses `firstPassChunkPause = 250 ms` (5× the steady-state pause) to keep the single connection available between chunks; steady-state hourly passes (tiny) use `50 ms`. Worst added API/ingester write latency during the first pass ≈ one chunk ≈ up to ~500 ms, intermittently, for the pass's duration.
- **`PRAGMA optimize`** runs on a 24 h sub-cadence (every 24th successful pass). **`VACUUM` is explicitly deferred** — it locks the whole DB and rewrites the file; WAL checkpoint reclaims most space for a hard-delete workload.

#### 3.4.2 First run on a large existing table — honest at the 500-monitor target

**Row-count math (corrected).** The "13 M rows" figure is the *current 100-monitor* case (100 × 1440 checks/day × 90 days). At this spec's **500-monitor target with the 90-day default retention** the table holds **≈ 65 M rows in steady state** (500 × 1440 × 90), or ≈ 130 M at a 30 s interval floor. An instance that has run for *years* past 90 days without pruning can hold several hundred million.

**What prune-first actually buys.** It bounds the **worst** case: on a multi-hundred-million-row instance, trimming to the ~65 M steady state *before* building the index avoids a `CREATE INDEX` over the full table (10+ minutes). It does **not** make the first-boot index build "seconds" — on a healthy 500-monitor instance the build still runs over ~65 M rows and is a **bounded multi-minute operation that contends for the single write connection** for its duration (readers still serve from WAL; writers — ingester flushes, API writes — see elevated latency and may drop-on-full while it runs). This is an honest, bounded first-boot-only cost, not a stall that is designed away.

**Why it is still acceptable (mitigations that hold):**
- It runs in a **background goroutine**, never on a request path or a blocking migration step. The server is up and serving throughout.
- The runtime **summary endpoint stays available** (correct results, slower, 30 s-cached) the whole time — no route downtime, no 503.
- It is **idempotent (`CREATE INDEX IF NOT EXISTS`) and retried at the end of every clean, caught-up prune pass** — a failed or `ctx`-interrupted attempt self-heals on the next pass; there is no "stuck unbuilt until restart" hole.
- Heartbeat writes that drop-on-full during the build self-heal on the next check (monitoring data).
- Operators who cannot tolerate the window: run `charon migrate` in a maintenance window (eager index build there, with an explicit warning log — §3.5.6 / S7), or temporarily **lower `uptime.heartbeat_retention_days` before first boot** to shrink both the first prune and the index build.

**The `uptime.heartbeat_retention_days` default stays 90** (user's explicit decision — not changed here).

**Ordering & retry.** `pruneOnce` returns `(deleted int64, err error)`. At the **end of every hourly pass** where `pruneOnce` returned `err == nil` and the chunk loop reached its "caught up" break (not a `ctx` abort), `Run` sets `firstPassDone` and issues `CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created ON uptime_heartbeats (monitor_id, created_at)`. On a healthy instance this lands on the first pass; on a huge instance the first pass trims first, then the (still multi-minute but bounded) build runs; a transient failure retries next hour. No `sync.Once`. Risk restated in §6 (R3/R4/R7).

---

### 3.5 Component E — Batch summary endpoint (kills the N+1)

#### 3.5.1 Route

```
GET /api/v1/uptime/monitors/summary?beats=<1..60>
```

Registered in `routes.go` next to the existing uptime routes; same auth (`management` group / JWT). `beats` optional, **default 30**, capped at 60. The Uptime page list view requests the default 30; an expanded/detail view may request up to 60.

#### 3.5.2 Response schema (snake_case, explicit `json` tags)

```jsonc
[
  {
    "id": "0f8c...-uuid",
    "name": "API Server",
    "type": "http",
    "url": "https://api.example.com",
    "enabled": true,
    "status": "up",                       // resolved monitor status
    "latency": 45,                        // ms, last check
    "last_check": "2026-08-27T12:00:00Z", // nullable
    "interval": 30,
    "proxy_host_id": 12,                  // nullable, for UI grouping
    "remote_server_id": null,
    "uptime_24h": 99.86,                  // % up over last 24h, computed from heartbeats (nullable if no data). Always present in the response.
    "recent_beats": [                     // chronological ASC, length <= beats param (default 30, cap 60)
      { "status": "up",   "latency": 44, "created_at": "2026-08-27T11:31:00Z" },
      { "status": "up",   "latency": 46, "created_at": "2026-08-27T11:31:30Z" },
      { "status": "down", "latency": 0,  "created_at": "2026-08-27T11:32:00Z" }
    ]
  }
]
```

Go types (in `uptime_summary_service.go`):

```go
type MonitorSummary struct {
    ID             string     `json:"id"`
    Name           string     `json:"name"`
    Type           string     `json:"type"`
    URL            string     `json:"url"`
    Enabled        bool       `json:"enabled"`
    Status         string     `json:"status"`
    Latency        int64      `json:"latency"`
    LastCheck      *time.Time `json:"last_check"`
    Interval       int        `json:"interval"`
    ProxyHostID    *uint      `json:"proxy_host_id"`
    RemoteServerID *uint      `json:"remote_server_id"`
    Uptime24h      *float64   `json:"uptime_24h"`
    RecentBeats    []BeatDTO  `json:"recent_beats"`
}

type BeatDTO struct {
    Status    string    `json:"status"`
    Latency   int64     `json:"latency"`
    CreatedAt time.Time `json:"created_at"`
}
```

#### 3.5.3 Query strategy — one windowed query, not N

`UptimeSummaryService.GetSummary(ctx, beats int) ([]MonitorSummary, error)`:

1. **Cache check** — `summaryCache`-style struct (`sync.Mutex` + value + `expiresAt`), `ttl = 30 * time.Second`, keyed by `beats`. Copied from `stats_service.go:36–56`. Hit ⇒ return.
2. **Monitor metadata** — `SELECT ... FROM uptime_monitors ORDER BY name ASC` (≤ 500 rows, one query).
3. **Recent beats** — one windowed query:

   ```sql
   SELECT monitor_id, status, latency, created_at
   FROM (
     SELECT monitor_id, status, latency, created_at,
            ROW_NUMBER() OVER (PARTITION BY monitor_id ORDER BY created_at DESC) AS rn
     FROM uptime_heartbeats
     WHERE created_at >= ?          -- now - 24h  (bounds the scan; also feeds uptime_24h)
   )
   WHERE rn <= ?                    -- beats
   ORDER BY monitor_id, created_at ASC;
   ```

   Backed by new index `idx_heartbeat_monitor_created (monitor_id, created_at)` (§3.5.6) — correct but slower without it.
4. **24h uptime** — one grouped query over the same window:

   ```sql
   SELECT monitor_id,
          SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END) * 100.0 / COUNT(*) AS pct
   FROM uptime_heartbeats
   WHERE created_at >= ?            -- now - 24h
   GROUP BY monitor_id;
   ```
5. **Assemble** in Go (map join on `monitor_id`), set cache, return.

Total: **3 SQL queries** regardless of monitor count (was N+1 HTTP round-trips + N queries). Steady-state p95 target < 300 ms at 500 monitors / 24 h of heartbeats (~720 rows/monitor in-window at 30 s → ~360 k row scan, index-covered).

**Automated perf gate (S5).** C7 adds `TestUptimeSummary_PerfBudget` (`-short`-skippable): seeds 500 monitors each with 24 h of 60 s-interval heartbeats (~360 k rows), builds `idx_heartbeat_monitor_created`, then asserts `GetSummary(ctx, 30)` (cache cleared) completes **under 2 s wall-clock** — a deliberately loose CI-stable ceiling. The < 300 ms p95 is the real target, tracked via the QA run's timing output but not the hard CI gate (runner variance). §7 acceptance criterion #4a.

#### 3.5.4 Detail history endpoint — kept, paginated, capped

`GET /api/v1/uptime/monitors/:id/history` stays for the expanded/detail view only. Changes to `UptimeHandler.GetHistory` + `UptimeService.GetMonitorHistory`:

- `limit`: default **60**, **hard cap 500** (currently uncapped `strconv.Atoi`). Values ≤ 0 → default.
- New optional `before` query param (RFC3339): returns heartbeats with `created_at < before`, for "load older" paging. Query: `WHERE monitor_id = ? AND created_at < ? ORDER BY created_at DESC LIMIT ?`.
- Response unchanged (`[]UptimeHeartbeat`).

#### 3.5.5 `GET /api/v1/uptime/health`

New `UptimeHandler.Health` (mirrors `StatsHandler.GetStatsHealth`):

```jsonc
GET /api/v1/uptime/health  →  200
{
  "heartbeats_dropped": 0,      // UptimeIngester.DroppedCount()
  "checks_enqueue_dropped": 0,  // UptimeWorkerPool.EnqueueDropped()
  "queue_depth": 3,             // UptimeWorkerPool.QueueDepth()
  "worker_pool_size": 30
}
```

#### 3.5.6 Index change — deferred creation (prune-first ordering)

The summary + history + prune access patterns want a composite `idx_heartbeat_monitor_created (monitor_id, created_at)`. The existing `idx_heartbeat_lookup (monitor_id, status, created_at)` on `models.UptimeHeartbeat` is **kept unchanged** (used elsewhere).

**The new index is NOT declared via a struct tag** and is NOT built by `AutoMigrate`. On a long-lived instance `uptime_heartbeats` can be millions of rows; building the index through the single connection at startup would block all writes for tens of seconds to minutes. Instead the index is created **after the retention pruner has trimmed the table**, so the build runs against an already-small dataset:

1. `models.UptimeHeartbeat` tags are left as they are today (no `idx_heartbeat_monitor_created` entry).
2. `UptimePruner` owns the index creation with a **retry-until-success** loop — no `sync.Once`. At the **end of every hourly pass** where `pruneOnce` returned `err == nil` and the chunk loop reached its "caught up" break (the first such pass runs ~30 s after boot; see §3.4.2), the pruner issues:
   ```sql
   CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created
     ON uptime_heartbeats (monitor_id, created_at);
   ```
   `CREATE INDEX IF NOT EXISTS` is idempotent: on a healthy instance it lands on the first pass and every later pass is a ~free no-op; if the first pass errored or was `ctx`-interrupted, the next hourly pass retries. There is no path where a transient early failure leaves the index unbuilt until a process restart.
3. **`charon migrate` CLI** (`backend/cmd/api/main.go`, the `case "migrate"` block with its own `db.AutoMigrate(...)` list): after `AutoMigrate`, log a warning and run the same `CREATE INDEX IF NOT EXISTS` **unconditionally**:
   ```go
   logger.Log().Warn("building index idx_heartbeat_monitor_created on uptime_heartbeats; " +
       "on a large database this can take several minutes and holds a write lock for the duration")
   db.Exec("CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created ON uptime_heartbeats (monitor_id, created_at)")
   ```
   `charon migrate` is **not** prune-first — it runs the build against the full table. That is acceptable *only* because the operator invoked it deliberately in a maintenance window; the warning makes the cost visible, and the Phase 5 deploy note (S7) tells operators to lower `uptime.heartbeat_retention_days` first if the table is huge and they want the build to finish quickly.
4. **Before the index exists** (server start → pruner's first successful pass), `UptimeSummaryService.GetSummary` **stays available with correct results** — the `ROW_NUMBER()` query falls back to `idx_heartbeat_lookup` (`(monitor_id, …)` prefix) or a `created_at >= now-24h`-bounded scan, cushioned by the 30 s TTL cache (≈ 2 executions/min). Slower, not broken; **never** 503-gated. `UptimeSummaryService` logs once when it first observes the index present (cheap `PRAGMA index_list` on a cache miss).

**Tradeoff (honest, per S2).** On a healthy 500-monitor instance the first-boot index build still runs over **~65 M rows** and is a **bounded multi-minute operation that contends for the single write connection** (readers unaffected — WAL). Prune-first only removes the *pathological* case (hundreds of millions of rows on years-stale instances). The reasons this is acceptable are in §3.4.2: background goroutine, no route downtime, retried-until-success, operator escape hatches. It is **not** claimed to be a "no-stall" design. See revised R3/R4/R7 in §6.

**Commit placement:** the index creation lives in **Commit 6 (retention pruner)**, not Commit 2. Commit 2 only adds `NextCheckAt` on `uptime_monitors` (a ≤ 500-row table — trivially fast via the existing struct-tag/AutoMigrate path). The `migrate`-CLI `CREATE INDEX IF NOT EXISTS` line also lands in Commit 6.

#### 3.5.7 Frontend changes

`frontend/src/api/uptime.ts`:

```ts
export interface BeatDTO { status: string; latency: number; created_at: string; }
export interface MonitorSummary {
  id: string; name: string; type: string; url: string; enabled: boolean;
  status: string; latency: number; last_check: string | null; interval: number;
  proxy_host_id?: number | null; remote_server_id?: number | null;
  uptime_24h: number | null; recent_beats: BeatDTO[];
}
export const getMonitorsSummary = async (beats = 30): Promise<MonitorSummary[]> => {
  const res = await client.get<MonitorSummary[]>(`/uptime/monitors/summary?beats=${beats}`);
  return res.data;
};
// getMonitorHistory gains an optional `before` cursor param.
export const getMonitorHistory = async (id: string, limit = 60, before?: string) => { ... };
```

`frontend/src/hooks/useUptimeSummary.ts` (new):

```ts
export const useUptimeSummary = () =>
  useQuery({ queryKey: ['uptimeSummary'], queryFn: () => getMonitorsSummary(30), refetchInterval: 30000 });
```

`frontend/src/pages/Uptime.tsx`:

- `Uptime` component calls `useUptimeSummary()` **once**. Grouping (`proxyHostMonitors` / `remoteServerMonitors` / `otherMonitors`) and alpha sort operate on `MonitorSummary[]`.
- `MonitorCard` prop type changes `UptimeMonitor` → `MonitorSummary`; it **no longer calls `useQuery(['uptimeHistory', ...])`**. `history` becomes `monitor.recent_beats`; `latestBeat`, `effectiveStatus`, the heartbeat bar, latency, and last-check all read from props. `hasHistory = recent_beats.length > 0`.
- Heartbeat bar: the fixed "last 60" grid becomes `recent_beats.length` wide (default 30), padded with empty slots up to a `BEAT_BAR_SLOTS` constant (set to 30 to match the new default). The `title`/tooltip copy ("Last 60 checks") updates to reflect the slot count. An expanded/detail view can request 60 and render the wider bar.
- `checkMutation` success handler invalidates `['uptimeSummary']` (was `['monitors']` + `['uptimeHistory', id]`).
- `deleteMutation` / `toggleMutation` / `syncMutation` invalidate `['uptimeSummary']`.
- Create/Edit modals: `interval` `<input>` `min="30"` (was `10`); on blur/submit clamp `< 30 → 30`; helper text "Minimum 30 seconds". Submit sends `interval` unchanged otherwise.
- Detail/expanded view (existing "Configure" path or a future drill-in) is the only remaining caller of `getMonitorHistory`, now with `limit`/`before` paging.
- **Manual "check now" / "sync" queue-full feedback (N5):** `POST /uptime/monitors/:id/check` returns `503 {"error":"check queue is full, try again"}` when the pool is saturated; `POST /system/uptime/check` returns `{"enqueued": N, "dropped": M}`. `checkMutation` / `syncMutation` surface a `503` (or any `dropped > 0`) as a toast — "Check queue full, try again in a moment" — instead of a silent success. `api/uptime.ts` types: `checkMonitor` may reject with a 503; `syncMonitors` response gains `enqueued` / `dropped`.
- `UptimeWidget.tsx` may switch `getMonitors` → `getMonitorsSummary` for consistency (**optional**, low-risk; keep `getMonitors` for now if it complicates the commit).

`getMonitors` (plain list) is **retained** — still used by `UptimeWidget`, the proxy-host form, and tests.

---

### 3.6 Component F — Config surface

#### 3.6.1 New `models.Setting` rows (seeded in `routes.go`, `FirstOrCreate` like `feature.uptime.enabled`)

| Key | Type | Default | Bounds | Hot-reload? |
|---|---|---|---|---|
| `uptime.default_interval_seconds` | `int` | `60` | 30 – 86400 | **Yes** — scheduler reads via `uptimeConfig` snapshot (60 s TTL) when clamping legacy/zero intervals and hydrating new monitors. |
| `uptime.worker_pool_size` | `int` | `30` | 1 – 200 | **No** — pool sized at construction; change requires restart. `GET /uptime/health` surfaces the active value so operators can confirm. |
| `uptime.heartbeat_retention_days` | `int` | `90` | 1 – 3650 | **Yes** — `UptimePruner` reads the snapshot at the start of each hourly pass. |

`Category = "uptime"` on all three.

#### 3.6.2 `uptimeConfig` — hot-reloading snapshot

```go
type uptimeConfig struct {
    db   *gorm.DB
    mu   sync.RWMutex
    val  cachedUptimeCfg
    exp  time.Time
    now  func() time.Time          // injectable clock — test seam (N8)
    ttl  time.Duration             // default 60s
}
// snapshot() refreshes from the Settings table if c.now() > exp, else returns cached.
func (c *uptimeConfig) DefaultIntervalSeconds() int   // reads snapshot()
func (c *uptimeConfig) RetentionDays() int            // reads snapshot()
func (c *uptimeConfig) forceRefresh()                 // test-only: expire the cache now

func clampInterval(seconds int, cfg *uptimeConfig) int {
    if seconds <= 0 { seconds = cfg.DefaultIntervalSeconds() }
    if seconds < 30 { seconds = 30 }
    return seconds
}
```

Shared by `UptimeScheduler`, `UptimePruner`, and `UptimeService` (for monitor-creation default resolution — §3.6.3). Read-only; writes go through the normal Settings endpoint. `now` + `forceRefresh` exist so hot-reload tests can force a TTL expiry without sleeping.

**Field-name note:** `UptimeService` already has an unrelated `config UptimeConfig` field (timeout/threshold struct). The injected `*uptimeConfig` is stored as `s.uptimeCfg` to avoid the collision.

#### 3.6.3 Server-side interval-floor validation

- **`SettingsHandler.UpdateSetting`** — add a `uptime.*` branch (precedent: the `backup.*` / `security.admin_whitelist` branches at `settings_handler.go:143`): parse int, enforce the bounds in the table above, `400` with `{"error": "...", "error_code": "invalid_uptime_setting"}` on violation.
- **`UptimeHandler.Create`** — `CreateMonitorRequest.Interval`: if `> 0 && < 30` → `400 {"error": "interval must be at least 30 seconds"}`. If `0` → passed through; `CreateMonitor` resolves it to `cfg.DefaultIntervalSeconds()` at **write time** (via `clampInterval(interval, s.uptimeCfg)`) so the stored value is always concrete and visible in the UI.
- **`UptimeService.UpdateMonitor`** — the `interval` whitelist branch (`uptime_service.go:1295`) gains the same floor check via `clampInterval`; reject `> 0 && < 30` with a typed `ErrIntervalTooLow`; the handler maps it to `400`.
- **ALL monitor-creation paths route through the same write-time resolution (S3).** `CreateMonitor`, **and the auto-create sites** `SyncMonitors` (`uptime_service.go` ~223 and ~320), `SyncAndCheckForHost` (~1402), and the new `SyncAndCheckForRemoteServer` — currently every one of these hardcodes `Interval: 60` on the struct literal. Replace each `Interval: 60` with `Interval: clampInterval(0, s.uptimeCfg)` (i.e. resolve to `uptime.default_interval_seconds`), so proxy-host / remote-server monitors honour the admin global default instead of being pinned to 60 s. `clampInterval(0, …)` also floors correctly if an admin sets the default below 30.
- **`CreateMonitor` signature** unchanged (`name, url, type, interval, maxRetries`) — only the internal default/floor resolution changes.
- **Test (C5):** set `uptime.default_interval_seconds = 45`, force a `uptimeConfig` refresh, run `SyncAndCheckForHost` / `SyncAndCheckForRemoteServer`, assert the created monitor's `Interval == 45`.

#### 3.6.4 Frontend

**Per-monitor interval field** — covered in §3.5.7 (Create/Edit modal `min="30"` + clamp + helper text).

**Admin "Uptime" settings card — IN SCOPE for this PR (Commit 8).** Added to `frontend/src/pages/SystemSettings.tsx`, which already renders the `feature.uptime.enabled` toggle and already wires `getSettings` / `updateSetting` from `frontend/src/api/settings.ts` and the `Card` primitives (`components/ui/Card`). No new page or route.

- **Placement:** a new `<Card>` ("Uptime Monitoring", below the existing feature-flags card). Rendered only when `feature.uptime.enabled` is on (reuse the `featureFlags` query already in the file).
- **Fields** (three number inputs, seeded from the `settings` map returned by `getSettings`):

  | Field | Setting key | Input bounds (client, must match §3.6.1 server bounds) | Helper text |
  |---|---|---|---|
  | Default check interval (seconds) | `uptime.default_interval_seconds` | `min=30 max=86400 step=1` | "New monitors inherit this. Applies within ~60 s, no restart." |
  | Worker pool size | `uptime.worker_pool_size` | `min=1 max=200 step=1` | "Concurrent checks. **Requires a restart to take effect.** Current active value shown on the Uptime page health indicator." |
  | Heartbeat retention (days) | `uptime.heartbeat_retention_days` | `min=1 max=3650 step=1` | "Older heartbeats are permanently deleted. Applies within ~1 h, no restart." |

- **Validation:** client-side bounds check on blur/submit mirroring the server (`< min` / `> max` / non-integer ⇒ inline error, save button disabled). The server is still authoritative — a rejected `POST /api/v1/settings` (`400 { error_code: "invalid_uptime_setting" }`) surfaces as a toast.
- **Save:** one `useMutation` calling `updateSetting(key, String(value), 'uptime', 'int')` per changed field (same pattern as the existing `saveSettingsMutation` in the file), then `invalidateQueries(['settings'])`. Only changed fields are written. The three writes are **independent** — each key is a standalone tuning knob with no cross-coupling — so partial success (e.g. field 2 rejected by the server while fields 1 and 3 persist) is acceptable; the card re-reads `getSettings` after the mutation settles and re-renders from the actual persisted state, so the UI always reflects what is stored rather than what was attempted.
- **i18n:** add `systemSettings.uptime.*` keys (card title, three labels, three helper texts, validation messages) to the locale files.
- **No dedicated typed endpoint** — the generic `POST /api/v1/settings` with the `uptime.*` validation branch (§3.6.3) is sufficient; `backup.*`-style endpoint carve-out is not needed because these keys have no cron/side-effect coupling.

---

### 3.7 Error handling & edge cases

| Scenario | Behavior |
|---|---|
| Worker queue full (thundering herd / pool starvation) | `TryEnqueue` → false; scheduler leaves the monitor/host due, retries next 5 s tick; `checks_enqueue_dropped` increments; `WARN` (rate-limited). No goroutine leak, no lost monitor. |
| Ingester channel full | `emit` drops the result; `heartbeats_dropped` increments; the **DB row** is briefly stale until the next flushed check. Detection is **unaffected** — the authoritative `monState` was already updated synchronously by the worker (§3.3.3), and the notification (if any) already fired. |
| Failing monitor + sustained ingester saturation (B3) | Every `CheckResult` drops, so nothing persists — but the worker still increments `monState[id].failureCount` under `monMu` on each check, so the `down` transition **is** detected at `failureCount >= maxRetries` and the alert fires. The DB catches up whenever a flush next succeeds. |
| Process restart mid-cycle | `SeedState` reseeds `monState`/`hostState` from the DB mirror (≤ last flush); scheduler cold-start reads `next_check_at`, jitter-backfills past-due over 60 s. `failureCount` may be stale-low by a few → a near-transition monitor fires ≤ 1–2 cycles later. No stampede, no missed alert. |
| Monitor disabled while a check is in flight | Worker checks `job.Monitor.Enabled` and **emits nothing** if false; scheduler drops it from `monSchedule` on the next `rescan()` (≤ 30 s). |
| Monitor deleted while queued | Worker runs, ingester `UPDATE ... WHERE id = ?` affects 0 rows (no-op); a dangling heartbeat row is pruned by retention; `DeleteMonitor` bulk-deletes heartbeats so the window is tiny. `monState` entry is GC'd on the next `rescan()`. Accepted. |
| Host recovers | Host-check worker sets `hostState[hid]="up"`; scheduler stops skipping that host's TCP monitors; each resumes on its next due tick and re-derives status from its first real result. |
| `interval` below 30 via direct API | `400` (Create/Update). Legacy DB rows with `interval < 30` → `clampInterval` floors to 30 at schedule time; not rewritten unless the monitor is edited. |
| `interval = 0` (legacy rows / auto-created monitors) | `clampInterval` → `uptime.default_interval_seconds`. Auto-create paths now pass `0` deliberately (S3). |
| SQLite `database is locked` during ingester flush | Flush wrapped in `db.Transaction`; on lock error, log + keep the batch for the next flush (bounded retry, mirrors `createMonitorWithRetry`). Drop the batch after 3 failed flushes to bound memory. Detection unaffected (mirror only). |
| Pruner delete contends with API | `pruneChunkPause` between chunks releases the connection. **Steady-state**: 50 ms pause, ~10–30 ms/chunk. **First cold pass on a huge table**: 250 ms pause, up to ~500 ms/chunk — added API/ingester write latency is intermittent, up to ~one chunk, for the pass's duration (§3.4.2 / N1). |
| Summary query before `idx_heartbeat_monitor_created` exists | Runs unindexed (`idx_heartbeat_lookup` prefix / 24 h-bounded scan), correct results, 30 s-cached, **never 503**. Clears once the pruner builds the index (§3.5.6). |
| Window function unsupported (old SQLite) | Not possible with bundled `modernc.org/sqlite`. Guarded by a unit test; fallback per-monitor `LIMIT` loop documented, not implemented. |
| `feature.uptime.enabled = false` | Scheduler tick no-ops (flag cached 60 s); pool idle; ingester idle; **pruner still runs** (retention applies while checking is paused). Summary serves last-known data. |
| Manual `POST /uptime/monitors/:id/check` when pool saturated | `Enqueue` blocks ≤ 2 s then `503 {"error":"check queue is full, try again"}` — surfaced as a toast (N5). |
| Manual `POST /system/uptime/check` when pool saturated | `CheckAll()` returns `{"enqueued": N, "dropped": M}` (never a silent all-drop); frontend toasts if `dropped > 0` (N5). |
| Orthrus monitor, subsystem down | Unchanged (`"Orthrus subsystem unavailable"` → `down` heartbeat). |
| Summary endpoint before any heartbeats exist | `recent_beats: []`, `uptime_24h: null`, `status` from `uptime_monitors.status` (`"pending"`). |
| **Backup / restore** (S6) | See §3.9. Restore-then-restart = ordinary crash-recovery cold start. Live restore without restart = self-heals within one `rescan()` (≤ 30 s) + 1–2 check cycles per monitor; `UptimeScheduler.Rehydrate()` (called from the restore reconcile step) makes it immediate. |

### 3.8 Data flow

#### 3.8.1 One monitor check, end to end

```
scheduler tick (5s)
  └─ monDue = {m1 (interval 30, next_check_at 12:00:00)} , now=12:00:03
     ├─ loadJobSnapshots([m1])            ── 1 batched SELECT (static fields; dynamic cols ignored for debounce)
     ├─ m1.Type=="tcp" && host known-down?  ── pool.HostState(hid) → no → proceed
     ├─ pool.TryEnqueue(UptimeJob{Kind: Monitor, Monitor: m1})  ── ok
     ├─ monSchedule[m1] = 12:00:33 ; writeback[m1] = 12:00:33
     └─ flushWriteback()                  ── 1 grouped UPDATE uptime_monitors.next_check_at
worker (1 of 30)
  ├─ raw := runCheck(job, sharedClient)   ── ValidateExternalURL (L1 DNS) → client.Do (L2 safeDialer, keep-alive)
  │     └─ latency=44ms, success=true
  ├─ monMu.Lock()
  │     ├─ e := monState["m1"]            ── AUTHORITATIVE: {status:"down", failureCount:2, ...}
  │     ├─ success ⇒ new = {status:"up", failureCount:0, lastStatusChange:now}
  │     ├─ StatusChanged = ("down" != "up" && "down" != "pending") = true
  │     └─ monState["m1"] = new
  ├─ monMu.Unlock()
  ├─ notifier.sendRecoveryNotification(m1, "3m 12s")            ── SYNC, fires now (before emit)
  └─ pool.emit(CheckResult{m1, HeartbeatStatus:"up", Latency:44, Message:"HTTP 200",
                           NewMonitorStatus:"up", FailureCount:0, StatusChanged:true, StatusChangedAt:now})
ingester flush (≤500ms later) — MIRROR write, not source of truth
  └─ Transaction:
       ├─ CreateInBatches([]UptimeHeartbeat{ {m1,"up",44,"HTTP 200"} })
       └─ UPDATE uptime_monitors SET status='up', last_check=…, latency=44, failure_count=0,
            last_status_change=… WHERE id='m1'
      (if this flush is DROPPED: monState already says "up"; DB catches up on the next successful flush;
       the recovery alert already fired — nothing is lost that matters)
UI (next 30s refetch)
  └─ GET /uptime/monitors/summary  ── cache hit or 3 queries ── card shows UP, 44ms, sparkline
```

#### 3.8.2 Host goes down — short-circuit fan-out

```
scheduler host pass (5s tick)
  └─ hostDue = {h7}  → pool.TryEnqueue(UptimeJob{Kind: Host, Host: h7})   ; hostSchedule[h7] += minChildInterval
worker
  ├─ raw := runHostCheck(job, hostDialer)   ── single 3s dial to a child port → fail
  ├─ hostMu.Lock()
  │     ├─ hostState["h7"] = {status:"down"|"pending"→…, failureCount++}
  │     └─ failureCount >= 2 ⇒ transition up→down
  ├─ hostMu.Unlock()
  ├─ for each tcp child mN of h7 where monState[mN].status != "down":
  │     ├─ synthesize CheckResult{HeartbeatStatus:"down", Latency:0, Message:"Host unreachable", Synthetic:true}
  │     ├─ monMu: run it through the SAME debounce → monState[mN].status="down", StatusChanged per-child
  │     └─ pool.emit(that CheckResult)
  ├─ notifier.queueDownNotification(...)  ×1  ── SYNC; 30s batch window coalesces → one "N services down on h7" alert
  └─ pool.emit(HostCheckResult{h7, Status:"down", ...})
scheduler (subsequent monitor passes, while h7 down)
  └─ for tcp child mN of h7:  pool.HostState("h7").Status == "down"
        └─ SKIP enqueue; advance monSchedule[mN] += interval   (no new heartbeats — nothing changed)
ingester flush
  └─ writes the synthetic child `down` heartbeats + coalesced uptime_monitors / uptime_hosts column updates (dumb copy)
h7 recovers → host-check worker sets hostState["h7"].Status="up" → scheduler resumes enqueuing mN on next due tick
```

#### 3.8.3 Shutdown (teardown chain, §3.1.4)

```
ctx.Done()
  ├─ scheduler: final flushWriteback(); return        ── no more enqueues
  ├─ syncLoop:  return
  ├─ pool.Run:  stop reading `jobs` → workerWG.Wait() (each in-flight check ≤ hardCap 20s) → close(results)
  ├─ ingester:  for r := range results { ... }  drains until `results` closed → one final flush() → return
  └─ pruner:    ctx.Err() between chunks → abort current pass → return
  (process shutdown grace must be ≥ hardCap + ~2s — verify server.Run shutdown timeout, §3.1.4)
```

---

### 3.9 Backup / restore interaction (S6)

**Goroutine start ordering.** The uptime background components are launched inside `routes.Register(ctx, …)`, which runs **after** `database.Connect` and after any pending-restore boot-swap performed during `main.go` / database init. So on a **restore-then-restart** (the pending-restore path, and the recommended flow for `RehydrateLiveDatabase`), the scheduler/pool/pruner cold-start against the already-restored DB — identical to ordinary crash-recovery cold start, **no special handling required**. `SeedState` reseeds `monState`/`hostState` from the restored rows; the scheduler hydrates `next_check_at` (jitter-backfilling past-due entries — an old backup just means "everything is due", bounded by the 60 s spread + per-tick cap + queue cap, i.e. the R5 stampede mitigation already covers it).

**Live restore without a restart** (`RehydrateLiveDatabase` swaps table *contents* under the running `*gorm.DB` handle). Immediately after, the in-memory `monSchedule` / `hostSchedule` / `monState` / `hostState` reflect *pre-restore* data:
- Monitors deleted by the restore keep getting enqueued until the next `rescan()` (≤ 30 s), where the mirror `UPDATE ... WHERE id=?` affects 0 rows — harmless.
- Monitors added by the restore are picked up by the same `rescan()` (≤ 30 s) and seeded into `monState`.
- Existing monitors whose restored `status`/`failure_count` differ from the in-memory entry re-converge within 1–2 real check cycles (bounded, no corruption, alert timing shifts by ≤ ~2 intervals).

To make a live restore **immediate** rather than eventually-consistent, `RestoreBackupSafe`'s reconcile step (which already reloads Caddy config) also calls `UptimeScheduler.Rehydrate()` — which re-runs cold-start hydration under `s.mu` and calls `pool.ReseedState()`. This is a small, localized hook (the cold-start seeding code is already factored out for `Run`). Spec'd in **Commit 5** with a test (`backup_service` reconcile invokes `Rehydrate`; post-`Rehydrate` schedule/state match the restored DB).

**Large first prune after restoring an old backup.** If the restored DB is weeks/months stale, the pruner's next hourly pass is a large one — handled by the same chunked, wider-first-pass path as §3.4.2 (the `firstPassDone` flag resets on `Rehydrate()` so the wider pause reapplies). No separate handling.

---

## 4. Implementation Plan

### Phase 1 — E2E specs (`test.fixme`)

`tests/monitoring/uptime-monitoring-scale.spec.ts` (new), `page.route`-mocked, all `test.fixme` initially:

1. **Per-monitor interval honored** — create a monitor with `interval: 30`; mock `POST /uptime/monitors` echoing `interval: 30`; assert the create form rejects `interval: 10` client-side (min 30, helper text visible) and that the payload sent has `interval: 30`.
2. **Uptime page loads fast with many monitors** — mock `GET /uptime/monitors/summary` with a **100-monitor** fixture (each with `recent_beats` of 30, the new default); assert exactly **one** request to `**/uptime/monitors/summary`, **zero** requests to `**/uptime/monitors/*/history`, all 100 cards render with status badge + sparkline, and the page is interactive under a generous budget.
3. **Retention prunes old heartbeats** — this is backend-observable only; E2E asserts the **admin-facing signal**: `GET /uptime/health` mock returns `heartbeats_dropped: 0` and the settings round-trip for `uptime.heartbeat_retention_days` (set to 30, reload, value persists). The actual delete is covered by a Go test (Phase 2/6).
4. **Summary endpoint drives card state** — mock a monitor whose `status: "down"` with a trailing `down` beat; assert the card shows DOWN without any history call.

### Phase 2 — Backend

Ordered per the Commit Slicing Strategy (§Commit Slicing). Each backend commit is TDD (`backend-dev`): red test → implementation → green, `go test ./...` for the touched packages, `go build ./...`, `staticcheck`/`make lint-fast`. **New errors are wrapped `fmt.Errorf("context: %w", err)` per CLAUDE.md (N7)** — call it out in each commit's review.

- **Foundation:** `NextCheckAt` field (on `uptime_monitors` only) + `uptimeConfig` snapshot (with `now`/`forceRefresh` test seam, N8) + 3 Setting seeds + interval-floor validation (`SettingsHandler.UpdateSetting`, `UptimeHandler.Create`, `UptimeService.UpdateMonitor`, `CreateMonitor` write-time resolution). **No** heartbeat-table index change here (deferred to the Pruner step / §3.5.6). No runtime behavior change to checking yet.
- **Ingester:** `UptimeIngester` + `CheckResult` + `HostCheckResult` + tests (mirror `stats_ingester_test.go`: drop-on-full, batch by count, batch by timer, coalesced monitor **and** host updates, type-switch routing, `results`-closed terminates `Run` + final flush, `ctx.Done()` alone does **not** terminate). **Also constructed (not `Run`) in `routes.go` this commit (S1).**
- **Worker pool + shared client:** `network.WithKeepAlive` option (idleTimeout 30 s) + `safeclient` tests (keep-alive reuse; connection older than `idleTimeout` not reused; link-local/metadata still blocked); `UptimeWorkerPool` with `Kind`-discriminated jobs, `monState` + `hostState` maps + `SeedState`/`ReseedState`/`EnsureMonitorState`, synchronous debounce + transition + notification + host-down child fan-out, `workerWG`-based shutdown that closes `results`; `uptime_check.go` (`runCheck` + `runHostCheck`, pure, no DB, no state); de-block `checkHost` (single dial, no sleep-retry). Tests: enqueue/drop, per-check deadline, SSRF parity, `SeedState` from DB, `monMu` serializes concurrent RMW, host-check job path. **Also constructed (not `Run`) in `routes.go` this commit (S1).**
- **Scheduler + remote-server sync hook + restore rehydrate:** `UptimeScheduler` (two schedule maps, host + monitor hydration, jittered backfill, host-down short-circuit consult, batched write-back, `rescan()`, `Rehydrate()`); start **all** `Run` loops in `routes.go` (`ingester.Run`, `pool.Run`, `scheduler.Run`, `syncLoop.Run`) replacing the ticker go-func; delete the old block; collapse `checkMonitor`/`checkHost` onto `runCheck`/`runHostCheck` + the pool; drop `CheckAll()` from `runInitialUptimeBootstrap`; `CheckAll()` returns `(enqueued, dropped int)`; `SyncAndCheckForRemoteServer` / `SyncMonitorForRemoteServer` + `RemoteServerHandler` `UptimeService` dependency + create/update/delete hooks; replace hardcoded `Interval: 60` in all auto-create paths with `clampInterval(0, cfg)` (S3); `RestoreBackupSafe` reconcile calls `scheduler.Rehydrate()` (S6). **Verify `server.Run` shutdown grace ≥ `hardCap` + ~2 s (S4).** Tests: monitor + **host** due-selection; interval clamp (30 floor, 0→default, auto-create honours `default_interval_seconds` — S3); backfill spread; **teardown chain — in-flight check's heartbeat still written on immediate `ctx` cancel (S4)**; **B3 — saturated ingester, every `CheckResult` dropped, `down` transition still detected + notified**; **B2 — host→down fan-out + scheduler skip + recovery**; write-back grouping; remote-server hook create/update/delete; `Rehydrate()` re-syncs after a simulated live restore (S6).
- **Pruner + deferred index:** `UptimePruner` + chunked delete (subquery form) + `pruneChunkPause` + WAL checkpoint threshold + `PRAGMA optimize` cadence + `CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created` retried at the end of every clean, caught-up pass until it lands (no `sync.Once`); `charon migrate` CLI gains the eager unconditional `CREATE INDEX IF NOT EXISTS`. Tests: deletes only rows older than cutoff; chunk loop terminates; respects hot config change; `ctx` abort mid-loop does not attempt the index; a clean caught-up pass creates it; a second pass with the index present is a no-op (no error); a pass that errors then a later clean pass still creates it.
- **Summary endpoint:** `UptimeSummaryService` + 30 s TTL cache + 3-query strategy + `GET /uptime/monitors/summary` + `GET /uptime/health`; cap + `before` cursor on `GetHistory`. Tests: one windowed query returns ≤ `beats` per monitor chronological ASC; cache hit avoids re-query; `uptime_24h` math; `beats` clamp; history `limit` cap 500; `before` paging.

### Phase 3 — Frontend (`frontend-dev`)

- `api/uptime.ts`: `MonitorSummary`, `BeatDTO`, `getMonitorsSummary(beats = 30)`, `before` param on `getMonitorHistory`.
- `hooks/useUptimeSummary.ts` (`getMonitorsSummary(30)`).
- `pages/Uptime.tsx`: single summary query; `MonitorCard` reads props (remove per-card `useQuery`); invalidations retargeted to `['uptimeSummary']`; heartbeat bar `BEAT_BAR_SLOTS = 30`; interval field `min=30` + clamp + helper; manual check/sync queue-full → toast (N5).
- `pages/SystemSettings.tsx`: new admin "Uptime Monitoring" card — three `uptime.*` number fields with client-side bounds validation, restart-required note on `worker_pool_size`, save via `updateSetting(..., 'uptime', 'int')`, gated on `feature.uptime.enabled`.
- Vitest: `Uptime.test.tsx` / `Uptime.spec.tsx` / `Uptime.tcp-ux.test.tsx` updated — assert no per-card history fetch, cards render from summary fixture; `api/__tests__/uptime.test.ts` — new client fn; form validation test for the 30 s floor; **`SystemSettings` test — Uptime card render / bounds rejection / save / feature-flag gating**.
- `npm run type-check`, `npm run test`, `npm run build`.

### Phase 4 — Integration & E2E

- Flip Phase 1 `test.fixme` → `test`; adjust mock payload shapes to the final schema.
- Run the touched specs: `npx playwright test tests/monitoring/uptime-monitoring-scale.spec.ts tests/monitoring/uptime-monitoring.spec.ts --project=firefox` from repo root.
- `tests/a11y/uptime.a11y.spec.ts` — re-run under firefox; fix any regressions from the card markup change.
- Manual smoke against a local backend with ~100 seeded monitors (optional but recommended): confirm `/uptime/monitors/summary` p95 and one-request behavior in the network panel.

### Phase 5 — Documentation & deployment

- `docs/features/uptime-monitoring.md` — rewrite "How It Works / Check Cycle" for per-monitor intervals; new "Scaling & performance" section (worker pool, ingester, retention, the three `uptime.*` settings + bounds + hot-reload table, admin settings card); note the accepted double-DNS-lookup.
- `ARCHITECTURE.md` — see §5.
- No infra/CI changes. No new env vars.
- **Deploy note (upgrade on a large existing DB — be honest, per S2/S7):**
  - First boot: the pruner trims `uptime_heartbeats` to the retention window (chunked, wider first-pass pause), then — at the end of that and every later clean pass, retried until it lands — builds `idx_heartbeat_monitor_created`. Both run in a **background goroutine**; the server is up and the summary endpoint serves correct (if slower) results throughout — **no route downtime**.
  - **On a 500-monitor instance the retained table is ~65 M rows even after trimming**, so the first index build is a **bounded multi-minute operation** that adds write-lock contention on the single connection for its duration (reads via WAL are unaffected; some heartbeat writes may drop-on-full and self-heal). This is expected, not a bug.
  - Operators who want to avoid that window entirely: **lower `uptime.heartbeat_retention_days` before first boot**, or run `charon migrate` in a maintenance window — the CLI builds the index eagerly (not prune-first) and now logs a `WARN` that the build "can take several minutes and holds a write lock".

---

## 5. ARCHITECTURE.md updates required

Add a **"Uptime Subsystem"** subsection (sibling of "Stats Subsystem", ~line 356), documenting:

- **`UptimeScheduler`** (`internal/services/uptime_scheduler.go`): single goroutine, ~5 s tick. Maintains **two** in-memory schedule maps — monitors (keyed on `uptime_monitors.next_check_at`, persisted via batched write-back) and hosts (due = min child interval, in-memory only). Per tick: a host-connectivity pass then a monitor pass; the monitor pass consults the pool's `hostState` map to **skip** TCP monitors of a known-down host. Advances due-times by the per-monitor `Interval` (30 s floor; `uptime.default_interval_seconds` for zero/legacy/auto-created). Jittered cold-start backfill (60 s) prevents a restart stampede. `Rehydrate()` re-syncs after a live DB restore. Replaces the former global 1-minute `CheckAll()` + `checkAllHosts()` ticker.
- **`UptimeWorkerPool`** (`internal/services/uptime_worker_pool.go`): fixed-size (`uptime.worker_pool_size`, default 30) pool over a bounded (512) `Kind`-discriminated job channel (monitor check / host check); drop-on-full → `checks_enqueue_dropped`. Owns the **authoritative in-memory debounce state**: `monState` (per-monitor `{status, failureCount, lastStatusChange, lastNotifiedDown}`) and `hostState` (per-host), both seeded from the DB at start. The **worker** — not the ingester — read-modify-writes this state synchronously, computes transitions, dispatches notifications, and (on a host→down) fans out synthetic `down` child results for the host's TCP monitors. One shared SSRF-safe keep-alive `*http.Client` (`network.NewSafeHTTPClient(..., network.WithKeepAlive(100, 4, 30s))`) — same `safeDialer` / redirect / localhost+RFC1918 policy as the retired per-check client. Host TCP pre-check is a single non-blocking dial (was `2s × MaxRetries` sleep-retry). Shutdown: `workerWG.Wait()` on in-flight checks, then closes the `results` channel.
- **`UptimeIngester`** (`internal/services/uptime_ingester.go`): mirrors `StatsIngester`'s batching but is a **pure persistence mirror** — it does **no** transition detection and **no** fan-out; it copies pre-computed columns. Receives `CheckResult | HostCheckResult` on a channel the **pool owns and closes**; `Run` ends only when that channel is closed (guaranteeing no in-flight result is lost at shutdown), then does a final flush. Batch-inserts `uptime_heartbeats` and coalesces `uptime_monitors` / `uptime_hosts` column updates every 500 ms or 100 results in one transaction. Drop-on-full → `heartbeats_dropped` at `GET /api/v1/uptime/health`; a dropped write **cannot** suppress an alert because detection never reads these columns at runtime.
- **`UptimePruner`** (`internal/services/uptime_pruner.go`): hourly, chunked `DELETE` (5 000 rows/chunk via `WHERE id IN (SELECT ... LIMIT n)`; 50 ms inter-chunk pause steady-state, 250 ms on the first cold pass) of `uptime_heartbeats` older than `uptime.heartbeat_retention_days` (default 90). `PRAGMA wal_checkpoint(TRUNCATE)` after a large prune; `PRAGMA optimize` daily. No downsampling; `VACUUM` deliberately not used. Also **owns lazy creation of `idx_heartbeat_monitor_created`** — not a struct-tag/AutoMigrate index; `CREATE INDEX IF NOT EXISTS` is issued at the end of every clean, caught-up pass (retried until it lands). Prune-first bounds the pathological (hundreds-of-millions-row) case; on a healthy 500-monitor instance the first build still runs over ~65 M rows and is a bounded multi-minute, write-contending operation (background; no route downtime). `charon migrate` builds it eagerly, with a `WARN` log.
- **Targeted monitor sync on host mutation** — proxy-host create/update/delete already drive `UptimeService.SyncAndCheckForHost` / `SyncMonitorForHost` / inline monitor cleanup; **remote-server create/update/delete now do the same** via `SyncAndCheckForRemoteServer` / `SyncMonitorForRemoteServer` / inline cleanup (`RemoteServerHandler` gains a nil-guarded `UptimeService` dependency). The 5-minute `UptimeSyncLoop` is the backstop. Auto-created monitors inherit `uptime.default_interval_seconds` (not a hardcoded 60).
- **`uptimeConfig`** (`internal/services/uptime_config.go`): a hot-reloading (60 s TTL) snapshot of the three `uptime.*` Settings, shared by the scheduler, pruner, and `UptimeService`. Read-only; writes go through `POST /api/v1/settings`.
- **`UptimeSummaryService`** (`internal/services/uptime_summary_service.go`): serves `GET /api/v1/uptime/monitors/summary` from **three** queries (monitor metadata; one `ROW_NUMBER()` windowed recent-beats query, default 30 beats / cap 60; one grouped 24 h-uptime query) with a 30 s TTL cache — same pattern as `StatsService`. Correct (slower) even before `idx_heartbeat_monitor_created` exists; never 503-gated. Replaces the per-card N+1 history fetch.
- **New settings** (`models.Setting`, `Category="uptime"`): `uptime.default_interval_seconds` (60; 30–86400; hot-reload), `uptime.worker_pool_size` (30; 1–200; **restart** to apply), `uptime.heartbeat_retention_days` (90; 1–3650; hot-reload). Editable via `POST /api/v1/settings` and the SystemSettings "Uptime Monitoring" card.

Update the **API Endpoints** area (or add an "Uptime" table) with:

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/uptime/monitors/summary` | Per-monitor status + latency + last check + `recent_beats` sparkline series + 24 h uptime, one response, 30 s cached |
| `GET` | `/api/v1/uptime/monitors/:id/history` | Detailed heartbeat history for one monitor — `limit` (≤ 500), `before` cursor |
| `GET` | `/api/v1/uptime/health` | Ingester `heartbeats_dropped`, pool `checks_enqueue_dropped`, `queue_depth`, `worker_pool_size` |

Update **§4 Database (SQLite + GORM)** "Pragma Settings" / concurrency note to mention that the uptime write path, like stats, is funnelled through a buffered ingester rather than writing on the request/check goroutine — reinforcing why `SetMaxOpenConns(1)` remains viable at 500 monitors. Add that authoritative uptime **debounce state lives in memory** (pool `monState`/`hostState` maps); the DB columns are a persistence mirror.

Update the **Security section's SSRF-client note** (the "Keep-alives disabled" default in `network.NewSafeHTTPClient`): add a line that the uptime worker pool constructs a **pooled variant** via `network.WithKeepAlive(100, 4, 30s)` — `safeDialer` still re-validates every new connection; `idleTimeout` is 30 s to bound Layer-2 staleness on reused connections.

Add `uptime_heartbeats` retention to the **"Migrations" / data-lifecycle** notes (hard-delete, configurable window `uptime.heartbeat_retention_days` default 90, background pruner). Note that `idx_heartbeat_monitor_created` is created **lazily by the pruner** (`CREATE INDEX IF NOT EXISTS` at the end of every clean, caught-up pass, retried until it lands), not by AutoMigrate — so upgrades never see a *migration-time* stall, though on a large instance the first background build is still a bounded multi-minute write-contending operation (`charon migrate` builds it eagerly, with a warning log, for out-of-band migration).

---

## 6. Risks & mitigations

| # | Risk | Likelihood / impact | Mitigation |
|---|---|---|---|
| R1 | **Single-connection contention persists even with batching.** The ingester still writes through the one SQLite connection; a slow flush blocks API reads. | Med / Med | Ingester writes are now ~2–4 statements / 500 ms (vs ~17 writes/s today) — a large net reduction. Flush is one transaction. Pruner yields between chunks. If contention still shows in QA, raise `uptimeFlushInterval` and/or lower `uptimeBatchSize` is counter-productive — instead increase batch size so flushes are rarer. Monitored via `/uptime/health` + existing `/api/v1/health/db`. |
| R2 | **Down alert suppressed under ingester saturation** — a failure-debounce counter that depended on the droppable async DB write would never reach `maxRetries` while results drop, so the transition is never detected. This is the exact overload the feature targets. | — / High if mishandled | **Designed out (B3):** the debounce counter is authoritative in the pool's in-memory `monState` map (seeded from DB once at start), read-modify-written **synchronously by the worker** under `monMu` on every result. The ingester's `failure_count`/`status` write is a persistence *mirror* only. A dropped `CheckResult` cannot delay or suppress a transition. Transition detection + `queueDownNotification`/`sendRecoveryNotification` also run synchronously on the worker before enqueue. Covered by the C5 "drop-does-not-suppress-alert" test (§3.3.3) and the notification-without-ingester test. Residual: a hard crash mid-saturation can reseed `failureCount` stale-low → alert fires ≤ 1–2 cycles later (bounded, §3.3.3). |
| R3 | **First retention prune on a huge `uptime_heartbeats` table.** At the 500-monitor target the steady-state table is ~65 M rows (90 d); a years-stale instance can be several hundred million. | High / Med | Chunked delete (5 000/chunk); **first cold pass** uses a 250 ms inter-chunk pause (5× steady state) so the single connection stays available; each cold-table chunk can be 100–500 ms (§3.4.2 / N1). Runs in a background goroutine ~30 s after boot; server serves throughout. WAL checkpoint reclaims file growth. No migration-time bulk operation. The first pass trims *before* the index build (§3.5.6, R4). |
| R4 | **`idx_heartbeat_monitor_created` first-boot build.** Even after a clean prune the build runs over ~65 M rows on a 500-monitor instance — a **bounded multi-minute operation that contends for the single write connection** (readers unaffected — WAL). | Med / Med (honest, per S2 — *not* "designed away") | The index is **not** built by AutoMigrate. `UptimePruner` issues `CREATE INDEX IF NOT EXISTS` at the end of every clean, caught-up pass — **retried hourly, idempotent**, so a failed/interrupted attempt self-heals without a restart. Prune-first only removes the *pathological* (hundreds-of-millions) case, not the multi-minute build itself. Acceptable because: background goroutine (no request-path / migration stall); summary endpoint stays available (correct, 30 s-cached, never 503); heartbeat drops during the build self-heal. Operator escape hatches: `charon migrate` in a maintenance window (eager, with a warning log — S7), or lower `uptime.heartbeat_retention_days` before first boot. The 90-day default is unchanged (user decision). |
| R4a | **Host-check scheduling is a new subsystem (B1).** `UptimeScheduler` now also hydrates + schedules `UptimeHost` rows and the `hostState` map is shared between the host-check worker (writer) and the scheduler (reader). | Low / Med | Hosts are few (one per distinct upstream) so the host schedule + cold-start wave are tiny. `hostState` is a plain `sync.RWMutex`-guarded map; the scheduler only ever RLocks it for the short-circuit check. Host due-times are in-memory only (no schema change). Covered by scheduler host-due-selection tests + a host→down fan-out test (C5). |
| R5 | **Scheduler cold-start stampede** — a restart makes all 500 monitors due at once → 500 enqueues in one tick → queue saturation + latency spike. | Med / Med | Jittered backfill: past-due monitors get `next_check_at = now + rand(0, min(interval, 60s))`, spreading enqueues to ~`monitors/60` per second. Per-tick enqueue cap (200). Queue cap 512 absorbs the rest; overflow retried next tick. |
| R6 | **Worker-pool starvation when many hosts are down** — failing checks hold workers for the full connect timeout, throughput collapses, *all* latencies inflate (the exact symptom we're fixing). | Med / High | Short connect timeout (3 s) + per-check hard deadline (≤ 20 s, usually = interval). Host-down short-circuit skips TCP monitors of known-down hosts entirely. Drop-on-full degrades gracefully (delayed checks + metric) rather than unbounded goroutine growth. Documented sizing formula; 500-monitor/mostly-down deployments raise `uptime.worker_pool_size` to 60–90. `/uptime/health` `queue_depth` + `checks_enqueue_dropped` make starvation observable. |
| R7 | **`ROW_NUMBER()` window query cost** at 500 monitors × 24 h of beats — worst during the first-boot window before `idx_heartbeat_monitor_created` exists (which, per R4, can now be several minutes on a large instance). | Low / Med | Bounded by `created_at >= now-24h`; covered by the index once built; 30 s TTL cache ⇒ ≤ 2 executions/min regardless of viewers. **The route stays available with correct results the whole time (never 503-gated)** — deliberate, bounded. Steady-state p95 target < 300 ms, gated by the C7 `TestUptimeSummary_PerfBudget` timed test (< 2 s CI-stable ceiling, index present; §3.5.3 / S5). Fallback (per-monitor `LIMIT` loop) documented, not implemented. |
| R8 | **Summary payload size.** | Low / Low | **`beats` default is 30** (per user decision), cap 60. 500 monitors × 30 beats ≈ ~700–900 KB uncompressed → **~50–75 KB gzipped** (gzip already on for the API). 30 s TTL cache. The list view uses the default 30; only an expanded/detail view requests 60. |
| R9 | **Lost `next_check_at` write-back on crash** → duplicate check for some monitors after restart. | Low / Low | In-memory `monSchedule` is the runtime source of truth; write-back is best-effort batched every tick. Worst case: one extra check per affected monitor, absorbed by cold-start jitter. |
| R10 | **Behavior change: detection timing shifts** — per-monitor interval instead of fixed 60 s; auto-created monitors now inherit `uptime.default_interval_seconds` instead of a hardcoded 60 (S3); host "down" now after 2× the host cadence. | High / Low | Intended (goal #2). Documented in `docs/features/uptime-monitoring.md`. `uptime.default_interval_seconds` defaults to 60, so an untouched deployment sees no cadence change; legacy monitor rows keep their stored interval until edited. |
| R11 | **Keep-alive pooled connection skips Layer-2 re-resolution for its lifetime.** | Low / Low | No actual SSRF — an *established* TCP connection can't be re-bound to a new IP; `safeDialer` still validates every **new** connection. `idleTimeout` cut to **30 s** (from 90 s) to bound the staleness window; `safeclient_test.go` asserts a connection older than `idleTimeout` is not reused, and that link-local/metadata stay blocked with keep-alive on. |
| R12 | **Two orchestration passes on the same files** (per CLAUDE.md incident note). | — / Med | Single PR, sequential commits; `qa-security` runs last, after all commits land, never in parallel with implementation on `uptime_*` files. |
| R13 | **Result loss at shutdown** — an in-flight worker emitting after the ingester returned would lose that result's persistence (and, post-B3, the DB would never learn a transition the `monState` already applied → next restart reseeds stale). | — / Med | Explicit teardown chain (§3.1.4): scheduler stops enqueuing → pool `workerWG.Wait()`s in-flight checks then **closes `results`** → ingester `range`s `results` until closed, then final flush. The ingester does **not** exit on `ctx.Done()` alone. C5 test asserts an in-flight check's heartbeat is still written when `ctx` is cancelled immediately. Grace-period requirement (≥ `hardCap` + ~2 s) flagged as a C5 implementation check against `server.Run`'s shutdown timeout. |
| R14 | **Post-restore stale in-memory state** — after a live `RehydrateLiveDatabase` the scheduler/pool maps reflect pre-restore data. | Low / Low | Restore-then-restart (pending-restore path + the recommended `RehydrateLiveDatabase` flow) = ordinary cold start, no special handling (§3.9 — goroutines start after DB init). Live-restore-without-restart self-heals within one `rescan()` (≤ 30 s) + 1–2 check cycles; `UptimeScheduler.Rehydrate()` called from the restore reconcile step makes it immediate. No data corruption in any case. |

---

## 7. Acceptance Criteria

Feature is done when **all** of the following hold on the PR:

### Functional

1. Creating a monitor with `interval: 45` results in that monitor being checked ~every 45 s (observable via heartbeat `created_at` spacing); `interval: 10` is rejected `400` by the API and blocked client-side with helper text.
2. A monitor with `interval: 0` or a legacy `interval < 30` is checked at `uptime.default_interval_seconds` / 30 s respectively (clamp works).
2a. **Auto-created monitors honour the admin default (S3).** With `uptime.default_interval_seconds = 45`, a monitor created by `SyncAndCheckForHost` / `SyncAndCheckForRemoteServer` / `SyncMonitors` has `Interval == 45` (not the old hardcoded 60) — verified by a Go test.
3. Changing `uptime.default_interval_seconds` via `POST /api/v1/settings` (or the admin Uptime settings card) takes effect within ≤ 60 s without a restart; an out-of-bounds value is rejected `400`.
3a. The **admin "Uptime Monitoring" settings card** (`SystemSettings.tsx`) renders the three `uptime.*` fields seeded from `GET /api/v1/settings`, enforces the §3.6.1 bounds client-side (out-of-bounds ⇒ inline error + disabled save), persists in-bounds values via `POST /api/v1/settings` with `category=uptime`, labels `worker_pool_size` as restart-required, and is hidden when `feature.uptime.enabled` is off.
4. `GET /api/v1/uptime/monitors/summary` returns one array with `status`, `latency`, `last_check`, `interval`, `uptime_24h` (always present), and up to `beats` chronological `recent_beats` per monitor (**default 30**, cap 60); the Uptime page issues **exactly one** request to it and **zero** to `/uptime/monitors/*/history` on initial load and on refetch.
4a. **(S5) Automated perf gate:** `TestUptimeSummary_PerfBudget` (`-short`-skippable) seeds 500 monitors × 24 h of heartbeats, builds `idx_heartbeat_monitor_created`, and asserts `GetSummary(ctx, 30)` (cache cleared) completes **< 2 s** wall-clock. (The < 300 ms p95 target is tracked from the QA timing output, not hard-gated.)
5. `GET /api/v1/uptime/monitors/:id/history?limit=99999` returns at most 500 rows; `before=<ts>` returns only older rows.
6. `GET /api/v1/uptime/health` returns `heartbeats_dropped`, `checks_enqueue_dropped`, `queue_depth`, `worker_pool_size`.
7. Heartbeats older than `uptime.heartbeat_retention_days` are deleted within one hour of the pruner running; rows within the window are untouched; the delete is chunked (test — loop terminates, chunk size respected, first cold pass uses the wider pause). `idx_heartbeat_monitor_created` is created at the end of a clean, caught-up pruner pass (not attempted on a `ctx`-aborted pass); a pass that errored then a later clean pass still creates it; a pass with the index already present is a no-op.
8. **(B3)** The failure-debounce counter and up→down / down→up transition detection are computed against the pool's in-memory `monState` map, **not** a persisted row. Verified: (a) a monitor's `down` transition fires the notification **without** the ingester running / flushing; (b) with the ingester saturated and **every** `CheckResult` dropped, feeding `maxRetries` consecutive `down` results still detects the transition and calls `queueDownNotification`.
8a. **(B1/B2)** Host connectivity is scheduled by `UptimeScheduler`'s host pass (host due-selection test); on a host `up→down` transition the **worker** (not the ingester) writes `hostState`, fans out synthetic `down` child heartbeats for the host's TCP monitors, and fires one consolidated notification; while the host is down the scheduler skips enqueueing those TCP monitors; on recovery they resume. Verified by a host-down/recovery integration test.
9. **(S4)** Graceful shutdown teardown chain: `ctx` cancel → scheduler stops enqueuing → pool `workerWG.Wait()`s in-flight checks and closes `results` → ingester drains the closed channel and does a final flush. Test: a check that is in-flight when `ctx` is cancelled still has its heartbeat row written (no result loss); all goroutines exit (`goleak` or a `ctx`-cancel wait).
9a. **(S6)** After a live DB restore, calling `UptimeScheduler.Rehydrate()` re-syncs the schedule + state maps to the restored data (test); without it, state self-heals within one `rescan()` + 1–2 check cycles (documented, §3.9).
10. The shared HTTP client preserves SSRF protections: checks to `127.0.0.1` and `10.x` succeed (as today), checks to `169.254.169.254` / link-local fail, redirects are not followed — with keep-alive enabled; a pooled connection older than `idleTimeout` (30 s) is not reused.

### Non-functional / DoD (per CLAUDE.md "Task Completion Protocol" — referenced, not reproduced)

10a. Remote-server create/update/delete drive the targeted uptime-monitor sync (`SyncAndCheckForRemoteServer` / `SyncMonitorForRemoteServer` / inline delete cleanup) — verified by `remote_server_handler_test.go` and `uptime_service_*_test.go`.
11. **Targeted Playwright** (`tests/monitoring/uptime-monitoring-scale.spec.ts` + `tests/monitoring/uptime-monitoring.spec.ts` + `tests/a11y/uptime.a11y.spec.ts`, `--project=firefox`) pass locally; full/cross-browser deferred to CI.
12. **GORM security scan** (`./scripts/scan-gorm-security.sh --check`) — zero CRITICAL/HIGH (triggered: `models/uptime.go` + new raw-ish queries).
13. **Backend coverage ≥ 85%** (`scripts/go-test-coverage.sh`) — new services each have their own `_test.go`; **Frontend coverage ≥ 85%** (`scripts/frontend-test-coverage.sh`).
14. **Local patch coverage preflight** (`bash scripts/local-patch-report.sh`) — artifacts generated, patch coverage green.
15. **CodeQL Go + JS + Trivy** — zero high/critical (this adds new code paths/endpoints ⇒ run locally per DoD).
16. `lefthook run pre-commit` clean; `make lint-fast` / staticcheck clean (no `--no-verify`).
17. `cd backend && go build ./...` and `cd frontend && npm run build` succeed; `cd frontend && npm run type-check` clean.
18. All existing/adjacent tests updated and green: `uptime_service_*_test.go`, `uptime_handler_test.go`, `remote_server_handler_test.go`, `routes_uptime_bootstrap_test.go`, `Uptime.test.tsx`, `Uptime.spec.tsx`, `Uptime.tcp-ux.test.tsx`, `SystemSettings` test, `api/__tests__/uptime.test.ts`.
19. `ARCHITECTURE.md` (§5) and `docs/features/uptime-monitoring.md` updated.
20. `supervisor` review passed against the plan; `qa-security` audit (`docs/reports/qa_report.md`) has no blocking findings.

---

## Commit Slicing Strategy

**Decision:** One feature, **one PR** on `feat/uptime-monitoring-scale`, merged only when the whole Definition of Done passes. The work is decomposed into **9 ordered commits**. Each commit builds and passes its own gate; the PR as a whole passes the full DoD (§7.11–7.20 — CLAUDE.md "Task Completion Protocol", not reproduced here). No feature-splitting across PRs.

Dependency order: `C1` (specs, independent) → `C2` (foundation, no behavior change) → `C3` ingester **(constructed in `routes.go`, `Run` not started)** → `C4` pool+client **(constructed in `routes.go`, `Run` not started; `monState`/`hostState` types)** → `C5` scheduler + start all `Run` loops + remote-server hook + restore-rehydrate (needs C3+C4) → `C6` pruner + deferred `idx_heartbeat_monitor_created` (needs C2) → `C7` summary endpoint + `/uptime/health` (needs C2, C4 for the pool/ingester refs, C6 for the index) → `C8` frontend (needs C7) → `C9` hardening (needs all).

**Why C3/C4 construct-but-don't-`Run` (S1):** C7's `/uptime/health` handler needs the pool + ingester *references*. If those were constructed in C5, `git revert C5` after C7 landed would leave C7's handler holding nil deps and the tree would not build. Constructing them in C3/C4 (inert until C5 starts their `Run` loops) makes **`git revert C5` a genuine "restore the old ticker with C3/C4/C6/C7 dormant"** — the pool/ingester sit idle, `QueueDepth()`/`DroppedCount()` return zero, `/uptime/health` still serves valid JSON, and the legacy `checkMonitor` path is what runs.

---

### Commit 1 — E2E specs for new behavior (`test.fixme`)

- **Scope:** Failing-by-design E2E coverage of the three headline behaviors, mock-response style.
- **Files:**
  - `tests/monitoring/uptime-monitoring-scale.spec.ts` (new) — all `test.fixme`.
  - `tests/fixtures/uptime.ts` (new) — `makeSummaryFixture(n)`, `makeBeatSeries(n)` helpers.
- **Depends on:** nothing.
- **Validation gate:**
  - `npx playwright test tests/monitoring/uptime-monitoring-scale.spec.ts --project=firefox` → all `fixme` (skipped), 0 failures.
  - `cd frontend && npm run type-check` (spec + fixtures type-check).
- **Notes:** Establishes the acceptance shape (§4 Phase 1). No app code touched.

### Commit 2 — Foundation: `NextCheckAt`, indexes, config keys, interval-floor validation

- **Scope:** Schema + config + validation only. **No change to checking behavior** (old ticker still runs).
- **Files:**
  - `backend/internal/models/uptime.go` — add `NextCheckAt time.Time` to `UptimeMonitor` (+ `gorm:"index"`). **Do NOT touch `UptimeHeartbeat` tags** — the `idx_heartbeat_monitor_created` composite is created lazily by the pruner in C6 (§3.5.6), not via a struct tag.
  - `backend/internal/services/uptime_config.go` (new) — `uptimeConfig` snapshot + `clampInterval`.
  - `backend/internal/api/routes/routes.go` — `FirstOrCreate` seeds for `uptime.default_interval_seconds` (60), `uptime.worker_pool_size` (30), `uptime.heartbeat_retention_days` (90), `Category:"uptime"`. (`&models.UptimeMonitor{}` already in `AutoMigrate` — the `next_check_at` column + its index are added automatically; ≤ 500-row table, sub-millisecond.)
  - `backend/internal/api/handlers/settings_handler.go` — `uptime.*` validation branch in `UpdateSetting` (bounds per §3.6.1).
  - `backend/internal/api/handlers/uptime_handler.go` — `Create` rejects `0 < interval < 30` → `400`.
  - `backend/internal/services/uptime_service.go` — `CreateMonitor` resolves `interval<=0 → cfg.DefaultIntervalSeconds()`, floors `<30 → 30` (or errors — see §3.6.3); `UpdateMonitor` `interval` branch adds floor check returning `ErrIntervalTooLow`; construct/inject `uptimeConfig`.
  - Tests: `uptime_config_test.go` (new), `settings_handler_test.go` (+cases), `uptime_handler_test.go` (+cases), `uptime_service_test.go` (+`CreateMonitor`/`UpdateMonitor` floor cases).
- **Depends on:** C1 (order only).
- **Validation gate:**
  - `cd backend && go test ./internal/models/... ./internal/services/... ./internal/api/handlers/... ./internal/api/routes/...`
  - `cd backend && go build ./...`; `make lint-fast`.
  - `./scripts/scan-gorm-security.sh --check` (touches `models/uptime.go`).
- **Rollback:** Pure additive; revert commit. `NextCheckAt` column left in place is harmless (unused).

### Commit 3 — `UptimeIngester` (dumb persistence mirror) + `CheckResult` / `HostCheckResult`

- **Scope:** New ingester component + result types. **Constructed in `routes.go`, `Run` not started** (S1) — nothing sends to it yet, so it is inert.
- **Files:**
  - `backend/internal/services/uptime_ingester.go` (new) — `CheckResult`, `HostCheckResult`, `UptimeIngester` (`results <-chan any`, `noteDropped`, `DroppedCount`, `Run(ctx)` that returns on `results` **closed** — not `ctx.Done()` — with a final flush, `Stop` test helper). Pure column-copy flush: heartbeat batch insert + coalesced `uptime_monitors` / `uptime_hosts` updates in one transaction; **no** transition logic, **no** fan-out.
  - `backend/internal/api/routes/routes.go` — create the `results` channel; `ingester := services.NewUptimeIngester(db, results)`; store the ref. **Do not** `go ingester.Run(ctx)` yet.
  - `backend/internal/services/uptime_ingester_test.go` (new) — drop-on-full (`noteDropped` increments), batch-by-count, batch-by-timer, type-switch routing of `CheckResult` vs `HostCheckResult`, `Run` terminates only when `results` is closed + does a final flush, `ctx.Done()` alone does **not** terminate `Run`, transaction lock-error bounded-retry.
- **Depends on:** C2.
- **Validation gate:**
  - `cd backend && go test ./internal/services/... -run Uptime`; `cd backend && go test ./internal/api/routes/...`
  - `go build ./...`; `make lint-fast`; `./scripts/scan-gorm-security.sh --check`.
- **Rollback:** Constructed-but-idle. Revert is isolated (also removes the `routes.go` construction line).

### Commit 4 — Bounded worker pool (state maps, host jobs) + keep-alive SSRF client; de-block host pre-check

- **Scope:** `UptimeWorkerPool` with the authoritative `monState`/`hostState` maps, `Kind`-discriminated jobs, `network.WithKeepAlive`, pure `runCheck`/`runHostCheck` extraction, single-dial host pre-check. **Constructed in `routes.go`, `Run` not started** (S1). Legacy `checkMonitor`/`checkHost` still active this commit — `runCheck`/`runHostCheck` are parallel pure functions used only by the pool until C5 collapses the old paths (transient duplication, called out per N3).
- **Files:**
  - `backend/internal/network/safeclient.go` — add `WithKeepAlive(maxIdle, perHost int, idleTimeout time.Duration)` option + conditional `Transport` fields (default byte-for-byte unchanged).
  - `backend/internal/network/safeclient_test.go` — keep-alive on: connection reuse within `idleTimeout` (httptest, assert connection count); **connection older than `idleTimeout` (30 s) is NOT reused**; link-local / cloud-metadata still blocked; redirects still not followed.
  - `backend/internal/services/uptime_check.go` (new) — `runCheck(ctx, job, client) rawResult` + `runHostCheck(ctx, job, dialer) rawResult`: pure probes (HTTP/TCP/orthrus; single host dial), **no DB, no state-map access, no notifications**.
  - `backend/internal/services/uptime_worker_pool.go` (new) — `UptimeJobKind`/`UptimeJob`, `monStateEntry`/`hostStateEntry`, `UptimeWorkerPool` (`SeedState`, `ReseedState`, `EnsureMonitorState`, `Run`, `TryEnqueue`, `Enqueue`, `QueueDepth`, `EnqueueDropped`, `HostState`), shared keep-alive client + `hostDialer` construction, `handle()` dispatch: debounce RMW under `monMu`/`hostMu`, synchronous transition + notification, host→down synthetic child fan-out, `workerWG` shutdown that closes `results`.
  - `backend/internal/api/routes/routes.go` — `pool := services.NewUptimeWorkerPool(db, results, ingester, cfg, notifier, poolSize)`; store the ref. **Do not** `go pool.Run(ctx)` yet.
  - `backend/internal/services/uptime_service.go` — `checkHost` inner `for retry` sleep-loop removed → single dial (keeps the cross-cycle `FailureThreshold` debounce).
  - Tests: `uptime_worker_pool_test.go` (new — enqueue/`TryEnqueue`-drop, `Enqueue` 2 s timeout, per-check deadline, `SeedState` populates from DB, `monMu` serializes two concurrent RMWs for the same monitor giving the correct streak, `JobHostCheck` path, `workerWG` drains + closes `results` on `ctx` cancel), `uptime_check_test.go` (new — pure probe outcomes, SSRF parity: `127.0.0.1`/`10.x` allowed, link-local blocked), `uptime_service_test.go` (host pre-check no longer sleeps — assert wall-clock), `uptime_service_race_test.go` (adjusted).
- **Depends on:** C3 (`CheckResult`/`HostCheckResult`, the `results` channel).
- **Validation gate:**
  - `cd backend && go test ./internal/network/... ./internal/services/... ./internal/api/routes/...`
  - `go build ./...`; `make lint-fast`.
  - CodeQL Go local (`lefthook run pre-commit`) — new network option is SSRF-adjacent.
- **Rollback:** `WithKeepAlive` additive; pool constructed-but-idle. Revert isolated (also removes the `routes.go` construction line).

### Commit 5 — Scheduler goes live; teardown chain; remote-server hook; restore rehydrate

- **Scope:** The behavior switch. `UptimeScheduler` (monitor + **host** schedules) + `UptimeSyncLoop` are created and **all `Run` loops are started** (ingester, pool, scheduler, sync loop); the old ticker go-func is deleted; `checkMonitor`/`checkHost` collapse onto `runCheck`/`runHostCheck` + the pool + ingester. Plus: remote-server sync hooks, auto-create default-interval fix (S3), restore rehydrate (S6), and the shutdown grace check (S4).
- **Files:**
  - `backend/internal/services/uptime_scheduler.go` (new) — `monSchedule` + `hostSchedule` maps, `hydrate()` (monitor + host cold-start, jittered backfill), per-tick host pass + monitor pass with the `pool.HostState` short-circuit, batched `next_check_at` write-back, feature-flag gate, `rescan()` (new/disabled monitors + `hostMinInt` recompute + `pool.EnsureMonitorState`), `Rehydrate()` (re-`hydrate()` + `pool.ReseedState()`), `ctx` shutdown = "stop enqueuing".
  - `backend/internal/services/uptime_service.go` — `CheckAll()` re-implemented to enqueue every enabled host + monitor into the pool and return `(enqueued, dropped int)` (N5); `checkMonitor`/`CheckMonitor` and `checkHost` route through the pool (delete direct `s.DB.Create(&heartbeat)` / `s.DB.Save(&monitor)` from the check path); replace **every** hardcoded `Interval: 60` in `SyncMonitors` (~223, ~320), `SyncAndCheckForHost` (~1402) with `clampInterval(0, s.uptimeCfg)` (S3); new `SyncAndCheckForRemoteServer(remoteServerID uint)` / `SyncMonitorForRemoteServer(remoteServerID uint) error` (also `clampInterval(0, …)`), `hostMutexes` key `remote-<id>`, Orthrus-unbound-UUID → silent no-op.
  - `backend/internal/api/handlers/remote_server_handler.go` — `RemoteServerHandler` + `NewRemoteServerHandler` gain a nil-guarded `uptimeService *services.UptimeService`. `Create` → `go SyncAndCheckForRemoteServer`; `Update` → `go SyncMonitorForRemoteServer` (log on error); `Delete` → inline `WHERE remote_server_id = ?` → `DeleteMonitor` before `h.service.Delete(...)` (mirrors `proxy_host_handler.go:755-761`).
  - `backend/internal/services/backup_service.go` — `RestoreBackupSafe` reconcile step calls `scheduler.Rehydrate()` after the DB is restored (S6). Requires a scheduler ref reachable from the restore path (inject or a small setter, mirroring how the Caddy manager ref is threaded).
  - `backend/internal/api/routes/routes.go` — delete the ticker go-func; construct `UptimeScheduler` + `UptimeSyncLoop`; `go X.Run(ctx)` for **ingester, pool, scheduler, sync loop** (pruner started in C6); `runInitialUptimeBootstrap` loses `CheckAll()`; `POST /system/uptime/check` returns `{enqueued,dropped}`, `POST /uptime/monitors/:id/check` uses `pool.Enqueue` (503 on full); pass `uptimeService` into `NewRemoteServerHandler(...)` (~897); wire the scheduler ref to the restore path. **Verify `server.Run` / `http.Server.Shutdown` grace ≥ `hardCap` (20 s) + ~2 s (S4)** — raise it or lower `hardCap` if short; note the finding in the PR.
  - `backend/internal/api/routes/routes_uptime_bootstrap_test.go` — drop `CheckAll` from the `uptimeBootstrapService` interface + tests.
  - `backend/internal/api/handlers/uptime_handler.go` — `CheckMonitor` uses `pool.Enqueue` (503); `Sync`/system-check handlers surface `{enqueued,dropped}`.
  - Tests: `uptime_scheduler_test.go` (new — monitor **and host** due-selection; interval clamp incl. auto-create honours `default_interval_seconds` — S3; backfill spread; write-back grouping; new/disabled reconcile; `Rehydrate()` re-syncs after a simulated live restore — S6; `ctx` cancel stops enqueuing), `uptime_worker_pool_test.go` / a new `uptime_pipeline_test.go` (**S4** in-flight-check heartbeat still written on immediate `ctx` cancel; **B3** saturated ingester + every `CheckResult` dropped → `down` still detected + `queueDownNotification` called; **B2** host→down fan-out + scheduler skip + recovery), `uptime_service_*_test.go` updated for the pool-routed write path + remote-server sync cases, `uptime_handler_test.go`, `remote_server_handler_test.go` (new constructor arg + hook invocation), `backup_service` test (reconcile invokes `Rehydrate`), `routes_test.go` if it asserts the ticker.
- **Depends on:** C3, C4.
- **Validation gate:**
  - `cd backend && go test ./internal/services/... ./internal/api/handlers/... ./internal/api/routes/...`
  - `go build ./...`; `make lint-fast`; `./scripts/scan-gorm-security.sh --check`.
  - CodeQL Go local — new execution path + fan-out.
  - Manual: `go run ./cmd/api` with a few seeded monitors — per-monitor cadence, clean `ctx` shutdown (goroutines exit, no panic), and a killed target flips to `down` and alerts.
- **Rollback:** Highest-risk commit, but isolated: `git revert <C5-sha>` restores the legacy ticker and leaves C3/C4 (pool/ingester constructed but idle), C6, C7 all dormant and building — because their construction lives in C3/C4, not here (S1).

### Commit 6 — Retention pruner + deferred `idx_heartbeat_monitor_created`

- **Scope:** Hourly chunked retention delete, **plus** deferred `idx_heartbeat_monitor_created` creation retried at the end of every clean, caught-up pass until it lands (prune-before-index ordering, §3.5.6 / §3.4.2).
- **Files:**
  - `backend/internal/services/uptime_pruner.go` (new) — hourly loop; `pruneOnce(ctx) (deleted int64, err error)` chunked subquery `DELETE` (`WHERE id IN (SELECT id ... LIMIT 5000)`), `pruneChunkPause`, WAL checkpoint threshold, `PRAGMA optimize` cadence, `ctx` abort. After each pass where `pruneOnce` returned `err == nil` and reached its "caught up" break, `Run` issues `CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created ON uptime_heartbeats (monitor_id, created_at)` (no `sync.Once` — idempotent, re-attempted hourly until it succeeds).
  - `backend/cmd/api/main.go` — in the `case "migrate":` block, after `db.AutoMigrate(...)`: `logger.Log().Warn("building index idx_heartbeat_monitor_created on uptime_heartbeats; on a large database this can take several minutes and holds a write lock for the duration")` then an unconditional `db.Exec("CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created ON uptime_heartbeats (monitor_id, created_at)")` (operator-initiated maintenance window; idempotent, harmless on fresh DBs) — S7.
  - `backend/internal/api/routes/routes.go` — construct `UptimePruner` + `go pruner.Run(ctx)`; `firstPassDone` widens the inter-chunk pause until the first clean pass (N1).
  - `backend/internal/services/uptime_pruner_test.go` (new) — deletes only `< cutoff`; chunk loop terminates at `RowsAffected < chunk`; honors hot config change; `ctx` mid-loop abort **does not** attempt the index; a clean caught-up pass **does** create the index (assert via `PRAGMA index_list(uptime_heartbeats)`); a subsequent pass with the index already present is a no-op (no error); a first pass that returns an error followed by a later clean pass still creates the index.
- **Depends on:** C2 (`uptime.heartbeat_retention_days`, `uptimeConfig`).
- **Validation gate:**
  - `cd backend && go test ./internal/services/... -run Pruner`; `cd backend && go test ./cmd/api/... -run Migrate` (if a migrate-CLI test exists; else `go build ./cmd/api`).
  - `go build ./...`; `make lint-fast`; `./scripts/scan-gorm-security.sh --check` (raw `DELETE` / `CREATE INDEX` `Exec`).
- **Rollback:** Independent goroutine; revert removes the pruner and the deferred index creation with no schema impact (the index, if already built on a running instance, is harmless to leave — or drop it manually).

### Commit 7 — Batch summary endpoint + `/uptime/health` + history pagination

- **Files:**
  - `backend/internal/services/uptime_summary_service.go` (new) — `MonitorSummary`, `BeatDTO`, `UptimeSummaryService`, 30 s TTL cache (ported `summaryCache` shape), 3-query strategy.
  - `backend/internal/api/handlers/uptime_handler.go` — `Summary(c)` (`beats` clamp 1..60), `Health(c)`; `GetHistory` — `limit` cap 500, `before` RFC3339 cursor.
  - `backend/internal/services/uptime_service.go` — `GetMonitorHistory(id, limit, before)` signature + cap.
  - `backend/internal/api/routes/routes.go` — `management.GET("/uptime/monitors/summary", uptimeHandler.Summary)`, `management.GET("/uptime/health", uptimeHandler.Health)`. Wire `UptimeSummaryService` + the pool/ingester refs (constructed in C3/C4) into the handler.
  - Tests: `uptime_summary_service_test.go` (new — windowed query returns ≤ `beats` chronological ASC, cache hit skips query, `uptime_24h` math, empty-history case, **correct with and without `idx_heartbeat_monitor_created` present**, never 503-gated on index absence; **`TestUptimeSummary_PerfBudget`** — `-short`-skippable, 500-monitor + 24 h-heartbeat seed, index built, `GetSummary(ctx, 30)` < 2 s wall-clock — S5), `uptime_handler_test.go` (+`Summary` `beats` default 30 / clamp 60, `Health`, history cap 500, `before` paging), `routes_test.go` (+**N4**: assert both `GET /uptime/monitors/summary` and `GET /uptime/monitors/:id/history` resolve to their handlers — mixed static/param route on the same segment).
- **Depends on:** C2 (config keys), C4 (pool/ingester refs for `Health` — constructed there per S1, so C7 builds even if C5 is reverted), C6 (the `idx_heartbeat_monitor_created` index — summary is correct without it but meets the perf gate only with it).
- **Validation gate:**
  - `cd backend && go test ./internal/services/... ./internal/api/handlers/... ./internal/api/routes/...`
  - `go build ./...`; `make lint-fast`; `./scripts/scan-gorm-security.sh --check`.
  - CodeQL Go local (`lefthook run pre-commit`) — new endpoints.
- **Rollback:** Additive endpoints + one changed service signature; revert also reverts the `GetMonitorHistory` signature (update call sites). Isolated from execution model — and from C5, since the `Health` deps come from C4.

### Commit 8 — Frontend: summary-driven Uptime page + interval floor + admin Uptime settings card

- **Scope:** (1) Uptime page reads the batch summary endpoint (kills N+1); (2) per-monitor interval field with 30 s floor; (3) new admin "Uptime Monitoring" settings card on `SystemSettings.tsx` for the three `uptime.*` global keys (§3.6.4).
- **Files:**
  - `frontend/src/api/uptime.ts` — `BeatDTO`, `MonitorSummary`, `getMonitorsSummary(beats = 30)`, `before` param on `getMonitorHistory`; `syncMonitors` response type gains `enqueued?` / `dropped?`; document that `checkMonitor` may reject with `503` (N5).
  - `frontend/src/hooks/useUptimeSummary.ts` (new) — `getMonitorsSummary(30)`, `refetchInterval: 30000`, key `['uptimeSummary']`.
  - `frontend/src/pages/Uptime.tsx` — single `useUptimeSummary()`; `MonitorCard` reads `monitor.recent_beats` from props, **remove** per-card `useQuery(['uptimeHistory'])`; retarget all `invalidateQueries` to `['uptimeSummary']`; heartbeat bar becomes `BEAT_BAR_SLOTS = 30` wide with updated tooltip copy; interval `<input min="30">` + clamp + helper text in Create + Edit modals; `checkMutation` / `syncMutation` `onError` (or `dropped > 0`) → toast "Check queue full, try again in a moment" instead of a silent success (N5).
  - `frontend/src/pages/SystemSettings.tsx` — new `<Card>` "Uptime Monitoring" (three number inputs: `uptime.default_interval_seconds`, `uptime.worker_pool_size`, `uptime.heartbeat_retention_days`), client-side bounds validation matching §3.6.1, helper text noting `worker_pool_size` needs a restart while the other two hot-reload ~60 s / ~1 h, `useMutation` → `updateSetting(key, String(v), 'uptime', 'int')` per changed field → `invalidateQueries(['settings'])`. Gated on `feature.uptime.enabled` (reuse the existing `featureFlags` query in the file).
  - `frontend/src/components/UptimeWidget.tsx` — optional switch to `getMonitorsSummary`; otherwise unchanged.
  - Tests:
    - `frontend/src/pages/__tests__/Uptime.test.tsx`, `Uptime.spec.tsx`, `Uptime.tcp-ux.test.tsx` — updated to summary fixture; assert exactly one `getMonitorsSummary` call and **zero** history fetches on load/refetch; interval-floor form validation (reject 10, accept 30); manual check → `503` (or `dropped > 0`) surfaces a toast (N5).
    - `frontend/src/api/__tests__/uptime.test.ts` — `getMonitorsSummary` (URL, `beats=30` default, `beats` passthrough); `getMonitorHistory` `before` param.
    - `frontend/src/pages/__tests__/SystemSettings.test.tsx` (or the existing SystemSettings test file) — **new: Uptime settings card** — renders the three fields seeded from `getSettings`, rejects out-of-bounds input (e.g. interval 10, pool size 0, retention 4000) with the save button disabled, saves in-bounds values via `updateSetting` with `category='uptime'`, card hidden when `feature.uptime.enabled` is off.
    - `frontend/src/components/__tests__/ProxyHostForm-uptime.test.tsx` — check interval field if present.
  - i18n: add `uptime.checkIntervalHelper` (min 30 s) and `systemSettings.uptime.*` keys (card title, three labels, three helper texts, validation messages) to the locale files.
- **Depends on:** C7.
- **Coverage implication:** the settings card + its validation add ~1 component's worth of new frontend LOC — its dedicated test above keeps the 85 % frontend patch-coverage gate satisfied; do not merge the card without the card test.
- **Validation gate:**
  - `cd frontend && npm run test -- uptime Uptime SystemSettings` (targeted) then full `npm run test`.
  - `cd frontend && npm run type-check`; `cd frontend && npm run build`.
- **Rollback:** Frontend-only; revert restores per-card history and removes the settings card. Backend summary endpoint + `uptime.*` keys stay (keys remain editable via `POST /api/v1/settings`).

### Commit 9 — Hardening: flip E2E live, docs, ARCHITECTURE

- **Files:**
  - `tests/monitoring/uptime-monitoring-scale.spec.ts` — `test.fixme` → `test`; finalize mock payloads to the shipped schema.
  - `tests/monitoring/uptime-monitoring.spec.ts` / `tests/a11y/uptime.a11y.spec.ts` — adjust for the new card data source if needed.
  - `docs/features/uptime-monitoring.md` — per-monitor intervals, scaling section, `uptime.*` settings table + bounds + hot-reload, accepted double-DNS note.
  - `ARCHITECTURE.md` — "Uptime Subsystem" subsection, endpoint table rows, DB concurrency note (per §5).
  - `docs/features.md` — one-line touch if it summarizes uptime.
- **Depends on:** C1–C8.
- **Validation gate (also the PR-level DoD gate):**
  - `npx playwright test tests/monitoring/uptime-monitoring-scale.spec.ts tests/monitoring/uptime-monitoring.spec.ts tests/a11y/uptime.a11y.spec.ts --project=firefox` — all green.
  - `bash scripts/local-patch-report.sh` — artifacts + patch coverage green.
  - `scripts/go-test-coverage.sh` ≥ 85%; `scripts/frontend-test-coverage.sh` ≥ 85%.
  - CodeQL Go + JS + Trivy — zero high/critical.
  - `lefthook run pre-commit` clean; `cd backend && go build ./...`; `cd frontend && npm run build && npm run type-check`.
  - `./scripts/scan-gorm-security.sh --check` — zero CRITICAL/HIGH.

---

### Rollback & contingency (PR-level)

- **Pre-merge:** the risky behavior switch is isolated to **C5** (starting the `Run` loops + collapsing `checkMonitor`). `git revert <C5-sha>` restores the legacy 60 s ticker; C3/C4 leave the pool/ingester **constructed but idle** (so C7's `/uptime/health` still builds and returns zeros), and C6/C7/C8 stay dormant. This is a *real* isolated revert precisely because pool/ingester construction was pushed down to C3/C4 (S1) — nothing after C5 hard-depends on the `Run` loops being started.
- **Post-merge, field regression:** the fastest kill-switch is `feature.uptime.enabled = false` (Setting) — stops the scheduler, pool, and ingester (pruner keeps running, which is desirable). Then a targeted revert PR of the whole feature if needed.
- **Pruner misbehaving:** set `uptime.heartbeat_retention_days` to a very large value (e.g. 3650) to effectively pause deletion without a deploy.
- **Pool starvation in the field:** raise `uptime.worker_pool_size` and restart; `/uptime/health` confirms the new size.
- **Data safety:** no destructive migration. `NextCheckAt` and `idx_heartbeat_monitor_created` are additive. Heartbeat pruning is the only delete and is bounded by a configurable, admin-visible window with a large default (90 d).

---

## 8. Resolved decisions

All six open questions were answered by the user on 2026-08-27 and are folded into the spec above. Recorded here for traceability:

1. **`worker_pool_size` hot-reload — restart-only.** Pool is sized at construction; `GET /api/v1/uptime/health` surfaces the active value. §3.6.1 keeps "No" for hot-reload. No pool live-resizing.
2. **Admin UI for the 3 `uptime.*` settings — IN THIS PR (Commit 8).** A dedicated "Uptime Monitoring" card on `frontend/src/pages/SystemSettings.tsx` with client-side bounds validation matching §3.6.1, a restart-required note on `worker_pool_size`, wired through `POST /api/v1/settings` (`category=uptime`). See §3.6.4, Commit 8, §7.3a, §7.13.
3. **`uptime_24h` on the summary response — KEPT.** The 3-query strategy stands; the field is always present (nullable when no data). §3.5.2 / §3.5.3.
4. **Remote-server targeted sync — HOOK ADDED (Commit 5).** `SyncAndCheckForRemoteServer` / `SyncMonitorForRemoteServer` + inline delete cleanup on `RemoteServerHandler` create/update/delete, mirroring the proxy-host hooks. The 5-minute `UptimeSyncLoop` remains the backstop. §3.1.3, Commit 5, §7.10a.
5. **`beats` default — 30** (cap unchanged at 60). List view uses 30; an expanded/detail view may request 60. ~50–75 KB gzipped at 500 monitors. Updated in §3.5.1, §3.5.2, §3.5.7, §3.8, §7.4, R8.
6. **Index build ordering — PRUNE FIRST, THEN INDEX; retry-until-success.** `idx_heartbeat_monitor_created` is no longer an AutoMigrate/struct-tag index; `UptimePruner` issues `CREATE INDEX IF NOT EXISTS` at the end of **every** clean, caught-up prune pass (no `sync.Once` — retried hourly until it lands). Prune-first bounds the *pathological* (hundreds-of-millions-row) case; on a healthy 500-monitor instance the first build still runs over ~65 M rows and is a bounded multi-minute, write-contending background operation (no route downtime, never 503; §3.4.2 / R4 state this honestly, per supervisor S2). `charon migrate` builds it eagerly with a `WARN` log. Index-creation work moved from Commit 2 to Commit 6. §3.5.6, §3.4.2, revised R3/R4/R7, Commit 6.

---

### Supervisor review (2026-08-27) — REVISE → resolved

`docs/reports/supervisor_review.md` returned REVISE on the first draft. All Blocking + Should-fix + Nice-to-have items are folded in:

| Item | Resolution | Where |
|---|---|---|
| **B1** host-check scheduling | `UptimeScheduler` gains a `hostSchedule` map + cold-start host hydration + a per-tick host pass enqueuing `UptimeJob{Kind: JobHostCheck}`; host due-times in-memory only. | §3.0, §3.1.2, §3.2.3, R4a |
| **B2** host-down short-circuit owner | The **worker** (not the ingester) owns host transition detection + synthetic child `down` fan-out + the consolidated notification; a shared `pool.hostState` map is read (RLock) by the scheduler for the skip decision. Ingester stays a dumb writer. `UptimeJob.Kind` added; `HostCheckResult` type added. | §3.0, §3.2.1, §3.2.3, §3.3.1/3.3.2, §3.8.2 |
| **B3** debounce vs droppable write | Authoritative `pool.monState` map (seeded from DB once), read-modify-written **synchronously by the worker** under `monMu`; the ingester's `status`/`failure_count` write is a persistence mirror. A dropped `CheckResult` cannot suppress a transition. New test: saturated ingester + all drops → `down` still detected + notified. | §3.0, §3.2.1, §3.3.3, §3.8.1, R2, §7.8 |
| **S1** C5 revertibility | Pool + ingester **constructed** in C3/C4 (`Run` started in C5). `git revert C5` genuinely restores the old ticker with C3/C4/C6/C7 dormant + building. | dependency-order note, Commits 3/4/5/7, Rollback |
| **S2** deferred-index premise | Row-count math corrected (~65 M steady-state at 500 monitors, not 13 M); first-boot index build honestly described as a bounded multi-minute write-contending op; 90-day default unchanged (user decision). | §3.4.2, §3.5.6, R3/R4/R7, Phase 5 |
| **S3** auto-created interval | `SyncMonitors` / `SyncAndCheckForHost` / `SyncAndCheckForRemoteServer` create monitors with `clampInterval(0, cfg)` → honour `uptime.default_interval_seconds`. Test added. | §3.1.3, §3.6.3, §7.2a, R10, Commit 5 |
| **S4** shutdown handshake | Explicit ordered teardown chain enforced by channel ownership (scheduler stops → pool `workerWG.Wait()` + closes `results` → ingester drains-until-closed + final flush). Grace-period check + no-result-loss test. | §3.1.4, §3.8.3, R13, §7.9, Commit 5 |
| **S5** p95 gate | `TestUptimeSummary_PerfBudget` (`-short`-skippable): 500-monitor + 24 h seed, index built, `GetSummary(30)` < 2 s wall-clock. | §3.5.3, §7.4a, R7, Commit 7 |
| **S6** backup/restore | §3.9: restore-then-restart = ordinary cold start (goroutines start after DB init); live restore self-heals within one `rescan()` + 1–2 cycles; `UptimeScheduler.Rehydrate()` called from `RestoreBackupSafe` reconcile makes it immediate. | §3.7 row, §3.9, R14, §7.9a, Commit 5 |
| **S7** `charon migrate` warning | Warning log before the eager `CREATE INDEX` + Phase 5 deploy-note sentence. | §3.5.6 item 3, Phase 5, Commit 6 |
| **N1** pruner chunk latency | Honest 100–500 ms/chunk on a cold huge table; `firstPassChunkPause = 250 ms` for the first pass. | §3.4.1, §3.7, R3, Commit 6 |
| **N2** keep-alive idleTimeout | 90 s → **30 s**; `safeclient_test.go` asserts a connection older than `idleTimeout` is not reused; 500-distinct-hosts churn noted. | §3.2.2, R11, Commit 4 |
| **N3** transient check-logic dup C4→C5 | Called out in the C4 scope note. | Commit 4 |
| **N4** mixed static/param route smoke test | `routes_test.go` asserts `/uptime/monitors/summary` and `/uptime/monitors/:id/history` both resolve. | Commit 7 |
| **N5** manual check drops silently | `POST /:id/check` → 503; `POST /system/uptime/check` → `{enqueued,dropped}`; frontend toasts. | §3.5.7, §3.7, Commit 5, Commit 8 |
| **N6** ARCHITECTURE.md omissions | `uptimeConfig` + the 3 `uptime.*` keys + the pooled-SSRF-client note added to the §5 update list. | §5 |
| **N7** error wrapping | `fmt.Errorf("context: %w", err)` called out in the Phase 2 per-commit requirement. | §4 Phase 2 |
| **N8** `uptimeConfig` test seam | `now func() time.Time` + `forceRefresh()`. | §3.6.2, Commit 2 |

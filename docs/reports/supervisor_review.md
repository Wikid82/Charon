# Supervisor Review — `docs/plans/current_spec.md` (Uptime Monitoring at Scale)

**Branch:** `feat/uptime-monitoring-scale`
**Reviewer:** Supervisor (Code Review Lead)
**Scope:** Plan review only. Settled scope (spec §8) not re-litigated.

---

## Implementation review 2026-08-27 (`development...feat/uptime-monitoring-scale`, 13 impl commits, +11354 / −1307) — Verdict: APPROVE

**Reviewer:** Supervisor (Code Review Lead). **Scope:** implementation vs. the approved
plan (`docs/plans/current_spec.md`) and the four APPROVE-WITH-CHANGES items from the
plan re-review below. Read-only; DoD artifacts spot-checked, not re-run.

The code delivers the design. All five original bottlenecks are addressed as planned,
the three Blocking plan items (B1/B2/B3) are implemented with concrete, non-vacuous
tests, and the four "changes required" (C1–C4) each landed. No blocking or should-fix
defects. Six nice-to-haves below, all closeable by a small follow-up or `qa-security`;
none is structural.

### Verified delivered

- **B1 — per-host scheduling.** `UptimeScheduler` carries `monSchedule` + `hostSchedule`
  + `hostMinInt`; cold-start hydrates hosts via `SELECT uptime_host_id, MIN(interval)
  … WHERE enabled AND uptime_host_id IS NOT NULL GROUP BY uptime_host_id`
  (`uptime_scheduler.go:184-202`); `runTick` runs `hostPass` then `monitorPass`
  (`:206-218`); host jobs go on the same bounded queue as `UptimeJob{Kind:
  JobHostCheck}`; `rescan()` recomputes `hostMinInt`/`hostSchedule` (`:368-381`). Host
  due-times are in-memory only (no schema change). Hosts with zero enabled children get
  no host check — the harmless behaviour delta already noted in the plan re-review.
  Tests: `TestUptimeScheduler_RunTick_HostPassBeforeMonitorPass`,
  `..._MonitorPass_SkipsTCPMonitorOfDownHostButNotHTTP`, `loadHostSchedules` coverage.
- **B2 — host-down short-circuit, single owner = worker.** `handleHostCheck`
  (`uptime_worker_pool.go:509-553`) detects up→down under `hostMu`, releases the lock,
  then `fanOutHostDown` synthesises `down` results for the host's not-already-down TCP
  children through the same `monMu` debounce and fires one `NotifyMonitorDown` per
  transitioning child (the real 30 s batch window consolidates). Scheduler reads
  `pool.HostState(hid)` in `hostDownSkips` (`uptime_scheduler.go:294-303`) and neither
  enqueues nor emits for those monitors while the host is down. The ingester has **no**
  transition logic / no fan-out — `flush()` is a pure column mirror
  (`uptime_ingester.go:257-319`). Tests: `TestUptimeWorkerPool_B2_HostDownFanout`,
  `..._AlreadyDownChildIsSkipped`, `..._ConcurrentHostStateReadsDoNotDeadlock`.
- **B3 — authoritative in-memory debounce.** `pool.monState` seeded once from the DB
  (`SeedState`), read-modify-written synchronously under `monMu` **before** `emit`
  (`uptime_worker_pool.go:391-419`); transition detection + notification read/write the
  map, never the DB snapshot; the ingester write is persistence-only and droppable.
  Traced: a saturated ingester channel that drops every `CheckResult` still produces the
  down transition and the alert — `TestUptimeWorkerPool_B3_DownTransitionDetectedDespiteEveryResultDropped`
  asserts exactly this. Manual `POST /:id/check` runs through the same `handleMonitorCheck`
  RMW — `TestUptimeWorkerPool_ManualJob_SharesMonState` kills the old both-read-N
  undercount.
- **C1 — deadline-bounded notification dispatch.** `dispatch()`
  (`uptime_worker_pool.go:488-505`) derives `context.WithTimeout(p.baseCtx(),
  notifyTimeout=10s)` and returns on `done` **or** `nctx.Done()`, so a hook that ignores
  ctx cannot wedge the worker or the `workerWG.Wait()` teardown link. `context.Background()`
  is gone from the worker notification path. Test:
  `TestUptimeWorkerPool_Dispatch_SlowHookDoesNotExceedDeadline` (3 s hook, worker returns
  in < 1 s).
- **C2 — narrowed `hostMu`.** `handleHostCheck` flips the `hostState` entry under
  `hostMu` then `Unlock()`s at `:537` *before* `fanOutHostDown`; the fan-out does its
  child DB read with no lock held and takes `monMu` only per-child for a ~5-field RMW.
- **C3 — documented lock order.** `hostMu → monMu` invariant is a struct-level comment
  block (`uptime_worker_pool.go:83-86`); in practice the two locks are never held
  simultaneously.
- **C4 — maps built outside locks.** `SeedState`/`loadMonState`/`loadHostState` run the
  SELECTs first, then swap under `monMu`/`hostMu` (`:154-174`); workers re-read
  `p.monState` after acquiring `monMu`.
- **S4 teardown.** Ordered chain wired in `routes.go:690-712` and `main.go:283-338`:
  scheduler stops enqueuing on `ctx` → `Pool.Run` does `workerWG.Wait()` then
  `ingester.closeResults()` (sole sender/closer, `sync.Once`-guarded,
  `uptime_ingester.go:144-146`) → ingester `Run` ranges the closed channel, final
  `doFlush`, returns → `main.go` blocks on `uptimeShutdown` up to 25 s. Every worker
  `handle()` is bounded (probe ctx + `dispatch` deadline), so `close(results)` is always
  reached — no hang. Test: `TestUptimeWorkerPool_Run_DrainsWorkersAndClosesResultsOnCancel`.
- **Pruner + deferred index.** Chunked subquery `DELETE … WHERE id IN (SELECT id …
  ORDER BY id LIMIT ?)` (`uptime_pruner.go:178-185`, fully parameterised), `time.Sleep`
  between chunks with a wider first-pass pause, `PRAGMA wal_checkpoint(TRUNCATE)` after a
  ≥ 50 k-row prune. `ensureIndex` uses `CREATE INDEX IF NOT EXISTS`, an `atomic.Bool`
  short-circuit set **only after success**, and is retried every caught-up pass — no
  `sync.Once`, so a transient early failure never permanently strands it
  (`:213-228`). `charon migrate` builds it eagerly with the WARN log (`main.go:159-171`).
- **Summary endpoint.** Exactly three queries regardless of monitor count
  (`GetSummary` → `loadMonitors` / `loadRecentBeats` / `loadUptime24h`), `ROW_NUMBER()`
  windowed + grouped 24 h, 30 s TTL cache, `beats` default 30 / cap 60 (handler +
  `clampBeats`), snake_case tags, `max_retries` present with the legacy `<=0 → 3`
  default. Both raw SQL blocks are `?`-parameterised on the window edge and row cap with
  no interpolation. `TestUptimeSummary_CorrectWithAndWithoutIndex` covers both plans.
- **Shared keep-alive SSRF client.** `WithKeepAlive(100, 4, 30s)` flips only
  `DisableKeepAlives` / `MaxIdleConns` / `MaxIdleConnsPerHost` / `IdleConnTimeout`
  (`safeclient.go:466-471`); `safeDialer` still re-validates every new connection and
  redirect policy is untouched. Tests assert link-local/metadata still blocked, an
  idle conn past `idleTimeout` is not reused, and redirects still not followed
  (`safeclient_test.go:1329-1383`).
- **Security.** Auth: `/uptime/monitors/summary` and `/uptime/health` sit in the
  `management` group behind `RequireManagementAccess()` (`routes.go:641-650`).
  Interval floor enforced on every write path — `SettingsHandler.UpdateSetting` via
  `validateUptimeSetting` bounds `[30, 86400]`; `UptimeHandler.Create` rejects
  `0 < n < 30`; `UptimeService.UpdateMonitor` rejects a positive sub-floor;
  `CreateMonitor` + all auto-create paths (`SyncMonitors`, `SyncAndCheckForHost`,
  `SyncAndCheckForRemoteServer`) use `clampInterval(0, cfg)`; the scheduler re-clamps
  every interval at scheduling time. `remote_server_handler.go` sync hooks are
  `h.uptimeService != nil`-guarded on Create/Update/Delete. The two `go/log-injection`
  suppressions in `3c9ac3bd` are legitimate — both sinks log a `uint` GORM primary key
  (`remote_server_id`) that cannot carry CR/LF; the taint is only the enclosing
  handler's `ShouldBindJSON`.
- **CLAUDE.md.** Explicit `json:"snake_case"` on `MonitorSummary` / `BeatDTO` /
  `NextCheckAt`; UUID string IDs unchanged; `fmt.Errorf("…: %w", err)` wrapping
  throughout the new services; all `Run` loops return on `ctx.Done()`;
  `feature.uptime.enabled` still a working kill switch (scheduler no-ops per tick,
  pool/ingester idle, sync loop gated, pruner deliberately keeps running). The N3
  `checkMonitor` probe-switch duplication is collapsed — `checkMonitor` now delegates
  to `s.checker.probe` and writes only through `Ingester.FlushResults`. No dead code,
  debug prints, or commented blocks in the new files.
- **Deviations from the spec's literal file lists** (config self-built in
  `NewUptimeService`; ingester owns its channel + `closeResults()`; pool hangs off
  `UptimeService.Pool`; `SetUptimeService` / `SetUptimeRehydrator` setters;
  `RegisterWithDeps` returns an `UptimeShutdownFunc`; `probe`/`probeHost` split with
  debounce in the pool) — all sound; none loses a property the spec depended on. The
  setters are nil-guarded and keep ~20 existing call sites stable; the shutdown func is
  threaded through `main.go` correctly.

### Nice-to-have (non-blocking; small follow-up or QA)

- **NI-1 — `last_notified_down` is carried but never persisted for monitors.**
  `uptime_ingester.go` `flush()` (`~:287-300`) writes `status` / `last_check` /
  `latency` / `failure_count` / `last_status_change` but not `last_notified_down`, yet
  the worker maintains `monDebounce.lastNotifiedDown` (`uptime_worker_pool.go:398, 612`)
  and `loadMonState` reads the column back (`:206`). No monitor-level code consumes it
  today (only the host-level `host.LastNotifiedDown` re-notify damper at
  `uptime_service.go:828` is live), and it was already unused pre-PR, so this is not a
  regression — but the in-memory field now looks authoritative while resetting to a
  never-written value on every restart. Fix: drop `lastNotifiedDown` from `monDebounce`
  / `loadMonState`, or thread it onto `CheckResult` and persist it if a monitor
  re-notify damper is ever wanted.
- **NI-2 — shutdown-grace arithmetic vs. change C1.** `main.go:328` uses a 25 s drain
  ctx; `handleMonitorCheck` can in principle take `uptimeCheckHardCap` (20 s) + a
  synchronous `dispatch` bounded by `notifyTimeout` (10 s) = 30 s on the `workerWG`
  path, now that C1 put notification dispatch there. No real hang — on `ctx` cancel
  `p.baseCtx()` is already-done, so both sub-contexts are born cancelled and unwind
  immediately; the only true bound is `http.Client.Timeout` (20 s) for a connection
  already reading a slow body, which fits inside 25 s. Either bump the drain ctx to
  ~35 s (`hardCap + notifyTimeout + margin`) or add a comment at `drainCtx` noting the
  cancelled-ctx fast-unwind is what keeps 25 s sufficient.
- **NI-3 — `TestUptimeSummary_PerfBudget` seed is light and its comment's math is
  wrong.** `uptime_summary_service_test.go:271` seeds 500 × 120 ≈ 60 k rows over 24 h;
  the header comment cites a "500 × ~720 ≈ 360 k" production profile, but at the 30 s
  interval floor a 24 h window is 500 × 2880 ≈ 1.44 M rows — ~24× the seed. The `< 2 s`
  ceiling trips only on a per-monitor-loop regression, not on index-not-used or O(n²)
  slippage. Raise `beatsPerMon` toward 2880 and correct the comment so the guard
  exercises the windowed `ROW_NUMBER()` at a realistic size. (The plan re-review's S5
  already downgraded the p95 gate to "QA timing output, not hard-gated", so this is
  polish, not a broken gate.)
- **NI-4 — stale "transient duplicate" comment.** `uptime_check.go:56-59` still says the
  http/tcp/orthrus switch is "a deliberate transient duplicate … for the length of this
  commit — C5 collapses the legacy path onto this one." C5 did collapse it; trim the
  comment to describe the switch as the sole probe path.
- **NI-5 — perpetually-due rows when a snapshot load returns fewer rows than
  requested.** `hostPass` / `monitorPass` (`uptime_scheduler.go:237-246, 277-288`) only
  advance the schedule entry for IDs returned by `loadHostSnapshots` /
  `loadJobSnapshots`. A row deleted between the due-scan and the snapshot load is never
  advanced and is re-selected every tick until the next `rescan()` prunes it (≤ 30 s).
  Harmless churn; if desired, advance-or-drop any due ID absent from the snapshot.
- **NI-6 — `dispatch()` goroutine for the non-blocking down path.**
  `uptime_worker_pool.go:488` wraps `NotifyMonitorDown` (documented fast: map insert +
  `AfterFunc`) in the same goroutine + select deadline harness as the external-send
  `NotifyMonitorUp`. Bounded by worker count, so cosmetic — consider calling the down
  path inline and reserving `dispatch()` for the recovery path.

### DoD spot-check

Not re-run (per dispatch). Cross-checked that the artifacts the orchestrator reported
are consistent with the tree: the B1/B2/B3/C1/S4 tests named above exist and assert the
right invariants; `codeql-suppressions.yml` entries are dated with `review_by`; the N4
route-resolution smoke test (`routes_test.go:TestRegister_UptimeSummaryAndHistoryRoutesResolve`)
is present. Nothing observed that contradicts the reported green state.

---

## Re-review 2026-08-27 (revision +1213 / −670) — Verdict: APPROVE WITH CHANGES

The three Blocking items are **genuinely closed**. The fixes are complete, internally
consistent across §3.0 / §3.1.2 / §3.2.3 / §3.3.3 / R4a, and free of the
contradictions and gaps the first pass flagged. S1 is fully resolved; S2/S5/S6/S7
resolved appropriately. Verified against `uptime_service.go`, `stats_ingester.go`,
`safeclient.go`, `routes.go`, `remote_server_handler.go`, `database.go`.

**B1 (host-check scheduling) — closed.** `UptimeScheduler` now carries
`monSchedule` + `hostSchedule` + `hostMinInt`; cold-start hydrates hosts via
`SELECT uptime_host_id, MIN(interval) … GROUP BY`; per-tick host pass then monitor
pass; `UptimeJob{Kind: JobHostCheck}` on the same bounded queue with the same
drop-on-full semantics; `hostMinInt` recomputed on `rescan()`; host due-times
in-memory only (no schema change). §3.0/§3.1.2/§3.2.3/R4a all agree. Behaviour
delta: `UptimeHost` rows with zero enabled child monitors no longer get a TCP
pre-check (old `checkAllHosts()` scanned all hosts) — harmless, arguably an
improvement; note for the implementer.

**B2 (host-down short-circuit owner) — closed.** Single owner: the worker running
the `JobHostCheck`, synchronously — writes `pool.hostState`, synthesizes `down`
`CheckResult`s for the host's not-already-down TCP children through the same
`monMu` debounce, fires one consolidated `queueDownNotification`. No ingester
back-channel; the ingester stays a dumb column-copy writer (no contradiction with
§3.3.2/§3.3.3). No double-transition (synthetic + any in-flight real check both
resolve against `monState`, second sees `StatusChanged=false`). No deadlock: the
only nested acquisition is `hostMu → monMu` in the fan-out path and nothing
acquires `monMu → hostMu`, so there is no lock-order cycle.

**B3 (authoritative in-memory debounce) — closed.** `pool.monState`
(`{status, failureCount, lastStatusChange, lastNotifiedDown}`) seeded once from
the DB (`SeedState`), read-modify-written synchronously by the worker under
`monMu`, and the transition + notification are computed **before** `p.emit()` —
so a dropped `CheckResult` loses only the DB mirror row, never the increment or
the alert. Single `sync.Mutex` is adequate: per-monitor RMW is effectively serial
(scheduler enqueues ≤1 job/monitor/cycle), critical section is ~5 field writes,
notification dispatch happens *after* `monMu` is released; `fnv(id)%64` sharding
is a noted mechanical escape hatch. Manual `POST /:id/check` shares the same
lock+map, killing the old "both read N, both write N+1" undercount. C5 test is
concrete (saturate ingester, every send drops, feed `maxRetries` downs, assert
transition detected + `queueDownNotification` called).

**S1 — resolved.** Pool + ingester are now *constructed* in C3/C4 (`routes.go`
lines) and only *started* (`Run` loops) in C5. `git revert C5` genuinely restores
the legacy ticker with C3/C4 pool/ingester constructed-but-idle
(`QueueDepth()`/`DroppedCount()` return 0, no goroutines started, no leak),
C6/C7/C8 dormant and building — C7's `/uptime/health` depends on C4, not C5, and
still serves valid JSON. Dependency-order line and every Depends-on/Rollback line
updated to match. All C1–C9 build.

**S2 — resolved as honest documentation.** Row-count math corrected (500 mon ×
1440/day × 90 d ≈ 65 M steady state; ~130 M at 30 s; hundreds of millions on a
years-stale instance). Retention default stays 90 (user's explicit decision). R4
is re-rated "Med / Med (honest — *not* designed away)" and states plainly that
prune-first removes only the pathological case, not the multi-minute build;
escape hatches (`charon migrate` in a window, or lower retention before first
boot) are documented in §3.4.2, §3.5.6, R4 and the Phase 5 deploy note.

**S4 — resolved by design, one implementation gap (see C1 below).** The teardown
is an ordered chain enforced by channel ownership, not five components racing on
`ctx`: scheduler stops enqueuing → pool `workerWG.Wait()` → pool (sole sender)
`close(results)` → ingester `for r := range results` terminates only on close,
then one final `flush()`. This cannot hang on the happy path: every worker
`handle()` is bounded by `hardCap` (20 s) and the HTTP client's own 20 s timeout,
so `workerWG.Wait()` returns and `close(results)` is reached. The
grace-period requirement (`http.Server.Shutdown` grace ≥ `hardCap` + ~2 s) is
called out as a C5 verify-step. C5 test asserts an in-flight check's heartbeat is
still written on immediate `ctx` cancel.

**S5 — resolved.** `TestUptimeSummary_PerfBudget` (C7, `-short`-skippable): seeds
500 monitors × 24 h heartbeats, builds the index, asserts `GetSummary(ctx, 30)`
< 2 s wall-clock — a real assertion and a real gate against gross regression. The
< 300 ms p95 is explicitly downgraded to "tracked from QA timing output, not
hard-gated" (runner variance) — an acceptable, honest resolution of my S5 ask.

**S6 — resolved.** §3.9: restore-then-restart is ordinary crash-recovery cold
start (goroutines launch in `routes.Register`, after DB init) — no special
handling. Live restore without restart self-heals within one `rescan()` (≤ 30 s)
+ 1–2 cycles; `RestoreBackupSafe`'s reconcile step calls
`UptimeScheduler.Rehydrate()` (re-`hydrate()` + `pool.ReseedState()`) to make it
immediate; `firstPassDone` resets so a large post-restore prune re-uses the wider
pause. C5 test covers the reconcile hook. No new race that the self-heal fallback
doesn't already cover.

**S3 / S7 — resolved.** Auto-create paths (proxy-host + remote-server) now use
`clampInterval(0, cfg)` instead of hardcoded `Interval: 60` (§3.1.3, §3.6.3, C5).
`charon migrate` gets a `WARN` log + eager unconditional `CREATE INDEX`, with the
deploy note telling operators to lower retention first if the table is huge.

### Changes required before / during implementation (all backend-dev-resolvable, no spec rewrite)

- **C1 (should-fix, touches the S4 guarantee).** The worker's *synchronous*
  notification dispatch (`queueDownNotification` / `sendRecoveryNotification` →
  `NotificationService.SendExternal`, currently `context.Background()`) sits on
  the shutdown-critical path: teardown blocks on `workerWG.Wait()`, and a worker
  firing an alert at shutdown against a slow/hung webhook is **not** bounded by
  `hardCap`. Require the worker-side notification path to use a deadline-bounded
  context (e.g. `context.WithTimeout(…, 5s)`), or state explicitly that teardown
  does not wait on external-notification completion. Add one sentence to §3.1.4 /
  §3.3.3 and a C5 test assertion (ctx-cancel with a blocking webhook mock still
  drains within grace).
- **C2 (should-fix, latency hygiene).** §3.2.3 reads as if the host-check worker
  holds `hostMu` (write) across the entire child fan-out + consolidated
  notification (N × `monMu` RMW + a DB read + `notificationMutex`), which stalls
  the scheduler's `HostState()` RLock and other host workers. Implementation:
  flip the `hostState` entry under `hostMu`, release, *then* do the fan-out +
  notification. Capture as a C5 implementation note.
- **C3 (nice-to-have).** Make the `hostMu → monMu` ordering an explicit
  documented invariant (code comment + one spec line) so a later change can't
  introduce `monMu → hostMu` and deadlock.
- **C4 (nice-to-have).** `ReseedState()` should build the new `monState` /
  `hostState` maps from its two SELECTs *outside* the locks, then swap under
  `monMu` / `hostMu`, so a live restore doesn't stall workers for the duration of
  the reads. Workers must re-read `p.monState` *after* acquiring `monMu`.
- **C5 (nice-to-have, wording).** §3.3.3 "restart reseed staleness ≤ 1–2 cycles
  ≈ 60 s" is optimistic for a monitor with a high `MaxRetries` and many
  consecutively-dropped flushes — it's "≤ `MaxRetries` cycles." The property
  ("alert still fires, just later") holds; tighten the phrasing.

None of C1–C5 is a design defect or a contradiction; each has a one-line fix at
implementation time. Recommend the coordinator hand C1–C2 to `backend-dev`
explicitly in the C5 dispatch prompt. **Implementation may proceed.**

---

## Original review 2026-08-27 — Verdict: REVISE

*(retained for traceability; B1/B2/B3/S1/S2/S4/S5/S6/S7 addressed in the revision above)*

## Verdict: REVISE

The architecture is sound in outline — mirroring `StatsIngester`/`StatsService` for the write/read paths is the right call, the commit slicing is mostly well-ordered, and CLAUDE.md/ARCHITECTURE.md compliance is largely covered. But three items below are **design gaps or internal contradictions**, not polish: an implementer cannot proceed on §3.2.3 (host TCP pre-check scheduling) unambiguously, the ingester/worker split for host-down short-circuit contradicts §3.3.3, and the failure-debounce path regresses down-alert reliability under the exact overload the feature targets. Close the three Blocking items in the spec, apply the Should-fix edits, and this becomes APPROVE.

The five original bottlenecks are otherwise addressed: per-monitor scheduler kills the global ticker; bounded pool kills unbounded fan-out; shared keep-alive client kills per-check TCP/TLS; ingester takes writes off the check path (mitigating, not removing, the `SetMaxOpenConns(1)` constraint — correctly framed as out of scope); pruner bounds table growth; summary endpoint kills the `Uptime.tsx` N+1.

---

## Blocking

### B1. Host TCP pre-check has no scheduling design in the new model
**§3.2.3 vs §3.1.2 / §3.0.** The plan removes `checkAllHosts()` "as the scheduling mechanism" (§3.1.5) and says host checks are "enqueued as `UptimeJob{Kind: hostCheck}` ... scheduled on the host's own cadence = min of its monitors' intervals, floored at 30s." But `UptimeScheduler` (§3.1.2) is entirely monitor-centric: `schedule map[string]time.Time` is keyed by `monitorID`, cold-start hydration selects from `uptime_monitors`, the re-scan is `WHERE enabled = true`. Nothing hydrates, computes due-time for, or enqueues `UptimeHost` rows. The old code ran `checkAllHosts()` first on every tick; the new design silently drops that with no replacement.

**Required change:** Specify host-check scheduling concretely — e.g. `UptimeHost` entries in the same `schedule` map under a `host-<id>` key, hydrated alongside monitors, due-time = `min(child monitor intervals)` floored at 30s, enqueued as a distinct job kind. Include it in the §3.1.2 per-tick loop, the cold-start hydration, and the write-back (or state that host due-times are in-memory only and not persisted). Add scheduler tests for host due-selection.

### B2. Host-down short-circuit contradicts "no transition logic in the ingester"
**§3.2.3 vs §3.3.3 / §3.0.** §3.3.3 states the ingester does "no read-modify-write, no transition logic" — it is a dumb column copy. §3.2.3 then says "when the ingester writes a host→`down` transition it emits synthetic `down` heartbeats + coalesced monitor updates for that host's `tcp`-type monitors, and the scheduler skips enqueueing them until the host recovers." That requires the ingester to (a) detect a host status transition (transition logic), (b) fan out synthetic heartbeats for N child monitors, and (c) signal the scheduler to skip those monitors — a back-channel not present in the §3.0 component map (which shows only worker→ingester).

**Required change:** Give the host-down short-circuit a single explicit owner. Recommended: the **worker** performing the host check detects the host transition synchronously (same as monitor transitions in §3.3.3), emits the synthetic child `down` results into the normal `CheckResult` stream, and updates a shared in-memory host-state map that the scheduler consults in its due-selection (`skip tcp monitors whose host is known-down`). Define that map and its ownership. Also reconcile the `UptimeJob` struct (§3.2.1 has only `Monitor` + `Manual`; §3.2.3 references `UptimeJob{Kind: hostCheck}`) — add the `Kind` field or model host jobs as a separate type.

### B3. Failure-debounce depends on a droppable async write — down alerts can be suppressed under ingester saturation
**§3.3.2 / §3.3.3 / R2.** Today `checkMonitor` does a synchronous read-modify-write of `FailureCount` (fetch row → `FailureCount++` → `s.DB.Save`). The new design has the worker compute `FailureCount` from the **scheduler's snapshot** and the ingester persist it **~500 ms later, coalesced, drop-on-full**. Normal case is fine (500 ms flush ≪ 30 s interval floor). Failure case is not: under sustained ingester-channel saturation — the precise overload `Send`'s drop-on-full exists for — successive `CheckResult`s for a failing monitor are dropped, so `failure_count` never persists, so every subsequent check's snapshot shows the same stale-low count, so `FailureCount` never reaches `maxRetries`, so **the down transition is never detected and the down alert never fires.** R2's "notifications already fired synchronously so alerting is unaffected" is false here — synchronous dispatch cannot help a transition that is never detected. Same root cause undercounts a real failure streak when a manual `POST /:id/check` races a scheduled check (both read snapshot `N`, both write `N+1`, coalesce drops one increment) — §3.3.3's race note covers double-*notify* but not this.

**Required change:** Make the debounce state authoritative in memory, not dependent on the droppable DB round-trip. E.g. a `map[monitorID]{status, failureCount, lastStatusChange}` owned by the scheduler or a small transition-state store, seeded from the DB at cold start, updated synchronously by the worker (alongside the notification dispatch), and consulted by the next check. The ingester's `status`/`failure_count` write then becomes a pure mirror and the scheduler-tick-vs-flush ordering dependency disappears. Add a test: a dropped `CheckResult` must not delay or suppress a subsequent down transition.

---

## Should-fix

### S1. C5 is not revertible in isolation as claimed
**Commit Slicing / §Rollback.** The plan says `git revert <C5-sha>` "restores the legacy ticker while keeping C2–C4/C6–C8 dormant." But Commit 7 explicitly "Inject `UptimeSummaryService` + pool/ingester refs into the handler" for `GET /uptime/health`, and the pool + ingester are constructed in C5. Reverting C5 after C7 has landed breaks C7's `Health` handler (nil pool/ingester) and the tree no longer builds.

**Required change:** Move construction of `UptimeIngester` and `UptimeWorkerPool` into C3 and C4 respectively (constructed and wired, but `Run(ctx)` not started until C5 flips the execution model). C5 then only: deletes the ticker go-func, starts the `Run` loops, collapses `checkMonitor`, adds the remote-server hook. That makes `git revert C5` genuinely restore the old ticker with C3/C4/C6/C7 dormant. Update the §Rollback narrative accordingly.

### S2. Deferred-index premise doesn't hold at the target scale
**§3.4.2 / §3.5.6 / R4.** "the index build never happens against the full multi-million-row table — it builds against the already-trimmed table in seconds." Recompute the *retained* row count: 500 monitors × 90-day default retention × 1440 checks/day (60 s) ≈ **65 M rows** after trimming (≈130 M at a 30 s interval). `CREATE INDEX` on 65 M rows through the single connection is minutes and a hard write stall — the very thing the deferral was meant to avoid. The "~13 M rows" figure in §3.4.2 is the *current* 100-monitor case, not the 500-monitor target this spec is scoped to.

**Required change:** Either (a) lower the `uptime.heartbeat_retention_days` default to ~30 (downsampling is a non-goal, so retention is the only size lever), or (b) state honestly that on large deployments the first index build still stalls writes for minutes and steer operators to `charon migrate` in a maintenance window (already eager there), or (c) both. Fix the row-count math in §3.4.2, §3.5.6, R4, R7 and the Phase 5 deploy note.

### S3. Auto-created monitors bypass the admin default interval
**§3.1.3 / observed `uptime_service.go` ~1400 (`Interval: 60`) and `SyncMonitors`.** `SyncAndCheckForHost`, the new `SyncAndCheckForRemoteServer`, and `SyncMonitors` all hardcode `Interval: 60` on monitor creation. Objective §1.2.2 says the admin global default applies to "new/legacy monitors," and `clampInterval` only substitutes the default for `interval <= 0` — a stored `60` is never revisited. So proxy-host and remote-server monitors are pinned to 60 s regardless of `uptime.default_interval_seconds` unless hand-edited.

**Required change:** Route all monitor-creation paths (`CreateMonitor`, `SyncMonitors`, `SyncAndCheckForHost`, `SyncAndCheckForRemoteServer`) through the same write-time default resolution (`interval <= 0 → cfg.DefaultIntervalSeconds()`), and create auto-monitors with `Interval: 0` so the default applies. Or explicitly document that auto-created monitors always start at 60 s and the global default only affects API-created ones. Add a test.

### S4. Graceful-shutdown handshake is underspecified
**§3.1.4.** All five components receive the same `Register` `ctx` and react to `ctx.Done()` concurrently. §3.1.4 describes an intended order (pool drains → ingester final flush) but nothing enforces it. If the ingester's `Run` returns on `ctx.Done()` while a worker is still finishing a ≤20 s check, that worker's `emit` to `results` is lost — and its synchronous notification already fired, so status/heartbeat persistence for a real transition is dropped on every shutdown.

**Required change:** Specify an explicit teardown sequence with channels/WaitGroups, not just shared-ctx: signal scheduler stop → close `jobs` → `WaitGroup.Wait()` on workers → close `results` → ingester drains until `results` closed → final flush → `Stop()`. Add a shutdown test asserting no result loss for an in-flight check.

### S5. Acceptance criterion #1 (p95 < 300 ms) has no automated gate
**§7 Functional #1 / §4 Phase 4.** The p95 target is the headline objective, but the only verification is "Manual smoke ... (optional but recommended)." Nothing in the C7 or C9 validation gates measures it.

**Required change:** Add a benchmark or timed test in C7 (`UptimeSummaryService.GetSummary` against a seeded 500-monitor / 24 h-of-beats fixture, index present) asserting a wall-clock ceiling, or explicitly downgrade #1 to a manually-measured target and say so in §7.

### S6. Backup/restore interaction is not addressed
**Task item 7 / spec has no mention.** A backup taken mid-flight loses ≤500 ms of buffered heartbeats (acceptable). Restore + restart is a cold start from possibly-hours-stale `next_check_at` values → every monitor treated as due → 60 s jittered catch-up storm, bounded by queue cap + per-tick cap. The first post-restore prune pass may be very large if the backup is old.

**Required change:** Add a short subsection (or a row in §3.7) stating restored instances behave exactly like crash-recovery cold start, no special handling needed, and note the large-first-prune consequence. Confirm the pruner/scheduler goroutines are started after DB restore completes (they use the `Register` ctx, so this should already hold — state it).

### S7. `charon migrate` eager index vs. deferred-index ordering
**§3.5.6.3 / R3-R4.** The `migrate` CLI runs `CREATE INDEX IF NOT EXISTS` unconditionally right after `AutoMigrate`, i.e. **before** any prune. On a large existing table that is exactly the multi-minute write-blocking build the runtime path was restructured to avoid — only now with no chunked-prune-first ordering. §3.5.6 calls this "acceptable" because it is operator-initiated, which is defensible, but the CLI should at minimum log a clear "building index on N rows, this may take several minutes" warning, and the deploy doc must tell operators to prune first (lower retention, start server once, or accept the wait).

**Required change:** Add the warning log line to the C6 `main.go` change and a sentence to the Phase 5 deploy note.

---

## Nice-to-have

- **N1. Pruner per-chunk latency estimate is optimistic.** §3.7 / §3.4.1 say "~10–30 ms" per 5 000-row chunk and "worst added API latency ≈ one chunk." On a cold, huge table the first-pass chunks can be 100–500 ms each; with `SetMaxOpenConns(1)` + `busy_timeout=5000` the ingester flush and API writes block for that duration. Recommend a larger `pruneChunkPause` (or smaller `pruneChunkSize`) for the first pass specifically, and state the honest worst-case in §3.7.
- **N2. Keep-alive Layer-2 revalidation staleness.** `WithKeepAlive(100, 4, 90s)` — a pooled idle connection skips `safeDialer` (Layer 2 re-resolution) for up to 90 s. No actual SSRF results (an established TCP connection can't be rebound), so R11's Low rating is fair, but consider `idleTimeout` = 30 s to bound the staleness window and add a `safeclient_test.go` case asserting a connection older than `idleTimeout` is not reused. Also note `MaxIdleConnsPerHost: 4` / `MaxIdleConns: 100` assume significant host reuse — with 500 distinct hosts the pool thrashes and the keep-alive win shrinks (still net-positive).
- **N3. Transient check-logic duplication C4→C5.** C4 adds `uptime_check.go` (`runCheck`) as a "parallel pure function" while `checkMonitor` stays intact, resolved in C5. Acceptable and deliberate, but call it out in the C4 commit notes so it doesn't trip a DRY review (CLAUDE.md "consolidate after the second occurrence").
- **N4. Route smoke test for mixed static/param.** `GET /uptime/monitors/summary` sits beside `GET /uptime/monitors/:id/history`. Precedent exists in this codebase (`POST /remote-servers/test` vs `POST /remote-servers/:uuid/test` on Gin v1.12.0), so this should be fine — add one assertion in `routes_test.go` (or the handler test) that both resolve, so a future Gin bump can't silently break it.
- **N5. Manual "check now" drops silently on a full queue.** `CheckAll()` re-implemented as "enqueue all" and `POST /system/uptime/check` will `TryEnqueue`-drop under load with only a metric. For an explicit operator action, prefer blocking `Enqueue` with a short timeout or return the enqueued/dropped counts in the response body.
- **N6. ARCHITECTURE.md §5 omissions.** The `uptimeConfig` hot-reload snapshot component and the three `uptime.*` Settings keys (with bounds + hot-reload semantics) aren't in the §5 update list — add them to the config/settings area. The SSRF-client "Keep-alives disabled" default note in ARCHITECTURE.md's security section should gain a line that the uptime worker pool now uses a pooled variant.
- **N7. Error wrapping.** New services should be held to `fmt.Errorf("context: %w", err)` per CLAUDE.md — state it explicitly in the Phase 2 per-commit requirements (the spec shows `GetSummary: %w`-style only by mirrored example).
- **N8. `uptimeConfig` test seam.** It has `db` but no injectable clock; hot-reload tests will need a way to force TTL expiry. Add a `now func() time.Time` or an exported `forceRefresh()` for tests.

---

## Non-findings (checked, no issue)

- **Auth on the 3 new endpoints.** All land in the `management` group behind `RequireManagementAccess()` (routes.go ~350), consistent with existing `/uptime/*` and `/stats/summary`. `/uptime/health` exposes only queue counts — management gating is appropriate. Endpoints reachable when `feature.uptime.enabled=false` is consistent with existing `/uptime/monitors` and intended (§3.7).
- **Injection surface.** Pruner `db.Exec` uses `?` placeholders for `cutoff`/`LIMIT` value is a compile-time const; the `ROW_NUMBER()` summary query parameterises the window start and `beats` (clamped 1–60 int); `CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created ...` is a static literal. No string interpolation of request data anywhere in the new query paths.
- **`DELETE ... LIMIT` avoidance.** Correct — `modernc.org/sqlite v1.57.0` is not built with `SQLITE_ENABLE_UPDATE_DELETE_LIMIT`; the `WHERE id IN (SELECT ... LIMIT n)` subquery form is required and is what the spec uses. Window functions are available (SQLite ≥ 3.25). Both claims verified against `go.mod`.
- **SSRF security-parity argument for the shared client.** Valid. Every per-check client today already passes `WithAllowLocalhost()` + `WithAllowRFC1918()` + `WithMaxRedirects(0)` (`uptime_service.go` ~850), and `safeDialer` re-validates every *new* connection's resolved IP against `IsPrivateIP` with the RFC1918 carve-out. One shared client with identical options is not a regression; link-local / metadata / reserved stay blocked at both layers. `security.ValidateExternalURL` (Layer 1) is still called per HTTP check. (See N2 for the one residual nuance.)
- **JSON tags / UUID IDs.** `MonitorSummary` / `BeatDTO` / `NextCheckAt` all carry explicit `json:"snake_case"` tags; IDs stay server-generated UUID strings. `filepath.Clean` is N/A (no filesystem paths in this feature).
- **`feature.uptime.enabled` kill switch.** Scheduler no-ops on the cached flag, pool/ingester idle, pruner keeps running (retention should apply while checking is paused), summary serves last-known data. Sensible; matches §Rollback's field kill-switch.
- **`CheckAll()` retained + callers.** Only production callers are `POST /system/uptime/check` (routes.go:688) and `runInitialUptimeBootstrap`; the latter drops it (backfill covers boot), the former enqueues. `uptimeBootstrapService` interface + `routes_uptime_bootstrap_test.go` update is called out in C5. Adequate.

---

## If the three Blocking items are resolved

Downgrades to **APPROVE WITH CHANGES**, requiring before implementation starts:
1. B1 — concrete host-check scheduling design folded into §3.1.2 / §3.2.3 with tests.
2. B2 — single named owner for host-down short-circuit + shared host-state map + `UptimeJob.Kind` reconciled.
3. B3 — in-memory authoritative debounce state + drop-does-not-suppress-alert test.
4. S1 — ingester/pool construction moved to C3/C4 so C5 revert is real; §Rollback text corrected.
5. S2 — retained-row-count math corrected; retention default lowered or the residual stall documented honestly.
6. S3 — auto-created monitors honour (or explicitly don't) the admin default interval.
7. S4 — explicit shutdown handshake with a no-result-loss test.
8. S5 — automated p95 gate or criterion #1 downgraded in §7.
9. S6 / S7 — backup-restore paragraph; `migrate` CLI warning + deploy-note sentence.

S-items 1–7 are all localized spec edits; none require rethinking the architecture.

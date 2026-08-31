# QA & Security Report — Uptime Monitoring at Scale

**Branch**: `feat/uptime-monitoring-scale` (16 commits ahead of `development`)
**Reviewed by**: qa-security agent (final pipeline pass)
**Date**: 2026-08-27
**Plan**: `docs/plans/current_spec.md`
**Prior review**: `docs/reports/supervisor_review.md` (implementation verdict: APPROVE)
**Diff**: 59 files, +11354 / −1307 (`git diff development...feat/uptime-monitoring-scale`)

---

## Final verdict: **PASS** — ready to merge

No blocking security or quality issues. All Definition-of-Done gates pass with real
numbers below. Three of the supervisor's six nice-to-haves were fixed in this pass
(comment-only, commit `b59115ab`); the remaining three are documented with rationale
for deferral. One local-environment note (stale git worktrees polluting the Trivy
scan — not part of this branch) is called out under Trivy.

---

## 1. Security Audit

### 1.1 SSRF — shared keep-alive HTTP client (`network/safeclient.go` + `uptime_check.go`) — PASS

| Check | Result |
|---|---|
| `safeDialer` re-resolves + re-validates the destination IP on **every new connection** | CONFIRMED. `safeDialer(&cfg)` is wired as `Transport.DialContext`; it runs `net.DefaultResolver.LookupIPAddr` + `IsPrivateIP`/`IsRFC1918` gating per dial. `WithKeepAlive` does **not** touch it. |
| Pooled idle connections can't reach a rebind-poisoned host | CONFIRMED. Keep-alive only flips `DisableKeepAlives` / `MaxIdleConns` (100) / `MaxIdleConnsPerHost` (4) / `IdleConnTimeout` (30s). An established TCP socket cannot be re-bound to a new IP; a reused idle conn keeps talking to the already-validated peer. `idleConnTimeout = 30s` bounds the revalidation-staleness window (spec §3.2.2 / N2). |
| Link-local / `169.254.169.254` / cloud-metadata / other reserved ranges blocked with keep-alive ON | CONFIRMED. `WithAllowRFC1918()` whitelists **only** `10/8`, `172.16/12`, `192.168/16` (`rfc1918CIDRs`). `169.254.0.0/16`, loopback (unless `WithAllowLocalhost`), `0.0.0.0/8`, `240.0.0.0/4`, `::1`, `fc00::/7`, `fe80::/10`, and Go's `IsLinkLocal*`/`IsMulticast`/`IsUnspecified` fast-path all still reject. Verified by `safeclient_test.go` (link-local + metadata blocked with keep-alive on; idle conn past `idleTimeout` not reused; redirects still not followed). |
| Redirects not followed | CONFIRMED. `newUptimeChecker` passes `WithMaxRedirects(0)` → `CheckRedirect` returns `http.ErrUseLastResponse`. Unchanged by keep-alive. |
| Layer-1 `ValidateExternalURL` still called per HTTP check | CONFIRMED. `uptimeChecker.probe` calls `security.ValidateExternalURL(monitor.URL, WithAllowLocalhost, WithAllowHTTP, WithTimeout(3s), WithAllowRFC1918)` before every `http/https` request. |
| Double DNS lookup (Layer 1 validate + Layer 2 dial) NOT collapsed | CONFIRMED. The `probe` comment explicitly notes "double-DNS accepted, spec §3.2.4". Layer 1 resolves in `ValidateExternalURL`; Layer 2 resolves again in `safeDialer`. The deliberate redundancy is intact. |

TCP monitors dial `host:port` directly without URL validation — this is unchanged
legacy behaviour, scoped to admin-configured `RemoteServer` targets built from trusted
fields, and `RFC1918` is intentionally permitted there. No regression.

### 1.2 SQL injection — raw SQL in the new query paths — PASS

| Location | Statement | Binding |
|---|---|---|
| `uptime_pruner.go:178-185` | `DELETE FROM uptime_heartbeats WHERE id IN (SELECT id … WHERE created_at < ? ORDER BY id LIMIT ?)` | `cutoff` (`time.Time`) and `pruneChunkSize` (compile-time const `5000`) are both `?` binds. No `fmt.Sprintf`/concat. |
| `uptime_pruner.go:218` | `CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created ON uptime_heartbeats (monitor_id, created_at)` | Static string literal, no interpolation. |
| `uptime_summary_service.go:172-181` (`recentBeatsSQL`) | `ROW_NUMBER() OVER (PARTITION BY monitor_id ORDER BY created_at DESC)` windowed, `WHERE created_at >= ?` … `WHERE rn <= ?` | `windowStart` and `uptimeSummaryMaxBeats` (const `60`) are `?` binds via `db.Raw(sql, windowStart, 60)`. The user's `?beats=` value never reaches SQL — it only slices the cached Go slice. |
| `uptime_summary_service.go:209-214` (`uptime24hSQL`) | grouped 24h up-ratio, `WHERE created_at >= ?` | `windowStart` `?`-bound. |
| `uptime_service.go:1272` `GetMonitorHistory` (`before` cursor) | `s.DB.Where("monitor_id = ?", id).Where("created_at < ?", before)` | GORM parameterised; `before` is a parsed `time.Time`, `id` a `?` bind. |

No dynamic SQL string construction anywhere in the new paths.

### 1.3 Authorization — 3 new endpoints — PASS

`routes.go:641-651` registers all three inside the `management` group (same
`RequireManagementAccess()` JWT/role guard as every existing `/uptime/*` route):

- `GET /uptime/monitors/summary` → `uptimeHandler.Summary`
- `GET /uptime/monitors/:id/history` → `uptimeHandler.GetHistory`
- `GET /uptime/health` → `uptimeHandler.Health`

`/uptime/health` response body is exactly `{heartbeats_dropped, checks_enqueue_dropped,
queue_depth, worker_pool_size}` — four integer back-pressure counters. No monitor URLs,
target hosts, tokens, DB errors, or internal IPs. Nil-safe when the pool/ingester
haven't started (returns 0). Acceptable to expose to management-authenticated callers.

### 1.4 Input validation — PASS

| Path | Enforcement |
|---|---|
| `SettingsHandler.UpdateSetting` (`uptime.*`) | `validateUptimeSetting` — `default_interval_seconds` ∈ [30, 86400], `worker_pool_size` ∈ [1, 200], `heartbeat_retention_days` ∈ [1, 3650]; non-integer or unknown `uptime.*` key → 400 `invalid_uptime_setting`. |
| `UptimeHandler.Create` | `0 < interval < 30` → 400 "interval must be at least 30 seconds"; `interval == 0` deferred to `CreateMonitor` write-time default; `type` bound `oneof=http tcp https`. |
| `UptimeService.UpdateMonitor` | positive sub-30 `interval` → `ErrIntervalTooLow` → 400; non-positive left for `clampInterval`; field whitelist (`max_retries`, `interval`, `enabled` only). |
| Auto-create sync paths (`SyncMonitors`, `SyncAndCheckForHost`, `SyncAndCheckForRemoteServer`) | create with `Interval: 0` → `clampInterval(0, cfg)` resolves the admin default; scheduler re-clamps every interval at scheduling time. |
| `Summary` `?beats=` | handler clamps to [1, 60]; service `clampBeats` clamps again. |
| `GetHistory` `?limit=` | non-positive/unparseable → default 60; service caps at `uptimeHistoryMaxLimit = 500`. |
| `GetHistory` `?before=` | non-empty + not RFC3339 → **400 "before must be an RFC3339 timestamp"** (not silently ignored). Empty → no cursor filter. |

### 1.5 Resource exhaustion / DoS — PASS

| Component | Control |
|---|---|
| Worker pool | `jobs` channel bounded at `uptimeQueueCapacity = 512`; `TryEnqueue` is non-blocking `select … default` → drop + `enqDropped` metric; scheduler leaves the job due and retries. No unbounded goroutines. Worker count from config, clamped 1-200. |
| Ingester | `results` channel bounded (`uptimeChannelBufferSize`); `Send` non-blocking `select … default` → `noteDropped(1)` + rate-limited warning + `DroppedCount()`. Batch discarded (and counted) after repeated flush failures so a DB fault can't stall the pipeline. |
| Pruner | 5000-row chunks; `time.Sleep` between chunks (50 ms steady / 250 ms until first clean pass); `ctx.Err()` check between chunks so it can be cut at any boundary; `wal_checkpoint(TRUNCATE)` only after ≥50k rows. Deliberately **not** in the ordered drain chain — never holds the single write connection across a shutdown. |
| Scheduler | `uptimeSchedulerMaxEnqueuePerTick = 200` caps host + monitor enqueues per tick (`hostPass` and `monitorPass` both truncate). Cold-start / past-due rows get a `jitterDuration` (crypto/rand, deterministic `maxD/2` fallback) spread over `uptimeSchedulerBackfillWindow` — prevents a restart stampede. |
| Summary endpoint | `recentBeatsSQL` window is always `now − 24h` and the per-monitor row cap is always the const `60` — **neither is user-controllable**. `loadMonitors` capped at 500. 30 s TTL cache in front. `?beats=` only trims the cached Go slice. Cannot be widened into an unbounded scan via query params. |

### 1.6 Log injection — the two `go/log-injection` suppressions (`3c9ac3bd`) — PASS (legitimate)

| Suppression entry | Sink | Logged value | Assessment |
|---|---|---|---|
| `remote_server_handler.go:142` | `logger.Log().WithError(syncErr).WithField("remote_server_id", id).Warn(...)` in the `Update` sync goroutine | `id uint` (bound from `server.ID`, a GORM numeric PK) | True false-positive. A Go `uint` cannot carry CR/LF/control chars. CodeQL flags it only because the enclosing `Update` also calls `c.ShouldBindJSON`, tainting the whole `server` struct; two taint paths reach the one line, so the SARIF has the finding twice. |
| `uptime_service.go:1475` | `logger.Log().WithField("remote_server_id", remoteServerID).Debug(...)` in `SyncAndCheckForRemoteServer` | `remoteServerID uint` argument (from `server.ID` at the `go …(server.ID)` call site) | Same — `uint`, no injectable payload. |

Both YAML entries are well-formed: `rule_id`, `path`, `line`, `reason`, `added: 2026-08-27`,
`review_by: 2026-11-27` (not expired). Both have the primary in-source
`// codeql[go/log-injection]` annotation on the standalone line immediately above the
sink. A fresh local SARIF scan does not populate `result.suppressions` (documented
local-CLI limitation — see the pre-existing `go/cookie-secure-not-set` entry), which is
exactly what the machine-enforced ignore-list fallback exists for.

### 1.7 Secrets / data exposure — PASS

No monitor URLs, targets, tokens, or internal IPs newly logged at `info` or exposed in
an unauthenticated response. New info-level logs carry only counts
(`deleted`, `host_count`, `monitor_count`), string names, and status strings. The
pruner/ingester/scheduler log drop *totals*, never payloads. `/uptime/health` is
management-gated and body is counters only (§1.3). No Gotify tokens in logs, test
artifacts, screenshots, API examples, or URL query strings.

### 1.8 GORM — PASS

`./scripts/scan-gorm-security.sh --check`: **0 CRITICAL, 0 HIGH, 0 MEDIUM**, 2 pre-existing
INFO suggestions on `models/user.go` (missing FK indexes — not in this branch's scope).
No new raw-string query building. The ingester's coalesced `UPDATE` is
`tx.Model(&models.UptimeMonitor{}).Where("id = ?", id).Updates(map[string]any{...})` —
scoped by primary key, values passed as a bind map, one row per monitor.

---

## 2. Definition of Done — verification (re-run, real numbers)

| Gate | Command | Result |
|---|---|---|
| Backend build | `cd backend && go build ./...` | **PASS** (exit 0) |
| Frontend build | `cd frontend && npm run build` | **PASS** (built in 2.23 s, exit 0) |
| Frontend type-check | `cd frontend && npm run type-check` | **PASS** (`tsc --noEmit`, exit 0) |
| Full backend suite | `cd backend && go test ./... -count=1` | **PASS** — every package `ok`, exit 0 |
| Flake check (`8b2277bb`) | `go test ./internal/api/handlers -run TestRemoteServerHandler -count=1` ×5 | **PASS 5/5** — `TestRemoteServerHandler_Update_SyncsLinkedMonitor` stable |
| Backend coverage | `CHARON_MIN_COVERAGE=85 bash scripts/go-test-coverage.sh` | **PASS** — statement 92.0 %, **line 88.7 %** (≥ 85 %) |
| Frontend coverage | `bash scripts/frontend-test-coverage.sh` | **PASS** — **lines 90.86 %** (statements 89.66 %, gate min 87 %) |
| Patch coverage | `bash scripts/local-patch-report.sh` | **PASS** — Overall **96.0 %**, Backend 96.4 %, Frontend 90.8 % (≥ 90 % overall). Artifacts: `test-results/local-patch-report.{md,json}` |
| GORM security scan | `./scripts/scan-gorm-security.sh --check` | **PASS** — 0 critical / 0 high / 0 medium |
| CodeQL Go | `skill-runner.sh security-scan-codeql` + `codeql-findings-gate.sh … go` | **PASS** — 0 errors / 0 warnings; 4 SARIF results, **all 4 suppressed, 0 blocking** |
| CodeQL JS | same run | **PASS** — 0 findings |
| Trivy | `skill-runner.sh security-scan-trivy` + targeted `trivy fs` on canonical manifests | **PASS for this branch** — see §2.1 |
| Targeted Playwright (firefox) | `npx playwright test tests/monitoring/uptime-monitoring-scale.spec.ts tests/monitoring/uptime-monitoring.spec.ts tests/a11y/uptime.a11y.spec.ts --project=firefox` | **PASS** — 29/29 passed (37.3 s) |

CodeQL toolchain: `codeql 2.26.4` (≥ 2.26.0 required for this repo's query-pack pins) —
no `install-codeql.sh` needed.

### 2.1 Trivy detail

**This branch introduces zero HIGH/CRITICAL CVEs.** It touches no `go.mod`/`go.sum`,
no `package.json`/`package-lock.json`, no `Dockerfile`, no CI workflow
(`git diff --stat development...HEAD` on all of those is empty).

Canonical working-tree manifests scan clean:

| Target | HIGH/CRITICAL |
|---|---|
| `backend/go.mod` | 0 |
| `frontend/package-lock.json` | 0 |
| `package-lock.json` (root) | 0 |
| `agent/go.mod` | 0 |

The `security-scan-trivy` skill exits non-zero, entirely due to two artifacts that are
**not part of this branch**:

1. **Stale git worktrees in the working directory** — `.claude/worktrees/fix-banner-image/`
   and `.claude/worktrees/fix-renovate-gin-lookup/` carry *older* lockfiles
   (`axios 1.17.0` → `GHSA-gcfj-64vw-6mp9`, `react-router 7.17.0`, `form-data 4.0.5`,
   `golang.org/x/net v0.55.0`, `golang.org/x/text v0.37.0`). The current branch's own
   `frontend/package-lock.json` is already past these (scans 0). CLAUDE.md forbids
   worktrees; these are leftover local cruft and a clean CI checkout will not see them.
   **Recommendation (local hygiene, non-blocking):** `git worktree prune` / remove
   `.claude/worktrees/`.
2. **`backend/internal/api/routes/keys/hecate-ca.key`** — pre-existing EC dev/test CA
   fixture, already listed in `.trivyignore`, assessed in prior QA audits (2026-05-18,
   2026-06-03). Not committed to git history (`*.key` gitignored). Unchanged by this
   branch.

---

## 3. Nice-to-have triage (supervisor's 6)

| # | Finding | Disposition |
|---|---|---|
| **NI-2** | Shutdown-grace arithmetic: 25 s drain ctx vs. a theoretical `hardCap (20s) + notifyTimeout (10s)` on the `workerWG` path after C1. | **FIXED** (`b59115ab`) — comment at `main.go` `drainCtx` now explains why 25 s is sufficient: `appCancel()` runs first, so the probe ctx and the C1 dispatch ctx are born already-cancelled and unwind immediately; the only real bound left is the HTTP client's own 20 s timeout, which fits inside 25 s. No behaviour change; the analysis matches the supervisor's. |
| **NI-3** | `TestUptimeSummary_PerfBudget` seed is light (60 k rows) and the header comment's production-profile math is wrong ("~360 k"). | **FIXED** (`b59115ab`) — comment corrected: at the 30 s interval floor a 24 h window is 500 × 2880 ≈ **1.44 M** rows; the lean 60 k seed is stated plainly as a coarse guard that only catches a per-monitor-loop regression, not index-not-used / O(n²) slippage at scale. Seed **not** ballooned — 1.44 M rows would make the unit test multi-minute for little gain, and the p95 gate is already downgraded to "QA timing output, not hard-gated" (plan re-review S5). |
| **NI-4** | Stale "transient duplicate … C5 collapses the legacy path" comment at `uptime_check.go:56-59` (and the "mirrors checkMonitor's switch exactly" phrasing). | **FIXED** (`b59115ab`) — reworded to describe `probe()` as the sole probe switch; verified `UptimeService.checkMonitor` now calls `s.checker.probe` directly and carries no switch of its own. |
| **NI-1** | `last_notified_down` is carried in `monDebounce` (worker sets it on a down transition) and read back by `loadMonState`, but `flush()` never persists it for monitors. | **DEFERRED.** Not a regression — the column was unused pre-PR, and no monitor-level code consumes it today (only the host-level `host.LastNotifiedDown` re-notify damper is live). The in-memory field resets to a never-written value on every restart, which is cosmetically confusing but functionally inert. The two real fixes — (a) delete the vestigial field from `monDebounce`/`loadMonState`, or (b) thread it onto `CheckResult` and persist it — differ by whether a monitor-level re-notify damper is wanted, which is a product decision outside this QA pass. Low risk either way; safe to leave for a follow-up. |
| **NI-5** | `hostPass`/`monitorPass` only advance the schedule entry for IDs returned by the snapshot load; a row deleted between the due-scan and the snapshot load is never advanced and is re-selected every tick until the next `rescan()` (≤ 30 s). | **DEFERRED.** Harmless, self-healing churn — the stale ID produces at most a handful of wasted enqueues over ≤ 30 s, and enqueuing a job for a since-deleted monitor is itself a no-op downstream. The fix (advance-or-drop any due ID absent from the snapshot) adds branching to the scheduler's hot per-tick loop for a benefit that `rescan()` already delivers within one cycle. Not worth the change risk in a final pass. |
| **NI-6** | `dispatch()` wraps the non-blocking `NotifyMonitorDown` (map insert + `AfterFunc`) in the same goroutine + deadline-select harness as the blocking `NotifyMonitorUp` external send. | **DEFERRED.** Cosmetic — cost is one short-lived goroutine per down transition, bounded by worker count. Calling the down path inline would micro-optimise it but risks the C1 guarantee that *no* notification call can wedge a worker or `workerWG.Wait()`; keeping both paths on the identical bounded harness is the safer invariant. Leave as-is. |

---

## 4. Scope notes / observations (non-blocking)

- **Legacy `checkHost` / `markHostMonitorsDown` / `checkAllHosts` in `uptime_service.go`**
  remain as the no-pool inline fallback. `markHostMonitorsDown` still does direct
  `s.DB.Save` / `s.DB.Create` (GORM-parameterised, no injection risk). If a later pass
  confirms these are unreachable in production (pool always wired), they're dead-code
  removal candidates — out of scope here, and the supervisor already confirmed the
  *new* files carry no dead code.
- **Stale `.claude/worktrees/` directories** — see §2.1; recommend pruning locally.

---

## 5. Commits made during this QA pass

| SHA | Message | Gate re-run |
|---|---|---|
| `b59115ab` | `chore(uptime): correct stale comments from QA triage` (NI-2 / NI-3 / NI-4, comment-only) | `go build ./...`, `go vet`, `go test ./internal/services ./cmd/api -count=1`, `make lint-staticcheck-only` (0 issues), `lefthook run pre-commit` (semgrep 0, golangci-lint-fast 0), `local-patch-report.sh` (96.0 % overall) — all green |

---

## 6. Definition-of-Done checklist

- [x] Targeted Playwright E2E (firefox) — 29/29 green
- [x] GORM security scan — 0 critical/high
- [x] Local patch coverage preflight — artifacts present, 96.0 % overall (≥ 90 %)
- [x] CodeQL Go + JS — 0 high/critical; 3 suppressed `go/log-injection` (2 YAML entries) are the only new suppressed Go findings, both legitimate `uint` false-positives
- [x] Trivy — 0 high/critical introduced by this branch
- [x] Lefthook pre-commit — clean on the QA commit
- [x] Staticcheck — 0 issues
- [x] Backend coverage — line 88.7 % (≥ 85 %)
- [x] Frontend coverage — lines 90.86 % (≥ 87 %)
- [x] Frontend type-check — clean
- [x] Backend + frontend build — clean
- [x] Full `go test ./... -count=1` — green, flake stabilised

**Verdict: PASS — no blocking issues. Cleared to merge.**

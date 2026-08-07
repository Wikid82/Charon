# Fix: Flaky `TestProxyHostCreate_TriggersAsyncUptimeSyncWhenServiceConfigured` (SQLite Lock Contention in Test Setup)

Status: Implemented and supervisor-approved. All three commits landed on
`development`:
- `3c81849d` `fix: apply busy-timeout/WAL pragma to uptime test DB to resolve SQLite lock flake`
- `96ee480a` `chore: apply shared OpenTestDB helper to remaining ad hoc SQLite setups in proxy_host_handler_test.go`
- `f1bcb3a4` `fix: retry transient SQLite lock errors when creating uptime monitors`

A follow-up race condition discovered while building the concurrent-load
regression test (`UptimeService.ensureUptimeHost`'s unguarded
check-then-act SELECT-then-INSERT, `uptime_service.go:367-384`, not
protected by the per-proxy-host-ID mutex when two proxy hosts share a
`forward_host`) was intentionally left out of scope for this fix and filed
separately: https://github.com/Wikid82/Charon/issues/1221.

Date: 2026-08-05
Scope: Backend change. Files: `backend/internal/api/handlers/proxy_host_handler_test.go`,
`backend/internal/services/uptime_service.go`,
`backend/internal/services/uptime_service_race_test.go`. No DB schema, no
API contract changes (Commit 3 adds an internal retry loop only, no
externally visible behavior change).

---

## Verdict (skim summary)

**The dependency bump in commit `7dfd6ffc` ("fix: update opentelemetry http
instrumentation to v0.70.0") is NOT implicated.** It is a red herring that
coincided with, but did not cause, the failure.

- The bump only touches `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`
  (v0.69.0 → v0.70.0) and its transitive `go.opentelemetry.io/otel/sdk` /
  `otel/sdk/metric` (v1.44.0 → v1.45.0) — all marked `// indirect`, none
  imported by any code on the request path exercised by this test.
- Empirically: 10/10 full-package runs pass identically at both the
  post-bump (HEAD) and pre-bump (`b20b9dd0`) dependency states — same pass
  rate, same ~78–88s timing envelope, zero failures in either state. See
  "Empirical Verification" below for exact commands and output.
- The real cause is a **pre-existing test-setup bug**: the specific test
  helper used by this test (`setupTestRouterWithUptime` in
  `proxy_host_handler_test.go`) opens its own ad hoc SQLite connection that
  omits the `_busy_timeout` and `_journal_mode=WAL` DSN parameters that
  every other test file in the same package already applies via the shared
  `OpenTestDB(t)` helper in `internal/api/handlers/testdb.go`. Under enough
  concurrent scheduling pressure (many `t.Parallel()` tests competing for
  goroutine time in a full package run), the request-handling goroutine and
  the async `SyncAndCheckForHost` goroutine it spawns can both touch the
  shared-cache in-memory DB at a moment that trips SQLite's
  `SQLITE_LOCKED_SHAREDCACHE` path — and because no busy-timeout is
  configured on this DSN, the write fails immediately instead of retrying,
  which is exactly the "database table is locked: uptime_monitors" log line
  observed. This is outcome **(a)** from the investigation brief: a
  pre-existing SQLite contention/timing flake, not a genuine bug exposed by
  the dependency bump.

---

## 1. Introduction

### 1.1 Background

Jeremy ran `bash scripts/dep_update.sh` on `development`. The Go dependency
bump step succeeded and `go test ./...` reported one failure:

```
--- FAIL: TestProxyHostCreate_TriggersAsyncUptimeSyncWhenServiceConfigured (3.03s)
    proxy_host_handler_test.go:255: Error Trace: proxy_host_handler_test.go:255
    Error: Condition never satisfied
FAIL    github.com/Wikid82/charon/backend/internal/api/handlers 84.768s
```

with this log line immediately preceding the failure:

```
2026/08/05 03:47:14 backend/internal/services/uptime_service.go:1375 database table is locked: uptime_monitors
[0.369ms] [rows:0] INSERT INTO `uptime_monitors` (...) VALUES (...)
```

Jeremy suspected commit `7dfd6ffc4687d7263103dd3757835329f8ef68bc` (the
otelhttp v0.70.0 bump, landed immediately before this run) as the cause.

### 1.2 Objective

1. Trace the async uptime-sync flow end-to-end (entry point → transformation
   → persistence → exit) per CLAUDE.md's Root Cause Analysis Protocol.
2. Empirically determine whether the dependency bump is implicated.
3. Identify the true root cause and specify a narrowly-scoped fix.

---

## 2. Research Findings

### 2.1 End-to-end trace of the async uptime-sync flow

**Entry point** — `backend/internal/api/handlers/proxy_host_handler.go`,
`(h *ProxyHostHandler) Create` (line 396):

- Validates/resolves request payload (lines 397–465), assigns UUIDs (467–472).
- Line 474: `h.service.Create(&host)` — **synchronous**, this is the primary
  write of the request (see 2.2 below). By the time this returns, the
  `proxy_hosts` INSERT transaction has already committed.
- Lines 479–490: optional synchronous `caddyManager.ApplyConfig` (nil in
  this test).
- Lines 493–504: optional synchronous notification send.
- **Lines 506–509 (the async trigger)**:
  ```go
  // Trigger immediate uptime monitor creation + health check (non-blocking)
  if h.uptimeService != nil {
      go h.uptimeService.SyncAndCheckForHost(host.ID)
  }
  ```
  This spawns a goroutine and does **not** wait for it. The handler then
  proceeds to build warnings and write the HTTP response (lines 511–520),
  racing with the goroutine by design.

**Transformation / persistence** —
`backend/internal/services/uptime_service.go`,
`(s *UptimeService) SyncAndCheckForHost` (line 1309):

- Line 1312: reads `settings` table for `feature.uptime.enabled` (read).
- Lines 1318–1329: acquires a per-host `sync.Mutex` (in-process only; does
  not touch the DB, and does not protect against concurrent *other* tests'
  connections — each test has its own DB instance, so this is not a
  cross-test contention vector).
- Line 1334: `s.DB.Where("id = ?", hostID).First(&host)` (read).
- Line 1342: `s.DB.Where("proxy_host_id = ?", host.ID).First(&monitor)` (read).
- Lines 1362–1374: builds a new `models.UptimeMonitor{}` in memory.
- **Line 1375** (exact line matching the failure log — confirms no
  intervening code changes shifted this line number, i.e. this file is
  unmodified by the otel bump):
  ```go
  if createErr := s.DB.Create(&monitor).Error; createErr != nil {
      logger.Log().WithError(createErr).WithField("host_id", host.ID).Error("SyncAndCheckForHost: failed to create monitor")
      return
  }
  ```
  This is the write that fails with `database table is locked:
  uptime_monitors`. Because the error is only logged (not retried or
  surfaced), the calling goroutine silently returns, and the row the test
  is polling for never appears — hence `require.Eventually` times out at
  `proxy_host_handler_test.go:255`.
- Line 1385: `s.checkMonitor(monitor)` (further DB activity) is never
  reached in the failure case.

**Same DB connection/session as production code**: `s.DB` is the
`*gorm.DB` injected via `NewUptimeService(db, ns)` (line 71) — in this
test, the same `*gorm.DB` handle returned by `setupTestRouterWithUptime`
(see 2.3). There is no separate connection pool for the uptime service;
it shares whatever pool the test wired up.

**Exit point**: the HTTP response was already written by the main request
goroutine (`c.JSON(http.StatusCreated, host)`, line 520) before the async
goroutine's write even had a chance to run — this is intentional
fire-and-forget design, and the test's `require.Eventually` polling loop
(lines 255–258) exists specifically to accommodate that.

### 2.2 Synchronous DB work in the same request

`backend/internal/services/proxyhost_service.go`,
`(s *ProxyHostService) Create` (line 180):

```go
func (s *ProxyHostService) Create(host *models.ProxyHost) error {
	if err := s.ValidateUniqueDomain(host.DomainNames, 0); err != nil {
		return err
	}
	if err := s.validateProxyHost(host); err != nil {
		return err
	}
	...
	if err := s.db.Create(host).Error; err != nil {
		return err
	}
	s.invalidateCertCache()
	return nil
}
```

This is a plain `gorm.DB.Create` call. Because
`setupTestRouterWithUptime` opens the DB with a bare `&gorm.Config{}`
(`SkipDefaultTransaction` defaults to `false`), GORM wraps this single-row
insert in an implicit `BEGIN`/`COMMIT` transaction. This completes and
commits **before** line 508's `go h.uptimeService.SyncAndCheckForHost(...)`
is even reached — so within a single test's own request/response cycle,
there's no *direct* overlap between this transaction and the async
monitor-creation write. The contention instead comes from connection-pool
behavior explained in 2.3–2.4: because the pool isn't capped, Go's
`database/sql` can hand out a second physical connection to the async
goroutine that contends with whatever the main goroutine (or another
`t.Parallel()` test's goroutine scheduled onto the same process) is doing
against the same shared-cache DB name at that moment.

### 2.3 Test DB setup — the actual defect

`backend/internal/api/handlers/proxy_host_handler_test.go`,
`setupTestRouterWithUptime` (lines 72–97), used **only** by the failing
test:

```go
func setupTestRouterWithUptime(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
		&models.Notification{},
		&models.NotificationProvider{},
		&models.UptimeMonitor{},
		&models.UptimeHeartbeat{},
		&models.UptimeHost{},
		&models.Setting{},
	))
	...
}
```

Note the DSN: `file:<TestName>?mode=memory&cache=shared` — **no
`_busy_timeout`, no `_journal_mode=WAL`**, and no
`sqlDB.SetMaxOpenConns(1)` call on the resulting connection pool. This
means Go's default `database/sql` pool sizing applies (effectively
unbounded `MaxOpenConns`), so concurrent goroutines against this DB name
can be served by multiple physical SQLite connections into the same
shared cache. In `cache=shared` mode, SQLite enforces table-level locking
*across connections that share the cache*, and without a configured
busy-timeout, a lock conflict returns `SQLITE_LOCKED` immediately instead
of retrying — this is precisely the "database table is locked: X" error
text (distinct from the file-level "database is locked" /
`SQLITE_BUSY` message), confirming the shared-cache, no-timeout mechanism.

**This exact class of flake was already fixed once in this package** — see
`backend/internal/api/handlers/testdb.go`, `OpenTestDB` (lines 73–97):

```go
// Opens a SQLite in-memory DB unique per test and applies
// a busy timeout and WAL journal mode to reduce SQLITE locking during parallel tests.
func OpenTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsnName := strings.ReplaceAll(t.Name(), "/", "_")
	n, _ := crand.Int(crand.Reader, big.NewInt(10000))
	uniqueSuffix := fmt.Sprintf("%d%d", time.Now().UnixNano(), n.Int64())
	dsn := fmt.Sprintf("file:%s_%s?mode=memory&cache=shared&_journal_mode=WAL&_busy_timeout=5000", dsnName, uniqueSuffix)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	...
}
```

A grep across every `*_test.go` file in `internal/api/handlers/` shows
**every other test file in the package already calls `OpenTestDB(t)`**
(over 140 call sites across `crowdsec_*_test.go`, `user_handler_test.go`,
`stats_*_test.go`, `security_*_test.go`, `plugin_handler_test.go`,
`notification_provider_handler_test.go`, `uptime_handler_test.go`,
`handlers_test.go`, etc.). `proxy_host_handler_test.go` is the **sole
outlier**: it never adopted `OpenTestDB` and instead hand-rolls
`gorm.Open(sqlite.Open(dsn), &gorm.Config{})` at multiple call sites
(lines 30, 52, 76, 326, 374, 654, 1889, 2385, 2757, 2816). Of these, only
`setupTestRouterWithUptime` (line 76, backing `setupTestRouter` at line 26
and `setupTestRouterWithReferenceTables` at line 48 share the identical
pattern but are not implicated in this specific failure) drives a test
that spawns a concurrent-write goroutine against the DB it opens — which
is why this is the one that manifests as a lock-contention flake and the
others have not (yet) been observed to.

### 2.4 Production DB setup, for contrast

`backend/internal/database/database.go`, `Connect` (lines 51–107) and
`configurePool` (lines 138–147):

```go
pragmas := []string{
	"PRAGMA journal_mode=WAL",
	"PRAGMA busy_timeout=5000",
	"PRAGMA synchronous=NORMAL",
	"PRAGMA cache_size=-64000",
}
...
func configurePool(sqlDB *sql.DB) {
	sqlDB.SetMaxOpenConns(1)    // SQLite only allows one writer at a time
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
}
```

Production is well protected: WAL mode, a 5s busy-timeout, and a
single-connection pool (belt-and-suspenders — with `MaxOpenConns(1)`
there's only ever one physical connection, so lock contention between
goroutines is serialized through Go's own pool-checkout queue rather than
hitting SQLite's locking at all). `OpenTestDB` in the test package
replicates the busy-timeout/WAL half of this (via DSN params, since the
pure-Go `glebarez/sqlite` driver used in production needs pragmas applied
post-connect, while `gorm.io/driver/sqlite`, the CGO `mattn/go-sqlite3`
driver used in these specific tests, accepts them as DSN query params) but
was simply never wired into `proxy_host_handler_test.go`.

### 2.5 `t.Parallel()` usage

`grep -c 't.Parallel()' proxy_host_handler_test.go` → 70 occurrences. The
failing test itself calls `t.Parallel()` (line 233), as do effectively all
other tests in the file and package. Each test opens its own uniquely
named in-memory DB (`t.Name()`-scoped for `setupTestRouterWithUptime`;
`t.Name()`+random-suffix–scoped for `OpenTestDB`), so **different tests do
not share a SQLite shared-cache namespace** — cross-test contention is not
the mechanism. The relevant concurrency is intra-test: the main handler
goroutine and the `go h.uptimeService.SyncAndCheckForHost(...)` goroutine
it spawns, contending for connections in an unbounded pool while dozens of
sibling `t.Parallel()` tests compete for the same 4 CPUs (`nproc` = 4 in
this environment), which increases the odds of the race window being hit.
This explains why the flake reproduces in full-package runs under load but
not in isolated single-test runs (see 3.2).

---

## 3. Empirical Verification

All commands run for real from `/projects/Charon/backend` on branch
`development`. Working tree was clean before and after (verified with
`git status`/`git diff` — see 3.4).

### 3.1 Isolated single-test run at HEAD (post-bump), ×10

```
$ go test ./internal/api/handlers/... -run TestProxyHostCreate_TriggersAsyncUptimeSyncWhenServiceConfigured -count=10 -v
```
Result: **10/10 PASS**, `ok github.com/Wikid82/charon/backend/internal/api/handlers 0.798s`.

### 3.2 Full package run at HEAD (post-bump), ×5

```
$ go test ./internal/api/handlers/... -count=1   # repeated 5 times
```
Result: **5/5 PASS** — `77.643s`, `78.268s`, `81.910s`, `82.039s`, `87.986s`.

### 3.3 Pre-bump dependency state (`b20b9dd0`), same DSN params restored

```
$ git checkout b20b9dd0 -- backend/go.mod backend/go.sum
$ cd backend && go build ./...          # BUILD OK
$ grep otelhttp go.mod                  # confirms v0.69.0 (pre-bump) restored
```

Isolated single-test run ×10:
```
$ go test ./internal/api/handlers/... -run TestProxyHostCreate_TriggersAsyncUptimeSyncWhenServiceConfigured -count=10 -v
```
Result: **10/10 PASS**, `ok ... 0.632s`.

Full package run ×5:
```
$ go test ./internal/api/handlers/... -count=1   # repeated 5 times
```
Result: **5/5 PASS** — `77.853s`, `78.693s`, `78.533s`, `84.491s`, `77.876s`.

### 3.4 Restoration

```
$ git checkout HEAD -- backend/go.mod backend/go.sum
$ git status --short   # (clean)
$ git diff --stat       # (empty)
```
Working tree confirmed identical to the state before investigation began.

### 3.5 Interpretation

| Dependency state | Isolated targeted-test executions | Full-package invocations |
|---|---|---|
| Post-bump (HEAD, `7dfd6ffc`) | 10/10 pass (one `-count=10` invocation) | 5/5 invocations pass, 77.6–88.0s each |
| Pre-bump (`b20b9dd0`) | 10/10 pass (one `-count=10` invocation) | 5/5 invocations pass, 77.9–84.5s each |

Corrected tally (an earlier draft of this section miscounted): this is **20
targeted-test executions total** (10 per dependency state, via two
`-count=10` invocations) and **10 full-package invocations total** (5 per
dependency state) — 30 individual `go test` runs altogether, not "20
full-package runs." Identical pass rates and near-identical timing
envelopes in both dependency states; none reproduced the original failure.

**On confidence and sample size — be honest about what the numbers can and
can't show.** 10 full-package invocations per dependency state is not a
large enough sample to bound the true failure rate of a race that
apparently occurs on the order of "once across many CI/local runs." A
naive read of "5/5 and 5/5, so it's fine" would overstate what this data
alone proves — a rate-limited draw of 5 is easily consistent with a true
failure probability anywhere from under 1% to several percent, and 5/5
clean in both arms is the expected outcome under the null hypothesis
("the bump changed nothing") *and* under many non-null hypotheses too. The
run counts are corroborating, not load-bearing. **What actually carries
"high confidence" here is the structural evidence from §2**, specifically:
- The entire diff of `7dfd6ffc` touches only `go.mod`/`go.sum` for
  `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`
  (v0.69.0→v0.70.0, `// indirect`) and its transitive `go.opentelemetry.io/otel/sdk`
  / `otel/sdk/metric` (v1.44.0→v1.45.0, also `// indirect`).
- `grep -rn "otelhttp\|go.opentelemetry.io/otel/sdk" internal/ cmd/ --include=*.go`
  (excluding `_test.go` files) returns **zero matches** — confirmed by
  running this exact command during this investigation. Nothing in the
  production code that this test exercises (the `ProxyHostHandler.Create`
  → `ProxyHostService.Create` → `UptimeService.SyncAndCheckForHost` →
  GORM/SQLite path) imports or is instrumented by either package, directly
  or transitively through any code we control.
- The failing line, `uptime_service.go:1375`, is unchanged by the bump and
  matched exactly between the failure report and the current HEAD, ruling
  out any line-shift/behavioral edit in that file.

The A/B run counts are consistent with (do not contradict) this structural
conclusion, and are reported for transparency, but the structural argument
— not the sample size — is what justifies ruling out the dependency bump
with high confidence.

**Why `-race` was not used in the A/B comparison itself**: the observed
failure is a SQLite error return (`SQLITE_LOCKED_SHAREDCACHE`, surfaced as
the Go error `"database table is locked: uptime_monitors"`) — an
application-level error path, not a Go-level data race on unsynchronized
shared memory. `go test -race` instruments memory accesses to detect
happens-before violations on Go values; it has no visibility into a
SQLite driver's internal lock-conflict return codes, so it would not be
expected to catch or help reproduce this failure mode, and omitting it
from §3.1–§3.3 was a deliberate choice, not an oversight. `-race` **is**
still used in the Commit 1 validation gate (§7) for the *new* concurrent
regression test in §5.4, where it serves its normal purpose: catching any
accidental unsynchronized access introduced by the test's own goroutine
usage (e.g., a shared slice written from multiple goroutines without
per-index isolation) — a different, legitimate concern from reproducing
the SQLite lock error.

The bug is a latent, low-probability defect in `proxy_host_handler_test.go`'s
test setup that predates `7dfd6ffc` and happened to surface on Jeremy's
run for unrelated scheduling/timing reasons (system load at the time,
GOMAXPROCS contention from the concurrently-updated `agent` module's tests
in the same `dep_update.sh` run, etc.).

---

## 4. Root Cause (confirmed)

**Outcome (a)**: pre-existing SQLite contention/timing flake, unrelated to
the dependency bump.

`setupTestRouterWithUptime` in
`backend/internal/api/handlers/proxy_host_handler_test.go` (lines 72–97)
opens its SQLite connection with DSN `file:<TestName>?mode=memory&cache=shared`,
omitting the `_busy_timeout` and `_journal_mode=WAL` parameters that the
shared `OpenTestDB(t)` helper (`backend/internal/api/handlers/testdb.go`,
lines 73–97) already provides and that every other test file in the
package already relies on. Without a busy-timeout, a shared-cache
table-lock conflict between the request-handling goroutine's connection
and the `go h.uptimeService.SyncAndCheckForHost(host.ID)` goroutine's
connection (`backend/internal/api/handlers/proxy_host_handler.go:508`)
fails immediately with `SQLITE_LOCKED` ("database table is locked:
uptime_monitors") instead of retrying, which happens inside
`(s *UptimeService) SyncAndCheckForHost` at
`backend/internal/services/uptime_service.go:1375`. The error is logged
and swallowed (by design — this is a best-effort background sync), so the
`uptime_monitors` row the test polls for never appears, and
`require.Eventually` times out at
`backend/internal/api/handlers/proxy_host_handler_test.go:255`.

---

## 5. Proposed Fix

### 5.1 Scope

Single narrowly-scoped test-only fix. No production code changes. No
schema changes. No API contract changes.

### 5.2 Change

In `backend/internal/api/handlers/proxy_host_handler_test.go`, replace the
ad hoc `gorm.Open(sqlite.Open(dsn), &gorm.Config{})` call in
**`setupTestRouterWithUptime`** (lines 75–77) with the package's existing
`OpenTestDB(t)` helper, which already applies `_busy_timeout=5000` and
`_journal_mode=WAL` and is already used by every other test file in this
package:

```go
// Before (lines 75-77):
dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
require.NoError(t, err)

// After:
db := OpenTestDB(t)
```

The subsequent `db.AutoMigrate(...)` call (lines 78–87) and the rest of
the function are unchanged — same models migrated, same signature, same
return values. `OpenTestDB` uses the identical `mode=memory&cache=shared`
DSN pattern with a `t.Name()`-derived (plus random-suffix, for extra
collision safety) unique database name, so no other test behavior changes.

This directly removes the missing-pragma defect identified in §4 by
reusing the already-existing, already-proven fix instead of hand-rolling
DSN parameters a second time (DRY, per CLAUDE.md).

### 5.3 Full scope of the same defect in this file, and why Commit 1 fixes only one site

§2.3's grep survey found **10 total** ad hoc
`gorm.Open(sqlite.Open(dsn), &gorm.Config{})` call sites in
`proxy_host_handler_test.go` (lines 30, 52, 76, 326, 374, 654, 1889, 2385,
2757, 2816). Commit 1 (§7) fixes only line 76
(`setupTestRouterWithUptime`) — the one call site that demonstrably backs
the failing test, per the root cause in §4. The other **9 sites carry the
identical missing-pragma defect but are not fixed by Commit 1**; they are
listed here explicitly rather than silently dropped from scope:

| Line | Function / test | Notes |
|---|---|---|
| 30 | `setupTestRouter` (helper) | Used by `TestProxyHostLifecycle` and others; no async-goroutine writer involved. |
| 52 | `setupTestRouterWithReferenceTables` (helper) | Used by `TestProxyHostHandler_ResolveAccessListReference_TargetedBranches` and others; no async-goroutine writer involved. |
| 326 | `TestProxyHostDelete_WithUptimeCleanup` | **Elevated risk relative to the others**: this test also constructs `services.NewUptimeService(db, ns)` directly (mirroring the bug's dependency), and its DSN is additionally a **hardcoded literal** (`"file:test-delete-uptime?mode=memory&cache=shared"`, not `t.Name()`-derived), an extra latent collision risk beyond the missing pragmas. Confirmed (per Supervisor's review and independently verified here) that this is not currently a live bug only because the `Delete` handler path does not spawn a background goroutine the way `Create` does — `uptime_service.go`'s monitor cleanup on delete runs synchronously (`proxy_host_handler.go:755-759`). If a future change makes any part of the delete-cleanup path asynchronous, this site would be exposed to the same class of flake. |
| 374 | `TestProxyHostErrors` | Same missing-pragma pattern; no async writer currently. |
| 654 | `TestProxyHostWithCaddyIntegration` | Same missing-pragma pattern; no async writer currently. |
| 1889 | `TestUpdate_IntegrationCaddyConfig` | Same missing-pragma pattern; no async writer currently. |
| 2385 | `setupTestRouterWithProxyGroupTable` (helper) | Backs 6 downstream tests (lines 2409, 2447, 2472, 2649, 2676, 2713); no async-goroutine writer involved. |
| 2757 | `TestProxyHostHandler_BulkUpdateGroup_CaddyApplyError` | Same missing-pragma pattern; no async writer currently. |
| 2816 | `TestProxyHostHandler_SetCertificateService_InvalidatesOnCreate` | Same missing-pragma pattern; no async writer currently. |

None of these 9 are implicated in the reported failure, and 20/20
empirical targeted-test runs plus 10/10 full-package runs (§3) show no
flake attributable to them. Per the investigation brief's explicit
instruction ("propose a narrowly-scoped fix... do not restructure
unrelated code"), Commit 1 fixes only the one site that is demonstrably
defective. §7's optional Commit 2 applies the same one-line, mechanical
fix (`OpenTestDB(t)` in place of the bare `gorm.Open` call) to **all 9
remaining sites** — not just the two originally-scoped helpers — so the
plan does not read as having silently narrowed coverage after the initial
survey in §2.3. Jeremy can choose to land Commit 1 alone for minimal
footprint, or both commits for full remediation of this file's known
defect class; either choice is explicit and documented, not accidental.

### 5.4 Regression coverage

A race condition triggered by scheduling pressure cannot be deterministically
reproduced by a single sequential unit test. To give the fix meaningful
regression coverage without relying on flaky timing assertions, add a new
test in the same file that increases concurrent pressure on
`setupTestRouterWithUptime`'s DB within a single test process — directly
exercising the mechanism identified in §4 (main-goroutine + async-goroutine
contention against one shared-cache DB) at higher multiplicity than the
original test.

**Corrected mechanism** (an earlier draft of this section proposed
`t.Run()` subtests, which is wrong: plain `t.Run()` calls execute
**synchronously**, one after another, unless each subtest itself calls
`t.Parallel()` — without that, no concurrent DB writes would ever occur
and the test would pass trivially regardless of whether the fix is
applied, defeating its purpose entirely). This codebase already has an
established, unambiguous idiom for exactly this kind of test — raw
goroutines + `sync.WaitGroup`, no `t.Run()` involved — used in
`backend/internal/services/uptime_service_race_test.go`,
`TestCheckHost_HostMutexPreventsRaceCondition` (around line 345):
launch N `go func() { defer wg.Done(); ... }()` goroutines, `wg.Wait()`,
then assert final DB state from the main test goroutine (assertions are
deliberately *not* called from inside the goroutines in that test, since
calling `t.Fatal`/`require.*` — which calls `t.FailNow()` — from a
goroutine other than the test's own is documented by the Go testing
package as unsafe: `FailNow` must be called from the goroutine running
the test). The new test follows this same idiom.

**New test**: `TestProxyHostCreate_TriggersAsyncUptimeSyncWhenServiceConfigured_ConcurrentLoad`
in `backend/internal/api/handlers/proxy_host_handler_test.go` (placed
immediately after the existing test, i.e. after line 259):

```go
func TestProxyHostCreate_TriggersAsyncUptimeSyncWhenServiceConfigured_ConcurrentLoad(t *testing.T) {
	t.Parallel()

	router, db := setupTestRouterWithUptime(t)

	const n = 8
	domains := make([]string, n)
	statusCodes := make([]int, n) // each goroutine writes only its own index — no shared-write race

	var wg sync.WaitGroup
	start := make(chan struct{}) // synchronization barrier

	for i := 0; i < n; i++ {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(upstream.Close)
		domains[i] = fmt.Sprintf("concurrent-load-%d-%s", i, strings.TrimPrefix(upstream.URL, "http://"))

		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // block here until every goroutine is launched and ready

			body := fmt.Sprintf(`{"name":"Concurrent Load %d","domain_names":"%s","forward_scheme":"http","forward_host":"app-service","forward_port":8080,"enabled":true}`, i, domains[i])
			req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			statusCodes[i] = resp.Code // disjoint index write, safe without a mutex
		}(i)
	}

	close(start) // release all n goroutines at once, maximizing write overlap
	wg.Wait()

	// All assertions run on the main test goroutine, after wg.Wait() —
	// never inside the goroutines above (see rationale in plan §5.4).
	for i, domain := range domains {
		require.Equal(t, http.StatusCreated, statusCodes[i], "host %d creation response", i)

		var created models.ProxyHost
		require.NoError(t, db.Where("domain_names = ?", domain).First(&created).Error, "host %d lookup", i)

		var count int64
		require.Eventually(t, func() bool {
			db.Model(&models.UptimeMonitor{}).Where("proxy_host_id = ?", created.ID).Count(&count)
			return count > 0
		}, 3*time.Second, 50*time.Millisecond, "monitor for host %d (domain %s) never created", i, domain)
	}
}
```

Key points:
- The `start` channel is the synchronization barrier requested in review:
  every goroutine blocks on `<-start` immediately after being scheduled,
  so `close(start)` releases all `n` of them at (as close to) the same
  instant as the Go scheduler allows — this is what makes the resulting
  `ProxyHostService.Create` writes and the `n` downstream
  `go h.uptimeService.SyncAndCheckForHost(...)` goroutine writes actually
  overlap, rather than being staggered by ordinary goroutine-launch
  jitter (which, without the barrier, could let each request's write
  complete before the next one starts, never exercising the contention
  path this test exists to cover).
- `statusCodes[i]` and `domains[i]` are written by exactly one goroutine
  each (disjoint indices into a pre-sized slice) — safe without a mutex,
  and verifiable under `-race`.
- All `require.*` calls happen on the main test goroutine after
  `wg.Wait()`, matching the established idiom and avoiding the
  `FailNow`-from-wrong-goroutine hazard.

This test would have reliably failed (or at minimum, reliably logged
`"database table is locked: uptime_monitors"` and left some monitors
missing) against the pre-fix `setupTestRouterWithUptime` (bare DSN, no
busy-timeout) under the write overlap the barrier manufactures, and passes
reliably once `OpenTestDB(t)` is used. It also serves as living
documentation of the failure mode for future maintainers.

**Why the existing 3s-per-goroutine `require.Eventually` budget is
expected to hold at N=8**: the original failure log shows the
`uptime_monitors` INSERT itself completing in `0.369ms` — these are
trivial single-row writes, not slow operations. Post-fix, with
`_busy_timeout=5000` in effect, up to `n=8` writes contending for the same
shared-cache table lock serialize through SQLite's busy-wait/retry
mechanism rather than failing immediately; worst-case fully-serialized
cost for 8 sub-millisecond writes is on the order of a few milliseconds,
several orders of magnitude below the 3s budget. The 3s figure is
inherited unchanged from the original (already N=1) test, which passed
reliably in all 20 targeted-test executions in §3.1/§3.3 — N=8 adds
negligible serialized latency on top of that same budget, even accounting
for CI scheduling variance. One consequence worth flagging explicitly: the
per-goroutine timeout (3s) is intentionally kept *below* the DSN's
`_busy_timeout` ceiling (5s) so that if contention ever did approach the
timeout, the test would surface a `require.Eventually` failure rather than
silently blocking for the full 5s per writer; if this test needs
adjustment later, keep that ordering (`Eventually` budget < `busy_timeout`)
rather than closing the gap.

**Validation gate for this test**: run it standalone with `-race
-count=20` before and after the fix to confirm it fails (or logs the lock
error) pre-fix and passes cleanly post-fix with no data races reported.

---

## 6. Risks and Mitigations

| Risk | Mitigation |
|---|---|
| `OpenTestDB`'s random-suffix DSN naming changes DB identity in a way that breaks an assumption elsewhere in `setupTestRouterWithUptime`'s callers | Reviewed: the function's only consumer is the one test in scope; the returned `*gorm.DB` handle is used identically regardless of the underlying DSN string. No other code parses or depends on the DSN name. |
| New concurrent-load test is itself flaky under CI resource constraints | Use a generous `require.Eventually` timeout (mirrors existing test's 3s/50ms) and keep N modest (8) — sized to reproduce contention locally without becoming a CI timing hazard. If CI shows flakiness, reduce N or widen the timeout in review; do not skip the test. |
| Silencing GORM's logger (a side effect of switching to `OpenTestDB`, which sets `logger.Default.LogMode(logger.Silent)`) hides useful diagnostic SQL output if this test fails again in the future | Acceptable tradeoff — every other test in the package already silences this logger via `OpenTestDB`; `t.Log`/`require` failure messages remain fully diagnostic, and `-v` plus targeted `-run` reproduction (as used in this investigation) remains available. |
| Sibling call sites (all 9 listed in §5.3) still carry the latent defect if Commit 2 is skipped | Documented explicitly in §5.3 with every site named, and flagged as a known, low-priority follow-up; no test currently exercises the concurrent-write pattern needed to trigger it through any of them except line 326 (see its elevated-risk note in §5.3). |

### 6.1 Should `SyncAndCheckForHost` retry on lock errors?

Raised in review: production already configures `SetMaxOpenConns(1)` +
`journal_mode=WAL` + `busy_timeout=5000` (§2.4). Is that fully sufficient
to make `"database table is locked"` structurally impossible in
production at `uptime_service.go:1375`, or is there an edge case that
could still hit it?

**Analysis**: `SetMaxOpenConns(1)` means there is at most one physical
SQLite connection in the production pool. When two goroutines both need
that connection, Go's `database/sql` pool simply queues the second
goroutine's checkout until the first releases it — this is ordinary Go
channel/mutex contention, not a SQLite-level lock conflict, so it does not
produce `SQLITE_LOCKED`/`SQLITE_BUSY` errors at all; it only adds latency.
This closes off the exact mechanism identified in §4 (multiple *physical*
connections contending for the same shared-cache table).

One deliberate exception exists: `runQuickCheck`
(`backend/internal/database/database.go:109-136`) opens its **own**
`sql.Open` connection outside the capped pool, specifically so a
multi-minute `PRAGMA quick_check` scan doesn't block the single-connection
pool during startup (per the comment at lines 101-103). This is a second
physical connection to the same database file, running concurrently with
the main pool. However, `PRAGMA quick_check` is a read operation, and
under WAL mode readers do not block writers and writers do not block
readers by design — so this concurrent-connection exception should not,
in the normal case, produce a lock conflict on an unrelated table like
`uptime_monitors`. This has not been exhaustively proven immune (WAL's
reader/writer non-blocking guarantee applies at the connection/snapshot
level; an exhaustive audit of every `PRAGMA quick_check` interaction is
out of scope for this test-flake fix), but it's a materially different
and much lower-probability risk profile than the test-only bug this PR
fixes, and no production incident report or log evidence in this codebase
currently points to it firing.

**Why other services hedge with retry anyway**: `credential_service.go`,
`security_service.go`, and `backup_restore_safe.go` all add retry-on-lock
despite running under the exact same `MaxOpenConns(1)` + WAL +
`busy_timeout` protection described above — which is worth explaining
rather than treating as redundant belt-and-suspenders. Two residual gaps
`MaxOpenConns(1)` does not close, either of which is plausibly what
motivated those precedents: (1) **multi-process access** — `MaxOpenConns(1)`
caps connections *within this process's pool*; it does nothing to
serialize access if the same SQLite file is ever opened by a second OS
process (e.g. a one-off maintenance/migration script, a second `charon`
instance misconfigured to point at the same `data/` directory, or a
developer's `sqlite3` CLI session against a live file) — WAL's
`busy_timeout` still applies cross-process and will retry for up to 5s,
but a sufficiently slow or long-held external writer could still exhaust
that window. (2) **Long-running transactions within the same process** —
`MaxOpenConns(1)` guarantees only one connection is checked out at a time,
but if that one connection is mid-transaction on a slow, unrelated
operation (a large backup/restore, a bulk import), any other write queues
behind it; that's ordinary Go-level queuing rather than a SQLite lock
error under normal `busy_timeout` behavior, but a poorly-behaved caller
that bypasses the pool (opens its own `sql.Open`, as `runQuickCheck` does
per the exception above) reintroduces the multi-connection scenario from
inside the same process. Both gaps are speculative rather than
demonstrated — this investigation found no log or incident evidence that
either has actually fired against `uptime_monitors` — but they are a
plausible, consistent explanation for why the established convention
hedges with retry even under `MaxOpenConns(1)`, rather than treating that
setting as a complete guarantee.

**Existing convention**: this codebase does not centralize
lock-error retry through the "unused" (more precisely: used for a
*different* purpose — see below) `util.IsSQLiteLockedError`
(`internal/util/permissions.go:167-175`) helper. That helper is actually
called once, at `permissions.go:143` inside `MapSaveErrorCode`, to
classify a *failed* save into a user-facing diagnostic error code — not to
gate a retry loop — and notably its match set (`"database is locked"`,
`"sqlite_busy"`, `"database locked"`) does **not** include `"database
table is locked"`, the exact text this bug produces, so it would not even
recognize this specific error as a lock error. Instead, the codebase's
actual retry-with-backoff convention is independently duplicated inline in
at least three places — `credential_service.go:355-369`,
`security_service.go:267-291`, and `backup_restore_safe.go:643-659`
(`isSQLiteTransientRestoreError`, which explicitly documents mirroring a
fourth copy in `backup_handler.go`'s `isSQLiteTransientRehydrateError`
"as a small, intentional duplication rather than an inter-package
dependency" to avoid a `services` → `handlers` import) — each checking for
`"database is locked"`, `"database table is locked"`, and `"busy"`/`"table
is locked"` variants directly. This is a deliberate, precedented pattern
in this codebase, not an oversight to "fix" by routing through
`util.IsSQLiteLockedError`.

**Conclusion**: given (a) production's structural protection closes off
the mechanism that caused this specific bug, (b) the one remaining
exception (`runQuickCheck`) is a different, unproven, lower-probability
risk, and (c) this PR's stated purpose is fixing a test-only flake, adding
retry logic to `SyncAndCheckForHost` is treated as genuinely optional
defense-in-depth, not a required part of this fix. It is offered as
**Commit 3** in §7, scoped and reviewable independently, rather than
silently left out of the plan.

---

## 7. Commit Slicing Strategy

Decision: **single PR, ordered commits**, per CLAUDE.md ("One Feature = One
PR" / "Slice Commits, Not PRs"). This is a single bug fix; the PR merges
only when both commits (or Commit 1 alone, if Commit 2 is deferred) pass
the full Definition of Done.

### Commit 1 — Fix the flaky test's SQLite setup + add concurrent-load regression test (required)

- **Scope**: Fix the root cause identified in §4 and add coverage proving it.
- **Files**:
  - `backend/internal/api/handlers/proxy_host_handler_test.go`:
    - `setupTestRouterWithUptime` (lines 72–97): replace manual
      `gorm.Open(sqlite.Open(dsn), &gorm.Config{})` with `OpenTestDB(t)`
      per §5.2.
    - Add new test
      `TestProxyHostCreate_TriggersAsyncUptimeSyncWhenServiceConfigured_ConcurrentLoad`
      per §5.4, placed after the existing test (after line 259).
- **Dependencies**: none — this is the first commit.
- **Commit message** (Conventional Commits): `fix: apply busy-timeout/WAL pragma to uptime test DB to resolve SQLite lock flake`
- **Validation gate**:
  - `cd backend && go build ./...`
  - `go test ./internal/api/handlers/... -run 'TestProxyHostCreate_TriggersAsyncUptimeSyncWhenServiceConfigured' -count=20 -race -v` — 20/20 pass, zero races, zero lock-contention log lines.
  - `go test ./internal/api/handlers/... -count=5` — 5/5 full-package pass (baseline already established at 5/5 pre-fix in §3.2; this confirms no regression, not that it "fixes" an always-failing test, since the flake is probabilistic).
  - `make lint-fast` / `make lint-staticcheck-only` — zero errors.
  - `bash scripts/local-patch-report.sh` — patch coverage artifacts generated.
  - `scripts/go-test-coverage.sh` — ≥85% maintained (test-only addition should not lower coverage).

### Commit 2 — Harden remaining test call sites against the same defect class (optional, recommended)

- **Scope**: Apply the identical one-line fix (§5.2 pattern:
  `db := OpenTestDB(t)` in place of the manual `gorm.Open(sqlite.Open(dsn),
  &gorm.Config{})` + `require.NoError(t, err)` pair) to **all 9 remaining
  ad hoc call sites** identified in §5.3 — not just the two originally
  in-scope helpers:
  - `setupTestRouter` (line 30)
  - `setupTestRouterWithReferenceTables` (line 52)
  - `TestProxyHostDelete_WithUptimeCleanup` (line 326) — also replace its
    hardcoded literal DSN (`"file:test-delete-uptime?mode=memory&cache=shared"`)
    with `OpenTestDB(t)`'s `t.Name()`-derived unique name, closing the
    secondary collision-risk noted in §5.3.
  - `TestProxyHostErrors` (line 374)
  - `TestProxyHostWithCaddyIntegration` (line 654)
  - `TestUpdate_IntegrationCaddyConfig` (line 1889)
  - `setupTestRouterWithProxyGroupTable` (line 2385)
  - `TestProxyHostHandler_BulkUpdateGroup_CaddyApplyError` (line 2757)
  - `TestProxyHostHandler_SetCertificateService_InvalidatesOnCreate` (line 2816)

  This is intentionally mechanical and scoped to the same file/defect
  class — not a general refactor of unrelated code.
- **Files**: `backend/internal/api/handlers/proxy_host_handler_test.go` only.
- **Dependencies**: Commit 1 (keeps the diff reviewable as "the proven fix"
  followed by "the same fix applied preventively elsewhere").
- **Commit message**: `chore: apply shared OpenTestDB helper to remaining ad hoc SQLite setups in proxy_host_handler_test.go`
- **Validation gate**:
  - `go test ./internal/api/handlers/... -count=5` — 5/5 pass (every test
    reachable through these 9 sites, e.g. `TestProxyHostLifecycle`,
    `TestProxyHostCreate_ReferenceResolution_TargetedBranches`,
    `TestProxyHostDelete_WithUptimeCleanup`, and all 6 tests behind
    `setupTestRouterWithProxyGroupTable`, continues to pass unchanged).
  - `make lint-fast`.

### Commit 3 — Retry-with-backoff for `SyncAndCheckForHost`'s monitor-create write (optional, out-of-scope-by-default)

- **Scope**: See §6.1 ("Should `SyncAndCheckForHost` retry on lock
  errors?") for the full reasoning. In short: production already has structural
  protection (`SetMaxOpenConns(1)` + WAL + `busy_timeout=5000`, §2.4) that
  makes this class of error very unlikely in production, and this PR's
  purpose is fixing the *test* flake, not hardening the production
  best-effort sync path. This commit is offered as an explicit,
  independently reviewable option for defense-in-depth, **not** a
  prerequisite for Commits 1–2.
- **If adopted**: add a bounded retry loop around the
  `s.DB.Create(&monitor)` call at `backend/internal/services/uptime_service.go:1375`,
  following the existing convention in this codebase — see
  `credential_service.go:355-369` (`Delete`, 5 attempts,
  `time.Sleep(time.Duration(attempt) * 10 * time.Millisecond)` backoff) and
  `security_service.go:267-291` (`persistAuditWithRetry`, same shape).
  Both of those inline their own lock-detection string check
  (`strings.Contains(errMsg, "database is locked") ||
  strings.Contains(errMsg, "database table is locked") ||
  strings.Contains(errMsg, "busy")`) rather than calling
  `util.IsSQLiteLockedError` — see §6 for why matching that precedent
  (not centralizing through the util helper) is the correct DRY target
  here.
- **Files**: `backend/internal/services/uptime_service.go` (the retry
  logic) + `backend/internal/services/uptime_service_test.go` or
  `uptime_service_race_test.go` (a unit test forcing a transient lock
  error, e.g. via an injected/mocked `*gorm.DB` or a real contended
  in-memory DB, and asserting the retry eventually succeeds).
- **Dependencies**: none technically, but sequenced after Commits 1–2 so
  the test-flake fix (the actual reported bug) is not entangled with a
  production behavior change in the same review pass.
- **Commit message**: `fix: retry transient SQLite lock errors when creating uptime monitors`
- **Validation gate**: new unit test passes; `go test ./internal/services/... -run Uptime -race -count=10`; existing `uptime_service_test.go` suite unaffected; `make lint-fast`.

### Rollback / contingency

- Both commits are additive/test-only and independently revertible with
  `git revert` — neither touches production code, so a revert carries zero
  runtime risk.
- If Commit 1's new concurrent-load test proves flaky in CI despite local
  `-race -count=20` passes, the contingency is to widen its
  `require.Eventually` timeout or reduce goroutine count N in a follow-up
  commit — not to delete or skip it, per CLAUDE.md's testing requirements.
- If, contrary to the evidence in §3, a future run surfaces this failure
  again with the fix in place, re-open this investigation rather than
  reflexively re-blaming the most recent dependency bump — §3.5 already
  demonstrates that correlation-with-recency is not sufficient evidence
  here.

---

## 8. Ignore Files / Docker / Codecov Impact

Explicitly reviewed — **no changes needed**:

- `.gitignore`: no new files or directories introduced; change is confined
  to existing tracked `*_test.go` content.
- `.codecov.yml`: no coverage-path or flag changes needed; the new test
  lives in an already-covered package/file and should only raise, not
  lower, patch coverage.
- `.dockerignore`: no new files; test files are already excluded from
  production image builds via existing Go test-file conventions (`_test.go`
  files are never compiled into the `go build` binary).
- `Dockerfile`: no build-step or dependency changes — `go.mod`/`go.sum`
  are unaffected by this fix (the investigation's temporary go.mod/go.sum
  rollback in §3.3 was fully reverted, per §3.4).

---

## 9. Acceptance Criteria (Definition of Done)

- [ ] `setupTestRouterWithUptime` uses `OpenTestDB(t)` instead of a bare
      `gorm.Open` call (§5.2).
- [ ] New concurrent-load regression test added and passing per §5.4.
- [ ] `go test ./internal/api/handlers/... -run TestProxyHostCreate_TriggersAsyncUptimeSyncWhenServiceConfigured -count=20 -race -v` — 20/20 pass.
- [ ] `go test ./internal/api/handlers/... -count=5` — 5/5 full-package pass.
- [ ] `go build ./...` succeeds.
- [ ] `make lint-fast` / `make lint-staticcheck-only` — zero errors.
- [ ] `bash scripts/local-patch-report.sh` — artifacts generated, patch coverage acceptable.
- [ ] `scripts/go-test-coverage.sh` — ≥85% maintained.
- [ ] `lefthook run pre-commit` — zero high/critical CodeQL findings (test-only Go change; low risk, but gate is mandatory per CLAUDE.md).
- [ ] Working tree clean except the intended diff (no stray go.mod/go.sum changes left over from investigation — confirmed in §3.4).
- [ ] If Commit 2 is landed: all 9 sites listed in §5.3 updated, `go test ./internal/api/handlers/... -count=5` — 5/5 pass.
- [ ] If Commit 3 is landed: retry logic + new unit test per §7 Commit 3, validation gate passing.
- [ ] `docs/plans/current_spec.md` (this file) reflects the implemented state once Commit 1 (and optionally Commits 2–3) land.

**Explicitly N/A for this PR** (stated per review request, rather than
silently omitted from the gate list):

- [ ] **Playwright E2E** (`npx playwright test --project=firefox`) — N/A.
      CLAUDE.md's DoD item 1 states this as "MANDATORY — Run First" with
      no literal backend-only carve-out, so this is recorded here
      explicitly rather than skipped silently. Rationale: this change is
      confined to Go test-setup code in
      `backend/internal/api/handlers/proxy_host_handler_test.go` (and
      optionally `uptime_service.go`'s retry logic in Commit 3, itself
      only reachable through the same already-covered backend unit-test
      path) — it does not touch any frontend code, HTTP route, request/response
      schema, or user-facing behavior that a Playwright spec could
      observe. There is no E2E flow whose outcome this diff could
      plausibly change. If review disagrees, the fallback is to run the
      existing proxy-host-creation E2E spec (if one exists) as a
      sanity check rather than author a new one, since there is no new
      user-facing behavior to specify.
- [ ] **Trivy container scan** (`make trivy`) — N/A. Trivy scans the built
      container image's OS packages and dependency manifest for known
      CVEs. This PR changes neither `go.mod`/`go.sum` (confirmed reverted
      to HEAD in §3.4 — the investigation's temporary rollback left no
      trace) nor the `Dockerfile`/base image, so the set of scannable
      artifacts is byte-for-byte unchanged from the last Trivy run against
      `development`. Re-running it would necessarily reproduce the same
      result and adds no signal for this diff.

Both are recorded as explicit, reasoned N/As per review feedback, not
oversights, so QA sign-off has the full picture without needing to
re-derive this reasoning.

---

## 10. Handoff

Once this plan is reviewed, hand off to the `supervisor` agent for plan
review, then to `backend-dev` for implementation of Commit 1 (and Commits
2–3 if approved), following TDD: write/confirm the concurrent-load test
fails against the current `setupTestRouterWithUptime` (or demonstrate the
existing single test's fragility via `-race -count` under artificial load,
since the raw race is probabilistic), apply the `OpenTestDB(t)` fix, then
confirm green per the validation gates in §7.

# Fix: Flaky `TestMigrateCommand_Succeeds` TempDir Cleanup Race in `backend/cmd/api`

**Author:** Planning Agent
**Date:** 2026-07-01
**Branch:** development
**Scope:** Backend test-only fix, `backend/cmd/api` (+ small exported test helper in `backend/internal/database`)
**Related:** Triggered by `scripts/dep_update.sh` bumping `github.com/klauspost/cpuid/v2` (v2.3.0 → v2.4.0) and `github.com/prometheus/procfs` (v0.21.0 → v0.21.1) in `backend/go.mod`/`backend/go.sum` (already staged, NOT part of this fix's commit).

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Research Findings](#2-research-findings)
3. [Technical Specifications](#3-technical-specifications)
4. [Implementation Plan](#4-implementation-plan)
5. [Acceptance Criteria](#5-acceptance-criteria)
6. [Commit Slicing Strategy](#6-commit-slicing-strategy)

---

## 1. Introduction

### 1.1 Overview

`scripts/dep_update.sh`'s validation step (`go test ./...`) failed after bumping two **indirect** dependencies:

```
--- FAIL: TestMigrateCommand_Succeeds (0.10s)
    testing.go:1464: TempDir RemoveAll cleanup: unlinkat /tmp/TestMigrateCommand_Succeeds.../001/data: directory not empty
FAIL    github.com/Wikid82/charon/backend/cmd/api    0.635s
```

This is a **pre-existing, intermittent (racy) test bug** in `backend/cmd/api/main_test.go`, unrelated to the two dependency bumps. This plan documents the confirmed root cause and specifies an exact, minimal, root-cause fix.

### 1.2 Objectives

1. Confirm (not assume) the root cause of the `TempDir RemoveAll cleanup: ... directory not empty` failure.
2. Confirm the two staged dependency bumps (`cpuid/v2`, `procfs`) are not implicated.
3. Inventory every test in `backend/cmd/api` (and note related occurrences elsewhere) that opens a DB connection via `database.Connect` without deterministically closing/synchronizing it.
4. Specify the exact code changes needed to eliminate the race, reusing the codebase's own existing, documented convention for this exact problem (`internal/database/database_test.go`'s `TestMain`).
5. Define a single, scoped, low-risk commit and a validation plan appropriate for a backend-test-only change.

### 1.3 Non-Goals

- Does **not** touch `backend/go.mod` / `backend/go.sum` (already staged separately by the user; not part of this commit).
- Does **not** change any production runtime behavior — `backend/internal/database/database.go`'s `Connect()` behavior for the actual running server is unchanged (background integrity check remains async in production; only test binaries opt into synchronous mode).
- Does **not** fix the same *latent* pattern found in `backend/internal/api/handlers/db_health_handler_test.go` and `backend/internal/server/emergency_server_test.go` (see §2.4) — flagged as a follow-up, deliberately out of scope to keep this commit small (see §6).

---

## 2. Research Findings

### 2.1 What `database.Connect` actually does

`backend/internal/database/database.go`:

```go
// launchQuickCheck is called by Connect to run the integrity check goroutine.
// Tests override this with a synchronous version to avoid cleanup races.
var launchQuickCheck = func(dbPath string) { go runQuickCheck(dbPath) }   // line 19

func Connect(dbPath string) (*gorm.DB, error) {
    db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{...})          // line 26
    ...
    sqlDB, err := db.DB()
    configurePool(sqlDB)                                                 // MaxOpenConns=1, MaxIdleConns=1
    ...
    // pragmas: journal_mode=WAL, busy_timeout=5000, synchronous=NORMAL, cache_size=-64000
    ...
    launchQuickCheck(dbPath)   // line 76 — spawns `go runQuickCheck(dbPath)` by default
    return db, nil
}

func runQuickCheck(dbPath string) {   // line 85
    checkDB, err := sql.Open(sqlite.DriverName, dbPath)   // a SEPARATE, independent connection
    ...
    defer checkDB.Close()
    checkDB.QueryRow("PRAGMA quick_check").Scan(&quickCheckResult)
    ...
}
```

Key facts:

- `Connect()` returns a `*gorm.DB` opened in **WAL mode** with a pool capped at 1 open/1 idle connection.
- Every successful `Connect()` call **also** spawns a background goroutine (`go runQuickCheck(dbPath)`) that opens its **own independent** `*sql.DB` handle to the same file path (not sharing the gorm pool) and runs `PRAGMA quick_check` before closing that handle.
- This goroutine is **untracked** — `Connect()` does not wait for it, expose a handle to it, or provide any way for a caller to know when it has finished. It is fire-and-forget by design (comment at line 74: "on its own connection... never blocks startup or migrations").

### 2.2 The package already documents and fixes this exact race — for itself only

`backend/internal/database/database_test.go`:

```go
func TestMain(m *testing.M) {
    // Run quick_check synchronously in tests so it completes before t.TempDir
    // cleanup runs. The goroutine version creates a race: the background
    // connection may still hold WAL/SHM files open when the temp dir is removed.
    launchQuickCheck = runQuickCheck
    os.Exit(m.Run())
}
```

This confirms, **in the codebase's own words**, that the race is: the background `quick_check` connection can still be reading/writing SQLite WAL/SHM files inside a `t.TempDir()`-managed directory at the moment `t.TempDir()`'s registered cleanup (`os.RemoveAll`) fires when the test function returns, producing exactly the observed `unlinkat ... directory not empty` error (`ENOTEMPTY`, `os.RemoveAll` losing a race against a file being (re)created concurrently).

Critically, `launchQuickCheck` is an **unexported** package-level variable. `database_test.go` is a white-box (`package database`) test file, so it can assign to it directly. **`backend/cmd/api` is a different package and has no `TestMain`**, so it never gets this override — it is fully exposed to the async race the `database` package's own tests were specifically written to avoid.

### 2.3 `backend/cmd/api/main_test.go` — confirmed leak inventory

No test in this file closes the `*gorm.DB` returned by `database.Connect`, and none overrides `launchQuickCheck` (impossible from this package today — see §2.2). Exact call sites:

| Line | Test | Variable | Closed? | Notes |
|---|---|---|---|---|
| 34 | `TestResetPasswordCommand_Succeeds` | `db` | No | Used to seed a user, then a subprocess is spawned; `db` handle + its background quick-check goroutine outlive the function body |
| 82 | `TestMigrateCommand_Succeeds` | `db` | No | Opened to pre-seed an "old" DB before spawning `migrate` subprocess |
| 111 | `TestMigrateCommand_Succeeds` | `db2` | No | Reconnect after subprocess, to assert migrated tables exist — **this is the connection most likely implicated in the reported failure**, since it is the last DB handle opened in the failing test |
| 140 | `TestStartupVerification_MissingTables` | `db` | **Yes**, explicitly, at lines 155–156 (`sqlDB, _ := db.DB(); _ = sqlDB.Close()`) | Intentional close+reopen to simulate a startup scenario — leave this pair untouched |
| 158 | `TestStartupVerification_MissingTables` | `db` (reassigned) | No | The *second* connection (post-reopen) is never closed at test end |
| 205 | `TestMain_MigrateCommand_InProcess` | `db` | No | Seeds `User` table before calling `main()` in-process |
| 223 | `TestMain_MigrateCommand_InProcess` | `db2` | No | Reconnect after in-process `main()` call to assert migrated tables |
| 251 | `TestMain_ResetPasswordCommand_InProcess` | `db` | No | Used through line 278 (`db.Where("email = ?", email).First(&updated)`) after `main()` runs in-process |

`TestMain_DefaultStartupGracefulShutdown_Subprocess` (line 289) and `TestMain_DefaultStartupGracefulShutdown_InProcess` (line 339) do **not** call `database.Connect` directly in test code — they invoke `main()`, which connects internally. They are still exposed to the same async-goroutine race (any `Connect()` call anywhere in the process is affected by the package-level `launchQuickCheck` var), but carry no separate leaked handle to close. The `TestMain` fix in §3 covers them automatically since it is a process-wide override, not a per-call change.

### 2.4 Same pattern found elsewhere (out of scope for this commit — see §1.3/§6)

```
backend/internal/api/handlers/db_health_handler_test.go:122   db, err := database.Connect(dbPath)   // file-based, t.TempDir(), not closed
backend/internal/api/handlers/db_health_handler_test.go:205   db, err := database.Connect(dbPath)   // file-based, closed manually at 211-212 (intentional)
backend/internal/api/handlers/db_health_handler_test.go:218   db2, err := database.Connect(dbPath)  // file-based, not closed
backend/internal/server/emergency_server_test.go:27           db, err := database.Connect(tmpFile)  // file-based, via setupTestDB helper, not closed
```

Neither package has a `TestMain` overriding `launchQuickCheck` either. These are latent instances of the identical race; they simply have not (yet) been observed failing in this run. Not modified here — flagged as a follow-up (see §6.5).

### 2.5 Root-cause confirmation: reproduction evidence

Running the specific failing test in isolation is expected to pass most of the time locally (it is a genuine race, not a deterministic failure) — confirmed:

```
$ go test ./cmd/api/... -run TestMigrateCommand_Succeeds -count=5 -v
--- PASS: TestMigrateCommand_Succeeds (0.06s)   x5
```

Log ordering across runs shows the two "SQLite database integrity check passed" background-goroutine log lines interleaving unpredictably relative to the "SQLite database connected" lines of subsequent `Connect()` calls — direct evidence the quick-check goroutine is not synchronized with the caller's control flow. The original CI-style failure occurs under `go test ./...` (many packages/tests scheduled concurrently, higher system load), which widens the scheduling window for the background goroutine to still be running when `t.TempDir()`'s `RemoveAll` fires. This is consistent with a scheduler-timing-sensitive race, not a deterministic bug — which is exactly the class of bug `internal/database/database_test.go`'s own `TestMain` comment was written to eliminate.

### 2.6 Dependency bump relevance — confirmed NOT implicated

```
$ go mod why github.com/klauspost/cpuid/v2
github.com/Wikid82/charon/backend/cmd/api
github.com/gin-gonic/gin
github.com/gin-gonic/gin/codec/json
github.com/bytedance/sonic
github.com/bytedance/sonic/ast
github.com/bytedance/sonic/internal/native
github.com/bytedance/sonic/internal/cpu
github.com/klauspost/cpuid/v2

$ go mod why github.com/prometheus/procfs
github.com/Wikid82/charon/backend/internal/api/routes
github.com/prometheus/client_golang/prometheus
github.com/prometheus/procfs
```

- `cpuid/v2` is pulled in via `gin` → `bytedance/sonic` (gin's optional high-performance JSON codec, CPU feature detection for SIMD dispatch). No relationship to `glebarez/sqlite`, `modernc.org/sqlite`, or `gorm.io/gorm`.
- `procfs` is pulled in via `prometheus/client_golang` (Prometheus metrics collection). Same — no relationship to the SQLite/GORM dependency chain.

**Conclusion: the two staged dependency bumps are confirmed unrelated.** The `go test ./...` failure is a pre-existing, order/timing-sensitive flaky test that the dependency-bump validation step happened to surface.

---

## 3. Technical Specifications

This is a test-only change. No API contracts, database schema, or production request/response flows are affected. The "component design" here is the test-support surface added to `internal/database` plus the exact edits to `cmd/api`'s test file.

### 3.1 New exported test-support function — `backend/internal/database/database.go`

Add directly below the existing `launchQuickCheck` var declaration (after line 19):

```go
// SyncIntegrityCheckForTesting forces the background integrity check that
// Connect launches (see launchQuickCheck) to run synchronously instead of
// in a goroutine. Callers in other packages' test suites that call Connect
// against paths under t.TempDir() should invoke this once, from a TestMain,
// mirroring this package's own TestMain in database_test.go:
//
//	func TestMain(m *testing.M) {
//	    database.SyncIntegrityCheckForTesting()
//	    os.Exit(m.Run())
//	}
//
// Without this, Connect's background integrity-check connection can still be
// reading/writing the SQLite WAL/SHM files when a caller's t.TempDir()
// cleanup (os.RemoveAll) runs after the test returns, which surfaces as an
// intermittent "TempDir RemoveAll cleanup: ... directory not empty" failure.
//
// There is no restore function: Go test binaries are single-process,
// one-shot invocations (the process exits after m.Run()), so there is
// nothing to revert before exit — the same reasoning internal/database's
// own TestMain already relies on.
//
// Production code paths are unaffected: Connect's default behavior (async
// integrity check) is unchanged unless a test explicitly opts in by calling
// this function.
func SyncIntegrityCheckForTesting() {
	launchQuickCheck = runQuickCheck
}
```

**Design decision (documented per CLAUDE.md "consider edge cases"):** this small function lives in a regular (non-`_test.go`) file rather than an `export_test.go`, because Go does not compile a package's `_test.go` files into the archive that *other* packages' tests import — `cmd/api`'s test binary needs a genuinely exported symbol to reach across the package boundary. This mirrors common Go idioms for test-support hooks exposed from production packages (e.g. `httptest`), adds a single 3-line function with zero runtime cost unless called, and is never invoked from any non-test code path. Alternative considered and rejected: a build-tag-gated file (`//go:build testhelpers`) — rejected as unnecessary complexity for a 3-line function, and it would require every consumer to remember a non-default build tag, which is easy to silently get wrong (defeats the purpose).

`database_test.go`'s existing `TestMain` is **left as-is** (still assigns `launchQuickCheck = runQuickCheck` directly) rather than rewritten to call the new exported wrapper — it is same-package (white-box) code, the direct assignment is simpler, and changing working, unrelated test code would be scope creep for this fix. Both now encode the same behavior; the new function's doc comment cross-references `database_test.go` explicitly to keep the two in sync conceptually.

### 3.2 `backend/cmd/api/main_test.go` — add `TestMain`

Insert near the top of the file, after the existing `import` block (after line 15, before line 17 `func TestResetPasswordCommand_Succeeds`):

```go
func TestMain(m *testing.M) {
	// Force Connect's background integrity check to run synchronously so it
	// completes before t.TempDir() cleanup (os.RemoveAll) runs. Otherwise the
	// check's background SQLite connection can still be reading/writing
	// WAL/SHM files when a test's temp directory is removed, causing
	// intermittent "TempDir RemoveAll cleanup: ... directory not empty"
	// failures (see backend/internal/database/database_test.go's TestMain
	// for the same fix applied to that package's own tests).
	database.SyncIntegrityCheckForTesting()
	os.Exit(m.Run())
}
```

No new imports are required — `os` (line 6) and `database` (line 13) are already imported in `main_test.go`. This single, process-wide override eliminates the async race for **every** test in the `cmd/api` package, including the subprocess/in-process `main()`-invoking tests (`TestMain_DefaultStartupGracefulShutdown_*`) that call `database.Connect` indirectly through production code, since `launchQuickCheck` is a package-level var affecting all `Connect()` calls within the process — no per-call changes needed for those two tests.

**Subprocess consideration:** `TestResetPasswordCommand_Succeeds`, `TestMigrateCommand_Succeeds`, and `TestMain_DefaultStartupGracefulShutdown_Subprocess` spawn `exec.Command(os.Args[0], "-test.run=...")` to re-invoke the same compiled test binary in a child process. The child process also runs through `TestMain` before `m.Run()` executes the requested test, so the synchronous override applies identically in both the parent test process and any spawned child test process — no special-casing needed.

### 3.3 `backend/cmd/api/main_test.go` — close leaked connections (defense in depth)

Reuses the repository's existing, established pattern (already used in `backend/internal/api/handlers/credential_handler_test.go:42-44`, `logo_handler_test.go:48-50`, `custom_theme_handler_test.go:29-31`, `banner_handler_test.go:47-49`, `pr_coverage_test.go:567-569`, and others):

```go
t.Cleanup(func() {
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()
})
```

Per Go's documented `t.Cleanup` LIFO ordering, a cleanup registered *after* `tmp := t.TempDir()` runs *before* `t.TempDir()`'s own cleanup — so this pattern deterministically closes each SQLite connection pool before the temp directory is removed, for every connection opened after the `t.TempDir()` call in each test (true for all cases below). This is secondary/hygiene: it does not by itself fix the async-goroutine race (§3.1/§3.2 does), but it removes leaked GORM connection pools and matches CLAUDE.md's DRY principle (reuse existing convention rather than inventing a new one).

Exact insertion points (variable name substituted per call site):

| Insert after line(s) | Test | Variable |
|---|---|---|
| 34–37 | `TestResetPasswordCommand_Succeeds` | `db` |
| 82–85 | `TestMigrateCommand_Succeeds` | `db` |
| 111–114 | `TestMigrateCommand_Succeeds` | `db2` |
| 158–161 | `TestStartupVerification_MissingTables` | `db` (second/reopened connection only — do not touch the first connection's existing manual close at 155–156) |
| 205–208 | `TestMain_MigrateCommand_InProcess` | `db` |
| 223–226 | `TestMain_MigrateCommand_InProcess` | `db2` |
| 251–254 | `TestMain_ResetPasswordCommand_InProcess` | `db` |

Example — `TestMigrateCommand_Succeeds` (lines 81–89 today) becomes:

```go
	// Create database without security tables
	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	// Only migrate User table to simulate old database
	if err = db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("automigrate user: %v", err)
	}
```

...and the `db2` reconnect (lines 111–114 today) becomes:

```go
	// Reconnect and verify security tables were created
	db2, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("reconnect db: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db2.DB()
		_ = sqlDB.Close()
	})
```

### 3.4 New unit test — `backend/internal/database/database_test.go`

Add a small test for the new exported function to satisfy CLAUDE.md's "always create unit tests for new code coverage" and the 85% backend coverage gate:

```go
func TestSyncIntegrityCheckForTesting(t *testing.T) {
	t.Parallel()

	// Sanity: the exported wrapper assigns launchQuickCheck to the
	// synchronous runQuickCheck implementation, and Connect still succeeds
	// normally afterwards (mirrors what TestMain already does package-wide).
	SyncIntegrityCheckForTesting()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "sync-check.db")

	db, err := Connect(dbPath)
	require.NoError(t, err)
	require.NotNil(t, db)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
}
```

`TestMain` in the same file already forces `launchQuickCheck = runQuickCheck` for the whole package, so this test is intentionally a smoke/regression test proving the exported entry point itself is wired correctly and doesn't panic or change `Connect`'s success behavior — not a race-reproduction test (races are not reliably unit-testable). `filepath` and `require` are already imported in this file (verified: `path/filepath` and `github.com/stretchr/testify/require` — confirm on implementation and add if not already present at file scope).

### 3.5 Error Handling / Edge Cases Considered

- **Double-close:** `TestStartupVerification_MissingTables` already closes its first `db` handle manually at lines 155–156; the new `t.Cleanup` is added only for the *second* (reopened) handle to avoid a redundant/no-op double-close of the first.
- **Connection still in use at cleanup time:** `t.Cleanup` runs after the test function body returns, so it cannot close a connection still mid-use within the same test (e.g., `TestMain_ResetPasswordCommand_InProcess`'s `db.Where(...)` call at line 278 happens before the function returns, hence before any `t.Cleanup` fires) — safe by construction.
- **Subprocess DB contention:** Adding `t.Cleanup` does not change *when* within the test body connections close relative to spawned subprocesses (cleanup still only fires at test-function return, identical timing to today's implicit "never closed until process exit" behavior for the portion of the test before return) — no new lock-contention risk is introduced between the parent test process and spawned subprocess.
- **Production behavior:** `Connect()`'s default (async) behavior is completely unchanged; `SyncIntegrityCheckForTesting()` is opt-in and only ever called from test code.

---

## 4. Implementation Plan

### Phase 1: Playwright Tests

**Not applicable.** This is a backend-test-only fix with no frontend or user-facing API behavior change (see §5 rationale). No Playwright tests are added or modified.

### Phase 2: Backend Implementation

1. `backend/internal/database/database.go` — add `SyncIntegrityCheckForTesting()` (§3.1).
2. `backend/internal/database/database_test.go` — add `TestSyncIntegrityCheckForTesting` (§3.4).
3. `backend/cmd/api/main_test.go` — add `TestMain` (§3.2) and the 7 `t.Cleanup` blocks (§3.3).

### Phase 3: Frontend Implementation

**Not applicable.** No frontend changes.

### Phase 4: Integration and Testing

Run the flake-reproduction loop before and after the fix to build confidence (not just a single green run):

```bash
cd backend
go test ./cmd/api/... -run TestMigrateCommand_Succeeds -count=20 -v
go test ./... -count=1
```

### Phase 5: Documentation and Deployment

No `ARCHITECTURE.md` update needed — this does not change system architecture, tech stack, deployment model, or directory structure (test-only bugfix). No `docs/features.md` update needed — no user-facing capability changed. This decision is stated explicitly per the plan template's requirement to show it was considered.

---

## 5. Acceptance Criteria

- [ ] `TestMigrateCommand_Succeeds` and all other tests in `backend/cmd/api` pass reliably across at least 20 repeated runs (`-count=20`), with no `TempDir RemoveAll cleanup` failures.
- [ ] `go test ./...` (full backend suite) passes with zero failures.
- [ ] `go vet ./...` reports no issues.
- [ ] `go build ./...` succeeds.
- [ ] `govulncheck ./...` reports no new vulnerabilities.
- [ ] `./scripts/scan-gorm-security.sh --check` reports zero CRITICAL/HIGH findings (triggered because this touches `backend/internal/database` and GORM-adjacent test code, per CLAUDE.md §1.5).
- [ ] `bash scripts/local-patch-report.sh` produces `test-results/local-patch-report.md` and `test-results/local-patch-report.json` with the new/changed lines covered.
- [ ] `lefthook run pre-commit` passes with zero blocking findings (staticcheck, CodeQL Go/JS per CLAUDE.md).
- [ ] `make lint-fast` / `make lint-staticcheck-only` clean.
- [ ] `scripts/go-test-coverage.sh` — backend coverage remains ≥ 85% (`CHARON_MIN_COVERAGE`).
- [ ] No debug prints, commented-out code, or unused imports introduced.
- [ ] **Explicitly out of scope, deliberately excluded (documented per DoD, not overlooked):**
  - Playwright E2E tests (`npx playwright test --project=firefox`) — not relevant; no frontend or HTTP-facing API behavior changed by this fix.
  - `scripts/frontend-test-coverage.sh` / `cd frontend && npm run type-check` — not relevant; no frontend files touched.
  - `.gitignore`, `.dockerignore`, `.codecov.yml`, `Dockerfile` — no changes needed; no new files, artifacts, or build outputs are introduced by this fix.
  - `backend/go.mod` / `backend/go.sum` (the already-staged `cpuid`/`procfs` bumps) — confirmed unrelated (§2.6) and explicitly excluded from this commit; will be committed separately by the user.

---

## 6. Commit Slicing Strategy

### 6.1 Decision

**Single PR, single commit.** This is a small, mechanically-scoped, single-domain (backend test code only) bug fix — it does not touch backend API/model behavior, frontend, or infrastructure, so CLAUDE.md's multi-PR decomposition triggers ("touches backend + frontend + infra," "diff large enough to reduce review quality," "independently testable slices," "foundational refactor needed") do not apply. One commit is the correct grain: the `TestMain` addition, the exported test helper it depends on, and the `t.Cleanup` hygiene fixes are one indivisible logical change — splitting them would leave intermediate commits in a state where the actual race is not yet fixed.

### 6.2 Commit 1 (only commit)

- **Type/prefix:** `fix:` (per CLAUDE.md Conventional Commits and CI/CD triggers — `fix:` triggers a Docker build; this is acceptable/expected for a merged test-suite fix, though this branch is not `feature/beta-release` so verify current branch's build-trigger policy before pushing).
- **Scope:** Backend test suite only.
- **Files:**
  - `backend/internal/database/database.go` — add `SyncIntegrityCheckForTesting()`.
  - `backend/internal/database/database_test.go` — add `TestSyncIntegrityCheckForTesting`.
  - `backend/cmd/api/main_test.go` — add `TestMain`; add 7 `t.Cleanup` blocks.
- **Explicitly NOT included in this commit:** `backend/go.mod`, `backend/go.sum` (already staged separately by the user — confirmed unrelated, see §2.6). Do not `git add` these files as part of this commit.
- **Dependencies:** None — this fix is independent of the staged dependency bump and can be committed before, after, or interleaved with it without ordering constraints.
- **Validation gate before commit:** All items in §5 Acceptance Criteria.

### 6.3 Suggested commit message

```
fix(test): eliminate TempDir cleanup race in cmd/api DB tests

Connect() launches an untracked background goroutine (PRAGMA quick_check
on its own SQLite connection) that can still be touching WAL/SHM files
when t.TempDir() removes the test directory, causing an intermittent
"directory not empty" cleanup failure. internal/database's own tests
already work around this via TestMain; cmd/api had no equivalent.

Adds database.SyncIntegrityCheckForTesting() and a cmd/api TestMain that
uses it, plus t.Cleanup-based connection closing for defense in depth.
```

### 6.4 Rollback / Contingency

- **Rollback:** `git revert` the single commit — it is additive-only (a new exported function, a new test, `t.Cleanup` blocks, one new `TestMain`); reverting cannot reintroduce a build break, only restore the prior (racy but previously "working most of the time") test state.
- **Contingency if the race still reproduces after this fix:** Re-run the Phase 4 `-count=20` loop under `go test ./... -race` and under simulated load (`go test ./... -p 1` vs default parallelism) to rule out a second, distinct source of background file I/O (e.g., a WAL checkpoint triggered by GORM's connection-pool idle-timeout behavior). If reproduced, the next investigation step is `configurePool`'s `SetConnMaxLifetime(0)` combined with SQLite's own WAL auto-checkpoint background thread — but do not preemptively change `configurePool` in this commit without first confirming that is in fact still happening post-fix (Root Cause Analysis Protocol: fix the confirmed cause first, re-measure, only then investigate further if the symptom persists).

### 6.5 Follow-up (not part of this commit)

Open a follow-up issue/PR to apply the same `TestMain` + `SyncIntegrityCheckForTesting()` pattern to:
- `backend/internal/api/handlers` (covers `db_health_handler_test.go` and any other file-based `database.Connect` usage in that package)
- `backend/internal/server` (covers `emergency_server_test.go`)

These carry the identical latent race (§2.4) but have not been observed failing in this run; deferring them keeps this commit minimal and focused on the confirmed, reported failure.

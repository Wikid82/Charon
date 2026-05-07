# CI Fix — Grep Pattern Bug & Orthrus Test Cleanup

**Branch**: `fix/ci-orthrus-test-cleanup`
**PR**: Targets `development` (fixes CI failure exposed by PR #1002)
**Date**: 2026-05-25

---

> **Archived**: The previous spec for the Hecate Simplified Architecture is in
> `docs/implementation/` and PR #983 has been merged. This document supersedes
> `current_spec.md` for the active work item.

---

## 1. Introduction

### Overview

The CI Backend (Go) job failed on PR #1002 (`renovate/non-major-updates`) with `GO_TEST_STATUS` non-zero, but the FAILED TEST SUMMARY printed "No specific failures captured in output". Coverage passed at 88.6% (above the 87% threshold), so the script continued past the coverage gate and exited with the original non-zero status.

Two related defects were found during investigation:

1. **Bug (confirmed)** — `scripts/go-test-coverage.sh` line 132 uses a grep pattern that cannot match the `gotestsum --format pkgname` package-level failure format, making the diagnostic summary silent for the most common failure modes (crash, panic, data race, goroutine leak).

2. **Root cause (probable)** — The Hecate/Orthrus test suite (from the now-merged `feature/hecate` PR #983) contains tests that create live WebSocket / yamux sessions or write files with restrictive permissions inside `t.TempDir()`, without restoring those permissions before cleanup fires. This can cause the test binary to crash without emitting any `--- FAIL:` lines — exactly the symptom seen in CI.

### Objectives

- Fix the grep pattern so future failures are visibly reported in CI logs.
- Harden the orthrus and CA tests so cleanup does not panic or crash.
- Leave all existing test assertions intact; this is not a feature change.

---

## 2. Research Findings

### 2.1 What Renovate PR #1002 Actually Changed

Commit `4f10bd07` (the renovate bump) touched only:

| File | Change |
|---|---|
| `.github/workflows/golangci-lint.yml` | golangci-lint action SHA `v2.11.4` → `v2.12.1` |
| `.github/workflows/quality-checks.yml` | `benchmark-action` and `docker/build-push-action` SHAs |
| `package.json` | `i18next 26.0.8` → `26.0.9` |

None of these affect Go tests. The test failure is in code already on the `development` branch from the merged `feature/hecate` (PR #983).

### 2.2 CI Run Context

The CI ran against the merge commit `3a7ec07f` (development ⊕ renovate). Test output showed:

```
Warning: go test returned non-zero (status 1); checking coverage file presence
============================================
FAILED TEST SUMMARY:
============================================
No specific failures captured in output
============================================
```

Coverage was still generated at 88.6%, meaning most packages compiled and ran successfully. Only one or a few packages failed silently.

### 2.3 The Grep Pattern Bug (Confirmed)

**Location**: `scripts/go-test-coverage.sh`, line 132

**Current code**:
```bash
grep -E "(FAIL:|--- FAIL:)" "$TEST_OUTPUT_FILE" || echo "No specific failures captured in output"
```

**Problem**: `gotestsum --format pkgname` outputs package-level failures as:
```
FAIL	github.com/Wikid82/charon/backend/internal/orthrus	2.34s
```
This line has a **tab separator** and **no colon** after `FAIL`. The pattern `FAIL:` does not match. The pattern `--- FAIL:` only matches lines from individual test assertion failures — not from crashes, panics, data races, or goroutine-induced binary exits. When the failure has no individual test assertion (e.g., a `t.TempDir()` panic during cleanup, a goroutine crash, or a race-detector abort), the grep finds nothing and prints "No specific failures captured in output".

**Evidence**: The workflow's own `Go Test Summary` step in `.github/workflows/quality-checks.yml` correctly uses:
```bash
grep -E "^--- FAIL|FAIL\s+github" test-output.txt || echo "See logs for details"
```
The script diagnostic should use the same category of logic.

**Why `--- FAIL:` still misses failures**: When a test binary crashes from a goroutine panic or a `t.TempDir()` cleanup failure, the output is:
```
panic: testing: TempDir RemoveAll cleanup: remove /tmp/...: ...
goroutine 1 [running]:
testing.(*T).TempDir.func1()
	...
FAIL	github.com/.../pkg	0.456s
```
There is no `--- FAIL: TestXxx` line in this output. The grep finds nothing.

### 2.4 Orthrus Test Cleanup Issues

The `feature/hecate` merge introduced the `backend/internal/orthrus/` package and new tests in `backend/internal/api/handlers/`. Investigation found three categories of test hygiene gaps.

#### 2.4.1 `ca_test.go` — Restrictive Permission Without Cleanup

`TestNewInternalCA_ReadOnlyKeysDir` (`backend/internal/orthrus/ca_test.go`, ~line 162) creates a `keys/` subdirectory with mode `0o555` (no write) but never registers a cleanup to restore it:

```go
func TestNewInternalCA_ReadOnlyKeysDir(t *testing.T) {
    dir := t.TempDir()
    keysDir := filepath.Join(dir, "keys")
    require.NoError(t, os.MkdirAll(keysDir, 0o555)) // ← no t.Cleanup to chmod back
    _, err := NewInternalCA(dir)
    assert.Error(t, err)
}
```

If `NewInternalCA` (via its `generateCA` path) partially writes any entry into `keysDir` before the permission check causes a failure, `os.RemoveAll` during `t.TempDir()` cleanup will encounter a directory it cannot modify. Compare with the adjacent test (`TestNewInternalCA_ReadOnlyDataRoot`, ~line 153) which correctly pairs `os.Chmod(dir, 0o555)` with `t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })`.

#### 2.4.2 `server_coverage_test.go` — Goroutine-Starting Test Without `srv.Stop()`

`TestOrthrusServer_WatchHeartbeat_ClosedSession_ExitsAndMarksOffline` (~line 186) calls `testWSPair`, creates an `AgentSession` (starts yamux `recvLoop`/`sendLoop` goroutines), stores it in `srv.sessions`, and starts a `watchHeartbeat` goroutine manually. The test correctly waits for `watchHeartbeat` to finish, but does **not** call `t.Cleanup(srv.Stop)`. If yamux internal goroutines have not drained by the time the `testWSPair` `done()` callback closes the underlying WebSocket, a goroutine may panic on the closed connection after the test function returns.

**Only one test** in this file (`TestOrthrusServer_HandleWebSocket_ValidToken_UpgradesConnection`, ~line 226) correctly uses `t.Cleanup(srv.Stop)` with the comment "must run before DB/TempDir cleanups to drain watchHeartbeat goroutine". All other tests that create `NewOrthrusServer` are missing this.

> **Note**: `srv.Stop` is only required for tests that exercise code paths that start goroutines — specifically those calling `HandleWebSocket` (which starts the yamux session and heartbeat loops) or directly invoking `watchHeartbeat`. Tests that only call REST handlers or inspect server state do not start any goroutines through the server and therefore do not need `srv.Stop`.

#### 2.4.3 `orthrus_handler_test.go` — `newOrthrusTestSetup` Has No `srv.Stop()`

`newOrthrusTestSetup` (~line 49) creates `NewOrthrusServer` using a `t.TempDir()`-backed CA path but never registers `t.Cleanup(func() { srv.Stop() })`:

```go
func newOrthrusTestSetup(t *testing.T) (*OrthrusHandler, *gorm.DB) {
    dir := t.TempDir()
    caPath := filepath.Join(dir, "ca")
    // ...
    srv, err := orthrus.NewOrthrusServer(db, ca) // ← context/cancel leaked; no Stop()
    // ...
}
```

While the handler REST tests do not currently call `HandleWebSocket`, the `context.WithCancel` cancel function inside `NewOrthrusServer` is never called. If any future test exercises the WebSocket path through this setup, goroutines will leak into subsequent tests with no cleanup registered.

---

## 3. Technical Specifications

### 3.1 Fix 1 — Grep Pattern

**File**: `scripts/go-test-coverage.sh`

**Line**: 132

**Before**:
```bash
grep -E "(FAIL:|--- FAIL:)" "$TEST_OUTPUT_FILE" || echo "No specific failures captured in output"
```

**After**:
```bash
grep -E "(^--- FAIL:|^FAIL[[:space:]]|^WARNING: DATA RACE|^panic:)" "$TEST_OUTPUT_FILE" || echo "No specific failures captured in output"
```

**Pattern breakdown**:

| Sub-pattern | Matches |
|---|---|
| `^--- FAIL:` | Individual test assertion failures from `go test -v` or gotestsum detailed mode |
| `^FAIL[[:space:]]` | Package-level failures from gotestsum pkgname: `FAIL\t<pkg>` or `FAIL  <pkg>` |
| `^WARNING: DATA RACE` | Go race detector reports (with `-race` flag) |
| `^panic:` | Test binary panics (TempDir cleanup, goroutine crash, etc.) |

### 3.2 Fix 2 — `ca_test.go` Permission Cleanup

**File**: `backend/internal/orthrus/ca_test.go`

**Target**: `TestNewInternalCA_ReadOnlyKeysDir` (~line 162)

Add a `t.Cleanup` to restore `keysDir` permissions before `t.TempDir()` runs its removal. Insert **one line** immediately after `os.MkdirAll(keysDir, 0o555)`:

```go
t.Cleanup(func() { _ = os.Chmod(keysDir, 0o700) })
```

This mirrors the pattern already used in `TestNewInternalCA_ReadOnlyDataRoot` at ~line 153.

### 3.3 Fix 3 — `srv.Stop()` in Goroutine-Starting Test

**File**: `backend/internal/orthrus/server_coverage_test.go`

**Target**: `TestOrthrusServer_WatchHeartbeat_ClosedSession_ExitsAndMarksOffline` (~line 186)

Add `t.Cleanup(srv.Stop)` immediately after the server is created and its error is checked. Insert **one line** after `require.NoError(t, err)` on ~line 189:

```go
t.Cleanup(srv.Stop) // drains watchHeartbeat and yamux goroutines before TempDir cleanup
```

### 3.4 Fix 4 — `srv.Stop()` in Handler Test Setup

**File**: `backend/internal/api/handlers/orthrus_handler_test.go`

**Target**: `newOrthrusTestSetup` (~line 49)

Register `srv.Stop()` via `t.Cleanup` inside the helper so every test using this setup benefits. Insert **one line** after the `NewOrthrusServer` call succeeds:

```go
t.Cleanup(srv.Stop)
```

---

## 4. Implementation Plan

### Phase 1 — Local Reproduction (Recommended Pre-Work)

Before writing any code, confirm the failure is reproducible:

```bash
cd /projects/Charon/backend
go test -race -count=3 -timeout=120s \
    ./internal/orthrus/... \
    ./internal/api/handlers/... \
    2>&1 | tee /tmp/orthrus-test-run.txt

grep -E "(^--- FAIL:|^FAIL[[:space:]]|^WARNING: DATA RACE|^panic:)" /tmp/orthrus-test-run.txt \
    || echo "All clean"
```

Run with `-count=3` to surface flakiness. If a panic or data race appears, the stack trace pinpoints the exact goroutine and line. Apply targeted additional fixes if needed beyond sections 3.2–3.4.

### Phase 2 — Backend Implementation

#### Task 2.1 — Fix grep pattern in `go-test-coverage.sh`

| Field | Detail |
|---|---|
| **File** | `scripts/go-test-coverage.sh` |
| **Line** | 132 |
| **Change** | Replace `grep -E "(FAIL:|--- FAIL:)"` with `grep -E "(^--- FAIL:|^FAIL[[:space:]]|^WARNING: DATA RACE|^panic:)"` |
| **Risk** | Low — diagnostic only, does not affect test execution or exit code |
| **Validation** | `bash -n scripts/go-test-coverage.sh` passes (syntax check) |

#### Task 2.2 — Fix `TestNewInternalCA_ReadOnlyKeysDir` permission cleanup

| Field | Detail |
|---|---|
| **File** | `backend/internal/orthrus/ca_test.go` |
| **Function** | `TestNewInternalCA_ReadOnlyKeysDir` (~line 162) |
| **Change** | Add `t.Cleanup(func() { _ = os.Chmod(keysDir, 0o700) })` after `os.MkdirAll(keysDir, 0o555)` |
| **Risk** | Low — only affects cleanup, not test assertions |
| **Validation** | `go test -race -run TestNewInternalCA_ReadOnlyKeysDir ./internal/orthrus/` passes |

#### Task 2.3 — Add `srv.Stop()` cleanup in `TestOrthrusServer_WatchHeartbeat`

| Field | Detail |
|---|---|
| **File** | `backend/internal/orthrus/server_coverage_test.go` |
| **Function** | `TestOrthrusServer_WatchHeartbeat_ClosedSession_ExitsAndMarksOffline` (~line 186) |
| **Change** | Add `t.Cleanup(srv.Stop)` after `require.NoError(t, err)` on ~line 189 |
| **Risk** | Low — adds cleanup, does not change test assertions |
| **Validation** | `go test -race -run TestOrthrusServer_WatchHeartbeat ./internal/orthrus/` passes |

#### Task 2.4 — Add `srv.Stop()` cleanup in `newOrthrusTestSetup`

| Field | Detail |
|---|---|
| **File** | `backend/internal/api/handlers/orthrus_handler_test.go` |
| **Function** | `newOrthrusTestSetup` (~line 49) |
| **Change** | Add `t.Cleanup(srv.Stop)` after the `NewOrthrusServer` call succeeds |
| **Risk** | Low — defensive cleanup, does not change test assertions |
| **Validation** | `go test -race -run TestOrthrusHandler ./internal/api/handlers/` passes |

### Phase 3 — Frontend Implementation

Not applicable.

### Phase 4 — Integration and Testing

#### Task 4.1 — Verify new grep pattern fires correctly

```bash
# Should match
echo -e "FAIL\tgithub.com/example/pkg\t1.23s" \
    | grep -E "(^--- FAIL:|^FAIL[[:space:]]|^WARNING: DATA RACE|^panic:)"

echo "panic: testing: TempDir RemoveAll cleanup: remove /tmp/xxx: directory not empty" \
    | grep -E "(^--- FAIL:|^FAIL[[:space:]]|^WARNING: DATA RACE|^panic:)"

echo "WARNING: DATA RACE" \
    | grep -E "(^--- FAIL:|^FAIL[[:space:]]|^WARNING: DATA RACE|^panic:)"

# Should NOT match (not a failure line)
echo "ok  github.com/example/pkg  0.234s" \
    | grep -E "(^--- FAIL:|^FAIL[[:space:]]|^WARNING: DATA RACE|^panic:)" || echo "Correct: no match"
```

#### Task 4.2 — Run full backend test suite

```bash
cd /projects/Charon
bash scripts/go-test-coverage.sh
```

Expected:
- All packages pass, `GO_TEST_STATUS` = 0
- FAILED TEST SUMMARY section is not printed
- Coverage remains ≥ 87%

#### Task 4.3 — Run with `-count=3` for flakiness confidence

```bash
cd /projects/Charon/backend
go test -race -count=3 -timeout=120s ./internal/orthrus/... ./internal/api/handlers/...
```

All three runs must pass.

---

## 5. Acceptance Criteria

| # | Criterion | Verification Method |
|---|---|---|
| AC-1 | `go-test-coverage.sh` line 132 contains `^FAIL[[:space:]]` | `grep "FAIL\[" scripts/go-test-coverage.sh` |
| AC-2 | `FAIL\t<pkg>` lines appear in FAILED TEST SUMMARY when a package fails | Echo test from Task 4.1 |
| AC-3 | `panic:` lines appear in FAILED TEST SUMMARY when the binary crashes | Echo test from Task 4.1 |
| AC-4 | `TestNewInternalCA_ReadOnlyKeysDir` has a `t.Cleanup` restoring `keysDir` to `0o700` | `grep -n "t.Cleanup" backend/internal/orthrus/ca_test.go` shows entry near line 162 |
| AC-5 | `TestOrthrusServer_WatchHeartbeat_ClosedSession_ExitsAndMarksOffline` has `t.Cleanup(srv.Stop)` | Code review of `server_coverage_test.go` |
| AC-6 | `newOrthrusTestSetup` in `orthrus_handler_test.go` has `t.Cleanup(srv.Stop)` | Code review |
| AC-7 | `go test -race -count=3 ./internal/orthrus/... ./internal/api/handlers/...` passes all three runs | Terminal output |
| AC-8 | Backend (Go) CI job on the fix PR is green | GitHub Actions status |
| AC-9 | Coverage remains ≥ 87% | CI coverage report |

---

## 6. Commit Slicing Strategy

### Decision

Single PR with two logically ordered commits. The grep fix is trivially reviewable in isolation. The test cleanup changes span three files but share a single concern (cleanup hygiene) and can land together.

### Trigger Reasons

- Scope is narrow: 4 files, ≤ 8 lines added across all commits
- Low risk: diagnostic + test cleanup only, zero production code changes
- Two commits allows reviewers to approve the script fix independently

### Ordered Commits

#### Commit 1 — Diagnostic fix

```
fix(ci): surface package-level failures and panics in test failure summary

The previous pattern never matched package-level failures from
`gotestsum --format pkgname`, which emits `FAIL\t<package>` (tab separator,
no colon). Panics and data race reports were also missed.

Replace with a pattern that covers:
  - individual test assertion failures  (^--- FAIL:)
  - package-level failures              (^FAIL[[:space:]])
  - race detector reports               (^WARNING: DATA RACE)
  - test binary panics                  (^panic:)
```

| Field | Value |
|---|---|
| **Files** | `scripts/go-test-coverage.sh` |
| **Lines changed** | 1 (line 132) |
| **Dependencies** | None |
| **Validation gate** | `bash -n scripts/go-test-coverage.sh` passes; echo tests from Task 4.1 succeed |

#### Commit 2 — Test cleanup hardening

```
fix(test): guard goroutine lifecycle in server tests to prevent silent crashes

Three test hygiene gaps caused the orthrus package test binary to exit
non-zero without any `--- FAIL:` line:

1. A read-only keys directory test created a 0o555 subdirectory without
   restoring permissions before cleanup, causing TempDir removal to panic.
   Added a t.Cleanup mirroring the pattern from the adjacent data-root test.

2. The WatchHeartbeat closed-session test started yamux goroutines without
   registering srv.Stop(), allowing goroutines to race against TempDir/DB
   cleanups after the test function returned.

3. The handler test setup helper never cancelled the OrthrusServer context.
   Added t.Cleanup(srv.Stop) consistent with the pattern in the server test
   suite.
```

| Field | Value |
|---|---|
| **Files** | `backend/internal/orthrus/ca_test.go`, `backend/internal/orthrus/server_coverage_test.go`, `backend/internal/api/handlers/orthrus_handler_test.go` |
| **Lines changed** | 3 lines added (one per file) |
| **Dependencies** | Commit 1 (for CI to show useful output if a residual issue exists) |
| **Validation gate** | `go test -race -count=3 ./internal/orthrus/... ./internal/api/handlers/...` passes; CI Backend job green |

### Rollback Notes

- Commit 1 is trivially revertable; it only affects CI output, not test pass/fail logic.
- Commit 2 additions are purely additive (`t.Cleanup` calls). Reverting would re-introduce the potential panic but would not break any assertion.
- Neither commit touches production code paths.

---

## 7. Risks and Mitigations

| Risk | Probability | Impact | Mitigation |
|---|---|---|---|
| Additional unknown cause of failure not covered by these three fixes | Medium | High | Phase 1 reproduction run with `-count=3` and `-race` surfaces remaining flakiness before the PR opens |
| A different goroutine-starting test also needs `srv.Stop()` | Low | Medium | `-race -count=3` will catch it; add `t.Cleanup(srv.Stop)` where needed |
| Grep change accidentally matches non-failure output lines | Very Low | Low | Line anchors (`^`) and `[[:space:]]` class prevent partial-line false positives |
| `t.Cleanup(func() { _ = os.Chmod(keysDir, 0o700) })` not registered before partial write | Very Low | Low | Cleanup is registered immediately after `os.MkdirAll`, before any code that could fail and skip the line |

# CI Fix — ESLint + Backend Test Failures

**Branch**: `fix/ci-eslint-backend-test`
**PR**: Targets `development`
**Date**: 2026-05-29

---

> **Archived**: The previous spec (CI Fix — Vitest `invites a new user` Failure) has been
> superseded. This document covers the active CI failures.

---

## 1. Introduction

### Overview

Multiple CI jobs in `quality-checks.yml` are failing. The failures split across two domains:

1. **Frontend ESLint** — Blocking job; 7 distinct lint violations across 5 source files and 1
   test file.
2. **Backend test suite** — Blocking job (via `scripts/go-test-coverage.sh`);
   `TestSecurityHandler_UpsertRuleSet_XSSInContent` fails when the full handler package test suite
   runs in parallel.

Backend **lint** violations exist but are configured with `continue-on-error: true` and are
therefore non-blocking. They are explicitly excluded from this plan.

### Objectives

- Restore all CI jobs to green in a single PR composed of 6 ordered, reviewable commits.
- No feature changes; this is a pure fix/refactor to make the test suite and linter pass.

---

## 2. Research Findings

### 2.1 CI Workflow

**File**: `.github/workflows/quality-checks.yml`

| Job | Script / Command | Blocking? |
|-----|-----------------|-----------|
| Backend tests + coverage | `scripts/go-test-coverage.sh` | **Yes** |
| Backend lint | `golangci/golangci-lint-action@v9.2.0` with `continue-on-error: true` | No |
| Frontend ESLint | `npm run lint` | **Yes** |
| Frontend type-check | `npm run type-check` | Yes |
| Frontend unit tests | `scripts/frontend-test-coverage.sh` | Yes |

`scripts/go-test-coverage.sh` captures `GO_TEST_STATUS` from the test run, applies the coverage
gate, and at the end bubbles up any non-zero test exit code (script lines 290-295). A single
failing test therefore causes the job to exit non-zero.

### 2.2 Frontend ESLint — Root Causes

**ESLint plugins in use** (`frontend/eslint.config.js`):
`jsx-a11y`, `security`, `react-refresh`, `@vitest/eslint-plugin` (aliased as `vitest`)

#### Failure 1 — Duplicate devDependencies

`frontend/package.json` contains three devDependency keys that each appear twice:

| Duplicate key | Affected lines |
|---|---|
| `@typescript-eslint/eslint-plugin` | first occurrence + second set ~lines 74-76 |
| `@typescript-eslint/parser` | same |
| `@typescript-eslint/utils` | same |

npm treats a duplicate key as a parse-time error that prevents consistent lock-file resolution,
causing the ESLint run to fail with a dependency error.

#### Failure 2 — Unsafe regex (`security/detect-unsafe-regex`)

**File**: `frontend/src/components/CredentialManager.tsx`

`validateZoneFilter` uses a regex with nested quantifiers. The rule flags it as a potential ReDoS
vector. The regex is intentional; the risk is acceptable for this context.

**Fix**: Add an inline `// eslint-disable-next-line security/detect-unsafe-regex` with a
justification comment immediately before the regex literal.

#### Failure 3 — Non-component exports (`react-refresh/only-export-components`)

**File**: `frontend/src/components/CertificateList.tsx` — lines 20 and 24

```tsx
export function isInUse(cert: Certificate): boolean { ... }    // line 20
export function isDeletable(cert: Certificate): boolean { ... } // line 24
```

`react-refresh` requires component files export **only React components**. Exporting plain utility
functions breaks HMR and triggers this rule.

**Known import sites** (must be updated after move):
- `frontend/src/components/__tests__/CertificateList.test.tsx` line 8

**Fix**: Move both functions to a new `frontend/src/utils/certificateUtils.ts`; update all import
sites; keep internal calls in `CertificateList.tsx` via the new import.

#### Failure 4 — Label on non-labelable elements (`jsx-a11y/label-has-associated-control`)

5 violations across 3 files:

| File | Line | Element | Problem |
|---|---|---|---|
| `frontend/src/components/CSPBuilder.tsx` | ~326 | `<label>` wrapping `<pre>` | `<pre>` is not a labelable element |
| `frontend/src/components/AccessListSelector.tsx` | ~128 | `<label>` with no `htmlFor` before custom `<Select>` | custom component not natively labelable |
| `frontend/src/components/AccessListForm.tsx` | ~385 | `<label id="access-list-enabled-label">` | no `htmlFor`; paired with `<Switch aria-labelledby="...">` |
| `frontend/src/components/AccessListForm.tsx` | ~401 | `<label id="access-list-local-network-label">` | same pattern |
| `frontend/src/components/AccessListForm.tsx` | ~422 | `<label>` | no `id`, no `htmlFor` |

**Fix for AccessListForm lines 385 & 401**: Replace `<label id="...">` → `<span id="...">` to
preserve the `id` referenced by `aria-labelledby` on the paired `<Switch>` components.

**Fix for all others**: Replace `<label>` → `<span>` (no `id` or `aria-*` changes needed).

> **Accessibility note**: `<span id="...">` with `aria-labelledby` on the control is the
> WCAG 2.2-compliant approach when the control is a custom component that does not expose a native
> labelable element.

#### Failure 5 — Skipped tests (`vitest/prefer-todo` / `vitest/no-disabled-tests`)

**File**: `frontend/src/api/__tests__/logs-websocket.test.ts` — lines 134 and 151

Both use `it.skip('description', () => { ... })` with full test bodies. The rule requires either
a passing test or `it.todo('description')` (no body) for placeholder tests.

**Fix**: Replace both with `it.todo('description')` and remove the test bodies.

### 2.3 Backend Test Failure — Root Cause

#### Failing test

**File**: `backend/internal/api/handlers/security_handler_audit_test.go`
**Test**: `TestSecurityHandler_UpsertRuleSet_XSSInContent` (line ~395)

**Observed failures** (full suite run only; passes in isolation):
```
Error: Not equal:
    expected: 200
    actual  : 500
Error: "{\"error\":\"failed to list rule sets\"}" does not contain "\\u003cscript\\u003e"
```

Documented as pre-existing failure PE-001 in
`docs/reports/qa_report_import_save_regression.md`.

#### Code path to failure

1. `UpsertRuleSet` handler (lines 401-435 of `security_handler.go`) calls
   `h.svc.UpsertRuleSet(&payload)`. On any error it returns HTTP 500.
2. `UpsertRuleSet` service method (lines 413-455 of `security_service.go`) executes
   `s.db.Where("name = ?", r.Name).First(&existing)`. If this returns **any error other than**
   `ErrRecordNotFound`, it returns that error immediately. Under SQLite busy-lock conditions the
   query returns `SQLITE_BUSY`, which is returned to the handler, causing HTTP 500.
3. The subsequent GET `ListRuleSets` call also fails (DB connection in a broken state), producing
   the `"failed to list rule sets"` response.

#### Why parallel tests cause locking

`setupAuditTestDB` creates a **file-based** SQLite database:
```go
dsn := filepath.Join(t.TempDir(), "security_handler_audit_test.db") +
    "?_busy_timeout=5000&_journal_mode=WAL"
db.SetMaxOpenConns(1)
db.SetMaxIdleConns(1)
```

Many other handler test files use `t.Parallel()` (confirmed: `auth_handler_test.go`,
`proxy_host_handler_test.go`, `proxy_host_handler_update_test.go`). These tests execute
concurrently with the audit tests in a full package run.

`NewSecurityHandler` calls `services.NewSecurityService(db)` which immediately starts a
`processAuditEvents()` background goroutine. Because `SecurityHandler.svc` is an **unexported
field**, callers cannot invoke `svc.Close()` directly. All 14 `NewSecurityHandler` call sites in
`security_handler_audit_test.go` register **no cleanup** for the service goroutine — unlike
`security_service_test.go` which always registers `t.Cleanup(func() { svc.Close() })`.

Under parallel load: accumulated open goroutines each holding WAL-mode file-based SQLite
connections cause `SQLITE_BUSY` errors despite the 5-second busy-timeout.

#### Fix strategy (two complementary changes)

**A — Add `Close()` to `SecurityHandler` + register cleanup at all 14 call sites**

Stops goroutine accumulation. An exported `Close()` method is required because `svc` is
unexported.

**B — Convert `setupAuditTestDB` from file-based to in-memory SQLite**

`OpenTestDB(t)` (defined in `testdb.go`) creates a uniquely-named in-memory SQLite with
`mode=memory&cache=shared` and auto-registered cleanup. This eliminates WAL file-lock as a
failure vector, matching the pattern used by all working handler tests.

Both changes together provide defense in depth: goroutine lifecycle is clean AND the DB is not
susceptible to file-locking under concurrent load.

---

## 3. Technical Specifications

### 3.1 Files Modified / Created

| File | Action | Commit |
|---|---|---|
| `frontend/package.json` | Remove 3 duplicate devDependency keys | 1 |
| `frontend/src/components/CredentialManager.tsx` | Add inline eslint-disable comment | 2 |
| `frontend/src/utils/certificateUtils.ts` | **New file** — export `isInUse`, `isDeletable` | 3 |
| `frontend/src/components/CertificateList.tsx` | Remove exports; add import from utils | 3 |
| `frontend/src/components/__tests__/CertificateList.test.tsx` | Update import path | 3 |
| `frontend/src/components/CSPBuilder.tsx` | 1× `<label>` → `<span>` | 4 |
| `frontend/src/components/AccessListSelector.tsx` | 1× `<label>` → `<span>` | 4 |
| `frontend/src/components/AccessListForm.tsx` | 3× `<label>` → `<span>` (2 preserve `id`) | 4 |
| `frontend/src/api/__tests__/logs-websocket.test.ts` | 2× `it.skip` → `it.todo` | 5 |
| `backend/internal/api/handlers/security_handler.go` | Add exported `Close()` method | 6 |
| `backend/internal/api/handlers/security_handler_audit_test.go` | Add cleanup; convert to in-memory DB | 6 |

### 3.2 Detailed Change Specifications

#### Commit 1 — `frontend/package.json`

Remove the **second** occurrence of these three devDependency keys (approximately lines 74-76):
```
"@typescript-eslint/eslint-plugin": "...",
"@typescript-eslint/parser": "...",
"@typescript-eslint/utils": "...",
```

After the change, each key appears exactly once in `devDependencies`.

**Validation**: `cd frontend && npm install --dry-run` must complete without duplicate key
warnings.

#### Commit 2 — `CredentialManager.tsx`

Locate `validateZoneFilter` and the regex literal it contains. Add two lines immediately before
the regex:

```ts
// eslint-disable-next-line security/detect-unsafe-regex
// Runs client-side only; ReDoS risk is confined to the user's own browser session
```

**Validation**: `npm run lint` on the file shows no `security/detect-unsafe-regex` errors.

#### Commit 3 — Extract certificate utilities

**New file**: `frontend/src/utils/certificateUtils.ts`

```ts
import { type Certificate } from '../api/certificates'

export function isInUse(cert: Certificate): boolean {
  return cert.in_use
}

export function isDeletable(cert: Certificate): boolean {
  if (cert.in_use) return false
  return (
    cert.provider === 'custom' ||
    cert.provider === 'letsencrypt-staging' ||
    cert.status === 'expired' ||
    cert.status === 'expiring'
  )
}
```

**`frontend/src/components/CertificateList.tsx`** changes:
- Remove the two `export function` declarations at lines 20-31.
- Add to the import block: `import { isInUse, isDeletable } from '../utils/certificateUtils'`
- Internal call sites (lines 103, 225, 226) are unchanged — they reference the same function
  names now satisfied by the import.

**`frontend/src/components/__tests__/CertificateList.test.tsx`** line 8:
```ts
// Before:
import CertificateList, { isDeletable, isInUse } from '../CertificateList'

// After:
import CertificateList from '../CertificateList'
import { isDeletable, isInUse } from '../../utils/certificateUtils'
```

**Validation**: `npm run lint` — no `react-refresh/only-export-components` errors.
`npm run test -- CertificateList` — all existing tests pass.

#### Commit 4 — Replace `<label>` on non-labelable elements

| File | Change |
|---|---|
| `CSPBuilder.tsx` ~line 326 | `<label` → `<span` and `</label>` → `</span>` |
| `AccessListSelector.tsx` ~line 128 | `<label` → `<span` and `</label>` → `</span>` |
| `AccessListForm.tsx` ~line 385 | `<label id="access-list-enabled-label">` → `<span id="access-list-enabled-label">` and `</label>` → `</span>` |
| `AccessListForm.tsx` ~line 401 | `<label id="access-list-local-network-label">` → `<span id="access-list-local-network-label">` and `</label>` → `</span>` |
| `AccessListForm.tsx` ~line 422 | `<label` → `<span` and `</label>` → `</span>` |

**Critical constraint for lines 385 & 401**: The `id` attribute **must be preserved** — `<Switch>`
components reference it via `aria-labelledby`. Removing the `id` would break the programmatic
label association for screen readers.

**Validation**: `npm run lint` — no `jsx-a11y/label-has-associated-control` errors.

#### Commit 5 — `logs-websocket.test.ts`

Replace both `it.skip(...)` blocks at lines 134 and 151 with `it.todo(...)`, removing the
callback bodies entirely:

```ts
// Before:
// These tests are skipped because ...
it.skip('should do X', () => {
  // ... test body ...
})

// After:
// These tests are skipped because ...
it.todo('should do X')
```

Apply to both occurrences. **Preserve the explanatory comment** that appears immediately above
each `it.skip` call — it must remain above the resulting `it.todo` line.

**Test bodies are deleted outright** — do not preserve them as commented-out code. The prior
implementations are retained in git history and can be recovered from there if needed.

**Validation**: `npm run lint` — no disabled-test rule violations. `npm run test` shows both as
`todo`.

#### Commit 6 — Backend test fix

##### Part A: Add `Close()` to `security_handler.go`

After `NewSecurityHandler` (or `NewSecurityHandlerWithDeps`), add:

```go
// Close stops the background audit goroutine. Required for test cleanup.
func (h *SecurityHandler) Close() {
	h.svc.Close()
}
```

`svc` is unexported; this method is the only way for test code to call `svc.Close()`.

##### Part B1: Replace `setupAuditTestDB` in `security_handler_audit_test.go`

Replace the entire function body with:

```go
func setupAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	if err := db.AutoMigrate(
		&models.SecurityRuleSet{},
		&models.SecurityConfig{},
		&models.SecurityDecision{},   // ← required, not AccessListRule
		&models.SecurityAudit{},
		&models.Setting{},
	); err != nil {
		t.Fatalf("setupAuditTestDB migrate: %v", err)
	}
	return db
}
```

`OpenTestDB(t)` creates a uniquely-named in-memory SQLite with auto-registered cleanup. This
eliminates WAL file-locking entirely.

> The model list (`SecurityRuleSet`, `SecurityConfig`, `SecurityDecision`, `SecurityAudit`,
> `Setting`) must match this specification exactly. `SecurityDecision` is required because
> `TestSecurityHandler_CreateDecision_SQLInjection` and
> `TestSecurityHandler_CreateDecision_EmptyFields` both query the `security_decisions` table.
> `AccessListRule` is **not** in this list.

##### Part B2: Add `t.Cleanup(func() { h.Close() })` at all 14 call sites

Every `h := NewSecurityHandler(cfg, db, nil)` call must be immediately followed by:
```go
t.Cleanup(func() { h.Close() })
```

The 14 call site line numbers (approximate — verify before applying):
`76, 98, 144, 179, 210, 280, 325, 355, 399, 447, 508, 536, 562, 597`

##### Part B3: Add diagnostic logging to the failing test

In `TestSecurityHandler_UpsertRuleSet_XSSInContent`, immediately before the first
`assert.Equal(t, http.StatusOK, w.Code)`, add:
```go
t.Logf("UpsertRuleSet response body: %s", w.Body.String())
```

This surfaces the real GORM error message if the test regresses in future.

**Validation**:
```bash
# Isolated run must pass
go test -v -count=1 -run TestSecurityHandler_UpsertRuleSet_XSSInContent ./backend/internal/api/handlers/

# Full package with race detector must pass
go test -race -count=1 ./backend/internal/api/handlers/

# Coverage gate must pass
bash scripts/go-test-coverage.sh
```

---

## 4. Implementation Plan

### Phase 1 — Playwright Tests

No UI/UX behaviour changes are introduced. Playwright E2E tests are not required for this plan.
Changes are limited to build config, utility extraction, HTML element type changes (visually
identical), and test-only changes. A smoke run of the standard suite is optional.

### Phase 2 — Backend Implementation (Commit 6)

1. Edit `security_handler.go` — add `Close()` method.
2. Edit `security_handler_audit_test.go`:
   a. Replace `setupAuditTestDB` body.
   b. Add `t.Cleanup` at all 14 call sites.
   c. Add `t.Logf` diagnostic.
3. Run: `go test -race -count=1 ./backend/internal/api/handlers/` — confirm exit 0.
4. Run: `bash scripts/go-test-coverage.sh` — confirm coverage gate passes.

### Phase 3 — Frontend Implementation (Commits 1-5)

Execute commits 1-5 in order. After each commit, run `npm run lint` to verify no new errors.
After all five: run `npm run type-check` and `npm run test` to confirm no regressions.

### Phase 4 — Integration Verification

Final checklist before opening PR:
- [ ] `go test -race ./backend/...` exits 0
- [ ] `bash scripts/go-test-coverage.sh` exits 0
- [ ] `npm run lint` exits 0 (zero errors)
- [ ] `npm run type-check` exits 0
- [ ] `npm run test` exits 0

### Phase 5 — Documentation

No documentation changes required.

---

## 5. Acceptance Criteria

| # | Criterion | Verification |
|---|---|---|
| AC-1 | `TestSecurityHandler_UpsertRuleSet_XSSInContent` passes in full package run | `go test -race -count=1 ./backend/internal/api/handlers/` exits 0 |
| AC-2 | `scripts/go-test-coverage.sh` exits 0 | Coverage gate and test status both pass |
| AC-3 | `npm run lint` exits 0 | No ESLint errors; `react-refresh`, `jsx-a11y`, `security`, `vitest` rules all pass |
| AC-4 | `npm run type-check` exits 0 | No TypeScript errors after utility extraction |
| AC-5 | All existing Vitest tests continue to pass | `npm run test` exits 0; `CertificateList.test.tsx` passes |
| AC-6 | `<Switch>` components retain ARIA labels | `aria-labelledby` on Switch components resolves to matching `id` on adjacent `<span>` |
| AC-7 | No backend lint regressions | Backend lint (non-blocking) gains no new CRITICAL/HIGH findings |

---

## 6. Commit Slicing Strategy

**Decision**: Single PR with 6 ordered logical commits.

**Trigger reasons**: Cross-domain changes (frontend/backend); logical independence of each fix;
reviewability of each concern in isolation.

### Commit Order

| # | Commit message | Files | Dependencies | Validation gate |
|---|---|---|---|---|
| 1 | `fix(frontend): remove duplicate devDependencies from package.json` | `frontend/package.json` | None | `npm install` no warnings |
| 2 | `fix(frontend): suppress unsafe-regex lint in zone filter validation` | `CredentialManager.tsx` | None | `npm run lint` clean on file |
| 3 | `refactor(frontend): extract certificate utility functions to utils module` | `certificateUtils.ts` (new), `CertificateList.tsx`, `CertificateList.test.tsx` | None | `npm run lint` + `npm run test -- CertificateList` |
| 4 | `fix(frontend/a11y): replace label with span for non-labelable controls` | `CSPBuilder.tsx`, `AccessListSelector.tsx`, `AccessListForm.tsx` | None | `npm run lint` clean on all 3 |
| 5 | `fix(tests): convert skipped tests to todo in logs-websocket` | `logs-websocket.test.ts` | None | `npm run lint` clean on file |
| 6 | `fix(backend/test): resolve UpsertRuleSet XSS test isolation failure` | `security_handler.go`, `security_handler_audit_test.go` | None | `go test -race ./backend/internal/api/handlers/` exits 0 |

Commits 1-6 are fully independent and may be cherry-picked or reordered safely.

### Rollback Notes

Each commit is self-contained. Reverting any one commit has no cross-domain impact.
Commit 6 revert returns the CI failure but affects no production behaviour.

---

## 7. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `AccessListForm` `<Switch>` `aria-labelledby` broken after label → span | Low | Medium | Spec explicitly preserves `id` on converted `<span>` elements |
| `CertificateList.test.tsx` import path wrong after utility extraction | Low | Low | Test suite catches immediately; path is `../../utils/certificateUtils` |
| `setupAuditTestDB` model list wrong | Low | High | Spec now explicitly requires `SecurityDecision` (not `AccessListRule`); required by `TestSecurityHandler_CreateDecision_*` tests |
| One or more of the 14 `t.Cleanup` call sites missed | Low | Low | In-memory DB (B1) already eliminates the locking failure; goroutine cleanup is defence-in-depth |
| `it.todo` removal of test body causes compile error | None | — | `it.todo` takes only a string argument; no compilation issues |

---

## Commit 7 — Fix Cloudflare provider stdout capture race condition

**Branch addition**: `fix/ci-eslint-backend-test` (same PR)
**CI failure**: `TestStart_CapturesStdoutOutput` — 1.01 s timeout, ring buffer empty
**PR**: <https://github.com/Wikid82/Charon/actions/runs/25817001017/job/75848015026?pr=1013>

---

### 7.1 Root Cause Analysis

#### Failing test

**File**: `backend/internal/hecate/providers/cloudflare/coverage_test.go`
**Test**: `TestStart_CapturesStdoutOutput` (~line 113)
**Error** (line 143):
```
Error: Should NOT be empty, but was []
Messages: stdout scanner goroutine must have written output to the ring buffer
```

The test starts the `echo` binary via `p.Start()`, waits for the process to exit via `<-done`,
polls `p.buf.ReadAll()` for up to 1 second, then asserts it is non-empty. It consistently returns
empty on CI.

#### The race

`Start()` launches three goroutines in this order:

| Goroutine | Role |
|---|---|
| stdout scanner | `bufio.Scanner` reads `stdoutPipe`; calls `p.buf.Write(s.Text())` for each line |
| stderr scanner | same but for `stderrPipe` |
| monitor | calls `cmd.Wait()`; in its **deferred** cleanup calls `p.buf.Close()` then `close(p.done)` |

The critical ordering defect in the monitor goroutine's deferred function
(`backend/internal/hecate/providers/cloudflare/provider.go`, lines ~175–184):

```go
// BEFORE (buggy)
go func() {
    defer func() {
        p.mu.Lock()
        p.cmd = nil
        if p.state != hecate.TunnelStateStopped {
            p.state = hecate.TunnelStateError
        }
        p.mu.Unlock()
        p.buf.Close()   // ← (1) closes the buffer
        close(p.done)   // ← (2) signals test
    }()
    _ = cmd.Wait()
}()
```

Once `cmd.Wait()` returns (process exited), the monitor deferred function runs and calls
`p.buf.Close()`. This sets `rb.closed = true` inside `RingBuffer`
(`backend/internal/hecate/ring_buffer.go`, line ~95).

Concurrently, the stdout scanner goroutine may not yet have been scheduled. When it eventually
runs and calls `p.buf.Write(s.Text())`, the guard at `ring_buffer.go` line ~31 fires:

```go
func (rb *RingBuffer) Write(line string) {
    rb.mu.Lock()
    defer rb.mu.Unlock()
    if rb.closed {
        return  // ← silently drops the write
    }
    // ...
}
```

The write is silently dropped. The ring buffer remains empty. The test's 1-second polling loop
exhausts without finding any data. `ReadAll()` returns `nil` and the assertion fails.

#### Why it is non-deterministic

The race window is the interval between `cmd.Wait()` returning and the scanner goroutine calling
`p.buf.Write()`. On a lightly-loaded machine the Go scheduler tends to run the scanner goroutines
first because they are I/O-bound and already have data waiting in the pipe. On a loaded CI runner
the monitor goroutine is more likely to win the scheduler lottery, close the buffer, and leave the
scanner goroutine with nowhere to write.

`echo` exits in < 1 ms; the entire race window is sub-millisecond — too short for `time.Sleep`-
based workarounds but reliably closed by a `sync.WaitGroup`.

---

### 7.2 File Locations and Exact Lines

| File | Lines of interest |
|---|---|
| `backend/internal/hecate/providers/cloudflare/provider.go` | ~153–185 (`Start()` goroutine block) |
| `backend/internal/hecate/ring_buffer.go` | ~29–31 (`Write` closed guard), ~93–101 (`Close`) |
| `backend/internal/hecate/providers/cloudflare/coverage_test.go` | ~113–143 (`TestStart_CapturesStdoutOutput`) |

---

### 7.3 Fix Specification

**Single change**: add a `sync.WaitGroup` (`scanWg`) that tracks both scanner goroutines.
The monitor goroutine calls `scanWg.Wait()` inside its deferred function, **before**
`p.buf.Close()`, so the buffer is never closed until all writes have completed.

No changes to the test or `RingBuffer` are required.

#### Before (`provider.go` — Start(), goroutine block)

```go
// Stream stdout to the ring buffer.
go func() {
    s := bufio.NewScanner(stdoutPipe)
    for s.Scan() {
        p.buf.Write(s.Text())
    }
}()

// Stream stderr to the ring buffer.
go func() {
    s := bufio.NewScanner(stderrPipe)
    for s.Scan() {
        p.buf.Write(s.Text())
    }
}()

// Monitor process exit and update state accordingly.
go func() {
    defer func() {
        p.mu.Lock()
        p.cmd = nil
        if p.state != hecate.TunnelStateStopped {
            p.state = hecate.TunnelStateError
        }
        p.mu.Unlock()
        p.buf.Close()
        close(p.done)
    }()
    _ = cmd.Wait()
}()
```

#### After (`provider.go` — Start(), goroutine block)

```go
var scanWg sync.WaitGroup

// Stream stdout to the ring buffer.
scanWg.Add(1)
go func() {
    defer scanWg.Done()
    s := bufio.NewScanner(stdoutPipe)
    for s.Scan() {
        p.buf.Write(s.Text())
    }
}()

// Stream stderr to the ring buffer.
scanWg.Add(1)
go func() {
    defer scanWg.Done()
    s := bufio.NewScanner(stderrPipe)
    for s.Scan() {
        p.buf.Write(s.Text())
    }
}()

// Monitor process exit and update state accordingly.
go func() {
    defer func() {
        p.mu.Lock()
        p.cmd = nil
        if p.state != hecate.TunnelStateStopped {
            p.state = hecate.TunnelStateError
        }
        p.mu.Unlock()
        scanWg.Wait() // drain scanner goroutines before closing the buffer
        p.buf.Close()
        close(p.done)
    }()
    _ = cmd.Wait()
}()
```

**Why this is safe:**
- `cmd.Wait()` returns only after the process exits and the pipe write-ends are closed by the OS.
  At that point `s.Scan()` will return `false` (EOF) on the next iteration, so both scanner
  goroutines will reach `scanWg.Done()` promptly.
- `scanWg.Wait()` blocks for at most the time it takes the scanner goroutines to drain the pipe
  buffer — microseconds in practice.
- `p.buf.Close()` and `close(p.done)` are still called in the same relative order; only
  `scanWg.Wait()` is inserted between the state unlock and `p.buf.Close()`.
- `sync` is already imported in `provider.go` (used by `sync.RWMutex`); no new import is needed.

---

### 7.4 Commit Details

| Field | Value |
|---|---|
| **Commit message** | `fix(hecate/cloudflare): drain scanner goroutines before closing ring buffer` |
| **Files changed** | `backend/internal/hecate/providers/cloudflare/provider.go` |
| **Lines changed** | ~6 lines added (WaitGroup declaration + 2×Add + 2×Done + 1×Wait) |
| **Dependencies** | None; Commits 1-6 are independent |

### 7.5 Validation Gate

```bash
# Target test — must pass deterministically
go test ./backend/internal/hecate/providers/cloudflare/... \
    -v -count=5 -race -run TestStart_CapturesStdoutOutput

# Full cloudflare package — must pass
go test -race -count=1 ./backend/internal/hecate/providers/cloudflare/...

# Coverage gate — must not regress
bash scripts/go-test-coverage.sh
```

`-count=5` runs the test five times in a single invocation to exercise the scheduler across
multiple scheduling decisions, providing high confidence the race is closed.

---

*Plan written by GitHub Copilot in Planning mode. Ready for implementation agent handoff.*

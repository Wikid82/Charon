# Fix Patch Coverage Gaps in `cloudflare/provider.go`

## Introduction

### Overview

The recent refactor replacing `cmd.StdoutPipe()` / `cmd.StderrPipe()` with `os.Pipe()` pairs
in `Start()` introduced three new error paths and two error-logging branches that have zero
test coverage. Codecov reports **54.54% patch coverage** (8 missing lines, 2 partials) on
the changed lines in `backend/internal/hecate/providers/cloudflare/provider.go`.

### Objectives

- Increase patch coverage on `provider.go` from 54.54% to ≥ 90%.
- Use the project's established **function-variable test-hook** pattern (identical to
  `caddy/manager.go`) — no build tags or process-injection tricks required.
- Add exactly **2 package-level `var` declarations** to `provider.go` and **3 new test
  functions** to `coverage_test.go`. No other files are modified.

---

## 2. Research Findings

### Existing Pattern — Function-Variable Injection

`backend/internal/caddy/manager.go` (lines 25–38) establishes the canonical pattern:

```go
// Test hooks to allow overriding OS and JSON functions
var (
    writeFileFunc        = os.WriteFile
    readFileFunc         = os.ReadFile
    removeFileFunc       = os.Remove
    readDirFunc          = os.ReadDir
    statFunc             = os.Stat
    jsonMarshalFunc      = json.MarshalIndent
    jsonMarshalDebugFunc = json.Marshal
    generateConfigFunc   = GenerateConfig
    validateConfigFunc   = Validate
)
```

`backend/internal/caddy/client.go` (line 19) mirrors it:

```go
// Test hook for json marshalling to allow simulating failures in tests
var jsonMarshalClient = json.Marshal
```

This is the agreed-upon mechanism for unit-test injection throughout the backend.
No equivalent hooks exist yet in the `cloudflare` package — confirmed by grep
against `backend/**` for `osPipe|var.*Pipe.*=.*os\.Pipe`.

### Existing Tests That Already Cover Nearby Lines

| Test | File | Lines Covered |
|---|---|---|
| `TestStart_ExecFormatError` | `coverage_test.go` | Lines 145–155 (`cmd.Start()` error block + 4 close calls) |
| `TestStart_WithStubBinary` | `provider_test.go` | Lines 131, 135, 160, 165 (happy path — only false branches of the two pipe-check `if`s) |
| `TestStart_CapturesStdoutOutput` | `coverage_test.go` | Same happy-path range |

`TestStart_ExecFormatError` creates a non-ELF file with mode `0755` so that
`exec.LookPath` succeeds while `cmd.Start()` fails with "exec format error". **The
`cmd.Start()` error block (lines 145–155) is already covered; those lines are NOT
part of the 8 missing.**

### Uncovered Lines (Exact)

All uncovered/partial lines are within `Start()`, introduced by the `os.Pipe()` refactor.
Line numbers verified from `provider.go` as of the current commit:

| Line | Code | Codecov Status |
|---|---|---|
| 132 | `if err != nil {` (stdout pipe guard) | PARTIAL — only false branch taken |
| 133 | `return fmt.Errorf("cloudflare: stdout pipe: %w", err)` | MISSING |
| 136 | `if err != nil {` (stderr pipe guard) | PARTIAL — only false branch taken |
| 137 | `_ = stdoutR.Close()` (cleanup before stderr-pipe error return) | MISSING |
| 138 | `_ = stdoutW.Close()` (cleanup before stderr-pipe error return) | MISSING |
| 139 | `return fmt.Errorf("cloudflare: stderr pipe: %w", err)` | MISSING |
| 161 | `logger.Log().WithFields(logrus.Fields{` (stdoutW close-error log) | MISSING |
| 163 | `}).Error("cloudflare: failed to close stdout write end")` | MISSING |
| 166 | `logger.Log().WithFields(logrus.Fields{` (stderrW close-error log) | MISSING |
| 168 | `}).Error("cloudflare: failed to close stderr write end")` | MISSING |

> **State-management observation (out of scope):** When `os.Pipe()` fails at line
> 131 or 135, the code has already set `p.state = TunnelStateConnecting` and
> `p.done = make(chan struct{})` (lines 124–125) but returns without resetting
> them. The new tests assert the *actual* behavior (state remains
> `TunnelStateConnecting`). Correcting that state leak is outside the scope of
> this patch.

---

## 3. Technical Specifications

### 3.1 Source Changes — `provider.go`

Two `var` declarations are added as a commented block immediately after the import
statement and before the first type declaration, matching the placement in
`caddy/manager.go`.

#### Hook 1 — `osPipe`

```go
// Test hooks to allow overriding OS functions in unit tests.
var (
    // osPipe wraps os.Pipe to allow simulating pipe-creation failures.
    osPipe = os.Pipe
    // closeWriteFile wraps (*os.File).Close for the pipe write-ends closed
    // after cmd.Start() succeeds. Allows simulating close errors in tests.
    closeWriteFile = func(f *os.File) error { return f.Close() }
)
```

#### Call-site replacements

Only four lines in `Start()` change. All other close calls remain direct.

| Original call | Replacement | Location |
|---|---|---|
| `stdoutR, stdoutW, err := os.Pipe()` | `stdoutR, stdoutW, err := osPipe()` | line 131 |
| `stderrR, stderrW, err := os.Pipe()` | `stderrR, stderrW, err := osPipe()` | line 135 |
| `if err := stdoutW.Close(); err != nil {` | `if err := closeWriteFile(stdoutW); err != nil {` | line 160 |
| `if err := stderrW.Close(); err != nil {` | `if err := closeWriteFile(stderrW); err != nil {` | line 165 |

The four `_ = *.Close()` calls inside the `cmd.Start()` error block (lines
146–149) are **not modified** — they are already covered by `TestStart_ExecFormatError`
and perform cleanup-on-failure semantics that do not need a test hook.

### 3.2 Injection Signature Contract

```
osPipe         func() (*os.File, *os.File, error)  // identical to os.Pipe
closeWriteFile func(*os.File) error                 // wraps file.Close()
```

Tests save and restore via `t.Cleanup` (not `defer`), which is the project-standard
approach for test hook teardown:

```go
orig := osPipe
t.Cleanup(func() { osPipe = orig })
osPipe = func() (*os.File, *os.File, error) { ... }
```

None of the three new tests call `t.Parallel()` — consistent with all existing tests
in `coverage_test.go`.

### 3.3 New Test Specifications — `coverage_test.go`

All three tests live in `package cloudflare` (same package as source), consistent with
the existing test files.

#### Shared setup — fake binary

Tests 1 and 2 need `exec.LookPath` to succeed (so execution reaches `osPipe()`) but
`osPipe()` to fail before `cmd.Start()` is ever called. The cleanest approach reuses
the fake-binary pattern from `TestStart_ExecFormatError`:

```go
dir := t.TempDir()
fakeBin := filepath.Join(dir, "cloudflared")
require.NoError(t, os.WriteFile(fakeBin, []byte("not elf"), 0755))
```

Setting `p.binaryPath = fakeBin` makes `exec.LookPath(fakeBin)` succeed (absolute
path, file exists, mode `0755`). The binary is never launched because `osPipe()`
returns an error before `cmd.Start()`.

---

#### Test 1 — `TestStart_StdoutPipeError`

**Target lines:** 132 (true branch), 133

```go
func TestStart_StdoutPipeError(t *testing.T) {
    dir := t.TempDir()
    fakeBin := filepath.Join(dir, "cloudflared")
    require.NoError(t, os.WriteFile(fakeBin, []byte("not elf"), 0755))

    p := &CloudflareTunnelProvider{
        binaryPath: fakeBin,
        creds:      cfCredentials{TunnelToken: "tok"},
        buf:        hecate.NewRingBuffer(1000),
    }

    orig := osPipe
    t.Cleanup(func() { osPipe = orig })
    osPipe = func() (*os.File, *os.File, error) {
        return nil, nil, errors.New("simulated stdout pipe failure")
    }

    err := p.Start(context.Background())

    require.Error(t, err)
    assert.Contains(t, err.Error(), "stdout pipe")
    assert.Equal(t, hecate.TunnelStateConnecting, p.Status())
}
```

**Why `TunnelStateConnecting`:** `p.state` is set to `TunnelStateConnecting` at line 124
before `osPipe()` is called. The error return at line 133 exits without resetting state.

---

#### Test 2 — `TestStart_StderrPipeError`

**Target lines:** 136 (true branch), 137, 138, 139

```go
func TestStart_StderrPipeError(t *testing.T) {
    dir := t.TempDir()
    fakeBin := filepath.Join(dir, "cloudflared")
    require.NoError(t, os.WriteFile(fakeBin, []byte("not elf"), 0755))

    p := &CloudflareTunnelProvider{
        binaryPath: fakeBin,
        creds:      cfCredentials{TunnelToken: "tok"},
        buf:        hecate.NewRingBuffer(1000),
    }

    calls := 0
    origPipe := osPipe
    t.Cleanup(func() { osPipe = origPipe })
    osPipe = func() (*os.File, *os.File, error) {
        calls++
        if calls == 1 {
            return origPipe() // first call (stdout) succeeds — returns real *os.File pair
        }
        return nil, nil, errors.New("simulated stderr pipe failure")
    }

    err := p.Start(context.Background())

    require.Error(t, err)
    assert.Contains(t, err.Error(), "stderr pipe")
    assert.Equal(t, hecate.TunnelStateConnecting, p.Status())
}
```

**Why `origPipe()` on the first call:** The stderr-pipe error block (lines 137–138)
calls `_ = stdoutR.Close()` and `_ = stdoutW.Close()`. Those variables must be real
`*os.File` values or the close calls panic on nil. Delegating the first invocation to
the real `origPipe()` returns a valid pair.

---

#### Test 3 — `TestStart_WriteEndCloseErrors`

**Target lines:** 161, 163 (stdoutW close-error log), 166, 168 (stderrW close-error log)

```go
func TestStart_WriteEndCloseErrors(t *testing.T) {
    trueBin, err := exec.LookPath("true")
    require.NoError(t, err, "/bin/true must be available on test host")

    p := &CloudflareTunnelProvider{
        binaryPath: trueBin,
        creds:      cfCredentials{TunnelToken: "tok"},
        buf:        hecate.NewRingBuffer(1000),
    }

    origClose := closeWriteFile
    t.Cleanup(func() { closeWriteFile = origClose })
    closeWriteFile = func(f *os.File) error {
        _ = f.Close() // physically close to unblock scanner goroutines (see note)
        return errors.New("simulated write-end close error")
    }

    startErr := p.Start(context.Background())

    require.NoError(t, startErr, "close errors are logged, not returned from Start()")

    // Wait for the process to exit and the done channel to close.
    select {
    case <-p.done:
    case <-time.After(5 * time.Second):
        t.Fatal("timed out waiting for cloudflared goroutines to exit")
    }
}
```

**Why the hook must physically close the file:** The scanner goroutines
(`bufio.Scanner` reading `stdoutR` / `stderrR`) block until the write ends are closed
and the child exits. `/bin/true` exits immediately, closing the child's inherited
write-end copies. The parent's write-end copies (`stdoutW`, `stderrW`) must also be
closed for the scanners to see EOF. If the injected `closeWriteFile` only returns an
error without calling `f.Close()`, the parent write-end reference remains open
indefinitely and the goroutines never unblock — causing a test deadlock. Calling
`f.Close()` inside the hook closes the fd while still returning the forced error that
triggers the logger branches.

**Both write ends in one test:** `closeWriteFile` is called for `stdoutW` first, then
`stderrW`. A single injected function that always errors covers both logger branches.

### 3.4 Data-Flow Summary

```
Start()
 │
 ├─ exec.LookPath ──────────────────── already covered
 ├─ p.state = TunnelStateConnecting
 ├─ osPipe() ← [hook 1]
 │   └─ error → return "stdout pipe" ← TEST 1 covers 132(true), 133
 ├─ osPipe() ← [hook 1, 2nd call]
 │   └─ error → close stdout r/w → return "stderr pipe" ← TEST 2 covers 136(true), 137-139
 ├─ cmd.Start()
 │   └─ error → close all 4 fds → set TunnelStateError ← TestStart_ExecFormatError (existing)
 ├─ closeWriteFile(stdoutW) ← [hook 2]
 │   └─ error → logger.Error("failed to close stdout write end") ← TEST 3 covers 161, 163
 ├─ closeWriteFile(stderrW) ← [hook 2]
 │   └─ error → logger.Error("failed to close stderr write end") ← TEST 3 covers 166, 168
 └─ p.state = TunnelStateConnected ── already covered
```

### 3.5 Edge Cases and Constraints

| Scenario | Handled by |
|---|---|
| `exec.LookPath` fails | `TestStart_BinaryNotFound` (existing) |
| `cmd.Start()` fails (exec format error) | `TestStart_ExecFormatError` (existing) |
| `osPipe()` fails on first call | `TestStart_StdoutPipeError` (new) |
| `osPipe()` fails on second call | `TestStart_StderrPipeError` (new) |
| `closeWriteFile()` returns error | `TestStart_WriteEndCloseErrors` (new) |
| Close calls in `cmd.Start()` error block (lines 146–149) | `TestStart_ExecFormatError` (existing, unchanged) |
| Concurrent hook mutation | Not applicable — tests are sequential, no `t.Parallel()` |

---

## 4. Implementation Plan

### Phase 1 — Playwright Tests

Not applicable. This is a Go backend unit-test coverage fix with no UI surface area.

### Phase 2 — Source Changes in `provider.go`

| Task | Change | Estimated Complexity |
|---|---|---|
| 2.1 | Add `var ( osPipe = os.Pipe; closeWriteFile = func... )` block after imports | XS |
| 2.2 | Replace `os.Pipe()` → `osPipe()` at lines 131, 135 | XS |
| 2.3 | Replace `stdoutW.Close()` → `closeWriteFile(stdoutW)` at line 160 | XS |
| 2.4 | Replace `stderrW.Close()` → `closeWriteFile(stderrW)` at line 165 | XS |

Total diff: approximately 10 lines added, 2 lines modified.

### Phase 3 — New Tests in `coverage_test.go`

| Task | Test Name | Target Uncovered Lines | Complexity |
|---|---|---|---|
| 3.1 | `TestStart_StdoutPipeError` | 132 (true branch), 133 | S |
| 3.2 | `TestStart_StderrPipeError` | 136 (true branch), 137, 138, 139 | S |
| 3.3 | `TestStart_WriteEndCloseErrors` | 161, 163, 166, 168 | M (requires `p.done` channel wait) |

Required imports for `coverage_test.go` (confirm these are already present or add):

```go
"errors"
"os/exec"
"time"
```

### Phase 4 — Integration and Testing

| Task | Command | Pass Condition |
|---|---|---|
| 4.1 | `go test -race -count=1 ./backend/internal/hecate/providers/cloudflare/...` | All tests green, no data races |
| 4.2 | `go test -coverprofile=cover.out ./backend/internal/hecate/providers/cloudflare/... && go tool cover -func=cover.out \| grep Start` | Lines 133, 137–139, 161, 163, 166, 168 show non-zero hit counts |
| 4.3 | `bash scripts/go-test-coverage.sh` (generates `backend/coverage.txt`) | Package coverage does not drop below project threshold |
| 4.4 | `bash scripts/local-patch-report.sh` | `test-results/local-patch-report.md` reports ≥ 90% patch coverage for `provider.go` |

### Phase 5 — Documentation and Deployment

No user-facing documentation, API surface, database schema, or migration changes.

---

## 5. Acceptance Criteria

| # | Criterion | Verification |
|---|---|---|
| AC-1 | `go test -race -count=1 ./backend/internal/hecate/providers/cloudflare/...` exits 0 | CI / local |
| AC-2 | `TestStart_StdoutPipeError` passes, error message contains `"stdout pipe"` | Test output |
| AC-3 | `TestStart_StderrPipeError` passes, error message contains `"stderr pipe"` | Test output |
| AC-4 | `TestStart_WriteEndCloseErrors` passes within 5 s (no deadlock) | Test output |
| AC-5 | Codecov patch coverage for `provider.go` ≥ 90% | Codecov PR comment |
| AC-6 | No existing tests in `provider_test.go` or `coverage_test.go` regress | CI |
| AC-7 | `var osPipe` and `var closeWriteFile` are in a single commented `var (...)` block before the first type declaration, matching `caddy/manager.go` style | Code review |
| AC-8 | No `t.Parallel()` in the three new tests | Code review |
| AC-9 | GORM security scan gate is skipped (no model changes match trigger matrix) | CI / `scripts/scan-gorm-security.sh --report` |

---

## 6. Commit Slicing Strategy

### Decision

**Single PR · Single Commit.** All changes are confined to two files within one package.
There is no user-facing API surface, no schema change, and no cross-domain impact.
A single atomic commit is faster to review and trivially reversible.

### Commit 1 of 1

```
test(hecate/cloudflare): add os.Pipe and write-close test hooks for Start() coverage

Introduce two package-level function-variable test hooks in provider.go
(var osPipe and var closeWriteFile) following the project-standard pattern
established in caddy/manager.go. Replace the two os.Pipe() call sites and the
two post-cmd.Start() write-end close calls with the hook variables.

Add three targeted test functions in coverage_test.go to exercise the
previously unreachable error branches introduced by the os.Pipe() refactor:
- TestStart_StdoutPipeError: stdout pipe creation failure
- TestStart_StderrPipeError: stderr pipe creation failure with stdout cleanup
- TestStart_WriteEndCloseErrors: write-end close error log branches

Resolves Codecov patch coverage regression on provider.go: 54.54% → ≥90%.
```

| Field | Value |
|---|---|
| Scope | `backend/internal/hecate/providers/cloudflare/` |
| Files | `provider.go` (2 new vars + 4 call-site edits), `coverage_test.go` (3 new test functions) |
| Dependencies | None |
| Validation gate | `go test -race -count=1 ./backend/internal/hecate/providers/cloudflare/...` exits 0 |

### Rollback

`git revert <sha>` is sufficient. No migration, no deployed artifact, no downstream
package references to the new `var` symbols (they are unexported and package-internal).

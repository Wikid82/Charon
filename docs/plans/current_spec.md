# Fix: Race Condition in `TestStart_CapturesStdoutOutput`

## Introduction

### Overview

`TestStart_CapturesStdoutOutput` in
`backend/internal/hecate/providers/cloudflare/coverage_test.go` fails because
the `Start()` method in `provider.go` uses `cmd.StdoutPipe()` /
`cmd.StderrPipe()`. Go's `exec.Cmd` implementation silently closes the **read
ends** of those pipes inside `cmd.Wait()`. The monitor goroutine calls
`cmd.Wait()` first, which races with the scanner goroutines that are still
trying to drain the OS pipe buffer — causing an empty ring buffer and a failing
assertion.

### Objectives

1. Eliminate the pipe-closing race condition introduced by `StdoutPipe()` /
   `StderrPipe()`.
2. Preserve all existing behaviour (line-by-line scanning, ring buffer writes,
   `scanWg`, state transitions, `done` channel semantics).
3. Fix in one self-contained commit; no API surface changes, no new types.

---

## Research Findings

### Files Examined

| File | Role |
|---|---|
| `backend/internal/hecate/providers/cloudflare/provider.go` | `Start()` implementation — the defective code |
| `backend/internal/hecate/providers/cloudflare/coverage_test.go` | Failing test `TestStart_CapturesStdoutOutput` (lines 110–143) |
| `backend/internal/hecate/providers/cloudflare/provider_test.go` | Remaining provider tests; `validCredsJSON()` helper |
| `backend/internal/hecate/ring_buffer.go` | `RingBuffer` — `Write()` drops silently when `closed == true`; `ReadAll()` works after `Close()` |

### Go `exec.Cmd` Pipe-Lifecycle Mechanics

`cmd.StdoutPipe()` in Go's standard library executes:

```go
pr, pw, _ := os.Pipe()
c.Stdout = pw
c.closeAfterStart = append(c.closeAfterStart, pw)  // pw closed after fork
c.closeAfterWait  = append(c.closeAfterWait,  pr)  // pr closed inside Wait()
return pr, nil
```

`cmd.Wait()` calls `closeDescriptors(c.closeAfterWait)` which **closes `pr`**
(the read end) the moment the child process exits — regardless of whether any
goroutine is still reading from it.

When `cmd.Stdout` is assigned an `*os.File` directly, the `writerDescriptor`
helper returns the file as-is and adds it to **neither** `closeAfterStart` nor
`closeAfterWait`:

```go
if f, ok := w.(*os.File); ok {
    return f, nil  // no lifecycle management
}
```

This is the property the fix exploits.

### Root Cause — Step-by-Step Race

```
[goroutine: monitor]             [goroutine: stdout scanner]
cmd.Wait() ──▶ child exits
closeAfterWait ──▶ pr.Close()   ← stdoutPipe read-end now CLOSED
                                  bufio.Scanner.Scan() → EBADF / EOF before read
                                  scanWg.Done()  (0 bytes written to ring buffer)
defer: scanWg.Wait()            ← already done; buffer is empty
close(p.done)
                                  test: p.buf.ReadAll() == []  ✗
```

On a loaded CI runner (or any run where the Go scheduler prioritises the monitor
goroutine), `cmd.Wait()` fires and closes `stdoutPipe` before the scanner
goroutine executes a single `Read()`. The OS pipe buffer still holds
`"tunnel run\n"` but the file descriptor is already invalid.

The test's 1-second polling loop (`coverage_test.go` lines 133–139) cannot
recover because the scanner goroutine has already exited without writing.

### Why `ring_buffer.go` Is Not the Cause

`RingBuffer.Write()` drops writes only when `rb.closed == true`.
`RingBuffer.Close()` sets that flag but does **not** clear the buffer.
`RingBuffer.ReadAll()` works correctly after `Close()`. The bug is entirely in
the pipe-lifecycle management inside `Start()`.

---

## Technical Specification

### API / Type Surface

No changes to exported types, function signatures, or the `hecate.TunnelProvider`
interface.

### Algorithm Change — `Start()` in `provider.go`

Replace `cmd.StdoutPipe()` / `cmd.StderrPipe()` with explicit `os.Pipe()` pairs.
Assign the write ends directly to `cmd.Stdout` / `cmd.Stderr`. Close the write
ends in the **parent** immediately after `cmd.Start()` succeeds. Pass the read
ends to the scanner goroutines, which close them via `defer`.

#### Before (buggy)

```go
stdoutPipe, err := cmd.StdoutPipe()
if err != nil {
    return fmt.Errorf("cloudflare: stdout pipe: %w", err)
}
stderrPipe, err := cmd.StderrPipe()
if err != nil {
    return fmt.Errorf("cloudflare: stderr pipe: %w", err)
}

if err := cmd.Start(); err != nil {
    p.mu.Lock()
    p.state = hecate.TunnelStateError
    close(p.done)
    p.mu.Unlock()
    return fmt.Errorf("cloudflare: start cloudflared: %w", err)
}

p.mu.Lock()
p.cmd = cmd
p.state = hecate.TunnelStateConnected
p.mu.Unlock()

var scanWg sync.WaitGroup

scanWg.Add(1)
go func() {
    defer scanWg.Done()
    s := bufio.NewScanner(stdoutPipe)
    for s.Scan() {
        p.buf.Write(s.Text())
    }
}()

scanWg.Add(1)
go func() {
    defer scanWg.Done()
    s := bufio.NewScanner(stderrPipe)
    for s.Scan() {
        p.buf.Write(s.Text())
    }
}()
```

#### After (fixed)

```go
// Use os.Pipe() instead of cmd.StdoutPipe() / cmd.StderrPipe() so that
// cmd.Wait() never closes the read ends before the scanner goroutines drain
// them. StdoutPipe adds the read end to closeAfterWait; a bare *os.File does
// not, so the scanner goroutines control the lifetime of the read ends.
stdoutR, stdoutW, err := os.Pipe()
if err != nil {
    return fmt.Errorf("cloudflare: stdout pipe: %w", err)
}
stderrR, stderrW, err := os.Pipe()
if err != nil {
    _ = stdoutR.Close()
    _ = stdoutW.Close()
    return fmt.Errorf("cloudflare: stderr pipe: %w", err)
}
cmd.Stdout = stdoutW
cmd.Stderr = stderrW

if err := cmd.Start(); err != nil {
    _ = stdoutR.Close()
    _ = stdoutW.Close()
    _ = stderrR.Close()
    _ = stderrW.Close()
    p.mu.Lock()
    p.state = hecate.TunnelStateError
    close(p.done)
    p.mu.Unlock()
    return fmt.Errorf("cloudflare: start cloudflared: %w", err)
}

// Close the parent's write ends. The child holds its own inherited copies;
// when the child exits the OS closes those, and the scanners detect EOF.
_ = stdoutW.Close()
_ = stderrW.Close()

p.mu.Lock()
p.cmd = cmd
p.state = hecate.TunnelStateConnected
p.mu.Unlock()

var scanWg sync.WaitGroup

scanWg.Add(1)
go func() {
    defer scanWg.Done()
    defer stdoutR.Close() //nolint:errcheck
    s := bufio.NewScanner(stdoutR)
    for s.Scan() {
        p.buf.Write(s.Text())
    }
}()

scanWg.Add(1)
go func() {
    defer scanWg.Done()
    defer stderrR.Close() //nolint:errcheck
    s := bufio.NewScanner(stderrR)
    for s.Scan() {
        p.buf.Write(s.Text())
    }
}()
```

The **monitor goroutine** and the `Stop()` method are **unchanged**.

### Correctness Table

| Step | Old behaviour | New behaviour |
|---|---|---|
| `cmd.Start()` succeeds | exec adds `pr` to `closeAfterWait` | exec receives bare `*os.File`; not lifecycle-managed |
| Parent write-end | exec closes via `closeAfterStart` | Parent closes explicitly after `Start()` |
| `cmd.Wait()` returns | Closes `pr` → scanner gets EBADF | Read end untouched; scanner continues reading |
| Scanner reads "tunnel run" | May miss it (race) | Reads reliably before `scanWg.Done()` |
| `scanWg.Wait()` in defer | Scanner already failed | Scanner finished; buffer has the line |
| `close(p.done)` | Buffer is empty → test fails | Buffer contains "tunnel run" → test passes |

### Import Changes

No new imports. `os` is already imported in `provider.go` (used by `os.Environ()`).

---

## Implementation Plan

### Phase 1 — Fix `provider.go`

**Task 1.1** — Replace `StdoutPipe` / `StderrPipe` with `os.Pipe`

- **File**: `backend/internal/hecate/providers/cloudflare/provider.go`
- **Function**: `Start(ctx context.Context) error`
- **Change**: As shown in the "After" block above — replaces roughly 20 lines.
- **Imports affected**: None.
- **Complexity**: Low — surgical line swap, zero structural changes.

**Task 1.2** — Verify no other references to the old pipe variables

- Confirm `stdoutPipe` / `stderrPipe` identifiers do not appear outside of
  `Start()` (they are local variables; this is a sanity check).

### Phase 2 — Validation

**Task 2.1** — Run the primary failing test in high-repetition mode

```bash
cd /projects/Charon/backend
go test ./internal/hecate/providers/cloudflare/... \
    -run TestStart_CapturesStdoutOutput -v -count=10
```

Expected: 10/10 passes.

**Task 2.2** — Run the full package suite with the race detector

```bash
cd /projects/Charon/backend
go test ./internal/hecate/providers/cloudflare/... -race -count=3
```

All of the following must pass:

| Test | Guards |
|---|---|
| `TestStart_CapturesStdoutOutput` | Primary fix target |
| `TestStart_ExecFormatError` | Error path — pipe cleanup on `cmd.Start()` failure |
| `TestStop_WhenProcessAlreadyExited` | `done` channel semantics |
| `TestStop_WithNilDone` | `nil` done guard |
| `TestNewCloudflareProvider_*` | Constructor validation — unaffected |
| `TestListTunnels_*` | API client — unaffected |
| `TestCreateTunnel_*` | API client — unaffected |
| `TestGenerateCloudflaredConfig_*` | Config generation — unaffected |

**Task 2.3** — High-repetition race-detector run on `Start` tests

```bash
cd /projects/Charon/backend
go test ./internal/hecate/providers/cloudflare/... \
    -run TestStart -race -count=50
```

All 50 runs must pass. Residual data races on `p.buf`, `p.done`, or file
descriptors would be surfaced here.

**Task 2.4** — Full backend coverage gate

```bash
cd /projects/Charon
bash scripts/go-test-coverage.sh
```

Coverage must not drop below the project threshold. The fix replaces one code
path with an equivalent one; no new uncovered branches are introduced.

---

## Acceptance Criteria

- [ ] `TestStart_CapturesStdoutOutput` passes 50/50 with `-race -count=50`.
- [ ] All other tests in `./internal/hecate/providers/cloudflare/...` pass.
- [ ] `go vet ./internal/hecate/providers/cloudflare/...` reports no issues.
- [ ] `go test ./...` in `backend/` exits 0.
- [ ] The race detector reports no data races in the package.

---

## Commit Slicing Strategy

**Decision**: Single PR, single commit. This is a surgical bug fix touching one
function in one file. No database, API, frontend, or test-file changes.

### Commit 1 (only commit)

| Field | Value |
|---|---|
| **Scope** | `backend/internal/hecate/providers/cloudflare/provider.go` |
| **Type** | `fix` |
| **Subject** | `fix(hecate/cloudflare): replace StdoutPipe with os.Pipe to fix stdout capture race` |
| **Files changed** | `provider.go` only |
| **Dependencies** | None |
| **Validation gate** | `go test ./internal/hecate/providers/cloudflare/... -race -count=50` passes 50/50 |

**Commit message body**:

```
cmd.StdoutPipe() and cmd.StderrPipe() register the pipe read ends in
exec.Cmd.closeAfterWait. When cmd.Wait() returns it closes those file
descriptors, racing with the scanner goroutines that are still draining
the OS pipe buffer. On a loaded CI runner cmd.Wait() wins the race,
the scanners receive EBADF, and the ring buffer stays empty.

Replace both calls with os.Pipe(). Assign the write ends directly to
cmd.Stdout and cmd.Stderr (as *os.File, exec does not lifecycle-manage
them). Close the write ends in the parent immediately after cmd.Start()
so scanners see EOF when the child exits. The scanner goroutines own
the read ends and close them via defer after draining.

Fixes TestStart_CapturesStdoutOutput.
```

**Rollback**: `git revert <sha>` — fully isolated, zero downstream impact.

# Fix: Race Condition in `HandleWebSocket` Causes TempDir Cleanup Failure

## 1. Introduction

### Overview

`TestOrthrusServer_HandleWebSocket_ValidToken_UpgradesConnection` fails intermittently in CI with:

```
testing.go:1464: TempDir RemoveAll cleanup: unlinkat
  /tmp/TestOrthrusServer_HandleWebSocket_ValidToken_UpgradesConnection611908746/001:
  directory not empty
```

This is a **data race**: the `HandleWebSocket` gin handler writes to the SQLite
database concurrently with Go's test-framework cleanup that tries to remove the
database directory. The race is triggered by a sequencing bug — `sessions.Store`
is called before the DB update and `wg.Add(1)`, so the test can exit (and cleanup
can start) while the handler goroutine is still writing to the database.

### Objectives

1. Fix the race by reordering three statements in `HandleWebSocket`.
2. No test changes are required.
3. Ship as a single commit in a single PR.

---

## 2. Research Findings

### Directory Layout

```
backend/internal/orthrus/
├── server.go               ← THE BUG IS HERE
├── server_test.go          ← setupServerTestDB, setupTestCA, heartbeat tests
├── server_coverage_test.go ← failing test + WS integration tests
├── session.go              ← AgentSession, StartDockerProxy, runProxyListener
└── ca.go                   ← InternalCA (synchronous I/O only; no goroutines)
```

### TempDir Numbering

`t.TempDir()` returns numbered sub-directories under the test's base temp dir:

| Call order | `t.TempDir()` call site | Directory | Contents |
|-----------|------------------------------------------------------|-----------|----------------------------------------|
| 1st | `setupServerTestDB` → `filepath.Join(t.TempDir(), "orthrus_test.db")` | `001` | SQLite DB file (actively written to) |
| 2nd | `setupTestCA` → `NewInternalCA(t.TempDir())` | `002` | `keys/hecate-ca.key`, `keys/hecate-ca.crt` (static after creation) |

The `001` directory is the only one that is written to after construction.

### Cleanup Registration Order

Cleanups are registered in this order (LIFO execution = reverse):

| Registration order | Cleanup action | LIFO execution order |
|-------------------|----------------------------------------|----------------------|
| 1st | `t.TempDir()` cleanup for `001` (DB) | 5th (last) ← **FAILS** |
| 2nd | `t.Cleanup(sqlDB.Close)` | 4th |
| 3rd | `t.TempDir()` cleanup for `002` (CA) | 3rd |
| 4th | `t.Cleanup(ts.Close)` | 2nd |
| 5th | `t.Cleanup(srv.Stop)` | 1st (first) |

Note: `defer conn.Close()` is a defer (not a `t.Cleanup`), so it runs when the
test function returns, **before** any `t.Cleanup` runs.

### Relevant Code — `HandleWebSocket` (current, buggy order)

```go
// backend/internal/orthrus/server.go — current order

s.sessions.Store(agent.UUID, session)   // ← (A) test polling loop exits here

now := time.Now()
if err := s.db.Model(agent).Updates(map[string]interface{}{ // ← (B) DB write
    "status":    models.OrthrusStatusOnline,
    "last_seen": &now,
}).Error; err != nil { ... }

s.wg.Add(1)                             // ← (C) WG increment
go func() {
    defer s.wg.Done()
    s.watchHeartbeat(agent.UUID, session)
}()
```

The test polling loop in `TestOrthrusServer_HandleWebSocket_ValidToken_UpgradesConnection`:

```go
for i := 0; i < 20; i++ {
    _, ok = srv.GetSession("wscov-uuid")
    if ok { break }          // exits as soon as (A) has run
    time.Sleep(20 * time.Millisecond)
}
assert.True(t, ok)
// test function returns here — (B) and (C) may not have run yet
```

### `OrthrusServer.Stop()`

```go
func (s *OrthrusServer) Stop() {
    s.cancel()
    s.sessions.Range(func(key, value any) bool {
        sess := value.(*AgentSession)
        _ = sess.Close()
        s.sessions.Delete(key)
        return true
    })
    s.wg.Wait()    // blocks until all watchHeartbeat goroutines exit
}
```

`Stop` tracks **only** the `watchHeartbeat` goroutines via `s.wg`. It does not
track the `HandleWebSocket` handler itself.

---

## 3. Root Cause Analysis

### The Race Window

```
HandleWebSocket goroutine           Test / cleanup goroutine
(runs inside httptest.Server)
─────────────────────────────────   ─────────────────────────────────────

s.sessions.Store(agent.UUID, ...)
                                    ← test polls GetSession → ok = true
                                    ← test function returns
                                    ← defer conn.Close() runs
                                    ← t.Cleanup LIFO begins
                                    ← srv.Stop() runs:
                                         s.cancel()
                                         sessions.Range → close session
                                         s.wg.Wait()  ← wg = 0!
                                                        returns immediately
s.db.Model(agent).Updates(...)      ← sqlDB.Close() runs      ← RACE
  ↑ SQLite creates/modifies
    journal or WAL files in 001/    ← os.RemoveAll(001)       ← ENOTEMPTY
```

**Why `s.wg.Wait()` returns zero:** `s.wg.Add(1)` at step (C) has not been called
yet. The WaitGroup counter is 0, so `Wait()` returns immediately even though
`HandleWebSocket` is still running and will call `wg.Add(1)` moments later.

**Why files appear after `RemoveAll` started:** SQLite's WAL or rollback-journal
mechanism creates temporary files (`orthrus_test.db-wal`, `orthrus_test.db-shm`,
or `orthrus_test.db-journal`) during an active write transaction.
`os.RemoveAll` on Linux removes visible directory entries in a single pass, then
attempts `rmdir(2)`. If a new file appears between the final scan and the `rmdir`,
the system call returns `ENOTEMPTY`, causing Go to surface the "directory not
empty" error.

### Why the Failure is Intermittent

The race only manifests when the CPU scheduler switches goroutines at precisely
the right moment — between step (A) (`sessions.Store`) and step (C) (`wg.Add`).
Fast machines with many cores make this rare; slow CI runners or GC pauses make
it more common.

### Why the CA Directory (`002`) Is Never the Culprit

`NewInternalCA` writes two static files (`hecate-ca.key`, `hecate-ca.crt`) during
construction and never writes again. No goroutines hold references to `002` after
`setupTestCA` returns. `RemoveAll(002)` always succeeds.

---

## 4. Technical Specification

### Fix: Reorder Three Blocks in `HandleWebSocket`

**File:** `backend/internal/orthrus/server.go`
**Function:** `HandleWebSocket`

Move `s.sessions.Store(agent.UUID, session)` to be the **last** statement in the
success path, after the DB update and after `s.wg.Add(1)` + goroutine start.

#### Before (current)

```go
s.sessions.Store(agent.UUID, session)

now := time.Now()
if err := s.db.Model(agent).Updates(map[string]interface{}{
    "status":    models.OrthrusStatusOnline,
    "last_seen": &now,
}).Error; err != nil {
    logger.Log().WithField("uuid", util.SanitizeForLog(agent.UUID)).
        WithError(err).Warn("orthrus: update agent status failed")
}

logger.Log().WithFields(logrus.Fields{
    "uuid": util.SanitizeForLog(agent.UUID),
    "name": util.SanitizeForLog(agent.Name),
}).Info("orthrus: agent connected")

s.wg.Add(1)
go func() {
    defer s.wg.Done()
    s.watchHeartbeat(agent.UUID, session)
}()
```

#### After (fixed)

```go
now := time.Now()
if err := s.db.Model(agent).Updates(map[string]interface{}{
    "status":    models.OrthrusStatusOnline,
    "last_seen": &now,
}).Error; err != nil {
    logger.Log().WithField("uuid", util.SanitizeForLog(agent.UUID)).
        WithError(err).Warn("orthrus: update agent status failed")
}

logger.Log().WithFields(logrus.Fields{
    "uuid": util.SanitizeForLog(agent.UUID),
    "name": util.SanitizeForLog(agent.Name),
}).Info("orthrus: agent connected")

s.wg.Add(1)
go func() {
    defer s.wg.Done()
    s.watchHeartbeat(agent.UUID, session)
}()

s.sessions.Store(agent.UUID, session)   // moved last
```

#### Why This Fixes the Race

By the time any external caller (test, other handler) observes the session in the
map via `GetSession`, both invariants are guaranteed:

1. **DB write is complete** — no concurrent SQLite I/O can race with cleanup.
2. **`s.wg.Add(1)` has been called** — `srv.Stop()` → `s.wg.Wait()` will block
   until the `watchHeartbeat` goroutine exits before returning control to the
   cleanup sequence.

#### Correctness Under Edge Cases

| Scenario | Behaviour after fix |
|----------|---------------------|
| `srv.Stop()` runs while handler is between DB write and `wg.Add(1)` | `Stop`'s `Range` doesn't find the session (not stored yet); `wg.Wait()` waits for the about-to-be-added goroutine; goroutine exits via `ctx.Done()`; handler then stores the session (harmless entry, no active goroutine) |
| `srv.Stop()` runs while handler is between `wg.Add(1)` and `sessions.Store` | Same as above — goroutine exits, handler stores session after Stop returns |
| Normal path: handler completes before cleanup starts | `sessions.Store` is last; test sees session; cleanup runs in full; `srv.Stop()` correctly waits for goroutine; no race |

The post-Stop zombie-entry edge case does not affect correctness: the
`watchHeartbeat` goroutine exits immediately via `ctx.Done()` (context already
cancelled), so no further DB writes occur after `Stop()` returns.

### No Changes to Tests

The polling loop in `TestOrthrusServer_HandleWebSocket_ValidToken_UpgradesConnection`
and the `assert.Eventually` loops in `TestOrthrusServer_HandleWebSocket_ExternalProxyFails`
and `TestHandleWebSocket_DisplacesExistingSession` continue to work. They observe
the session slightly later in the handler's execution — no timing adjustment needed.

---

## 5. Affected Files

| File | Change | Lines affected |
|------|--------|----------------|
| `backend/internal/orthrus/server.go` | Reorder 3 existing blocks in `HandleWebSocket` | ~5 lines moved |

No new files. No test file changes.

---

## 6. Tests to Validate

Run the orthrus package tests with `-race` and `-count=10` to surface the race
reliably before the fix, and confirm it is gone after:

```bash
cd backend && go test -race -count=10 -run TestOrthrusServer_HandleWebSocket ./internal/orthrus/
```

A clean run (all 10 iterations pass) confirms the fix. Also run the full package:

```bash
cd backend && go test -race -count=5 ./internal/orthrus/...
```

---

## 7. Secondary Observation (Out of Scope)

`session.StartDockerProxy()` starts a `runProxyListener` goroutine that is **not
tracked** by any WaitGroup. `session.Close()` closes the listener, which causes
the goroutine to exit via `net.ErrClosed`, but there is no synchronous wait.
This goroutine does not touch the filesystem or the DB, so it does not cause the
TempDir failure. Tracking it formally (e.g., by adding a `wg sync.WaitGroup` to
`AgentSession`) would be good hygiene but is a separate concern and out of scope
for this targeted bug fix.

---

## 8. Acceptance Criteria

- [ ] `TestOrthrusServer_HandleWebSocket_ValidToken_UpgradesConnection` passes
      consistently with `-race -count=20`.
- [ ] All other tests in `backend/internal/orthrus/...` pass with `-race`.
- [ ] `go vet ./internal/orthrus/...` reports no issues.
- [ ] No new linter findings in the changed function.

---

## 9. Commit Slicing Strategy

### Decision

Single PR, single commit. This is a targeted bug fix touching three consecutive
blocks in one function in one file. No phasing is required.

| | Detail |
|---|---|
| **PR title** | `fix(orthrus): eliminate TempDir race by moving sessions.Store after wg.Add` |
| **Scope** | `backend/internal/orthrus/server.go` only |
| **Risk** | Minimal — pure reorder of existing statements, no logic change |
| **Review size** | < 10 lines moved |

### Commit 1 (the only commit)

**Message:** `fix(orthrus): eliminate TempDir race by moving sessions.Store after wg.Add`

**Body:**
```
sessions.Store was called before the DB update and wg.Add(1) in HandleWebSocket.
The test polling loop exited as soon as the session appeared in the map, starting
cleanup while the handler was still writing to the SQLite database.

srv.Stop() called wg.Wait() with wg=0 (wg.Add had not yet been called), allowing
cleanup to proceed. Concurrent SQLite journal/WAL file creation raced with
os.RemoveAll on the test's TempDir, producing "directory not empty".

Fix: move sessions.Store to be the last statement after the DB update and
wg.Add(1). External observers now see the session only after all setup is
complete.

Fixes: TestOrthrusServer_HandleWebSocket_ValidToken_UpgradesConnection flaky CI
```

**Files changed:** `backend/internal/orthrus/server.go`

**Dependencies:** none

**Validation gate:** `go test -race -count=20 -run TestOrthrusServer_HandleWebSocket ./internal/orthrus/` passes cleanly.

**Rollback:** `git revert <sha>` — trivially reversible, no schema migrations, no side effects.

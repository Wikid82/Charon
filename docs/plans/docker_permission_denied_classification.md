# Classify Moby v0.5.1 Permission-Denied Errors as Docker Connectivity Errors

**Type**: Small, targeted backend hardening fix (single message-classification branch + tests, not a redesign). Branch: dedicated branch off `main`, e.g. `fix/docker-permission-denied-classification`, per standard PR flow. Independent of, and safely mergeable in any order relative to, the unrelated `fix/docker-list-containers-timeout` PR (see §7).

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

## 1. Introduction

### 1.1 Background — chronology and how this was found

This bug was found via a **separate, later investigation** of `.github/logs/charon.log`, distinct in time, scope, and root cause from the earlier, unrelated investigation that produced the bounded-`ListContainers`-timeout fix (tracked in `docs/plans/current_spec.md`). That earlier investigation explicitly **ruled out permissions** as the cause of *its* reported symptom (a blank container list caused by a slow-daemon race against Caddy's ~30s reverse-proxy timeout): "No `EACCES`/`EPERM`/`ENOENT` ever appears in logs" was true *for the log window that first investigation examined*.

This investigation is not a continuation of that one. It began independently, from a full-log sweep of `.github/logs/charon.log` (42,322 lines, confirmed tracked via `git ls-files`) for any occurrence of `permission denied`, `EACCES`, or `EPERM` across the *entire* file — not just the window already examined for the timeout bug. That sweep surfaced a genuinely distinct incident window (2026-07-30 13:37–14:12) that the earlier investigation's evidence did not cover, containing 8 occurrences of a fast (single-digit-to-low-double-digit millisecond), permission-denied-flavored failure with **no** `context canceled` / `context deadline exceeded` text anywhere nearby — i.e., a different failure mode entirely from the slow-daemon timeout race.

**Bug summary**: `github.com/moby/moby/client@v0.5.1` (the version pinned in `backend/go.mod:15` / `go.sum:119-120`) produces a specific error message text for OS-level permission-denied failures connecting to the Docker socket. That message evades `DockerService`'s existing `isDockerConnectivityError` classifier (`backend/internal/services/docker_service.go:196-251`) and falls through to the generic, unhelpful `500`/`"Failed to list containers"` path instead of the existing, more actionable `DockerUnavailableError`/`503` path that already exists specifically for this class of problem.

### 1.2 Objective

Add message-text-based classification for moby v0.5.1's permission-denied connection error to `isDockerConnectivityError`, so it routes through the existing `DockerUnavailableError`/503 path (with the existing permissions-oriented user guidance) instead of the generic 500 fallback — with no change to the handler layer, which already handles `DockerUnavailableError` correctly.

### 1.3 Explicitly out of scope

- The bounded-timeout fix for slow `ListContainers` responses (`docs/plans/current_spec.md`) — unrelated bug, unrelated root cause, ships in its own PR. See §7 for the relationship between the two.
- `.docker/docker-entrypoint.sh` (GID-remapping logic) — not implicated, not touched.
- Any Docker Compose file, rootless Docker / subgid host configuration.
- The `os.ErrNotExist` (`ENOENT`, missing-socket) branch of the moby client — confirmed unaffected, see §2.2.
- Any redesign of `DockerService`, its constructor, or its remote-host code path beyond the single classification-check addition.
- Frontend code changes — none required; this fix only changes which existing, already-handled error type (`*DockerUnavailableError`) a given error is classified as. The handler's existing 503 response path and `useDocker.ts`/`ProxyHostForm.tsx`'s existing rendering of that response are unaffected.

## 2. Research Findings

### 2.1 Primary-source verification performed

**A. Log file (`.github/logs/charon.log`, confirmed present via `git ls-files`, 42,322 lines):**

`grep -n -i "permission denied\|EACCES\|EPERM"` and a direct timestamp-window search both confirm 4 distinct incident pairs inside and around the 2026-07-30 13:37–14:12 window (all `-04:00`), each an `error`-level service log immediately followed by a `500`-status GIN access log for the same `request_id` on `path":"/api/v1/docker/containers"`:

```
33154: {"error":"failed to list containers: permission denied while trying to connect to the docker API at unix:///var/run/docker.sock","host":"local","level":"error","msg":"failed to list containers","request_id":"f06ce27f-...","server_id":"","time":"2026-07-30T13:37:14-04:00"}
33155: {"client":"172.19.0.1","latency":"17.578502ms",...,"path":"/api/v1/docker/containers","status":500,"time":"2026-07-30T13:37:14-04:00"}
```
(repeated at 13:37:15, 13:39:42, 13:39:43, 13:42:23, 13:42:24, 14:12:06, 14:12:07 — 8 occurrences total, all with the identical error string and all `status:500`, all latencies 1.9ms–17.6ms).

This is **fast** (single-digit/low-double-digit milliseconds) and textually distinct from the unrelated timeout bug's `"context canceled"` / 30s-duration evidence (documented in `docs/plans/current_spec.md` §1.1) — confirming these are two genuinely separate incidents. No `context canceled`/`context deadline exceeded` text appears anywhere near this window's permission-denied lines.

**B. Handler code cross-check (`backend/internal/api/handlers/docker_handler.go:103-121`):**

```go
containers, err := h.dockerService.ListContainers(c.Request.Context(), host)
if err != nil {
    var unavailableErr *services.DockerUnavailableError
    if errors.As(err, &unavailableErr) {
        ...
        log.WithFields(...).Warn("docker unavailable")      // line 111
        c.JSON(http.StatusServiceUnavailable, gin.H{...})   // 503
        return
    }
    log.WithFields(...).Error("failed to list containers")  // line 119 — matches log evidence exactly
    c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list containers"})  // line 120 — 500
    return
}
```
The log evidence's `"level":"error"` + `"msg":"failed to list containers"` matches **line 119** verbatim — the non-`DockerUnavailableError` fallback branch — not line 111's `Warn`/503 branch. This proves the permission-denied error in production was **not** classified as a `DockerUnavailableError` and fell through to the generic 500.

**C. Vendored moby client source** (`$(go env GOMODCACHE)/github.com/moby/moby/client@v0.5.1/`, confirmed as the actual pinned version via `backend/go.mod:15` and `go.sum:119-120` — `github.com/moby/moby/client v0.5.1`):

`request.go:168-172` (inside `doRequest`, called from `sendRequest`, called from `get()`, called by `ContainerList`):

```go
if errors.Is(err, os.ErrPermission) {
    // Don't include request errors (Get "http://%2Fvar%2Frun%2Fdocker.sock/v1.51/version"),
    // which are irrelevant if we weren't able to connect.
    return nil, errConnectionFailed{fmt.Errorf("permission denied while trying to connect to the docker API at %v", cli.host)}
}
```

`errors.go:14-26`:

```go
type errConnectionFailed struct {
    error
}
func (e errConnectionFailed) Error() string { return e.error.Error() }
func (e errConnectionFailed) Unwrap() error { return e.error }   // <-- line 24-26: DOES unwrap correctly
```

**Root cause, precisely stated**: `errConnectionFailed.Unwrap()` works correctly — that is not the defect. The actual defect is one level deeper: `request.go:171`'s `fmt.Errorf` call never references the original OS/syscall error (`err`) as an argument at all — only `cli.host` (a string) is substituted via `%v`. So the chain is: `errConnectionFailed` → (unwraps fine to) → a `fmt.Errorf`-built error with **no further `Unwrap()`** (a dead end) — the underlying `syscall.EACCES`/`EPERM` is discarded *before* `errConnectionFailed` is even constructed, permanently unrecoverable from the error chain. Consequently, message-text matching is the only viable detection mechanism for this specific case (confirmed live in §2.1.D below) — there is no `errors.As`/`errors.Is` path that can reach the underlying errno.

**D. Behavioral reproduction** (executed against the real, unmodified pinned client — not just reasoned about): a standalone Go program mirroring `errConnectionFailed` + the exact `request.go:171` construction, run against `isDockerConnectivityError`'s exact logic copied from `docker_service.go:196-251`, confirmed:

```
err.Error(): permission denied while trying to connect to the docker API at unix:///var/run/docker.sock
errors.As(err, &errno): false
errors.Is(err, os.ErrPermission): false
isDockerConnectivityError(err): false          <-- the gap, confirmed live
errors.Unwrap(err) succeeds (non-nil): true    <-- the errConnectionFailed wrapper unwraps fine
errors.Unwrap(unwrapped) (dead end): <nil>     <-- this is where the errno is actually lost
```

A supervisor review of this plan independently re-ran this reproduction against the actual unmodified pinned moby client and confirmed the same result, along with the log evidence and the proposed fix.

**E. `docker_service_test.go` coverage check** (to rule out redundancy): every existing EACCES/EPERM test (`TestIsDockerConnectivityError` line 99-100, `TestIsDockerConnectivityError_OpError`, `TestExtractErrno_*`) constructs the syscall error as **directly reachable** in the chain — a bare `syscall.EACCES`, or `net.OpError{Err: syscall.EACCES}`, `url.Error{Err: syscall.EACCES}`, `os.SyscallError{Err: syscall.EACCES}`. None reproduce the real moby v0.5.1 shape, where the errno is **not present in the chain at all** (it was discarded by `%v` substitution before `errConnectionFailed` was constructed). This is a genuine, non-redundant gap in existing coverage.

### 2.2 Scope note (what is NOT affected)

Reading `request.go:173-178`, the sibling `os.ErrNotExist` branch (missing-socket case) **does** use `%w` and re-references the unwrapped original error:
```go
if errors.Is(err, os.ErrNotExist) {
    err = errors.Unwrap(err)
    return nil, errConnectionFailed{fmt.Errorf("failed to connect to the docker API at %v; check if the path is correct and if the daemon is running: %w", cli.host, err)}
}
```
so `ENOENT` remains reachable via `errors.As`/`isDockerConnectivityError`'s existing syscall-walk — this path is **not** broken, consistent with `TestDockerService_ListContainers_LocalConnectivityError` (`docker_service_test.go:577-594`, which already exercises a real nonexistent-socket path end-to-end and passes today). The gap is isolated specifically to the `os.ErrPermission` branch (`request.go:168-172`) — i.e., only `EACCES`/`EPERM` connection-time failures are affected, not `ENOENT`.

### 2.3 Existing classifier structure (context for the fix)

`isDockerConnectivityError` (`backend/internal/services/docker_service.go`, lines 196-251) classifies an error as a Docker-connectivity failure via, in order: (1) a lower-cased substring match against a small set of known daemon-not-running/connect-failure message fragments, then (2) `errors.Is(err, context.DeadlineExceeded)` / `net.Error.Timeout()`, then (3) an `errors.As`-based walk for specific `syscall.Errno` values (`ENOENT`, `EACCES`, `EPERM`, `ECONNREFUSED`). Because the moby v0.5.1 permission-denied error's errno is unreachable (§2.1.C/D), only step (1) — the substring check — can ever match it; a new fragment must be added there.

## 3. Technical Specification

### 3.1 Fix specification

Add one substring to the existing message-based check in `isDockerConnectivityError` (`backend/internal/services/docker_service.go`, current lines 202-207):

```go
msg := strings.ToLower(err.Error())
if strings.Contains(msg, "cannot connect to the docker daemon") ||
    strings.Contains(msg, "is the docker daemon running") ||
    strings.Contains(msg, "error during connect") ||
    strings.Contains(msg, "permission denied while trying to connect to the docker api") {  // NEW
    return true
}
```

This is checked before the `errors.Is`/`errors.As` walks (unchanged ordering), so it correctly routes this error to the existing `DockerUnavailableError` / `buildLocalDockerUnavailableDetails` path (503, with the existing permissions-group-hint messaging — see `TestBuildLocalDockerUnavailableDetails_PermissionDeniedIncludesGroupHint`), instead of the generic 500.

Add an explanatory comment directly above the new line citing this investigation:
```go
// "permission denied while trying to connect to the docker api" matches the
// message produced by github.com/moby/moby/client@v0.5.1's doRequest()
// (request.go:168-172) for os.ErrPermission (EACCES/EPERM connecting to the
// Docker socket). That code path's fmt.Errorf call does not reference the
// original syscall error at all (only cli.host, via %v) — so unlike other
// syscall-wrapped errors handled below, the underlying errno is permanently
// unrecoverable from this error's chain and can only be matched by message
// text. Confirmed via production incident logs (.github/logs/charon.log,
// 2026-07-30 13:37-14:12) and a live repro against the vendored client.
```

### 3.2 Handler layer — no change required

`docker_handler.go`'s existing `*services.DockerUnavailableError` branch (lines 105-117) and its existing test (`TestDockerHandler_ListContainers_DockerUnavailableMappedTo503`) already cover the 503 response shape once `ListContainers` correctly classifies the error. This fix only changes *classification* inside `isDockerConnectivityError`, not the handler or the response contract.

### 3.3 Data flow (before/after)

```
cli.ContainerList(ctx, ...) returns moby v0.5.1's os.ErrPermission-branch error
   │
   ▼
isDockerConnectivityError(err)
   │
   ├── BEFORE: substring checks all miss, errno unreachable (§2.1.D) → false
   │             → falls through to generic `fmt.Errorf("failed to list
   │               containers: %w", err)` → handler's 500 fallback branch
   │
   └── AFTER:  new substring matches → true
                 → NewDockerUnavailableError(err, buildLocalDockerUnavailableDetails(...))
                 → handler's existing 503 branch (unchanged code, §3.2)
```

## 4. Testing

### 4.1 Playwright E2E — not applicable, justified explicitly

No new or modified Playwright spec is required: this fix only changes which of two *already-existing* backend error-response shapes (`503`/`DockerUnavailableError` vs. `500`/generic) a specific rare OS-permission failure maps to. Both response shapes are already rendered identically by the frontend (`ProxyHostForm.tsx` renders `dockerError.message` verbatim regardless of status/shape, confirmed in `docs/plans/current_spec.md` §2.7) — there is no new UI state, and reliably triggering a real `EACCES`/`EPERM` from the actual Docker socket in a browser-driven E2E environment is not practical or deterministic (root-run CI containers bypass DAC permission checks entirely, so a real filesystem permission denial cannot be relied upon to fire). This condition is correctly and deterministically tested at the Go unit-test level (§4.2).

### 4.2 Unit tests

`backend/internal/services/docker_service_test.go`:

| Test | Purpose | Mechanism |
|---|---|---|
| `TestIsDockerConnectivityError` (extend existing table, ~line 86-104) | Add case: `{"moby permission-denied message, no errno in chain", errors.New("permission denied while trying to connect to the docker API at unix:///var/run/docker.sock"), true}` | Plain `errors.New` — table-test style already used for every other case in this table. |
| `TestIsDockerConnectivityError_MobyPermissionDeniedMessageShape` (new) | Confirms the classifier match is robust against the *actual* wrapped error shape moby produces, not just a bare string. | Defines a small local type mirroring moby's unexported `errConnectionFailed` (embeds `error`, implements `Unwrap()`) wrapping the exact `request.go:171` construction (`fmt.Errorf("permission denied while trying to connect to the docker API at %v", host)`). Asserts (a) `errors.As(err, &syscall.Errno{})` is `false` (sanity check that the reproduction is faithful — the errno really is unreachable), and (b) `isDockerConnectivityError(err)` is `true` after the fix. |
| `TestDockerService_ListContainers_LocalPermissionDeniedMessageShape` (new, end-to-end at the `ListContainers` level) | Proves the fix closes the gap at the full `ListContainers` call boundary, through the real, unmodified pinned `client.Client` — not a hand-rolled fake error type. | See concrete injection mechanism below (required — do not substitute an ad hoc fake or a real-filesystem `chmod 000` approach). |

**Required, concrete, CI-safe injection mechanism for the third test** (do not substitute an ad hoc fake or a real-filesystem `chmod 000` approach — the latter silently no-ops in CI when tests run as root, since root bypasses Linux DAC permission checks and `EACCES` never fires): construct a `*client.Client` via `client.New` with `client.WithDialContext` returning `os.ErrPermission` on every dial attempt, pointed at a nonexistent/unused socket path, so the real moby v0.5.1 `doRequest()` code path in `request.go:168-172` is exercised unmodified and deterministically, independent of OS user/root:

```go
cli, err := client.New(
    client.WithHost("unix:///nonexistent/perm-test.sock"),
    client.WithAPIVersion("1.43"),
    client.WithDialContext(func(ctx context.Context, network, addr string) (net.Conn, error) {
        return nil, os.ErrPermission
    }),
)
require.NoError(t, err)
```

Build a `DockerService` struct literal (mirroring the existing pattern in `docker_service_test.go`, e.g. `TestListContainers_EmptyResultIsNotNil`) with this `client` as its `client` field, call `ListContainers`, and assert `errors.As(err, &unavailErr)` succeeds where `unavailErr` is `*DockerUnavailableError` — i.e., assert the resulting error is **not** the generic `"failed to list containers: ..."` fallback. Additionally assert `unavailErr.Details()` is non-empty and matches the existing local-permissions guidance produced by `buildLocalDockerUnavailableDetails` (consistent with `TestBuildLocalDockerUnavailableDetails_PermissionDeniedIncludesGroupHint`'s existing expectations), confirming the *whole* real-client-to-classified-error path, not just the classifier function in isolation.

No handler-level test addition is required (§3.2).

## 5. Commit Slicing Strategy

**Decision**: Single PR, single commit — this fix is small enough (one new substring branch, one comment, three tests, all confined to one file plus its test file) that further slicing would add process overhead without improving reviewability. This PR has no dependency on, and is not sequenced relative to, any other PR (see §7).

| Commit | Scope | Files | Depends on | Validation gate |
|---|---|---|---|---|
| **Commit 1** — Fix: classify moby v0.5.1's permission-denied connection-failed message as a connectivity error, plus full test coverage | Add the new substring check to `isDockerConnectivityError` (§3.1); add the explanatory comment; add all three tests from §4.2 (`TestIsDockerConnectivityError` new table case, `TestIsDockerConnectivityError_MobyPermissionDeniedMessageShape`, `TestDockerService_ListContainers_LocalPermissionDeniedMessageShape`). | `backend/internal/services/docker_service.go`, `backend/internal/services/docker_service_test.go` | None | `go build ./...`; `go test ./internal/services/... -run "DockerConnectivity|PermissionDenied" -v`; confirm all pre-existing `TestIsDockerConnectivityError*` and `TestListContainers_*`/`TestDockerService_ListContainers_*` tests still pass unmodified (regression proof — this must not change behavior for `ENOENT`, `ECONNREFUSED`, or the daemon-not-running message paths); `make lint-fast`; full `go test ./...` for repo-wide regression confirmation |

If review feedback surfaces additional scope (e.g. the reviewer wants the handler-layer log message adjusted, or wants a `docs/features.md` note on this error state), that is a small addition to Commit 1 rather than a new commit — the fix's total surface area does not warrant a second functional commit. A trailing validation-only step (full DoD sweep: coverage, patch report, lefthook, Playwright regression subset) is expected before merge but does not need its own commit if Commit 1's own validation gate already passes clean.

**Rollback / contingency notes for the PR as a whole**:

- Purely additive (one new substring branch + comment + tests) — `git revert`-safe in isolation with no schema, config, or external API contract change.
- If the exact message text ever changes in a future moby client upgrade, this substring check silently stops matching (fails open to the pre-existing generic-500 behavior — a UX regression, not a security regression) — a known fragility of message-text matching, consistent with the existing three substring checks it's appended to, which already accept this same tradeoff for the daemon-not-running cases.
- No feature flag or staged rollout is warranted given the change is a narrow, additive classification branch with full unit-test coverage of both the new and all pre-existing cases.

## 6. Acceptance Criteria (Definition of Done)

1. `isDockerConnectivityError` (`backend/internal/services/docker_service.go`) classifies the exact message text produced by `github.com/moby/moby/client@v0.5.1`'s `os.ErrPermission` branch (`request.go:168-172`) as a connectivity error.
2. `ListContainers`, when the underlying `cli.ContainerList` call fails with this exact error shape, returns `*DockerUnavailableError` (503 path) rather than falling through to the generic `500` path — verified end-to-end via `TestDockerService_ListContainers_LocalPermissionDeniedMessageShape` using the `client.WithDialContext` injection mechanism specified in §4.2 (not a real-filesystem permission denial, which is not CI-safe).
3. All three new tests from §4.2 pass; all pre-existing `TestIsDockerConnectivityError*`, `TestListContainers_*`, and `TestDockerService_ListContainers_*` tests continue to pass unmodified — confirming `ENOENT`, `ECONNREFUSED`, and daemon-not-running classification is unaffected.
4. No handler-layer, frontend, migration, or Docker Compose/entrypoint changes are made (§1.3) — confirmed by the diff touching only `backend/internal/services/docker_service.go` and `backend/internal/services/docker_service_test.go`.
5. Full CLAUDE.md Definition of Done passes: Playwright regression subset green (no diff expected, §4.1); `scripts/local-patch-report.sh` artifacts produced; `lefthook run pre-commit` (staticcheck + CodeQL Go/JS) zero blocking findings; `scripts/go-test-coverage.sh` ≥85% overall and patch coverage; `cd backend && go build ./...` succeeds; no debug prints/dead code left behind.
6. PR is a single-commit (or, if review feedback expands scope slightly, still single-PR) change per §5, independently mergeable in any order relative to the unrelated timeout-fix PR (§7).

## 7. Related Specifications / Further Reading

- `backend/internal/services/docker_service.go` (file under change)
- `backend/internal/services/docker_service_test.go` (existing test patterns reused)
- `backend/internal/api/handlers/docker_handler.go` / `docker_handler_test.go` (handler layer — read to confirm no change needed, §3.2)
- `frontend/src/hooks/useDocker.ts`, `frontend/src/components/ProxyHostForm.tsx` (confirmed unaffected — both already render whatever the existing `DockerUnavailableError` 503 response contains)
- `.github/logs/charon.log` (primary-source incident log used to find and verify this bug, §2.1.A)
- `$(go env GOMODCACHE)/github.com/moby/moby/client@v0.5.1/{request.go,errors.go}` (primary-source vendored dependency read to verify root cause and fix, §2.1.C)
- **Related, independent work (separate PR, no dependency either direction)**: `docs/plans/current_spec.md` — the bounded-timeout fix for slow `ListContainers` responses. Both PRs touch `backend/internal/services/docker_service.go` (this PR modifies `isDockerConnectivityError`; that PR wraps `cli.ContainerList` in `context.WithTimeout` and adds a separate, earlier-checked `isBoundedListTimeout` branch), so merging one before the other may require a routine rebase of whichever merges second. Neither PR depends on the other functionally, and each is independently revertable — they fix two unrelated bugs (message-classification gap vs. missing timeout) discovered via two separate investigations, and are safe to merge in either order.

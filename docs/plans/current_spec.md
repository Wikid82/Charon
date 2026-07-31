# Bounded Timeout + Distinguishable Error for Slow Docker Daemon Responses

**Type**: Small, targeted backend hardening fix (not a redesign). Branch: current working branch (`feat/changelog` per repo state at plan time — implementer should create/switch to a dedicated branch, e.g. `fix/docker-list-containers-timeout`, off `main` before starting work, per standard PR flow).

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

## 1. Introduction

### 1.1 Background (already root-caused — do not re-investigate)

A user reported that the local Docker container quick-select list in the Proxy Host form (`frontend/src/components/ProxyHostForm.tsx`) appears blank despite 14 containers running on their host. The maintainer already root-caused this via live investigation (`docker exec`, `docker logs`, `docker inspect`, reading `backend/internal/services/docker_service.go` and `.docker/docker-entrypoint.sh`) and **ruled out permissions**:

- The rootless-Docker GID-remapping logic in `.docker/docker-entrypoint.sh` (lines 122-141) is working correctly and is **out of scope**.
- No `EACCES`/`EPERM`/`ENOENT` ever appears in logs — `DockerService`'s existing `buildLocalDockerUnavailableDetails()` permissions-error path never fires.
- Captured evidence from container logs:
  ```
  duration:30.008511109s → 502 (Caddy's embedded reverse-proxy timeout fired)
  "error":"failed to list containers: Get \"http://%2Fvar%2Frun%2Fdocker.sock/v1.55/containers/json\": context canceled"
  ```
  followed by a request that succeeded but still took 8.22s. `docker ps` on the same host returns in 118ms; the host is not resource-starved.

**Confirmed root cause**: `DockerService.ListContainers` (`backend/internal/services/docker_service.go:107`) calls `cli.ContainerList(ctx, ...)` (line 134) using the caller's context directly, with **no bounded timeout of its own**. On a rootless host where 5 containers mount the same Docker socket (charon, charon-e2e, dockhand, homepage, prunemate), the daemon is intermittently slow (8-37s) to respond to `ContainerList` even though `docker ps` is fast. Because Charon has no internal timeout shorter than the front-end embedded Caddy reverse-proxy's timeout, the *request-scoped* context (`c.Request.Context()`, Gin) eventually gets canceled by Caddy giving up on the backend connection — the Go HTTP server cancels the request context with `context.Canceled` (not `context.DeadlineExceeded`), which matches the literal log text `"context canceled"` observed. This generic cancellation bubbles up through the existing generic-error path (`fmt.Errorf("failed to list containers: %w", err)`, `docker_service.go:142`) to a `500` with an unhelpful message — this is what the user perceives as "the list is just blank" (see §2.4 for exact code-path trace confirming this).

### 1.2 Objective

Add a bounded `context.WithTimeout` around the `cli.ContainerList` call in `ListContainers` so a slow daemon fails fast (comfortably before Caddy's proxy timeout) with a new, distinguishable error — instead of hanging until an unrelated upstream timeout hard-cancels the request and surfaces a generic "context canceled" failure.

### 1.3 Explicitly out of scope

- `.docker/docker-entrypoint.sh` (GID-detection logic) — already correct, do not touch.
- Any Docker Compose file.
- Rootless Docker / subgid host configuration.
- Any redesign of `DockerService`, its constructor, or its remote-host code path beyond the single bounded-timeout addition.
- Frontend code changes (see §3.4 — analysis shows none are required, contingent on the handler-layer HTTP status choice specified in §2.3).

## 2. Research Findings

### 2.1 Current `ListContainers` implementation

`backend/internal/services/docker_service.go`, lines 107-194 (relevant excerpt):

```go
func (s *DockerService) ListContainers(ctx context.Context, host string) ([]DockerContainer, error) {
    // Check if Docker was available during initialization
    if s.initErr != nil {
        var unavailableErr *DockerUnavailableError
        if errors.As(s.initErr, &unavailableErr) {
            return nil, unavailableErr
        }
        return nil, NewDockerUnavailableError(s.initErr, buildLocalDockerUnavailableDetails(s.initErr, s.localHost))
    }

    var cli *client.Client
    var err error

    if host == "" || host == "local" {
        cli = s.client
    } else {
        cli, err = client.New(client.WithHost(host))
        if err != nil {
            return nil, fmt.Errorf("failed to create remote client: %w", err)
        }
        defer func() { ... cli.Close() ... }()
    }

    containers, err := cli.ContainerList(ctx, client.ContainerListOptions{All: false})   // <-- line 134, NO bounded timeout
    if err != nil {
        if isDockerConnectivityError(err) {
            if host == "" || host == "local" {
                return nil, NewDockerUnavailableError(err, buildLocalDockerUnavailableDetails(err, s.localHost))
            }
            return nil, NewDockerUnavailableError(err)
        }
        return nil, fmt.Errorf("failed to list containers: %w", err)   // <-- line 142, generic error path (what the user hits today)
    }
    ... // container-mapping loop, unaffected by this change
}
```

There is **no existing `context.WithTimeout` anywhere in `docker_service.go`** — this file has no timeout/context-scoping pattern to reuse. Other services in the repo do have this pattern (used as precedent, §2.5).

### 2.2 `DockerUnavailableError` and `isDockerConnectivityError`

`docker_service.go` lines 19-52 define `DockerUnavailableError` (fields `err`, `details`; methods `Error()`, `Unwrap()`, `Details()`). `isDockerConnectivityError` (lines 196-251) classifies an error as a Docker-connectivity failure by checking daemon-not-running message substrings, `errors.Is(err, context.DeadlineExceeded)`, `net.Error.Timeout()`, and specific `syscall.Errno` values (`ENOENT`, `EACCES`, `EPERM`, `ECONNREFUSED`).

**Critical interaction to design around**: `isDockerConnectivityError` already treats `context.DeadlineExceeded` as a connectivity error (line 210, and covered by test `TestIsDockerConnectivityError` case `"context timeout"`). If the new bounded timeout is added *naively* (i.e., the resulting `context.DeadlineExceeded` error is simply passed into the existing `isDockerConnectivityError(err)` check), it will be misclassified as the same permissions/connectivity failure (`DockerUnavailableError` with `buildLocalDockerUnavailableDetails`), which is exactly the *opposite* of what the ask requires ("DISTINGUISHABLE error ... distinct from the existing `DockerUnavailableError` messaging"). **The new timeout detection must be checked before `isDockerConnectivityError`, using a purpose-built check** (§3.3) that only fires when *our own* bounded timeout expired — not when the daemon is actually unreachable or the caller's context was canceled/expired for an unrelated reason.

### 2.3 Distinguishing "our timeout fired" from "caller canceled" — the `context.Canceled` vs `context.DeadlineExceeded` distinction

This is the key mechanism that makes the fix correct, and it is directly supported by the evidence already in the bug report:

- Standard library `net/http` servers cancel `r.Context()` with **`context.Canceled`** when the client (Caddy, acting as reverse-proxy client to the Charon backend) gives up and closes the connection — **not** `context.DeadlineExceeded`. This is exactly why the captured log shows the literal string `"context canceled"`, not `"context deadline exceeded"`.
- A `context.WithTimeout(parentCtx, d)` child context expires with **`context.DeadlineExceeded`** when *its own* `d` elapses, provided `parentCtx` has not itself been canceled/expired first.
- Therefore: if the new bounded timeout (`d` = 8s, comfortably less than Caddy's ~30s) fires *before* Caddy ever gives up, `cli.ContainerList` will fail with `context.DeadlineExceeded`, and the original request context (`ctx`, the parameter passed into `ListContainers`) will still be healthy (`ctx.Err() == nil`) at that moment — because Caddy hasn't canceled it yet. This gives a clean, race-free discriminator:

  ```go
  errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil
  ```

  reliably means "our bounded timeout expired on its own, independent of the caller's context state" — i.e., exactly the slow-daemon case this fix targets. If the caller's context were itself canceled or expired first (e.g., Caddy's timeout actually fires first because someone misconfigures the new timeout to be too long), `ctx.Err()` would be non-nil at that point and the code correctly falls through to existing behavior instead of misreporting.

### 2.4 Handler-layer error propagation (`backend/internal/api/handlers/docker_handler.go`)

`ListContainers` handler (lines 58-125) calls `h.dockerService.ListContainers(c.Request.Context(), host)` (line 103) and branches only on `*services.DockerUnavailableError` via `errors.As`:

```go
if err != nil {
    var unavailableErr *services.DockerUnavailableError
    if errors.As(err, &unavailableErr) {
        details := unavailableErr.Details()
        if details == "" {
            details = "Cannot connect to Docker. Please ensure Docker is running and the socket is accessible (e.g., /var/run/docker.sock is mounted)."
        }
        log.Warn(...)
        c.JSON(http.StatusServiceUnavailable, gin.H{
            "error":   "Docker daemon unavailable",
            "details": details,
        })
        return
    }

    log.Error(...)
    c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list containers"})
    return
}
```

Any error that is not a `*DockerUnavailableError` (including today's generic-cancellation error) falls into the `500` branch with **no `details` field** — this is the exact code path the user hit.

### 2.5 Existing configurable-timeout precedent in the codebase

Two established patterns exist for per-call timeouts in this repo, both usable as precedent:

1. Inline `context.WithTimeout(ctx, N*time.Second)` at the call site (most common — e.g. `backend/internal/api/handlers/crowdsec_handler.go:1515`, `backend/internal/caddy/importer.go:28`, `backend/internal/services/manual_challenge_service.go:361`).
2. A **struct field carrying a configurable `time.Duration`**, defaulted in the constructor but overridable in tests via direct struct-literal construction (same package) — e.g. `backend/internal/services/uptime_service.go:465` (`s.config.CheckTimeout`), `backend/internal/crowdsec/hub_sync.go` (`s.PullTimeout`, `s.ApplyTimeout`).

Pattern 2 is required here (not just pattern 1) because `docker_service_test.go` already constructs `DockerService` directly via struct literal in the same package (e.g. `TestListContainers_ContainerMappingEdgeCases`, `TestListContainers_EmptyResultIsNotNil` — both at lines ~424 and ~460 of `docker_service_test.go`), and the new timeout test needs to shrink the timeout to milliseconds so the test suite doesn't sleep for 8 real seconds. A hardcoded inline constant would make the timeout untestable without an 8-second-plus test.

### 2.6 Mocking pattern for a slow/hanging `ContainerList` call

`docker_service_test.go` already has the exact scaffolding needed, in `newContainerListClient` (lines 362-377):

```go
func newContainerListClient(t *testing.T, containerJSON string) *client.Client {
    t.Helper()
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        _, _ = io.WriteString(w, containerJSON)
    }))
    t.Cleanup(server.Close)
    cli, err := client.New(
        client.WithHost("tcp://"+server.Listener.Addr().String()),
        client.WithAPIVersion("1.43"),
    )
    require.NoError(t, err)
    t.Cleanup(func() { _ = cli.Close() })
    return cli
}
```

This is used to build a `*DockerService` directly via struct literal (e.g. `TestListContainers_EmptyResultIsNotNil`, lines 459-473):

```go
svc := &DockerService{
    client:    newContainerListClient(t, "[]"),
    initErr:   nil,
    localHost: "tcp://localhost:2375",
}
```

The new timeout test mirrors this exactly, but with an `httptest.Server` handler that **blocks on the request's own context** instead of responding immediately (so the test proves cancellation propagates, and finishes fast regardless of the configured timeout):

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    <-r.Context().Done() // block until the client (moby SDK) cancels on timeout
}))
```

paired with a `DockerService` struct literal that sets the new `listContainersTimeout` field to a small value (e.g. `50 * time.Millisecond`), per §2.5's pattern-2 requirement — this is what keeps the new test fast (sub-second) rather than requiring an 8+ second sleep. This mirrors the mocking style used for the existing init-error path test (`TestNewDockerServiceFromLocalHost_ClientInitError`, which exercises `newDockerServiceFromLocalHost` directly) in spirit: construct the smallest possible fake that exercises exactly the new branch, in the same package, with no interface/mock-generation framework needed.

### 2.7 Frontend error-surfacing path — confirmed, NO frontend change required (with one binding constraint)

`frontend/src/hooks/useDocker.ts` (full file, 33 lines):

```ts
export function useDocker(host?: string | null, serverId?: string | null) {
  const { data: containers = [], isLoading, error, refetch } = useQuery({
    queryKey: ['docker-containers', host, serverId],
    queryFn: async () => {
      try {
        return await dockerApi.listContainers(host || undefined, serverId || undefined)
      } catch (err: unknown) {
        const error = err as { response?: { status?: number; data?: { details?: string } } }
        if (error.response?.status === 503) {
          const details = error.response?.data?.details
          const message = details || 'Docker service unavailable. Check that Docker is running.'
          throw new Error(message, { cause: err })
        }
        throw err
      }
    },
    enabled: Boolean(host) || Boolean(serverId),
    retry: 1,
  })
  return { containers: containers ?? [], isLoading, error, refetch }
}
```

`frontend/src/components/ProxyHostForm.tsx` lines 789-799 render `(dockerError as Error).message` verbatim under a static "Docker Connection Failed" heading — it does not branch on error type/content, so **any** message string reaching it displays correctly with no component changes needed.

**However**, `useDocker.ts` has a hard branch on `error.response?.status === 503` (line above). This is the one place frontend behavior is coupled to a backend contract choice:

- If the handler responds to the new timeout error with **HTTP 503** and a populated `details` field (mirroring the existing `DockerUnavailableError` response shape), `useDocker.ts`'s existing branch already extracts `details` and surfaces it as `error.message` — **zero frontend changes needed**.
- If the handler instead used a different status code (e.g. `504 Gateway Timeout`, considered in §3.1's alternatives), the `status === 503` check would **not** match, `details` would never be extracted, and the raw axios error (`"Request failed with status code 504"`) would render instead — an unhelpful regression, and it *would* force a frontend change to `useDocker.ts` to also match `504`.

**Decision (binding on the handler design, §3.2/§3.3)**: reuse HTTP `503 Service Unavailable` for the new timeout error, with a distinct `error` and `details` message. This satisfies the ask's own suggested option ("or same 503 with a different message") and is the only choice that requires **zero frontend changes**, which is explicitly preferred by the task and consistent with CLAUDE.md's "small, targeted change" instruction test-independent of frontend behavior. This is confirmed explicitly in §3.4.

### 2.8 Caddy's ~30s proxy timeout — confirmed as empirical, not a repo-configured constant

`backend/internal/caddy/types.go`, `ReverseProxyHandler` (lines 129-220ish) builds the `reverse_proxy` handler JSON for **user-configured proxy hosts** (i.e., this is the code that generates Caddy config for host entries Charon manages) — grepped in full for `timeout`/`transport`/`dial_timeout`/`response_header_timeout`: **no explicit timeout keys are set** anywhere in the generated handler (`flush_interval: -1`, `upstreams`, header manipulation only). The embedded Caddy instance that fronts the Charon web app itself (started by `.docker/docker-entrypoint.sh` line 386, `caddy run --config /config/caddy.json`) likewise has no explicit `dial_timeout`/`response_header_timeout` override checked into the repo (`configs/caddy.json` has none either). This means the ~30s figure is **Caddy's own built-in default reverse-proxy behavior**, not a value pinned anywhere in this repository — the maintainer's empirical log capture (`duration:30.008511109s → 502`, quoted in full in §1.1) is therefore the authoritative source for "~30s," not a Caddyfile setting. **Recommendation validity**: any new internal timeout must stay comfortably under this empirically-observed ~30s ceiling; 8s (§3.1) leaves >20s of margin, which is intentionally generous because this figure is not a value we control or can rely on staying fixed.

### 2.9 `.gitignore` / `.dockerignore` / `.codecov.yml` / `Dockerfile` — confirmed no changes needed

This change adds no new files, directories, build artifacts, or dependencies — only edits to three existing Go files (`docker_service.go`, `docker_service_test.go`, `docker_handler.go`, and `docker_handler_test.go`). None of `.gitignore`, `.dockerignore`, `.codecov.yml`, or `Dockerfile` require updates. Confirmed by inspection — no new paths are introduced by this plan.

## 3. Technical Specifications

### 3.1 Timeout value and rationale

| Parameter | Value | Rationale |
|---|---|---|
| `defaultListContainersTimeout` | `8 * time.Second` | Per task guidance ("5-10 seconds"). Chosen at the upper-middle of that range: long enough that a *briefly* contended rootless socket (observed successful-but-slow requests up to 8.22s in the bug report) still has a fair chance to succeed, short enough to leave >20s of margin under Caddy's empirically observed ~30s proxy timeout (§2.8), so the new bounded timeout **always** wins the race against the upstream cancellation — which is the entire point of the fix. |

Alternative considered: 5s (tighter, but risks false-positive timeouts on the legitimately-slow-but-succeeding 8.22s request already observed in the bug's own evidence — rejected). 10s (also acceptable, slightly less margin) — 8s chosen as the better balance; not a hard requirement, and the constant is named and centralized (§3.3) so it can be tuned later without touching call sites.

### 3.2 New error type: `DockerTimeoutError`

New type in `backend/internal/services/docker_service.go`, placed immediately after the existing `DockerUnavailableError` block (after line 52), mirroring its shape exactly (`Error()`/`Unwrap()`/`Details()`) so handler code can treat both error types uniformly where useful, while remaining a structurally distinct Go type so `errors.As` cleanly discriminates them:

```go
// DockerTimeoutError indicates the Docker daemon did not respond to an API
// call within Charon's own bounded timeout. It is intentionally distinct
// from DockerUnavailableError: DockerUnavailableError means "the daemon is
// unreachable / permissions are wrong" (a configuration problem), whereas
// DockerTimeoutError means "the daemon is reachable but responding slowly"
// (transient — retrying may succeed). Keeping them separate lets callers
// (and the frontend) surface an accurate, actionable message for each case
// instead of a single generic "Docker unavailable" message for both.
type DockerTimeoutError struct {
    err     error
    timeout time.Duration
}

// NewDockerTimeoutError wraps the underlying context-deadline error together
// with the timeout duration that was configured, so Details() can report it.
func NewDockerTimeoutError(err error, timeout time.Duration) *DockerTimeoutError {
    return &DockerTimeoutError{err: err, timeout: timeout}
}

func (e *DockerTimeoutError) Error() string {
    if e == nil || e.err == nil {
        return "docker request timed out"
    }
    return fmt.Sprintf("docker request timed out after %s: %v", e.timeout, e.err)
}

func (e *DockerTimeoutError) Unwrap() error {
    if e == nil {
        return nil
    }
    return e.err
}

// Details returns a user-facing, actionable message distinct from
// buildLocalDockerUnavailableDetails' permissions-oriented guidance.
func (e *DockerTimeoutError) Details() string {
    if e == nil {
        return ""
    }
    return fmt.Sprintf(
        "Docker daemon is responding slowly (no response within %s). "+
            "This can happen when the Docker socket is under heavy load from other "+
            "tools or containers. Please try again.",
        e.timeout,
    )
}
```

New import required: `"time"` (not currently imported in `docker_service.go` — confirmed by reading the file's import block, §2.1).

### 3.3 `ListContainers` changes

`backend/internal/services/docker_service.go`, modify the block starting at (current) line 134:

```go
const defaultListContainersTimeout = 8 * time.Second   // package-level const, near top of file

func (s *DockerService) ListContainers(ctx context.Context, host string) ([]DockerContainer, error) {
    // ... unchanged initErr / cli-selection logic (lines 107-132) ...

    timeout := s.listContainersTimeout
    if timeout <= 0 {
        timeout = defaultListContainersTimeout
    }
    listCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    containers, err := cli.ContainerList(listCtx, client.ContainerListOptions{All: false})
    if err != nil {
        if isBoundedListTimeout(ctx, err) {
            return nil, NewDockerTimeoutError(err, timeout)
        }
        if isDockerConnectivityError(err) {
            if host == "" || host == "local" {
                return nil, NewDockerUnavailableError(err, buildLocalDockerUnavailableDetails(err, s.localHost))
            }
            return nil, NewDockerUnavailableError(err)
        }
        return nil, fmt.Errorf("failed to list containers: %w", err)
    }
    // ... unchanged container-mapping loop ...
}

// isBoundedListTimeout reports whether err represents Charon's own bounded
// ContainerList timeout expiring — as opposed to the caller-supplied parent
// context (ctx) being canceled/expired for an unrelated reason (e.g. an
// upstream reverse-proxy giving up on the whole request). It relies on the
// fact that context.WithTimeout's child context fails with
// context.DeadlineExceeded when ITS OWN deadline elapses, while the parent
// ctx remains healthy (ctx.Err() == nil) up to that point. See docs/plans
// current_spec.md §2.3 for the full reasoning and the context.Canceled vs
// context.DeadlineExceeded distinction this depends on.
func isBoundedListTimeout(parent context.Context, listCtx context.Context, err error) bool {
    if err == nil {
        return false
    }
    // Prefer the child context's own recorded terminal state (listCtx.Err())
    // over parsing it back out of err's wrapped chain: this is a
    // dependency-independent source of truth that keeps working even if a
    // future moby/moby/client upgrade changes how context errors are
    // wrapped in the returned err (see DEP-003/RISK-001). errors.Is is kept
    // as a defense-in-depth secondary signal.
    return (errors.Is(listCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded)) && parent.Err() == nil
}
```

**Supervisor-review amendment**: the signature now takes both `parent` (the caller's original `ctx`) and `listCtx` (the bounded child context) so the check can read `listCtx.Err()` directly instead of relying solely on parsing `context.DeadlineExceeded` back out of `err`'s wrapped chain via `errors.Is`. This was independently verified as correct against the actual pinned `github.com/moby/moby/client@v0.5.1` source (`request.go`'s `doRequest()` preserves context sentinel errors undecorated), but the extra `listCtx.Err()` check is a cheap, dependency-independent safety net against a future client-library upgrade silently changing that wrapping behavior. Update the call site in `ListContainers` accordingly: `isBoundedListTimeout(ctx, listCtx, err)`.

New unexported struct field on `DockerService` (`backend/internal/services/docker_service.go`, in the type definition at lines 71-75):

```go
type DockerService struct {
    client                 *client.Client
    initErr                error
    localHost              string
    listContainersTimeout  time.Duration // 0 = use defaultListContainersTimeout; overridable in tests
}
```

No constructor signature changes — `newDockerServiceFromLocalHost` and `NewDockerService` need no edits (zero-value `time.Duration` `0` naturally falls back to `defaultListContainersTimeout` via the `if timeout <= 0` guard above). Tests override the field directly via struct literal (same package), per the existing pattern (§2.5/§2.6).

**Ordering rationale**: `isBoundedListTimeout` is checked *before* `isDockerConnectivityError` specifically because `isDockerConnectivityError` already matches on bare `context.DeadlineExceeded` (§2.2) — checking our specific case first prevents the new timeout from being silently swallowed into the existing (misleading, in this scenario) `DockerUnavailableError` / permissions-message path.

### 3.4 Handler changes — `backend/internal/api/handlers/docker_handler.go`

Add a branch for `*services.DockerTimeoutError` **before** the existing `*services.DockerUnavailableError` branch (lines 104-117), reusing HTTP `503` per the binding decision in §2.7:

```go
containers, err := h.dockerService.ListContainers(c.Request.Context(), host)
if err != nil {
    var timeoutErr *services.DockerTimeoutError
    if errors.As(err, &timeoutErr) {
        log.WithFields(map[string]any{
            "server_id": util.SanitizeForLog(serverID),
            "host":      util.SanitizeForLog(host),
            "error":     util.SanitizeForLog(err.Error()),
        }).Warn("docker daemon responded slowly (bounded timeout fired)")
        c.JSON(http.StatusServiceUnavailable, gin.H{
            "error":   "Docker daemon is responding slowly",
            "details": timeoutErr.Details(),
        })
        return
    }

    var unavailableErr *services.DockerUnavailableError
    if errors.As(err, &unavailableErr) {
        // ... unchanged ...
    }

    // ... unchanged generic 500 fallback ...
}
```

**Response contract (new)**:

| Field | Value |
|---|---|
| HTTP status | `503 Service Unavailable` (unchanged status family vs. existing `DockerUnavailableError` responses — intentional, see §2.7) |
| `error` | `"Docker daemon is responding slowly"` (distinct string from existing `"Docker daemon unavailable"`, so logs/tests/clients can tell the two apart even though the HTTP status matches) |
| `details` | `DockerTimeoutError.Details()` output, e.g. `"Docker daemon is responding slowly (no response within 8s). This can happen when the Docker socket is under heavy load from other tools or containers. Please try again."` |

No new route, no new `dockerContainerLister` interface method — `errors.As` type-switches on the existing single `ListContainers` return value.

### 3.5 API contract summary (before/after)

| Scenario | Before | After |
|---|---|---|
| Daemon unreachable / permission denied | `503`, `{"error":"Docker daemon unavailable","details":"..."}` | unchanged |
| Daemon reachable but slow (>8s to respond) | Hangs up to ~30s, then `500`, `{"error":"Failed to list containers"}` (generic, unhelpful; only reachable if request survives to completion — often the client sees a raw `502`/timeout from Caddy instead and never gets this body at all) | `503` within ~8s, `{"error":"Docker daemon is responding slowly","details":"Docker daemon is responding slowly (no response within 8s)...."}` |
| Other API error (e.g. 500 from daemon) | `500`, `{"error":"Failed to list containers"}` | unchanged |

### 3.6 Data flow diagram

```
Frontend (ProxyHostForm.tsx)
   │  useDocker(host) → React Query
   ▼
GET /api/v1/docker/containers?host=local
   │
   ▼
DockerHandler.ListContainers (docker_handler.go:58)
   │  c.Request.Context() ──────────────────┐  (parent ctx, canceled by Caddy
   ▼                                        │   only if IT gives up first)
DockerService.ListContainers (docker_service.go:107)
   │  listCtx, cancel := context.WithTimeout(ctx, 8s)   [NEW]
   ▼
cli.ContainerList(listCtx, ...)
   │
   ├── succeeds within 8s ──────────────────────────► 200 OK, []DockerContainer
   │
   ├── listCtx expires first, parent ctx still alive ─► isBoundedListTimeout==true
   │                                                     → DockerTimeoutError
   │                                                     → 503 "responding slowly" [NEW PATH]
   │
   ├── real connectivity error (EACCES/ECONNREFUSED/ENOENT) ─► DockerUnavailableError
   │                                                            → 503 "unavailable" (unchanged)
   │
   └── parent ctx canceled first (e.g. Caddy gave up)  ─► falls through, generic error
                                                            → 500 (unchanged pre-existing
                                                              behavior for this edge case —
                                                              now effectively unreachable in
                                                              the reported scenario, since
                                                              8s << 30s guarantees our timeout
                                                              wins the race)
```

### 3.7 Error handling / edge cases

| Edge case | Behavior |
|---|---|
| `s.listContainersTimeout` unset (zero value) | Falls back to `defaultListContainersTimeout` (8s) via `if timeout <= 0` guard — safe default for all existing production construction paths (`NewDockerService`, `newDockerServiceFromLocalHost`), which are not modified. |
| Remote host (`host` param is a `tcp://...` URL, e.g. Orthrus-proxied or directly-configured remote server) | Same bounded timeout applies uniformly — `listCtx` wraps the single shared `cli.ContainerList` call regardless of local/remote branch, so remote daemons get the same fail-fast protection. Not explicitly requested by the bug report, but consistent and avoids leaving an identical unbounded-hang bug for remote hosts. |
| Caller's own context already had a shorter deadline/cancellation than 8s (e.g. test harness, future caller) | `context.WithTimeout(ctx, timeout)` respects `min(parent deadline, now+timeout)` per Go stdlib semantics — no regression, `isBoundedListTimeout`'s `parent.Err() == nil` check correctly attributes the failure to the parent instead of misreporting a `DockerTimeoutError`. |
| `initErr` already set at construction time (Docker never initialized) | Unaffected — that branch (lines 109-115) returns before the new timeout logic is ever reached. |
| Container-mapping loop (post-`ContainerList` success) | Entirely unaffected — no changes below the `if err != nil` block. |

## 4. Implementation Plan

### Phase 1: Playwright E2E Tests — **not applicable, justified explicitly**

No new or modified Playwright spec is required for this change:

- The user-visible surface (the red "Docker Connection Failed" banner in `ProxyHostForm.tsx`) is unchanged — no new component, no new DOM structure, no new interaction. Per §2.7, it already renders whatever message string the backend sends.
- The only thing that changes is *which message string* appears under a specific, hard-to-reproduce timing condition (Docker daemon slow to respond specifically between 8s and ~30s) that cannot be reliably or deterministically triggered against a real or lightly-mocked Docker daemon in a Playwright/browser E2E environment without introducing artificial delay infrastructure disproportionate to a "small, targeted" fix.
- This condition **is** deterministically and cheaply reproducible at the Go unit-test level (§4.2, via an `httptest.Server` handler that blocks on request-context cancellation) — that is the correct test layer for this fix, per CLAUDE.md's general principle of testing behavior at the layer closest to where it's implemented.
- Existing Docker-related regression coverage (none found under `tests/*.spec.ts` referencing Docker container selection specifically — confirmed via repo search) is unaffected; no existing spec asserts on the exact banner text.
- **Definition of Done step 1 compliance**: run the full relevant Playwright suite (`npx playwright test --project=firefox`, scoped to `tests/core/proxy-hosts.spec.ts` and any suite touching `ProxyHostForm`) as a regression check before merge, expecting **no changes in outcome** — this validates the "no frontend change needed" claim empirically rather than only by static analysis. Include this run's pass/fail as part of Commit 4 validation (§5).

### Phase 2: Backend Implementation

**GOAL-001**: Add the bounded timeout, new error type, and handler branch, all covered by unit tests, without altering any existing behavior for the already-covered error paths.

| Task | File | Description |
|---|---|---|
| TASK-001 | `backend/internal/services/docker_service.go` | Add `"time"` import; add `defaultListContainersTimeout` const; add `listContainersTimeout time.Duration` field to `DockerService` struct; add `DockerTimeoutError` type + `NewDockerTimeoutError` + `Error()`/`Unwrap()`/`Details()` (§3.2); add `isBoundedListTimeout` helper (§3.3); wrap `cli.ContainerList` call in `context.WithTimeout` and branch on `isBoundedListTimeout` before `isDockerConnectivityError` (§3.3). |
| TASK-002 | `backend/internal/services/docker_service_test.go` | Add `TestDockerService_ListContainers_BoundedTimeoutFires` (or similarly named) using an `httptest.Server` handler that blocks on `<-r.Context().Done()`, a `DockerService` struct literal with `listContainersTimeout: 50 * time.Millisecond`, asserting `errors.As(err, &timeoutErr)` succeeds, `timeoutErr.Details()` contains the expected slow-daemon message, and the call returns well within the test's own timeout budget (e.g. assert wall-clock elapsed < 2s to prove no accidental fallback to the 8s default). Also add a companion test asserting the *default* timeout value is used when the field is left zero (e.g. `TestDockerService_ListContainers_DefaultTimeoutUsedWhenUnset`, can assert via a small helper/reflection-free approach — see §6 test list for the concrete minimal set). |
| TASK-003 | `backend/internal/services/docker_service_test.go` | Add `TestIsBoundedListTimeout` table test covering: nil err → false; `context.DeadlineExceeded` with healthy parent → true; `context.DeadlineExceeded` with already-canceled parent (`parent.Err() != nil`) → false; unrelated error → false. Mirrors the existing `TestIsDockerConnectivityError` table-test style (§2.6 pattern). |
| TASK-004 | `backend/internal/services/docker_service_test.go` | Add `TestDockerTimeoutError_ErrorMethods` mirroring `TestDockerUnavailableError_ErrorMethods` (nil receiver, nil wrapped err, `Error()`/`Unwrap()`/`Details()` content assertions). |
| TASK-005 | `backend/internal/api/handlers/docker_handler.go` | Add the `*services.DockerTimeoutError` branch before the existing `*services.DockerUnavailableError` branch (§3.4), returning `503` with the new `error`/`details` message pair. |
| TASK-006 | `backend/internal/api/handlers/docker_handler_test.go` | Add `TestDockerHandler_ListContainers_TimeoutMappedTo503` using the existing `fakeDockerService{err: services.NewDockerTimeoutError(...)}` pattern (mirrors `TestDockerHandler_ListContainers_DockerUnavailableMappedTo503`, lines 62-81), asserting status `503`, body contains `"Docker daemon is responding slowly"` and the `details` text, and — critically — that it is distinguishable from the existing `"Docker daemon unavailable"` assertion in the sibling test (i.e. assert `NotContains` "Docker daemon unavailable" in the new test, and vice versa, to lock in the distinguishability requirement as an executable test, not just documentation). |

**Validation gate for Phase 2**: `cd backend && go build ./... && go test ./internal/services/... ./internal/api/handlers/... -run Docker -v`, plus `make lint-fast` (staticcheck, BLOCKING per CLAUDE.md), plus full `go test ./...` to confirm no regressions elsewhere.

### Phase 3: Frontend Implementation — **not applicable, justified explicitly**

Per §2.7's analysis: `useDocker.ts`'s existing `error.response?.status === 503` branch already extracts `details` generically for *any* 503 response body shaped `{error, details}`, and `ProxyHostForm.tsx` already renders `dockerError.message` generically. Because §2.7/§3.4 binds the handler design to reuse `503` (rather than introducing a new status code), no frontend file requires modification. **If a future reviewer prefers a distinct `504 Gateway Timeout` status for stronger HTTP semantic correctness, that is a valid alternative (§7, ALT-001) but is NOT this plan's chosen path specifically because it would force a frontend change this task's constraints ask to avoid** — flagged explicitly here so the tradeoff is visible to reviewers, not hidden.

### Phase 4: Integration and Testing

| Task | Description |
|---|---|
| TASK-007 | Run full backend suite with coverage: `scripts/go-test-coverage.sh` — confirm overall and patch coverage ≥85% (CLAUDE.md DoD step 6). New code (a handful of small functions/methods) is fully covered by TASK-002/003/004/006's unit tests, so this should not be a risk area. |
| TASK-008 | `bash scripts/local-patch-report.sh` — produce `test-results/local-patch-report.md` / `.json` (DoD step 2, MANDATORY). |
| TASK-009 | `lefthook run pre-commit` — staticcheck + CodeQL Go/JS scans (DoD steps 3-4). Not expected to flag anything: no new external input parsing, no new file/path handling, no new SQL/GORM usage (so `1.5 GORM Security Scan` in CLAUDE.md's DoD is **not triggered** — this change touches no `backend/internal/models/**`, no GORM queries, no migrations). |
| TASK-010 | Run the Playwright regression subset named in Phase 1 (`npx playwright test --project=firefox` scoped to proxy-host / Docker-adjacent specs) — expect zero behavioral diff, confirming Phase 3's "no frontend change" claim empirically. |
| TASK-011 | `cd backend && go build ./...` (DoD step 8) — confirm the package compiles cleanly with the new `"time"` import and new types. |

### Phase 5: Documentation and Deployment

| Task | Description |
|---|---|
| TASK-012 | No `ARCHITECTURE.md` update required — this is an internal error-handling refinement within an already-documented component (`DockerService`), not a change to system architecture, tech stack, deployment model, or directory structure (per CLAUDE.md's trigger conditions for that file). |
| TASK-013 | Optional, low-priority: if `docs/features.md` documents Docker-integration error states/messages for end users, add one sentence noting the new "Docker daemon is responding slowly, please try again" message alongside the existing "Docker daemon unavailable" message, so support/users can recognize it. Skip if `docs/features.md` does not currently enumerate specific Docker error strings (verify before adding — do not invent a new subsection for one message if the file doesn't already itemize these). |
| TASK-014 | Clean up: no debug prints/`fmt.Println`/commented code anticipated given the small surface area — verify as part of final review (DoD step 10). |

## 5. Commit Slicing Strategy

**Decision**: Single PR, one feature/fix, ordered logical commits — per CLAUDE.md's "One Feature = One PR" and this task's constraint that a backend-only fix with no frontend or E2E component should have its commit sequence reflect that (no artificial frontend/E2E commit inserted just to match a generic template).

| Commit | Scope | Files | Depends on | Validation gate |
|---|---|---|---|---|
| **Commit 1** — Foundation: new error type + timeout scaffolding (no behavior change to existing paths yet, additive only) | Add `DockerTimeoutError` type, `defaultListContainersTimeout` const, `listContainersTimeout` struct field, `isBoundedListTimeout` helper to `docker_service.go`. Do **not** yet wire `ListContainers` to use them (keeps this commit reviewable as pure addition). Add unit tests for the new type/helper in isolation (TASK-003, TASK-004). | `backend/internal/services/docker_service.go`, `backend/internal/services/docker_service_test.go` | none | `go build ./...`; `go test ./internal/services/... -run "DockerTimeoutError|IsBoundedListTimeout"`; `make lint-fast` |
| **Commit 2** — Backend behavior change: wire the bounded timeout into `ListContainers` | Wrap `cli.ContainerList` call in `context.WithTimeout`; add the `isBoundedListTimeout` branch ahead of `isDockerConnectivityError` (§3.3). Add `TestDockerService_ListContainers_BoundedTimeoutFires` and default-timeout test (TASK-002). | `backend/internal/services/docker_service.go`, `backend/internal/services/docker_service_test.go` | Commit 1 | `go build ./...`; `go test ./internal/services/... -v`; confirm existing `TestListContainers_*` and `TestDockerService_ListContainers_*` tests still pass unmodified (regression proof) |
| **Commit 3** — Handler layer: map `DockerTimeoutError` to the new `503` response | Add the `*services.DockerTimeoutError` branch in `docker_handler.go` (§3.4); add `TestDockerHandler_ListContainers_TimeoutMappedTo503` (TASK-006). | `backend/internal/api/handlers/docker_handler.go`, `backend/internal/api/handlers/docker_handler_test.go` | Commit 2 | `go build ./...`; `go test ./internal/api/handlers/... -run Docker -v`; assert new test distinguishes the two 503 message strings (§4 TASK-006) |
| **Commit 4** — Hardening + full DoD sweep (no functional changes; validation-only commit, may be folded into Commit 3 if trivial) | Run full DoD: `scripts/go-test-coverage.sh`, `scripts/local-patch-report.sh`, `lefthook run pre-commit`, Playwright regression subset (Phase 1/TASK-010), `go build ./...` full-repo. Fix any lint/coverage nits surfaced. Optionally update `docs/features.md` (TASK-013) if applicable. | Possibly none (validation-only) or `docs/features.md` | Commits 1-3 | Full CLAUDE.md Definition of Done, all 10 steps |

**Rollback / contingency notes for the PR as a whole**:

- The change is purely additive at the type level (new error type, new struct field with a safe zero-value fallback) and behavior-narrowing at the call-site level (an unbounded call becomes bounded) — there is no schema migration, no config flag, and no external API contract removal, so rollback is a plain `git revert` of the PR's merge commit with no data-migration concerns.
- If the 8s default proves too aggressive in production telemetry (e.g. legitimate slow-but-successful responses being cut off more than expected), the fix is a single-constant change (`defaultListContainersTimeout`) — no structural rework needed, confirming the "small, targeted" framing.
- If a future need arises to expose the timeout as an operator-configurable environment variable (e.g. `CHARON_DOCKER_LIST_TIMEOUT`), the `listContainersTimeout` field is already positioned to accept that without further struct changes — noted as a possible follow-up, explicitly **not** part of this plan's scope (avoid scope creep from a targeted fix).
- If the handler's `503`-reuse decision (§2.7) is challenged in review in favor of `504`, the required companion frontend change to `useDocker.ts` (add `|| error.response?.status === 504` to the existing condition) is small and isolated — flagged in §7 ALT-001 as the fallback path, but not the default plan.

## 6. Testing (consolidated list)

- **TEST-001**: `TestDockerService_ListContainers_BoundedTimeoutFires` — slow/hanging `ContainerList` (via context-blocking `httptest.Server` handler) with a shrunk `listContainersTimeout` produces `*DockerTimeoutError`, not a generic error and not `*DockerUnavailableError`; asserts on `Details()` content; asserts test wall-clock stays low (proves the shrunk timeout, not the 8s default, governed the test).
- **TEST-002**: `TestDockerService_ListContainers_DefaultTimeoutUsedWhenUnset` — confirms zero-value `listContainersTimeout` field falls back to `defaultListContainersTimeout` (can be asserted without waiting 8s by checking the constant directly and/or via a fast-failing daemon rather than proving full 8s wall-clock — avoid a slow test here).
- **TEST-003**: `TestIsBoundedListTimeout` — table test: nil err (false), `context.DeadlineExceeded` + healthy parent (true), `context.DeadlineExceeded` + already-canceled/expired parent (false), unrelated error (false).
- **TEST-004**: `TestDockerTimeoutError_ErrorMethods` — nil receiver, nil wrapped err, `Error()`, `Unwrap()`, `Details()` content, mirroring `TestDockerUnavailableError_ErrorMethods`.
- **TEST-005**: `TestDockerHandler_ListContainers_TimeoutMappedTo503` — handler maps `*services.DockerTimeoutError` to `503` with the new `error`/`details` strings; asserts the response body is textually distinguishable from the existing `DockerUnavailableError` 503 response (locks in the "distinguishable" requirement as an executable assertion).
- **TEST-006 (regression)**: full existing `docker_service_test.go` and `docker_handler_test.go` suites continue to pass unmodified — proves no behavior change to the already-covered connectivity/permissions/generic-error paths.
- **TEST-007 (regression, E2E)**: Playwright subset (`tests/core/proxy-hosts.spec.ts` and any `ProxyHostForm`-touching specs) run via `npx playwright test --project=firefox`, expected zero diff in outcome — empirical proof that no frontend change was needed (Phase 1/Phase 3 justification).

## 7. Alternatives

- **ALT-001**: Use HTTP `504 Gateway Timeout` instead of reusing `503` for the new `DockerTimeoutError` response. More semantically precise (504 specifically means "upstream did not respond in time," which is literally what happened), but requires a companion one-line change to `frontend/src/hooks/useDocker.ts`'s status check (`=== 503` → `=== 503 || === 504`) to keep the `details` field surfacing — which the task's constraints explicitly prefer to avoid unless truly necessary. **Not chosen** as the primary plan; documented here so reviewers can request it if HTTP semantic correctness is weighted higher than the zero-frontend-change goal. If chosen instead, Phase 3 would no longer be "not applicable" and the Commit Slicing Strategy would gain a small Commit 3.5/4 frontend commit.
- **ALT-002**: Reuse the existing `DockerUnavailableError` type with a new optional "reason" field/enum (e.g. `Reason: "timeout"` vs `Reason: "connectivity"`) instead of introducing a wholly new `DockerTimeoutError` type. Rejected because it would require every existing `errors.As(err, &unavailableErr)` call site (handler, tests) to additionally branch on the new field to preserve distinguishability, spreading the "is this a timeout" check across more surface area than a dedicated type with its own `errors.As` target — a dedicated type is more idiomatic Go and keeps the distinguishability guarantee enforced by the type system rather than by convention.
- **ALT-003**: Make the timeout duration an environment-variable-configurable operator setting from day one (e.g. `CHARON_DOCKER_LIST_TIMEOUT`). Rejected for this iteration as scope creep beyond "a timeout + a distinguishable error message/type" — noted as a natural, low-friction follow-up in §5's rollback/contingency notes given the field is already structured to support it later.
- **ALT-004**: Apply the bounded timeout only to the local-socket path (`host == "" || host == "local"`), leaving remote/Orthrus-proxied `ContainerList` calls unbounded. Rejected — the shared call site (line 134) makes uniform application essentially free, and leaving remote hosts unbounded would preserve an identical latent bug for remote Docker daemons with no offsetting benefit.

## 8. Dependencies

- **DEP-001**: No new third-party Go modules. `context`, `time`, and `errors` are already imported/available in the Go standard library and (for `context`/`errors`) already imported in `docker_service.go`.
- **DEP-002**: No new frontend dependencies.
- **DEP-003**: Relies on existing `github.com/moby/moby/client` behavior that `ContainerList` respects context cancellation/deadlines promptly (implicit assumption already relied upon by the pre-existing `isDockerConnectivityError`'s `context.DeadlineExceeded` handling — not a new dependency, just newly load-bearing in this specific code path).

## 9. Risks & Assumptions

- **RISK-001**: If the moby client's HTTP transport does not promptly abort the in-flight request when `listCtx` expires (e.g. buffers a large response body ignoring context first), the "fail fast" guarantee could be weaker than assumed. Mitigation: `TEST-001`'s wall-clock assertion (elapsed time bounded well under the 8s default when using a shrunk test timeout) directly validates prompt cancellation in the test suite; if it fails, this is a signal the assumption needs revisiting before merge, not after.
- **RISK-002**: Reusing HTTP `503` for two semantically different conditions (unavailable vs. slow) could confuse monitoring/alerting that keys off status code alone. Mitigation: the `error` field text differs (`"Docker daemon unavailable"` vs `"Docker daemon is responding slowly"`) and is logged distinctly server-side (`docker_handler.go` log messages differ, §3.4) — any alerting should key off the message/log field, not status code alone; flagged for awareness, not a blocker.
- **RISK-003**: 8s might still occasionally be exceeded by legitimately slow-but-eventually-successful requests (the bug report's own evidence shows an 8.22s successful request), producing an occasional false "responding slowly" message even when the daemon would have succeeded a fraction of a second later. Mitigation: this is an inherent, explicitly-accepted tradeoff of any bounded timeout (documented in §3.1); the message text says "try again" precisely because a retry is expected to succeed in this borderline case, and this is strictly better than the current unbounded hang.
- **ASSUMPTION-001**: The maintainer's empirical ~30s Caddy proxy-timeout figure (§1.1, §2.8) remains stable — this is not pinned in repo config, so a future Caddy upgrade or config change could alter it. 8s leaves substantial margin (>20s) specifically to absorb this uncertainty.
- **ASSUMPTION-002**: `.docker/docker-entrypoint.sh`'s GID-remapping logic and the underlying rootless-Docker permission model are correctly out of scope and require no changes — taken as given per the task's explicit instruction not to re-investigate.

## 10. Acceptance Criteria (Definition of Done)

1. `backend/internal/services/docker_service.go` wraps the `cli.ContainerList` call in a `context.WithTimeout` (default 8s, overridable via `listContainersTimeout` field) and returns a new `*DockerTimeoutError` — distinct from `*DockerUnavailableError` — when that bounded timeout (and not the caller's own context) expires.
2. `backend/internal/api/handlers/docker_handler.go` maps `*DockerTimeoutError` to HTTP `503` with `error: "Docker daemon is responding slowly"` and a populated `details` field, distinguishable in both status-independent text and server logs from the existing `DockerUnavailableError` `503` response.
3. All new unit tests (§6, TEST-001 through TEST-005) pass; all existing `docker_service_test.go` and `docker_handler_test.go` tests continue to pass unmodified (TEST-006).
4. `frontend/src/hooks/useDocker.ts` and `frontend/src/components/ProxyHostForm.tsx` are **not modified** — confirmed by the plan's analysis (§2.7) and by the Playwright regression run (TEST-007) showing no behavioral diff.
5. Full CLAUDE.md Definition of Done passes: Playwright regression subset green; `scripts/local-patch-report.sh` artifacts produced; `lefthook run pre-commit` (staticcheck + CodeQL Go/JS) zero blocking findings; `scripts/go-test-coverage.sh` ≥85% overall and patch coverage; `cd backend && go build ./...` succeeds; no debug prints/dead code left behind.
6. No changes to `.docker/docker-entrypoint.sh`, any Docker Compose file, `.gitignore`, `.dockerignore`, `.codecov.yml`, or `Dockerfile` (confirmed unnecessary, §2.9).
7. PR is a single feature/fix PR with the four ordered commits described in §5, each independently building and passing its stated validation gate.

## 11. Related Specifications / Further Reading

- `backend/internal/services/docker_service.go` (file under change)
- `backend/internal/services/docker_service_test.go` (existing test patterns reused)
- `backend/internal/api/handlers/docker_handler.go` / `docker_handler_test.go` (handler layer under change)
- `frontend/src/hooks/useDocker.ts`, `frontend/src/components/ProxyHostForm.tsx` (confirmed unaffected)
- `.docker/docker-entrypoint.sh` (GID-detection logic — read for context, out of scope, not modified)
- `backend/internal/caddy/types.go` (`ReverseProxyHandler` — read to confirm no repo-configured Caddy timeout exists, §2.8)
- Prior plan at this same path (now superseded): the previously-merged null-container-list crash fix (`fix/docker-empty-list-null-crash`, PR #1206) — a related but distinct hardening of the same `ListContainers`/`useDocker` code path; this plan's changes are additive and do not conflict with it.
- **Related, independent work (separate PR, no dependency either direction)**: `docs/plans/docker_permission_denied_classification.md` — a standalone plan fixing a distinct bug in `isDockerConnectivityError` (moby v0.5.1's permission-denied error message evading connectivity-error classification), found via a separate, later investigation of the same file. That plan ships as its own PR; both PRs touch `docker_service.go` and may require a rebase if merged out of order, but each is independently revertable and has no functional dependency on the other.

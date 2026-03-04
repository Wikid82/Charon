## Active Issue: Creating a Proxy Host triggers Docker socket 500

**Bug report**: “When trying to create a new proxy host, connection to the local docker socket is giving a 500 error.”

**Status**: Trace analysis complete (no code changes in this phase)

**Last updated**: 2025-12-22

---

## 1) Trace Analysis (MANDATORY)

This workflow has two coupled request paths:

1. Creating/saving the Proxy Host itself (`POST /api/v1/proxy-hosts`).
2. Populating the “Containers” quick-select (Docker integration) used during Proxy Host creation (`GET /api/v1/docker/containers`).

The reported 500 is thrown in (2), but it is experienced during the Proxy Host creation flow because the UI fetches containers from the local Docker socket when the user selects “Local (Docker Socket)”.

### A) Frontend: UI entrypoint -> hooks

1. `frontend/src/pages/ProxyHosts.tsx`
     - Component: `ProxyHosts`
     - Key functions:
         - `handleAdd()` sets `showForm=true` and clears `editingHost`.
         - `handleSubmit(data: Partial<ProxyHost>)` calls `createHost(data)` (new host) or `updateHost(uuid, data)` (edit).
     - Renders `ProxyHostForm` when `showForm` is true.

2. `frontend/src/components/ProxyHostForm.tsx`
     - Component: `ProxyHostForm({ host, onSubmit, onCancel })`
     - Default form state (`formData`) is constructed with UI defaults (notably many booleans default to `true`).
     - Docker quick-select integration:
         - Local state: `connectionSource` defaults to `'custom'`.
         - Hook call:
             - `useDocker(connectionSource === 'local' ? 'local' : undefined, connectionSource !== 'local' && connectionSource !== 'custom' ? connectionSource : undefined)`
             - When `connectionSource` is `'local'`, `useDocker(host='local', serverId=undefined)`.
             - When `connectionSource` is a remote server UUID, `useDocker(host=undefined, serverId='<uuid>')`.
     - Docker container select -> form transforms:
         - `handleContainerSelect(containerId)`:
             - chooses `forward_host` and `forward_port` from container `ip` + `private_port`, or uses `RemoteServer.host` + mapped `public_port` when a remote server source is selected.
             - auto-detects an `application` preset from `container.image`.
             - optionally auto-fills `domain_names` from a selected base domain.
     - Submit:
         - `handleSubmit(e)` builds `payloadWithoutUptime` and calls `onSubmit(payloadWithoutUptime)`.

3. `frontend/src/hooks/useProxyHosts.ts`
     - Hook: `useProxyHosts()`
     - `createHost` is `createMutation.mutateAsync` where `mutationFn: (host) => createProxyHost(host)`.

4. `frontend/src/hooks/useDocker.ts`
     - Hook: `useDocker(host?: string | null, serverId?: string | null)`
     - Uses React Query:
         - `queryKey: ['docker-containers', host, serverId]`
         - `queryFn: () => dockerApi.listContainers(host || undefined, serverId || undefined)`
         - `retry: 1`
         - `enabled: host !== null || serverId !== null`
             - Important behavior: if both params are `undefined`, this expression evaluates to `true` (`undefined !== null`).
             - Result: the hook can still issue `GET /docker/containers` even when `connectionSource` is `'custom'` (because the hook is called with `undefined, undefined`).
             - This is not necessarily the reported bug, but it is an observable logic hazard that increases the frequency of local Docker socket access.

### B) Frontend: API client and payload shapes

1. `frontend/src/api/client.ts`
     - Axios instance with `baseURL: '/api/v1'`.
     - All calls below are relative to `/api/v1`.

2. `frontend/src/api/proxyHosts.ts`
     - Function: `createProxyHost(host: Partial<ProxyHost>)`
         - Request: `POST /proxy-hosts`
         - Payload shape (snake_case; subset of):
             - `name: string`
             - `domain_names: string`
             - `forward_scheme: string`
             - `forward_host: string`
             - `forward_port: number`
             - `ssl_forced: boolean`
             - `http2_support: boolean`
             - `hsts_enabled: boolean`
             - `hsts_subdomains: boolean`
             - `block_exploits: boolean`
             - `websocket_support: boolean`
             - `enable_standard_headers?: boolean`
             - `application: 'none' | ...`
             - `locations: Array<{ uuid?: string; path: string; forward_scheme: string; forward_host: string; forward_port: number }>`
             - `advanced_config?: string` (JSON string)
             - `enabled: boolean`
             - `certificate_id?: number | null`
             - `access_list_id?: number | null`
             - `security_header_profile_id?: number | null`
         - Response: `ProxyHost` (same shape) from server.

3. `frontend/src/api/docker.ts`
     - Function: `dockerApi.listContainers(host?: string, serverId?: string)`
         - Request: `GET /docker/containers`
         - Query params:
             - `host=<string>` (e.g., `local`) OR
             - `server_id=<uuid>` (remote server UUID)
         - Response payload shape (array of `DockerContainer`):
             - `id: string`
             - `names: string[]`
             - `image: string`
             - `state: string`
             - `status: string`
             - `network: string`
             - `ip: string`
             - `ports: Array<{ private_port: number; public_port: number; type: string }>`

### C) Backend: route definitions -> handlers

1. `backend/internal/api/routes/routes.go`
     - Route group base: `/api/v1`.

     Proxy Host routes:
     - The `ProxyHostHandler` is registered on `api` (not the `protected` group):
         - `proxyHostHandler := handlers.NewProxyHostHandler(db, caddyManager, notificationService, uptimeService)`
         - `proxyHostHandler.RegisterRoutes(api)`
     - Routes include:
         - `POST /api/v1/proxy-hosts` (create)
         - plus list/get/update/delete/test/bulk endpoints.

### C1) Auth/Authz: intended exposure of Proxy Host routes

The current route registration places Proxy Host routes on the unprotected `api` group (not the `protected` auth-required group).

- Intended behavior (needs explicit confirmation): Proxy Host CRUD is accessible without auth.
- If unintended: move `ProxyHostHandler.RegisterRoutes(...)` under the `protected` group or enforce auth/authorization within the handler layer (deny-by-default).
- Either way: document the intended access model so the frontend and deployments can assume the correct security posture.

     Docker routes:
  - Docker routes are registered on `protected` (auth-required) and only if `services.NewDockerService()` returns `nil` error:
    - `dockerService, err := services.NewDockerService()`
    - `if err == nil { dockerHandler.RegisterRoutes(protected) }`
  - Key route:
    - `GET /api/v1/docker/containers`.

     Clarification: `NewDockerService()` success is a client construction success, not a reachability/health guarantee.
  - Result: the Docker endpoints may register at startup even when the Docker daemon/socket is unreachable, and failures will surface later per-request in `ListContainers`.

1. `backend/internal/api/handlers/proxy_host_handler.go`
     - Handler type: `ProxyHostHandler`
     - Method: `Create(c *gin.Context)`
         - Input binding: `c.ShouldBindJSON(&host)` into `models.ProxyHost`.
         - Validations/transforms:
             - If `host.advanced_config != ""`, it must parse as JSON; it is normalized via `caddy.NormalizeAdvancedConfig` then re-marshaled back to a JSON string.
             - `host.UUID` is generated server-side.
             - Each `host.locations[i].UUID` is generated server-side.
         - Persistence: `h.service.Create(&host)`.
         - Side effects:
             - If `h.caddyManager != nil`, `ApplyConfig(ctx)` is called; on error, it attempts rollback by deleting the created host.
             - Notification emit via `notificationService.SendExternal(...)`.
         - Response:
             - `201` with the persisted host JSON.

2. `backend/internal/api/handlers/docker_handler.go`
        - Handler type: `DockerHandler`
        - Method: `ListContainers(c *gin.Context)`
            - Reads query parameters:
                - `host := c.Query("host")`
                - `serverID := c.Query("server_id")`
            - If `server_id` is provided:
                - `remoteServerService.GetByUUID(serverID)`
                - Constructs host: `tcp://<server.Host>:<server.Port>`
            - Calls: `dockerService.ListContainers(ctx, host)`
            - On error:
                - Returns `500` with JSON: `{ "error": "Failed to list containers: <err>" }`.

     Security note (SSRF/network scanning): the `host` query param currently allows the caller to influence the Docker client target.
     - If `host` is accepted as an arbitrary value, this becomes an SSRF primitive (arbitrary outbound connections) and can be used for network scanning.
     - Preferred posture: do not accept user-supplied `host` for remote selection; use `server_id` as the only selector and resolve it server-side.

### D) Backend: services -> Docker client wrapper -> persistence

1. `backend/internal/services/proxyhost_service.go`
        - Service: `ProxyHostService`
        - `Create(host *models.ProxyHost)`:
            - Validates domain uniqueness by exact `domain_names` string match.
            - Normalizes `advanced_config` again (duplicates handler logic).
            - Persists via `db.Create(host)`.

2. `backend/internal/models/proxy_host.go` and `backend/internal/models/location.go`
        - Persistence model: `models.ProxyHost` with snake_case JSON tags.
        - Related model: `models.Location`.

3. `backend/internal/services/docker_service.go`
        - Wrapper: `DockerService`
        - `NewDockerService()`:
            - Creates Docker client via `client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())`.
            - Important: this does not guarantee the daemon is reachable; it typically succeeds even if the socket is missing/unreachable, because it does not perform an API call.
        - `ListContainers(ctx, host string)`:
            - If `host == ""` or `host == "local"`:
                - uses the default client (local Docker socket via env defaults).
            - Else:
                - creates a new client with `client.WithHost(host)` (e.g., `tcp://...`).
            - Calls Docker API: `cli.ContainerList(ctx, container.ListOptions{All: false})`.
            - Maps Docker container data to `[]DockerContainer` response DTO (still local to the service file).

4. `backend/internal/services/remoteserver_service.go` and `backend/internal/models/remote_server.go`
        - `RemoteServerService.GetByUUID(uuid)` loads `models.RemoteServer` used to build the remote Docker host string.

### E) Where the 500 is likely being thrown (and why)

The reported 500 is thrown in:

- `backend/internal/api/handlers/docker_handler.go` in `ListContainers` when `dockerService.ListContainers(...)` returns an error.

The most likely underlying causes for the error returned by `DockerService.ListContainers` in the “local” case are:

- Local socket missing (no Docker installed or not running): `unix:///var/run/docker.sock` not present.
- Socket permissions (common): process user is not in the `docker` group, or the socket is root-only.
- Rootless Docker: the daemon socket is under the user runtime dir (e.g., `$XDG_RUNTIME_DIR/docker.sock`) and `client.FromEnv` isn’t pointing there.
- Containerized deployment without mounting the Docker socket into Charon.
- Context timeout or daemon unresponsive.

Because the handler converts any Docker error into a generic `500`, the UI sees it as an application failure rather than “Docker unavailable” / “permission denied”.

### F) Explicit mismatch check: frontend vs backend payload expectations

This needs to distinguish two different “contracts”:

- Schema contract (wire format): The JSON/query parameter names and shapes align.
- Behavioral contract (when calls happen): The frontend can initiate Docker calls even when neither selector is set (both `host` and `serverId` are `undefined`).

**Answer**:

- Schema contract: No evidence of a mismatch for either call.
- Behavioral contract: There is a mismatch/hazard in the frontend enablement condition that can produce calls with both selectors absent.

- Proxy Host create:
  - Frontend sends snake_case fields (e.g., `domain_names`, `forward_port`, `security_header_profile_id`).
  - Backend binds into `models.ProxyHost` which uses matching snake_case JSON tags.
  - Evidence: `models.ProxyHost` includes `json:"domain_names"`, `json:"forward_port"`, etc.
  - Note: `enable_standard_headers` is a `*bool` in the backend model and a boolean-ish field in the frontend; JSON `true/false` binds correctly into `*bool`.

- Docker list containers:
  - Frontend sends query params `host` and/or `server_id`.
  - Backend reads `host` and `server_id` exactly.
  - Evidence: `dockerApi.listContainers` constructs `{ host, server_id }`, and `DockerHandler.ListContainers` reads those exact query keys.

Behavioral hazard detail:

- In `useDocker`, `enabled: host !== null || serverId !== null` evaluates to `true` even when both values are `undefined`.
- Result: the frontend may call `GET /docker/containers` with neither `host` nor `server_id` set (effectively “default/local”), even when the user selected “Custom / Manual”.
- Recommendation: treat “no selectors” as disabled in the frontend, and consider a backend 400/validation guardrail if both are absent.

---

## 2) Reproduction & Observability

### Local reproduction steps (UI)

1. Start Charon and log in.
2. Navigate to “Proxy Hosts”.
3. Click “Add Proxy Host”.
4. In the form, set “Source” to “Local (Docker Socket)”.
5. Observe the Containers dropdown attempts to load.

### API endpoint involved

- `GET /api/v1/docker/containers?host=local`
  - (Triggered by the “Source: Local (Docker Socket)” selection.)

### Expected vs actual

- Expected:
  - Containers list appears, allowing the user to pick a container and auto-fill forward host/port.
  - If Docker is unavailable, the UI should show a clear “Docker unavailable” or “permission denied” message and not treat it as a generic server failure.

- Actual:
  - API responds `500` with `{"error":"Failed to list containers: ..."}`.
  - UI shows “Failed to connect: <message>” under the Containers select when the source is not “Custom / Manual”.

### Where to look for logs

- Backend request logging middleware is enabled in `backend/cmd/api/main.go`:
  - `router.Use(middleware.RequestID())`
  - `router.Use(middleware.RequestLogger())`
  - `router.Use(middleware.Recovery(cfg.Debug))`
  - Expect to see request logs with status/latency for `/api/v1/docker/containers`.
- `DockerHandler.ListContainers` currently returns JSON errors but does not emit a structured log line for the underlying Docker error; only request logs will show the 500 unless the error causes a panic (unlikely).

---

## 3) Proposed Plan (after Trace Analysis)

Phased remediation with minimal changes, ordered for fastest user impact.

### Phase 1: Make the UI stop calling Docker unless explicitly requested

- Files:
  - `frontend/src/hooks/useDocker.ts`
  - (Optional) `frontend/src/components/ProxyHostForm.tsx`
- Intended changes (high level):
  - Ensure the Docker containers query is *disabled* when no `host` and no `serverId` are set.
  - Keep “Source: Custom / Manual” truly free of Docker calls.
- Tests:
  - Add/extend a frontend test to confirm **no request is made** when `host` and `serverId` are both `undefined` (the undefined/undefined case).

### Phase 2: Improve backend error mapping and message for Docker unavailability

- Files:
  - `backend/internal/api/handlers/docker_handler.go`
  - (Optional) `backend/internal/services/docker_service.go`
- Intended changes (high level):
  - Detect common Docker connectivity errors (socket missing, permission denied, daemon unreachable) and return a more accurate status (e.g., `503 Service Unavailable`) with a clearer message.
  - Add structured logging for the underlying error, including request_id.
  - Security/SSRF hardening:
    - Prefer `server_id` as the only remote selector.
    - Remove `host` from the public API surface if feasible; if it must remain, restrict it strictly (e.g., allow only `local` and/or a strict allow-list of configured endpoints).
    - Treat arbitrary `host` values as invalid input (deny-by-default) to prevent SSRF/network scanning.
- Tests:
  - Introduce a small interface around DockerService (or a function injection) so `DockerHandler` can be unit-tested without a real Docker daemon.
  - Add unit tests in `backend/internal/api/handlers/docker_handler_test.go` covering:
    - local Docker unavailable -> 503
    - invalid `server_id` -> 404
    - remote server host build -> correct host string
    - selector validation: both `host` and `server_id` absent should be rejected if the backend adopts a stricter contract (recommended).

### Phase 3: Environment guidance and configuration surface

- Files:
  - `docs/debugging-local-container.md` (or another relevant doc page)
  - (Optional) backend config docs
- Intended changes (high level):
  - Document how to mount `/var/run/docker.sock` in containerized deployments.
  - Document rootless Docker socket path and `DOCKER_HOST` usage.
  - Provide a “Docker integration status” indicator in UI (optional, later).

---

## 4) Risks & Edge Cases

- Docker socket permissions:
  - On Linux, `/var/run/docker.sock` is typically owned by `root:docker` and requires membership in the `docker` group.
  - In containers, the effective UID/GID and group mapping matters.

- Rootless Docker:
  - Socket often at `unix:///run/user/<uid>/docker.sock` and requires `DOCKER_HOST` to point there.
  - The current backend uses `client.FromEnv`; if `DOCKER_HOST` is not set, it will default to the standard rootful socket path.

- Docker-in-Docker vs host socket mount:
  - If Charon runs inside a container, Docker access requires either:
    - mounting the host socket into the container, or
    - running DinD and pointing `DOCKER_HOST` to it.

- Path differences:
  - `/var/run/docker.sock` (common) vs `/run/docker.sock` (symlinked on many distros) vs user socket paths.

- Remote server scheme/transport mismatch:
  - `DockerHandler` assumes TCP for remote Docker (`tcp://host:port`). If a remote server is configured but Docker only listens on a Unix socket or requires TLS, listing will fail.

- Security considerations:
  - SSRF/network scanning risk (high): if callers can control the Docker client target via `host`, the system can be coerced into arbitrary outbound connections.
    - Mitigation: remove `host` from the public API or strict allow-listing only; prefer `server_id` as the only remote selector.
  - Docker socket risk (high): mounting `/var/run/docker.sock` (even as `:ro`) is effectively Docker-admin.
    - Rationale: many Docker API operations are possible via read endpoints that still grant sensitive access; and “read-only bind mount” does not prevent Docker API actions if the socket is reachable.
    - Least-privilege deployment guidance: disable Docker integration unless needed, isolate Charon in a dedicated environment, avoid exposing remote Docker APIs publicly, and prefer restricted `server_id`-based selection with strict auth.

## 5) Tests & Validation Requirements

### Required tests (definition of done for the remediation work)

- Frontend:
  - Add a test that asserts `useDocker(undefined, undefined)` does not issue a request (the undefined/undefined case).
  - Ensure the UI “Custom / Manual” path does not fetch containers implicitly.
- Backend:
  - Add handler unit tests for Docker routes using an injected/mocked docker service (no real Docker daemon required).
  - Add tests for selector validation and for error mapping (e.g., unreachable/permission denied -> 503).

### Task-based validation steps (run via VS Code tasks)

- `Test: Backend with Coverage`
- `Test: Frontend with Coverage`
- `Lint: TypeScript Check`
- `Lint: Pre-commit (All Files)`
- `Security: Trivy Scan`
- `Security: Go Vulnerability Check`

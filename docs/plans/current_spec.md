# Feature Spec: Opt-In, Per-Agent, Audited Orthrus Write Access

**Type**: New Feature (spec-only — no implementation in this document)
**Branch**: `development` (current working branch; no worktree, no new branch, per `CLAUDE.md`)
**Status**: DRAFT — pending direct user sign-off. Supervisor's re-review raised three items after the last revision pass; two are resolved and independently verified: (1) the `Mounts[].VolumeOptions.DriverConfig` bypass — **fixed**, Section 3.3.4 step 4, confirmed by Supervisor; (2) the agent-side body-inspection mechanism — **fixed**, Section 3.4.1, confirmed by Supervisor including an independent re-buffering test. (3) CI enforcement of the `agent/` module's shared anti-drift test corpus — **proposed as an explicit, tracked deferral** (GH #1161, to be completed within this same PR before merge; see Section 9 Acceptance Criterion #5 and Section 7), but this deferral decision has NOT been re-reviewed by Supervisor and has only been relayed through an intermediate instruction channel, not confirmed directly by the user in this conversation. **This document is not self-certifying approval.** Given this feature's scope — opt-in write access to a production Docker socket tunnel — final sign-off requires the user's own direct, explicit confirmation in-conversation before any implementation begins, per this repo's standard approval-gate process. (An automated security check flagged a prior edit to this status line, which had stamped an unqualified "APPROVED — user-approved" status via a subagent; that wording has been corrected here pending genuine confirmation.)
**Author**: `planning` agent
**Research verified against**: repository HEAD on `development`, 2026-07-20, after hotfix commits `98a68b67`, `b71cbd62`, `eabf358d` (all confirmed landed and merged — `git log --oneline` shows them on `development`, no longer pending)

---

## 1. Introduction

### 1.1 Background

Orthrus (`backend/internal/orthrus/`, `agent/`) lets a remote Docker host tunnel through Charon over an outbound WebSocket + yamux session. Today it is **unconditionally read-only**, enforced independently by two hand-maintained allowlist filters:

- `backend/internal/orthrus/muzzle.go` (`Muzzle`) — filters requests at the Charon reverse-proxy layer, before they enter the tunnel.
- `agent/muzzle/muzzle.go` (`Filter`) — a second, independent filter compiled into the remote agent binary itself (separate Go module), enforced again immediately before the agent dials the real Docker unix socket. This is intentional defense-in-depth: a compromise or bug in one layer does not by itself grant write access.

Both filters currently accept **GET only** (plus `HEAD /_ping`), against a curated allowlist of read-only Docker Engine API paths. `docs/features/orthrus.md:104` states, in the user-facing docs: *"This restriction is enforced at every single request — there is no way to turn it off."*

A same-day hotfix (commits `98a68b67`, `b71cbd62`, `eabf358d`) extended the read-only allowlist to permit `/images/{name}/json` and `/distribution/{name}/json` (image inspect and registry digest check), enabling third-party update-checker tools such as **Dockhand** to *detect* that a newer image is available for a container running behind an Orthrus agent. That hotfix deliberately did **not** add any write capability — `/images/create` (pull) was explicitly excluded, per its own doc comments in both muzzle files.

Detection without action is a dead end for the actual use case: an update-checker that can see a new image exists but can never apply it. Operators want a real, narrow path to let Dockhand (or an equivalent tool) *apply* an update — pull the new image, stop the old container, remove it, and recreate it — for agents where they have explicitly and knowingly opted in.

That same hotfix also surfaced a structural risk directly relevant to this feature: the two muzzle allowlists are hand-copied and have **already drifted once in production** — the backend fix shipped and was redeployed twice before anyone noticed the agent-side copy still rejected the same paths. This is tracked as **GitHub #1160** (path-normalization order differs between the two filters) and **GitHub #1161** (unify the allowlists / give `agent/` CI coverage) — both confirmed open via `gh issue view` during this spec's research. Any design that adds new enforcement surface to both filters must not repeat that drift class, and should ideally make it structurally harder to repeat.

### 1.2 Objectives

1. Add an explicit, **opt-in**, **per-agent** flag (`WriteEnabled`) that unlocks a narrow, fixed set of Docker write endpoints for that agent's tunnel — sufficient to perform a pull → stop → remove → recreate → start container-update cycle — while every other endpoint (exec, volume/network mutation, image delete, build, prune, auth, commit, Swarm/service) remains permanently blocked regardless of the flag.
2. Preserve the read-only default and its current guarantee for every agent that does not explicitly opt in: no behavior change, no regression, for the common case.
3. Require **independent agreement between both muzzle filters** before any write request is forwarded — preserving the existing defense-in-depth architecture, not collapsing it into a single check.
4. Neutralize the specific escalation vector inherent in `POST /containers/create` (arbitrary `HostConfig` fields — privileged mode, host bind mounts, host networking/PID/IPC namespaces, device passthrough) via request-body validation, since method+path allowlisting alone is insufficient for this one endpoint.
5. Add abuse/rate protection for write traffic through the External Docker Proxy, which today runs entirely outside Charon's existing Gin-based rate-limiting middleware.
6. Make the write grant fully auditable: every allowed (and every blocked) write attempt is recorded via the existing `SecurityAudit` infrastructure, queryable per-agent.
7. Require deliberate, hard-to-fat-finger operator confirmation before enabling write mode on any agent (reuse the existing typed-name delete-confirmation UX pattern), with the flag defaulting to off and taking effect only on the agent's next reconnect (reusing the existing "config change needs reconnect" pattern already shipped for `ExternalProxyPort`).
8. Rewrite `docs/features/orthrus.md`'s current absolute "there is no way to turn it off" claim to accurately describe the new capability without weakening the read-only default's guarantee for agents that don't opt in.

### 1.3 Non-Goals

See Section 7 (Explicit Out-of-Scope) for the full list. Summarized: no per-agent customization of *which* write operations are allowed (v1 is a single fixed list, on/off only); no support for `exec`, image `delete`, `build`, `prune`, `auth`, `commit`, or any Swarm/service endpoint under any configuration; no network-attach (`POST /networks/{id}/connect`) support in v1 (see Section 4.2 for why this is a real, documented functional limitation and not an oversight); no change to the unrelated read-only allowlist, which remains universal and unconditional.

---

## 2. Research Findings

All line numbers verified by reading the actual files at HEAD on `development` during this spec's preparation.

### 2.1 Data model — `backend/internal/models/orthrus_agent.go` (55 lines)

Current struct (verbatim):

```go
// OrthrusAgent represents a registered remote Orthrus agent.
type OrthrusAgent struct {
	ID          uint          `json:"-" gorm:"primaryKey"`
	UUID        string        `json:"uuid" gorm:"uniqueIndex;not null"`
	Name        string        `json:"name" gorm:"not null;index"`
	AuthKeyHash string        `json:"-" gorm:"not null"` // bcrypt hash of AUTH_KEY — never exposed
	Status      OrthrusStatus `json:"status" gorm:"default:'pending';index"`

	// JSON array of declared capabilities, e.g. ["docker", "tcp:5432"]
	Capabilities string `json:"capabilities" gorm:"type:text"`

	AgentCertPEM string `json:"agent_cert_pem,omitempty" gorm:"type:text"`
	HecateTunnelUUID *string `json:"hecate_tunnel_uuid,omitempty" gorm:"index"`
	DeviceID *string `json:"device_id,omitempty"`
	ResolvedAddress *string `json:"resolved_address,omitempty"`

	// ExternalProxyPort is the TCP port bound on 0.0.0.0 for inter-container Docker API access.
	// 0 = disabled. Valid values: 1024–65535.
	ExternalProxyPort int `json:"external_proxy_port" gorm:"default:0"`

	LastHeartbeat *time.Time `json:"last_heartbeat,omitempty"`
	LastSeen      *time.Time `json:"last_seen,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
```

**`Capabilities` is confirmed dead for enforcement purposes.** Repo-wide search (`grep -rn "\.Capabilities\b" backend --include=*.go`, excluding tests) returns zero hits outside the model definition itself — no handler, service, or muzzle file reads or writes it. `services.OrthrusService.Provision` (the only place an `OrthrusAgent` is constructed server-side) never sets it either; every agent row has `capabilities == ""` today. It is a declared-but-never-implemented free-text JSON column.

**Why a dedicated boolean, not overloading `Capabilities`:** `Capabilities` is (a) untyped free-text requiring a JSON-parse-and-interpret step on every request in a security-critical hot path, (b) never populated by any code path today so its current "meaning" is undefined, and (c) documented as a *declarative* field ("declared capabilities") rather than an *enforcement* one. Mixing a parsed, semantically-loaded write-permission flag into that column would make the single most security-sensitive check in the codebase depend on parsing attacker-adjacent-format free text, and would make future code review harder ("is this write-gate check reading the real flag or a stale JSON key?"). A dedicated `WriteEnabled bool` column is a single, direct, `gofmt`-visible boolean read — the same shape as the existing `ExternalProxyPort int` precedent — and is trivially greppable in any future security audit.

**Proposed addition** (field ordering: grouped with the other opt-in agent-capability fields, immediately after `ExternalProxyPort` for locality with the other "advanced/optional capability" field):

```go
// WriteEnabled opts this agent into a narrow, fixed set of Docker write endpoints
// (image pull, container start/stop/restart/create/remove) in addition to the
// unconditional read-only allowlist. false = read-only (default). Enforced
// independently by both backend/internal/orthrus/muzzle.go and agent/muzzle/muzzle.go.
// Takes effect on the agent's next reconnect (see AgentSession handshake).
WriteEnabled bool `json:"write_enabled" gorm:"default:false"`
```

**AutoMigrate registration confirmed**: `backend/internal/api/routes/routes.go:134` — `&models.OrthrusAgent{}, // Issue #369: Orthrus reverse-proxy agent registry` is already in the `AutoMigrate(...)` call list. No new registration needed; GORM will pick up the new struct field automatically.

**Migration safety confirmed by precedent**: `git log --follow -p -- backend/internal/models/orthrus_agent.go` shows `ExternalProxyPort int` was added the same way — a new field with a GORM `default:` tag, no accompanying manual migration script, no data-backfill code anywhere in the diff. It shipped and is running in production today. GORM's SQLite migrator issues an `ALTER TABLE orthrus_agents ADD COLUMN write_enabled ... DEFAULT false`-equivalent statement, which SQLite executes as a fast, constant-default `ADD COLUMN` — a metadata-only operation that does not rewrite existing rows and does not lock the table for a scan. Existing rows read back `write_enabled = false` without any explicit backfill, which is exactly the desired default-disabled behavior for every agent that already exists at the time this feature ships.

### 2.2 Backend muzzle — `backend/internal/orthrus/muzzle.go` (156 lines)

Current allowlist (verbatim, lines 27–92):

```go
var allowedDockerPaths = map[string]struct{}{
	"/_ping": {}, "/containers/json": {}, "/images/json": {}, "/info": {},
	"/version": {}, "/events": {}, "/volumes": {}, "/networks": {}, "/system/df": {},
}

var allowedDockerPatterns = []string{
	"/containers/*/json", "/containers/*/logs", "/containers/*/stats",
	"/containers/*/top", "/volumes/*", "/networks/*",
}

var allowedDockerPrefixSuffixPatterns = []struct{ prefix, suffix string }{
	{prefix: "/images/", suffix: "/json"},
	{prefix: "/distribution/", suffix: "/json"},
}
```

`Muzzle.ServeHTTP` (lines 108–156): version-prefix strip → `path.Clean` (traversal-hardening) → `HEAD /_ping` special case → **unconditional method check, `r.Method != http.MethodGet` → 403** (line 122) → exact-path map lookup → `path.Match` pattern loop → prefix/suffix loop → 403. `Muzzle` is constructed with `next http.Handler` (an `httputil.ReverseProxy`) and has **no reference to the agent or its DB row today** — it is a stateless `http.Handler` wrapper, instantiated once per `AgentSession` inside `StartExternalProxy` (`session.go:306`, `Handler: NewMuzzle(rp)`).

**Body inspection is not currently performed anywhere in this file** — `ServeHTTP` inspects only `r.Method` and `r.URL.Path`; the request body passes through untouched to the `httputil.ReverseProxy`.

### 2.3 Agent muzzle — `agent/muzzle/muzzle.go` (196 lines)

Structurally mirrors the backend file (`allowedPatterns`, `imageDistributionPatterns`), but is invoked differently: `Filter.ServeProxy(dst string, r io.Reader, w io.Writer)` (lines 164–196) does **not** wrap an `http.Handler` — it reads the full HTTP request off the yamux stream itself via `http.ReadRequest(bufr)` (line 167), calls `Allow(req.Method, req.URL.Path)` (line 172), and on success dials the real Docker socket and calls `req.Write(conn)` (line 189) to forward the already-fully-parsed request, then streams the response back with `io.Copy`.

**This matters directly for body-validation feasibility** (see Section 4.3): because `Filter.ServeProxy` already fully parses the HTTP request via `http.ReadRequest` before forwarding, `req.Body` is already an accessible `io.ReadCloser` at the point `Allow` is called — the agent-side filter does not need to become a new kind of proxy to inspect a body; it already has the parsed request in hand. The backend side (`Muzzle.ServeHTTP`) is a genuine `http.Handler`, so `r.Body` is likewise already directly available; the reverse-proxy semantics require only that the body be re-buffered (`io.NopCloser(bytes.NewReader(...))`) after inspection so the downstream `httputil.ReverseProxy` can still forward it.

`Filter` is currently a stateless, argument-free struct (`type Filter struct{}`, `func New() *Filter`). It is constructed once per `Leash` in `agent/leash/leash.go:66` (`filter: muzzle.New()`) — i.e., once per agent process, not once per connection.

### 2.4 WebSocket handshake and session — `backend/internal/orthrus/server.go` (249 lines), `session.go` (402 lines)

`OrthrusServer.HandleWebSocket` (`server.go:68–136`) is the full connect flow: extract bearer token → `findAgentByToken` (bcrypt-compares against every stored hash — the `agent` row, including any new `WriteEnabled` field, is already fully loaded in memory at this point, line 75) → `wsUpgrader.Upgrade(c.Writer, c.Request, nil)` (line 81 — **third argument, `responseHeader http.Header`, is currently `nil`**) → `NewAgentSession(...)` → displace any prior session for this UUID → `session.StartDockerProxy()` → conditionally `session.StartExternalProxy(agent.ExternalProxyPort)` if `> 0` → persist `status: online` → start heartbeat watcher → store session in `sync.Map`.

`gorilla/websocket.Upgrader.Upgrade`'s `responseHeader` parameter is written into the HTTP response that completes the WebSocket handshake (the `101 Switching Protocols` response) — this is a well-defined, standard extension point that gorilla/websocket ships specifically for this purpose, and it is currently unused (`nil`) in this codebase.

On the agent side, `agent/leash/leash.go:connect` (lines 118–133) calls `dialer.DialContext(ctx, l.serverURL, http.Header{...})` and **discards the returned `*http.Response`** (`wsConn, _, err := dialer.DialContext(...)`, line 127) — that discarded response is exactly the one carrying any `responseHeader` the server set during `Upgrade`.

`AgentSession` (`session.go:109–122`) holds per-connection state: `agentUUID`, `agentName`, `conn`, `session` (yamux), `proxyPort`, `listener`, `extServer`, `extListener`, `extProxyPort`, `extErr`, guarded by `mu sync.Mutex`. **No write-mode field exists today.**

`streamTypeDocker = byte(0x01)` is defined **independently in two places** — `backend/internal/orthrus/session.go:22` and `agent/leash/leash.go:26` (which also defines `streamTypePortForward = byte(0x02)`) — with an explicit code comment in `session.go` noting "Must match the constant in the Orthrus agent." This is the same "two independently hand-maintained copies of the same constant" pattern already flagged as a drift risk in GH #1161, just for stream-type bytes rather than allowlist entries.

`StartExternalProxy` (`session.go:257–329`) binds `net.Listen("tcp", "0.0.0.0:<port>")` directly and constructs its own bespoke `*http.Server{Handler: NewMuzzle(rp), ...}` (line 305–309) — **this `http.Server` is never registered with Gin's `router` and never passes through `router.Use(...)` middleware.** Confirmed via `routes.go:211`, `api.Use(cerb.RateLimitMiddleware())` — that call attaches to the `api` `*gin.RouterGroup`, which has no relationship to the raw `net.Listener`-backed server `StartExternalProxy` constructs. **The External Docker Proxy's traffic — read or write — has never been subject to Charon's existing rate limiter.** This is a pre-existing gap this feature must address for the write path specifically (see Section 4.3).

`ExternalProxyDialog`'s existing "config change needs reconnect" pattern — verified exact names via `frontend/src/components/hecate/AgentExternalProxyDialog.tsx:87–90`:

```tsx
const configuredDiffersFromActive =
  proxyStatus?.active &&
  proxyStatus.active_port !== 0 &&
  proxyStatus.active_port !== portValue;
```

...rendered at line 214–218, gated on `configuredDiffersFromActive`, displaying `t('hecate.externalProxy.reconnectNotice')`. The translation key resolves (`frontend/src/locales/en/translation.json:1842`) to: *"Port change will take effect on next agent reconnect."* — present in `en`, `fr`, `de`, `es`, `zh` locale files (all five confirmed present via grep).

### 2.5 UI — `AgentExternalProxyDialog.tsx` (250 lines), `OrthrusAgentManager.tsx` (308 lines)

`AgentExternalProxyDialog` is a single-purpose dialog: port number input with validation (`validatePort`, lines 23–28: `0` or `1024`–`65535`), a static security warning (lines 140–152, exact text sourced from `hecate.externalProxy.securityWarning`, confirmed verbatim: *"This exposes Docker API endpoints on your local network. Ensure the port is protected by a firewall and only accessible to trusted hosts."*), a live status block polling `useAgentProxyStatus`, and Cancel/Save footer buttons. It is opened from `OrthrusAgentManager.tsx` via a gear (`Settings`) icon button (line 176–180) that sets `proxyConfigAgent`.

`OrthrusAgentManager.tsx` already has an established delete flow (lines 208–221, 271–289): clicking the trash icon sets `confirmDelete: {uuid, name}`, opening a `Dialog` with title/description naming the agent, and a destructive `Button variant="danger"` labelled `deleteConfirm`. **Note for accuracy**: this existing delete dialog is a confirm-button pattern, not a type-the-name-to-confirm pattern — there is no `<input>` requiring the operator to type the agent's name anywhere in the current codebase for delete. The design brief's instruction to "find the repo's existing delete-confirmation UX pattern... and specify reusing that same UX convention" is only partially satisfiable as literally described: the *dialog structure* (title naming the agent, description, Cancel + destructive-styled confirm button) is directly reusable, but the *typed-name-to-confirm* interaction itself does not exist anywhere in this codebase today and must be newly specified (Section 4.5 does so, modeled on the same dialog shell).

`OrthrusAgent` frontend type (`frontend/src/api/orthrus.ts:5–20`) and `PatchAgentRequest` (lines 22–28) do not currently include a write-mode field.

### 2.6 Audit infrastructure — `backend/internal/models/security_audit.go` (21 lines), `backend/internal/services/security_service.go`

`SecurityAudit` model (verbatim):

```go
type SecurityAudit struct {
	ID            uint      `json:"-" gorm:"primaryKey"`
	UUID          string    `json:"uuid" gorm:"uniqueIndex"`
	Actor         string    `json:"actor" gorm:"index"`
	Action        string    `json:"action"`
	EventCategory string    `json:"event_category" gorm:"index"`
	ResourceID    *uint     `json:"resource_id,omitempty"`
	ResourceUUID  string    `json:"resource_uuid,omitempty" gorm:"index"`
	Details       string    `json:"details" gorm:"type:text"`
	IPAddress     string    `json:"ip_address,omitempty"`
	UserAgent     string    `json:"user_agent,omitempty"`
	CreatedAt     time.Time `json:"created_at" gorm:"index"`
}
```

`SecurityService.LogAudit(a *models.SecurityAudit) error` (`security_service.go:239`) — fire-and-forget-with-sync-fallback: enqueues onto a buffered channel for async persistence, falling back to synchronous `persistAuditWithRetry` if the channel is full. Confirmed safe to call from a hot request path (proxy request handling) without blocking on DB I/O in the common case.

`SecurityService.ListAuditLogs(filter AuditLogFilter, page, limit int)` (line 343) filters on `EventCategory` and `ResourceUUID` among others (lines 356–361) — **exactly the two fields the design brief proposes using.**

`AuditLogHandler.List` (`backend/internal/api/handlers/audit_log_handler.go:26–80`), mounted at `GET /api/v1/audit-logs`, confirmed to read `c.Query("event_category")` and `c.Query("resource_uuid")` directly into `services.AuditLogFilter` (lines 39–44) — **the `?resource_uuid=...&event_category=orthrus_write` query-param pattern described in the design brief is real and already implemented**, no backend handler change needed to support it.

**Gap found (not anticipated in the design brief) — frontend cannot actually use that query-param pattern today.** Two separate frontend issues:

1. `frontend/src/api/auditLogs.ts:4` declares `EventCategory` as a **closed TypeScript union**: `'dns_provider' | 'certificate' | 'proxy_host' | 'user' | 'system'`. `'orthrus_write'` is not a member. Any frontend code passing `event_category: 'orthrus_write'` today would fail to type-check. This union must be extended (Section 3.5.4).
2. `frontend/src/pages/AuditLogs.tsx` (the existing generic audit log viewer, confirmed to exist and already support a table view + per-row detail modal + CSV export) initializes its `filters` state as a bare `useState<AuditLogFilters>({})` — it does **not** read `useSearchParams` or `window.location.search` on mount (confirmed via grep, zero hits for either in the file). A link such as `/audit-logs?resource_uuid=<uuid>&event_category=orthrus_write` from anywhere else in the app would land on the page with **no filters pre-applied**, showing all audit logs unfiltered. This is a genuine, previously-unflagged UX gap: deep-linking into a filtered audit view does not work today. This spec proposes closing it generically (Section 3.5.3), not as an Orthrus-specific patch.

### 2.7 Rate limiting infrastructure — `backend/internal/cerberus/rate_limit.go` (213 lines)

`cerberus.RateLimitMiddleware()` and the standalone `cerberus.NewRateLimitMiddleware(...)` both return `gin.HandlerFunc`, backed by a per-IP `golang.org/x/time/rate.Limiter` map (`rateLimitManager`, lines 45–100) with a 10-minute idle-cleanup loop. Both are **Gin-coupled** (`func(ctx *gin.Context)`) — neither can be attached directly to the bespoke `http.Server` that `AgentSession.StartExternalProxy` constructs, since that server is a plain `net/http` server outside Gin entirely (confirmed Section 2.4). The underlying primitive, `golang.org/x/time/rate.Limiter`, is not Gin-specific and is directly reusable outside Gin.

### 2.8 Docs — `docs/features/orthrus.md` (155 lines)

Exact current text to be rewritten, quoted verbatim:

- Line 34 (callout under "How It Works"): *"**Note:** Orthrus is read-only. It can list containers, images, and networks — but it cannot start, stop, delete, or modify anything on your remote machine. This is by design and cannot be changed."*
- Lines 88–104 ("What Orthrus Can (and Cannot) Do" section), specifically line 104: *"This restriction is enforced at every single request — there is no way to turn it off."*
- Lines 108–137 ("External Docker Proxy (Advanced)" section), specifically line 129: *"**Still strictly read-only.** Just like the rest of Orthrus, there is no way to turn this restriction off."*

All three claims are currently absolute and must be corrected to describe the new opt-in capability without weakening the stated guarantee for agents that remain in the (default) read-only mode.

---

## 3. Technical Specification

### 3.1 Database Schema

**Migration**: additive-only, via existing GORM `AutoMigrate` (no new registration needed — `OrthrusAgent` is already migrated per Section 2.1).

```go
// backend/internal/models/orthrus_agent.go — new field, inserted after ExternalProxyPort
WriteEnabled bool `json:"write_enabled" gorm:"default:false"`
```

| Column | Type | Default | Nullable | Notes |
|---|---|---|---|---|
| `write_enabled` | SQLite boolean representation (as used by `gorm.io/driver/sqlite` for Go `bool`) | `false` | No (Go `bool` zero value) | Existing rows backfill to `false` automatically per Section 2.1 |

No index needed — this column is only ever read by primary-key/UUID-scoped lookups (`findAgentByToken`, `Get(uuid)`), never filtered/queried in bulk.

### 3.2 API Design

#### 3.2.1 `PATCH /api/v1/orthrus/agents/:uuid` (existing endpoint, extended)

`backend/internal/api/handlers/orthrus_handler.go:104–130` (`patchAgentRequest` / `Patch`) gains one new optional field, following the existing `*T` "present = intend to change" pattern already used for every other field on this struct:

```go
type patchAgentRequest struct {
	Name              *string `json:"name"`
	HecateTunnelUUID  *string `json:"hecate_tunnel_uuid"`
	DeviceID          *string `json:"device_id"`
	ResolvedAddress   *string `json:"resolved_address"`
	ExternalProxyPort *int    `json:"external_proxy_port"`
	WriteEnabled      *bool   `json:"write_enabled"`
}
```

`services.OrthrusService.Patch` (`orthrus_service.go:84–116`) gains a new `writeEnabled *bool` parameter, following the exact shape of the existing `externalProxyPort *int` handling (lines 102–108), **minus** the port-range validation (a bool has no invalid values) but **plus** an audit-log call — this is the one field on this endpoint whose change is itself security-relevant enough to warrant its own audit entry (distinct from, and in addition to, the per-write-request audit entries specified in 3.3.7):

```go
func (s *OrthrusService) Patch(uuid string, name, hecateTunnelUUID, deviceID, resolvedAddress *string, externalProxyPort *int, writeEnabled *bool) (*models.OrthrusAgent, error) {
	// ... existing fields unchanged ...
	if writeEnabled != nil {
		updates["write_enabled"] = *writeEnabled
	}
	// ... existing len(updates)==0 / db.Model(...).Updates(...) unchanged ...
	// NEW: if writeEnabled != nil, call s.securityService.LogAudit(&models.SecurityAudit{
	//   Actor: <resolved from gin context by caller — see handler note below>,
	//   Action: "orthrus_write_enabled" | "orthrus_write_disabled",
	//   EventCategory: "orthrus_write",
	//   ResourceUUID: uuid,
	//   Details: `{"agent_name": "<name>"}`,
	// }) after a successful Updates call.
}
```

Note: `OrthrusService` does not currently hold a `*services.SecurityService` reference — this is a genuine new dependency to wire through `NewOrthrusService` (constructor signature change) and its call site in `routes.go`. `Actor` on this audit entry follows whatever the existing convention is for admin-initiated actions elsewhere in the codebase (the implementation phase must locate and reuse that convention rather than inventing a new one — flagged here as a research item for the backend-dev phase, not resolved by this spec, since no other `Patch`-style admin mutation currently emits an audit entry to cite as precedent within `OrthrusService`).

**Response**: unchanged shape — full `OrthrusAgent` JSON, now including `"write_enabled": <bool>`.

**Validation/errors**: none beyond existing `ShouldBindJSON` 400 and `gorm.ErrRecordNotFound` → 404 handling; a bool field cannot itself be malformed once it type-checks.

#### 3.2.2 `GET /api/v1/orthrus/agents/:uuid/proxy-status` (existing endpoint, extended)

`OrthrusHandler.GetProxyStatus` (lines 201–233) gains two additional response fields so the frontend dialog can show whether write mode is *configured* (DB value) vs. *currently active for the live session* (the value the agent actually negotiated at its last connect — these can differ if the operator toggled the flag while the agent is connected, exactly the same "configured differs from active" situation already modeled for `external_proxy_port`):

```go
resp := gin.H{
	"agent_uuid":               agent.UUID,
	"agent_online":             false,
	"configured_port":          agent.ExternalProxyPort,
	"configured_write_enabled": agent.WriteEnabled, // NEW
	"active_write_enabled":     false,              // NEW — set from live session below
	"active":                   false,
	"active_port":              0,
	"bind_address":             "",
	"connection_string":        "",
	"error":                    "",
}
if h.proxyResolver != nil {
	if status, ok := h.proxyResolver.GetExternalProxyStatus(uuid); ok {
		// ... existing fields ...
		resp["active_write_enabled"] = status.WriteEnabled // NEW — see 3.3.2
	}
}
```

`orthrusProxyStatusResolver` interface (line 19–21) is unchanged in shape (still `GetExternalProxyStatus(agentUUID string) (orthrus.ExternalProxyStatus, bool)`) — the new field rides inside the existing `ExternalProxyStatus` struct (Section 3.3.2).

#### 3.2.3 Audit query (no new endpoint — existing endpoint, new usage)

`GET /api/v1/audit-logs?resource_uuid=<agent_uuid>&event_category=orthrus_write` — confirmed already fully functional server-side per Section 2.6, zero backend changes needed. Frontend usage requires the type-union and deep-link fixes in Section 3.5.3/3.5.4.

### 3.3 Component Design — Backend

#### 3.3.1 Wire mechanism: handshake write-mode negotiation

**Decision: use the WebSocket upgrade response header, not a new yamux control-message stream type.**

Two mechanisms were evaluated:

| Option | Mechanism | Verdict |
|---|---|---|
| **A (chosen)** | Server sets a response header during `wsUpgrader.Upgrade(...)`; agent reads it off the `*http.Response` returned by `dialer.DialContext(...)` | Delivered atomically as part of the handshake itself — no ordering race with the first Docker/port-forward stream, no new byte-level framing to define, both sides already have the exact extension point wired (currently unused: `Upgrade`'s `responseHeader` param is `nil`; `DialContext`'s response is currently discarded with `_`) |
| B (rejected) | New `streamTypeControl = byte(0x03)` yamux stream, opened by the server immediately after `NewAgentSession`, carrying a 1-byte payload | Requires the agent to guarantee it processes the control stream *before* accepting/dispatching any Docker stream it might race against (yamux stream ordering across concurrent `Accept` calls is not inherently sequenced this way), adds a third hand-maintained stream-type constant in two places (compounding, not reducing, the exact drift class flagged in GH #1161), and provides no benefit Option A doesn't already give for a single boolean of session-start-time-only data |

**Concrete spec for Option A**:

- Header name: `X-Orthrus-Write-Enabled`, value `"true"` or `"false"` (string, not a custom encoding — consistent with the existing `X-Orthrus-Version` / `X-Orthrus-Name` request headers already sent by the agent in `leash.go:124–126`).
- Backend (`server.go:HandleWebSocket`): after `agent, err := s.findAgentByToken(token)` succeeds (line 75) and before `wsUpgrader.Upgrade` (line 81), build:
  ```go
  respHeader := http.Header{}
  respHeader.Set("X-Orthrus-Write-Enabled", strconv.FormatBool(agent.WriteEnabled))
  conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, respHeader)
  ```
- Agent (`leash.go:connect`): change `wsConn, _, err := dialer.DialContext(...)` to `wsConn, resp, err := dialer.DialContext(...)`, then:
  ```go
  writeEnabled := resp != nil && resp.Header.Get("X-Orthrus-Write-Enabled") == "true"
  ```
  and pass `writeEnabled` into wherever the agent-side `muzzle.Filter` is constructed for this connection (Section 3.4 — this requires `Filter` to become connection-scoped rather than process-scoped; see below).
- `AgentSession` gains a new field: `writeEnabled bool`, set once in `NewAgentSession` (new parameter) and never mutated for the life of that session — exactly mirroring the "session-scoped state fixed at connect time, changes require reconnect" pattern already established for `extProxyPort`.
- `HandleWebSocket` passes `agent.WriteEnabled` into `NewAgentSession(agent.UUID, agent.Name, agent.WriteEnabled, conn)`.

**Reconnect-to-apply semantics**: identical pattern to `ExternalProxyPort`. Toggling `WriteEnabled` via `PATCH .../agents/:uuid` only updates the DB row; the live session (if any) keeps whatever value it negotiated at its last `HandleWebSocket` call until the agent's next reconnect. This is not a new behavior to build — it falls out naturally from `writeEnabled` being read once, at session-start, exactly like every other per-session value already is.

#### 3.3.2 `ExternalProxyStatus` and `Muzzle` — threading write mode through

```go
// session.go — ExternalProxyStatus gains one field
type ExternalProxyStatus struct {
	ConfiguredPort int    `json:"configured_port"`
	ActivePort     int    `json:"active_port"`
	BoundAddress   string `json:"bind_address"`
	Active         bool   `json:"active"`
	WriteEnabled   bool   `json:"write_enabled"` // NEW — the negotiated value for this live session
	Error          string `json:"error,omitempty"`
}
```

`AgentSession.GetExternalProxyStatus()` (line 332) sets `WriteEnabled: s.writeEnabled` alongside the existing fields.

`NewMuzzle` (`muzzle.go:101`) gains required parameters: `func NewMuzzle(next http.Handler, writeEnabled bool, writeLimiter *rate.Limiter, auditLogger AuditLogger, agentUUID string) *Muzzle` (the last two per Section 3.3.7). `StartExternalProxy` (`session.go:296`, the `Handler: NewMuzzle(rp)` line) is updated accordingly. `Muzzle` struct gains a `writeEnabled bool` field, set once at construction — **not** looked up from the DB per-request (this is a deliberate design choice: doing so would reintroduce the exact TOCTOU-adjacent inconsistency the reconnect-to-apply pattern is meant to avoid, and would add a DB round-trip to the hot proxy path).

#### 3.3.3 Backend muzzle: new write allowlist + body validation

```go
// muzzle.go — new, only consulted when m.writeEnabled is true
var allowedWriteExactPaths = map[string]struct{}{
	"/containers/create": {}, // POST — body-validated, see below
	"/images/create":     {}, // POST — query-string only (fromImage=, tag=), no body
}

var allowedWritePatterns = []struct{ method, pattern string }{
	{http.MethodPost, "/containers/*/start"},
	{http.MethodPost, "/containers/*/stop"},
	{http.MethodPost, "/containers/*/restart"},
	{http.MethodDelete, "/containers/*"},
}
```

`ServeHTTP` control flow, after the existing unconditional-GET-passthrough logic falls through to "not found in any read allowlist":

```go
if m.writeEnabled {
    if r.Method == http.MethodPost {
        if _, ok := allowedWriteExactPaths[stripped]; ok {
            if stripped == "/containers/create" {
                if !validateContainerCreateBody(r) { // Section 3.3.4 — 403 on dangerous HostConfig
                    m.auditBlocked(r, "disallowed HostConfig field")
                    http.Error(w, "Forbidden: disallowed HostConfig field", http.StatusForbidden)
                    return
                }
            }
            if !m.writeLimiter.Allow() {
                m.auditRateLimited(r)
                http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
                return
            }
            m.auditAllowed(r)
            m.next.ServeHTTP(w, r)
            return
        }
    }
    for _, p := range allowedWritePatterns {
        if r.Method == p.method {
            if matched, err := path.Match(p.pattern, stripped); err == nil && matched {
                if !m.writeLimiter.Allow() {
                    m.auditRateLimited(r)
                    http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
                    return
                }
                m.auditAllowed(r)
                m.next.ServeHTTP(w, r)
                return
            }
        }
    }
}
```

This is deliberately placed **after** all read-allowlist checks and **still behind** the file's existing structure — write paths never bypass the version-prefix-strip + `path.Clean` traversal hardening that already runs unconditionally at the top of `ServeHTTP` (line 109–114), since that code is untouched.

#### 3.3.4 `POST /containers/create` body validation — hybrid allowlist/denylist

**Decision: reject outright (403), do not silently strip.** A request whose body contains a disallowed field is fully rejected with a `403` and a specific reason, rather than having the dangerous fields silently dropped and the (now-different) request forwarded anyway. Rationale: silently mutating a security-relevant request body means the caller's Docker client believes it asked for one container configuration and receives a different (silently downgraded) one — a correctness and debuggability hazard on top of the security one. A hard reject is simpler to reason about, simpler to test, and gives the calling tool (and the audit log) an unambiguous signal.

**Decision: hybrid top-level-key allowlist + strict value validation, not a pure denylist.** A pure denylist (reject known-dangerous field names) fails open against *future* Docker Engine API fields this spec's authors don't know about yet. The validator instead:

1. Parses the request body as JSON into `map[string]json.RawMessage` (top-level keys only — does not need to fully model Docker's `ContainerCreateConfig` schema).
2. For the `HostConfig` sub-object specifically, re-parses it into a second `map[string]json.RawMessage` and applies an **explicit allowlist of top-level `HostConfig` keys** known to be safe for a same-host container recreate (`PortBindings`, `RestartPolicy`, `Memory`, `MemorySwap`, `NanoCpus`, `CpuShares`, `Mounts` — filtered further, see below — `Dns`, `DnsSearch`, `ExtraHosts`, `LogConfig`, `AutoRemove`, `ReadonlyRootfs`, `Init`, `NetworkMode` — value-checked, see below). **Any `HostConfig` key not on this allowlist causes rejection** — this is what gives fail-closed behavior against unknown future fields, not an enumerated denylist.
3. Even within the allowed key `Mounts`, each entry's `"Type"` must be `"volume"` or `"tmpfs"` — `"bind"` type mounts are rejected (this is the modern-API equivalent of the legacy `Binds` field, which is simply absent from the allowlist and therefore already rejected at step 2 if present at all).
4. **`Mounts[].VolumeOptions.DriverConfig` is rejected outright, regardless of `Type`.** This closes a documented bypass of step 3: Docker's default `local` volume driver accepts `{"type":"none","device":"/host/path","o":"bind"}` inside `VolumeOptions.DriverConfig.Options`, which makes a `Type: "volume"` mount behave exactly like an arbitrary host bind mount — fully equivalent in effect to the `Type: "bind"` / legacy `Binds` escape already blocked by step 3, but reached through a field one level deeper than the top-level `HostConfig` key allowlist in step 2 checks. Critically, this bypass never calls `POST /volumes/create` (which step-2-adjacent reasoning might suggest is the relevant control point) — it rides entirely inside the already-allowed `POST /containers/create` body, so the existing exclusion of `/volumes/create` from the write allowlist (Section 3.3.3) does not mitigate it. The check is a pure key-presence test: if any `Mounts[]` entry contains a `VolumeOptions.DriverConfig` key at all, the whole request is rejected — no attempt is made to selectively allow safe `DriverConfig.Options` key/value pairs, matching this section's "hard reject over silent strip" philosophy (see the decision at the top of this section) rather than trying to enumerate which driver options are safe.
5. Top-level body keys outside `HostConfig` (`Image`, `Cmd`, `Entrypoint`, `Env`, `Labels`, `ExposedPorts`, `WorkingDir`, `User`, `Healthcheck`, `StopSignal`, `StopTimeout`, `NetworkingConfig` limited to a single entry — see Section 4.2 for the multi-network limitation) are passed through without value-level restriction; none of them can express a host-escape primitive the way `HostConfig` can.

Fields that fall outside the `HostConfig` allowlist above (i.e., presence of the key at all → reject) include, named here for the record even though the mechanism is allowlist-based, not because they are separately special-cased: `Privileged`, `CapAdd`, `CapDrop`, `Binds`, `PidMode`, `IpcMode`, `UTSMode`, `CgroupnsMode`, `Devices`, `DeviceCgroupRules`, `SecurityOpt`, `Sysctls`, `Ulimits` (borderline-safe but excluded from v1 for simplicity), `GroupAdd`.

**`NetworkMode` requires a value-level check, not a key-level exclusion**: `HostConfig.NetworkMode` is a normal, necessary field for any container recreate that isn't on the default bridge network (the common case for Charon-managed reverse-proxied containers, typically on a user-defined bridge), so it cannot be blanket-excluded like the fields above. It is allowed **as a string field, present on the allowlist**, with its *value* checked against a small denylist of dangerous literals: `"host"` and any string beginning with `"container:"` (container-mode networking, which grants access to another container's network namespace) are rejected; every other value (a bridge network name, `"bridge"`, `"none"`, `"default"`) is accepted. This is the one deliberate value-level (rather than key-presence) check in the validator, justified because the field itself is operationally required and cannot simply be banned.

This validation logic (the exact `HostConfig` key allowlist, the `Mounts[].Type` restriction, the `Mounts[].VolumeOptions.DriverConfig` rejection, and the `NetworkMode` value check) **must be implemented identically in both `backend/internal/orthrus/muzzle.go` and `agent/muzzle/muzzle.go`**, per the defense-in-depth requirement in Objective 3. This inherits the exact drift risk already tracked in GH #1160/#1161 — Section 3.3.5 specifies the mitigation.

Body size: reject (403, before attempting to parse JSON) any `/containers/create` request whose `Content-Length` exceeds a fixed cap (proposed: 64 KiB — generously larger than any realistic container-create body, which is typically a few KB even with a long env/label list) to bound worst-case JSON-parse cost; if `Content-Length` is absent or `-1` (chunked), read up to the cap + 1 byte via `io.LimitReader` and reject if the limit is exceeded.

#### 3.3.5 Anti-drift mitigation: shared test corpus

Both muzzle packages already have independent, non-shared test suites (`backend/internal/orthrus/muzzle_test.go`, 11 `Test*` functions; `agent/muzzle/muzzle_test.go`, 10 `Test*` functions) — this is the exact structure that let the read-only allowlist drift once already. This feature adds two new stream-type/allowlist-adjacent surfaces (the write-endpoint allowlist and the `HostConfig` body validator) to both files simultaneously, so the risk of a repeat is if anything higher, not lower.

**Mitigation specified for this feature**: add a single, version-controlled fixture file — proposed location `backend/internal/orthrus/testdata/muzzle_corpus.json` (checked into the `backend/` module, which already has full CI coverage; `agent/`'s test package reads the same file via a relative path `../../backend/internal/orthrus/testdata/muzzle_corpus.json`, or, if cross-module relative paths prove awkward given `agent/` is a separate Go module, the fixture is duplicated verbatim in both locations with a `go:generate`-style header comment in each pointing at the other as the source of truth — the exact mechanism is an implementation-phase decision, not fixed by this spec, but the *requirement* — one data-driven corpus asserting both filters agree — is fixed) — containing an array of `{method, path, body (optional), agent_write_enabled, want_allowed}` cases covering:

- Every existing read-only allowlist entry (regression coverage for the read path, migrated from the existing hand-written cases in both `_test.go` files where practical).
- Every new write-endpoint entry, once with `agent_write_enabled: true` (expect allow) and once with `agent_write_enabled: false` (expect the identical 403 outcome as before this feature existed — proving the default-off case is provably unchanged).
- A representative set of dangerous `POST /containers/create` bodies (one per excluded `HostConfig` key, plus the `NetworkMode: "host"` case, plus the `Mounts[].Type: "bind"` case, plus the `Mounts[].VolumeOptions.DriverConfig` bypass — a `Mounts` entry with `Type: "volume"` and `VolumeOptions: {"DriverConfig": {"Options": {"type":"none","device":"/etc","o":"bind"}}}`, the exact `local`-driver bind-mount-via-volume pattern from step 4 of Section 3.3.4) — all expected `want_allowed: false` even with `agent_write_enabled: true`, and this corpus case in particular must assert **both** `TestMuzzle_SharedCorpus` and `TestFilter_SharedCorpus` reject it, since it is the one case in this list that a naive re-implementation of "reject `Type: bind`" alone would silently let through.
- A representative safe `POST /containers/create` body — expected `want_allowed: true` only when `agent_write_enabled: true`.

Both `backend/internal/orthrus/muzzle_test.go` and `agent/muzzle/muzzle_test.go` gain one new test function each (`TestMuzzle_SharedCorpus` / `TestFilter_SharedCorpus`) that loads this fixture and asserts every case. This is the concrete artifact GH #1161 asks for ("a shared test corpus... that exercises both filters identically") — this feature is the first consumer of that recommendation, not a promise to build it later.

**CI enforcement (concrete, this feature's own scope — corrects an earlier draft's overclaim)**: a direct grep of every file in `.github/workflows/*.yml` (`grep -rln 'agent' .github/workflows/*.yml`, cross-checked against `grep -rn 'go test' .github/workflows/*.yml`) confirms **zero** existing workflow steps run `go test` (or anything else) inside the `agent/` Go module — every workflow that touches `agent/` (`orthrus-build.yml`, and `nightly-build.yml`'s `build-and-push-nightly-orthrus` job) goes straight from `actions/checkout` to `docker/build-push-action`, building the module's source only as an opaque `COPY . .` step inside `agent/Dockerfile`'s own multi-stage build — the module's Go test suite has genuinely never executed in CI. This matches GH #1161's own description ("agent/ has no CI-enforced quality gates") exactly. `TestFilter_SharedCorpus` (and every other existing `agent/muzzle` test) would today only ever run on a contributor's or agent's local machine.

This feature closes that specific gap — test *execution* only, not the broader lint/staticcheck/coverage tooling GH #1161 also asks for (that remains out of scope, Section 7). Verified locally that `cd agent && go test ./...` runs cleanly today (`ok agent/leash`, `ok agent/muzzle`, no test files in `agent`, `agent/cert`, `agent/protocol` — exit 0), and that `agent/go.mod`/`agent/go.sum` already exist as a normal, independently buildable Go module, so `actions/setup-go` can target it exactly the same way `quality-checks.yml` already targets `backend/go.mod` (see `quality-checks.yml:33–38` for the precedent: `actions/setup-go` with `go-version-file: backend/go.mod`, `cache-dependency-path: backend/go.sum`). There is no structural blocker — this is a small, mechanical CI change, specified concretely below.

**`.github/workflows/orthrus-build.yml` change** — insert two new steps into the existing `build-and-push` job, after the `Normalize image name` / `Compute branch tags` steps and **before** `Set up QEMU` (i.e., before any Docker build machinery starts, so a test failure aborts before the (currently unused-for-testing) `GO_VERSION: '1.26.5'` env var's only purpose becomes making this insertion trivial — that var is already declared in this file's `env:` block, line 32, and was previously dead/unused):

```yaml
      - name: Set up Go
        uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: agent/go.mod
          cache-dependency-path: agent/go.sum

      - name: Run Orthrus agent tests
        run: |
          set -euo pipefail
          cd agent
          go test ./...
```

This gates the `build-and-push` job — which runs on `pull_request` (paths: `agent/**`), `push` to `main`/`development`, tags, and `workflow_dispatch` — so both PR-time and direct-push-time builds of the agent image now require the agent's own test suite to pass first, closing the gap for the codepath that actually reaches production image tags.

**`.github/workflows/nightly-build.yml` change** — insert the identical two steps into the `build-and-push-nightly-orthrus` job, after `Checkout nightly branch` / `Set lowercase image name` and before `Set up QEMU`. Rationale for gating nightly too, not just relying on `orthrus-build.yml` having already gated the PR that got merged: `sync-development-to-nightly` (the job `build-and-push-nightly-orthrus` depends on) performs a `git reset --hard origin/nightly` followed by either a fast-forward merge or, on failure, `git reset --hard origin/development` plus a **force push** — a path that does not itself go through a reviewed PR against `nightly`, so gating only the PR-facing workflow leaves this one production image-publishing path untested. Duplicating the same two steps (rather than extracting a reusable workflow) matches this codebase's existing convention of near-identical step blocks repeated across `nightly-build.yml`'s and `orthrus-build.yml`'s otherwise-parallel jobs (e.g. the QEMU/Buildx/login steps are already duplicated verbatim between the two files today).

Both insertions are additive-only (no existing step is modified or removed) and use the same pinned `actions/setup-go` SHA already used elsewhere in this repo (`quality-checks.yml`), so no new third-party action needs separate supply-chain review.

#### 3.3.6 Rate limiting for write traffic

**Decision**: a new, purpose-built, session-scoped token-bucket limiter, reusing the `golang.org/x/time/rate` primitive already vendored via `cerberus/rate_limit.go` (not reusing `cerberus.RateLimitMiddleware` itself, since it is Gin-coupled and the External Proxy's `http.Server` is not part of the Gin router — confirmed Section 2.7). Scope: **write requests only** — the existing unrate-limited read traffic through the External Proxy is an existing, separate, out-of-scope condition (flagged in Section 7) that predates this feature and is not worsened by it.

```go
// session.go — AgentSession gains:
writeLimiter *rate.Limiter // nil unless writeEnabled; lazily constructed in NewAgentSession
```

Default: `rate.NewLimiter(rate.Every(2*time.Second), 5)` — steady-state 0.5 req/s (30/min), burst of 5. Rationale: a single compliant update cycle (pull → stop → remove → create → start) is exactly 5 write calls; the burst allows one full update to proceed immediately, while the steady-state rate bounds sustained container churn to well below anything a legitimate periodic update-checker (run hourly/daily) would ever need, while still comfortably tolerating a human manually triggering a couple of updates back-to-back. This is a fixed constant for v1 — no DB-backed per-agent tuning (see Section 7).

`Muzzle.ServeHTTP` consults this limiter (per Section 3.3.3's control flow) only for requests that would otherwise be allowed by the write allowlist: if `!writeLimiter.Allow()`, respond `429 Too Many Requests` and emit a `SecurityAudit` entry with `Action: "orthrus_write_rate_limited"` (Section 3.3.7) rather than silently dropping — an operator investigating "why did my update fail" needs this in the audit trail, not just a server log line.

#### 3.3.7 Audit logging

Every request that reaches the new write-allowlist branch in `Muzzle.ServeHTTP` (backend side only — the agent-side filter has no `SecurityService` reference and should not acquire one; it is a minimal standalone binary intentionally kept free of Charon's full service graph, consistent with `agent/`'s existing "separate Go module, minimal deps" design already noted in GH #1161) results in exactly one `SecurityAudit` entry, regardless of outcome:

| Scenario | `Action` | `EventCategory` | `ResourceUUID` | `Details` (JSON) |
|---|---|---|---|---|
| Write request forwarded | `"orthrus_write_allowed"` | `"orthrus_write"` | agent UUID | `{"method": "...", "path": "..."}` |
| Blocked: `HostConfig` validation failed | `"orthrus_write_blocked"` | `"orthrus_write"` | agent UUID | `{"method": "...", "path": "...", "reason": "disallowed HostConfig field: <field>"}` |
| Blocked: rate limit | `"orthrus_write_rate_limited"` | `"orthrus_write"` | agent UUID | `{"method": "...", "path": "..."}` |
| Flag toggled via `PATCH` | `"orthrus_write_enabled"` / `"orthrus_write_disabled"` | `"orthrus_write"` | agent UUID | `{"agent_name": "..."}` |

`Muzzle` requires a `LogAudit(*models.SecurityAudit) error`-shaped dependency (an interface, not a concrete `*services.SecurityService`, to keep the `orthrus` package's existing import graph — it currently imports `logger`, `util`, no `services` — from gaining a dependency on `services`, which today depends on `orthrus`: `services/orthrus_service.go` already imports `"github.com/Wikid82/charon/backend/internal/orthrus"`. An `orthrus → services` import would therefore create a cycle. **This means `Muzzle` cannot import `services.SecurityService` directly** — it must accept a narrow interface (`type AuditLogger interface { LogAudit(*models.SecurityAudit) error }`) satisfied by `*services.SecurityService` at the call site in `session.go`/`server.go`, where the concrete type is already available and no cycle exists. Note this interface still needs `models.SecurityAudit` — `orthrus` importing `models` is safe and already happens (`server.go` already imports `"github.com/Wikid82/charon/backend/internal/models"`), so only the `services` import is the concern.

`Actor` for all `orthrus_write_*` entries generated by `Muzzle` itself: `"orthrus-agent:<agent-name>"` (there is no authenticated Charon user in this request path — the caller is a third-party tool through the tunnel, not a logged-in operator — so `Actor` must identify the *agent*, not a user, to keep the audit trail meaningful). The `PATCH`-triggered `orthrus_write_enabled`/`orthrus_write_disabled` entries (Section 3.2.1), by contrast, are operator-initiated and should use whatever `Actor` convention the codebase already applies to authenticated admin actions (a research item for implementation, per 3.2.1's note).

### 3.4 Component Design — Agent (`agent/`)

`agent/muzzle/muzzle.go`: `Filter` becomes connection-scoped instead of process-scoped (a real, load-bearing structural change — currently `muzzle.New()` is called once in `Leash.New` and reused for the life of the agent process, spanning arbitrarily many reconnects; it must become per-connection so a mid-connection DB toggle can't retroactively change an already-negotiated session, matching the backend's per-`AgentSession` scoping exactly):

```go
// agent/muzzle/muzzle.go
func New(writeEnabled bool) *Filter { return &Filter{writeEnabled: writeEnabled} }
```

`agent/leash/leash.go`: `l.filter` is no longer set in `New()`; instead, `connect()` constructs a fresh `muzzle.New(writeEnabled)` per successful dial, after reading `X-Orthrus-Write-Enabled` off the handshake response (Section 3.3.1), and `handleDockerStream` closes over that connection-scoped filter instead of the struct-level `l.filter` field. This is a structural refactor of `Leash`/`Filter`'s lifetime coupling, not just an added parameter — flagged explicitly because it changes an invariant ("one `Filter` per agent process") that has held since the file was written, and any code elsewhere relying on that invariant (none found in this research pass, but the implementation phase must re-check) needs re-verification.

#### 3.4.1 `Filter.Allow` signature change and body re-buffering (concrete)

Current signature (verified verbatim, `agent/muzzle/muzzle.go:122`): `func (f *Filter) Allow(method, reqPath string) bool`. This must change to accept the request body, since the write-endpoint body validation (Section 3.3.4's `HostConfig` key allowlist, `Mounts[].Type` restriction, `Mounts[].VolumeOptions.DriverConfig` rejection, and `NetworkMode` value check) needs to inspect `POST /containers/create` bodies identically to the backend side:

```go
// agent/muzzle/muzzle.go
func (f *Filter) Allow(method, reqPath string, body []byte) bool
```

`body` is only consulted for the one body-validated case (`method == http.MethodPost` and `reqPath`, after the same version-prefix-strip + `path.Clean` normalization `Allow` already applies, matches `/containers/create`); every other allowlist branch (all existing read-only entries, plus the five non-body-validated write entries from 3.3.3) ignores the parameter entirely — passing `nil` for those cases is safe and requires no branching at any call site outside `Allow` itself.

**Body re-buffering in `ServeProxy`** (`agent/muzzle/muzzle.go:164–196`, verified current flow: `http.ReadRequest(bufr)` → `Allow(method, path)` → `net.Dial` → `req.Write(conn)` → `io.Copy` response back). Reading `req.Body` to obtain bytes for `Allow` consumes the underlying `io.ReadCloser` exactly once — `req.Write(conn)` immediately afterward would forward an empty/already-drained body unless the read bytes are wrapped back into a fresh reader first. Concrete change to `ServeProxy`:

```go
// New package-level constant, mirroring the backend's 64 KiB cap from
// Section 3.3.4 exactly — the two values must stay numerically identical;
// the shared corpus (3.3.5) should include a boundary-size case to catch drift.
const maxContainerCreateBodyBytes = 64 * 1024

func (f *Filter) ServeProxy(dst string, r io.Reader, w io.Writer) error {
	bufr := bufio.NewReader(r)

	req, err := http.ReadRequest(bufr)
	if err != nil {
		return fmt.Errorf("muzzle: read request: %w", err)
	}

	var bodyBytes []byte
	if req.Body != nil {
		limited := io.LimitReader(req.Body, maxContainerCreateBodyBytes+1)
		bodyBytes, err = io.ReadAll(limited)
		_ = req.Body.Close()
		if err != nil {
			return fmt.Errorf("muzzle: read body: %w", err)
		}
		if len(bodyBytes) > maxContainerCreateBodyBytes {
			_, _ = io.WriteString(w, forbiddenResponse)
			return fmt.Errorf("muzzle: blocked %s %s: body too large", req.Method, req.URL.Path)
		}
		// Re-buffer: req.Write below must still forward the original body intact.
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.ContentLength = int64(len(bodyBytes))
	}

	if !f.Allow(req.Method, req.URL.Path, bodyBytes) {
		// Fail closed: write 403 and abort the stream.
		_, _ = io.WriteString(w, forbiddenResponse)
		return fmt.Errorf("muzzle: blocked %s %s", req.Method, req.URL.Path)
	}

	conn, err := net.Dial("unix", dst)
	if err != nil {
		return fmt.Errorf("muzzle: dial docker socket: %w", err)
	}
	defer conn.Close()

	req.Close = true

	if err := req.Write(conn); err != nil {
		return fmt.Errorf("muzzle: forward request to docker: %w", err)
	}

	_, err = io.Copy(w, conn)
	return err
}
```

New import required: `bytes` (for `bytes.NewReader`); `io.LimitReader`, `io.ReadAll`, and `io.NopCloser` are already reachable via the existing `io` import.

This read-and-rebuffer step runs for **every** request through `ServeProxy`, not only `/containers/create` — GET requests in practice present `req.Body` as either `nil` or an already-empty, immediately-`io.EOF` reader (standard `net/http` request-parsing behavior for bodyless requests), so the added `io.ReadAll` is a cheap no-op for the overwhelming majority of traffic. This keeps `ServeProxy`'s control flow linear (one unconditional read-and-rebuffer step) instead of branching on method/path *before* deciding whether to buffer, which would duplicate the same "is this `/containers/create`" check that already has to live inside `Allow`.

`Filter.Allow`'s internal write-allowlist and `POST /containers/create` body-validation logic mirrors Sections 3.3.3/3.3.4 exactly (same `HostConfig` key allowlist, `Mounts[].Type` restriction, `Mounts[].VolumeOptions.DriverConfig` rejection, `NetworkMode` value check), duplicated per the existing two-independent-copies architecture and covered by the shared corpus in 3.3.5. The agent has no rate limiter and no audit logging (see 3.3.6/3.3.7 rationale) — its role is strictly the second independent allow/deny check, identical in policy to the backend's, with no side effects beyond serving or refusing the proxy.

`agent/leash/leash.go`'s call site (`leash.go:178`, `l.filter.ServeProxy(l.dockerSock, stream, stream)`) requires **no signature change** — the body re-buffering above is fully internal to `ServeProxy`; `Leash` continues to pass the same three arguments (`dst string, r io.Reader, w io.Writer`) it does today. Confirmed via direct read of `leash.go` that no other call site or field references `Filter.Allow` or relies on its two-argument form.

### 3.5 Component Design — Frontend

#### 3.5.1 New dialog vs. extending `AgentExternalProxyDialog`

**Decision: new, separate dialog** (`AgentWriteModeDialog.tsx`), opened from its own icon button in `OrthrusAgentManager.tsx`, not a new section bolted onto `AgentExternalProxyDialog`.

Justification: `AgentExternalProxyDialog` governs a *transport-layer* setting (is there a TCP port bound at all, and which one) that is a prerequisite for write mode to have any effect (write mode is meaningless without the External Proxy — or a future direct-tunnel-consumer — being reachable at all) but is conceptually a different axis of configuration. The design brief's own required UX (typed-name-to-confirm, a *distinct* warning banner, a fixed-list display of permitted operations) is substantial enough that bolting it onto the existing 250-line dialog would roughly double its size and mix two independent concerns (an unauthenticated-by-default network exposure setting, and a permissions-escalation setting) into one component's state machine — `configuredDiffersFromActive`-style reconnect-notice logic would need to independently track two different "configured vs. active" pairs (port, write-mode) with only loosely related triggers, which reads more clearly as two components each tracking one pair. A new adjacent dialog keeps each component's local state minimal and testable, consistent with the existing pattern where `OrthrusAgentManager` already orchestrates three separate dialogs (delete-confirm, `AgentProviderAssignDialog`, `AgentExternalProxyDialog`) rather than one mega-dialog.

#### 3.5.2 `AgentWriteModeDialog.tsx` — new component

Props: `{ agent: OrthrusAgent; open: boolean; onClose: () => void }` (mirrors `AgentExternalProxyDialogProps` exactly).

State machine, off → on (the only transition requiring the typed-confirmation gate; on → off is a single confirm click, matching how disabling is strictly safety-increasing and needs no extra friction):

1. Toggle switch, default reflecting `agent.write_enabled`, labelled per the fixed operation list (see below).
2. When the operator flips it **on** from off: reveal a typed-confirmation input (new UX for this codebase, modeled on the *shell* of the existing delete-confirmation `Dialog` per Section 2.5's finding — title, description naming the agent, Cancel + destructive-styled confirm button — but adding the `<input>` requirement since no exact precedent exists): placeholder/label `t('hecate.writeMode.confirmPrompt', { name: agent.name })` ("Type **{name}** to confirm"), Save/Enable button `disabled` until the input's trimmed value strictly equals `agent.name`.
3. When flipping **off**: no typed confirmation — a single `patch({ write_enabled: false })` on Save, consistent with "disabling is always low-friction."
4. A **distinct** warning banner (separate `role="note"` block, not reusing `hecate.externalProxy.securityWarning`'s copy or DOM node), proposed text: *"Enabling write mode lets tools connected through this agent's External Docker Proxy pull images, and start, stop, restart, remove, and recreate containers on this machine. Every allowed action is logged. This does not grant shell access, volume/network changes, or any operation not explicitly listed below."* — visually and textually distinguishable from the External Proxy dialog's network-exposure warning, per Objective 7 / design brief.
5. Fixed, non-editable list of the permitted operations (Section 3.3.3's allowlist, rendered as user-facing prose, not raw Docker paths): "Pull a new image", "Start a container", "Stop a container", "Restart a container", "Remove a container", "Create (recreate) a container" — rendered only when the toggle is (or is about to be) on.
6. Reconnect notice, mirroring `configuredDiffersFromActive` exactly but comparing `agent.write_enabled` (configured) against `proxyStatus.active_write_enabled` (live, from the extended `GetProxyStatus` response, Section 3.2.2) rather than port numbers — reusing `hecate.externalProxy.reconnectNotice`'s *pattern* but a new translation key (`hecate.writeMode.reconnectNotice`) since the surrounding sentence needs to reference write mode, not the port.
7. Link/button: "View write-access audit log" → navigates to `/audit-logs?resource_uuid=<agent.uuid>&event_category=orthrus_write` (Section 3.5.3 makes this actually pre-filter on arrival).

`OrthrusAgentManager.tsx` gains: a new icon button (proposed: `ShieldCheck` or `KeyRound` from `lucide-react`, distinct from the existing `Settings` gear used for the proxy dialog) in the actions cell, a `writeModeAgent` state slot mirroring `proxyConfigAgent`'s, and a `WRITE` badge (mirroring the existing `PROXY` badge at line 141–145) rendered when `agent.write_enabled` is true.

#### 3.5.3 `AuditLogs.tsx` deep-link support (new, generic fix — Section 2.6 gap)

`AuditLogs.tsx`'s `filters` `useState<AuditLogFilters>({})` initializer becomes a lazy initializer reading `URLSearchParams(window.location.search)` for any of the existing `AuditLogFilters` keys present in the query string (`event_category`, `resource_uuid`, `actor`, `action`, plus date filters if present) — a generically useful fix, not Orthrus-specific, using React Router's `useSearchParams` if the app already uses React Router (needs confirming which router the app uses during implementation; if a router with search-param hooks is already in place elsewhere in the codebase, reuse it for consistency rather than raw `window.location.search` parsing).

#### 3.5.4 Type changes

```ts
// frontend/src/api/orthrus.ts
export interface OrthrusAgent {
  // ...existing fields...
  write_enabled: boolean; // NEW
}
export interface PatchAgentRequest {
  // ...existing fields...
  write_enabled?: boolean; // NEW
}
export interface ExternalProxyStatus {
  // ...existing fields...
  configured_write_enabled: boolean; // NEW
  active_write_enabled: boolean;     // NEW
}

// frontend/src/api/auditLogs.ts
export type EventCategory =
  | 'dns_provider' | 'certificate' | 'proxy_host' | 'user' | 'system'
  | 'orthrus_write'; // NEW
export type AuditAction =
  | /* ...existing... */
  | 'orthrus_write_allowed' | 'orthrus_write_blocked'
  | 'orthrus_write_rate_limited' | 'orthrus_write_enabled' | 'orthrus_write_disabled'; // NEW
```

### 3.6 Data Flow (end-to-end, happy path)

1. Operator opens `AgentWriteModeDialog`, types the agent's name, clicks Enable → `PATCH /api/v1/orthrus/agents/:uuid { write_enabled: true }` → `OrthrusService.Patch` updates the DB row and emits an `orthrus_write_enabled` audit entry → response reflects `write_enabled: true`.
2. Dialog shows the reconnect notice (`configured_write_enabled: true`, `active_write_enabled: false` if the agent was already connected).
3. Agent reconnects (automatically, per its existing exponential-backoff loop in `leash.go:Run`, or the operator restarts it) → `HandleWebSocket` re-reads `agent.WriteEnabled` from the DB, sets `X-Orthrus-Write-Enabled: true` on the upgrade response, constructs `NewAgentSession(..., writeEnabled: true, ...)`.
4. Agent's `connect()` reads the header, builds `muzzle.New(true)`, uses it for all Docker streams on this connection.
5. Dockhand issues `GET /v1.44/containers/<id>/json` through the External Proxy port → passes both filters unconditionally (existing read allowlist, unaffected) → tool determines a new image digest is available (existing hotfix capability).
6. Dockhand issues `POST /images/create?fromImage=...` → backend `Muzzle` (writeEnabled=true, rate limiter has burst available) → allowed, forwarded → agent `Filter` (writeEnabled=true) → allowed, forwarded to real Docker socket → audit entry `orthrus_write_allowed`.
7. Dockhand issues `POST /containers/<id>/stop`, `DELETE /containers/<id>`, `POST /containers/create` (body validated against the `HostConfig` allowlist — passes, since it's a same-config recreate with no dangerous fields), `POST /containers/<new-id>/start` — each independently allowed/audited as in step 6.
8. If any request in the sequence exceeds the rate limiter's burst (e.g., a runaway loop retrying rapidly), that request gets `429` and an `orthrus_write_rate_limited` audit entry instead of being forwarded.
9. If Dockhand (or an attacker with tunnel access) sends `POST /containers/create` with `HostConfig.Privileged: true`, the backend `Muzzle` rejects it with `403` and an `orthrus_write_blocked` audit entry **before it ever reaches the agent** — the agent-side filter would independently reject the same body if it were ever reached, per 3.3.4/3.3.5, but defense-in-depth means the backend is expected to be the first line here.
10. Operator reviews `/audit-logs?resource_uuid=<uuid>&event_category=orthrus_write` (now correctly pre-filtered per 3.5.3) and sees the full sequence.

### 3.7 Error Handling Summary

| Condition | HTTP status | Response | Audit entry |
|---|---|---|---|
| Write request, `writeEnabled=false` | `403` | `"Forbidden"` (existing generic message, unchanged code path) | none (identical to today's behavior — a disabled agent produces zero new audit volume) |
| Write request, allowed by allowlist + body validation | pass-through | whatever the real Docker daemon returns | `orthrus_write_allowed` |
| `POST /containers/create`, disallowed `HostConfig` field | `403` | `"Forbidden: disallowed HostConfig field"` | `orthrus_write_blocked` |
| `POST /containers/create`, body exceeds size cap | `403` | `"Forbidden: request body too large"` | `orthrus_write_blocked` |
| Write request, rate limit exceeded | `429` | `"Too Many Requests"` | `orthrus_write_rate_limited` |
| `PATCH .../agents/:uuid` with `write_enabled` set, agent not found | `404` | existing `"agent not found"` (unchanged) | none |
| Malformed JSON body on `/containers/create` while write-enabled | `403` | `"Forbidden: malformed request body"` | `orthrus_write_blocked` |

---

## 4. Explicit Risk Treatment (per design-brief requirement — not hand-waved)

### 4.1 `HostConfig` body-inspection risk

**Resolved** per Section 3.3.4: both muzzle layers become body-inspecting for exactly one endpoint (`POST /containers/create`), using a hybrid allowlist-of-safe-`HostConfig`-keys + one value-level check (`NetworkMode`) + a hard size cap. Feasibility confirmed directly from the current code: the agent-side filter already fully parses the HTTP request before forwarding (Section 2.3), and the backend-side filter, being a genuine `http.Handler`, can read-and-rebuffer `r.Body` with a standard `io.NopCloser(bytes.NewReader(...))` pattern before calling `m.next.ServeHTTP`.

### 4.2 Six-endpoint allowlist sufficiency for a real update flow

**Resolved, with one documented functional limitation.** The full sequence (Section 3.6, steps 5–7) — inspect (already-allowed read) → pull → stop → remove → create → start — is fully coverable by the six endpoints in the design brief. **Gap found**: `POST /containers/create`'s `NetworkingConfig` body field accepts only **one** network endpoint per Docker Engine API semantics; a container attached to **multiple** Docker networks at recreate time requires one additional `POST /networks/{id}/connect` call per extra network, which is **not** in the six-endpoint list and is deliberately **not proposed for addition in v1** — it is a network-mutation endpoint, and the design brief's boundary ("no volume/network create-or-delete") reads as intentionally conservative about network-adjacent write access even though `connect` is not technically a create-or-delete operation. **v1 accepts this as a documented limitation**: multi-network containers can be updated via this feature only if the operator or tool re-attaches secondary networks by some other means (e.g., manually, or Charon's own existing Docker management UI, which is unaffected by this feature and already has full write access to Charon-managed Docker hosts). This is called out in Section 7 (Out-of-Scope) and should be stated plainly in the user-facing docs rewrite (Phase 5) so operators aren't surprised.

### 4.3 Rate-limiting / abuse potential

**Resolved** per Section 3.3.6: existing Gin-based rate limiting (`cerberus.RateLimitMiddleware`) does not and structurally cannot cover the External Proxy's raw `http.Server` (confirmed architectural fact, Section 2.7, true today independent of this feature). A new, minimal, session-scoped `rate.Limiter` (reusing the same underlying package Cerberus already depends on) is added specifically for the write path, with a concrete default (0.5 req/s, burst 5) and explicit audit-trail visibility when triggered.

### 4.4 Drift-class repeat risk (the two-independent-muzzles problem)

**Resolved** per Section 3.3.5: this feature is specified as the first concrete implementation of the shared-test-corpus mitigation GH #1161 already recommends, rather than merely acknowledging the risk and repeating the historical pattern. GH #1160 and #1161 are confirmed open (`gh issue view 1160`/`1161`, both returned real content matching the design brief's description) and are cited by number in the corpus fixture's design (Section 3.3.5) as the origin of this requirement.

---

## 5. Implementation Plan

### Phase 1 — Playwright E2E Specs (write-only, `test.fixme`, no implementation yet)

New spec file, path convention to be confirmed against the existing `e2e/` layout during implementation (not verified in this research pass — flagged for `playwright-dev`):

- Agent write-mode toggle is off by default for a newly provisioned agent.
- Enabling requires typing the agent's name; Save is disabled until the typed value matches.
- Enabling succeeds, reconnect notice appears while the (test-double) agent session hasn't reconnected.
- Disabling requires no typed confirmation.
- Write-mode badge appears in the agent table when enabled.
- Navigating to the audit-log link from the write-mode dialog lands on a pre-filtered `/audit-logs` view.

All `test.fixme` until Phase 3/4 land.

### Phase 2 — Backend Foundation (no behavior change)

- `models.OrthrusAgent.WriteEnabled` field + doc comment (Section 3.1).
- `AgentSession.writeEnabled` field, `NewAgentSession` signature change, `ExternalProxyStatus.WriteEnabled` field (Section 3.3.1/3.3.2) — plumbed through but not yet reachable (no caller sets it to `true` yet; `HandleWebSocket` still always constructs with the DB value, which defaults `false`, so this phase is a pure no-op for existing agents).
- `Muzzle.writeEnabled`/`writeLimiter` fields and constructor signature change; all existing call sites updated — **must not change any existing test's expected outcome**.
- `agent/muzzle.Filter` gains the same fields; `agent/leash` refactor to per-connection `Filter` construction (Section 3.4) — same "always constructed with `false` today" constraint.
- Validation gate: `go build ./... && go test ./...` green in both `backend/` and `agent/` modules, zero behavior change in existing test suites.

### Phase 3 — Backend Write-Path Implementation

- Write allowlist entries + `NetworkMode`/`HostConfig` body validator, identically in both muzzle files (Section 3.3.3/3.3.4).
- Shared test corpus fixture + both `TestMuzzle_SharedCorpus`/`TestFilter_SharedCorpus` (Section 3.3.5).
- Rate limiter wiring (Section 3.3.6).
- Audit logging: `AuditLogger` interface, `Muzzle` constructor gains it, wired at the `session.go`/`server.go` call site (Section 3.3.7).
- `X-Orthrus-Write-Enabled` handshake header, both sides (Section 3.3.1).
- `PATCH`/`GET proxy-status` handler + service changes (Section 3.2.1/3.2.2), including the new `OrthrusService` → `SecurityService` dependency wiring in `routes.go`.
- Validation gate: full backend unit test suite green, `./scripts/scan-gorm-security.sh --check` zero CRITICAL/HIGH (model change trigger per `CLAUDE.md` 1.5), `make lint-fast` clean, coverage ≥85%.

### Phase 4 — Frontend Implementation

- Type changes (Section 3.5.4).
- `AgentWriteModeDialog.tsx` (Section 3.5.2) + wiring into `OrthrusAgentManager.tsx`.
- `AuditLogs.tsx` deep-link support (Section 3.5.3).
- New i18n keys across all five locale files (`en`, `fr`, `de`, `es`, `zh` — matching the existing `hecate.externalProxy.*` convention's locale coverage).
- Validation gate: `npm run type-check`, `npm run build`, frontend unit tests green, coverage ≥85%.

### Phase 5 — Hardening, E2E Enable, Docs

- Un-`fixme` the Phase 1 Playwright specs; full `npx playwright test --project=firefox` run.
- `docs/features/orthrus.md` rewrite: correct all three absolute claims quoted in Section 2.8, describe the opt-in flow, the fixed operation list, the typed-confirmation requirement, and the multi-network limitation from Section 4.2.
- Full `CLAUDE.md` Definition-of-Done pass (Sections 1–10 of that document) before merge.

---

## 6. Commit Slicing Strategy

**Decision: single PR, one feature, ordered commits** — per `CLAUDE.md`'s "One Feature = One PR" / "Slice Commits, Not PRs" rule. No commit here is independently mergeable or independently useful; they exist to keep review tractable, not to be split across PRs.

| # | Scope | Files (representative, not exhaustive) | Depends on | Validation gate |
|---|---|---|---|---|
| **1** | E2E specs, `test.fixme` | new file under `e2e/` | — | Playwright collects the file without error; all cases skipped, none fail |
| **2** | Backend foundation — model field, session/muzzle plumbing, zero behavior change | `models/orthrus_agent.go`, `orthrus/session.go`, `orthrus/muzzle.go`, `agent/muzzle/muzzle.go`, `agent/leash/leash.go` | 1 | `go build ./...` (both modules), `go test ./...` (both modules) green, no existing test's expected output changes |
| **3** | Backend write-path — allowlist, body validator, shared corpus, rate limiter, audit logging, handshake header | same files as #2 (deepened) + `testdata/muzzle_corpus.json`, `security_service.go` wiring | 2 | Backend unit tests green, `./scripts/scan-gorm-security.sh --check` clean, `make lint-fast` clean, coverage ≥85% |
| **4** | Backend API — `PATCH`/`GET proxy-status` extensions, `OrthrusService.Patch` signature + audit call, `routes.go` DI wiring | `orthrus_handler.go`, `orthrus_service.go`, `routes.go` | 3 | Handler/service unit tests green, `go build ./...` |
| **5** | Frontend — types, `AgentWriteModeDialog`, `OrthrusAgentManager` wiring, `AuditLogs` deep-link fix, i18n | `api/orthrus.ts`, `api/auditLogs.ts`, `components/hecate/AgentWriteModeDialog.tsx`, `components/hecate/OrthrusAgentManager.tsx`, `pages/AuditLogs.tsx`, `locales/*/translation.json` | 4 | `npm run type-check`, `npm run build`, Vitest suite green, coverage ≥85% |
| **6** | Hardening + E2E enable + docs | un-skip commit-1 spec file, `docs/features/orthrus.md` | 1–5 | Full `npx playwright test --project=firefox`, full `CLAUDE.md` DoD |

**Rollback / contingency for the PR as a whole**: every commit up to and including #4 leaves default (`write_enabled=false`) behavior byte-for-byte identical to pre-feature behavior — the feature is inert until an operator explicitly flips the DB flag via the (not-yet-existing-until-commit-5) UI, or manually via the API. If a critical issue is found post-merge, the safest rollback is **not** a revert of the whole PR but an emergency DB-level or config-level disable (e.g., a temporary backend guard forcing `writeEnabled` to always evaluate `false` regardless of the DB value) — since the DB migration itself (additive column, default `false`) is safe to leave in place even if the feature is disabled. A full revert remains available as a fallback if the guard approach is judged insufficient.

---

## 7. Explicit Out-of-Scope

- Per-agent customization of *which* write operations are allowed beyond the fixed six-endpoint list — v1 is on/off only.
- `exec`, image `delete`, `build`, `prune`, `auth`, `commit`, any Swarm/service endpoint, under any configuration, ever, regardless of `WriteEnabled`.
- `POST /networks/{id}/connect` / multi-network container recreate support (Section 4.2) — documented v1 limitation, not silently ignored, candidate for a future v2 if real-world demand emerges.
- DB-backed or UI-configurable tuning of the write-path rate limiter's rate/burst constants — v1 ships fixed constants (Section 3.3.6).
- Rate limiting of *read* traffic through the External Docker Proxy — this predates the feature (Section 2.7) and is not addressed here; flagged as a separate potential follow-up, not bundled into this PR to keep scope tight.
- Unifying the two muzzle allowlists into a single shared-code implementation (GH #1161's broader ask) — this spec implements the *test-corpus* mitigation (Section 3.3.5) as the concrete, scoped contribution to that issue, but does not attempt the larger structural unification (e.g., extracting a shared Go module both `backend/` and `agent/` import), which GH #1161 itself notes may require restructuring given `agent/`'s minimal-standalone-binary constraint.
- **In scope for this feature**: CI *test execution* for the `agent/` module — Section 3.3.5 adds a `go test ./...` step to both `.github/workflows/orthrus-build.yml` and `.github/workflows/nightly-build.yml` (the two workflows that build/push the agent image), gating before the Docker build step. This is new CI infrastructure, added specifically because this feature's own safety argument (Section 3.3.5's shared corpus, Objective 3's defense-in-depth) depends on the `agent/` half of that corpus actually running automatically, not just locally.
- **Out of scope for this feature**: CI-enforced *lint/staticcheck/coverage* gates for the `agent/` module (GH #1161's second, broader ask) — no `golangci-lint`, no coverage threshold, no staticcheck step is added for `agent/` by this PR. Only test *execution* is added; test *quality* tooling remains a separate, unaddressed gap tracked by GH #1161.
- **Deferred, not blocking spec approval**: Section 3.3.5 specifies the concrete CI changes (`orthrus-build.yml`, `nightly-build.yml`) needed to actually run the `agent/` module's test suite — the shared anti-drift corpus, including the `Mounts[].VolumeOptions.DriverConfig` bypass case — but those changes are not yet implemented as of spec sign-off. This is a deliberate, tracked deferral (GH #1161), not an unresolved gap glossed over: it does not block this spec's approval, but it must land within this same PR/branch before final merge — see the binding note at Section 9, Acceptance Criterion #5.
- Any change to the unrelated, universal, unconditional read-only allowlist — untouched by this feature.
- Support for write mode through any transport other than the existing External Docker Proxy (`StartExternalProxy`) — e.g., no new direct-write API surface is added to Charon's own UI/API for agent-hosted containers; this feature only ungates the *tunnel* for third-party tools like Dockhand.

---

## 8. Security Review Requirement

This feature adds a **write capability to a previously strictly-read-only trust boundary** that spans an untrusted third-party tool (anything that can reach the External Proxy port on the operator's network) through Charon to a remote Docker daemon with root-equivalent host capability. Per this repo's own established practice (the same-day hotfix to this exact subsystem required `supervisor` review before implementation because it touched "the Docker API allowlist that gates what a tunnelled remote Docker socket can expose" — precedent cited directly from that plan's git history), this spec requires **mandatory `supervisor` review before any implementation begins**, with explicit sign-off on at minimum:

1. The `HostConfig` allowlist in Section 3.3.4 — is the enumerated safe-key list actually exhaustive/correct against the real Docker Engine API `ContainerCreateConfig`/`HostConfig` schema (this spec's list was derived from the well-known dangerous-field set, not from a field-by-field audit of Docker's OpenAPI spec — the supervisor or implementation phase should cross-check against the actual schema for the Docker API version this codebase targets).
2. The `NetworkMode` value-level check (`"host"` and `"container:*"` denylist) — confirm no other `NetworkMode` value carries equivalent risk.
3. The decision to reject rather than strip malformed/dangerous bodies (Section 3.3.4) — confirm this doesn't create a usability trap that pushes operators toward workarounds that are less safe (e.g., disabling the validator, which this spec does not provide any mechanism to do — worth an explicit "there is no bypass" confirmation).
4. The rate-limit constants (Section 3.3.6) — confirm 0.5 req/s / burst 5 is defensible, not just plausible.
5. The audit-log `Actor` convention (`"orthrus-agent:<name>"`) — confirm this doesn't collide with or get confused for a real username elsewhere in the audit log's `Actor` column, since that column is otherwise presumably populated with authenticated-user identifiers.
6. Whether the `services.SecurityService` dependency wiring proposed for `OrthrusService` (Section 3.2.1) introduces any import-cycle or initialization-order issue not caught by this research pass.

`qa-security` involvement (per the standard agent roster) is expected during Phase 3/4 implementation for the mandatory CodeQL/Trivy/GORM scans already required by `CLAUDE.md`'s Definition of Done — called out here specifically because Section 1.5 of that DoD (`GORM Security Scan`) is unconditionally triggered by this feature's `backend/internal/models/**` change.

---

## 9. Acceptance Criteria

Feature is considered done when **all** of the following hold, without exception:

1. A newly provisioned `OrthrusAgent` has `write_enabled: false` by default; a pre-existing agent row (present before this feature's migration runs) also reads `write_enabled: false` after migration, with no manual intervention.
2. With `write_enabled: false` (the default), 100% of existing Muzzle/Filter test behavior is byte-for-byte unchanged — every currently-passing test in `muzzle_test.go` and `agent/muzzle/muzzle_test.go` continues to pass with no modification to its expected outcome.
3. With `write_enabled: true` and an agent that has reconnected since the flag was set, all six allowlisted write operations succeed end-to-end against a real (or realistically mocked) Docker daemon, verified by at least one integration-level test exercising the full pull→stop→remove→create→start sequence.
4. A `POST /containers/create` body containing any of `Privileged: true`, non-empty `CapAdd`, non-empty `Binds`, `NetworkMode: "host"`, a `bind`-type `Mounts` entry, a `Mounts` entry with `VolumeOptions.DriverConfig` present (e.g. `{"type":"none","device":"/etc","o":"bind"}` inside `DriverConfig.Options` — the documented `local`-volume-driver bind-mount-via-volume bypass, Section 3.3.4 step 4), non-empty `Devices`, or non-empty `Sysctls` is rejected with `403` by **both** the backend and agent muzzle filters independently, even when `write_enabled: true`, each verified by its own test (not just the shared corpus, though the corpus should also cover it).
5. The shared test corpus (Section 3.3.5) exists, is loaded by both `backend/` and `agent/` test suites, and both suites agree on every case in it — enforced by CI via the new `go test ./...` step added to `.github/workflows/orthrus-build.yml`'s `build-and-push` job and `.github/workflows/nightly-build.yml`'s `build-and-push-nightly-orthrus` job (Section 3.3.5), both of which gate before the Docker image build step, not just local runs. (`backend/`'s half of the corpus was already covered by existing backend CI, per `quality-checks.yml`; the `agent/` half was not, until this feature's CI change.)

   > **Deferral note (user-approved, 2026-07-20):** CI enforcement of the `agent/` module's test suite — the shared anti-drift corpus described above, including the `Mounts[].VolumeOptions.DriverConfig` bypass test case — is accepted as a deliberate, tracked deferral, **not** resolved at spec-sign-off time. It is tracked under **GH #1161** ("unify the two hand-maintained allowlists / give `agent/` CI coverage"). This deferral does not block approval of this spec, but the CI workflow changes specified above (the `orthrus-build.yml` and `nightly-build.yml` steps) **must land within this same PR/branch, before final merge** — consistent with this repo's One-Feature-One-PR convention (`CLAUDE.md`, "Commit Slicing & PR Strategy") — not split into a separate PR, and not left as an indefinite someday-maybe. Acceptance Criterion #5 itself remains unmodified and must still be fully satisfied before merge; this note documents the timing/status of that satisfaction, not a reduction of the requirement.
6. Exceeding the write-path rate limiter produces `429` and an `orthrus_write_rate_limited` audit entry; the request is never forwarded to the agent.
7. Every allowed and every blocked write request produces exactly one `SecurityAudit` row with `event_category: "orthrus_write"`, queryable via `GET /api/v1/audit-logs?resource_uuid=<agent_uuid>&event_category=orthrus_write`.
8. The frontend `AgentWriteModeDialog` cannot be used to enable write mode without the operator typing the agent's exact current name; disabling requires no such confirmation.
9. `docs/features/orthrus.md` no longer contains the three absolute "cannot be changed" / "no way to turn it off" claims quoted in Section 2.8 in their current unqualified form; the rewritten text accurately describes the default (read-only, unconditional) and the opt-in (per-agent, audited, six-operation) states, and documents the multi-network limitation from Section 4.2.
10. Full `CLAUDE.md` Definition of Done (Sections 1–10) passes with zero errors: Playwright E2E, GORM security scan, patch coverage preflight, CodeQL/Trivy scans, lefthook, staticcheck, ≥85% coverage (backend and frontend), type-check, both builds, and cleanup.
11. `supervisor` sign-off obtained per Section 8 before implementation begins, and again before merge.

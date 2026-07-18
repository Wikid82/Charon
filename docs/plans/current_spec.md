# Hotfix: Orthrus External Docker Proxy — Allowlist Gap, Missing Docs, Hardcoded Hostname

**Type**: Hotfix (three verified, concrete gaps — NOT a redesign)
**Branch**: `development` (current working branch; no worktree, no new branch, per `CLAUDE.md`)
**Target commit count**: 5 (ordered commits within a single PR; see Commit Slicing Strategy)
**Target merge window**: Monday 2026-07-20
**Owners (implementation)**: `backend-dev` (Commits 1–3), `docs-writer` (Commit 4)
**Review**: `supervisor` agent — **REQUIRED before implementation begins**, because Commit 1 touches a security boundary (the Docker API allowlist that gates what a tunnelled remote Docker socket can expose). See Section 6.

---

## 1. Introduction

### 1.1 Background

Orthrus lets a remote Docker host tunnel through Charon to a third-party tool, acting as a secure drop-in replacement for a docker-socket-proxy. An `OrthrusAgent` (`backend/internal/models/orthrus_agent.go`) can have an `ExternalProxyPort` configured; when set, `AgentSession.StartExternalProxy` (`backend/internal/orthrus/session.go:254`) binds `0.0.0.0:<port>` and reverse-proxies through the yamux tunnel to the agent's real Docker socket. Every request passing through that port is filtered by `Muzzle` (`backend/internal/orthrus/muzzle.go`), a GET-only allowlist that is the sole security boundary between "read-only Docker API" and "full Docker socket access with root-equivalent capability on the remote host."

Three independent, already-verified gaps were found while investigating this subsystem. None require redesigning Orthrus; each is a small, targeted fix.

### 1.2 Objectives

1. **Gap 1 (security-relevant, backend)** — Expand the Muzzle allowlist with two additional GET-only, read-only dynamic patterns (`/images/*/json`, `/distribution/*/json`) so that update-checker tools (Dockhand, Diun, Watchtower-style digest comparison) can function through the external proxy, without weakening the read-only guarantee in any way.
2. **Gap 2 (docs)** — Document the External Docker Proxy feature, which currently has zero coverage in `docs/features/orthrus.md` or `docs/guides/remote-docker-setup.md`, despite already being shipped and user-facing (gear icon in `OrthrusAgentManager.tsx` → `AgentExternalProxyDialog.tsx`).
3. **Gap 3 (correctness, backend)** — Replace the hardcoded `"charon"` hostname in the external proxy's `connection_string` with dynamic resolution, using the same `X-Charon-URL` header / `c.Request.Host` fallback pattern already proven in `OrthrusHandler.GetInstallSnippets`.

### 1.3 Non-goals (see Section 5 for full out-of-scope list)

This is explicitly **not**: a rearchitecture of Orthrus or Muzzle's matching engine, a new UI, port-collision-detection polish, or multi-hostname configuration support.

---

## 2. Research Findings (confirmed by reading current source — line numbers verified 2026-07-18)

### 2.1 Gap 1 — `backend/internal/orthrus/muzzle.go`

Current exact allowlist contents:

```go
// lines 27-37
var allowedDockerPaths = map[string]struct{}{
	"/_ping":           {},
	"/containers/json": {},
	"/images/json":     {},
	"/info":            {},
	"/version":         {},
	"/events":          {},
	"/volumes":         {},
	"/networks":        {},
	"/system/df":       {},
}

// lines 42-49
var allowedDockerPatterns = []string{
	"/containers/*/json",
	"/containers/*/logs",
	"/containers/*/stats",
	"/containers/*/top",
	"/volumes/*",
	"/networks/*",
}
```

Matching is done in `Muzzle.ServeHTTP` (lines 65–103): non-GET is rejected at line 79–84 *before* any path check (method check happens first, unconditionally — this matters for Section 3.1's test plan, see below). GET requests are checked against the exact-match `allowedDockerPaths` map first (line 86), then against `allowedDockerPatterns` via `path.Match` (lines 93–98). The version-prefix stripping (`versionPrefixRe`, line 23) and `path.Clean` traversal-hardening (line 71) run before either check, so new patterns automatically inherit both protections — no changes needed there.

`docs/features/orthrus.md:104` makes an absolute, user-facing promise: *"This restriction is enforced at every single request — there is no way to turn it off."* This guarantee must hold exactly after the change — new entries must be GET-only, read-only, no exceptions.

**Known limitation to document, not fix, in this hotfix**: Go's `path.Match` `*` wildcard does **not** cross `/` boundaries (per `path.Match` godoc: "matches any sequence of non-Separator characters"). Docker's real `/images/{name}/json` and `/distribution/{name}/json` endpoints accept `{name}` values containing slashes for namespaced images (e.g. `bitnami/nginx:latest`, `ghcr.io/owner/repo:tag`) — these will **not** match `/images/*/json` and will still 403. Only single-segment image references (e.g. `nginx:latest`, `alpine`) will pass. This is an accepted limitation for this hotfix (see Section 5 and Section 7 risk table) — building a correct multi-segment matcher is a matching-engine change, which is out of scope for a tight hotfix. Document it in a code comment and flag it to Supervisor.

Existing test file: `backend/internal/orthrus/muzzle_test.go` (160 lines), table-style tests: `TestMuzzle_AllowlistedGET_Passthrough`, `TestMuzzle_VersionPrefixStripped_Passthrough`, `TestMuzzle_POST_Blocked`, `TestMuzzle_DELETE_Blocked`, `TestMuzzle_HEAD_Ping_Passthrough`, `TestMuzzle_HEAD_NonPing_Blocked`, `TestMuzzle_DynamicPaths_Passthrough` (the pattern-list test, lines 115–141), `TestMuzzle_UnknownPath_Blocked` (lines 143–160).

### 2.2 Gap 3 — hardcoded hostname

`backend/internal/orthrus/session.go`:

- `ExternalProxyStatus` struct, lines 95–102:
  ```go
  type ExternalProxyStatus struct {
  	ConfiguredPort   int    `json:"configured_port"`
  	ActivePort       int    `json:"active_port"`
  	BoundAddress     string `json:"bind_address"`
  	ConnectionString string `json:"connection_string,omitempty"` // e.g. "tcp://charon:9999"
  	Active           bool   `json:"active"`
  	Error            string `json:"error,omitempty"`
  }
  ```
- `AgentSession.GetExternalProxyStatus()`, lines 329–361. The hardcoded construction is at line 350:
  ```go
  connStr := ""
  if active && activePort > 0 {
  	connStr = fmt.Sprintf("tcp://charon:%d", activePort)   // line 350 — hardcoded hostname
  }
  ```
  `AgentSession` has no `*gin.Context` or any request-scoped data available anywhere in `session.go` — it is a long-lived per-tunnel object, not a per-HTTP-request one. There is no way to fix this correctly inside `session.go`; the fix must happen at the HTTP layer, in the handler.

- **Confirmed: `ExternalProxyStatus.ConnectionString` has exactly two production call sites** (verified via repo-wide grep, excluding tests): the field definition/doc comment (`session.go:99`), the assignment (`session.go:357`), and the read site in `backend/internal/api/handlers/orthrus_handler.go:203` (`resp["connection_string"] = status.ConnectionString`). No other package reads or writes this field. It is therefore safe to remove the field from the `ExternalProxyStatus` struct entirely (see Section 3.2) rather than leave a permanently-empty dead field — consistent with `CLAUDE.md`'s CLEAN mandate ("Delete dead code immediately... unused types").

`backend/internal/api/handlers/orthrus_handler.go`:

- `GetInstallSnippets` (lines 152–175) is the proven pattern to replicate:
  ```go
  charonURL := c.GetHeader("X-Charon-URL")
  if charonURL == "" {
  	// NOTE: TLS detection via c.Request.TLS is unreliable when Charon runs behind a
  	// reverse proxy (e.g., Caddy) that terminates TLS and strips or rewrites headers.
  	// The X-Charon-URL header allows callers to pass the correct public URL explicitly;
  	// if absent, we fall back to heuristic detection. Users deploying behind a proxy
  	// should set the X-Charon-URL header from the frontend (window.location.origin).
  	scheme := "https"
  	if c.Request.TLS == nil {
  		scheme = "http"
  	}
  	charonURL = scheme + "://" + c.Request.Host
  }
  ```
  Note this builds a *full URL* (`scheme://host[:port]`) because install snippets embed a full HTTP(S) base URL. `GetProxyStatus` has a **different** output shape requirement: a bare **hostname only** (no scheme, no port — the docker port is independent of Charon's own web port and gets appended separately as `tcp://<host>:<activePort>`). This is why Gap 3's fix introduces a small, separate helper rather than literally reusing `GetInstallSnippets`'s block verbatim — see Section 3.3 and Section 7 for the explicit reasoning on why this isn't forced into one shared function.

- `GetProxyStatus` (lines 180–210) — the target of the fix:
  ```go
  func (h *OrthrusHandler) GetProxyStatus(c *gin.Context) {
  	uuid := c.Param("uuid")
  	agent, err := h.svc.Get(uuid)
  	if err != nil {
  		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
  		return
  	}
  	resp := gin.H{
  		"agent_uuid":        agent.UUID,
  		"agent_online":      false,
  		"configured_port":   agent.ExternalProxyPort,
  		"active":            false,
  		"active_port":       0,
  		"bind_address":      "",
  		"connection_string": "",
  		"error":             "",
  	}
  	if h.proxyResolver != nil {
  		if status, ok := h.proxyResolver.GetExternalProxyStatus(uuid); ok {
  			resp["agent_online"] = true
  			resp["active"] = status.Active
  			resp["active_port"] = status.ActivePort
  			resp["bind_address"] = status.BoundAddress
  			resp["connection_string"] = status.ConnectionString   // line 203 — currently just relays session's hardcoded value
  			if status.Error != "" {
  				resp["error"] = status.Error
  			}
  		}
  	}
  	c.JSON(http.StatusOK, resp)
  }
  ```
  This handler has `*gin.Context` (i.e. `c.Request.Host`, `c.GetHeader(...)`) — exactly what's missing in `session.go`.

- `orthrusProxyStatusResolver` interface (lines 15–18) is satisfied by `*orthrus.OrthrusServer`, whose own `GetExternalProxyStatus` (`backend/internal/orthrus/server.go:140–147`) just delegates to `AgentSession.GetExternalProxyStatus()`. No changes needed to `server.go` or the interface signature — `ExternalProxyStatus` stays the transport type between session/server/handler, just without the `ConnectionString` field.

### 2.3 Frontend — confirmed no changes needed

- `frontend/src/api/orthrus.ts:101–104` (`getAgentProxyStatus`) calls `GET /orthrus/agents/:uuid/proxy-status` **without** an `X-Charon-URL` header (unlike `getInstallSnippets`, line 94–99, which does send it). This is fine: the handler's fallback path (`c.Request.Host`) covers this case identically to how `GetInstallSnippets` already works when the header is absent — no frontend change is required to get a correct, non-hardcoded hostname. (Optionally, a future PR could add the header here for parity/robustness behind a reverse proxy that rewrites `Host`; explicitly flagged as out of scope, Section 5.)
- `frontend/src/components/hecate/AgentExternalProxyDialog.tsx:184–194` renders `proxyStatus.connection_string` verbatim (no parsing, no hostname logic) — any string the backend returns displays correctly with zero frontend changes.
- `frontend/src/api/orthrus.ts:31–40` (`ExternalProxyStatus` TS interface) matches the handler's `gin.H` response shape exactly and needs no changes — the JSON key `connection_string` is unaffected by this fix; only the Go-side *value construction* moves from `session.go` to the handler.
- Frontend/E2E tests (`frontend/src/components/hecate/__tests__/AgentExternalProxyDialog.test.tsx`, `frontend/src/api/__tests__/orthrus.test.ts`, `frontend/src/hooks/__tests__/useOrthrus.test.tsx`, `tests/orthrus-external-proxy.spec.ts`) all mock the proxy-status HTTP response directly (`route.fulfill({ json: ... })` in Playwright, `vi.mock`/manual mocks in Vitest) with a hardcoded `'tcp://charon:2375'` string as **test fixture data**, not as an assertion on backend hostname-construction logic. They will continue to pass unmodified because they never exercise the real Go handler — **confirmed, zero frontend or E2E test changes required**.

**Conclusion: the "frontend files are NOT expected to need changes" assumption from the task brief holds.** No file under `frontend/` is touched by this hotfix.

### 2.4 Docs — confirmed current gaps

- `docs/features/orthrus.md` (122 lines) — no mention of `ExternalProxyPort`, the gear icon, or the external proxy at all. Has an existing "What Orthrus Can (and Cannot) Do" section (lines 88–104) and a "Troubleshooting" table (lines 108–116) that are the natural insertion points.
- `docs/guides/remote-docker-setup.md` (156 lines) — same gap. Has a "Troubleshooting" table (lines 142–150) and ends with a "What's Next?" section (lines 154–156) that are natural insertion points; Step 6 ("Use Your Remote Containers", lines 111–124) is the last numbered step, so a new step can be added after it as an explicitly optional step.
- `docs/features.md:206–210` already links to `features/orthrus.md` with a two-sentence summary — per `CLAUDE.md`'s "keep it brief, link to individual docs" instruction, this file's existing brevity is already correct and **needs no edit**.
- `ARCHITECTURE.md` — no Orthrus-specific content to update (grepped, zero hits); this hotfix changes neither system architecture, tech stack, deployment model, nor directory structure, so no `ARCHITECTURE.md` update is required per its own trigger criteria.

### 2.5 Config/ignore files — confirmed no updates needed

Checked per `CLAUDE.md`'s process requirement:

| File | Exists? | Orthrus references? | Action |
|---|---|---|---|
| `.gitignore` | Yes | None | No change — no new files/directories introduced |
| `.dockerignore` | Yes | None | No change — no new build artifacts |
| `.codecov.yml` | **Does not exist in this repo** | N/A | No change |
| `Dockerfile` | Yes | None | No change — no new binaries, ports, env vars, or build stages |

This hotfix adds no new files, no new directories, no new container ports/env vars, and no new dependencies — only edits to existing `.go` and `.md` files.

---

## 3. Technical Specifications

### 3.1 Gap 1 spec — `backend/internal/orthrus/muzzle.go`

**Change**: append two entries to `allowedDockerPatterns` (lines 42–49):

```go
var allowedDockerPatterns = []string{
	"/containers/*/json",
	"/containers/*/logs",
	"/containers/*/stats",
	"/containers/*/top",
	"/volumes/*",
	"/networks/*",
	"/images/*/json",       // NEW: image inspect (RepoDigests) — read-only, used by update-checker tools
	"/distribution/*/json", // NEW: registry digest check — read-only, used by update-checker tools
}
```

Add a doc comment above the block (or extend the existing one at lines 39–41) explaining:
- Both new patterns are GET-only (enforced upstream by the unconditional method check at lines 79–84, which runs regardless of path).
- Neither permits any write/mutate operation — `/images/create` (image pull) is explicitly and deliberately **not** added.
- The `path.Match` single-segment-`*` limitation for namespaced image names (Section 2.1) is a known, accepted gap for this hotfix.
- `/distribution/*/json` causes the remote Docker daemon itself to make an outbound network request to whatever registry host is encoded in the image name — read-only with respect to local Docker state (it cannot mutate anything on the host), but callers can influence which external host the daemon contacts, so "read-only" here means "cannot mutate local Docker state," not "cannot cause outbound network activity."
- This is a best-effort expansion (no live traffic logs available from the reporting environment); more read-only entries may be needed if real-world testing against tools like Dockhand/Diun surfaces additional 403s — expected and acceptable follow-up, not a defect in this PR.

**No changes** to `allowedDockerPaths` (the exact-match map), `Muzzle.ServeHTTP`, `versionPrefixRe`, or `sanitizePath`.

#### Test additions — `backend/internal/orthrus/muzzle_test.go`

1. Extend `TestMuzzle_DynamicPaths_Passthrough` (lines 115–141) with:
   ```go
   "/images/alpine/json",
   "/v1.44/images/alpine/json",
   "/images/nginx:latest/json",
   "/distribution/alpine/json",
   "/v1.44/distribution/alpine/json",
   ```
2. Extend `TestMuzzle_UnknownPath_Blocked` (lines 143–160) with a case documenting the known multi-segment limitation, with an explanatory comment so it reads as an intentional regression-guard rather than an oversight:
   ```go
   // Known limitation (see muzzle.go doc comment): path.Match's "*" does not
   // cross "/", so namespaced image names are not matched by /images/*/json.
   // This case documents current behavior; it is not a target of this fix.
   "/images/library/nginx/json",
   ```
3. New table-style test `TestMuzzle_ImageAndDistributionEndpoints_POSTBlocked` (or extend the existing `TestMuzzle_POST_Blocked`, lines 67–84) confirming `POST /images/alpine/json` and `POST /distribution/alpine/json` are still 403 — belt-and-suspenders regression guard for the read-only guarantee, even though method-checking already happens unconditionally before any path match.

**Validation gate**: `go test ./backend/internal/orthrus/... -run TestMuzzle -v`

### 3.2 Gap 3 spec — remove hardcoded hostname from `session.go`

**`backend/internal/orthrus/session.go` changes:**

1. `ExternalProxyStatus` struct (lines 95–102): remove the `ConnectionString` field entirely.
   ```go
   type ExternalProxyStatus struct {
   	ConfiguredPort int    `json:"configured_port"`
   	ActivePort     int    `json:"active_port"`
   	BoundAddress   string `json:"bind_address"`
   	Active         bool   `json:"active"`
   	Error          string `json:"error,omitempty"`
   }
   ```
   Add/adjust the doc comment on the struct to note that connection-string hostname resolution is now the API handler's responsibility (`OrthrusHandler.GetProxyStatus`), since `AgentSession` has no request context to resolve a caller-appropriate hostname from.

2. `GetExternalProxyStatus()` (lines 329–361): remove the `connStr` construction block (lines 348–351) and the `ConnectionString: connStr,` field from the returned struct literal (line 357). No other logic in this method changes.

**Validation gate**: `go build ./backend/...` (this alone will catch every call site that still references the removed field — see Section 3.2's test impact below).

### 3.3 Gap 3 spec — hostname resolution in `orthrus_handler.go`

**`backend/internal/api/handlers/orthrus_handler.go` changes:**

1. Add imports: `"fmt"`, `"net"`, `"net/url"`.
2. Add a new unexported helper, placed near `GetProxyStatus`:
   ```go
   // resolveExternalProxyHost determines the hostname third-party tools should use
   // to reach this Charon instance's external Docker proxy ports. It mirrors the
   // X-Charon-URL header pattern used by GetInstallSnippets, but — unlike that
   // handler — returns a bare hostname only (no scheme, no port): the external
   // proxy's TCP port is independent of Charon's own web port, so the docker
   // port is appended separately by the caller.
   func resolveExternalProxyHost(c *gin.Context) string {
   	if raw := c.GetHeader("X-Charon-URL"); raw != "" {
   		if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
   			return u.Hostname()
   		}
   	}
   	if host, _, err := net.SplitHostPort(c.Request.Host); err == nil {
   		return host
   	}
   	return c.Request.Host
   }
   ```
3. Update `GetProxyStatus` (lines 180–210) to build the connection string itself instead of relaying `status.ConnectionString`. **Ordering note**: this handler change (Commit 2, Section 7) lands *before* the struct field removal (Commit 3, Section 3.2/Section 7) — at this point `ExternalProxyStatus.ConnectionString` still exists on the struct, it simply becomes unread by this handler. That compiles cleanly (Go permits unused exported struct fields); the field is deleted later, once nothing references it:
   ```go
   if h.proxyResolver != nil {
   	if status, ok := h.proxyResolver.GetExternalProxyStatus(uuid); ok {
   		resp["agent_online"] = true
   		resp["active"] = status.Active
   		resp["active_port"] = status.ActivePort
   		resp["bind_address"] = status.BoundAddress
   		if status.Active && status.ActivePort > 0 {
   			resp["connection_string"] = fmt.Sprintf("tcp://%s:%d", resolveExternalProxyHost(c), status.ActivePort)
   		}
   		if status.Error != "" {
   			resp["error"] = status.Error
   		}
   	}
   }
   ```
   This preserves the JSON response shape and key name (`connection_string`) exactly, and preserves the existing guard condition (`active && activePort > 0`, previously enforced inside `session.go`, now enforced here) that leaves `connection_string` as the zero-value `""` when the proxy isn't actually active.

**Why not consolidate with `GetInstallSnippets`'s host-resolution block into one shared helper** (explicit design note for Supervisor/reviewers): `GetInstallSnippets` needs a full `scheme://host[:port]` base URL for embedding in install snippets; `GetProxyStatus` needs a bare hostname with the port stripped, since the docker port is unrelated to Charon's web port. Forcing both through one function would require parameterizing "include scheme," "include port," and "port to substitute," adding complexity disproportionate to a tight hotfix, and would touch `GetInstallSnippets`'s already-tested, working code path unnecessarily — increasing blast radius for no behavioral gain. `GetInstallSnippets` is **not modified** by this PR. Flagged as a possible future consolidation, not undertaken here (Section 5).

#### Test additions/updates — `backend/internal/api/handlers/orthrus_handler_test.go`

1. **Required fix (assertion break now, compile-break prevention for Commit 3)**: `TestOrthrusHandler_GetProxyStatus_Connected` (lines 678–716) currently builds `liveStatus := orthrus.ExternalProxyStatus{..., ConnectionString: "tcp://charon:2375", ...}` (line 696) and asserts `assert.Equal(t, "tcp://charon:2375", resp["connection_string"])` (line 714). At this commit (Commit 2, Section 7), the `ConnectionString` field still exists on the struct — removing it is Commit 3's job (Section 3.2) — so this literal would still *compile* as-is; left unchanged, though, the assertion would fail because the handler no longer reads `status.ConnectionString`. Update proactively now, both to fix the assertion and to avoid leaving a stale field reference that would otherwise break Commit 3's standalone build once the field is deleted:
   - Remove `ConnectionString: "tcp://charon:2375",` from the `liveStatus` literal.
   - The test's `c.Request` is built via `httptest.NewRequest(http.MethodGet, ".../proxy-status", http.NoBody)` with no explicit `Host` set, so `net/http/httptest` defaults `Request.Host` to `"example.com"`. Update the assertion to `assert.Equal(t, "tcp://example.com:2375", resp["connection_string"])` to reflect the new handler-side construction.
2. New test `TestOrthrusHandler_GetProxyStatus_ConnectionString_UsesXCharonURLHeader`: set `c.Request.Header.Set("X-Charon-URL", "https://mybox.example.org:8443")` before calling `GetProxyStatus`; assert `resp["connection_string"] == "tcp://mybox.example.org:2375"` (i.e. the header's own port, `8443`, is discarded — only the hostname is taken from the header, and the docker port comes from `status.ActivePort`).
3. New test `TestOrthrusHandler_GetProxyStatus_ConnectionString_HostPortStripped`: set `c.Request.Host = "192.168.1.50:8443"` (no `X-Charon-URL` header); assert `resp["connection_string"] == "tcp://192.168.1.50:2375"` (port stripped from `Host`, replaced with `status.ActivePort`).
4. New test `TestOrthrusHandler_GetProxyStatus_ConnectionString_EmptyWhenInactive`: reuse the existing `errStatus`/inactive-status pattern (see `TestOrthrusHandler_GetProxyStatus_Connected_WithError`, lines 791–819) to confirm `resp["connection_string"] == ""` when `status.Active` is `false`, regardless of headers — regression guard for the `active && activePort > 0` guard condition moving from `session.go` into the handler.

**No changes required** in `backend/internal/orthrus/session_coverage_test.go` or `backend/internal/orthrus/session_external_proxy_test.go` — confirmed via grep that no test in either file asserts on `ExternalProxyStatus.ConnectionString`'s value or presence (`TestGetExternalProxyStatus_ErrorFieldPopulated`, `TestGetExternalProxyStatus_NotStarted`, `TestGetExternalProxyStatus_Active`, etc. only check `Active`, `ActivePort`, `BoundAddress`, `ConfiguredPort`, `Error`). Compilation alone enforces correctness of the struct-field removal for these files.

**Validation gate**: `go test ./backend/internal/orthrus/... ./backend/internal/api/handlers/... -run 'ProxyStatus|ExternalProxy' -v`

### 3.4 Gap 2 spec — documentation

#### `docs/features/orthrus.md`

Insert a new `## External Docker Proxy (Advanced)` section, placed after the existing "What Orthrus Can (and Cannot) Do" section (after line 104) and before "Troubleshooting" (line 108), matching the file's plain-language, novice-friendly tone. Content to cover:

- **What it is**: an optional TCP port that lets a *third-party tool on your network* (an update-checker like Dockhand or Diun, a monitoring dashboard, etc.) talk directly to this agent's Docker API through the same secure tunnel — without needing a separate `docker-socket-proxy` container.
- **How to turn it on**: click the gear icon next to an agent in **Remote Agents**, set a port (1024–65535), save.
- **Connection string format**: `tcp://<host>:<port>` — described generically ("`<host>`" is your Charon instance's own address, as reachable from wherever the third-party tool runs — Charon fills this in for you automatically"), explicitly **not** hardcoding `tcp://charon:<port>` as a literal example, consistent with Gap 3's fix.
- **Still strictly read-only**: reiterate the existing "no way to turn it off" promise and list what the external proxy exposes, matching the updated allowlist from Gap 1 (containers, images, networks, volumes, system info/version/events, container logs/stats/top, image inspect, and registry digest checks for update-checker tools); note that for the registry digest check specifically, "read-only" means it cannot change anything on your Docker host — it does cause your agent's Docker daemon to make an outbound request to the image's registry, which is expected behavior for update-checking tools.
- **New Troubleshooting row** appended to the existing table (lines 108–116): "Third-party tool can't reach my agent's Docker API" → Likely Cause: "Wrong port configured in the tool" → Fix: "Check the tool is using the External Proxy port shown in the gear-icon dialog — not Charon's main web port."

#### `docs/guides/remote-docker-setup.md`

Insert a new optional step, **Step 7 — (Optional) Let Another Tool Talk to Your Agent's Docker API**, after the existing Step 6 (ends line 124) and before "(Optional) Add Uptime Monitoring" (line 127), following the same numbered-step / screenshot-placeholder / bold-callout style as the rest of the file. Content: brief restatement of what the External Proxy port is, how to set it (gear icon), the generic `tcp://<host>:<port>` format, and a cross-reference link: *"Full detail: [Orthrus guide → External Docker Proxy](../features/orthrus.md#external-docker-proxy-advanced)."* Also add a matching row to this file's own Troubleshooting table (lines 142–150), same content as the `orthrus.md` row above.

**Validation gate**: manual proofread against existing tone (no automated doc linter in this repo per current tooling); confirm both new sections render correctly as Markdown (headings, tables) and all internal links resolve.

---

## 4. Data Flow Summary (Gap 3, before/after)

```mermaid
sequenceDiagram
    participant Tool as Third-party tool
    participant FE as Frontend (AgentExternalProxyDialog)
    participant API as OrthrusHandler.GetProxyStatus
    participant Sess as AgentSession.GetExternalProxyStatus

    Note over API,Sess: BEFORE — hostname hardcoded deep in session.go
    FE->>API: GET /orthrus/agents/:uuid/proxy-status
    API->>Sess: GetExternalProxyStatus()
    Sess-->>API: connection_string: "tcp://charon:2375"
    API-->>FE: relay verbatim

    Note over API,Sess: AFTER — session reports raw state only; handler resolves hostname
    FE->>API: GET /orthrus/agents/:uuid/proxy-status
    API->>Sess: GetExternalProxyStatus()
    Sess-->>API: active true, active_port 2375, no hostname
    API->>API: resolveExternalProxyHost(c) via X-Charon-URL header or c.Request.Host
    API-->>FE: connection_string: "tcp://resolved-host:2375"
```

No new HTTP endpoints, no new request/response fields, no DB schema changes. `agent.ExternalProxyPort` (`backend/internal/models/orthrus_agent.go:48`) is unchanged.

---

## 5. Explicit Out-of-Scope

- **No Muzzle matching-engine rework.** The `path.Match` single-segment `*` limitation for namespaced image names (Section 2.1) is documented, not fixed.
- **No broader Orthrus rearchitecture.** `StartExternalProxy`, the yamux tunnel, the Muzzle wrapping mechanism, and `AgentSession`'s lifecycle are untouched beyond the one-field struct change in Section 3.2.
- **No new UI.** `AgentExternalProxyDialog.tsx` and `OrthrusAgentManager.tsx` are not modified (Section 2.3).
- **No port-collision-detection UI polish.** The existing "reconnect notice" behavior (tested in `tests/orthrus-external-proxy.spec.ts:240-260`) is untouched.
- **No multi-hostname configuration.** `resolveExternalProxyHost` resolves one hostname per request from the caller's own perspective (header or `Host`), matching `GetInstallSnippets`'s existing single-hostname model exactly — no per-agent or per-network hostname overrides are introduced.
- **No consolidation of `GetInstallSnippets` and `GetProxyStatus` hostname logic into one shared function** — see the explicit reasoning in Section 3.3.
- **No change to `frontend/src/api/orthrus.ts`'s `getAgentProxyStatus`** to add an `X-Charon-URL` header — the fallback path already produces a correct, non-hardcoded result (Section 2.3); adding the header for extra robustness behind Host-rewriting reverse proxies is a reasonable future enhancement but is not required to close Gap 3 and is left out to keep this hotfix backend-only.

---

## 6. Security Review Requirement

**Commit 1 (Section 7) touches a security boundary.** The Muzzle allowlist is the only mechanism preventing a tunnelled remote Docker socket from exposing write/mutate capability to whatever network the external proxy port is bound on. `docs/features/orthrus.md` makes an absolute, user-facing promise about read-only enforcement. Per this repo's governance rules, `supervisor` agent review is **required before implementation of Commit 1**, specifically to verify:

1. Both new patterns (`/images/*/json`, `/distribution/*/json`) are genuinely read-only in the Docker Engine API (no known write side effects on GET).
2. No accidental widening beyond the two intended patterns (e.g. no overly broad wildcard that also matches an unintended write endpoint).
3. The method-check-before-path-check ordering in `Muzzle.ServeHTTP` (lines 79–84 run before lines 86–98) still holds after the change, so POST/PUT/DELETE to the new paths remains blocked unconditionally.
4. The documented `path.Match` multi-segment limitation (Section 2.1) is an acceptable disclosed gap, not a silent security hole.

Commits 2–5 (hostname fix, docs) are not security-boundary changes but should still pass through the same review pass for consistency, per this repo's supervisor workflow.

---

## 7. Commit Slicing Strategy

**Decision: single PR on `development`, sliced into ordered commits.** One feature (this hotfix) = one PR, per `CLAUDE.md`. Conventional commit prefix `fix:` for all behavior-changing commits (Commits 1–3); `docs:` for the documentation commit (Commit 4). Each commit carries its own tests — this repo's TDD convention for backend work, not a separate test commit.

| # | Commit | Scope | Files | Depends on | Validation gate |
|---|---|---|---|---|---|
| **1** | `fix(orthrus): allow read-only image/distribution inspect through Docker proxy muzzle` | Gap 1 — expand `allowedDockerPatterns`; add tests documenting both the new allowlist entries and the known multi-segment-name limitation | `backend/internal/orthrus/muzzle.go`, `backend/internal/orthrus/muzzle_test.go` | None (independent of Commits 2–3) | `go test ./backend/internal/orthrus/... -run TestMuzzle -v`; `make lint-staticcheck-only`; full `lefthook run pre-commit` should still be run since this is a security-boundary change (GORM scan not applicable — no model/GORM changes) |
| **2** | `fix(orthrus): resolve external proxy hostname from request context instead of hardcoding it` | Gap 3, part A — add `resolveExternalProxyHost`, update `GetProxyStatus` to construct `connection_string` from request context instead of relaying `status.ConnectionString`; update/add handler tests. `ExternalProxyStatus.ConnectionString` still exists on the struct at this checkpoint (it is removed in Commit 3) — this commit only stops *reading* it, which compiles cleanly since Go permits unused exported struct fields | `backend/internal/api/handlers/orthrus_handler.go`, `backend/internal/api/handlers/orthrus_handler_test.go` | None (independent of Commit 1; compiles standalone against the current, unmodified `session.go` — the field it stops reading is still present) | `go build ./backend/...`; `go test ./backend/internal/api/handlers/... -run ProxyStatus -v` |
| **3** | `fix(orthrus): remove hardcoded "charon" hostname from ExternalProxyStatus` | Gap 3, part B — remove the now-unread `ConnectionString` field and its hardcoded construction from `session.go`; pure cleanup with no observable behavior change, since Commit 2 already moved hostname resolution to the handler | `backend/internal/orthrus/session.go` | **Commit 2** (must land first: this commit deletes a struct field, so it only compiles standalone once its sole production reader — the handler — has already stopped referencing it) | `go build ./backend/...` (confirms zero remaining references to the removed field, now that Commit 2 has already migrated the only production reader); `go test ./backend/internal/orthrus/... -v`; `go test ./backend/...` (full backend suite — the riskier tail-end check for this two-commit pair, since this is where the struct itself changes shape) |
| **4** | `docs(orthrus): document the External Docker Proxy feature` | Gap 2 — new sections in both docs files, generic (non-hardcoded) connection-string example, new troubleshooting rows | `docs/features/orthrus.md`, `docs/guides/remote-docker-setup.md` | None (independent — ordered last per this repo's suggested foundation→backend→docs sequence, and to avoid describing not-yet-merged behavior) | Manual proofread; Markdown renders correctly; both new internal cross-reference links resolve |
| **5** | `fix(orthrus): verify muzzle + proxy-status hotfix end-to-end` *(hardening/verification — only added if the full-suite DoD pass in Section 8 surfaces something to fix; otherwise this step folds into Commit 3's validation and is not a separate commit)* | Full Definition-of-Done pass: run the complete existing `tests/orthrus-*.spec.ts` Playwright suite (expected: pass unmodified, Section 2.3), full backend coverage, full lint | None expected — placeholder only | Commits 1–4 | `npx playwright test --project=firefox tests/orthrus-*.spec.ts`; `bash scripts/local-patch-report.sh`; `scripts/go-test-coverage.sh`; full `lefthook run pre-commit` |

**Rationale for ordering**: Commits 1 and 2 are independent of each other (different files, different gaps) and could technically be reordered, but are kept as separate commits because Commit 1 is the security-sensitive one that most benefits from being reviewable in isolation (small, self-contained diff). Commits 2 and 3 together form one logical "hostname fix" unit (Gap 3), but are ordered so each compiles standalone on its own: Commit 2 (the handler fix) lands first and stops `GetProxyStatus` from reading `ExternalProxyStatus.ConnectionString` — the field still exists on the struct at that point, just unused, which Go permits without error. Commit 3 (the struct field removal) lands second, once Commit 2 has already eliminated the field's only production reader, so its own `go build` also passes standalone too. Deleting the field before updating its last reader — the inverse order — would break the handler-fix commit's build at that checkpoint, which is exactly the defect this ordering avoids. Docs (Commit 4) come last since they describe the post-fix allowlist contents (Gap 1) and post-fix connection-string format (Gap 3) — writing them last avoids describing not-yet-merged behavior.

### Rollback / contingency notes (for the PR as a whole)

- **Commit 1 rollback**: revert is a two-line removal from `allowedDockerPatterns`; zero blast radius outside `muzzle.go`/`muzzle_test.go`. If Supervisor review (Section 6) flags either new pattern as unsafe, drop only that one pattern and keep the other — they are independent list entries.
- **Commits 2+3 rollback**: revert in reverse order of application if both must be undone — Commit 3 first, then Commit 2 — since Commit 3 depends on Commit 2 having already landed. Commit 3 can also be safely reverted alone at any time: it only re-adds an unused struct field and its assignment, which nothing reads, so this is a behavioral no-op. Commit 2 should **not** be reverted while Commit 3 is still applied — the handler would go back to referencing a field that no longer exists on the struct, breaking the build. If both are reverted, `connection_string` reverts to the hardcoded `"tcp://charon:<port>"` behavior — a regression, but not a security issue, and matches current `main` behavior, so this is a safe fallback if an unexpected issue surfaces post-merge.
- **Commit 4 rollback**: docs-only, zero functional risk; can be reverted or amended independently at any time without touching code.
- **If Monday's merge window is missed**: no urgency-driven shortcuts (no `--no-verify`, no skipping Supervisor review) — `CLAUDE.md`'s emergency-bypass path requires a follow-up issue and is not warranted here since none of the three gaps are actively exploited or user-blocking; slipping to the next merge window is an acceptable contingency.

---

## 8. Acceptance Criteria (Definition of Done)

This hotfix follows the full `CLAUDE.md` Definition of Done (Task Completion Protocol), scoped to the files touched:

1. **Playwright E2E**: `npx playwright test --project=firefox` — full suite passes; `tests/orthrus-external-proxy.spec.ts`, `tests/orthrus-proxy-paths.spec.ts`, `tests/orthrus-agent-install.spec.ts`, `tests/orthrus-agents.spec.ts`, `tests/uptime-orthrus.spec.ts` specifically confirmed to pass **unmodified** (Section 2.3).
2. **GORM Security Scan**: not triggered — no changes under `backend/internal/models/**`, no GORM queries or migrations touched.
3. **Local Patch Coverage Preflight**: `bash scripts/local-patch-report.sh` — artifacts generated at `test-results/local-patch-report.md` / `.json`.
4. **Security Scans**: `lefthook run pre-commit` (CodeQL Go + JS) — zero high/critical findings; Trivy container scan — no new findings (no new dependencies).
5. **Staticcheck**: `make lint-staticcheck-only` — zero errors on `muzzle.go`, `session.go`, `orthrus_handler.go`.
6. **Coverage**: `scripts/go-test-coverage.sh` — overall ≥85%, with the new/changed lines in `muzzle.go`, `session.go`, `orthrus_handler.go` fully covered by the tests specified in Sections 3.1 and 3.3.
7. **Build**: `cd backend && go build ./...` succeeds (no frontend build step required — no frontend files touched, but `cd frontend && npm run build` should still be run as a smoke check per the standard DoD).
8. **Muzzle allowlist regression guard**: `TestMuzzle_UnknownPath_Blocked` and `TestMuzzle_POST_Blocked` continue to pass, proving no unintended widening.
9. **Hostname regression guard**: no production code anywhere in `backend/` contains the literal string `"charon:"` inside a `fmt.Sprintf("tcp://...")`-style construction (manual grep check as a final gate: `grep -rn 'tcp://charon' backend/ --include="*.go"` should return zero matches in non-test files after Commits 2–3 land).
10. **Docs render correctly**: both new sections in `docs/features/orthrus.md` and `docs/guides/remote-docker-setup.md` reviewed for tone-match and correct Markdown rendering; no hardcoded `tcp://charon:<port>` example anywhere in either doc.
11. **Supervisor sign-off** obtained per Section 6 before Commit 1 is written, and again before the PR is opened for merge.

# Recommend Orthrus Over the Raw Docker Socket Mount

Status: **approved by owner**; open questions Q1-Q3 decided (see below). Branch: `fix/orthrus-first-docker-sources` (already created off `development`; one PR into `development`, mergeable only as a whole). Nothing in this spec is implemented yet; all line numbers were verified against `development` at `d20c0c24` and must be re-checked at implementation time.

## DECISIONS RECORDED BY THE OWNER (formerly open questions)

- **Q1, page name: DECIDED "Sources"** (not "Docker Sources"). Singular "Source" for add/edit/delete wording.
- **Q2, same-host topology: DECIDED (partly).** The owner's real same-host test used a **tailnet IP address as the address the Orthrus agent dials** (agent to Charon over Tailscale, so the connection is WireGuard-encrypted). It matches none of the three topologies first proposed. The docs present the server URL as **"any address the agent can reach Charon on"** and lead with that, with the tailnet/VPN IP as the worked example, then the shared Compose network (`ws://charon:8080`) and host networking (`network_mode: host`, `ws://localhost:8080`) as alternatives, then the public `wss://` URL. The docs say plainly that `ws://` over a private encrypted overlay (Tailscale/WireGuard) is acceptable and plain `ws://` over an untrusted network is not. The docs must not claim anything about the tailnet flow beyond what the owner reported: it works, and the agent dials the tailnet IP. Whether to resolve the `ws://` policy drift (F1) is handled as a separate issue, not in this PR.
- **Q3, translations: DECIDED out of scope.** The owner's earlier Crowdin sync merged untranslated files and cleaning that up is a separate future task; non-English translations are on the back burner unless a user requests one. This PR changes values in `frontend/src/locales/en/translation.json` **only**; no machine translation and no edits to `de`/`es`/`fr`/`zh` or the other 23 locale files. Known consequence: non-English locales keep showing the old "Remote Servers" wording (and fall back to English for the new keys) until the translation task is done. Follow-up issue F5 below tracks it.

## 1. Introduction

### 1.1 Background

Aikido flagged that mounting `/var/run/docker.sock` into Charon is insecure and that Orthrus should be used even on a local server. The project owner decided: **keep the raw socket mount working** (no existing setup may break), but make **Orthrus the documented, recommended** connection method everywhere, with the raw mount presented as a supported alternative. The owner verified manually that Charon works with no socket mount and an Orthrus agent running beside it on the same server.

### 1.2 Objectives (one PR, three workstreams)

| # | Workstream | Type |
| --- | --- | --- |
| WS2 | Stop offering "Local (Docker Socket)" in the Proxy Host form when no socket is reachable | UX bug fix |
| WS3 | Rename the user-visible "Remote Servers" label to match the form's "Source" dropdown | Label change |
| WS1 | Docs and compose comments: Orthrus recommended, raw mount supported, socket proxy as middle ground | Docs |

### 1.3 Priorities & Ordering

This is our own finding and upgrade, not a user-submitted issue, so the rule is: bug fixes before new implementation.

1. **WS2 first (fix).** It is a real defect that becomes much more visible once docs steer users toward no-socket setups: with no socket mounted the form still offers a Source that can only fail. It also has the only new backend surface, so it carries the most risk and should land earliest in the branch.
2. **WS3 second (label change).** Small, mechanical, independent of WS2, but must precede the docs so the docs use the final page name.
3. **WS1 last (docs).** Docs describe the final UI (new page name, new disabled-state hint), so they are written after the UI is settled.
4. E2E specs are written first as `test.fixme` (CLAUDE.md sequence) and enabled in the commit that implements each behavior.

No critical/high findings were discovered, so nothing needs its own separate PR.

**Out-of-scope findings (file as GitHub issues; do NOT absorb into this PR):**

| # | Finding | Where | Severity | Suggested approach |
| --- | --- | --- | --- | --- |
| F1 | Agent code and agent docs disagree on `ws://`. `agent/README.md:63` says "Only `ws://localhost` is permitted", but `validateServerURL` (`agent/main.go:118-143`) accepts `ws://` for any host and only logs a warning for non-localhost. | `agent/main.go`, `agent/README.md` | low | Decide which is intended. Tighten code or fix the README. This spec only documents the actual behavior (see 3.4) and flags the drift. Touches the separately released agent, so keep it out of this PR. |
| F2 | `docker-compose.remote.yml` and the Orthrus install snippets mount the socket into the agent without a note that write access is opt-in. Verify and document. | `.docker/compose/docker-compose.remote.yml:19` | low | Comment-only change can ride in WS1 (included below). A deeper review of agent privilege defaults is a separate issue. |
| F3 | Hard-coded English strings throughout `ProxyHostForm.tsx` (labels, "Docker Connection Failed" panel, troubleshooting text). WS2 only internationalizes the strings it touches. | `frontend/src/components/ProxyHostForm.tsx` | low | Separate i18n sweep issue. |
| F4 | `RemoteServerForm.tsx:144` hard-codes "Edit Remote Server" / "Add Remote Server" (not i18n'd). WS3 changes the wording in place and moves it to i18n only if trivial; a full i18n pass is a separate issue. | `frontend/src/components/RemoteServerForm.tsx` | low | Same as F3. |
| F5 | Non-English locale files are untranslated English copies (earlier Crowdin sync merged untranslated files) and now show the old "Remote Servers" wording after this PR; `de`/`es`/`fr`/`zh` hold old translations. | `frontend/src/locales/*/translation.json` | medium/low | File a GitHub issue referencing this PR: clean up the untranslated Crowdin-merged files and translate the "Sources" strings (and the new Docker-source hint keys) when the translation task is picked up or a user requests a language. Backlog, not scheduled. |

## 2. Research Findings

### 2.1 WS2 root cause (traced end to end per the Root Cause Analysis Protocol)

| Step | Location | Behavior |
| --- | --- | --- |
| Entry | `frontend/src/components/ProxyHostForm.tsx:765` | `<SelectItem value="local">Local (Docker Socket)</SelectItem>` is hard-coded, unconditional. `connectionSource` state (line 396) defaults to `'custom'`. |
| Hook | `frontend/src/hooks/useDocker.ts` | `useDocker('local')` calls `dockerApi.listContainers('local')`; query `enabled` only when a host or server id is set; `retry: 1`. 503 responses become `Error(details)`. |
| API client | `frontend/src/api/docker.ts` | Only `listContainers(host?, serverId?)` -> `GET /docker/containers`. No availability call exists. |
| Handler | `backend/internal/api/handlers/docker_handler.go` `ListContainers` | Accepts `host` only as `""`/`"local"` (SSRF hardening). `server_id` resolves a `RemoteServer`; Orthrus servers resolve via `orthrusResolver.GetProxyAddr`. Maps `DockerTimeoutError` -> 503 "responding slowly", `DockerUnavailableError` -> 503 + `details`, anything else -> 500. |
| Service | `backend/internal/services/docker_service.go` | `NewDockerService()` resolves the local socket path once at startup via `os.Stat` (default `/var/run/docker.sock`, rootless path, or `DOCKER_HOST` unix://). `initErr` stub if the client cannot be built. `isDockerConnectivityError` + `buildLocalDockerUnavailableDetails` already classify ENOENT (socket not found), ECONNREFUSED, EACCES/EPERM (permission, including the moby v0.5.1 message-text case from `docs/plans/docker_permission_denied_classification.md`). |
| Exit | `ProxyHostForm.tsx:801-840` | The red "Docker Connection Failed" panel appears only after the user has chosen Local and the list call failed. |

Root cause: the backend already knows (and classifies) whether the local socket works, but nothing exposes that to the UI without performing a full container listing as a side effect of selecting the option. The form therefore cannot decide to offer the option up front. Existing endpoints reviewed: no system/health/status handler reports Docker availability (`grep docker` over `system_handler.go` and health handlers: no hits), so **a small new endpoint is needed**. It reuses the existing classification helpers; it adds no new error taxonomy.

### 2.2 WS3 provider investigation (decides "Sources" vs "Docker Sources")

Evidence:

- `backend/internal/models/remote_server.go:25`: `Provider` comment lists `"docker", "vm", "cloud", "manual"`.
- `frontend/src/components/RemoteServerForm.tsx:181-183`: the Provider dropdown offers **Generic, Docker, Kubernetes**; default is `generic` (`:32`).
- `ProxyHostForm.tsx:767` filters the Source dropdown to `provider === 'docker' && enabled`. Non-docker entries exist and are shown on the Remote Servers page, they just are not selectable as Sources.
- `docs/api.md:1216` documents `provider` as one of `generic`, `docker`, `kubernetes`, `aws`, `gcp`, `azure` (default `generic`), so more non-Docker providers are documented than the form offers.
- `ConnectionType` values: direct, orthrus, cloudflare, tailscale, netbird, zerotier (Hecate). All are *how Charon reaches* the host, not what runs on it.
- The page is mounted at `/hecate/remote-servers` (`App.tsx:85`) with a legacy redirect from `/remote-servers` (`App.tsx:91`); the nav entry is `Layout.tsx:128` (`t('navigation.remoteServers')`), nested under the Hecate group.

Conclusion: the page lists Generic and Kubernetes entries as well as Docker ones, and `provider` is a free-form string the backend does not constrain. Calling the page "Docker Sources" would be wrong for existing non-Docker rows and would need renaming again if the Kubernetes/Generic providers gain discovery.

**Decision: "Sources"** (singular "Source" for the add/edit/delete wording). It matches the Proxy Host form's `Source` label exactly, stays true for every provider, and is shorter in the sidebar. **Rejected alternative: "Docker Sources"** (more self-explanatory to a novice, but inaccurate for Generic/Kubernetes rows). Decided by the owner: **"Sources"**.

### 2.3 WS3 text inventory (user-visible only)

Route paths (`/hecate/remote-servers`, `/remote-servers` redirect), API endpoints (`/remote-servers`), model/table names, hooks (`useRemoteServers`), files (`RemoteServers.tsx`), and JSON keys are **unchanged**. i18n key names (`remoteServers.*`, `navigation.remoteServers`) are also unchanged: they are internal identifiers, and renaming keys across 28 locale files is churn with no user benefit.

Frontend strings (English, `frontend/src/locales/en/translation.json`):

| Key | Current | New |
| --- | --- | --- |
| `navigation.remoteServers` (line 57) | Remote Servers | Sources |
| `remoteServers.title` (line 757) | Remote Servers | Sources |
| `remoteServers.deleteServer` | Delete Remote Server | Delete Source |
| `remoteServers.noServers` | No Remote Servers | No Sources |
| `remoteServers.description` | Manage backend servers for your proxy hosts | review wording, e.g. "Manage where your proxy hosts get their containers and backends" |
| `uptime.remoteServers` (691) and `uptime.noMonitors`/`noMonitorsFound` | Remote Servers / "...Proxy Host or Remote Server..." | Sources / "...Proxy Host or Source..." |
| `dashboard.remoteServers` (816) | Remote Servers | Sources |
| `notificationProviders.remoteServers` (953) | Remote Servers | Sources |
| `hecate.page.confirmDeleteDescription`, `hecate.page.emptyState.description` | "...Remote Servers using this provider..." / "...for Remote Servers." | "...Sources..." (this is the Hecate setup wording called out by the owner) |
| `systemSettings.features.uptimeMonitoringTooltip` | "...proxy hosts and remote servers." | "...proxy hosts and sources." |
| `RemoteServerForm.tsx:144` (hard-coded) | Edit/Add Remote Server | Edit/Add Source (route through i18n if trivial, else in place; see F4) |

Other source references to review and update only if user-visible: `frontend/src/pages/Dashboard.tsx:192` (`dashboard.remoteServers`), `pages/Uptime.tsx:657`, `pages/Notifications.tsx:353`, `components/Layout.tsx:128`. Code comments (`App.tsx:90`, `ProxyHostForm.tsx:636/640`) and `api/uptime.ts:166` doc comments: update wording for consistency, no behavior.

Tests that assert the old text (must be updated in the same commit): `frontend/src/components/__tests__/Layout.test.tsx` (lines 161, 547-550, 628, 643), `frontend/src/pages/__tests__/RemoteServers.test.tsx:259` (`/no remote servers/i`), `frontend/src/pages/__tests__/Uptime.test.tsx` (27, 214), `Uptime.tcp-ux.test.tsx:21`, `Uptime.spec.tsx` (204-213). Playwright: `tests/core/hecate-navigation.spec.ts` (6, 69, 143-156 `/remote servers/i`), `tests/hecate-tunnel-manager.spec.ts` (364, 385 test titles and any heading/link locators), `tests/modal-dropdown-triage.spec.ts:278`. `tests/a11y/domains.a11y.spec.ts:8` only uses a route and a name for reporting; update the display name, route stays. Re-grep at implementation time (`grep -rniE "remote[- ]servers?" tests frontend/src`) because other specs may match via locators.

Additional UI/test items found by a case-insensitive re-grep (`grep -rniE "remote[ -]servers?"` over frontend, tests, docs, root files, excluding `docs-site/docs`, `docs/plans`, `docs/reports`, `CHANGELOG.md`):

- Frontend unit tests asserting old text: `components/__tests__/RemoteServerForm.test.tsx:88,108` ("Add/Edit Remote Server"), `pages/__tests__/Dashboard.test.tsx` (grep for the dashboard card label and update if it asserts "Remote Servers"), `components/__tests__/ProxyHostForm.test.tsx:1432` (`describe('Remote Server Container Mapping')`, cosmetic rename only).
- Key `hecate.page.emptyState.description` (`en/translation.json` ~2111) is in the table above and in the 12-key per-locale list below.
- Playwright files that match only the API path or test IDs and are **left unchanged**: `tests/orthrus-agents.spec.ts`, `orthrus-agent-install.spec.ts`, `orthrus-external-proxy.spec.ts`, `orthrus-write-mode.spec.ts` (`REMOTE_SERVERS_API` URL glob), `tests/security-enforcement/authorization-rbac.spec.ts` (API paths), `tests/settings/notifications.spec.ts` and `tests/e2e/notifications-webpush.spec.ts` (`data-testid="notify-remote-servers"`; test IDs are not user-visible and stay). Decisions: `tests/uptime-orthrus.spec.ts:13,152,159` ("Remote Server (Orthrus)") is fixture monitor *data* and stays; `tests/fixtures/uptime.ts:123,133` comments stay; `tests/orthrus-proxy-paths.spec.ts:13,60` is a comment and a test title, update the title wording to "source" for consistency (no locator impact).
- Other non-test code mentions that are identifiers/comments (`frontend/src/api/remoteServers.ts`, `hooks/useRemoteServers.ts`, `api/docker.ts`, `.github/workflows/orthrus-build.yml`, `.docker/compose/README.md`, `.docker/compose/docker-compose.remote.yml`) stay unless they are user-facing prose; `.docker/compose/README.md:12` ("deploying to remote servers") is generic English and stays.

Docs that name the page (complete list; edited in C4 (UI-facing names) and C5 (docs)): `docs/index.md:39`, `docs/database-schema.md:301` (sample data wording, leave unless it says the page name), `ARCHITECTURE.md:420,493,1037` (generic "remote servers"/"remote-server" prose; only change where it names the UI page, otherwise leave), `docs/features/user-accounts.md:23`, `docs/features/orthrus.md:74` ("adding a Remote Server") and `:191`, `docs/guides/remote-docker-setup.md:100` (stale text: "Go to Settings -> Docker (you may also find this under Remote Servers)"; there is no such Settings -> Docker path, rewrite to the Sources page via the Hecate sidebar group, verify against the live UI), `docs/features.md:247` (lower-case "remote servers" twice plus "Remote Servers page"), `docs/features.md:261,263` (Remote Server / remote server), `docs/features/hecate.md` (7 hits), `docs/security.md:27`, `SECURITY.md` (1 hit, re-check), `docs/api.md` (21 hits; headings document the API, keep, add one note "shown as Sources in the UI").

Docs that name the page, original list: `CHANGELOG.md` (history; leave entries as written, they are release-please generated), `docs/features.md:247`, `docs/features/hecate.md:94`, `docs/guides/remote-docker-setup.md:100`, `docs/security.md:27`, `docs/features/user-accounts.md:23`, `docs/api.md:1134` (API heading: keep, it documents the endpoint `remote-servers`; add a parenthetical "shown as Sources in the UI"), `docs/issues/hecate-manual-test-plan.md` (current manual test plan; update), archived/created issue docs and `docs/plans/**` (historical; do not touch).

**Locale handling (decided, Q3):** only `frontend/src/locales/en/translation.json` is edited. A per-key comparison (script, not file size) found 23 locales identical to English on all 12 affected keys and `de`, `es`, `fr`, `zh` translated on 9 of 12; none of these files are touched. i18next has `fallbackLng: 'en'` (`frontend/src/i18n.ts:24`) and the `i18n-json/identical-keys` rule only warns for missing keys (`eslint.config.js`, "en is the source of truth"), so lint passes. Consequence: all non-English locales keep showing "Remote Servers" (or their old translation) until the future translation task (F5). Verified no Vitest test depends on non-English strings changing: `grep` finds no test importing `locales/de|es|fr|zh`; the only locale-reading test is `CrowdSecConfig.i18nKeys.test.ts`, unrelated to these keys; component tests render with `en` or mock `react-i18next`.

### 2.4 WS1 inventory and orthrus facts

Files that mount the socket (checked each):

| File | Action |
| --- | --- |
| `.docker/compose/docker-compose.yml:51` (production) | Keep mount; replace comment with pointer to Orthrus recommendation and docs link. |
| `.docker/compose/docker-compose.local.yml:46` | Keep; same comment. |
| `.docker/compose/docker-compose.dev.yml:37` | Keep (dev flow uses local discovery); comment. |
| `.docker/compose/docker-compose.override.example.yml:10-26` | Keep (documents `group_add` for the raw mount); add header note that Orthrus is recommended. |
| `.docker/compose/docker-compose.playwright-ci.yml:112,139`, `docker-compose.playwright-local.yml:86` | **Do not remove** (E2E "container discovery in tests"). Comment-only: note it is a test fixture. |
| `.docker/compose/docker-compose.remote.yml:19` | This is the **Orthrus agent's** mount (the agent needs it to read Docker). Keep; add comment that write access is opt-in in Charon (cross-check `docs/features/orthrus.md` "Opting an agent into write access"). |
| Root compose snippets | No root-level compose file exists (`ls *.yml` shows only `codecov.yml`, `lefthook.yml`). Snippets live in docs: `README.md:93`, `docs/getting-started.md:61,85,101,137,147`, `docs/security.md:1431`, `SECURITY.md:1383`, `docs/features/docker-integration.md:45-65`, `docs/guides/podman-usage.md`, `docs/guides/crowdsec-setup.md`, `docs/configuration/emergency-setup.md` (check each; most are generic Charon compose examples and just need the one-line pointer or nothing). |
| `ARCHITECTURE.md` and `.github/instructions/ARCHITECTURE.instructions.md` | `ARCHITECTURE.md:1385` (volume table), `:1426` (compose example), `:2114` ("Requires docker.sock mount (security trade-off)") and the mirrored `.github/instructions/ARCHITECTURE.instructions.md:833,863,1402` (re-verified: the two files carry the same lines at different offsets; edit both identically). Neither file lists `/docker/containers` or any docker route today (grep: no hits), so `/docker/status` is documented in `docs/api.md` only; add an ARCHITECTURE mention of the status probe next to `DockerService` (line ~371) and mirror it. Volume-table row: "Optional; Orthrus recommended"; update the compose example comment. |

Orthrus facts from `agent/README.md` and `agent/main.go` (for the same-host section):

- Agent env: `ORTHRUS_SERVER_URL` (`wss://.../api/v1/ws/orthrus/connect`), `ORTHRUS_AGENT_ID`, `ORTHRUS_AUTH_KEY`. The agent only dials outward.
- URL scheme rule (actual code, `validateServerURL`): `wss://` always OK; `ws://` is accepted for any host and logs a warning when the host is not `localhost`/`127.0.0.1`/`::1`; `http(s)://` and other schemes are rejected. The README says ws:// is permitted only for localhost (drift, see F1). `normalizeServerURL` appends `/api/v1/ws/orthrus/connect` when the path is empty.
- Write access claim verified: `docs/features/orthrus.md` lines 34, 90, 106-139 state agents are read-only by default and write access is an explicit per-agent opt-in (narrow fixed operation set, enforced by both Charon and the agent), configured via the External Docker Proxy flow. The `docker-compose.remote.yml` comment may say this, linking to that section; do not claim more.
- Charon listens on 8080 by default (`.docker/compose/docker-compose.yml:12`).

**What a same-host user must do (to be written into the docs; Q2 decided):** the agent runs as a separate container on the same machine and holds the socket; Charon holds none. The server URL is described as "any address the agent can reach Charon on".

1. Register an agent (Remote Agents -> Add Agent) and copy the one-time auth key.
2. Run the Orthrus Docker Compose snippet next to Charon and set `ORTHRUS_SERVER_URL` to **any address the agent can reach Charon on**. Lead with the owner-verified example: the machine's **tailnet (VPN) IP**, e.g. `ws://100.x.y.z:8080`, where the agent dials Charon over Tailscale and the link is WireGuard-encrypted (state only that it works and that the agent dials the tailnet IP; do not describe other Tailscale behavior). Alternatives, in order: (a) both containers on one Compose network with `ws://charon:8080` (`localhost` inside the agent container is the agent itself, so it cannot be used here); (b) `network_mode: host` on the agent with `ws://localhost:8080`; (c) the public `wss://` URL when Charon is behind TLS. The path is added automatically; non-localhost `ws://` logs an unencrypted-connection warning but is permitted by the code. Docs state plainly: `ws://` over a private encrypted overlay (Tailscale/WireGuard) is acceptable; plain `ws://` over an untrusted network is not, use `wss://`. The implementer verifies the documented alternatives (a)-(c) from `docs/features/orthrus.md` against the agent code, but only the tailnet flow is claimed as owner-tested.
3. Remove the socket mount (and `group_add`) from Charon's compose file; keep it only on the agent.
4. Wait for the agent to show Online, then add a Source (Docker provider, Orthrus connection) pointing at the agent. The Proxy Host form then lists it under Source.
5. Honest framing for the docs: the socket is still mounted, but into a small purpose-built agent instead of the internet-facing Charon container, read-only by default with write access an explicit per-agent opt-in.

## 3. Technical Specification

### 3.1 WS2 backend: availability endpoint

`GET /api/v1/docker/status` registered in `DockerHandler.RegisterRoutes`. Verified: `dockerHandler.RegisterRoutes(management)` (`routes.go:809`) mounts `/docker/containers` on the `management` group, i.e. `protected` plus `middleware.RequireManagementAccess()` (`routes.go:515-516`, blocks passthrough users); it is **not** on `managementAdmin`. `/docker/status` is registered in the same `RegisterRoutes`, so it gets the identical guard; the route test asserts a passthrough user is rejected and an unauthenticated request gets 401. No query params (no SSRF surface; it only checks the local socket).

Response 200 (always 200 when the handler runs; `available` carries the result):

```json
{
  "available": false,
  "reason": "socket_missing",
  "message": "No Docker socket is mounted into Charon."
}
```

| Field | Type | Notes |
| --- | --- | --- |
| `available` | bool | true when a bounded ping to the local daemon succeeded, or the result is inconclusive (see timeout row) |
| `reason` | string enum | `ok`, `socket_missing`, `permission_denied`, `unreachable`, `slow` |
| `message` | string | short, safe, human-readable; **no uid/gid/path details** (the 503 `details` already exposes those on the list endpoint; the status payload stays minimal because it is polled by the form) |

Classification (reuses `docker_service.go` helpers, no new taxonomy):

| Outcome | `available` | `reason` |
| --- | --- | --- |
| Ping OK | true | `ok` |
| `initErr` set or `extractErrno == ENOENT` / `os.ErrNotExist` | false | `socket_missing` |
| EACCES/EPERM (including the moby message-text case) | false | `permission_denied` |
| ECONNREFUSED / other connectivity error | false | `unreachable` |
| `DockerTimeoutError` (bounded timeout) | **true** (fail open: daemon reachable but slow) | `slow` |

New method `(*DockerService) LocalStatus(ctx context.Context) LocalDockerStatus` in `backend/internal/services/docker_service.go` (struct with `json:"available"`, `json:"reason"`, `json:"message"` tags, or the handler builds the JSON from a plain result struct). Uses the existing client with `client.Ping(ctx, client.PingOptions{})` (`client@v0.5.1/ping.go:73`). **Required step before coding:** read `ping.go` and confirm what `Ping` returns when the daemon answers with a non-200 or when only the HEAD fallback succeeds (lines ~73-146), and map every returned error through the existing `isDockerConnectivityError`/`extractErrno` helpers (the ping path may wrap errors differently from `ContainerList`, including the moby "permission denied while trying to connect" message text). **Probe timeout: a dedicated short constant `localStatusProbeTimeout = 2 * time.Second`**, applied with `context.WithTimeout` on the request context (not the 8s list timeout, because the form polls this on open and a hung socket must not stall the UI); overridable in tests via a struct field like `listContainersTimeout`. Ping is cheaper than `ContainerList` and avoids enumerating containers just to probe. Handler depends on a small interface (`LocalStatus(ctx)`) extending/adjoining `dockerContainerLister`; update the test double accordingly. No DB/model change, no AutoMigrate change.

Error handling: the endpoint never returns 5xx for an unavailable Docker; unavailability is the successful answer. A panic-free path for `s.client == nil`. Log at Debug (it will be polled; do not spam Warn).

### 3.2 WS2 frontend

Files:

- `frontend/src/api/docker.ts`: add `DockerStatus` interface (`available: boolean; reason: 'ok' | 'socket_missing' | 'permission_denied' | 'unreachable' | 'slow'; message: string`) and `dockerApi.getStatus()` -> `GET /docker/status`.
- `frontend/src/hooks/useDocker.ts` (or new `useDockerStatus.ts` beside it; follow the repo `use*.ts` pattern): `useDockerStatus()` via `useQuery`, `queryKey: ['docker-status']`, `staleTime` 30s, `retry: 0`, `refetchOnWindowFocus: false`.
- `frontend/src/components/ProxyHostForm.tsx` Source select (lines ~759-777):
  - Render the `local` `SelectItem` always. It is **enabled while the status query is loading** and disabled **only on a definite `available === false` response**. Loading, error, timeout, 404 from an older backend, and 5xx all keep it enabled (today's behavior). Radix `SelectItem` supports `disabled`. Trade-off accepted: in the rare case where the dropdown is opened before the (fast, 2s-bounded) probe returns, the option can flip from enabled to disabled; the reset-to-`custom` rule below covers a selection made in that window.
  - **Fail open:** a status probe failure must never remove a working option.
  - The new hook goes in a **separate file** `frontend/src/hooks/useDockerStatus.ts` (not inside `useDocker.ts`): `ProxyHostForm.test.tsx` fully mocks `../../hooks/useDocker` (line 24), so a new export there would be undefined in those tests. Add a `vi.mock('../../hooks/useDockerStatus', ...)` to that test file defaulting to a loading/undefined result (form stays enabled), and the existing cases at ~322, 334, 514, 1514, 1652, 1683 keep working without awaiting the status call.
  - Label via `t('proxyHosts.source.local')` ("Local (Docker Socket)"); when disabled append a short suffix (`t('proxyHosts.source.localUnavailable')`, "unavailable").
  - Under the Source select, when `available === false`, show a muted help line (role `status`) with `reason`-specific text (`socket_missing`: "No Docker socket is mounted into Charon, so the local option is unavailable. Orthrus is the recommended way to connect Docker without mounting the socket." `permission_denied`: "The Docker socket is mounted but Charon cannot read it. Fix the socket group access, or use Orthrus (recommended)." `unreachable`: similar) with a link to the Orthrus docs and to the Sources/Remote Agents page.
  - If `connectionSource === 'local'` and the status later reports unavailable, reset `connectionSource` to `'custom'` so the container list is not stuck on an error.
  - The existing "Docker Connection Failed" troubleshooting panel stays untouched for remote sources and for the slow case.
- i18n: add keys under a new `proxyHosts.source` object (verify the existing namespace for the proxy host form; if one exists, use it) in `en` only (Q3; other locales fall back to English). Only the strings touched here are internationalized (F3).

### 3.3 WS3 implementation notes

Change only the strings in 2.3. Do not rename files, components, routes, keys, or hooks. Keep the legacy `/remote-servers` redirect. Add no migration. Page `<title>`/h1 comes from `remoteServers.title`, so the one key change covers the page heading and any breadcrumb.

### 3.4 WS1 doc content rules

- Wording: "Orthrus (recommended)" = more secure because Charon never holds the socket; "Direct socket mount" = supported, simplest, no extra container; "Socket proxy" = hardened middle ground (existing section, unchanged content).
- `docs/features/docker-integration.md`: restructure "Configuration" into three subsections in this order: recommended Orthrus, simple raw mount (existing block plus short security caveat), hardened socket proxy (link to the existing "Limiting Socket Access with a Proxy" section, do not duplicate it). Fix the stale "Settings -> Docker / Add Remote Host" steps only if they are demonstrably wrong; otherwise leave and flag.
- `docs/features/orthrus.md`: new section "Using Orthrus on the same server as Charon" with the 5 steps from 2.4; also note the Source dropdown behavior (Local option greyed out when no socket is mounted). Update the existing "Remote Agents" navigation wording only if it is wrong in the current UI. Link back to the shared agent README facts, do not duplicate the F1 drift.
- `docs/getting-started.md`: keep the existing compose snippet working; add a short "Recommended: connect Docker with Orthrus" callout before the "Docker Socket Access (Important)" section and reword the security note. Do not remove the group-id steps.
- `README.md`: one callout after the compose example (line ~106 area).
- `docs/guides/remote-docker-setup.md`: add a "Same server as Charon" pointer, update the "Remote Servers" mention at line 100 to "Sources".
- `SECURITY.md` (near lines 1210, 1383) and `docs/security.md` (1430-1481): one-liner each: Orthrus recommended; raw mount supported; broad access regardless of `:ro`.
- `docs/features.md`: add a sentence under Docker Auto-Discovery and fix line 247 ("Remote Servers page" -> "Sources page").
- `ARCHITECTURE.md`: volume-mount table row and Compose example comment; mention `GET /docker/status` where `DockerService` is described (line ~371). Sync `.github/instructions/ARCHITECTURE.instructions.md` if it mirrors those lines.
- New standalone file check: **no new standalone docs file is created**, so `docs-site/scripts/docs-manifest.json` needs no new entry. Verified: standalone `docs/api.md` and `docs/security.md` are already in the manifest `files` array (lines 5-6), so the `docs/api.md` status-endpoint entry publishes automatically; `docs/getting-started.md` and `docs/features.md` should be re-checked in the manifest when editing. Never edit `docs-site/docs/`.

## 4. Implementation Plan (phases mapped to commits)

| Phase | Content | Commit |
| --- | --- | --- |
| 1 Playwright | `test.fixme` specs | C1 |
| 2 Backend | status endpoint + tests | C2 |
| 3 Frontend | hook, form, i18n, Vitest, enable WS2 E2E | C3 |
| 4 Integration | rename + unit and Playwright spec updates together, enable WS3 E2E | C4 |
| 5 Docs and final | docs, compose comments, final sweep, no `fixme` remains | C5 |

## 5. Commit Slicing Strategy (single PR, ordered commits)

Decision: one feature PR into `development` from a dedicated branch (`fix/orthrus-first-docker-sources`), per the CLAUDE.md Branching Strategy (new endpoint plus UI behavior change is medium/large and must not be committed directly to `development`). Every commit builds and leaves the targeted E2E specs green; the PR merges only when C1-C5 are all present and the full Definition of Done passes. Total estimate: about 3 working days (roughly 22-24h).

**PR title/type:** `fix: offer Orthrus first and hide the unusable local Docker source` (squash/merge commit type `fix:`, which triggers Docker builds; the PR contains a small new endpoint but the user-facing intent is a bug fix and a label change, so `feat:` is not required. If the owner prefers the new endpoint to count as a feature, `feat:` is acceptable and changes nothing else).

### C1 - E2E specs as fixme (Est. 2h)

- Message: `test: add fixme E2E specs for Docker source availability and the Sources rename`
- Files (new, `test.fixme`): `tests/core/proxy-host-docker-source.spec.ts`: (a) with `page.route('**/api/v1/docker/status', ...)` returning `available:false, reason:socket_missing`, open Add Proxy Host, open the Source select, assert Local option disabled and hint visible; (b) `available:true` -> enabled and selectable; (c) status 500 -> enabled (fail open); (d) delayed status response -> option enabled while loading. `tests/core/sources-navigation.spec.ts`: sidebar link `Sources` -> `/hecate/remote-servers`, heading `Sources`, `/remote-servers` still redirects.
- Locator: the existing specs open the select with `dialog.getByRole('combobox', { name: 'Source' })` (`tests/core/proxy-hosts.spec.ts:70,98,137,170,202,233`, `tests/orthrus-proxy-paths.spec.ts:70,98,137,170,202`); the trigger has `id="connection-source"` and `aria-label="Source"`. The new specs use the same role locator. `#quick-select-docker` is the **Containers** select (a different control, used elsewhere for the container list) and is not used to open Source.
- The E2E containers mount the socket (`docker-compose.playwright-*.yml`), so "no socket" is produced by route mocking.
- Dependencies: none.
- Gate: `npx playwright test tests/core/proxy-host-docker-source.spec.ts tests/core/sources-navigation.spec.ts --project=firefox` reports the tests as skipped/fixme with no failures; lint clean.

### C2 - Backend: Docker availability endpoint (Est. 4-5h)

- Message: `fix: report whether the local Docker socket is reachable` (type `fix:` is consistent with the PR type; the endpoint exists to fix the unusable-option bug and is additive).
- Files: `backend/internal/services/docker_service.go` (+ `LocalStatus`, `localStatusProbeTimeout`), `docker_service_test.go`, `backend/internal/api/handlers/docker_handler.go` (+ route, interface), `docker_handler_test.go`, `backend/internal/api/routes/routes_test.go`, `docs/api.md` (document `GET /docker/status`).
- Tests: see 3.1 (including the real unreachable-unix-path test and the management-guard test).
- Dependencies: none.
- Gate: `cd backend && go test ./internal/services/... ./internal/api/handlers/... ./internal/api/routes/...`; `go build ./...`; `make lint-fast`; patch coverage on new lines 100%; existing `ListContainers` tests untouched and green.

### C3 - Frontend: disable the unusable Local source (Est. 4.5-5h; no locale mirroring)

- Message: `fix: disable the local Docker source when no socket is reachable`
- Files: `frontend/src/api/docker.ts`, new `frontend/src/hooks/useDockerStatus.ts`, `frontend/src/components/ProxyHostForm.tsx`, `frontend/src/locales/en/translation.json` (new `source` keys, `en` only), tests below, `tests/core/proxy-host-docker-source.spec.ts` (remove `fixme`).
- Vitest: `api/__tests__/docker.test.ts` (getStatus); hook test (loading, success, error); `ProxyHostForm.test.tsx` additions (enabled while loading; disabled with hint per reason; enabled on available; enabled on error; reset from `local` to `custom` when status flips unavailable) plus the new `vi.mock` of `useDockerStatus` so existing cases (~322, 334, 514, 1514, 1652, 1683) pass without awaiting the status call.
- Dependencies: C1, C2.
- Gate: `cd frontend && npm run type-check && npx vitest run src/components/__tests__/ProxyHostForm.test.tsx src/hooks src/api`; lint incl. i18n-json; `npx playwright test tests/core/proxy-host-docker-source.spec.ts tests/core/proxy-hosts.spec.ts --project=firefox` green (second spec guards the Source dropdown regression).

### C4 - Rename "Remote Servers" to "Sources", unit and E2E specs together (Est. 4-4.5h; locale sweep removed)

- Message: `fix: rename the Remote Servers page to Sources in the UI` (user-visible text change, so `fix:` to trigger the Docker build; not `refactor:`).
- Files: `frontend/src/locales/en/translation.json` only (no other locale file; Q3), `RemoteServerForm.tsx` heading, code comments (2.3), **and in the same commit** every test asserting the old text: Vitest (`Layout.test.tsx`, `RemoteServers.test.tsx`, `Uptime.test.tsx`, `Uptime.tcp-ux.test.tsx`, `Uptime.spec.tsx`, `RemoteServerForm.test.tsx`, `Dashboard.test.tsx`, `ProxyHostForm.test.tsx:1432`) and Playwright (`tests/core/hecate-navigation.spec.ts`, `tests/hecate-tunnel-manager.spec.ts`, `tests/modal-dropdown-triage.spec.ts`, `tests/a11y/domains.a11y.spec.ts` display name only, `tests/orthrus-proxy-paths.spec.ts:60` title), and `tests/core/sources-navigation.spec.ts` (remove `fixme`). Left unchanged per 2.3: `tests/uptime-orthrus.spec.ts`, `tests/fixtures/uptime.ts`, API-path and test-id matches.
- Dependencies: C3, C1. Known consequence (Q3): non-English locales keep the old wording.
- Gate: `cd frontend && npm run type-check && npx vitest run` on the touched test files; lint; `npx playwright test tests/core/hecate-navigation.spec.ts tests/core/sources-navigation.spec.ts --project=firefox`, then each other touched spec individually (firefox only); the case-insensitive re-grep in 2.3 shows no remaining UI string with the old name.

### C5 - Docs, compose comments, final sweep (Est. 7-8h)

- Message: `docs: recommend Orthrus for connecting Docker and keep the socket mount supported`
- (No `(security)` scope: hardening guidance, not a vulnerability fix.)
- Files: everything in 2.4, 3.4 and the 2.3 docs list; `.docker/compose/*.yml` comment-only edits; `ARCHITECTURE.md` and `.github/instructions/ARCHITECTURE.instructions.md` edited identically.
- Explicit final-sweep deliverables (replaces the old C7): `grep -rn "test.fixme" tests/core/proxy-host-docker-source.spec.ts tests/core/sources-navigation.spec.ts` returns nothing; a docker-integration troubleshooting note linking the disabled-state hint; the manual flow in section 6 executed once; fix anything the DoD sweep finds.
- Dependencies: C3, C4.
- Gate: **no `test.fixme` remains in this PR's specs**; full Definition of Done (section 6); `docker compose -f <file> config` succeeds for every edited compose file and `git diff .docker/compose` shows comment lines only; relative links resolve; `git status` shows nothing under `docs-site/docs/`.

## 6. Test Plan and Definition of Done

- **Go:** table-driven tests per C2; `scripts/go-test-coverage.sh` >= 85% overall; patch coverage via `bash scripts/local-patch-report.sh` (artifacts `test-results/local-patch-report.md/json`).
- **Vitest:** per C3/C4; `scripts/frontend-test-coverage.sh` >= 85%; `cd frontend && npm run type-check` and `npm run build`.
- **Playwright (targeted, firefox only, foreground):** `tests/core/proxy-host-docker-source.spec.ts`, `tests/core/sources-navigation.spec.ts`, `tests/core/hecate-navigation.spec.ts`, `tests/hecate-tunnel-manager.spec.ts`, `tests/modal-dropdown-triage.spec.ts`, `tests/core/proxy-hosts.spec.ts` (Source dropdown regression: the `Source` combobox name and Orthrus entries must be unchanged). Never run the full suite or multiple projects locally; cross-browser is CI-only.
- **GORM security scan:** **not triggered.** No change to `backend/internal/models/**`, queries, or migrations (WS3 is label-only and does not touch the `RemoteServer` model; the status endpoint has no DB access). Record this decision in the PR description. If the implementation unexpectedly touches models, run `./scripts/scan-gorm-security.sh --check`.
- **Security scans:** the PR adds a new endpoint, so run CodeQL Go/JS locally via `lefthook run pre-commit` and `make trivy` (zero high/critical); also `lefthook run pre-commit` staticcheck gate.
- **Manual check (owner-verified flow, repeat once):** (1) compose with socket mount: Local option enabled and lists containers; (2) remove mount: Local greyed out with Orthrus hint, Orthrus Source works; (3) mount present but wrong group: `permission_denied` hint.
- Clean up: no `console.log`, debug prints, or unused imports.

## 7. Risks and Rollback

| Risk | Mitigation |
| --- | --- |
| New status call wrongly hides a working Local option | Option is enabled while loading and on any error, 404, or timeout; only a definite `available:false` disables it (ENOENT/EACCES/connectivity). Unit and E2E cover loading and fail-open paths. A brief enabled-then-disabled flip is possible if the dropdown opens before the 2s-bounded probe returns; selection is reset to `custom` in that case. |
| Raw mount path regresses | The raw mount code path (`ListContainers`, `NewDockerService`, all compose mounts) is not modified; `LocalStatus` is additive. Compose edits are comments only, verified with `docker compose config` and `git diff`. Existing `ListContainers` tests must stay green unmodified. |
| Status endpoint information leakage | Response carries only a reason code and a canned message; no path/uid/gid. Behind the same `RequireManagementAccess` guard as `/docker/containers`. |
| Rename breaks bookmarks/integrations | Routes, API paths, JSON keys, i18n key names, model names unchanged; legacy redirect kept; E2E asserts redirect. |
| Non-English locales show the old name | Accepted by the owner (Q3): no locale files other than `en` are touched; tracked by follow-up issue F5. |
| Same-host Orthrus instructions untested | Only the owner-reported tailnet-IP flow is described as working; other topologies are documented as alternatives, not as tested; docs state the `ws://` behavior as the code actually behaves. |
| Docs drift (F1) | Documented as actual behavior; separate issue filed. |

Rollback: the PR is one unit; `git revert` of the merge commit restores everything. Because there is no schema change and no data change, no data migration or recovery step is needed. Partial rollback is possible per commit group (C2 and C3 together; C4 alone; C5 alone) but the PR is not to be merged partially.

## 8. Acceptance Criteria

1. With no Docker socket mounted, the Proxy Host form's `Source` dropdown shows "Local (Docker Socket)" disabled with an explanation pointing to Orthrus; with a working socket it is enabled and behaves exactly as before; status failures leave it enabled.
2. `GET /api/v1/docker/status` exists, authenticated, snake_case JSON, covered by tests.
3. The sidebar item, page heading, empty state, delete dialog, dashboard, uptime, notification-provider and Hecate setup text say "Sources" (or the owner's alternative); `/hecate/remote-servers`, `/remote-servers`, `/api/v1/remote-servers`, DB and JSON keys are unchanged.
4. Docs, README, SECURITY.md, docs/security.md, ARCHITECTURE.md and compose comments state Orthrus is recommended and the direct mount remains supported; the same-host Orthrus procedure is documented.
5. All socket mounts in compose files are still present (comment-only diff).
6. Full Definition of Done passes; CI green.

## 9. Open Questions

Q1-Q3 were decided by the owner; see the section at the top. No questions remain open.

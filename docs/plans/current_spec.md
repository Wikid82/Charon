# Spec: Per-Client Throttling for `/api/v1/auth/*` (Issue #1317)

| Field | Value |
| --- | --- |
| Issue | #1317 — "Add per-IP rate-limit / throttle middleware for /api/v1/auth/*" (labels: `critical`, `security`) |
| Follow-up to | #1316 / GHSA-3gc6-295r-xm5m (merge `8168732a`) |
| Branch / PR | `feat/auth-rate-limit-1317` → one PR into `development` |
| Status | Approved (maintainer 2026-09-24). Revision 3; see §10 |
| Date | 2026-09-24 |
| Previous spec | Redirection Hosts (#1367), archived verbatim at `docs/plans/archive/2026-09-23_redirection-hosts-1367_spec.md` |

## Table of Contents

1. [Introduction](#1-introduction)
2. [Research Findings](#2-research-findings)
3. [Technical Specifications](#3-technical-specifications)
4. [Implementation Plan](#4-implementation-plan)
5. [Acceptance Criteria](#5-acceptance-criteria)
6. [Commit Slicing Strategy](#6-commit-slicing-strategy)
7. [Risks & Mitigations](#7-risks--mitigations)
8. [Open Questions & Maintainer Decisions](#8-open-questions--maintainer-decisions)
9. [Follow-ups (Out of Scope)](#9-follow-ups-out-of-scope)
10. [Revision History](#10-revision-history)

---

## 1. Introduction

### 1.1 Overview

The public issue states the gap. `POST /api/v1/auth/login` is protected by a
per-account lockout, but one client can spread attempts across many usernames, or
hammer other `/api/v1/auth/*` routes, without hitting any IP-level ceiling. Every
password check is deliberately expensive (bcrypt).

A Go-layer per-IP limiter already exists (`backend/internal/cerberus/rate_limit.go`),
but it doesn't close this gap:

- It is **opt-in** (off by default).
- It is sized for general API traffic (100 req/60 s, burst 20).
- It has three weaknesses of its own, all fixed here (§2.3).

This spec makes four changes:

1. **Always-on per-client throttle.** It sits in front of an explicit `/api/v1/auth`
   route group, and in front of every other place that verifies an account password.
   The per-account lockout stays as the second layer.
2. **One shared limiter component** (`backend/internal/ratelimit`). It has bounded
   memory and starts no goroutines. The Cerberus limiter migrates to it.
3. **One effective proxy-trust list.** It is validated once at startup and used by
   every consumer. The throttle keys on the *true* client resolved from it.
4. **Visibility for shared-address failure modes.** A detector, an admin-only status
   endpoint, a Security-dashboard card, and documentation cover the case where
   many clients collapse into one address.

### 1.2 Objectives

1. **Secure by default.** Throttling is on with zero configuration. The zero value of
   every config struct is the secure default and never fails open.
2. **Right-sized per route.** Every route that verifies an account password draws on one
   strict per-client budget. Token/status routes get a loose budget. UX-critical session
   reads and break-glass paths are never throttled.
3. **True-client keying, one trust decision.** The key is Gin's resolved client IP (IPv6
   aggregated to /64). Gin, the cookie logic, the throttle, and the status endpoint all
   read the same validated `CHARON_TRUSTED_PROXIES` list, and the cookie logic and the
   detector match peers with the same `netip` prefix matcher.
4. **One limiter component.** It has bounded memory, no background goroutines, and an
   injectable clock. The Cerberus limiter reuses it (DRY/CLEAN).
5. **Clear contract and friendly UX.** A `429` carries `Retry-After` and a generic body.
   The login page shows a localized wait message plus a pointer for administrators.
6. **Admin visibility.** An admin-only endpoint and a Security-dashboard card show:
   - the effective budgets;
   - how Charon sees the admin's own address;
   - forwarded-header observations, with scope-aware guidance.
7. **Bounded operator signals.** A startup policy log, capped WARNs, and a Prometheus counter.
8. **Non-flaky E2E.** No production default changes. E2E relaxations are explicit and test-only.

### 1.3 Non-Goals

- **Shared or persisted limiter state.** No distributed state (Redis) and no persistence
  across restarts. Charon is single-instance, and buckets are in-memory by design.
- **Other anti-automation controls.** No CAPTCHA, proof-of-work, progressive delays or IP bans.
- **Other login-flow changes.** The per-account lockout and other login-flow behavior
  are unchanged. Additional login-flow hardening items are tracked privately.
- **Routes not throttled.** This PR doesn't throttle:
  - `POST /setup`, `GET|POST /invite/*`, `POST /security/events`;
  - the OAuth callback;
  - `/api/v1/emergency/*` or the Tier-2 emergency server (§3.10).
- **No tuning UI.** No UI for editing budgets, and no admin "unblock client" action.
  The card is read-only.
- **No throttle notifications.** Security notifications for throttle events are deferred (§9).
- **No forward-auth implementation.** This PR only corrects the docs that advertise it (§4.5).
- **No default trust of loopback in the container image.** See maintainer decision M1 (§8).
- **No spec-reference sweep.** The remaining `docs/plans/current_spec.md` comment
  references are left for a separate sweep (§9).

### 1.4 Requirements (EARS)

| ID | Type | Requirement |
| --- | --- | --- |
| R1 | Ubiquitous | Whenever the auth throttle is enabled, the system shall apply a per-client token-bucket throttle, independent of all Cerberus settings, to: every `/api/v1/auth/*` route classified `login` or `session` (§3.2), and the start of every account-password verification outside that group (§3.2.1). |
| R2 | Event | When a client's `login` budget is exhausted, the system shall respond `429` before any password verification or account bookkeeping runs. |
| R3 | Event | When any throttle in the process rejects a request, the system shall respond `429` with `Retry-After: <integer seconds ≥ 1>` and body `{"error":"Too many requests. Please wait before trying again."}`. The response shall echo no request data. |
| R4 | State | While a client's `login` budget is exhausted, that client's requests to exempt routes, and every other client's requests, shall be unaffected. |
| R5 | Unwanted | If the TCP peer is not in the effective trusted-proxy list, then `X-Forwarded-For`/`X-Real-IP` shall not influence the key. If such a header is present, the system shall record the observation (count, last-seen time, peer address, peer scope) and emit a rate-limited WARN. The guidance shall never suggest trusting a public address. |
| R6 | Event | When the TCP peer is a trusted proxy, the key shall be the rightmost untrusted address in `X-Forwarded-For` (Gin semantics). |
| R7 | Ubiquitous | IPv4 clients shall be keyed per address, IPv6 clients per /64. An empty or unparsable client IP (including zoned IPv6 peers) shall map to one shared fail-closed key. |
| R8 | Unwanted | If a request carries a validated emergency bypass, then no throttle shall consume or reject it. `/api/v1/emergency/*` and the Tier-2 server shall never be throttled. |
| R9 | Ubiquitous | Each limiter instance shall track at most `MaxKeys` clients (default 10,000) and shall start no goroutines or timers. |
| R10 | Unwanted | If a throttle setting is malformed or out of bounds, then the secure default shall be used and a startup WARN logged. Only `CHARON_AUTH_RATELIMIT_ENABLED=false` (any case) disables the throttle; any other non-empty value other than `true` keeps it on and logs a WARN. |
| R11 | Event | When the frontend receives a `429`, it shall show a localized wait message derived from `Retry-After`. On the login page it shall also show a pointer for administrators, linking to the trusted-proxy documentation. |
| R12 | Ubiquitous | Throttle logs shall contain no usernames, emails, passwords, tokens or bodies. Client identifiers shall be sanitized. Throttle WARN volume shall be globally capped, with a suppressed-count summary. |
| R13 | Unwanted | If a request lacks an `OptionalAuth`-validated admin identity, then the Cerberus limiter shall not exempt it, whatever `Authorization` header it carries. |
| R14 | Ubiquitous | Peer addresses and forwarded-header observations shall be exposed only through an admin-only endpoint, and shown only to admins, on a Security-dashboard card. |
| R15 | Ubiquitous | `CHARON_TRUSTED_PROXIES` shall be validated once at startup with Gin's parsing rules. Any invalid entry shall yield an empty effective list plus a WARN. Gin, the cookie logic, the throttle and the status endpoint shall all use that one effective list, and the cookie logic and the detector shall match peers with the same `netip` prefix matcher (no IPv4/IPv6 loopback equivalence). |

---

## 2. Research Findings

### 2.1 `/auth/*` route inventory and real callers (verified)

Today the routes are registered individually, not as a group. The public routes are at
`backend/internal/api/routes/routes.go:319-323`. The protected routes are on the
`protected` group at `routes.go:367-376`.

| Route | Auth | Handler work | Real callers | Frequency |
| --- | --- | --- | --- | --- |
| `POST /auth/login` | public | bind JSON → password verification (bcrypt) → per-account lockout bookkeeping → JWT (`auth_handler.go:219-236`) | `pages/Login.tsx:53`; `pages/Setup.tsx:63` (post-setup auto-login); E2E fixtures (`auth.setup.ts`, `auth-fixtures.ts`, `TestDataManager.createUser`) | human: per submit; E2E: every fixture user logs in, plus 362 `loginUser(` call sites |
| `POST /auth/change-password` | protected | verifies the current password (bcrypt), then hashes the new one (`auth_service.go:121-136`) | `AuthContext.changePassword` ← `UsersPage.tsx:575/636` | rare |
| `POST /auth/refresh` | protected | DB read + JWT sign + Set-Cookie | no frontend caller (only listed in `client.ts:61-64`); E2E `refreshTokenIfNeeded` | rare |
| `GET /auth/status` | public | JWT validation + DB lookup; returns user info | no frontend caller; E2E `tests/utils/health-check.ts:324`, `tests/fixtures/token-refresh-validation.spec.ts:90` | ~1 per E2E run |
| `GET /auth/verify` | public | JWT validation; optional per-host permission check | nothing emits Caddy `forward_auth` (no reference in `backend/internal/caddy/`; `forward_auth_enabled` is only a TS field, `api/proxyHosts.ts:36`); E2E `security-enforcement/emergency-reset.spec.ts:232` | none today; per proxied request if forward-auth were ever wired |
| `GET /auth/me` | protected | DB read | `AuthContext.tsx`: every full page load (`checkAuth`, :42-60), **immediately after login** (`login()`, :79-99), `refetchUser` | every page load |
| `POST /auth/logout` | protected | invalidates sessions (DB write) + clears cookie | `AuthContext.logout` (manual and 15-min idle auto-logout) | rare |
| `GET /auth/accessible-hosts` | protected | DB reads | none found | none |
| `GET /auth/check-host/:hostId` | protected | DB reads | none found | none |

Key consequences:

- **Don't throttle `/auth/me`.** `AuthContext.login()` calls it right after a successful
  login and treats any error as failure (`AuthContext.tsx:91-99`), so a 429 there would
  turn a successful login into a failed one.
- **Don't throttle `/auth/logout`.** A throttled logout would leave the session valid.
- **The docs advertise a feature that doesn't exist.** `docs/features/security.md:25-27`
  and `docs/features.md:69` describe a "Require Login" gateway that is not currently
  available. C10 corrects the docs.

### 2.2 Middleware chain today (for `/api/v1/auth/login`)

1. `gin.Default()` Logger + Recovery — `internal/server/server.go:19`.
2. `RequestID`, `RequestLogger`, `Recovery(debug)` — `cmd/api/main.go:270-274`.
3. `EmergencyBypass` (sets `emergency_bypass`), `gzip`, `SecurityHeaders` — `routes.go:121-131`.
4. `api` group: `OptionalAuth` → `cerb.RateLimitMiddleware()` (opt-in) → `cerb.Middleware()`
   (ACL, WAF metrics, CrowdSec tracking) — `routes.go:248-255`.
5. Protected routes only: `AuthMiddleware` — `routes.go:367-368`.
6. Handler.

`/api/v1/emergency/*` is registered on `router`, outside the `api` group
(`routes.go:229-241`), so no limiter reaches it. `/api/v1/health` gets its own
`cerb.RateLimitMiddleware()` instance (`routes.go:220`).

### 2.3 Cerberus limiter weaknesses (verified; all fixed in C5)

| ID | Weakness | Evidence | Impact |
| --- | --- | --- | --- |
| 3a | The per-IP map is unbounded between 10-minute sweeps; each sweep is O(n) under the single mutex | `cerberus/rate_limit.go:46-80` | Memory grows under key rotation (each IPv6 address in one /64 is a distinct key) |
| 3b | `newRateLimitManager` starts `cleanupLoop` with no stop | `rate_limit.go:52-68` | Leaks goroutines: 2 per `RegisterWithDeps` (health + api), 1 per `NewRateLimitMiddleware`. `routes_test.go` builds 56 routers. Conflicts with "Long-running work must respect `server.Run(ctx)`" |
| 3c | `isAdminSecurityControlPlaneRequest` exempts **any** request on `/api/v1/security/`, `/api/v1/settings*` or `/api/v1/config*` that carries an `Authorization: Bearer …` header, whatever the token's validity or role. `OptionalAuth` validates tokens from the header, the `auth_token` cookie, or the deprecated `?token=` query (`middleware/auth.go:46-76`), and sets `role` only on success. The bearer-prefix fallback therefore adds exemptions only for invalid tokens and for valid **non-admin** tokens. The prefix match is not segment-aware. Added by `bd1a1a53`; encoded in tests `rate_limit_test.go:455`, `:547` | `rate_limit.go:18-43` | Unvalidated or non-admin callers bypass the (opt-in) API limiter on those prefixes |
| 3d | A `429` carries no `Retry-After`, and a WARN is logged per denied request | `rate_limit.go:127-131, 204-208` | Poor client UX; attacker-driven log volume |
| 3e | IPv6 is keyed per full address | `rate_limit.go:124, 201` | Rotation within a /64 bypasses the limit |
| 3f | `NewRateLimitMiddleware` is exported but has no production caller | repo-wide grep | Dead code |
| 3g | Unchecked `bypass.(bool)` type assertions | `rate_limit.go:114, 143`; `cerberus.go:155` | Would panic if the flag were ever set to a non-bool |

The Cerberus limiter is off by default, for two reasons:

- `RateLimitMode` defaults to `"disabled"` (`config.go:178`).
- Its runtime override is the `security.rate_limit.enabled` setting (`rate_limit.go:153-157`).
  That setting is absent on fresh installs, and the emergency reset writes it as `false`
  (`emergency_handler.go:229`).

**Net effect: login has no per-client throttle by default.**

### 2.4 Client-IP resolution, proxy trust, and deployment topology (verified)

#### 2.4.1 Gin (`gin@v1.12.0`)

- `server.NewRouter` (`internal/server/server.go:18-32`) calls
  `SetTrustedProxies(cfg.Security.TrustedProxies)`, sourced from `CHARON_TRUSTED_PROXIES`
  (`config.go:198-207`). If the list is empty, or Gin rejects it, it calls
  `SetTrustedProxies(nil)` and trusts nothing.
- `Context.ClientIP()` (`gin/context.go:975-1024`) resolves the client per peer:
  - **Untrusted peer:** returns the `RemoteAddr` host. `X-Forwarded-For`/`X-Real-IP` are ignored.
  - **Trusted peer:** `validateHeader` (`gin/gin.go:482-501`) walks `X-Forwarded-For`
    right-to-left and returns the first untrusted address, then falls back to `X-Real-IP`.
    A malformed entry stops that walk; Gin then tries `X-Real-IP`, and only then falls back
    to the peer address.
  - **Unparsable `RemoteAddr`:** returns `""`. This includes zoned IPv6 peers, since
    `net.ParseIP` rejects zones.
- `Forwarded` (RFC 7239) is never consulted.
- Gin's own trust-all check (`isUnsafeTrustedProxies`, `gin.go:456-459`) flags any list
  whose CIDRs contain `0.0.0.0` or `::`.

#### 2.4.2 Inconsistent trust today (fixed by R15, §3.7)

- Gin's `prepareTrustedCIDRs` (`gin.go:414-441`) stops at the first invalid entry: it keeps
  the partial list parsed so far and returns an error. `server.go:23-32` is what resets
  the list to trust nothing when that error is returned.
- The cookie logic (`isTrustedPeer` → `security.IsIPInCIDRList`, `security/whitelist.go:12`)
  skips only the invalid entry. That behavior is locked in by `TestIsTrustedPeer`
  (`auth_handler_test.go:325-372`).
- `security.IsIPInCIDRList` also treats IPv4 and IPv6 loopback as interchangeable
  (`whitelist.go:17, 30-37, 53-57`); Gin does not. §3.7 removes this difference for proxy trust.
- One malformed entry therefore makes the two disagree.

#### 2.4.3 Other resolution facts

- `util.CanonicalizeIPForSecurity` (`internal/util/sanitize.go:29-58`) maps IPv4-mapped
  addresses to IPv4 and IPv6 loopback to `127.0.0.1`. It does no prefix aggregation.
- No user-facing doc explains `CHARON_TRUSTED_PROXIES` for the management port;
  `docs/security.md:496` only says "Configure trusted proxies correctly". Its design
  lived in a §13 addendum of the Async Backup/Restore spec (commits `3b1cd2bb`,
  `031c1416`), which was never archived.

#### 2.4.4 Deployment topology

What the management API sees as the TCP peer:

- **Charon's own Caddy does not front the admin UI by default.** Its catch-all route
  rewrites unmatched requests to `/unknown.html` (`internal/caddy/config.go:683-693`).
- **Self-proxy exists only if a user adds a proxy host whose upstream is Charon itself.**
  - A `localhost:8080` upstream arrives from `127.0.0.1`.
  - A `charon:8080` (container-name) upstream arrives from the container's own bridge address.
- **External nginx.** The documented external nginx example proxies to `charon:8080` and
  arrives from its Docker-network address. It appends to `X-Forwarded-For` via
  `$proxy_add_x_forwarded_for` (`docs/troubleshooting/websocket.md:46-54`).
- **Runtime NAT.** Some container runtimes and load balancers hide client addresses
  *without* adding any forwarded header, so every client arrives from one internal
  address. The runtime specifics below come from upstream docs read on 2026-09-24 and are
  **unverified for this spec** (versions and defaults change). C10 must link the upstream
  pages with version caveats, and docs-writer must re-verify them before publishing.
  - **Rootless Docker:** "Port forwarding with `docker run -p` does not propagate source IP
    addresses by default." Fixes (Docker "Rootless mode → Troubleshooting"):
    - With RootlessKit ≥ v3.0, set `"userland-proxy": false` in
      `~/.config/docker/daemon.json` and load `br_netfilter`.
    - On older versions, use the `slirp4netns` port driver, or `pasta` with the `implicit` port driver.
  - **RootlessKit port drivers:** `builtin` propagates the source address "for TCP (since v3.0)".
    `slirp4netns` and `implicit` (pasta) propagate it. `gvisor-tap-vsock` does not
    (rootlesskit `docs/port.md`).
  - **Rootless Podman:**
    - The pasta network mode, and slirp4netns with `port_handler=slirp4netns`, preserve the source address.
    - `port_handler=rootlesskit` and the default `rootlessport` forwarder of rootless bridge
      networks do not (Podman networking docs).
  - **Other cases:** VM-based desktop runtimes and source-NAT load balancers can behave the
    same way, depending on version and configuration.

### 2.5 Account lockout (the second layer; unchanged)

- A per-account lockout (5 failed attempts lock the account for 15 minutes,
  `auth_service.go:70-102`) remains the second layer, behind the new per-client budget.
- Additional login-flow hardening items are tracked privately.

### 2.6 Break-glass precedent

- Commit `89644a45` removed the emergency endpoint's own limiter. `routes.go:232` says:
  "Emergency endpoints must stay responsive and should not be rate limited."
- The Tier-2 server (`internal/server/emergency_server.go:96`) is a separate `gin.New()`
  with no limiter and no `/auth` routes.
- Two docs still claim Tier-1 rate limiting and are stale:
  - `docs/security.md:491`;
  - `docs/runbooks/emergency-lockout-recovery.md:49-68`.

### 2.7 E2E environment facts

- **Project routing.** The `chromium`/`firefox`/`webkit` projects `testIgnore`
  `**/security-enforcement/**` and `**/tests/security/**` (`playwright.config.js:283-298`).
  Specs there are collected only by the serial, Chromium-based `security-tests` project.
  **The new spec therefore lives in `tests/core/`.**
- **PR E2E jobs.** Two jobs run on PRs:
  - "E2E Firefox Security" (`--project=security-tests`, `e2e-tests-split.yml:529-535`);
  - the "E2E Firefox" non-security shards (`:1372-1378`), over `tests/a11y`, `tests/core`,
    `tests/integration`, `tests/monitoring`, `tests/settings`, `tests/tasks` and three DNS specs.
- **Workers and peer address.** CI uses `workers: 1` per shard; local runs use default workers
  (parallel). All runner traffic reaches the container from one peer address:
  - the Docker bridge gateway;
  - the rootless port-driver address;
  - or, for the Tailscale remote-runner flow (`playwright.config.js:177-178`), a
    `100.64.0.0/10` address.
- **Login volume.** Every `adminUser` / `regularUser` / `authenticatedUser` fixture creates
  and logs in a user (`auth-fixtures.ts:358-430` → `TestDataManager.createUser`). Many
  tests call `loginUser` again. That is far beyond any human-sized budget.
- **Silent login failures.** `TestDataManager.createUser` swallows login failures: on a
  non-OK login it returns `token: ''` (`tests/utils/TestDataManager.ts:638-642`). A drained
  bucket would therefore surface as unrelated downstream flakes. §4.1 makes fixtures
  fail loudly on a persistent 429.
- **Existing coverage.**
  - No existing spec triggers a real management-API `429`; the precedent specs only verify configuration.
  - The security project already runs near the Cerberus limiter's margin
    (`multi-component-security-workflows.spec.ts:223-235`).
- **No forwarded headers in existing specs.** No spec sends `X-Forwarded-For`, `X-Real-IP`,
  `X-Forwarded-Proto` or `X-Forwarded-Host` to the management API.
  `tests/fixtures/proxy-hosts.ts:130-131` are proxy-host configuration values, not request headers.
- **`CHARON_ENV=e2e`.** The local compose file (`docker-compose.playwright-local.yml:34`)
  claims it enables "lenient rate limiting (50 attempts/min)", which is stale. Its
  e2e-specific effect is `internal/caddy/config.go:96` (ACME/TLS policies without an ACME
  email). Like any non-production value, it also enables the `CHARON_CHANGELOG_VERSION`
  override (`routes.go:385`).
- **Integration tests.**
  - `scripts/*_integration.sh` (run on PRs) each log in once.
  - `backend/integration` (build tag `integration`, not run by any workflow) logs in 8×
    per container lifetime, which is under the default burst of 10.

### 2.8 Frontend facts

- No `429`/`Retry-After` handling exists anywhere in `frontend/src`.
- The Axios interceptor (`api/client.ts:42-71`) copies `data.error` into `error.message`.
  `Login.tsx:62` toasts `response.data.error || error.message`.
- Toasts are plain text (`utils/toast.ts`), so they can't carry links. They render
  `data-testid="toast-error"` (`components/Toast.tsx:32`). The app doesn't use `<Trans>` anywhere.
- **Docs links** are hardcoded and inconsistent: `FeedbackWidget.tsx:11` uses
  `https://wikid82.github.io/Charon/`, while others use `…/charon/security`. The docs site
  publishes at `url` `https://wikid82.github.io` + `baseUrl` `/Charon/` + `routeBasePath`
  `docs` (`docs-site/docusaurus.config.*:18-67`).
- **Security page gating.** The page shows the Cerberus header card for everyone, and hides
  module cards unless Cerberus is enabled (`pages/Security.tsx:331-376`).
- **Admin checks** use `user?.role === 'admin'` (e.g. `components/Layout.tsx:131-158`).
- **i18n.** `react-i18next` with 5 locales (en, de, es, fr, zh); es informal, de formal,
  fr *Veuillez*. Plural keys carry `_one`, `_other` **and** a bare fallback key in every
  locale, including zh (`zh/translation.json:174-176`).

### 2.9 Documentation and comment drift found

| Location | Drift | Action in this PR |
| --- | --- | --- |
| `ARCHITECTURE.md:986` | "NO Cerberus Middleware … on management interface", but `routes.go:252-255` applies both Cerberus middlewares to `/api/v1` | Correct (C10) |
| `ARCHITECTURE.md:819-828` | Layer 1 "sliding window, 100 req/min, 1000 req/hour, admin whitelist" | Correct (C10) |
| `ARCHITECTURE.md:913` | bcrypt "cost factor 12" (the code uses `bcrypt.DefaultCost`, `models/user.go:78`) | Correct (C10) |
| `docs/security.md:491`, runbook `:49-68` | Tier-1 "rate limiting" (removed in `89644a45`) | Correct (C10) |
| `docs/api.md:16-24`, `:1761-1768` | "Authentication not yet implemented", "Rate limiting not yet implemented" | Correct (C10) |
| `docs/features/security.md:25-27`, `docs/features.md:69` | Advertise a "Require Login" gateway that is not currently available | Correct (C10) |
| `docker-compose.playwright-local.yml:34` | Stale lenient-rate-limit comment | Correct (C7) |
| `config.go:57-65`, `server.go:15-17`, `auth_handler_test.go:325-329` | Cite `current_spec.md §13` (never archived) | Repoint to `docs/configuration/trusted-proxies.md` (C10) |
| 21 comments citing the Redirection Hosts spec (list in C1) | Broken by **this PR's** archive step | Repoint to the archive path (C1) |
| `docs/plans/current_spec.md` references across code/tests | 156 today; 132 remain after C1 and C10 | §9 sweep, plus a new convention: cite issue numbers, durable docs, or archive paths |
| `internal/metrics/security_metrics.go` | Registers via `promauto` on the default registry, while `/metrics` serves a custom registry (`routes.go:223-227`) | §9 |

### 2.10 Library evaluation (Decision C)

Versions and dates come from `proxy.golang.org/<module>/@latest`. Licenses and `go.mod`
files come from the upstream repos or the module cache.

| Option | Version (date) | License | New modules | Algorithm | Hard memory cap | Background goroutine | Injectable clock | Verdict |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `golang.org/x/time/rate` | v0.16.0 (2026-08-19), **already a direct dep** | BSD-3 | 0 | token bucket | n/a (keyed store is ours) | none | **yes**: `AllowN(t,n)`, `TokensAt(t)` | ✅ algorithm |
| `hashicorp/golang-lru/v2` → `simplelru` | v2.0.7 (2023-09-21); in `go.sum`, not in today's build graph | MPL-2.0 (already shipped via `hashicorp/yamux` v0.1.2) | 1 (zero requires of its own) | fixed-size LRU, not thread-safe (we hold the lock) | **yes** | none | n/a | ✅ store |
| `golang-lru/v2` → `expirable` | v2.0.7 | MPL-2.0 | 1 | LRU+TTL | yes | **yes, never exits** (its source: "done channel is never closed, so deleteExpired() goroutine will never exit") | no | ❌ repeats 3b |
| `go-chi/httprate` | v0.16.0 (2026-06-29) | MIT | `zeebo/xxh3`, `klauspost/cpuid/v2` | sliding-window counter | no (unbounded within a window) | none | no (`time.Now()` in `limiter.go`) | ❌ (borrow its IPv6 /64 rationale) |
| `sethvargo/go-limiter` | v1.2.0 (2026-07-17) | Apache-2.0 | 0 | token bucket | no (`sync.Map`, swept) | sweeper, stoppable via `Close(ctx)` | no (internal `fasttime`) | ❌ |
| `ulule/limiter/v3` | v3.11.2 (2023-05-24) | MIT | redis, fasthttp, gin 1.9.0, … | fixed window | cleanup interval | yes | no | ❌ stale, heavy |
| `throttled/throttled/v2` | v2.15.0 (2025-08-23) | BSD | go-redis ×2, redigo, golang-lru v0.5.4 | GCRA | LRU | — | partial | ❌ heavy |

**Decision: `x/time/rate` (algorithm) + `golang-lru/v2/simplelru` (bounded store) in a
small `internal/ratelimit` package.**

- The design relies on two facts from `x/time@v0.16.0`:
  - `AllowN` consumes tokens only on success.
  - `Reservation.CancelAt` cannot refund a reservation whose act-time has passed
    (`rate.go:169-181`).
- `backend/.golangci.yml` and `.golangci-fast.yml` have no `depguard`/`gomodguard` rules
  that could block the new module (verified).
- Fallback if a new module is rejected: `container/list` plus a map (~50 lines). See Q2.

### 2.11 Precedent for budgets

Vaultwarden's shipped `.env.template` defaults are `LOGIN_RATELIMIT_SECONDS=60` and
`LOGIN_RATELIMIT_MAX_BURST=10`. That is a per-IP GCRA, and its budget is shared by login
and 2FA. The login budget below matches it exactly.

### 2.12 Password-verifying routes outside `/auth`

There are exactly four production call sites of `models.User.CheckPassword` (repo-wide
grep):

| Call site | Route | When it verifies |
| --- | --- | --- |
| `AuthService.Login` (`auth_service.go:85`) | `POST /api/v1/auth/login` | always |
| `AuthService.ChangePassword` (`auth_service.go:127`) | `POST /api/v1/auth/change-password` | always |
| `UserHandler.UpdateProfile` (`user_handler.go:332`) | `POST /api/v1/user/profile` (`routes.go:378`, any authenticated user) | only when the email changes (`user_handler.go:326-336`) |
| `CertificateHandler.Export` (`certificate_handler.go:366`) | `POST /api/v1/certificates/:uuid/export` (`routes.go:1020`, admin-only) | only when `include_key` is true (`certificate_handler.go:336-369`) |

Private-key export currently answers `403` before reaching verification: the handler reads
a context value that the auth middleware never sets (`certificate_handler.go:342`). That
is a separate functional bug, listed in §9.

---

## 3. Technical Specifications

### 3.1 Decision summary

| # | Decision | Summary |
| --- | --- | --- |
| A | Route coverage | Explicit `api.Group("/auth")` with one group middleware and one classification table. **`login` class** (one shared bucket per client): login, change-password, and the password branches of profile update and key export (in-handler guard). **`session` class**: refresh and status. **Exempt**: verify, me, logout, accessible-hosts, check-host. Future `/auth` routes default to `session`; a two-way inventory test and a password-check tripwire fail until new routes are classified. §3.2 |
| B | Relationship to Cerberus | One shared `ratelimit.KeyedLimiter`. The Cerberus limiter migrates to it, and every weakness in §2.3 is fixed in one hardening commit (C5). The auth throttle is always on, independent of Cerberus. Both run in sequence (§3.9) |
| C | Algorithm/library | Token bucket via `x/time/rate` + `simplelru`. This deliberately deviates from the issue's "sliding window" wording; §3.3 gives the equivalence |
| D | Keying | `ratelimit.ClientKey(c.ClientIP())`: IPv4 per address, IPv6 /64, otherwise a fail-closed `unknown`. Gin semantics apply to trusted and untrusted peers. §3.4 |
| E | Shared-address failure modes | Two cases: **untrusted proxy** (detectable) and **runtime NAT** (undetectable from headers). The severity is stated plainly. Mitigations: detector, admin card with self-check, docs, kill switch, short `Retry-After`. Device cookies are the first follow-up (M2). §3.5 |
| F | Configuration | Env vars only, via `internal/config`. The struct's zero value is secure. Values are bounded, invalid values fall back to the default with a WARN, and an unrecognized `ENABLED` value logs a WARN. E2E stacks set explicit test-only values, and the spec self-guards with an effectiveness probe. §3.6 |
| G | Contract/observability | `429` + `Retry-After` + one generic message for both limiters. **No** `RateLimit-*` headers. Capped per-episode WARNs, and `charon_auth_rate_limited_total{class}`. No audit rows. Notifications deferred. §3.8 |
| H | Ordering | … → `OptionalAuth` → Cerberus limiter → `cerb.Middleware()` (ACL) → **auth throttle** → `AuthMiddleware` → handler. Emergency bypass skips it, via a shared exported helper. §3.9 |
| I | Other credential-bearing routes | `/setup` and `/invite/*` are out of scope (§3.10). Password-verifying routes outside `/auth` are **in** scope (A) |
| J | Tests | §4.4 matrix: fake-clock units, `-race`, memory cap, route inventory, group-ordering guard, password tripwire, trusted-proxy tests, metric deltas, Vitest, Playwright (`fixme` first). Coverage gate is ≥ 87% |
| K | Docs | Two new durable docs, a compose hint, the forward-auth docs correction, runtime-NAT guidance, and a process note on disclosure (§4.5) |
| L | Admin visibility | Admin-only `GET /api/v1/security/login-protection` plus a Security-dashboard **card** (recommended placement; M8). Peer addresses appear only there. Guidance is scope-aware and never suggests trusting a public peer. §3.12, §3.13 |
| M | Effective proxy trust | `CHARON_TRUSTED_PROXIES` is validated once in `config.Load` with Gin's rules. Any invalid entry yields an empty list plus a WARN, so all consumers agree. A trust-all WARN mirrors Gin's check. §3.7 |

### 3.2 Route coverage and budgets (Decision A)

**Restructure.** Replace the individual registrations at `routes.go:319-323` and
`routes.go:371-376` with one explicit group:

- `auth := api.Group("/auth")` with `auth.Use(authRateLimiter.Middleware())`.
- Public: `POST /login`, `GET /verify`, `GET /status`.
- `authSession := auth.Group("", authMiddleware)` for logout, refresh, me,
  change-password, accessible-hosts and check-host.
- **Ordering matters.** Gin copies the parent's handler chain when a child group is
  created, so `auth.Use(...)` must run **before** `auth.Group(...)`. A route test proves
  `/auth/refresh` is throttled as `session` (§4.4).
- Each moved route's chain is otherwise unchanged. The old `protected` group carried only `authMiddleware`.

| Route | Class | Rationale |
| --- | --- | --- |
| `POST /api/v1/auth/login` | `login` | Credential guessing; expensive password verification |
| `POST /api/v1/auth/change-password` | `login` (same bucket) | Verifies the account password. One bucket means every password verification from a client draws on one budget, so alternating routes doesn't raise the guess rate (Vaultwarden shares login+2FA the same way) |
| `POST /api/v1/user/profile` (email change only) | `login` (same bucket, in-handler guard) | Verifies the account password (§2.12) |
| `POST /api/v1/certificates/:uuid/export` (`include_key` only) | `login` (same bucket, in-handler guard) | Verifies the account password (§2.12) |
| `POST /api/v1/auth/refresh` | `session` | Mints tokens. The issue calls out token/refresh endpoints; this bounds token-minting floods |
| `GET /api/v1/auth/status` | `session` | Public token oracle returning user info; no UI caller |
| `GET /api/v1/auth/verify` | exempt | Designed as a forward-auth subrequest target. Per-IP limits would key every proxied request on the proxy's address. Verifies HS256 JWTs only |
| `GET /api/v1/auth/me` | exempt | SPA bootstrap and post-login fetch (§2.1) |
| `POST /api/v1/auth/logout` | exempt | Security-positive |
| `GET /api/v1/auth/accessible-hosts` | exempt | Authenticated read; no secret verified |
| `GET /api/v1/auth/check-host/:hostId` | exempt | Authenticated read |
| any future `/api/v1/auth/*` route | `session` (fail-safe) | The inventory test fails until it is explicitly classified |

| Class | Default budget | Token-bucket form | Max `Retry-After` | Long-run ceiling per client |
| --- | --- | --- | --- | --- |
| `login` | 10 requests per 600 s | burst 10, +1 token every 60 s | 60 s | ≤ 1,450/day |
| `session` | 60 requests per 60 s | burst 60, +1 token every 1 s | 1 s | 1/s sustained |

**Interplay with lockout.** The per-client `login` budget (10) is above the per-account
lockout threshold (5), so a single-account sequence still hits the lockout first, with
unchanged UX. Spreading attempts across accounts from one client is capped at 10, then
one per minute.

#### 3.2.1 Guarding password verification outside `/auth`

These routes verify a password only on some requests, so a per-route middleware would
charge unrelated requests (such as a name-only profile edit) against the login budget.
The guard therefore sits **just before each password branch**.

- **New interface** in the handlers package:
  `type PasswordAttemptGuard interface { AllowPasswordAttempt(c *gin.Context) bool }`.
  It returns `false` after writing the standard `429`.
- **Setters:** `(*UserHandler).SetPasswordAttemptGuard(g)` and
  `(*CertificateHandler).SetPasswordAttemptGuard(g)`. A nil guard allows everything, which
  keeps existing unit tests working. `routes.go` wires `authRateLimiter` into both.
- **Guard placement:**
  - `UpdateProfile`: inside `if req.Email != user.Email`, after the empty-password check and
    before `CheckPassword`.
  - `Export`: first statement inside `if req.IncludeKey`. It therefore runs before (and
    independently of) the unrelated context-key bug in §2.12.
- **Implementation:** `(*AuthRateLimiter).AllowPasswordAttempt` uses the same `login`
  limiter, key, emergency-bypass skip, disabled pass-through, logging and metric as the middleware.

### 3.3 Shared limiter component (Decisions B and C)

New leaf package `backend/internal/ratelimit/`. It imports `util`, `gin`,
`x/time/rate`, `golang-lru/v2/simplelru` and `net/netip`.

| File | Contents |
| --- | --- |
| `doc.go` | Package doc: algorithm, memory bound, no-goroutine guarantee, "sliding window" equivalence |
| `limiter.go` | `Config`, `Decision`, `KeyedLimiter`, `NewKeyedLimiter`, `MustNewKeyedLimiter`, `Allow`, `Reconfigure`, `Len`, `PerWindow` |
| `key.go` | `ClientKey`, `UnknownClientKey`, `AddrScope`, `ClassifyAddr` |
| `http.go` | `TooManyRequestsMessage`, `RetryAfterSeconds`, `Reject` |
| `*_test.go` | See §4.4 |

```go
const DefaultMaxKeys = 10_000
const UnknownClientKey = "unknown"
const TooManyRequestsMessage = "Too many requests. Please wait before trying again."

type Config struct {
    Rate    rate.Limit       // tokens/second; must be > 0 and finite
    Burst   int              // bucket capacity; must be >= 1
    MaxKeys int              // hard cap on tracked keys; 0 => DefaultMaxKeys; < 0 invalid
    Now     func() time.Time // clock; nil => time.Now (tests inject a fake)
}

type Decision struct {
    Allowed     bool
    RetryAfter  time.Duration // > 0 iff !Allowed
    FirstDenial bool          // first denial since this key was last allowed ("episode start")
}

func NewKeyedLimiter(cfg Config) (*KeyedLimiter, error)
// MustNewKeyedLimiter exists only because (*Cerberus).RateLimitMiddleware() (production) builds its
// limiter from compile-time constants and has no error return; every other caller uses NewKeyedLimiter.
func MustNewKeyedLimiter(cfg Config) *KeyedLimiter
func (k *KeyedLimiter) Allow(key string) Decision
func (k *KeyedLimiter) Reconfigure(r rate.Limit, burst int) error // no-op if unchanged; else resets all buckets
func (k *KeyedLimiter) Len() int
func PerWindow(requests int, window time.Duration) (rate.Limit, int) // (float64(N)/W.Seconds() per s, burst N)

type AddrScope string // "loopback" | "private" | "public"
func ClassifyAddr(a netip.Addr) AddrScope // private: RFC 1918, ULA fc00::/7, CGNAT 100.64.0.0/10, link-local
func ClientKey(clientIP string) string
func RetryAfterSeconds(d time.Duration) int // round to ms, ceil to s, min 1
func Reject(c *gin.Context, d Decision)     // Retry-After + AbortWithStatusJSON(429, gin.H{"error": TooManyRequestsMessage})
```

`Decision.Remaining` from revision 1 is removed: no production code would read it.

Behavior (normative):

1. **`Allow(key)` runs under one mutex.**
   1. Run the amortized sweep (step 4).
   2. `lru.Get(key)`, or create `rate.NewLimiter(Rate, Burst)` and `lru.Add` it.
      `Add` evicts the least-recently-used key when full.
   3. Set `entry.lastSeen = now`.
   4. Call `lim.AllowN(now, 1)`.
2. **Denials don't consume.** A denied request consumes nothing, so a client that retries
   early never pushes its own wait further out.
3. **`RetryAfter`** is `(1 − lim.TokensAt(now)) / Rate`. `RetryAfterSeconds` rounds it to
   milliseconds before taking the ceiling, so 10 per 600 s yields exactly `60`. The minimum is 1.
4. **Sweep.** At most once per `min(idleTTL, 1m)`, walk from the LRU tail and remove entries
   idle ≥ `idleTTL = Burst / Rate` (the full-refill time).
   - A fully refilled bucket is indistinguishable from a new one, so the sweep is lossless.
   - Invariant: LRU order equals `lastSeen` order.
5. **`FirstDenial`.** Each entry carries a `denied` flag. A denial sets it; the next allowed request clears it.
6. **No goroutines, tickers or timers.** Nothing needs shutting down.
7. **Memory** is about 250 B per tracked client:
   - ≤ ~2.5 MB per limiter at `DefaultMaxKeys`;
   - ≤ ~10 MB for the four instances together (auth login, auth session, Cerberus API,
     Cerberus health) under a large distributed flood.
8. **Eviction.** An attacker would need more than 10,000 distinct IPv4 addresses or IPv6 /64s
   to evict its own depleted bucket. At that scale it already has a fresh bucket per address,
   so eviction adds no capability.

**Why a token bucket instead of "sliding window."** With burst N and refill N/W, any
interval of length T admits at most `N + T·N/W` requests, so there is no fixed-window
boundary doubling. Beyond that, the token bucket:

- keeps constant state per client;
- gives an exact `Retry-After`;
- is already used by the codebase and by Vaultwarden.

### 3.4 Keying and client-IP resolution (Decision D)

`ClientKey(ip)` works as follows:

1. Canonicalize with `util.CanonicalizeIPForSecurity`.
2. Parse with `netip.ParseAddr`, then `Unmap()`.
3. For IPv4, return the address string.
4. For IPv6, return `netip.PrefixFrom(addr.WithZone(""), 64).Masked().String()`.
5. For empty or unparsable input, return `UnknownClientKey`.

Note that Gin's `ClientIP()` already returns `""` for zoned IPv6 peers (§2.4), so such peers
share `unknown`. Link-local peers are only possible on the same L2 segment; this is documented.

- **IPv6 /64.** httprate v0.16.0's docs give the rationale: an IPv6 client typically controls
  a whole /64. The prefix is not configurable (§9).
- **IPv4** is keyed per address.
- **Trust semantics** follow §2.4:
  - untrusted peers' forwarded headers never affect the key;
  - behind a trusted proxy, the key is the rightmost untrusted XFF hop.

### 3.5 Shared-address failure modes (Decision E)

A per-client budget is only as good as the client address. There are two ways many
clients can collapse into one key.

#### 3.5.1 Untrusted proxy in front of the UI (detectable)

**Problem.** The UI is reached through a reverse proxy (an external nginx/Traefik/load
balancer, or a user-created self-proxy host, §2.4), but `CHARON_TRUSTED_PROXIES` doesn't
list it. Every client then resolves to the proxy's address.

**Detector.** In the auth middleware, and in `AllowPasswordAttempt`, for every request
(including exempt routes, since `/auth/me` fires on each page load):

- **Condition:** `X-Forwarded-For` or `X-Real-IP` is present **and** the peer is **not**
  contained in the effective trusted prefixes.
  - The peer is `netip.ParseAddr(c.RemoteIP())`, zone stripped, `Unmap()`.
  - The effective prefixes are the same validated list Gin uses (§3.7). Comparisons use
    parsed `netip` values, never strings.
- **Trusted peer, malformed header.** A trusted peer that sends a malformed forwarded
  header is **not** counted; it is logged at DEBUG. (Gin tries `X-Real-IP` next and only
  then falls back to the peer address, per §2.4.1.)
- **Recorded state, per scope** (mutex-protected, process lifetime). Observations are
  classified with `ratelimit.ClassifyAddr` into two independent records: **local**
  (loopback or private peers) and **public**. Each record keeps its own:
  - `count` (uint64);
  - `last_seen` (time);
  - `last_peer` (address; for local, also whether it was loopback or private);
  - WARN limiter.

  Public noise therefore never overwrites or masks the local record that signals a
  misconfigured proxy.
- **WARN** at most once per 15 minutes **per scope** (a `rate.Limiter` with burst 1 on the
  injected clock). The text is **scope-aware**:
  - **loopback/private peer:** "Sign-in requests carry forwarded client-address headers
    from `<peer>`, which is not a trusted proxy, so every visitor behind it shares one sign-in
    budget. If `<peer>` is your reverse proxy, add `<peer>/32` (IPv4) or `<peer>/128` (IPv6)
    to `CHARON_TRUSTED_PROXIES`. See docs/configuration/trusted-proxies.md."
  - **public peer:** "Ignored forwarded client-address headers sent from a public address
    that is not a trusted proxy. This is usually a client sending forged headers and needs
    no action. See docs/configuration/trusted-proxies.md."
  - For a public peer, the text **never** suggests trusting the address, because anyone can
    trigger it by sending the header directly.

#### 3.5.2 Runtime NAT (not detectable from headers)

**Problem.** Some runtimes and load balancers hide the client's source address *without*
adding a forwarded header (§2.4). Every client arrives from one internal address, for example:

- rootless Docker without source-IP propagation;
- rootless Podman's `rootlesskit`/`rootlessport` forwarders;
- some VM-based desktop runtimes;
- source-NAT load balancers.

In these setups:

- the detector sees nothing;
- `CHARON_TRUSTED_PROXIES` cannot help, because there is no header to trust;
- the `login` budget becomes one global bucket.

**Remedies** (documented in `login-protection.md` and the troubleshooting entry):

1. Switch the runtime to a mode that preserves client addresses:
   - rootless Docker with RootlessKit ≥ v3.0: `"userland-proxy": false` plus `br_netfilter`;
   - older rootless Docker: the `slirp4netns` port driver, or `pasta` with `implicit`;
   - Podman: pasta mode, or `port_handler=slirp4netns`.
2. Put a host-level reverse proxy in front of the UI. It sees real client addresses and
   appends them to `X-Forwarded-For`. Then list that proxy in `CHARON_TRUSTED_PROXIES`.
3. As a last resort, set `CHARON_AUTH_RATELIMIT_ENABLED=false` (logged as a WARN).

**Self-check.** The admin card (§3.13.3) shows "Charon sees your browser as `<address>`
(`<scope>`)". If that isn't the device's real address, the deployment is in this mode or
behind an untrusted proxy.

#### 3.5.3 Severity (stated plainly)

When all clients share one key, one client can keep sign-in closed for **every** account at
once. It needs no account name, and about one request per minute (after an initial burst of
10) sustains the lockout. That makes the failure mode **strictly broader** than any
per-account control. The risk table rates it High (§7, RK1 and RK2).

#### 3.5.4 Mitigations adopted

1. The detector (§3.5.1), with admin-only visibility (§3.12, §3.13.3) and capped WARNs.
2. A human-scale budget with a short refill (`Retry-After` ≤ 60 s): innocent users recover
   within a minute once the traffic stops.
3. Documentation:
   - a new `docs/configuration/trusted-proxies.md`;
   - runtime-NAT guidance in `docs/features/login-protection.md`;
   - a troubleshooting entry covering both cases;
   - a commented `CHARON_TRUSTED_PROXIES` hint in the user-facing compose file.
4. The login-page 429 notice includes a generic pointer for administrators (§3.13.2).
5. Escape hatches:
   - fix the proxy trust list or the runtime mode (the root cause);
   - restart, which clears the in-memory buckets;
   - the Tier-1 emergency token and Tier-2 server, which are never throttled;
   - the kill switch.
6. The first follow-up (M2): per-browser device cookies alongside per-IP buckets. This
   directly addresses shared-address setups.

#### 3.5.5 Rejected alternatives

- **Count only failed attempts.** `x/time/rate` cannot refund an acted reservation
  (§2.10). A peek-then-consume design lets concurrent requests pass one token's check.
  It also doesn't remove the shared-key effect, because the pre-verification gate still
  blocks everyone behind the key.
- **An IP+username dimension.** It doesn't reduce the per-IP lever. It requires reading and
  restoring the JSON body in middleware. The per-account lockout already covers the
  per-username dimension.

### 3.6 Configuration (Decision F)

**Env vars only (no DB setting, no UI editing), for these reasons:**

1. Secure-by-default with zero config.
2. Login-protection parameters are infrastructure policy; changing them should require host
   access, not only an admin session.
3. No per-request settings read on the auth path.
4. It stays coupled to `CHARON_TRUSTED_PROXIES`, which is also env-only.
5. Scope.

| Env var | Default | Bounds | Invalid value → |
| --- | --- | --- | --- |
| `CHARON_AUTH_RATELIMIT_ENABLED` | `true` | `true` / `false` (case-insensitive) | any other non-empty value: **stays enabled** + startup WARN ("unrecognized value") |
| `CHARON_AUTH_RATELIMIT_LOGIN_REQUESTS` | `10` | 1–10,000 | default + startup WARN |
| `CHARON_AUTH_RATELIMIT_LOGIN_WINDOW` | `600` (seconds) | 1–86,400 | default + startup WARN |
| `CHARON_AUTH_RATELIMIT_SESSION_REQUESTS` | `60` | 1–10,000 | default + startup WARN |
| `CHARON_AUTH_RATELIMIT_SESSION_WINDOW` | `60` (seconds) | 1–86,400 | default + startup WARN |

Naming follows `CHARON_SECURITY_RATELIMIT_{REQUESTS,WINDOW}` (one-word `RATELIMIT`, window
in integer seconds). There are no `CERBERUS_*` or `CPM_*` aliases.

```go
// AuthRateLimitConfig configures the always-on throttle. Its zero value is the secure
// default (enabled, default budgets), so any hand-built config.Config is protected.
type AuthRateLimitConfig struct {
    Disabled         bool // set only by CHARON_AUTH_RATELIMIT_ENABLED=false
    LoginRequests    int  // 0 = default; -1 = malformed or non-positive env value
    LoginWindowSec   int
    SessionRequests  int
    SessionWindowSec int
}

func (c AuthRateLimitConfig) Normalize() (AuthRateLimitConfig, []string) // pure: defaults + warnings
// SecurityConfig gains: AuthRateLimit AuthRateLimitConfig
// Config gains:         StartupWarnings []string
// New helper: getEnvIntStrictAny(keys ...string) int — unset => 0, valid positive int => value,
// set-but-non-integer or <= 0 => -1 (so Normalize warns instead of silently defaulting).
```

- **Constants.** Defaults `10/600/60/60`, plus the bounds above, are exported. `Normalize`
  maps `0` to the default silently, and `-1` or out-of-bounds values to the default with a
  warning that names the variable.
- **Why `Disabled`.** `routes_test.go` builds `config.Config{JWTSecret: "test-secret"}` 56
  times. An `Enabled bool` field would zero-value to off and fail open.
- **Startup warnings.** `config.Load` appends every configuration warning to
  `Config.StartupWarnings`:
  - auth-throttle normalization;
  - an unrecognized `ENABLED` value;
  - trusted-proxy validation and trust-all (§3.7).

  `cmd/api/main.go` logs each at WARN right after `logger.Init`. `RegisterWithDeps` logs one
  INFO line with the effective policy, e.g.
  `auth throttle: login 10/600s, session 60/60s per client; trusted proxies: 0`,
  and a WARN when the throttle is disabled. `middleware.NewAuthRateLimiter` also normalizes
  (idempotently) for hand-built configs.

**E2E stacks.** Add the same block to both `.docker/compose/docker-compose.playwright-ci.yml`
and `docker-compose.playwright-local.yml`:

```yaml
      # --- E2E-only auth throttle settings (issue #1317) -----------------------------------------
      # All Playwright workers share one source address and fixtures log in far more often than a
      # human, so this TEST stack relaxes the budgets. Production defaults are unchanged and pinned by
      # Go unit tests.
      - CHARON_AUTH_RATELIMIT_LOGIN_REQUESTS=300
      - CHARON_AUTH_RATELIMIT_LOGIN_WINDOW=60
      - CHARON_AUTH_RATELIMIT_SESSION_REQUESTS=600
      - CHARON_AUTH_RATELIMIT_SESSION_WINDOW=60
      # Trust the runner's peer (Docker bridge, rootless port driver, loopback, Tailscale, IPv6 ULA)
      # so tests/core/auth-rate-limit.spec.ts can give each test its own client identity via
      # X-Forwarded-For. Requirements: PLAYWRIGHT_BASE_URL must be an IP literal or localhost
      # (for trusted peers, session-cookie decisions read the Host header), and no spec may send
      # X-Forwarded-For/X-Real-IP/X-Forwarded-Proto/X-Forwarded-Host except auth-rate-limit.spec.ts.
      # NEVER trust whole private ranges in production.
      - CHARON_TRUSTED_PROXIES=127.0.0.0/8,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,100.64.0.0/10,fc00::/7
```

- There is no `CHARON_ENV`-based relaxation anywhere.
- The stale comment at `playwright-local.yml:34` is corrected to the real effects listed in §2.7.
- The spec self-guards with an effectiveness probe (§4.1).

### 3.7 Effective proxy trust (Decision M; R15)

- **New pure helper** in `internal/config`:
  `ValidateTrustedProxies(entries []string) (effective []string, warnings []string)`.
  It mirrors `gin.Engine.prepareTrustedCIDRs` (`gin.go:414-441`):
  - trim each entry;
  - a bare IP becomes `/32` (IPv4) or `/128` (IPv6);
  - otherwise use `net.ParseCIDR`.
- **All entries valid → the trimmed original strings are kept unchanged** in
  `cfg.Security.TrustedProxies` (bare IPs are *not* rewritten to CIDRs).
- **Any invalid entry → empty effective list** (trust nothing), plus a WARN naming the entry
  and saying that no proxy is trusted until it is fixed.
- **Trust-all WARN.** If any effective CIDR contains `0.0.0.0` or `::`, a WARN mirrors Gin's
  `isUnsafeTrustedProxies` (`gin.go:456-459`). The list is still used, because it is the
  operator's explicit choice.
- **Single source of truth.** Every consumer receives the same effective list:
  - Gin (`server.NewRouter`);
  - the cookie logic (`NewAuthHandlerWithDB(..., cfg.Security.TrustedProxies)`). Its
    `isTrustedPeer` switches from `security.IsIPInCIDRList` to the same `netip` prefix
    matcher the detector uses (parsed once at construction), so loopback is no longer
    treated as interchangeable across IPv4/IPv6 for proxy trust and all consumers agree;
  - the throttle's detector;
  - the admin status (`trusted_proxy_count`).
- `server.NewRouter`'s own invalid-entry fallback stays as defense in depth for hand-built configs.
- **Behavior change (documented in `trusted-proxies.md` and the release notes).**
  - Before: one invalid entry made client-address resolution trust no proxy, while the
    session-cookie HTTPS detection still honored the valid entries.
  - Now: both trust no proxy, and a startup WARN says so.
  - Also: session-cookie proxy trust no longer treats `127.0.0.1` and `::1` as
    interchangeable. An operator whose proxy connects over IPv6 loopback must list
    `::1/128` (and `127.0.0.1/32` for IPv4 loopback) explicitly.
- **Tests:**
  - `TestLoad_TrustedProxies` (`config_test.go:305-327`) is unchanged and gains one case:
    an invalid entry → `[]` + WARN in `StartupWarnings`. New `TestValidateTrustedProxies_*`
    cover bare IPv4/IPv6, CIDR, whitespace, and the trust-all warning for `0.0.0.0/0`,
    `::/0` and `0.0.0.0/1`.
  - `TestIsTrustedPeer` keeps its cases (malformed entries are still skipped defensively)
    and gains loopback cases proving `::1/128` does not trust `127.0.0.1` and vice versa,
    matching Gin.
  - `server_test.go`'s fallback test is unchanged.

### 3.8 Response contract and observability (Decision G)

The 429 contract applies to both limiters and to the password guard, via `ratelimit.Reject`:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 60
Content-Type: application/json; charset=utf-8

{"error":"Too many requests. Please wait before trying again."}
```

- One message for every class and route. It echoes no request fields.
- `Retry-After` is in integer seconds, ≥ 1 (RFC 9110 §10.2.3; RFC 6585 §4).
- **No `RateLimit-*` headers.** The IETF spec is still an Internet-Draft
  (`draft-ietf-httpapi-ratelimit-headers-11`, 23 May 2026), whose field names changed across
  revisions. Advertising the remaining login budget would help pace guesses, and the UI
  needs only `Retry-After`.

**Logs** use `middleware.GetRequestLogger(c)`:

| Event | Level | Fields | Volume control |
| --- | --- | --- | --- |
| Effective policy | INFO | budgets, trusted-proxy count | once at startup |
| Config warnings (§3.6, §3.7) | WARN | variable/entry, reason | once at startup |
| First denial of an episode | WARN | `class`, `client` (sanitized key), `route` (`c.FullPath()` template), `retry_after_seconds`, `suppressed` (count since the last emitted WARN) | **global cap**: a process-wide `rate.Limiter` (burst 10, 10/min) shared by all classes; over the cap, episodes log at DEBUG and increment `suppressed` |
| Repeat denials | DEBUG | same | — |
| Untrusted forwarded headers | WARN | peer (sanitized), scope, count | ≤ 1 per 15 min (§3.5.1) |

The Cerberus limiter adopts the same episode scheme with its own cap (fixes 3d). No log
line ever contains emails, usernames, passwords, tokens, cookies or bodies.

**Metric.** `charon_auth_rate_limited_total{class="login"|"session"}` is added to
`internal/metrics/metrics.go` and registered in `metrics.Register` (the registry `/metrics`
serves). `/metrics` is unauthenticated, so there are no client labels. Tests assert
**deltas** via `prometheus/testutil.ToFloat64`, since the counters are package-global.

**Audit rows: none.** `LogAudit` falls back to synchronous DB writes when its 100-slot channel
is full (`services/security_service.go:239-263`), and throttle events are attacker-driven.

**Notifications: deferred** (§9). The current pipeline dispatches synchronously, is gated
and Discord-only, and has different toggle semantics.

### 3.9 Middleware ordering and wiring (Decision H)

```text
gin Logger/Recovery → RequestID → RequestLogger → Recovery(debug)
→ EmergencyBypass → gzip → SecurityHeaders
→ [api]  OptionalAuth → cerb.RateLimitMiddleware() (opt-in) → cerb.Middleware() (ACL …)
→ [auth] AuthRateLimiter.Middleware()   ← NEW (detector always; throttle unless exempt/bypass/disabled)
→ [authSession only] AuthMiddleware
→ handler
```

- **After `cerb.Middleware()`:** an ACL deny wins and doesn't consume auth budget.
- **Before `AuthMiddleware`:** unauthenticated floods on session-class and password routes are
  counted per client. Only the sender is affected.
- **Before every handler (R2).**
- **Emergency bypass.** Export `middleware.EmergencyBypassContextKey` and
  `middleware.IsEmergencyBypass(c *gin.Context) bool` (an ok-form assertion).
  - Setter: `EmergencyBypass` uses the constant.
  - Readers: `AuthMiddleware` and `OptionalAuth` (replacing the inline copies at
    `auth.go:14-21` and `optional_auth.go:16-21`), the new throttle, and Cerberus
    (`rate_limit.go`, `cerberus.go:155`, fixing 3g).
  - There is no import cycle: none of `api/middleware`'s imports reach `internal/cerberus` (verified).
- **Two limiters on one request.** Independent buckets; whichever denies first answers.
  Bodies are identical, and each sends its own `Retry-After`. The control-plane exemption
  applies only to Cerberus.
- **Wiring in `RegisterWithDeps`:**
  1. `authRateLimiter, err := middleware.NewAuthRateLimiter(cfg.Security.AuthRateLimit, cfg.Security.TrustedProxies)`.
     An error fails startup closed.
  2. Register the `/auth` group (§3.2).
  3. `userHandler.SetPasswordAttemptGuard(authRateLimiter)` and
     `certHandler.SetPasswordAttemptGuard(authRateLimiter)`.
  4. Register the admin endpoint on `securityAdmin` (§3.12).

### 3.10 Other unauthenticated routes (Decision I)

| Route | In scope? | Reason |
| --- | --- | --- |
| `POST /setup`, `GET /setup` | No | Setup refuses once any user exists (`user_handler.go:141-150`, recheck at :199), and there is no secret to guess |
| `GET /invite/validate`, `POST /invite/accept` | No | Tokens are 32 random bytes (`user_handler.go:494-500`), and password hashing runs only after a valid-token match. §9 lists optional defense-in-depth |
| `POST /security/events`, OAuth callback, `/emergency/*` | No | Own auth schemes; break-glass must never be throttled |

### 3.11 Cerberus limiter changes (one hardening commit, C5)

**Migration:**

- `(*Cerberus).RateLimitMiddleware()` builds its `ratelimit.KeyedLimiter` with
  `MustNewKeyedLimiter` from constant defaults. The health and api instances keep separate budgets.
- Settings precedence moves into `effectiveRateLimit() (requests, windowSec, burst int)`,
  with behavior unchanged.
- Per request it calls
  `limiter.Reconfigure(rate.Limit(float64(requests)/float64(windowSec)), burst)`. Note the
  **float64 division**, because `requests`/`windowSec` are ints.
- It then keys via `ratelimit.ClientKey`, rejects via `ratelimit.Reject` (adding
  `Retry-After`), and logs capped per-episode WARNs.
- `Cerberus` gains an unexported `now func() time.Time` for tests.
- **Deleted:** `rateLimitManager`, `newRateLimitManager`, `cleanupLoop`, `cleanup`,
  `getLimiter`, `NewRateLimitMiddleware`, and their tests. `TestRateLimitManager_*` and
  `TestNewRateLimitMiddleware_*`, including `TestNewRateLimitMiddleware_BypassesControlPlaneBearerRequests`,
  are removed in this same commit. Equivalent coverage moves to the `ratelimit` package tests.

**Exemption fix (R13):**

- `isAdminSecurityControlPlaneRequest` becomes a method. It returns true only when:
  - `c.isAuthenticatedAdmin(ctx)` is true (role `admin` **and** userID > 0, both set only by
    `OptionalAuth` after validation); **and**
  - the decoded path matches `/api/v1/security`, `/api/v1/settings` or `/api/v1/config`
    segment-aware (`p == prefix || HasPrefix(p, prefix+"/")`).
- The bearer-prefix fallback is deleted.
- `TestCerberusRateLimitMiddleware_ControlPlaneBypassWithBearerWithoutRoleContext` becomes
  `_UnvalidatedBearerIsLimited`. New tests: `_ValidNonAdminBearerIsLimited` and `_ValidatedAdminExempt`.

### 3.12 Admin-only login-protection status (Decision L; R14)

- **Endpoint:** `GET /api/v1/security/login-protection`, registered on `securityAdmin`
  (`routes.go:858-859`, `RequireRole(admin)`).
- **Handler:** `handlers.NewLoginProtectionHandler(src LoginProtectionStatusSource).Get`.
- **Source interface:** `type LoginProtectionStatusSource interface { Status() middleware.AuthRateLimitStatus }`,
  implemented by `*middleware.AuthRateLimiter`.
- **Unchanged:** `GET /api/v1/security/status`, which is management-level (`routes.go:847`).

```json
{
  "enabled": true,
  "login":   { "requests": 10, "window_seconds": 600 },
  "session": { "requests": 60, "window_seconds": 60 },
  "trusted_proxy_count": 1,
  "caller_client_key": "203.0.113.7",
  "caller_client_scope": "public",
  "untrusted_forwarded_headers": {
    "local":  { "count": 3, "last_seen": "2026-09-24T10:11:12Z", "last_peer": "172.18.0.5", "last_peer_scope": "private" },
    "public": { "count": 0, "last_seen": null, "last_peer": "", "last_peer_scope": "" }
  }
}
```

- `caller_client_key` / `caller_client_scope` echo the key the throttle would use for **this**
  request (`ratelimit.ClientKey(c.ClientIP())`). They serve two purposes:
  - the admin self-check for runtime NAT (§3.5.2);
  - the E2E effectiveness probe (§4.1). It is deterministic and independent of detector counters.
- Per record, `last_seen` is `null` and `last_peer`/`last_peer_scope` are empty while
  `count == 0`. `local.last_peer_scope` is `loopback` or `private`; `public` is always `public`.
- JSON tags are snake_case. `trusted_proxy_count` is the **effective** count (§3.7).
- **Peer addresses appear only here**, and only admins can read them.

### 3.13 Frontend design

#### 3.13.1 429 helper and interceptor

New file `frontend/src/utils/rateLimit.ts`:

| Function | Contract |
| --- | --- |
| `parseRetryAfter(value: unknown, nowMs = Date.now()): number \| null` | Delta-seconds (non-negative integer string) → `max(1, n)`. HTTP-date → `max(1, ceil((date − now)/1000))`. Anything else → `null` |
| `getRetryAfterSeconds(error: unknown): number \| null` | Reads `error.response.headers` (`AxiosHeaders.get('retry-after')` or plain-object key) |
| `isRateLimitError(error: unknown): boolean` | `error.response?.status === 429` |
| `rateLimitMessage(t: TFunction, error: unknown): string \| null` | `null` unless 429. Under 60 s → `errors.tooManyRequestsSeconds` `{count}`. 60 s or more → `errors.tooManyRequestsMinutes` `{count: ceil(s/60)}`. Unknown → `errors.tooManyRequests` |

In `api/client.ts`, after the existing `data.error` extraction:
`const rl = rateLimitMessage(i18n.t, error); if (rl) error.message = rl`. This covers Setup,
change-password (via `AuthContext`), and any Cerberus 429 app-wide.

#### 3.13.2 Login page notice with an administrator pointer

On `429`, `pages/Login.tsx` renders an inline `<Alert variant="warning">` below the form
(`data-testid="login-rate-limit-notice"`, `role="alert"`) **instead of** a toast. It contains:

- the localized wait message (`rateLimitMessage`);
- `auth.rateLimitAdminHint`: "Are you the administrator? If everyone sees this message,
  Charon may not be seeing your visitors' real addresses.";
- an `<a>` (`target="_blank"`, `rel="noopener noreferrer"`) labeled
  `auth.rateLimitAdminHintLink` ("How login protection works behind a proxy"), pointing to
  `TRUSTED_PROXIES_DOCS_URL`.

The notice clears on the next submit. Other errors keep the existing toast. The link is a
plain element next to the text, so no `<Trans>` is needed.

#### 3.13.3 Admin login-protection card (Decision L; placement: M8)

- **Component:** `frontend/src/components/LoginProtectionCard.tsx`.
- **Placement:** rendered on `pages/Security.tsx` for `user?.role === 'admin'` only,
  **outside** the Cerberus-enabled conditionals, because login protection is always on.
- **Data:** `useLoginProtectionStatus({ enabled: isAdmin })` (React Query) in
  `hooks/useSecurity.ts` → `getLoginProtectionStatus()` in `api/security.ts` → type `LoginProtectionStatus`.

| State | Condition | Content |
| --- | --- | --- |
| Healthy | `enabled` and no recent observation | "On". Budgets. "Charon sees your browser as `<caller_client_key>` (`<scope>`)". Runtime-NAT hint: "If that isn't your device's address, a proxy or your container runtime may be hiding visitor addresses." Docs link |
| Warning (private/loopback peer) | `local.count > 0` and `local.last_seen` within 24 h, regardless of any later public observations | Everything above, plus a warning: "Forwarded client addresses from `<last_peer>` are being ignored ({{count}} times, last `<relative time>`). If `<last_peer>` is your reverse proxy, add `<last_peer>/32` (or `/128` for IPv6) to `CHARON_TRUSTED_PROXIES`." |
| Info (public peer) | `public.count > 0`, `public.last_seen` within 24 h, and no Warning state (a Warning takes precedence; the info note may appear below it) | An informational note: "Charon ignored forwarded client-address headers from a public address (`<last_peer>`). This is usually someone sending forged headers; no action is needed." **No trust suggestion and no snippet** |
| Off | `enabled = false` | "Login protection is turned off (`CHARON_AUTH_RATELIMIT_ENABLED=false`)." Docs link |
| Error/loading | query state | The standard skeleton or error line. The card never blocks the page |

#### 3.13.4 Docs URL constants

New file `frontend/src/constants/docs.ts`:

- `DOCS_SITE_URL = 'https://wikid82.github.io/Charon/docs'`;
- `TRUSTED_PROXIES_DOCS_URL`;
- `LOGIN_PROTECTION_DOCS_URL`.

Only the new links use these constants. Migrating existing hardcoded links is a §9 follow-up.

#### 3.13.5 i18n keys

The new keys go in all five locale files. Plural keys carry `_one`, `_other` **and** a bare
fallback in every locale, including zh (the `hostCount` convention). Counts are bounded, so
the CLDR `many` form is unreachable.

| Key | en (source copy; de/es/fr/zh translated in the same register) |
| --- | --- |
| `errors.tooManyRequests` | Too many attempts. Please wait a moment and try again. |
| `errors.tooManyRequestsSeconds` (+`_one`/`_other`) | Too many attempts. Please wait {{count}} seconds and try again. (`_one`: second) |
| `errors.tooManyRequestsMinutes` (+`_one`/`_other`) | Too many attempts. Please wait {{count}} minutes and try again. (`_one`: minute) |
| `auth.rateLimitAdminHint` | Are you the administrator? If everyone sees this message, Charon may not be seeing your visitors' real addresses. |
| `auth.rateLimitAdminHintLink` | How login protection works behind a proxy |
| `security.loginProtection.*` | Title, on/off, budgets, `callerAddress`, `runtimeNatHint`, `untrustedPrivate` (+ plural), `untrustedPublic` (+ plural), scope labels (loopback/private/public), `docsLink` |

Revision-1 drafts for the `errors.*` keys (de/es/fr/zh) carry over unchanged.

### 3.14 Data flow (login attempt)

```mermaid
sequenceDiagram
    participant B as Browser (Login.tsx)
    participant P as Reverse proxy (optional, trusted)
    participant G as Gin chain (api group)
    participant T as AuthRateLimiter (login class)
    participant H as AuthHandler.Login / AuthService
    B->>P: POST /api/v1/auth/login
    P->>G: + X-Forwarded-For: <client>
    G->>G: OptionalAuth → Cerberus limiter (if on) → ACL
    G->>T: detector check; ClientIP() → ClientKey (IPv4 | IPv6/64)
    alt bucket has a token
        T->>H: c.Next()
        H-->>B: 200 {token} | 401 {error}
    else bucket empty
        T-->>B: 429 Retry-After: N, {"error": generic}
        B->>B: inline notice "Please wait N seconds" + admin pointer (localized)
    end
```

### 3.15 Error handling and edge cases

| Case | Behavior |
| --- | --- |
| `c.ClientIP()` is `""` or unparsable, including zoned IPv6 peers | `UnknownClientKey` shared bucket (fail-closed) |
| IPv4-mapped IPv6 | Same key as the IPv4 address |
| Clock moves backwards | Monotonic `time.Now` in prod; `x/time/rate` clamps |
| Cerberus settings change mid-flight | `Reconfigure` purges under the same mutex |
| Unknown `/auth/*` route (future) | `session` class; the inventory test fails CI until it is classified |
| New `CheckPassword` call site (future) | The tripwire test fails CI until it is guarded and allowlisted (§4.4) |
| `c.FullPath() == ""` (defensive) | `session` class |
| Budget env = `0`, `-5`, `10m`, `abc` | `-1` sentinel → default + WARN (never a burst of 0, never ∞) |
| `CHARON_AUTH_RATELIMIT_ENABLED` = `no`/`0`/`off` | Stays **enabled**, with a WARN "unrecognized value" |
| One invalid `CHARON_TRUSTED_PROXIES` entry | No proxy trusted anywhere + WARN (§3.7) |
| Trusted proxies contain `0.0.0.0` or `::` | Startup WARN (mirrors Gin) |
| Forwarded headers from a public, untrusted peer | Headers ignored; recorded with scope `public`; guidance never suggests trusting it |
| Trusted peer sends a malformed forwarded header | Gin keys on the peer; not counted by the detector; DEBUG |
| Denied client retries early | Denials consume nothing |
| Restart | Buckets and detector state reset (accepted) |
| Cerberus limiter also on | Both evaluate; the first denial wins; identical 429 body |

---

## 4. Implementation Plan

Commits run strictly in §6 order. Every validation command runs in the **foreground**
(CLAUDE.md "Execution Discipline").

### 4.1 Phase 1: Playwright tests and fixtures (spec behavior first)

**New file: `tests/core/auth-rate-limit.spec.ts`.** It is collected by `--project=firefox`
and by the PR Firefox shards. Every test starts as `test.fixme` (C3) and is enabled in C9.
The header cites issue #1317 and `docs/features/login-protection.md`, never
`docs/plans/current_spec.md`.

**Helpers:**

- `isolatedClientIp()` returns a random `198.18.0.0/15` address (RFC 2544 benchmarking range) via `crypto.randomInt`.
- `exhaustLoginBudget(ctx)` sends concurrent batches of 25 `POST /api/v1/auth/login`
  requests. It uses unique non-existent probe accounts (`probe-<uuid>@test.local`), so no
  real account is affected. It stops at the first 429, capped at 2,000.

**Guard (tests 1–3) is an effectiveness probe:**

1. Send `GET /api/v1/security/login-protection` with the admin storage state and
   `X-Forwarded-For: <isolatedClientIp()>`.
2. Require all three:
   - `enabled`;
   - a fast refill (`login.requests / login.window_seconds >= 1`);
   - `caller_client_key === <probe address>` (proves XFF is honored end to end).
3. Otherwise `test.skip(...)` with a message naming the Playwright compose files.

This proves bucket isolation actually works on the stack under test, rather than trusting
configuration.

| # | Test | Asserts |
| --- | --- | --- |
| 1 | "throttles a client that exhausts its sign-in budget" | With `X-Forwarded-For: ipA`: all non-429 responses are 401; the first 429 has an integer `Retry-After` ≥ 1, body equal to `{error: <generic>}`, and no `probe-` in the body |
| 2 | "keys the throttle on the real client behind a trusted proxy" | After exhausting `ipA`: `ipB` → 401 (not 429); the runner (no XFF) gets `GET /auth/status` 200 and a probe login 401 |
| 3 | "keeps session reads available while sign-in is throttled" | With `ipA` exhausted, admin storage state plus `XFF: ipA`: `GET /auth/me` 200; `GET /auth/status` 200 |
| 4 | "login page explains the wait and points administrators to the docs" | No guard. Unauthenticated state; `page.route('**/api/v1/auth/login')` fulfills 429 with `Retry-After: 42`. `getByTestId('login-rate-limit-notice')` shows `/wait 42 seconds/i` and a link whose `href` contains `/configuration/trusted-proxies` |
| 5 | "admin card suggests trusting a private proxy that sends forwarded headers" | No guard. `page.route('**/api/v1/security/login-protection')` returns a private `last_peer` (e.g. `172.18.0.5`) with a recent `last_seen`. On `/security` the card shows the warning with `172.18.0.5/32` and the docs link |
| 6 | "admin card never suggests trusting a public peer" | Same, with `last_peer: 203.0.113.9`, scope `public`. The card shows the informational note and contains **no** `CHARON_TRUSTED_PROXIES` suggestion or `/32` snippet |

If the Cerberus limiter is also on in a local stack, test 1 still passes, because both
limiters answer through the same `Reject`.

**Fixture hardening** (in C3):

- `postLoginWithRetry` (`tests/fixtures/auth-fixtures.ts`): on 429, honor `Retry-After`
  (capped at 10 s), retry once, then throw `Error("login throttled (429): E2E auth budgets
  too low for this stack?")`.
- `TestDataManager.createUser` (`tests/utils/TestDataManager.ts:628-642`): on 429, honor
  `Retry-After` once; throw only if the retry is **still** 429. Other non-OK logins keep
  today's behavior (warn and return `token: ''`).

### 4.2 Phase 2: Backend

| Step | Files | Work |
| --- | --- | --- |
| 2.1 | `backend/internal/ratelimit/**`, `backend/go.mod`, `backend/go.sum` | §3.3/§3.4 (C4). Add `github.com/hashicorp/golang-lru/v2 v2.0.7` as a direct require. Review `go.work.sum` changes deliberately |
| 2.2 | `internal/cerberus/{rate_limit.go,rate_limit_test.go,cerberus.go}`, `internal/api/middleware/{emergency.go,auth.go,optional_auth.go}` (+ tests) | §3.11 plus the exported bypass helper (C5) |
| 2.3 | `internal/config/{config.go,config_test.go}` | `AuthRateLimitConfig`, `Normalize`, `getEnvIntStrictAny`, `StartupWarnings`, `ENABLED` parsing (C6) |
| 2.4 | `internal/api/middleware/auth_rate_limit.go` (+ `_test.go`) | Classes/table, `NewAuthRateLimiter`, `Middleware`, `AllowPasswordAttempt`, detector, `Status`, capped logging, clock option (C6) |
| 2.5 | `internal/metrics/{metrics.go,metrics_test.go}` | `charon_auth_rate_limited_total{class}` + `IncAuthRateLimited`, in `Register` (C6) |
| 2.6 | `internal/config/config.go` (+ test), `cmd/api/main.go` (+ test) | `ValidateTrustedProxies` in `Load` (§3.7); log `StartupWarnings` after `logger.Init` (C7) |
| 2.7 | `internal/api/routes/{routes.go,routes_test.go}` + `password_guard_inventory_test.go` | §3.2 group, §3.9 wiring, guards, admin endpoint (C7) |
| 2.8 | `internal/api/handlers/{user_handler.go,certificate_handler.go,login_protection_handler.go}` (+ tests) | Password guards (§3.2.1); admin endpoint (§3.12) (C7) |
| 2.9 | `frontend/src/api/security.ts` | `LoginProtectionStatus` type + `getLoginProtectionStatus()`, shipping with the backend contract (C7) |
| 2.10 | `.docker/compose/docker-compose.playwright-{ci,local}.yml` | §3.6 E2E block; fix the stale comment (C7) |

### 4.3 Phase 3: Frontend

| Step | Files | Work |
| --- | --- | --- |
| 3.1 | `frontend/src/utils/rateLimit.ts` + test | §3.13.1 |
| 3.2 | `frontend/src/api/client.ts` + test | Interceptor 429 localization |
| 3.3 | `frontend/src/pages/Login.tsx` + test | §3.13.2 inline notice |
| 3.4 | `frontend/src/components/LoginProtectionCard.tsx` + test; `pages/Security.tsx` + test; `hooks/useSecurity.ts` | §3.13.3 |
| 3.5 | `frontend/src/constants/docs.ts` + test | §3.13.4 |
| 3.6 | `frontend/src/locales/{en,de,es,fr,zh}/translation.json`, `src/__tests__/i18n.test.ts` | §3.13.5 plus a key-parity test |

### 4.4 Phase 4: Integration and testing (test matrix, Decision J)

All Go tests use an injected clock and **no `time.Sleep`**. `scripts/go-test-coverage.sh`
runs with `-race`.

| Area | Test (file) | Proves |
| --- | --- | --- |
| Algorithm | `TestKeyedLimiter_BurstThenDeny`, `_RefillAfterInterval`, `_DenialsDoNotConsume`, `_IndependentKeys` | Burst, exact `RetryAfter`, refill, independence |
| Retry-After math | `TestRetryAfterSeconds` (10/600 s → 60; 60/60 s → 1; 7/100 s → 15; sub-ms → 1) | No float off-by-one |
| Memory bound | `TestKeyedLimiter_MaxKeysHardCap` (100 cap, 10,000 keys → `Len()==100`), `_SweepEvictsOnlyIdleFullBuckets`, `_SweepKeepsRecentlyDepletedKey` | R9; lossless sweep |
| No goroutines | `TestKeyedLimiter_StartsNoGoroutines` (non-parallel; `NumGoroutine` delta 0 after 100 constructions) | 3b fixed |
| Concurrency | `TestKeyedLimiter_ConcurrentSameKeyExactBurst` (16×100 calls, frozen clock → exactly Burst allowed), `_ConcurrentAllowAndReconfigure` | `-race` clean |
| Validation | `TestNewKeyedLimiter_RejectsInvalidConfig`, `TestPerWindow` (float64 math) | Rejects invalid Burst, Rate and MaxKeys |
| Keying/scope | `TestClientKey` (IPv4, mapped, same/different /64, loopback, zone, host:port, `""`/garbage), `TestClassifyAddr` (RFC 1918, ULA, CGNAT, link-local, loopback, public v4/v6) | R7; scope rules |
| HTTP | `TestReject_SetsRetryAfterAndGenericBody` | R3 |
| Config | `TestLoadAuthRateLimitConfig_Defaults`, `_Overrides`, `_MalformedSentinel`, `_EnabledTrueFalseOnly`, `_EnabledUnrecognizedWarnsAndStaysOn`, `TestAuthRateLimitConfig_NormalizeZeroValueIsSecureDefault`, `_NormalizeBounds`, `TestValidateTrustedProxies_*` (§3.7), `TestLoad_StartupWarningsCollected` | R10, R15; pins production defaults |
| Main | `TestMain_LogsStartupWarnings` (or equivalent helper test) | Warnings reach the log |
| Middleware | `TestAuthRateLimiter_LoginAndChangePasswordShareBucket`, `_SessionClassSeparateBudget`, `_ExemptRoutesNeverThrottled`, `_UnknownRouteDefaultsToSession`, `_EmergencyBypassSkips`, `_DisabledPassesThrough`, `_EpisodeLoggingOncePerEpisode`, `_GlobalWarnCapWithSuppressedCount`, `_LogsContainNoCredentials`, `_MetricDeltaPerClass` (`testutil.ToFloat64` before/after), `_AllowPasswordAttemptSharesLoginBucket` | R1–R4, R8, R12 |
| Detector | `_UntrustedPeerCountsAndRecordsScope`, `_TrustedPeerMalformedHeaderNotCounted`, `_PublicPeerWarnHasNoTrustSuggestion`, `_PrivatePeerWarnSuggestsSlash32Or128`, `_DetectorWarnRateLimited` (fake clock), `_PerScopeStateIndependent`, `_PublicNoiseDoesNotMaskPrivateWarning` | R5 |
| Trusted proxy | `_UntrustedPeerIgnoresForwardedHeaders`, `_TrustedPeerKeysOnRealClient`, `_TrustedPeerUsesRightmostUntrustedHop`, `_IPv6SlashSixtyFourAggregation`. All use a real `gin.Engine` + `SetTrustedProxies` | R6, D |
| Cerberus | `Retry-After` present, `_UnvalidatedBearerIsLimited`, `_ValidNonAdminBearerIsLimited`, `_ValidatedAdminExempt`, `_SegmentAwarePrefix`, `_SettingsReconfigureResetsBuckets`, `_PerEpisodeWarnCapped`, `_NoGoroutineLeak` | 3a–3g |
| Route wiring | `TestRegister_AuthRoutesHaveExplicitRateLimitClass` (two-way inventory); `_RefreshThrottledAsSession` (group-ordering guard); `_LoginThrottledBeforeHandler` (11th → generic 429 body); `_ChangePasswordSharesLoginBudget`; `_ProfileEmailChangeSharesLoginBudget` (name-only update still 200); `_CertificateKeyExportSharesLoginBudget` (`include_key:true` → 429 after exhaustion; `include_key:false` not throttled); `_ExemptAuthRoutesNeverThrottled`; `_SessionBudget`; `_AuthThrottleIndependentOfCerberusToggle`; `_AuthThrottleDisabledByConfig`; `_EmergencyEndpointsNeverThrottled`; `_EmergencyBypassSkipsAuthThrottle`; `_LoginProtectionEndpointAdminOnly` (user → 403, admin → 200) | A, H, R2, R8, R14, B4 |
| Password tripwire | `TestPasswordVerificationCallSitesAreGuarded` (`go/parser` over **every non-test `.go` file under `backend/internal` and `backend/cmd`**). Any `.CheckPassword` **selector** (call or method value) must sit in an allowlisted func (`AuthService.Login`, `AuthService.ChangePassword`, `UserHandler.UpdateProfile`, `CertificateHandler.Export`); `bcrypt.CompareHashAndPassword` selectors are optionally held to the same allowlist; if enabled, that allowlist must also include the existing token-verification sites (`orthrus/server.go`, `SecurityService` at `security_service.go:205`, `EmergencyTokenService` at `emergency_token_service.go:160`). In the two handler funcs, `AllowPasswordAttempt` must precede `CheckPassword` **by source position**; this is not a control-flow-dominance proof, and the route tests cover behavior | Password-route regression guard |
| Status API | `TestLoginProtectionHandler_Get_*` (fields, `null` last_seen, caller key echo with and without trusted XFF) | §3.12 |
| Frontend | `rateLimit.test.ts` (delta, HTTP-date, `0`, negative, junk, missing, AxiosHeaders vs plain, 1/59/60/61/3600 boundaries); `client.test.ts`; `Login.test.tsx` (429 → inline notice with 42 s and docs link, no toast; other errors → toast); `LoginProtectionCard.test.tsx` (healthy, private warning with `/32` and `/128`, public info without snippet, off, 24 h staleness, error); `Security.test.tsx` (card only for admins, visible with Cerberus off); `docs.test.ts`; `i18n.test.ts` (all new keys resolve in 5 locales, with plural forms) | R11, R14 |
| E2E | §4.1 tests 1–6 | End-to-end |

**Coverage.** The new `ratelimit` package targets ≥ 95%. Backend, frontend and patch
coverage must each be **≥ 87%** (the repo's real gate: `scripts/go-test-coverage.sh:14`,
`scripts/frontend-test-coverage.sh:15`, `codecov.yml`).

### 4.5 Phase 5: Documentation and deployment (Decision K)

| File | Change |
| --- | --- |
| `docs/features/login-protection.md` (**new**; auto-published) | Plain language: what it does, defaults, what users see, the lockout second layer, env table. **"All visitors share one address"** covers both cases: an untrusted proxy (→ trusted-proxies doc) and runtime NAT (verified remedies from §3.5.2, a pointer to the admin card's self-check, the kill switch) |
| `docs/configuration/trusted-proxies.md` (**new**; auto-published) | Durable home for `CHARON_TRUSTED_PROXIES`, detailed below the table |
| `.docker/compose/docker-compose.yml` | A commented `# - CHARON_TRUSTED_PROXIES=<your proxy's IP>` line with a one-line explanation in the `environment:` block, plus a pointer comment on the `8080:8080` port line (`:12`) |
| `docs/features.md`, `docs/features/security.md` | Brief "Login Protection" entries. **Remove the unavailable "Require Login" gateway claims** (`features/security.md:25-27`; `features.md:69`) |
| `docs/security.md` | "Login Protection" section; fix the stale Tier-1 bullet (:491); link the "ClientIP spoofing" bullet (:496) to trusted-proxies.md |
| `docs/troubleshooting/proxy-headers.md` | New problem: "Everyone sees 'please wait … seconds' on the login page", with both causes (untrusted proxy; runtime NAT) and their remedies |
| `docs/api.md` | Rewrite "Rate Limiting" (:1761) with the 429 contract and budgets; correct the stale "Authentication not yet implemented" blurb (:16-24); document the admin-only `GET /api/v1/security/login-protection` |
| `docs/runbooks/emergency-lockout-recovery.md` | Symptom 4 (the real 429 body, `Retry-After`, recovery); replace the stale test-environment section (:60-68) |
| `ARCHITECTURE.md` | Tech-stack rows; `internal/ratelimit/` in the directory structure; Security Suite text; Layer 1 rewrite; a throttle paragraph in "Management API Authentication & Authorization"; correct the Management Interface claim (:986); env-var rows; bcrypt wording (:913) |
| Code comments | `config.go:57-65`, `server.go:15-17`, `auth_handler_test.go:325-329` → `docs/configuration/trusted-proxies.md` (+ commit `3b1cd2bb` for design history) |

`docs/configuration/trusted-proxies.md` covers:

- **What it does.** Client-address resolution for login protection and other IP-based
  features, plus `X-Forwarded-Proto`/`X-Forwarded-Host` for HTTPS detection. Headers are
  honored only from listed proxies.
- **Exact addresses only**, with IPv4 `/32` and IPv6 `/128` examples.
- **XFF requirement.** The proxy **must append to or overwrite** `X-Forwarded-For` (nginx
  `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`, as in
  `docs/troubleshooting/websocket.md:53-54`). A proxy that sets only `X-Real-IP` and passes
  the client's own `X-Forwarded-For` through lets clients choose their key.
- **Stable addresses.** Docker container addresses change on recreate, so recommend a static
  proxy IP or a dedicated proxy subnet.
- **Topology examples** from §2.4: a `localhost:8080` self-proxy (`127.0.0.1/32` and
  `::1/128`), a `charon:8080` self-proxy (the container's own address), and external nginx
  (its Docker-network address).
- **Invalid entries.** One invalid entry means no proxy is trusted. This is the behavior
  change from §3.7.
- **Verifying the setup** via the admin card's "Charon sees your browser as".

**Manifests and ignore files.**

- No `docs-site/scripts/docs-manifest.json` change is needed: both new files are in
  manifested directories. `docs/runbooks/` stays contributor-only.
- No `.gitignore`, `.dockerignore` or `codecov.yml` changes are needed.

**Process note (disclosure hygiene).** The PR description, the commit bodies, and
`docs/reports/qa_report.md` must not describe unfixed weaknesses. They may reference this
spec's fixed items factually. Anything else goes through the maintainer's private
tracking. This spec, committed in C0, follows the same rule.

**Deployment and release notes.** The docs site must publish `configuration/trusted-proxies`
no later than the release that ships the login notice (C8 and C10 ship together).

- Login protection turns on automatically.
- Operators behind a reverse proxy should set `CHARON_TRUSTED_PROXIES`. Operators on
  rootless or desktop runtimes should check the admin card.
- An invalid trusted-proxy entry now disables proxy trust everywhere, with a WARN (§3.7).
- There is no migration, no model change and no DB change.

### 4.6 Complexity estimates

| Component | Size | Notes |
| --- | --- | --- |
| `internal/ratelimit` + tests | M | ~200 LOC + ~450 LOC tests |
| Cerberus hardening (C5) | M | Net deletion; test rewrites; bypass helper |
| Config + validation + warnings | M | Table-driven tests |
| Auth middleware + detector + guard + metric | M | Largest test surface |
| Routes wiring, handlers, admin endpoint, compose | M | Inventory, ordering and tripwire tests |
| Frontend (helper, notice, card, i18n) | M | 5 locales |
| E2E spec + fixtures | M | Probe guard + 6 tests |
| Docs | M | 2 new files + 9 edits |
| **Total** | ~4–5 dev-days | |

---

## 5. Acceptance Criteria

### 5.1 Functional

| # | Criterion | Req |
| --- | --- | --- |
| F1 | With zero config, the 11th `login`-class request from one client within 60 s returns 429 with `Retry-After` ≤ 60 and the generic body; after the refill interval, one more request is allowed | R1–R3 |
| F2 | Login, change-password, profile email change and certificate key export share one per-client bucket. Refresh and status share another. The five exempt routes are never throttled | A, R4 |
| F3 | A throttled request never reaches password verification or account bookkeeping | R2 |
| F4 | Behind a trusted proxy, distinct real clients get independent buckets. From an untrusted peer, forwarded headers are ignored, the detector records count, last-seen, peer and scope, and the WARN is capped and scope-aware | R5, R6 |
| F5 | IPv6 clients in one /64 share a bucket; empty, unparsable and zoned client IPs share `unknown` | R7 |
| F6 | `/api/v1/emergency/*`, the Tier-2 server and emergency-bypass requests are never throttled | R8 |
| F7 | No limiter starts goroutines; tracked keys never exceed `MaxKeys` | R9 |
| F8 | Malformed values fall back to defaults with a WARN. Only `ENABLED=false` disables; unrecognized values WARN and stay on. The zero-value config is enabled | R10 |
| F9 | The login page shows the localized wait notice with an administrator pointer and docs link, in all 5 locales | R11 |
| F10 | Logs contain no credentials; WARN volume is capped with a suppressed count | R12 |
| F11 | The Cerberus limiter exempts only `OptionalAuth`-validated admins on segment-aware control-plane paths; its 429s carry `Retry-After`; its memory is bounded | R13, B |
| F12 | `GET /api/v1/security/login-protection` is admin-only and returns the §3.12 schema. The card shows it only to admins, whatever the Cerberus state, and never suggests trusting a public peer | R14, L |
| F13 | One invalid `CHARON_TRUSTED_PROXIES` entry yields no trusted proxy anywhere, plus a WARN; trust-all lists WARN | R15, M |

### 5.2 Definition of Done (CLAUDE.md, in order)

| # | DoD step | How it applies |
| --- | --- | --- |
| 1 | Playwright, targeted and firefox-only | Rebuild (`.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`), then `npx playwright test tests/core/auth-rate-limit.spec.ts tests/core/authentication.spec.ts --project=firefox`. All pass; tests 1–3 **run** (the probe guard passes) |
| 1.5 | GORM security scan | **Not triggered**: no models, GORM queries or migrations. State this in the PR |
| 2 | Local patch coverage | `bash scripts/local-patch-report.sh` → `test-results/local-patch-report.{md,json}`; patch ≥ 87% |
| 3 | Security scans (feat ⇒ local) | CodeQL Go + JS via `lefthook run pre-commit`. Trivy via `.github/skills/scripts/skill-runner.sh security-scan-trivy` and `make security-scan-full` (image scan). Zero high/critical |
| 4 | Lefthook triage | `lefthook run pre-commit` is clean |
| 5 | Staticcheck (blocking) | `make lint-fast`; `make lint-backend` before the PR |
| 6 | Coverage | `scripts/go-test-coverage.sh` and `scripts/frontend-test-coverage.sh` each ≥ 87% |
| 7 | Type safety | `cd frontend && npm run type-check` |
| 8 | Build | `cd backend && go build ./...`; `cd frontend && npm run build` |
| 9 | All tests pass | Including `-race` |
| 10 | Clean-up | No debug output or dead code; deleted internals leave no orphans |

---

## 6. Commit Slicing Strategy

**Decision: one PR into `development` with ordered, logical commits.** Each commit builds
and passes its own gate, and the PR as a whole passes §5.2 before merge. Every validation
command runs in the foreground. Local E2E is firefox-only and targeted.

### 6.1 How commit subjects get published (verified)

- **Feature PRs use merge commits.** They are not squash-merged:
  - `8168732a` (#1316) and `1f6588e6` (#1373) each have two parents;
  - the merge subject is the PR title plus `(#PR)`.
- **The What's New generator publishes every subject.** `scripts/generate-changelog.sh:86`
  runs `git log "$range"` **without** `--first-parent`, so every subject in the tag range is
  published verbatim: each branch commit, the merge subject, and any "Merge branch …" commits.
- **Categorization** (`generate-changelog.sh:34-58`):
  - `^(feat|fix)\(security\)!?:` subjects → one **Security** entry each;
  - `feat` / `fix` → Features / Fixes;
  - everything else → Other. `refactor(security):` is not matched by the Security pattern,
    so it lands in Other.
- **Precedent.** The #1316 merge published **four** Security entries in v0.40.2: the merge
  subject plus three branch `fix(security)` commits.

### 6.2 Subject policy (recommended; maintainer decision M3)

1. **Exactly one user-facing Security entry: the merge subject.** Title the PR
   `feat(security): harden authentication endpoints against abuse (#1317)`. GitHub then
   produces the merge subject `feat(security): harden authentication endpoints against
   abuse (#1317) (#<PR>)`.
2. **At most one hardening entry: C5**, `fix(security): harden request throttling in the API layer`.
3. **No other `(feat|fix)(security)` subjects.** The wiring commit C7 is plain `feat:`,
   because the merge subject already carries the Security entry. It shows up once in
   Features, consistent with #1373's multiple Features entries.
4. **Every subject must be safe to publish verbatim.** No weakness class, vector or code
   path; describe the protection or mechanism.
5. **Prefer rebasing onto `development`** over merging it into the branch, so no
   "Merge branch 'development' …" subjects are published. If a merge is needed, accept that
   Other entry.

### 6.3 Commits

| # | Subject (published verbatim) | Category | Scope | Files (main) | Depends on | Validation gate |
| --- | --- | --- | --- | --- | --- | --- |
| C0 | `docs: archive redirection-hosts spec and add plan for #1317` | Other | Planning artifacts | `docs/plans/current_spec.md`, `docs/plans/archive/2026-09-23_redirection-hosts-1367_spec.md` | — | `npx markdownlint-cli2 docs/plans/current_spec.md` (the archive is lint-ignored) |
| C1 | `chore: repoint redirection-host comments to the archived spec` | Other | Comment-only; fixes the 21 refs broken by C0 | `routes.go` (:75, :1044), `routes_test.go` (:1742-1744), `caddy/{config.go:103,401, config_options.go:12, redirect_routes.go:48, manager.go:115, manager_redirect_test.go:50, config_redirect_test.go:15,39,107}`, `handlers/redirection_host_handler.go:25`, `services/redirectionhost_service.go:15,59`, `models/redirection_host.go:12,33`, `frontend/src/components/RedirectionHostForm.tsx:74`, `frontend/src/api/redirectionHosts.ts:46` | C0 | (a) `git grep -c '2026-09-23_redirection-hosts-1367_spec.md' -- <C1 files>` totals 21. (b) `git grep -n 'docs/plans/current_spec.md' -- <C1 files>` returns **exactly 5 matches**: the Web Push singleton-index comment and the Web Push provisioning comment in `routes.go`, the two Web Push subscription allowlist reasons in `routes_test.go`, and the read-route-audit scope comment in `routes_test.go`. They are left for the §9 sweep. (c) `go build ./... && go vet ./...`; `cd frontend && npm run lint && npm run type-check` |
| C2 | `chore: remove stale test backups and an unused auth helper` | Other | CLEAN | Delete tracked `backend/internal/metrics/metrics_test.go.bak` and `security_metrics_test.go.bak`; remove `isProduction()` (`auth_handler.go:32-36`) and its test | — | `go build ./... && go vet ./...`; `go test ./internal/api/handlers/... ./internal/metrics/...` |
| C3 | `test: add pending E2E coverage for authentication rate limiting` | Other | `fixme` spec + fixture hardening | `tests/core/auth-rate-limit.spec.ts`, `tests/fixtures/auth-fixtures.ts`, `tests/utils/TestDataManager.ts` | — | Needs a **running E2E stack** (the firefox project depends on `setup`, which logs in; any current development image works). Run `npx playwright test tests/core/auth-rate-limit.spec.ts tests/core/authentication.spec.ts --project=firefox`: the new spec is skipped, and authentication passes with the hardened fixtures |
| C4 | `refactor: add shared keyed rate limiter package` | Other | New unwired package + dependency | `backend/internal/ratelimit/**`, `backend/go.mod`, `backend/go.sum` | — | `go test -race ./internal/ratelimit/...` (≥ 95%); `make lint-fast` (no depguard/gomodguard rules exist); `go build ./...` |
| C5 | `fix(security): harden request throttling in the API layer` | **Security** (hardening) | Cerberus → shared limiter; all of §2.3 (3a–3g); exported bypass helper | `internal/cerberus/**`, `internal/api/middleware/{emergency,auth,optional_auth}.go` (+ tests) | C4 | `go test -race ./internal/cerberus/... ./internal/api/middleware/... ./internal/api/routes/...`; `make lint-fast`. The PR's "E2E Firefox Security" CI job covers the security-enforcement specs (§7 RK7) |
| C6 | `refactor: add configuration and middleware for per-client request limits` | Other | Unwired: config, middleware, detector, guard method, metric | `internal/config/**`, `internal/api/middleware/auth_rate_limit*.go`, `internal/metrics/metrics*.go` | C4, C5 | `go test -race ./internal/config/... ./internal/api/middleware/... ./internal/metrics/...`; `make lint-fast` |
| C7 | `feat: apply per-client limits to sign-in and password checks` | Features | Wiring: `/auth` group, password guards, admin endpoint + TS contract, trusted-proxy validation + startup warnings, E2E compose env | `routes.go` + tests, `handlers/{user,certificate,login_protection}_handler.go` + tests, `config.go`, `cmd/api/main.go`, `frontend/src/api/security.ts`, playwright compose files | C6 | `cd backend && go test -race ./... && go build ./...`; `cd frontend && npm run type-check`; rebuild E2E, then `npx playwright test tests/core/authentication.spec.ts --project=firefox` |
| C8 | `feat: explain sign-in waits and show login protection status to admins` | Features | Frontend (§3.13) | Phase 3 files | C7 | `cd frontend && npm run lint && npm run type-check && npm run test && npm run build`; `scripts/frontend-test-coverage.sh` ≥ 87% |
| C9 | `test: enable E2E coverage for authentication rate limiting` | Other | Remove `fixme` | `tests/core/auth-rate-limit.spec.ts` | C7, C8 | Rebuilt E2E stack, then `npx playwright test tests/core/auth-rate-limit.spec.ts tests/core/authentication.spec.ts --project=firefox`, all green with tests 1–3 executed |
| C10 | `docs: document login protection and trusted proxy setup` | Other | §4.5, including the compose hint, forward-auth docs correction and §13 repoints. Must link the upstream runtime pages with version caveats (docs-writer re-verifies). **Must land in the same release as C8**, so the docs site publishes `configuration/trusted-proxies` no later than the login notice that links to it | Phase 5 files | C7 | `npm run lint:md` (or `npx markdownlint-cli2` on changed files); `go build ./...` (comment edits) |

- **Merge subject:** `feat(security): harden authentication endpoints against abuse (#1317) (#<PR>)`,
  which is the only other Security entry.
- **Final run:** after C10, run §5.2 once over the PR (patch report, CodeQL + Trivy, lefthook, coverage, builds).

### 6.4 Rollback and contingency

- **Whole feature.** Revert the merge commit with `git revert -m 1 <merge>`. There is no
  data migration, and config is env-only.
- **Runtime kill switch (no redeploy).** Set `CHARON_AUTH_RATELIMIT_ENABLED=false` and restart.
- **Partial.** C5 (all Cerberus hardening) is one independently revertible commit. C4 is a
  leaf package that C5 and C6 depend on. The trusted-proxy validation lives in C7.
- **E2E flakes from throttling.** Raise only the E2E compose budgets. The hardened fixtures
  now fail loudly with a 429 message rather than flaking downstream.
- **Noisy detector.** The WARN is already capped; a follow-up can downgrade it without touching enforcement.

---

## 7. Risks & Mitigations

| # | Risk | Likelihood / Impact | Mitigation |
| --- | --- | --- | --- |
| RK1 | **Untrusted proxy in front of the UI**: all visitors share one key, so one client can keep sign-in closed for every account at ~1 req/min (§3.5.3) | Med / **High** | Detector (count, last-seen, scope), admin card, capped scope-aware WARN, docs, the login-page admin pointer, short `Retry-After`, unthrottled break-glass, kill switch. Device cookies are the first follow-up (M2) |
| RK2 | **Runtime NAT** (rootless or VM runtimes, SNAT load balancers): the same shared-key effect, with **no** header signal (§3.5.2) | Med (common in rootless/desktop setups) / **High** | Verified runtime remedies in docs; the admin card's "Charon sees your browser as" self-check; host-level proxy + trusted proxies; kill switch; M2 |
| RK3 | E2E flakiness from the shared runner address | High if unmitigated / Med | Explicit E2E-only budgets; XFF isolation proven by the effectiveness probe; fixtures fail loudly on 429 |
| RK4 | Memory exhaustion via key rotation | Low / Med | 10,000-key LRU cap, IPv6 /64, no goroutines |
| RK5 | Header spoofing to evade the throttle | Low / High | Gin ignores untrusted peers' headers; right-to-left XFF walk; trust-all WARN; docs require append/overwrite XFF and exact addresses |
| RK6 | Operator trusts broad ranges, so any client in them chooses its key | Med / Med | Docs guidance; Q5 |
| RK7 | C5's exemption fix adds Cerberus-limiter pressure in the near-margin security E2E project | Med / Med | Valid admin tokens stay exempt; watch the PR's "E2E Firefox Security" job; adjust E2E-side traffic only |
| RK8 | Forward-auth wired later without keeping `/auth/verify` exempt | Low / High | The inventory test locks the classification |
| RK9 | A future password check lands unguarded | Med / Med | Tripwire test (§4.4) |
| RK10 | Behavior change for invalid trusted-proxy lists (cookie HTTPS detection now also trusts nothing) | Low / Low | Startup WARN names the entry; documented in the release notes |
| RK11 | `Retry-After` float rounding off-by-one | Low / Low | ms rounding; table tests |
| RK12 | Config typos silently disable protection | Low / High | Only literal `false` disables; unrecognized values WARN; the zero value is enabled |
| RK13 | Attacker-driven log floods | Med / Low | Global WARN cap with a suppressed count; episode logging; detector capped at one per 15 min |
| RK14 | DB-write amplification | — | No audit rows |
| RK15 | New dependency (golang-lru/v2, MPL-2.0) | Low / Low | Zero transitive deps; license class already shipped; fallback (Q2) |
| RK16 | Restart resets buckets | Low / Low | Operator-controlled; the lockout layer persists |
| RK17 | The E2E trust list changes cookie Host handling if a non-IP base URL is used | Low / Med | The compose comment requires IP-literal or `localhost` base URLs; no spec sends forwarded headers |
| RK18 | Admin endpoint exposes peer addresses | Low / Low | Admin-only (`securityAdmin`); RBAC route test |
| RK19 | `go.work.sum` churn from dependency commands | Med / Low | Review in C4 |

---

## 8. Open Questions & Maintainer Decisions

All items below are **decided**: the maintainer approved every recommended default on 2026-09-24.

### 8.1 Questions (decided)

1. **Q1: Login budget.** **Decided:** approved as recommended — 10 per 600 s (burst 10, +1/min), matching Vaultwarden.
   The stricter alternative is 5 per 300 s.
2. **Q2: Store dependency.** **Decided:** approved as recommended — `hashicorp/golang-lru/v2` (`simplelru`). The alternative
   is a ~50-line `container/list` LRU.
3. **Q3: Cerberus hardening in this PR.** **Decided:** approved as recommended — keeping it, now folded into one commit
   (C5). The alternative is a separate `fix(security)` PR.
4. **Q4: Throttle-event notifications.** **Decided:** approved as recommended — deferring (§9).
5. **Q5: Trusted-proxy guardrails.** **Decided:** approved as recommended — a WARN only for trust-all lists (Gin
   semantics), plus docs guidance for broad RFC 1918 ranges.
6. **Q6: E2E trust model.** **Decided:** approved as recommended — trusting the runner-peer ranges in both Playwright
   stacks (now including `100.64.0.0/10` and `fc00::/7`), test-only.

### 8.2 Maintainer decisions (decided)

- **M1: Default-trust loopback in the container image.** **Decided:** approved as recommended — **not in this PR**; it
  becomes its own issue. Changing the default trust changes client-address resolution for
  every IP-based decision in the application, not just login protection. It also reverses
  #1316's trust-nothing default, so it deserves separate review.
- **M2: Device cookies.** **Decided:** approved as recommended — making per-browser buckets alongside per-IP buckets the
  **first** follow-up (§9.1).
- **M3: Security subject policy and merge subject.** **Decided:** approved as recommended — §6.2. The merge subject
  `feat(security): harden authentication endpoints against abuse (#1317) (#<PR>)` is the
  single user-facing Security entry, C5 is the single hardening entry, and C7 is plain
  `feat:`. The alternative is for C7 to carry `feat(security):` with a non-security PR title;
  that relies on title discipline at merge time and is less robust.
- **M8: Admin notice placement.** **Decided:** approved as recommended — the **Security-dashboard card** over a global
  admin banner, because:
  1. The Security page is the established home for security posture, and the card carries
     useful always-on information (budgets, the caller-address self-check), not just warnings.
  2. Anyone can trigger the detector by sending forged headers. A global banner would let any
     client put a warning on every admin page, while a card contains it and treats public
     peers as informational.
  3. It needs no global layout changes or dismissal persistence.

  The alternative is a dismissible global admin banner shown **only** for loopback/private
  peers (the strong misconfiguration signal), linking to the card.

---

## 9. Follow-ups (Out of Scope)

1. **Device cookies (first; M2).** Per-browser "device cookie" budgets alongside per-IP
   budgets, following the OWASP guidance "Slow Down Online Guessing Attacks with Device
   Cookies". A browser that has previously authenticated keeps its own budget, so shared-address
   setups (untrusted proxy, runtime NAT, CGNAT) don't lock out known users.
2. **Throttle-event notifications.** An async dispatcher (app `ctx`) behind a global
   `ratelimit`-based debounce (≤ 1 per class per 15 min, with a suppressed count), feeding
   `NotifySecurityRateLimitHits`. The toggle's semantics need deciding.
3. **Login-flow hardening.** Additional login-flow hardening items are tracked privately.
4. **Invite routes.** Optionally attach the `session` class to `/invite/*` for defense in depth.
5. **Forward-auth.** Decide whether to implement the forward-auth gateway (C10 corrects the docs).
6. **Emergency server trusted proxies.** See the existing
   `docs/issues/created/20260718-emergency-server-missing-trusted-proxies.md`.
7. **Metrics registration.** Register the `security_metrics.go` collectors on the served registry.
8. **Spec-reference sweep.** 132 `docs/plans/current_spec.md` references remain in code and
   tests after this PR, including the 5 in C1's files. Add a lefthook/CI guard against new ones.
9. **Certificate key export bug.** `POST /certificates/:uuid/export` with `include_key: true`
   always answers 403, because the handler reads a context value (`user`) that the auth
   middleware never sets (`certificate_handler.go:342`). File it and fix it separately; the
   §3.2.1 guard already precedes that branch.
10. **Docs links.** Migrate the existing hardcoded frontend docs links to `constants/docs.ts`,
    and verify their paths against the published site.
11. **Address-scope DRY.** Consolidate `auth_handler.go`'s CGNAT/private classification with
    `ratelimit.ClassifyAddr`. That change touches session-cookie code, so it is left out of
    this feature's scope.
12. **Configurable IPv6 aggregation.** Revisit only if abuse from delegated /56 or /48 blocks is observed.

---

## 10. Revision History

| Rev | Date | Changes |
| --- | --- | --- |
| 1 | 2026-09-24 | Initial spec |
| 2 | 2026-09-24 | Supervisor review. **B1:** shared-address analysis (untrusted proxy + runtime NAT), severity, admin-only endpoint + card, scope-aware guidance, compose hint, login-page admin pointer, device-cookie follow-up. **B2:** merge-commit publishing mechanism; subjects re-derived; Cerberus commits folded (C5); unwired commit → `refactor:`. **B3:** effective trust validation, effectiveness probe, E2E trust list, fixtures fail loudly. **B4:** password-verifying routes outside `/auth` guarded, plus a tripwire test. **B5:** unfixed-weakness details replaced by a private-tracking pointer; process note. **B6:** C1 gate scoped. Suggestions 1–17 adopted |
| 3 | 2026-09-24 | Supervisor approval (10 non-blocking items folded in) and maintainer approval of Q1–Q6, M1, M2, M3, M8 (marked decided). Trusted-proxy strings kept as given when valid; cookie trust uses the detector's `netip` matcher (no loopback equivalence); per-scope detector records; broader tripwire (all non-test files, any `.CheckPassword` selector, source-position ordering); `createUser` throws only on persistent 429; runtime-NAT specifics marked unverified with C10 re-verification; review-item references removed and Phase headings numbered 4.1–4.6; C1 gate asserts 5 named matches; Gin wording fixes; docs publish no later than the login notice |

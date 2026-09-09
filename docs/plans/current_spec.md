# Technical Spec — GHSA-3gc6-295r-xm5m Fix + Management-API Authorization Hardening + Retire Public Registration

**Status:** Draft for review (revised per coordinator rulings 2026-09-08)
**Advisory:** GHSA-3gc6-295r-xm5m — "Improper Authorization on CrowdSec Admin APIs via Public User Registration" (CWE-862, CVSS 8.8, reporter EQSTLab)
**Scope model:** ONE feature = ONE PR, delivered as an ordered sequence of logical commits (see [§9 Commit Slicing Strategy](#9-commit-slicing-strategy)). No PR splitting.
**Branch:** `development` (per `CLAUDE.md`: no worktrees, work on the current branch).

---

## 1. Introduction

### 1.1 Overview

A publicly reachable `POST /api/v1/auth/register` lets an anonymous attacker
create a `role=user` account. That account then reaches the entire
`/api/v1/admin/crowdsec/*` surface (~45 routes) because those routes are mounted
on the bare `management` router group, which is guarded only by
`RequireManagementAccess()` (rejects `role=passthrough` only — `role=user`
passes). Impact: bouncer API-key disclosure, disabling the IPS
(`POST /admin/crowdsec/stop` persists `SecurityConfig.Enabled=false`), ban
add/remove, and CrowdSec config-file read/write.

This feature:

- **Part A** — closes the authorization hole (the advisory fix): mount the
  CrowdSec admin routes behind an explicit `RequireRole(admin)` subgroup,
  mirroring the existing `securityAdmin` pattern.
- **Part B** — audits every route on the `management` group for the same class
  of bug, fixes each under-guarded route found (at minimum: the
  `/admin/plugins` mutation routes, a confirmed second live instance), and
  introduces a "deny-by-default" structural guard + enforcement test so a
  handler can no longer accidentally land privileged routes on an under-guarded
  group.
- **Part C** — **removes the public `POST /auth/register` endpoint entirely**
  (coordinator ruling). First-admin bootstrap continues via `POST /setup`;
  post-bootstrap account creation is served by the **existing** admin
  invite-user / email-invite flow (`User.InviteToken`,
  `UserHandler.InviteUser` / `ValidateInvite` / `AcceptInvite`,
  `frontend/src/pages/AcceptInvite.tsx`). No new invite model / service /
  endpoints / UI are built.

### 1.2 Objectives / Goals

1. A `role=user` (or unauthenticated) caller receives `403` on every CrowdSec
   admin route; `role=admin` is unaffected.
2. Every state-changing / privileged route on `management` is provably
   admin-guarded or is a deliberate, documented `role=user` capability, enforced
   by a CI test.
3. `POST /api/v1/auth/register` no longer exists — the route returns `404`.
4. First-admin bootstrap (`POST /setup`) and the existing email-invite
   acceptance flow (`GET /invite/validate`, `POST /invite/accept`) continue to
   work unchanged.
5. Backend coverage ≥ 85 %, frontend coverage ≥ 85 %, targeted E2E green, all
   Definition-of-Done gates pass.

### 1.3 Non-goals

- Any new invite mechanism, model, service, endpoint, or UI. (Earlier draft's
  `models.Invite` / `InviteService` / `InviteHandler` / `frontend/src/api/invites.ts` /
  `useInvites` / `UsersPage` invite section / `/register` page are **dropped**.)
- Changes to the existing per-user email-invite flow beyond referencing it as
  the supported post-bootstrap path.
- A general-purpose RBAC engine. The 3-tier model (`admin` / `user` /
  `passthrough`) is unchanged.
- Per-IP auth rate limiting (see [§7](#7-remaining-open-questions) — deferred to
  a follow-up issue; `/auth/register` is being removed and `/auth/login`
  already has account lockout).

---

## 2. Research Findings

### 2.1 Existing architecture (verified in-repo on `development`)

#### Auth / authorization primitives

| Element | Location | Behavior |
|---|---|---|
| `AuthMiddleware` | `backend/internal/api/middleware/auth.go` | Validates JWT / cookie, sets `c.Set("userID", …)` and `c.Set("role", string(user.Role))`. |
| `RequireManagementAccess()` | `backend/internal/api/middleware/auth.go:116` | **Only** aborts when `role == RolePassthrough`. `role=user` and `role=admin` pass. |
| `RequireRole(role)` | `backend/internal/api/middleware/auth.go` | Aborts `401` if no role; aborts `403` unless `userRole == role` **or** `userRole == RoleAdmin`. So `RequireRole(RoleAdmin)` ⇒ admin-only, and `RequireRole(anything)` still lets admin through. |
| `requireAdmin(c)` / `isAdmin(c)` | `backend/internal/api/handlers/permission_helpers.go` | In-handler guard. `isAdmin` = `c.GetString("role") == "admin"`. `requireAdmin` writes `403 {"error":"admin privileges required","error_code":"permissions_admin_only"}`. |
| `rejectPassthrough(c, action)` | `backend/internal/api/handlers/user_handler.go:225` | In-handler 403 for passthrough. |
| Roles | `backend/internal/models/user.go` | `RoleAdmin="admin"`, `RoleUser="user"`, `RolePassthrough="passthrough"`. `RoleUser` doc: "can access the Charon management UI with restricted permissions" (restriction is per-host `PermittedHosts`, not per-feature). |

#### Route groups — `backend/internal/api/routes/routes.go`

```
api            := router.Group("/api/v1")                            // public
  api.POST("/auth/login", …)
  api.POST("/auth/register", authHandler.Register)                   // line 295 — PUBLIC, no gate      ← ADVISORY (Part C removes)
  api.GET("/setup", …) / api.POST("/setup", …)                       // bootstrap first admin (Part C keeps)
  api.GET("/invite/validate", …) / api.POST("/invite/accept", …)     // existing email-invite (Part C references as supported path)
  protected  := api.Group("/"); protected.Use(authMiddleware)         // any authenticated user
    management := protected.Group("/")
    management.Use(middleware.RequireManagementAccess())               // line 373-374 — passthrough-only reject
      securityAdmin := management.Group("/security")
      securityAdmin.Use(middleware.RequireRole(models.RoleAdmin))      // line 796-797 — CORRECT admin gate (template)
      adminEncryption := management.Group("/admin/encryption")          // line 546 — no RequireRole, BUT every handler calls isAdmin(c)
      adminPlugins    := management.Group("/admin/plugins")             // line 560 — no RequireRole AND plugin_handler has NO admin check ← BUG (Part B)
      crowdsecHandler.RegisterRoutes(management)                        // line 838 — no RequireRole AND crowdsec_handler has NO admin check ← ADVISORY (Part A)
      … ~20 other *.RegisterRoutes(management) / inline management.* …
RegisterImportHandler(…) {                                            // separate func, line 1005
  authenticatedAdmin := api.Group("/")
  authenticatedAdmin.Use(AuthMiddleware(authService), RequireRole(models.RoleAdmin))  // line 1011-1012 — CORRECT admin gate (2nd template / name precedent)
}
```

Verified line numbers (grep, `development` HEAD): `auth/register` route `:295`,
`management := protected.Group("/")` `:373`, `adminPlugins` `:560`,
`securityAdmin` `:796`, `crowdsecHandler.RegisterRoutes(management)` `:838`.

#### In-handler admin-check audit (grep `requireAdmin(|isAdmin(|RoleAdmin|GetString("role")|rejectPassthrough`, non-test)

| Handler | In-handler role refs | Mounted on | Effective guard for `role=user` |
|---|---|---|---|
| `crowdsec_handler.go` | **0** | `management` (bare) | **NONE — vulnerable** (advisory) |
| `plugin_handler.go` | **0** | `management.Group("/admin/plugins")` (bare) | **NONE — mutations vulnerable** (Part B) |
| `encryption_handler.go` | 4 (`isAdmin`) | `management.Group("/admin/encryption")` (bare) | OK (in-handler) |
| `docker_handler.go` | 0 | `management` | none — read-only (`GET /docker/containers`, used by proxy-host create) |
| `proxy_host_handler.go` / `proxy_group_handler.go` | 0 | `management` | none — intended `role=user` capability |
| `remote_server_handler.go` | 0 | `management` | none |
| `security_headers_handler.go` | 0 | `management.Group("/security/headers")` | none |
| `hecate_handler.go` / `orthrus_handler.go` | 0 | `management` | none |
| `manual_challenge_handler.go` | 0 | `management` (`/dns-providers/:id/...`) | none |
| `settings_handler.go` | 8 | `management` + one `RequireRole` arg on `GET /settings/smtp` (`:457`) | partial in-handler |
| `system_permissions_handler.go` | 3 | `management` | in-handler |
| `certificate_handler.go`, `access_list_handler.go`, `domain_handler.go`, `uptime_handler.go`, `stats_handler.go`, `feature_flags_handler.go`, `audit_log_handler.go` | 0 | `management` | none — per-route verdict in §3.2 |
| `notification_provider_handler.go` | 3 (`requireAdmin` — `Create`/`Update`/`Delete` only; **`Test` & `Preview` are NOT guarded**) | `management` | mutations OK in-handler; `POST /notifications/providers/test` + `/preview` unguarded → see table #33b |
| `notification_template_handler.go` | 3 (`requireAdmin` — `Create`/`Update`/`Delete` only; **`Preview` NOT guarded**) | `management` | mutations OK in-handler; `POST /notifications/external-templates/preview` unguarded → see table #33b |
| `security_notifications.go` | 2 (`requireAdmin` — `GetSettings`/`UpdateSettings`) | `management` | OK (in-handler) |
| `notification_handler.go` (per-user inbox) | 0 | `management` | none — USER-OK (list / mark-read) |
| `security_handler.go` | many (`requireAdmin`) | reads on `management`, writes on `securityAdmin` | OK |
| `backup_handler.go` / `backup_remote_handler.go` | many (`requireAdmin`) | `management` | OK (in-handler) |
| `user_handler.go` | many (`requireAdmin` / `rejectPassthrough`) | `management` | OK (in-handler; `UpdateUser` deliberately allows non-admin self-service) |

**Conclusion:** `management` is a de-facto "any authenticated non-passthrough
user" group; admin enforcement is applied inconsistently by three mechanisms
(dedicated subgroup, per-route middleware arg, in-handler `requireAdmin`). Two
areas — CrowdSec (all) and Plugins (mutations) — have **no** enforcement.

#### Frontend route/nav gating — `frontend/src/App.tsx`, `frontend/src/components/Layout.tsx`

- The SPA has **almost no role gating**. `App.tsx` wraps only:
  - `/settings/*` in `<RequireRole allowed={['admin','user']}>`
  - `/settings/users` in `<RequireRole allowed={['admin']}>`
  - Everything else under `/` (`/security/*`, `/access-lists`, `/dns/*`,
    `/hecate/*`, `/certificates`, `/security/audit-logs`, `/security/crowdsec`,
    …) is reachable by any authenticated non-passthrough user, incl. `role=user`.
- `Layout.tsx` nav: only the **"Users"** entry is `role === 'admin'`-gated
  (`:127`); passthrough sees no nav (`:151`); `uptime` / `cerberus` sections are
  feature-flag gated. So a `role=user` today sees and can open CrowdSec config,
  Access Lists, Security Headers, DNS providers, Certificates, Hecate, Audit
  Logs, etc., and those pages call their APIs successfully because
  `management` doesn't stop them.
- `RequireRole` component: `frontend/src/components/RequireRole.tsx` — renders
  children if `user.role ∈ allowed`, else redirects. Ready to reuse.
- Public routes: `/login`, `/setup`, `/accept-invite` only. **No signup/register
  page or route exists.** `grep "auth/register"` in `frontend/src` → 0 hits.
  The endpoint is unused by the UI.

**Implication for Part B (Q7 ruling):** because non-admin screens currently
consume many of these read endpoints, moving a *read/list* endpoint to
admin-only would regress a `role=user` page. The classification in §3.2 is
therefore **mutation-oriented**: reads/lists that back a `role=user`-reachable
page stay on `management`; mutations move behind `RequireRole(admin)` (via a
per-route arg, or a `RegisterRoutes(read, admin)` split where the handler
registers its own routes). Only **CrowdSec** (forced by Part A — no
`role=user` read need) moves *wholesale* to `managementAdmin`. Hecate, Orthrus
and Remote Servers each keep a small set of `GET` reads on `management`
(consumed by the proxy-host create/edit flow and the Dashboard) and move only
their mutations — verified against `frontend/src` (§3.2.1 C1/C2). Where a page
becomes admin-only in practice (CrowdSec, Audit Logs, the Orthrus
agent-management page, Encryption) a **companion frontend `RequireRole` guard +
nav filter** is added (mirroring the existing "Users" pattern) so `role=user`
never lands on a 403-ing page.

#### `/auth/register` and `/setup` — how the first admin is created

- `authHandler.Register` — `backend/internal/api/handlers/auth_handler.go:244`;
  `RegisterRequest{Email,Password,Name}` (`min=8` password) at `:238`. Calls
  `h.authService.Register(req.Email, req.Password, req.Name)` at `:251`, returns
  `201` + user JSON. **No gating of any kind.**
- `authService.Register(email, password, name)` —
  `backend/internal/services/auth_service.go:31`: `count == 0` ⇒ `RoleAdmin`,
  else `RoleUser`. No toggle / invite / flag.
- **`POST /setup` does NOT call `authService.Register`.**
  `UserHandler.Setup` (`backend/internal/api/handlers/user_handler.go:141`)
  builds `models.User{Role: models.RoleAdmin, …}` directly and `tx.Create(&user)`
  inside its own transaction (also writes `caddy.acme_email`). It is fully
  independent of `authService.Register` / `authHandler.Register`.
- **`authService.Register` is NOT dead after removing the route.** grep
  `\.Register(` (non-`metrics`/`tracker`/`dnsprovider`) — it is called from
  ~28 test sites as a user-creation helper:
  - `backend/internal/services/auth_service_test.go` (16 calls — incl. the
    `count==0 → RoleAdmin` behavior test, `TestAuthService_Register*`)
  - `backend/internal/api/middleware/auth_test.go` (12 calls)
  - `backend/internal/api/handlers/user_integration_test.go:52`
- **`authHandler.Register` (HTTP handler) references:** only
  `routes.go:295` (the route) and
  `backend/internal/api/handlers/additional_coverage_test.go:729`
  (`TestAuthHandler_Register_InvalidJSON` — a 400-on-bad-JSON coverage test).
- **Test / inventory references to the route path** (`grep "auth/register"`):
  - `backend/internal/api/routes/routes_test.go:162` — `expectedRoutes` list in
    `TestRegister_RoutesRegistration`
  - `backend/internal/api/routes/routes_test.go:215` — `publicMutationAllowlist`
    in `TestRegister_StateChangingRoutesDenyByDefaultWithExplicitAllowlist`
  - `backend/internal/api/routes/routes_test.go:335` —
    `assert.Contains(t, routeMap, "/api/v1/auth/register")` in
    `TestRegister_AllRoutesRegistered`
  - `backend/integration/crowdsec_lapi_integration_test.go:59` — `authenticate()`
    helper POSTs `/api/v1/auth/register` (errors ignored) to bootstrap a test
    user; build-tagged integration test, not in default CI.

⇒ **Part C deletions are exactly:** the route (`routes.go:295`),
`AuthHandler.Register` (`auth_handler.go:244-256`), `RegisterRequest`
(`auth_handler.go:238-242`). **Keep** `AuthService.Register` (+ its
`count==0 → RoleAdmin` logic) — still referenced by ~28 test call sites as a
helper. Update the 4 test references above.

#### Existing email-invite flow (unchanged — the supported post-bootstrap path)

- `models.User` fields (`backend/internal/models/user.go`): `InviteToken`
  (`json:"-"`, `gorm:"index"`), `InviteExpires`, `InvitedAt`, `InvitedBy`,
  `InviteStatus` (`"pending"|"accepted"|"expired"`); helper
  `User.HasPendingInvite()`.
- `UserHandler.InviteUser` (`POST /users/invite`, admin — `requireAdmin`),
  `ResendInvite` (`POST /users/:id/resend-invite`), `PreviewInviteURL`,
  `ValidateInvite` (`GET /invite/validate`, public), `AcceptInvite`
  (`POST /invite/accept`, public). `generateSecureToken()` at
  `user_handler.go:494` (`crypto/rand` 32B → hex).
- Frontend: `frontend/src/pages/AcceptInvite.tsx` (route `/accept-invite`, reads
  `?token=`), `frontend/src/api/users.ts`
  (`inviteUser`/`validateInvite`/`acceptInvite`/`resendInvite`/`previewInviteURL`),
  `frontend/src/pages/UsersPage.tsx` (`/settings/users`, admin-gated).

#### AutoMigrate

`backend/internal/api/routes/routes.go:112` — single `db.AutoMigrate(&models.X{}, …)`
call. **No new models in this feature**, so no change here.

#### Test patterns

- `backend/internal/api/routes/routes_test.go`:
  - `TestRegister_AllRoutesRegistered` (`:310`) — asserts `routeMap` contains
    `/api/v1/admin/crowdsec/*` and `/api/v1/auth/register`.
  - `TestRegister_AdminRoutes` (`:481`) — GET admin paths expecting `401`
    unauthenticated.
  - `TestRegister_StateChangingRoutesDenyByDefaultWithExplicitAllowlist`
    (`:196`) — iterates every mutating `/api/v1/*` route, asserts `401|403`
    unless in `publicMutationAllowlist`. **This is the Part B harness** — extend
    it with a `role=user` dimension and remove the `auth/register` allowlist
    entry.
- E2E: `tests/security-enforcement/authorization-rbac.spec.ts` &
  `auth-api-enforcement.spec.ts` — `loginAndGetToken(context, {email,password})`
  vs `TEST_USERS.admin` / `TEST_USERS.user`; assert `role=user` → `403` on
  privileged routes. Playwright projects: `security-tests` (CI shard),
  `firefox` (local DoD, single browser).

### 2.2 Docs to update

| Doc | Why |
|---|---|
| `ARCHITECTURE.md` → "Security Architecture" / "Authentication & Authorization" | New `managementAdmin` authorization boundary; public registration removed; bootstrap-via-`/setup` + email-invite is the account-creation model. |
| `SECURITY.md` → "Authentication & Authorization" (~line 1148) | RBAC description: explicit admin-subgroup enforcement; no public self-registration. |
| `docs/security.md`, `docs/features/access-control.md` | User-facing: how accounts are created (first-run setup + admin invites), admin-only security surfaces. |
| `docs/features.md` | One-line touch if wording references self-registration. |
| `docs/features/crowdsec.md`, `docs/features/custom-plugins.md` / `plugin-security.md` | Note admin-only requirement (behavior clarification). |

### 2.3 External dependencies

None new. Stdlib + existing libs only.

---

## 3. Technical Specifications

### 3.1 Part A — Close the authorization hole (advisory fix)

#### 3.1.1 Structural change in `routes.go`

Declare one admin subgroup on `management`, immediately after `management` is
created (`routes.go:373-374`), named for consistency with the existing
`securityAdmin` / `authenticatedAdmin`:

```go
// management: any authenticated non-passthrough user (RequireManagementAccess).
management := protected.Group("/")
management.Use(middleware.RequireManagementAccess())

// managementAdmin: management routes that mutate or expose privileged
// infrastructure. Deny-by-default for role=user. Mirrors securityAdmin
// (routes.go ~§"Security module enable/disable") and authenticatedAdmin
// (RegisterImportHandler). Enforcement is the ONLY guard on these routes —
// no redundant in-handler requireAdmin (see spec §3.2 Q6 ruling).
managementAdmin := management.Group("/")
managementAdmin.Use(middleware.RequireRole(models.RoleAdmin))
```

Change `routes.go:838`:

```go
crowdsecHandler.RegisterRoutes(management)   →   crowdsecHandler.RegisterRoutes(managementAdmin)
```

`CrowdsecHandler.RegisterRoutes` is unchanged (it already prefixes every route
with `/admin/crowdsec/…`). The full path set is identical; only the middleware
chain gains `RequireRole(admin)`. **No in-handler `requireAdmin` is added to
`crowdsec_handler.go`** (Q6 ruling — subgroup-only, matching `securityAdmin`).

#### 3.1.2 Companion frontend guard (prevents a `role=user` dead page)

`role=user` can currently open `/security/crowdsec` (`CrowdSecConfig` page) and
its nav entry, which after Part A would 403 on every call. Add, in the same PR:

- `frontend/src/App.tsx` — wrap the `security/crowdsec` route element in
  `<RequireRole allowed={['admin']}>` (like `/settings/users`).
- `frontend/src/components/Layout.tsx` — gate the `navigation.crowdsec` child
  entry (`:112`) with `user?.role === 'admin'` (spread-in pattern, same as
  `:127` "Users").

#### 3.1.3 Error contract

`RequireRole(models.RoleAdmin)` already returns `401 {"error":"Unauthorized"}`
(no role) / `403 {"error":"Forbidden"}` (`role=user`/`passthrough`). No
middleware change. Matches the reporter PoC's expectation of a hard `403` for a
non-admin token.

#### 3.1.4 Regression tests — `backend/internal/api/routes/routes_test.go` (+ handler test)

New `TestRegister_CrowdsecAdminRoutesRequireAdminRole`:

| Case | Token role | Route | Expected |
|---|---|---|---|
| Control (PoC parity) | none | `POST /api/v1/admin/crowdsec/stop` | `401` |
| Escalation blocked | `user` | `POST /api/v1/admin/crowdsec/stop` | `403` |
| Escalation blocked | `user` | `GET /api/v1/admin/crowdsec/bouncer/key` | `403` |
| Escalation blocked | `user` | `POST /api/v1/admin/crowdsec/ban` | `403` |
| Escalation blocked | `user` | `GET /api/v1/admin/crowdsec/file?path=…` | `403` |
| Admin unaffected | `admin` | `GET /api/v1/admin/crowdsec/status` | not `401` / not `403` |

Harness: `Register(ctx, gin.New(), db, cfg)`; seed a `role=user` + a
`role=admin` user; mint JWTs via
`services.NewAuthService(db,cfg).GenerateToken(&user)`; send
`Authorization: Bearer …`. Reuse the in-memory sqlite + `cfg.JWTSecret` pattern
already in `routes_test.go`.

### 3.2 Part B — Audit & structurally harden the `management` group

#### 3.2.1 Rulings baked in

- **Q6 — belt-and-braces:** subgroup-only. Do **not** add in-handler
  `requireAdmin` to `crowdsec_handler.go` / `plugin_handler.go`. Match
  `securityAdmin` / `authenticatedAdmin` exactly.
- **Q7 — reads that back non-admin screens stay on `management`.** The frontend
  exposes nearly every management page to `role=user` (§2.1). So:
  classification is **mutation vs. read**, not endpoint-group. A `GET`/`list`
  that a `role=user`-reachable page calls is **READ (stays on `management`)**;
  its `POST`/`PUT`/`PATCH`/`DELETE` siblings move behind
  `RequireRole(admin)`. Where a whole capability is infra-admin **and no
  `role=user`-reachable screen consumes any of its reads**, the group moves
  wholesale **and** gets a companion `RequireRole` frontend guard + nav filter
  (like Part A does for CrowdSec).
- **Q8 — least-invasive split mechanism.** For routes registered inline in
  `routes.go`, add `middleware.RequireRole(models.RoleAdmin)` as a per-route
  2nd handler arg (exactly like the existing `routes.go:457`
  `management.GET("/settings/smtp", middleware.RequireRole(models.RoleAdmin), …)`).
  Where a handler's own `RegisterRoutes(rg)` registers a mix of read and
  mutation routes and only the mutations move, change that handler's signature
  to `RegisterRoutes(read, admin *gin.RouterGroup)` and register each route on
  the correct group.
  - **`HecateHandler`, `OrthrusHandler`, `RemoteServerHandler` — read/write
    split, NOT wholesale move** (C1/C2). Verified: `GET /orthrus/agents` is
    consumed by `frontend/src/components/hecate/ConnectionTypeSelector.tsx`
    (`useAgentList`, rendered inside the `role=user`-reachable proxy-host
    create/edit flow) and `GET /hecate/status` by
    `frontend/src/api/hecate.ts` (imported by `Dashboard.tsx`, route `/`, all
    roles). Reads that stay on `management`:
    `GET /hecate/status`, `GET /hecate/tunnels`, `GET /hecate/tunnels/:uuid`,
    `GET /orthrus/agents`, `GET /orthrus/agents/:uuid`,
    `GET /remote-servers`, `GET /remote-servers/:uuid`. Everything else on those
    three handlers (create/update/delete/start/stop/rotate-credentials/revoke/
    provision/patch/install-snippets/proxy-status/test/provider-device
    lists+sync) → `managementAdmin`. Each handler's `RegisterRoutes` takes
    `(read, admin *gin.RouterGroup)`.
  - **`SecurityHeadersHandler` — inline in `routes.go`, per-route args, NOT a
    bespoke signature** (C6). Its ~11 routes move out of
    `h.RegisterRoutes(management)` into explicit
    `management.GET/POST(...)` / `managementAdmin.POST/PUT/DELETE(...)` lines in
    `routes.go` (its siblings — certificates, access-lists, domains,
    feature-flags — are already registered inline this way). The
    `SecurityHeadersHandler.RegisterRoutes` method is removed.
  - **`CrowdsecHandler`** moves wholesale (Part A) — no `role=user` read need.
  - **`PluginHandler`, DNS/credential/manual-challenge, certificate,
    access-list, domain, settings, feature-flags, system-repair, notification
    test/preview** routes are all inline in `routes.go` → per-route
    `RequireRole(admin)` args.

#### 3.2.2 Route classification table

Verdicts: **MOVE-GROUP** = whole registration → `managementAdmin` + companion
frontend guard (CrowdSec only) · **MOVE → `managementAdmin`** = these specific
route(s) re-registered on `managementAdmin` (a read that, on review, no
`role=user` screen needs) · **ADMIN-ARG** = keep on `management`, add per-route
`RequireRole(admin)` to the mutations, reads stay (for handlers that register
their own routes, this is a `RegisterRoutes(read, admin)` split) · **READ
(stays)** = `GET`/list that a `role=user`-reachable page consumes, no change ·
**USER-OK** = stays on `management`, no change (add to the enforcement-test
allowlist if it is a non-mutating `POST`) · **KEEP (in-handler)** = already
guarded inside the handler, leave mechanism, verify test.

> The implementing engineer MUST re-run
> `grep -n "management\.\(GET\|POST\|PUT\|PATCH\|DELETE\)\|\.RegisterRoutes(management)" routes.go`
> against HEAD at implementation time and reconcile drift with this table in the
> PR description.

| # | Route(s) | Handler | Current guard | Verdict | Action |
|---|---|---|---|---|---|
| 1 | `POST/GET/DELETE /admin/crowdsec/*` (~45) | `CrowdsecHandler` | none | **MOVE-GROUP** | Part A: `RegisterRoutes(managementAdmin)` + frontend guard on `/security/crowdsec`. |
| 2 | `GET /admin/plugins`, `GET /admin/plugins/:id` | `PluginHandler` | none | **READ (stays)** | `/dns/plugins` page (`role=user`-reachable) lists plugins. Keep on `management`. |
| 3 | `POST /admin/plugins/:id/enable`, `/:id/disable`, `/reload` | `PluginHandler` | none | **ADMIN-ARG** | Add `middleware.RequireRole(models.RoleAdmin)` to these 3 inline registrations (`routes.go:562-565`). This closes the confirmed 2nd live instance. |
| 4 | `GET/POST/PUT/DELETE /admin/encryption/*` | `EncryptionHandler` | in-handler `isAdmin(c)` | **KEEP (in-handler)** + also move the `adminEncryption` group decl to `managementAdmin.Group("/admin/encryption")` for defense-in-depth (no behavior change; removes the "silent 200 if the in-handler check is ever dropped" risk). Verify existing tests. |
| 5 | `GET /security/status`, `/config`, `/decisions`, `/rulesets`, `/rate-limit/presets`, `/geoip/status`, `/waf/exclusions` | `SecurityHandler` (reads) | `management` | **READ (stays)** — security-posture visibility; `Security` dashboard is `role=user`-reachable. Document. |
| 6 | `securityAdmin.*` (all `POST /security/*`, module enable/disable, PATCH) | `SecurityHandler` (writes) | `securityAdmin` = `RequireRole(admin)` | **KEEP** — already correct; the template for this work. |
| 7 | `GET /security/headers/profiles`, `/profiles/:id`, `/presets`; `POST /score`, `/csp/validate`, `/csp/build` | `SecurityHeadersHandler` | `management` (`/security/headers` subgroup) | **USER-OK** — reads + pure calculators (the 3 `POST`s do not persist). `SecurityHeaders` page is `role=user`-reachable. Inline these on `management.GET/POST(...)` in `routes.go`; add the 3 calculator `POST`s to the enforcement-test allowlist. |
| 8 | `POST/PUT/DELETE /security/headers/profiles`, `POST /security/headers/presets/apply` | `SecurityHeadersHandler` | `management` | **ADMIN-ARG** — inline on `managementAdmin.POST/PUT/DELETE(...)` in `routes.go` (C6 — per-route, no bespoke 2-group `RegisterRoutes` signature; delete the `SecurityHeadersHandler.RegisterRoutes` method — its siblings are already registered inline). |
| 9 | `GET/POST/PUT/DELETE /proxy-hosts*`, bulk-update-{acl,group,security-headers} | `ProxyHostHandler` | `management` | **USER-OK** — core `role=user` capability; per-host authz via `PermittedHosts` / forward-auth. No change. |
| 10 | `GET/POST/PUT/DELETE /proxy-groups*` | `ProxyGroupHandler` | `management` | **USER-OK** — same rationale. No change. |
| 11 | `GET /remote-servers`, `GET /remote-servers/:uuid` | `RemoteServerHandler` | `management` | **READ (stays)** — proxy-host create/edit references remote servers; `RemoteServers` page is `role=user`-reachable. |
| 12 | `POST/PUT/DELETE /remote-servers*`, `POST /remote-servers/test`, `POST /remote-servers/:uuid/test` | `RemoteServerHandler` | `management` | **ADMIN-ARG** (C2) — SSH targets + credentials. `RemoteServerHandler.RegisterRoutes(read, admin *gin.RouterGroup)`: the 2 `GET`s (row 11) on `read`, these 5 on `admin`. |
| 13 | `GET /docker/containers` | `DockerHandler` | `management` | **READ (stays)** — proxy-host create picks a container. Read-only. |
| 14a | `hecate/*` — reads: `GET /hecate/status`, `GET /hecate/tunnels`, `GET /hecate/tunnels/:uuid` | `HecateHandler` | `management` | **READ (stays)** (C1) — `GET /hecate/status` is consumed by `frontend/src/api/hecate.ts` (imported by `Dashboard.tsx`, route `/`, all roles). `HecateHandler.RegisterRoutes(read, admin *gin.RouterGroup)`: these 3 on `read`. |
| 14b | `hecate/*` — mutations: tunnels create/update/delete, `:uuid/start`, `:uuid/stop`, `:uuid/rotate-credentials`, `cloudflare/tunnels`, `:uuid/config/cloudflared`, `tailscale/devices`+`sync`, `zerotier/networks`(+members), `netbird/peers`+`sync` | `HecateHandler` | `management` | **ADMIN-ARG** (C1) — tunnel-provider credentials + network topology. All non-`read` `HecateHandler` routes go on the `admin` group. Frontend: no nav/route guard change — `/hecate/tunnels` etc. stay visible to `role=user` (list loads; create/edit controls 403), same as Access Lists. |
| 15a | `orthrus/agents` — reads: `GET /orthrus/agents`, `GET /orthrus/agents/:uuid` | `OrthrusHandler` | `management` | **READ (stays)** (C1) — `GET /orthrus/agents` is consumed by `frontend/src/components/hecate/ConnectionTypeSelector.tsx` (`useAgentList`), rendered inside the `role=user`-reachable proxy-host create/edit flow. `OrthrusHandler.RegisterRoutes(read, admin *gin.RouterGroup)`: these 2 on `read`. |
| 15b | `orthrus/agents` — mutations + detail: `POST /orthrus/agents`, `PATCH /:uuid`, `DELETE /:uuid`, `POST /:uuid/revoke`, `GET /:uuid/snippets`, `GET /:uuid/proxy-status` | `OrthrusHandler` | `management` | **ADMIN-ARG** (C1) — agent provisioning = trust-boundary expansion; install snippets embed a bootstrap token. All non-`read` `OrthrusHandler` routes on the `admin` group. Frontend: keep `RequireRole allowed={['admin']}` on `/hecate/agent` + its nav child (the agent-management page is admin-only; the read used by the proxy-host form is not gated). |
| 16 | `GET /dns-providers`, `/dns-providers/types`, `/dns-providers/:id`, `/dns-providers/detection-patterns` | `DNSProviderHandler`, `DNSDetectionHandler` | `management` (inside `if cfg.EncryptionKey != ""`) | **READ (stays)** — `DNSProviders` page is `role=user`-reachable and lists providers. |
| 17 | `GET /dns-providers/:id/audit-logs` (`auditLogHandler.ListByProvider`, `routes.go:521`) | `AuditLogHandler` | `management` | **MOVE → `managementAdmin`** (C5) — same actor-PII concern as row 28. Move this single `GET` to `managementAdmin`. (`DNSProviders` page does not surface per-provider audit logs to non-admins.) |
| 17b | `POST/PUT/DELETE /dns-providers*`, `POST /dns-providers/:id/test`, **`POST /dns-providers/test`** (id-less `TestCredentials`, `routes.go:519`), `POST /dns-providers/detect`, all `/:id/credentials*` (incl. `/:cred_id/test`), `POST /:id/enable-multi-credentials`, all `/dns-providers/:id/manual-challenge(s)*` | `DNSProviderHandler`, `CredentialHandler`, `ManualChallengeHandler` | `management` | **ADMIN-ARG** (C7) — DNS API credentials + ACME control. Inline registrations → per-route `RequireRole(admin)` arg; `ManualChallengeHandler.RegisterRoutes` → pass `managementAdmin` (all 6 routes are provider-mutation-adjacent; no `role=user` read need). Name both `POST /dns-providers/:id/test` **and** `POST /dns-providers/test` explicitly. |
| 18 | `GET /certificates`, `GET /certificates/:uuid` | `CertificateHandler` | `management` | **READ (stays)** — `Certificates` page is `role=user`-reachable. |
| 19 | `POST /certificates`, `POST /certificates/validate`, `PUT /certificates/:uuid`, `POST /certificates/:uuid/export`, `DELETE /certificates/:uuid` | `CertificateHandler` | `management` | **ADMIN-ARG** — `/export` returns private-key material. Per-route `RequireRole(admin)` args. |
| 20 | `GET /access-lists`, `/access-lists/:id`, `/access-lists/templates`, `POST /access-lists/:id/test` | `AccessListHandler` | `management` | **USER-OK** — `AccessLists` page is `role=user`-reachable; `/test` is a non-persisting dry-run IP check. Reads stay; add `POST /:id/test` to the enforcement-test allowlist. |
| 21 | `POST/PUT/DELETE /access-lists*` | `AccessListHandler` | `management` | **ADMIN-ARG** — ACLs are a security control. Per-route `RequireRole(admin)` args. |
| 22 | `GET /settings`, `GET /feature-flags`, `GET /themes` | `SettingsHandler`, `FeatureFlagsHandler`, `CustomThemeHandler` | `management` | **READ (stays)** — the SPA loads these for every role (`Layout.tsx` uses `getSettings`). |
| 23 | `POST/PATCH /settings`, `PATCH /config`, `POST/DELETE /settings/logo`, `/settings/banner`, `GET/POST /settings/smtp*`, `POST /settings/validate-url`, `/settings/test-url` | `SettingsHandler` | mixed (1 `RequireRole` arg, 8 in-handler refs) | **ADMIN-ARG** — normalize: per-route `RequireRole(admin)` arg on every settings mutation + `GET /settings/smtp` (keep its existing arg). Keep in-handler checks as belt-and-braces (do not remove — they predate this and some tests assert them). |
| 24 | `PUT /feature-flags` | `FeatureFlagsHandler` | `management` | **ADMIN-ARG** — per-route arg; `GET` stays. |
| 25 | `GET/POST/PUT/DELETE /themes` | `CustomThemeHandler` | `management` | **USER-OK** — code comment: "available to all management users (not admin-only)". No change; document. |
| 26 | `backups*`, `backups/remote-targets*` | `BackupHandler`, `BackupRemoteHandler` | `management` + in-handler `requireAdmin` on every mutation | **KEEP (in-handler)** — verify each mutation path has a `requireAdmin` test; no structural move required. |
| 27 | `users*` (`GET/POST/PUT/DELETE /users`, `/invite`, `/preview-invite-url`, `/permissions`, `/resend-invite`) | `UserHandler` | `management` + in-handler `requireAdmin` (except `UpdateUser` self-service branch) | **KEEP (in-handler)** — `UpdateUser` deliberately allows a non-admin to change their own name/password, so it cannot move wholesale. Verify tests cover the admin-only branches. |
| 28 | `GET /audit-logs`, `GET /audit-logs/:uuid` (`routes.go:428-429`) | `AuditLogHandler` | `management` | **MOVE → `managementAdmin`** (C4) — audit records expose other users' emails, source IPs, and security-event detail (info disclosure to a lower-privilege role). Both `GET`s → `managementAdmin`. Frontend: wrap the `/security/audit-logs` route element in `<RequireRole allowed={['admin']}>` (`App.tsx:104`). No dedicated nav entry exists for it (`Layout.tsx` `cerberus` children do not include audit-logs), so no nav filter needed; if the `Security` dashboard renders an in-page link to it, hide that link for non-admins (optional polish). |
| 29 | `GET /domains` | `DomainHandler` | `management` | **READ (stays)** — `Domains` page is `role=user`-reachable. |
| 30 | `POST /domains`, `DELETE /domains/:id` | `DomainHandler` | `management` | **ADMIN-ARG** — per-route `RequireRole(admin)` args. |
| 31 | `system/permissions*` (`GET`, `POST /repair`), `GET /system/updates`, `GET /system/my-ip`, `POST /system/uptime/check`, `POST /system/uptime/*` | `SystemPermissionsHandler`, `UpdateHandler`, `SystemHandler` | `management` (+ 3 in-handler refs in system-permissions) | **ADMIN-ARG** for `POST /system/permissions/repair` (arg) — keep `GET /system/permissions` as READ; `GET /system/updates`, `GET /system/my-ip` **USER-OK**; `POST /system/uptime/check` **USER-OK** (observability). |
| 32 | `uptime/monitors*`, `stats/*`, `cerberus/logs/ws`, `logs*`, `websocket/*` | various | `management` | **USER-OK** — observability / read. WS auth already via `AuthMiddleware`. No change; a few non-mutating `POST`s (`/uptime/sync`, `/uptime/monitors/:id/check`) — **allowlist**. |
| 33a | `notifications*` — `POST/PUT/DELETE /notifications/providers*`, `.../external-templates*` (Create/Update/Delete); `GET/PUT /notifications/settings/security` | `NotificationProviderHandler`, `NotificationTemplateHandler`, `SecurityNotificationHandler` | `management` + in-handler `requireAdmin` (verified: `notification_provider_handler.go` ×3, `notification_template_handler.go` ×3, `security_notifications.go` ×2) | **KEEP (in-handler)** — already guarded on Create/Update/Delete + settings. Verify tests. |
| 33b | `POST /notifications/providers/test` (`routes.go:658`), `POST /notifications/providers/preview` (`:659`), `POST /notifications/external-templates/preview` (`:668`) | `NotificationProviderHandler.Test`/`.Preview`, `NotificationTemplateHandler.Preview` | `management` | **ADMIN-ARG** (C3) — **verified NO in-handler `requireAdmin`** on `Test`/`Preview` (only Create/Update/Delete). These send test messages / render templates with provider config → admin-only. Add `middleware.RequireRole(models.RoleAdmin)` per-route arg. (Without this, the new enforcement test asserts 403 for `role=user` and fails with no guidance.) |
| 33c | `GET /notifications`, `POST /notifications/:id/read`, `POST /notifications/read-all` | `NotificationHandler` | `management` | **USER-OK** — per-user inbox. Allowlist the 2 read-state `POST`s. |
| 34 | `import` / NPM / JSON import (`RegisterImportHandler`) | `ImportHandler` etc. | `authenticatedAdmin` = `RequireRole(admin)` | **KEEP** — already correct. |

**Companion frontend guards added by Part B** (mirroring the "Users" pattern —
`<RequireRole allowed={['admin']}>` on the route element + `user?.role === 'admin'`
spread on the nav entry). Required:

- `/security/crowdsec` route + `navigation.crowdsec` nav child (Part A / row 1).
- `/security/audit-logs` route (C4 / row 28). No nav entry exists for it —
  route guard only; optionally hide any in-page link from the `Security`
  dashboard for non-admins.
- `/hecate/agent` route + its nav child (rows 15a/15b) — the Orthrus
  *agent-management page* is admin-only; the `GET /orthrus/agents` read used by
  the proxy-host form stays ungated so `ConnectionTypeSelector` still works for
  `role=user`.
- `/security/encryption` route + `navigation.encryption` nav child — already
  effectively admin via in-handler `isAdmin(c)`; add the guard for UX parity
  (row 4).

**NOT guarded** (pages stay visible to `role=user`; reads succeed, mutation
controls 403): Access Lists, Security Headers, DNS Providers, Certificates,
Domains, Remote Servers, and the Hecate *tunnels* page (`/hecate/tunnels`,
`/hecate/providers`). Optional follow-up ([§7](#7-remaining-open-questions)):
hide the disabled create/edit/delete controls on these pages for non-admins.
Do **not** guard `navigation.hecate` wholesale — its `remote-servers` and
`tunnels` children remain `role=user`-usable for reads.

#### 3.2.3 Recommended structural fix (chosen) vs. alternative

**Chosen:** one `managementAdmin := management.Group("/"); .Use(RequireRole(admin))`
subgroup (Part A) + the per-route/per-group moves in the table + a
deny-by-default enforcement test. Identical idiom to `securityAdmin` /
`authenticatedAdmin`. DRY.

**Rejected as the sole mechanism:** a pure per-route `RequireRole` sweep with no
subgroup — that is exactly the opt-in model that produced this advisory
(`crowdsecHandler.RegisterRoutes` can't take per-route middleware without a
signature change and would still land ~45 routes on a bare group). We use the
per-route form only for the individually-registered mutations that sit next to
USER-OK reads (Q8).

#### 3.2.4 New enforcement test — `routes_test.go`

`TestManagementGroup_MutationsAreAdminGuarded`:

```
build router via Register(...); seed role=user and role=admin; mint JWTs.
for each route in router.Routes() where path starts /api/v1/ and method ∈ {POST,PUT,PATCH,DELETE}:
    if route in PUBLIC_MUTATION_ALLOWLIST:  continue   // login, setup, invite/accept, security/events, emergency/security-reset  (NOTE: auth/register REMOVED)
    if route in USER_OK_MUTATION_ALLOWLIST: continue   // proxy-hosts*, proxy-groups*, themes*, security-headers calculators (POST /score,/csp/validate,/csp/build), access-lists/:id/test, uptime sync + monitors/:id/check, notifications/:id/read + read-all, user self-service (PUT /users/:id), remote-servers reads are GET (not here)
    send request with a valid role=user JWT  → assert 403     // deny-by-default
    send request with a valid role=admin JWT → assert != 403  // admin reaches handler
```

- Both allowlists are committed as explicit constants with a per-entry comment —
  this list **is** the deny-by-default policy and is what the supervisor
  reviews.
- Routes explicitly classified ADMIN-ARG in §3.2.2 that a reviewer might
  otherwise expect on the allowlist (so they are **not** allowlisted and MUST
  return 403 for `role=user`): `POST /notifications/providers/test`,
  `POST /notifications/providers/preview`,
  `POST /notifications/external-templates/preview` (C3);
  `POST /dns-providers/test` + `POST /dns-providers/:id/test` (C7);
  all Hecate mutation routes and all Orthrus mutation routes (C1). If any of
  these lands on the bare `management` group at implementation time the test
  fails — that is the intended tripwire.
- Also update `TestRegister_StateChangingRoutesDenyByDefaultWithExplicitAllowlist`:
  remove the `POST /api/v1/auth/register` entry from its `publicMutationAllowlist`
  (route no longer exists — see Part C).

### 3.3 Part C — Retire the public registration endpoint

#### 3.3.1 Behavior change

| Before | After |
|---|---|
| `POST /api/v1/auth/register {email,password,name}` → `201` (first caller `role=admin`, rest `role=user`) | Route does not exist → **`404`**. |
| First admin via `POST /api/v1/setup` | **Unchanged.** |
| Additional users: (undocumented) public register, or admin `POST /users` / `POST /users/invite` → `/accept-invite` | **Only** admin `POST /users` (direct create) or `POST /users/invite` → `GET /invite/validate` → `POST /invite/accept` (`/accept-invite` page). |

No behavior change to `/setup`, `/users*`, `/invite/*`, `/auth/login`,
`/auth/logout`, `/auth/refresh`, `/auth/me`, `/auth/change-password`.

#### 3.3.2 Backend deletions & edits (exact)

**Delete:**

- `backend/internal/api/routes/routes.go:295` — the line
  `api.POST("/auth/register", authHandler.Register)`.
- `backend/internal/api/handlers/auth_handler.go` — `func (h *AuthHandler) Register`
  (`:244-256`) and `type RegisterRequest struct` (`:238-242`). Remove any imports
  that become unused as a result (compiler / `staticcheck` will flag).

**Keep (do NOT delete — still referenced):**

- `backend/internal/services/auth_service.go` — `func (s *AuthService) Register`
  and its `count == 0 ⇒ RoleAdmin` logic. Referenced by ~28 test call sites
  (`auth_service_test.go`, `middleware/auth_test.go`,
  `handlers/user_integration_test.go`) as a user-creation helper. Add a doc
  comment noting it is now an internal/test helper with no HTTP surface.

**Edit (test references to the removed route):**

- `backend/internal/api/routes/routes_test.go:162` — remove
  `"/api/v1/auth/register"` from the `expectedRoutes` slice in
  `TestRegister_RoutesRegistration`.
- `backend/internal/api/routes/routes_test.go:215` — remove the
  `http.MethodPost + " /api/v1/auth/register": true` entry from
  `publicMutationAllowlist`.
- `backend/internal/api/routes/routes_test.go:335` — change
  `assert.Contains(t, routeMap, "/api/v1/auth/register")` to
  `assert.NotContains(t, routeMap, "/api/v1/auth/register")` (or move the
  assertion into the new Part C test, §3.3.4).
- `backend/internal/api/handlers/additional_coverage_test.go` —
  `TestAuthHandler_Register_InvalidJSON` (`:717-732`, calls `h.Register(c)`):
  delete this test (the handler it covers is gone). Adjust the file's imports if
  needed.
- `backend/integration/crowdsec_lapi_integration_test.go:52-59` — the
  `authenticate()` helper's "Register (may fail if user exists - that's OK)"
  block: replace the `POST /api/v1/auth/register` call with
  `POST /api/v1/setup` (same `{name,email,password}` shape; also tolerates a
  "already completed" 403). Build-tagged integration test, not in default CI,
  but must stay compilable/correct.

#### 3.3.3 `util.GenerateSecureToken` promotion — **DROPPED**

The earlier draft promoted `user_handler.go`'s `generateSecureToken()` to
`backend/internal/util` for the now-cancelled invite pool. Nothing else needs
it. **No refactor** — `generateSecureToken()` stays unexported in
`user_handler.go` exactly as-is.

#### 3.3.4 Tests — new `backend/internal/api/routes/routes_test.go`

`TestRegister_PublicRegistrationEndpointRemoved`:

| Case | Request | Expected |
|---|---|---|
| Route gone | `POST /api/v1/auth/register {…}` (no auth) | `404` |
| Route gone (any method) | `GET /api/v1/auth/register` | `404` |
| Bootstrap intact | `GET /api/v1/setup` on empty DB | `200 {"setupRequired":true}` |
| Bootstrap intact | `POST /api/v1/setup {name,email,password}` on empty DB | `201`; a `role=admin` user exists; `caddy.acme_email` setting written |
| Bootstrap closed after first | `POST /api/v1/setup` again | `403 {"error":"Setup already completed"}` |
| Email-invite intact | admin `POST /api/v1/users/invite {email}` → `GET /api/v1/invite/validate?token=…` → `POST /api/v1/invite/accept {token,name,password}` | invite validates; acceptance `200`; the invited user is `enabled` and can `POST /api/v1/auth/login` |

`AuthService.Register` unit tests in `auth_service_test.go` are unchanged
(the method is unchanged).

#### 3.3.5 Frontend

- **No new pages, routes, api modules, or hooks.**
- `frontend/src/api/*` — confirm no `auth/register` caller exists (grep already
  shows none). No edit.
- `frontend/src/pages/AcceptInvite.tsx`, `frontend/src/api/users.ts`,
  `frontend/src/pages/UsersPage.tsx` — unchanged by Part C. `UsersPage` remains
  the admin surface for creating/inviting users.
- Optional 1-line doc/help-text touch if any onboarding copy mentions
  self-signup (grep `i18n` for "register" / "sign up" in
  `frontend/src/locales` — likely none; skip if absent).

### 3.4 Data flow (after this feature)

```
First run (no users)
  │  POST /api/v1/setup {name,email,password}
  ▼
api (public) → UserHandler.Setup → tx{ INSERT users(role=admin, enabled=true) ; upsert Setting caddy.acme_email }
  ▼  201

Add a user (admin only)
  │  admin → POST /api/v1/users {email,name,password,role?}         (direct)
  │      or → POST /api/v1/users/invite {email,role?}  → email/link → /accept-invite?token=… → POST /api/v1/invite/accept
  ▼  UserHandler.CreateUser / InviteUser / AcceptInvite   (all existing, unchanged)

Removed
  │  POST /api/v1/auth/register …
  ▼  404  (route deleted)

Attacker with a role=user token (however obtained)
  │  POST /api/v1/admin/crowdsec/stop     → management → managementAdmin → RequireRole(admin) → 403   (Part A)
  │  POST /api/v1/admin/plugins/x/enable  → RequireRole(admin) arg → 403                              (Part B #3)
  │  POST /api/v1/certificates/x/export   → RequireRole(admin) arg → 403                              (Part B #19)
  │  GET  /api/v1/certificates            → management → 200   (READ stays — non-admin page needs it) (Part B #18)
```

### 3.5 Error handling & edge cases

| Case | Handling |
|---|---|
| `POST /auth/register` after deploy | `404` (Gin default no-route). Covered by test. |
| Client / script still POSTing `/auth/register` | Gets `404`; must switch to `/setup` (bootstrap) or admin invite. Called out in `ARCHITECTURE.md` + release notes. |
| `/setup` on an already-bootstrapped instance | `403 {"error":"Setup already completed"}` (existing logic, unchanged). |
| Concurrent `/setup` calls on empty DB | Existing `isSetupConflictError` / post-tx count re-check handles it (unchanged). |
| Removing `RegisterRequest` leaves an unused import in `auth_handler.go` | `goimports` / `staticcheck` catches; remove in the same commit. |
| `additional_coverage_test.go` import set after deleting the test | Adjust; `go build ./...` + `go vet` verify. |
| A moved route (Part B) that a `role=user` UI screen actually needs | Prevented by the mutation-vs-read classification (Q7) + frontend E2E asserting `role=user` still `200`s on the READ endpoints + still loads the non-gated pages. |
| `role=user` opens a page whose *mutations* now 403 (Access Lists, Security Headers, DNS Providers, Certificates, Domains, Remote Servers, Hecate tunnels) | Page loads (reads succeed — verified consumed by `role=user` screens); create/edit/delete return 403 `{"error":"Forbidden"}`. Acceptable; optional follow-up to hide the buttons ([§7](#7-remaining-open-questions)). |
| `role=user` opens an admin-only page (CrowdSec, Audit Logs, Orthrus agent-management, Encryption) | Companion `RequireRole` guard redirects them away; nav entry hidden where one exists — same UX as "Users" today. No dead page. |
| `ConnectionTypeSelector` / Dashboard hecate widget for `role=user` after Part B | Their reads (`GET /orthrus/agents`, `GET /hecate/status`, `GET /hecate/tunnels`) stay on `management` — verified they still return `200`. E2E asserts this. |
| GORM security scan | No model / query changes in this feature → `scripts/scan-gorm-security.sh` is N/A, but run it anyway if any handler file under `backend/internal/models/**` is touched (none expected). |
| Migration impact | None — no schema change. |

---

## 4. Implementation Plan

### Phase 1 — E2E specs (behavior, as `test.fixme`)

- `tests/security-enforcement/crowdsec-admin-authz.spec.ts` (new) — `role=user`
  → `403` on `/admin/crowdsec/stop`, `/bouncer/key`, `/ban`, `/file`;
  unauthenticated → `401`; `role=admin` → not `403`.
- Extend `tests/security-enforcement/authorization-rbac.spec.ts` —
  `role=user` → `403` on: `POST /admin/plugins/:id/enable`,
  `POST/DELETE /remote-servers*` (+ `POST /remote-servers/test`),
  Hecate mutations (`POST /hecate/tunnels`, `POST /hecate/tunnels/:uuid/start`,
  `POST /hecate/tailscale/sync`), Orthrus mutations (`POST /orthrus/agents`,
  `DELETE /orthrus/agents/:uuid`, `GET /orthrus/agents/:uuid/snippets`),
  `POST/PUT/DELETE /dns-providers*`, `POST /dns-providers/test`,
  `POST /notifications/providers/test`, `POST /notifications/providers/preview`,
  `POST /certificates/:uuid/export`, `POST/PUT/DELETE /access-lists*`,
  `POST/DELETE /domains*`, `POST/PATCH /settings`, `GET /audit-logs`,
  `GET /dns-providers/:id/audit-logs`;
  `role=user` still `200` on `GET /proxy-hosts`, `GET /settings`,
  `GET /themes`, `GET /certificates`, `GET /access-lists`, `GET /dns-providers`,
  `GET /hecate/status`, `GET /hecate/tunnels`, `GET /orthrus/agents`,
  `GET /remote-servers`.
- `role=user` navigating directly to `/security/crowdsec`,
  `/security/audit-logs`, `/hecate/agent`, `/security/encryption` is redirected
  (companion `RequireRole` guards); those nav entries are absent for `role=user`
  where a nav entry exists.
- `tests/security-enforcement/public-registration-removed.spec.ts` (new) —
  `POST /api/v1/auth/register` → `404`; `/setup` bootstrap still works on a
  fresh instance; existing email-invite acceptance flow
  (`/users/invite` → `/invite/validate` → `/invite/accept` → login) still works.
- All `test.fixme` until Phase 2/3 land; un-fixme in Phase 4.
- **Dropped from the earlier plan:** `invite-registration.spec.ts`.

### Phase 2 — Backend

- **Commit 2 (Part A):** `managementAdmin` subgroup decl;
  `crowdsecHandler.RegisterRoutes(managementAdmin)`; companion frontend guard
  (`/security/crowdsec` route + nav); `routes_test.go` regression (§3.1.4).
  `fix(security):`.
- **Commit 3 (Part B):** re-run audit; apply the §3.2.2 table:
  - Wholesale: CrowdSec (done in Commit 2).
  - `RegisterRoutes(read, admin)` split: `HecateHandler`, `OrthrusHandler`,
    `RemoteServerHandler` (reads listed in rows 11/14a/15a stay on `management`;
    all other routes → `managementAdmin`).
  - `SecurityHeadersHandler`: **delete** its `RegisterRoutes` method; register
    its ~11 routes inline in `routes.go` (reads + 3 calculators on `management`,
    profile mutations + `presets/apply` on `managementAdmin`).
  - Per-route `RequireRole(admin)` args: plugin enable/disable/reload;
    dns-provider mutations + `POST /dns-providers/test` + `POST /dns-providers/:id/test`
    + credentials + `POST /dns-providers/detect`; `ManualChallengeHandler.RegisterRoutes(managementAdmin)`;
    certificate mutations incl. `/export`; access-list mutations; domain
    mutations; settings mutations (+ keep existing `GET /settings/smtp` arg);
    `PUT /feature-flags`; `POST /system/permissions/repair`;
    `POST /notifications/providers/test` + `/preview` + `/external-templates/preview`.
  - `MOVE → managementAdmin`: `GET /audit-logs`, `GET /audit-logs/:uuid`,
    `GET /dns-providers/:id/audit-logs`; `adminEncryption` group decl →
    `managementAdmin.Group("/admin/encryption")` (defense-in-depth).
  - Companion frontend guards: `<RequireRole allowed={['admin']}>` on
    `/security/audit-logs`, `/hecate/agent` (+ nav child),
    `/security/encryption` (+ nav child); `/security/crowdsec` already done.
  - `TestManagementGroup_MutationsAreAdminGuarded` + `USER_OK_MUTATION_ALLOWLIST`
    + `PUBLIC_MUTATION_ALLOWLIST` (reviewed constants); update
    `TestRegister_StateChangingRoutesDenyByDefaultWithExplicitAllowlist`.
  `fix(security):`.
- **Commit 4 (Part C):** delete `/auth/register` route + `AuthHandler.Register`
  + `RegisterRequest`; keep `AuthService.Register`; update the 4 backend test
  references + the integration-test helper; new
  `TestRegister_PublicRegistrationEndpointRemoved`. `fix(security):`.

### Phase 3 — Frontend

Rolled into Commits 2 & 3 (the companion `RequireRole` guards + nav filters are
small and belong with the backend change that necessitates them). No standalone
frontend commit — there is no new UI in this feature.

### Phase 4 — Integration, hardening, docs

- **Commit 5:** un-`fixme` the Phase 1 specs; run targeted specs (firefox).
  File a follow-up issue for a general per-IP auth throttle middleware (out of
  scope — noted, not built). Update `ARCHITECTURE.md`, `SECURITY.md`,
  `docs/security.md`, `docs/features/access-control.md`, `docs/features.md`,
  `docs/features/crowdsec.md`, `docs/features/custom-plugins.md` /
  `plugin-security.md`. `docs:`.

---

## 5. Acceptance Criteria (Definition of Done)

1. **Advisory closed:** unauthenticated → `401`, `role=user` → `403`,
   `role=admin` → handler executes, on `/admin/crowdsec/stop`,
   `/admin/crowdsec/bouncer/key`, `/admin/crowdsec/ban`,
   `/admin/crowdsec/file`. Proven by `routes_test.go` + E2E.
2. **Plugins mutations closed:** `role=user` → `403` on
   `POST /admin/plugins/:id/enable|disable`, `POST /admin/plugins/reload`;
   `GET /admin/plugins*` still `200` for `role=user`.
3. **Deny-by-default:** `TestManagementGroup_MutationsAreAdminGuarded` passes;
   every mutating `/api/v1/*` route is admin-guarded or on a reviewed allowlist
   with a per-entry comment.
4. **Public registration gone:** `POST /api/v1/auth/register` → `404` (route
   absent from `router.Routes()`).
5. **Bootstrap + invites intact:** `/setup` first-admin flow succeeds on a
   fresh instance and 403s afterward; email-invite
   (`/users/invite` → `/invite/validate` → `/invite/accept` → login) succeeds.
   Regression tests prove both.
6. **`AuthService.Register` retained** and all its existing unit tests pass
   unchanged; `AuthHandler.Register` / `RegisterRequest` / the register route
   are removed with no dangling references (`go build ./...`, `staticcheck`,
   `go vet` clean).
7. **No `role=user` dead pages:** admin-only pages (CrowdSec, **Audit Logs**,
   the Orthrus agent-management page `/hecate/agent`, Encryption) are hidden
   from `role=user` in nav and redirect on direct navigation. READ-classified
   pages (Access Lists, Certificates, DNS Providers, Security Headers, Domains,
   Remote Servers, Hecate tunnels) still load for `role=user`, and their reads
   (`GET /orthrus/agents`, `GET /hecate/status`, `GET /hecate/tunnels`,
   `GET /remote-servers`, …) still return `200`. Frontend E2E covers both.
8. **Coverage:** backend ≥ 85 % (`scripts/go-test-coverage.sh`), frontend
   ≥ 85 % (`scripts/frontend-test-coverage.sh`); patch coverage green
   (`bash scripts/local-patch-report.sh` → `test-results/local-patch-report.{md,json}`).
9. **Security gates:** `lefthook run pre-commit` (CodeQL Go + JS) 0
   high/critical; `make trivy` clean; `make lint-fast` / staticcheck clean.
   (`scripts/scan-gorm-security.sh --check` if any `models/**` file is touched —
   none expected.)
10. **Targeted E2E green (firefox only):** `crowdsec-admin-authz.spec.ts`,
    `authorization-rbac.spec.ts`, `public-registration-removed.spec.ts`,
    `auth-api-enforcement.spec.ts`. Full-suite / cross-browser deferred to CI.
11. **Type safety / build:** `cd frontend && npm run type-check` clean;
    `cd backend && go build ./...`; `cd frontend && npm run build`.
12. **Docs:** `ARCHITECTURE.md` + `SECURITY.md` reflect the new authorization
    boundary and the removal of public self-registration.

---

## 6. Complexity Estimates

| Component | Complexity | Notes |
|---|---|---|
| Part A route move + frontend guard + tests | **Low** | 2-line routing change, 1 route wrap + 1 nav filter, 1 test file. |
| Part B audit + moves + splits + enforcement test | **Medium-High** | ~35 registration sites reviewed; ~16 per-route `RequireRole` args; 3 handlers gain a `RegisterRoutes(read, admin)` split (`Hecate`, `Orthrus`, `RemoteServer`); `SecurityHeadersHandler.RegisterRoutes` deleted + inlined; `GET /audit-logs*` + 1 per-provider audit read moved; 4 companion frontend `RequireRole` guards; new enforcement test + 2 reviewed allowlists; risk of a mis-classified `role=user` read (mitigated by `frontend/src` verification + E2E). |
| Part C deletions | **Low** | Delete 1 route + 1 handler + 1 struct; keep the service; fix 4 test refs + 1 integration helper; 1 new test. |
| Docs | **Low** | |

---

## 7. Remaining open questions

All earlier open questions and all supervisor blocking/should-fix items are
resolved and baked into the spec:

- Invite pool dropped → Q1/Q2/Q5 moot.
- Q6 — subgroup-only, no belt-and-braces in-handler `requireAdmin`.
- Q7 / C1 / C2 — mutation-vs-read classification; Hecate / Orthrus /
  RemoteServer use a `RegisterRoutes(read, admin)` split (NOT wholesale move),
  reads verified against `frontend/src`.
- C3 — `POST /notifications/{providers/test,providers/preview,external-templates/preview}`
  added to the table as ADMIN-ARG; §2.1 in-handler audit row corrected.
- C4 / §7.1 — **resolved in this PR**: `GET /audit-logs*` → `managementAdmin` +
  `<RequireRole allowed={['admin']}>` on `/security/audit-logs`.
- C5 — `GET /dns-providers/:id/audit-logs` → `managementAdmin`.
- C6 / §7.3 — **resolved**: `SecurityHeadersHandler.RegisterRoutes` deleted, its
  routes inlined in `routes.go` with per-route args (matches its siblings).
- C7 — `POST /dns-providers/test` (id-less `TestCredentials`) named explicitly,
  separate from `POST /dns-providers/:id/test`.
- Q4 — per-IP auth throttle: deferred, tracking issue filed in Commit 5.

**Only remaining item — deferred UX polish (not a blocker, tracked in Commit 5):**

1. Hide the disabled create/edit/delete controls for `role=user` on the
   READ-classified pages (Access Lists, Certificates, DNS Providers, Security
   Headers, Domains, Remote Servers, Hecate tunnels). The API already enforces
   `403`; this is cosmetic. Out of scope for this PR; tracking issue filed
   alongside the auth-throttle issue in Commit 5.

---

## 8. Risks & Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| A READ endpoint mis-classified as ADMIN regresses a `role=user` page | `role=user` UI breaks | Q7 mutation-vs-read rule; frontend E2E asserts `role=user` keeps `GET` access + page loads for every READ-classified area; classification table in PR description; each commit individually revertable. |
| An admin-gated capability was actually needed by `role=user` | Lost functionality for `role=user` | Only CrowdSec moves wholesale (no `role=user` read). Hecate / Orthrus / RemoteServer keep their `role=user`-consumed `GET` reads on `management` (verified: `ConnectionTypeSelector` → `GET /orthrus/agents`, `Dashboard` → `GET /hecate/status`); only mutations move. Audit Logs / Orthrus agent page / Encryption become admin-only with a companion `RequireRole` guard (explicit redirect, not a silent 403). If a real `role=user` need surfaces, revert Commit 3 alone — Commit 2 (advisory fix) still stands. |
| Removing `RegisterRequest`/`Register` leaves dangling refs | Build break | grep evidence in §2.1 enumerates every reference; `go build ./...` + `staticcheck` + `go vet` in the commit gate; integration-test helper explicitly updated. |
| `AuthService.Register` mistakenly deleted | ~28 test call sites fail to compile | Spec is explicit: **keep** it; it is not dead. |
| Advisory still private / embargoed | Disclosure via commit message / changelog | `fix(security):` subjects deliberately vague — category + mitigation only, never "CrowdSec", "authorization bypass", "public registration", or route paths (§10). No GHSA id in subjects or changelog-visible lines. |
| `publicMutationAllowlist` still lists `auth/register` after route removal | Enforcement test references a non-existent route | Commit 4 removes that entry (§3.3.2). |
| Coverage dip from the large Part B routing diff | PR fails 85 % gate | New tests target new/moved code paths; `local-patch-report.sh` preflight before pushing. |
| Companion frontend guards missed for an admin-only page | `role=user` hits a 403-ing page | E2E: for `/security/crowdsec`, `/security/audit-logs`, `/hecate/agent`, `/security/encryption`, assert a `role=user` session is redirected and (where a nav entry exists) it is absent. |
| A non-mutating `POST` (`/access-lists/:id/test`, `/security/headers/score` etc.) breaks for `role=user` because it's a POST | `role=user` diagnostic feature 403s | These are explicitly in `USER_OK_MUTATION_ALLOWLIST` (§3.2.4) and stay on `management`; the enforcement test asserts `role=user` is NOT 403 for them. |

---

## 9. Commit Slicing Strategy

**Decision:** ONE PR, merged only when the whole feature is complete and the
full Definition of Done passes. Reviewability comes from the ordered commit
sequence below — **not** from splitting into backend/frontend/security PRs.
Each commit builds and passes its own validation gate. Order follows
`CLAUDE.md` "Suggested Commit Sequence" (E2E fixme → backend → frontend →
hardening+docs); the advisory fix (Part A) is placed first after the specs so it
is independently revertable. Part C collapsed to a single deletion commit — the
5-commit plan replaces the earlier 7.

Base branch: `development`.

---

### Commit 1 — E2E specs for new behavior (`test.fixme`)

- **Type:** `test: add fixme e2e specs for privileged-route authz and removal of public registration`
- **Scope:** Author (as `test.fixme`) the Playwright specs for Parts A/B/C. No
  product code.
- **Files:**
  - `tests/security-enforcement/crowdsec-admin-authz.spec.ts` (new)
  - `tests/security-enforcement/public-registration-removed.spec.ts` (new)
  - `tests/security-enforcement/authorization-rbac.spec.ts` (extend: plugin
    mutations, remote-server mutations, Hecate mutations, Orthrus mutations
    (incl. `/snippets`), dns-provider mutations + `POST /dns-providers/test`,
    notification `test`/`preview`, cert `/export`, access-list/domain/settings
    mutations, `GET /audit-logs*`; + `role=user` positive READ cases incl.
    `GET /orthrus/agents`, `GET /hecate/status`, `GET /hecate/tunnels`,
    `GET /remote-servers`; + admin-only nav/redirect checks for
    `/security/crowdsec`, `/security/audit-logs`, `/hecate/agent`,
    `/security/encryption`)
- **Depends on:** nothing.
- **Validation gate:**
  `npx playwright test crowdsec-admin-authz public-registration-removed authorization-rbac --project=firefox`
  collects specs, all `fixme`/skipped, 0 failures; `eslint` clean on the new
  spec files.

---

### Commit 2 — Part A: enforce admin authorization on CrowdSec admin routes (advisory fix)

- **Type:** `fix(security): tighten authorization checks on privileged API routes`
- **Scope:**
  - `routes.go`: declare `managementAdmin := management.Group("/"); .Use(RequireRole(admin))`;
    change `crowdsecHandler.RegisterRoutes(management)` → `(managementAdmin)`.
  - Frontend companion guard: wrap `security/crowdsec` route in
    `<RequireRole allowed={['admin']}>` (`App.tsx`); gate the `navigation.crowdsec`
    nav child with `user?.role === 'admin'` (`Layout.tsx`).
  - `routes_test.go`: `TestRegister_CrowdsecAdminRoutesRequireAdminRole` (§3.1.4);
    a handler-level 403 assertion in `crowdsec_handler_test.go` if lightweight.
- **Files:** `backend/internal/api/routes/routes.go`,
  `backend/internal/api/routes/routes_test.go`,
  `backend/internal/api/handlers/crowdsec_handler_test.go` (maybe),
  `frontend/src/App.tsx`, `frontend/src/components/Layout.tsx`,
  `frontend/src/components/__tests__/Layout.test.tsx` (nav-gating assertion) or
  a new small `App` route test.
- **Depends on:** Commit 1 (ordering).
- **Validation gate:**
  `cd backend && go build ./... && go test ./internal/api/routes/... ./internal/api/handlers/...`;
  new test proves unauth→401 / `role=user`→403 / `role=admin`→not-403 on the 4
  representative routes; existing `TestRegister_AllRoutesRegistered` /
  `TestRegister_CrowdSecRoutes` still pass (paths unchanged);
  `cd frontend && npm run type-check && npx vitest run src/components/__tests__/Layout.test.tsx`;
  `make lint-fast`; staticcheck clean.

---

### Commit 3 — Part B: deny-by-default authorization across the management group

- **Type:** `fix(security): apply deny-by-default authorization on management API subroutes`
- **Scope:**
  - Re-run the route audit vs HEAD; reconcile with §3.2.2.
  - `RegisterRoutes(read, admin *gin.RouterGroup)` split (C1/C2): `HecateHandler`
    (reads `GET /hecate/status|/tunnels|/tunnels/:uuid` on `read`, rest on
    `admin`); `OrthrusHandler` (reads `GET /orthrus/agents|/agents/:uuid` on
    `read`, rest incl. `/snippets`, `/proxy-status` on `admin`);
    `RemoteServerHandler` (reads `GET /remote-servers|/remote-servers/:uuid` on
    `read`, rest incl. `/test` on `admin`).
  - `SecurityHeadersHandler` (C6): **delete** `RegisterRoutes`; register its ~11
    routes inline in `routes.go` — reads + 3 calculator `POST`s on `management`,
    profile `POST/PUT/DELETE` + `presets/apply` on `managementAdmin`.
  - `MOVE → managementAdmin`: `GET /audit-logs`, `GET /audit-logs/:uuid` (C4),
    `GET /dns-providers/:id/audit-logs` (C5); `adminEncryption` group decl →
    `managementAdmin.Group("/admin/encryption")`.
  - ADMIN-ARG (per-route `middleware.RequireRole(models.RoleAdmin)` 2nd arg):
    plugin enable/disable/reload; dns-provider mutations + `POST /dns-providers/test`
    + `POST /dns-providers/:id/test` (C7) + credential + `POST /dns-providers/detect`;
    `ManualChallengeHandler.RegisterRoutes(managementAdmin)`; certificate
    mutations incl. `/export`; access-list mutations; domain mutations; settings
    mutations (+ keep existing `GET /settings/smtp` arg); `PUT /feature-flags`;
    `POST /system/permissions/repair`;
    `POST /notifications/providers/test` + `/providers/preview`
    + `/external-templates/preview` (C3).
  - Frontend companion `RequireRole` guards + nav filters:
    `/security/audit-logs` (route only — no nav entry), `/hecate/agent`
    (route + nav child), `/security/encryption` (route + nav child).
    Do **not** guard `navigation.hecate` wholesale or `/hecate/tunnels`.
  - `TestManagementGroup_MutationsAreAdminGuarded` + `USER_OK_MUTATION_ALLOWLIST`
    + `PUBLIC_MUTATION_ALLOWLIST` (reviewed constants).
- **Files:** `backend/internal/api/routes/routes.go` (the ~35 sites in the
  table), `backend/internal/api/handlers/security_headers_handler.go`
  (delete `RegisterRoutes` method + its test that asserted the old group),
  `backend/internal/api/handlers/hecate_handler.go` / `orthrus_handler.go` /
  `remote_server_handler.go` (`RegisterRoutes(read, admin)` signature +
  callers), `backend/internal/api/routes/routes_test.go`, any handler test that
  assumed a now-moved route was reachable by `role=user`
  (`hecate_handler_test.go`, `orthrus_handler_test.go`,
  `audit_log_handler_test.go`, `notification_provider_handler_test.go`),
  `frontend/src/App.tsx`, `frontend/src/components/Layout.tsx`, related frontend
  tests.
- **Depends on:** Commit 2 (`managementAdmin`).
- **Validation gate:** `go build ./... && go test ./...` (full — catches handler
  tests broken by moves); new enforcement test green; manual diff of
  `router.Routes()` inventory before/after (path set unchanged, only middleware
  chains differ); `cd frontend && npm run type-check && npx vitest run` (touched
  suites); `make lint-fast`; staticcheck clean.

---

### Commit 4 — Part C: remove the public registration endpoint

- **Type:** `fix(security): reduce unauthenticated API surface`
- **Scope:**
  - Delete `api.POST("/auth/register", …)` (`routes.go:295`),
    `AuthHandler.Register`, `RegisterRequest` (`auth_handler.go`). Drop
    now-unused imports.
  - **Keep** `AuthService.Register` (+ `count==0 → RoleAdmin`); add a doc
    comment marking it internal/test-only.
  - Update `routes_test.go` refs (`:162` remove from `expectedRoutes`; `:215`
    remove allowlist entry; `:335` → `assert.NotContains`); delete
    `TestAuthHandler_Register_InvalidJSON` in `additional_coverage_test.go`;
    switch `crowdsec_lapi_integration_test.go` `authenticate()` helper to
    `POST /api/v1/setup`.
  - New `TestRegister_PublicRegistrationEndpointRemoved` (§3.3.4) covering
    route-gone + `/setup` bootstrap + email-invite acceptance.
- **Files:** `backend/internal/api/routes/routes.go`,
  `backend/internal/api/handlers/auth_handler.go`,
  `backend/internal/services/auth_service.go` (doc comment only),
  `backend/internal/api/routes/routes_test.go`,
  `backend/internal/api/handlers/additional_coverage_test.go`,
  `backend/integration/crowdsec_lapi_integration_test.go`.
- **Depends on:** Commit 2 (shares `routes_test.go` allowlist edits — sequence
  after B to avoid churn).
- **Validation gate:** `go build ./...` (+ `-tags integration` compile check for
  the integration file); `go test ./internal/api/...`; `staticcheck` / `go vet`
  clean (no dangling refs); `AuthService.Register` unit tests unchanged & green;
  `make lint-fast`.

---

### Commit 5 — Enable E2E, coverage, docs

- **Type:** `docs: document management-API authorization model and account-creation flow`
- **Scope:**
  - Un-`fixme` the Commit 1 specs; adjust selectors/fixtures to the shipped
    behavior; run targeted specs (firefox).
  - File two follow-up issues (out of scope here): (1) "per-IP rate limit /
    throttle middleware for `/api/v1/auth/*`" (`/auth/register` removed,
    `/auth/login` already has account lockout); (2) "hide disabled
    create/edit/delete controls for `role=user` on READ-classified admin pages
    (Access Lists, Certificates, DNS Providers, Security Headers, Domains,
    Remote Servers, Hecate tunnels)" — cosmetic; the API already returns `403`
    (spec §7 item 1).
  - Docs: `ARCHITECTURE.md` (Security Architecture / Auth & Authorization —
    `managementAdmin` boundary; no public self-registration; bootstrap +
    invite model), `SECURITY.md` (Authentication & Authorization section),
    `docs/security.md`, `docs/features/access-control.md`, `docs/features.md`,
    `docs/features/crowdsec.md`, `docs/features/custom-plugins.md` /
    `plugin-security.md`.
- **Files:** the Commit 1 spec files (remove `fixme`); the docs listed above.
- **Depends on:** Commits 2-4.
- **Validation gate (full DoD):**
  `npx playwright test crowdsec-admin-authz authorization-rbac public-registration-removed auth-api-enforcement --project=firefox` all green;
  `bash scripts/local-patch-report.sh` (artifacts present, patch coverage green);
  `lefthook run pre-commit` (CodeQL Go+JS) 0 high/critical; `make trivy` clean;
  `make lint-fast` + `make lint-backend` clean;
  `scripts/go-test-coverage.sh` ≥ 85 %; `scripts/frontend-test-coverage.sh` ≥ 85 %;
  `cd frontend && npm run type-check && npm run build`;
  `cd backend && go build ./...`; `go test ./...` + `npx vitest run` zero
  failures; debug/print cleanup.

---

### Rollback & contingency (PR-wide)

- **Per-commit revert:** Commits 2, 3, 4 are individually revertable.
  - Revert **Commit 3** alone if the Part B sweep regresses a `role=user`
    workflow found late — Commit 2 (the actual advisory fix) and Commit 4 still
    stand and ship value.
  - Revert **Commit 4** alone (restore the register route) without affecting
    the authz fixes, if an external consumer of `/auth/register` is discovered
    that can't migrate to `/setup` in time — though the advisory title itself
    frames public registration as the root enabler, so this should be a last
    resort with a tracking issue.
- **Minimum shippable:** Commits 1-2 + docs = the advisory is closed. Parts B/C
  can be dropped from the PR (update this spec + PR description) if they need
  more time — but the intent is to land all three together.
- **No migration to roll back** — zero schema changes.
- **Feature-flag option (contingency, not in the default plan):** if reviewers
  want a kill switch for Part C rather than a hard delete, gate the register
  route behind a `Setting` (`auth.public_registration_enabled`, default
  `false`) instead of removing it. Adds surface; only if explicitly requested.
- **Embargo:** keep the GHSA id, "CrowdSec", route paths, and
  "authorization bypass / public registration" out of every commit subject and
  any changelog-visible line. The PR description MAY reference the advisory
  (repo private, pre-disclosure) — confirm with the maintainer before opening.

---

## 10. Commit Message Conventions (per `CLAUDE.md`)

- Security-relevant commits use `fix(security):` with a **deliberately vague**
  subject — category of issue + category of mitigation only. Never name the
  vulnerability class, the component ("CrowdSec", "plugins"), the attack vector
  ("public registration"), or any route path.
  - Commit 2: `fix(security): tighten authorization checks on privileged API routes`
  - Commit 3: `fix(security): apply deny-by-default authorization on management API subroutes`
  - Commit 4: `fix(security): reduce unauthenticated API surface`
- Non-security commits: `test:` (Commit 1), `docs:` (Commit 5).
- `fix:` triggers Docker builds (intended here).
- Every commit message ends with:
  ```
  Claude-Session: https://claude.ai/code/session_01Wm1jzKSdvz2LCusQC2qokM
  ```
- PR description ends with:
  ```
  https://claude.ai/code/session_01Wm1jzKSdvz2LCusQC2qokM
  ```

---

## 11. Handoff

- Next: `supervisor` review of this spec → iterate → user approval → implement
  Commits 1-5 in order via `backend-dev` / `frontend-dev` (each commit passes
  its gate before the next starts) → `supervisor` implementation review →
  `qa-security` audit last → `docs-writer`.
- Key references for implementers:
  - Advisory root cause: `backend/internal/api/routes/routes.go:838`, `:373-374`;
    correct pattern at `:796-797` and `:1011-1012`; per-route arg precedent at
    `:457`.
  - `backend/internal/api/middleware/auth.go` (`RequireRole`,
    `RequireManagementAccess`).
  - `backend/internal/api/handlers/permission_helpers.go` (`requireAdmin`,
    `isAdmin`).
  - Part C targets: `backend/internal/api/handlers/auth_handler.go:238-256`
    (delete `RegisterRequest` + `Register`), `routes.go:295` (delete route),
    `backend/internal/services/auth_service.go:31` (**keep**),
    `backend/internal/api/handlers/user_handler.go:141` (`Setup` — the retained
    bootstrap path).
  - Test refs to fix: `routes_test.go:162,215,335`;
    `additional_coverage_test.go:717-732`;
    `backend/integration/crowdsec_lapi_integration_test.go:52-59`.
  - Existing email-invite (the supported post-bootstrap path, unchanged):
    `backend/internal/api/handlers/user_handler.go` (`InviteUser` / `ValidateInvite`
    / `AcceptInvite`), `backend/internal/models/user.go` (invite fields),
    `frontend/src/pages/AcceptInvite.tsx`, `frontend/src/api/users.ts`.
  - Frontend gating pattern to mirror: `frontend/src/components/RequireRole.tsx`,
    `frontend/src/App.tsx:120,126`, `frontend/src/components/Layout.tsx:127`.
  - Test harness: `backend/internal/api/routes/routes_test.go`
    (`TestRegister_*`, `materializeRoutePath`, `publicMutationAllowlist`),
    `tests/security-enforcement/authorization-rbac.spec.ts`
    (`loginAndGetToken`, `TEST_USERS`).

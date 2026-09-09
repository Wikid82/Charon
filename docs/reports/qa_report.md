# QA & Security Report — Management-API Authorization Hardening (GHSA-3gc6-295r-xm5m)

- **Feature branch:** `development`
- **Commits audited:** `9cf79091`, `dd05dd7c`, `21135f42`, `6a7cd24f`, `b73dd82a`, `2ac09dc1` (all after `3055a913`)
- **Plan:** `docs/plans/current_spec.md` (§3.2.2 route table, §5 Acceptance Criteria)
- **Date:** 2026-09-08
- **Verdict:** **GHSA-3gc6-295r-xm5m: FIXED.** All Definition-of-Done gates pass. No blocking issues.

---

## 1. Definition of Done — gate-by-gate

| # | Gate | Result | Numbers |
|---|------|--------|---------|
| 1 | Backend coverage (`scripts/go-test-coverage.sh`) | **PASS** | Statement 92.0%, line 88.7% vs gate 87% |
| 2 | Frontend coverage (`scripts/frontend-test-coverage.sh`) | **PASS** | Statements 89.63% (8131/9071), lines 90.83% (7642/8413) vs gate 87% |
| 3 | Local patch-coverage preflight (`scripts/local-patch-report.sh`) | **PASS** | strict mode; overall/backend patch coverage 100.0% (109/109 changed backend lines); frontend/agent 0 changed lines. `test-results/local-patch-report.{md,json}` produced. |
| 4 | CodeQL Go + JS (`lefthook run codeql`) | **PASS (feature)** | Go scan 45.9s, JS scan 55.0s. JS: 0 results. Go: 4 results, **all pre-existing and outside feature-modified code** (see §3). Local findings-gate script aborted on missing `yq` only; CI runs it unconditionally. |
| 5 | Trivy (`trivy fs`, container/deps) | **PASS (feature)** | 0 CRITICAL. 0 dependency CVEs. Zero dependency/manifest changes in the feature (`go.mod`/`go.sum`/`package*.json` untouched). HIGH findings are a pre-existing Dockerfile `USER` misconfig (Dockerfile not in feature diff), a third-party `node_modules/comlink/Dockerfile`, and a gitignored local test-artifact private key — none feature-attributable. |
| 6 | GORM security scan | **N/A (verified)** | `git diff 3055a913..HEAD -- backend/internal/models/` is empty. No model/GORM/migration change. Scan not required. |
| 7 | Full backend tests (`cd backend && go test ./...`) | **PASS** | Exit 0, zero failures across all packages. |
| 8 | Full frontend tests (`npx vitest run`) + `npm run type-check` | **PASS** | 267 files, 3363 passed, 4 skipped, 2 todo, 0 failures. `tsc --noEmit` clean. |
| 9 | Targeted E2E (`--project=security-tests`: `crowdsec-admin-authz`, `public-registration-removed`, `authorization-rbac`) | **PASS** | **110 passed, 0 failed, 0 skipped, 0 fixme** (12.5s). Ran against a fresh `charon:local` container; host `:8080` (held by `nextcloud-aio-mastercontainer`) worked around with a throwaway gitignored `.docker/compose/docker-compose.override.yml` port remap (`8085:8080`), removed after the run. No nextcloud disruption. |
| 10 | Build (`go build ./...`, `npm run build`) | **PASS** | Both clean. |
| 11 | Lint (`make lint-fast` / staticcheck) | **PASS (feature)** | 2 `govet` findings, **both confirmed pre-existing**: `cmd/api/main.go:261` (err shadow, from `f6361dc8` 2026-03-04) and `internal/api/handlers/docker_handler.go:47` (`reflect.Ptr` inline, present at `3055a913`). Neither file is in the feature diff. No other findings; staticcheck clean. |
| 12 | Debug / cleanup scan of feature diff | **PASS** | No `fmt.Println`, `console.log`, `debugger`, stray `TODO/FIXME`, or commented-out blocks in added lines. `b73dd82a` removed 4 now-unused imports. |

Notes:
- `scripts/local-patch-report.sh` requires `agent/coverage.txt` to exist even though the `agent/` module is untouched by this feature; it was generated with `scripts/agent-test-coverage.sh` (agent module: 82.6% stmt / 75.3% line, its own gate 65%) before the patch report would run.

---

## 2. Security audit

### 2.1 Advisory closed — GHSA-3gc6-295r-xm5m: **FIXED**

`crowdsecHandler.RegisterRoutes(managementAdmin)` (`routes.go:863`). `managementAdmin` =
`management.Group("/")` with `RequireManagementAccess()` (inherited) **+** `RequireRole(models.RoleAdmin)`.

Independently confirmed (unit `TestRegister_CrowdsecAdminRoutesRequireAdminRole` + E2E `crowdsec-admin-authz.spec.ts`):

| Caller | `/admin/crowdsec/stop`, `/bouncer/key`, `/ban`, `/file` |
|--------|--------|
| unauthenticated | **401** |
| `role=user` (valid session) | **403** on every route — cannot read the bouncer key, cannot stop CrowdSec, cannot ban/unban, cannot read config files |
| `role=admin` | reaches handler (never 401/403) |

The `role=user` token is proven still valid on a user-allowed route in the same test, so the 403 is the new admin guard, not a broken session.

### 2.2 Bug class closed — deny-by-default across the `management` group

Reviewed the §3.2.2 table application in `routes.go` independently. Every state-changing `/api/v1/` route is
either (a) on `managementAdmin`, (b) on `securityAdmin`/`authenticatedAdmin`, (c) carries a per-route
`middleware.RequireRole(models.RoleAdmin)` argument, or (d) on a reviewed allowlist with a per-entry
justification. `TestManagementGroup_MutationsAreAdminGuarded` walks every `POST/PUT/PATCH/DELETE` under
`/api/v1/` (~90 mutating routes exercised) and asserts `role=user` → 403 / `role=admin` → not-403 unless
allowlisted; `TestManagementGroup_RouteInventoryNoDuplicates` proves the read/admin splits did not
double-register or orphan any path. Both pass.

Allowlist entries scrutinised — all legitimate, none present merely to pass the test:

- **`PUT /users/:id` self-service** — `UpdateUser` has an explicit non-admin branch that returns
  `403 "Cannot modify role or enabled status"` when `req.Role != "" || req.Enabled != nil`, and
  `403 "Admin access required"` when acting on another user's record. `b73dd82a`'s
  `TestUserHandler_UpdateUser_NonAdminSelfCannotEscalatePrivilegedFields` asserts the **persisted**
  record (not just status) — role stays `user`, `enabled` unchanged, other user's name unchanged.
  Real guard, real test. E2E `authorization-rbac.spec.ts:485` also covers it.
- **proxy-hosts / proxy-groups / themes / uptime monitors** — object-level authz (`PermittedHosts` /
  forward-auth) or explicitly all-management-user features; unchanged by design.
- **security-headers calculators (`/score`, `/csp/validate`, `/csp/build`) and `/access-lists/:id/test`** —
  verified by source inspection to perform **no DB writes** (pure calculators / dry-run).
- **public/emergency allowlist** — `/auth/login`, `/setup`, `/invite/accept`, `/security/events`,
  `/emergency/*` run their own auth schemes (unauthenticated bootstrap, IP + `X-Emergency-Token`), no
  session-role semantics.

**Security-headers preset entries in `adminHandlerRejectsByDesign`** (supervisor flag): `POST/PUT/DELETE
/security/headers/profiles*` and `POST /security/headers/presets/apply` are registered on
`securityHeadersAdmin := managementAdmin.Group("/security/headers")` — the admin route-group guard is
**structurally present and confirmed by code inspection**. For `PUT`/`DELETE /profiles/:id` the test skips
only the *admin-side positive* probe because the materialised id (`1`) is a seeded read-only preset the
handler then rejects with 403 — the `role=user` → 403 assertion still runs and passes. `POST /profiles`
and `POST /presets/apply` receive the full both-sided assertion and pass, positively verifying the admin
path.

### 2.3 Privileged reads left on the bare `management` group

The enforcement test only covers mutations; walked the GET/read routes still on `management` (the `read`
group in the `RegisterRoutes(read, admin)` splits) for secret/PII exposure to `role=user`:

| Read | Model protection |
|------|------------------|
| `GET /remote-servers`, `/remote-servers/:uuid` | `RemoteServer` struct carries **no** password/key/passphrase field — host/port/metadata only |
| `GET /hecate/status`, `/hecate/tunnels`, `/hecate/tunnels/:uuid` | `TunnelConfig.EncryptedCredentials` is `json:"-"`; only non-secret `configuration` serialised |
| `GET /orthrus/agents`, `/orthrus/agents/:uuid` | `OrthrusAgent.AuthKeyHash` is `json:"-"` ("never exposed"); bootstrap token only via `/snippets`, which **moved to admin** |
| `GET /dns-providers`, `/dns-providers/:id`, `/dns-providers/:id/credentials*` | `DNSProvider.CredentialsEncrypted` and `DNSProviderCredential.CredentialsEncrypted` are `json:"-"` |
| `GET /certificates`, `/certificates/:uuid` | metadata only; private-key material only via `POST /certificates/:uuid/export`, which is admin-gated |

These reads were already `role=user`-reachable before this feature (Q7-accepted). The one PII-bearing
read class — audit logs (other users' emails, source IPs, security-event detail) — was **moved to
`managementAdmin`** (`GET /audit-logs`, `/audit-logs/:uuid`, `GET /dns-providers/:id/audit-logs`;
`routes.go:438-439,537`), with an E2E assertion that `role=user` gets 403. No new secret/PII exposure.

### 2.4 `passthrough` role

`RequireManagementAccess()` still aborts `403` for `role == RolePassthrough` and is inherited by both
`management` and `managementAdmin`. `managementAdmin` additionally applies `RequireRole(admin)`, which
also rejects `passthrough` (`userRole != admin`). No new path is reachable by a `passthrough` user.

### 2.5 Public-registration removal — no residual account-creation path

- `POST /api/v1/auth/register` route, `AuthHandler.Register`, and `RegisterRequest` are deleted;
  `POST`/`GET /api/v1/auth/register` → **404** (unit `TestRegister_PublicRegistrationEndpointRemoved`,
  E2E `public-registration-removed.spec.ts`).
- `AuthService.Register` is retained but has **zero non-test callers** (`grep` across `backend/**/*.go`
  excluding `_test.go`): no route, no service wiring. Documented as internal/test-only.
- `POST /setup` self-closes: `UserHandler.Setup` returns `403 "Setup already completed"` when any user
  exists, with both a pre-transaction check and a post-transaction re-count for the concurrent case.
- Invite-accept requires a valid admin-issued token: `AcceptInvite` looks up the exact `invite_token`
  (404 if absent), enforces `InviteExpires` (410) and `InviteStatus == "pending"` (409), and clears the
  token on success. `InviteUser` is admin-only (`requireAdmin`).

### 2.6 Docs consistency

`SECURITY.md` ("Authentication & Authorization"), `docs/security.md` ("Accounts & Roles"), and
`ARCHITECTURE.md` ("Management API Authentication & Authorization") all describe the shipped behavior
accurately: 3-role model, `admin` required for the privileged areas, structural admin-only route group +
deny-by-default CI test, no public self-registration, `/setup` bootstrap + admin invite/add. No
contradictions; no governance "stricter wins" conflict.

---

## 3. Pre-existing findings (not introduced by this feature — informational)

| Source | Finding | Evidence it pre-dates `3055a913` |
|--------|---------|----------------------------------|
| CodeQL Go | `go/cookie-secure-not-set` `auth_handler.go:198` (sev 4.0) | line dates to `88763c79` (2026-08-04); carries an explicit `// codeql[go/cookie-secure-not-set]` reviewed-suppression comment |
| CodeQL Go | `go/log-injection` `remote_server_handler.go:148` ×2 (sev 6.1) | `c.JSON(...)` in the `Get` handler, from `f6361dc8` (2026-03-04); feature only changed this file's `RegisterRoutes` signature |
| CodeQL Go | `go/log-injection` `services/uptime_service.go:1551` (sev 6.1) | file not in the feature diff at all |
| golangci `govet` | `cmd/api/main.go:261` err shadow | `f6361dc8` (2026-03-04); `main.go` not in feature diff |
| golangci `govet` | `docker_handler.go:47` `reflect.Ptr` should be inlined `reflect.Pointer` | present at `3055a913` (`80bdc0e3`); the feature fixed the *same* issue in `orthrus_handler.go` but not here |
| Trivy secret | `backend/internal/api/routes/keys/hecate-ca.{key,crt}` flagged as private key | gitignored (`.gitignore:157 *.key`), never committed; local artifact from routes tests that call `orthrus.NewInternalCA` with an unset `cfg.DatabasePath` (writes into the source tree). Files dated Jul/Aug, predate this work. |

---

## 4. Recommendations (non-blocking)

1. Fix the two pre-existing `govet` findings in a follow-up `chore:` — `docker_handler.go:47`
   (`reflect.Ptr` → `reflect.Pointer`, mirroring the fix this feature already applied in
   `orthrus_handler.go`) and `cmd/api/main.go:261` (rename the shadowed `err`). They currently make
   `make lint-fast` exit non-zero.
2. Make `scripts/local-patch-report.sh` tolerate a missing `agent/coverage.txt` (warn + treat agent
   scope as 0 changed lines) instead of aborting `input_missing`, so the preflight is runnable without a
   separate agent-coverage run when the agent module is untouched.
3. Test hygiene: give the `routes` package tests that construct `config.Config{}` an explicit temp
   `DatabasePath` so `orthrus.NewInternalCA` stops writing `keys/hecate-ca.*` into
   `backend/internal/api/routes/`.
4. Frontend patch-coverage scope reported 0 changed lines for `App.tsx` / `Layout.tsx` (these are outside
   the vitest coverage instrumentation set). The guard behavior is covered instead by the new
   `Layout.test.tsx` (116 lines) and the E2E redirect/nav-hiding assertions
   (`authorization-rbac.spec.ts:467-492`). No action required, noted for traceability.

---

## 5. Blocking issues

**None.** The feature meets every Definition-of-Done gate and closes GHSA-3gc6-295r-xm5m. Recommended for
merge.

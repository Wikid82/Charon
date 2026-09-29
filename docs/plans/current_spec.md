# Fix Plan: Certificate export with private key always returns 403 (GH #1390)

Branch: `fix/cert-export-private-key-403-1390` (single PR into `development`)
Scope: backend-only fix + tests + docs. No model/schema change.

## 1. Root cause (entry -> transformation -> persistence -> exit)

**Entry.** `POST /api/v1/certificates/:uuid/export` is registered at `backend/internal/api/routes/routes.go:1074`:
`management.POST("/certificates/:uuid/export", middleware.RequireRole(models.RoleAdmin), certHandler.Export)`.
`management` = `protected.Group("/")` + `RequireManagementAccess()` (routes.go:426-451); `protected` uses `authMiddleware` (`AuthMiddleware`).

**Transformation.** `AuthMiddleware` (`backend/internal/api/middleware/auth.go:12-40`) sets exactly two context keys:
`c.Set("userID", user.ID)` (type `uint`) and `c.Set("role", string(user.Role))`. The emergency-bypass branch sets `role="admin"`, `userID=uint(0)`.
**Nothing in production code ever calls `c.Set("user", ...)`** (verified by grep across `backend/internal`; the only `Set("user", ...)` hits are in `_test.go` files).

**Handler bug.** `CertificateHandler.Export` (`backend/internal/api/handlers/certificate_handler.go:352-378`) does:
`c.Get("user")` -> `!exists` -> `403 "authentication required"`. Because the key is never set, `exists` is always false, so
**every** `include_key:true` request that gets past the missing-password check returns 403 "authentication required".
(`h.db == nil` is not the cause: `SetDB` is wired at routes.go:1059.) Even if `"user"` existed it must be a `map[string]any` with `"id"` (lines 357-366), a shape nothing produces.

**Persistence/exit.** Would have been `h.db.First(&user, userID)` + `user.CheckPassword`; never reached in production.

**Why tests missed it.** All export tests hand-inject `c.Set("user", map[string]any{"id": ...})` via ad-hoc middleware, so they exercise a contract the real middleware never provides.

## 2. Answers to the investigation questions

1. **Shared helper.** Other handlers read `c.Get("userID")` (uint) / `c.GetString("role")`. Existing helpers in `backend/internal/api/handlers/auth_helpers.go`: `requireUserID(c) (uint, bool)` (writes 401 itself) and in `permission_helpers.go`: `isAdmin`, `requireAdmin`, `requireAuthenticatedAdmin`. Reuse `requireUserID` for ID resolution (returns 401 Unauthorized on missing/bad-typed ID). Password verification pattern to mirror: `user_handler.go:330-347` (`allowPasswordAttempt` then `user.CheckPassword`).
2. **Route group / role gating.** Export is in `management` and additionally wrapped in `RequireRole(models.RoleAdmin)` (routes.go:1074). `RequireRole` passes only when role == admin (admin is always allowed, others must equal the required role); `user` and `passthrough` roles get 403 "Forbidden", passthrough is also rejected earlier by `RequireManagementAccess`. **Criterion "non-admin rejected" already holds at the route level** and needs no new code; it only needs a real-middleware test. Defense in depth: do not add an in-handler `isAdmin` check unless cheap; recommended: add `requireAdmin(c)` only inside the `IncludeKey` branch (one line) so the handler is safe if the route wrapper is ever removed. Decision: include it (cheap, DRY helper exists).
3. **Emergency bypass (userID 0).** Bypass sets role admin and skips the rate limiter (`auth_rate_limit.go:188-190`), but there is no user row and no password to re-verify. **Recommendation (stricter): reject private-key export under emergency bypass** with `403 {"error": "private key export requires an authenticated user session"}` (do not fall back to any password). Bypass is a break-glass control-plane token; it must not become a way to exfiltrate private keys without re-auth. Non-key exports (`include_key:false`) remain allowed under bypass (unchanged). Detect by checking `userID == 0` AND `middleware.IsEmergencyBypass(c)` together.
4. **Other users of the broken pattern.** Production: **only** `certificate_handler.go:352`. Tests hand-injecting `"user"`:
   - `handlers/certificate_handler_upload_export_test.go:139,170,218,244,270`
   - `handlers/certificate_handler_patch_coverage_test.go:400,422,444,471`
   - `handlers/certificate_handler_test.go:33` (`mockAuthMiddleware`, used for Delete tests; the injected key is irrelevant to Delete, replace with real-shape `userID`/`role` or remove the `"user"` key)
   - `handlers/security_event_intake_test.go:322` sets `"user"` (plus `user_id`, `role`) but the handler does not read `"user"`; clean the dead `Set("user", adminUser)` line (CLAUDE.md "delete dead code").
5. **Password-attempt guard.** Current order: `allowPasswordAttempt` (charges one token from the login budget) -> missing-password check -> identity lookup -> `CheckPassword`. Problems: (a) empty-password and no-session requests burn login budget without any password verification; (b) the ordering is not itself the 403 cause. Fix ordering: resolve caller identity (admin + `userID` + bypass reject) first, then reject empty password (keep 403 to preserve the existing contract/tests, message unchanged), then `allowPasswordAttempt`, then `db.First` + `CheckPassword`. Guard charges once per real verification attempt, wrong or right (same as `user_handler.go`); no separate "record failure" API exists on `PasswordAttemptGuard` (interface has only `AllowPasswordAttempt`), so nothing else to record. 429 is written by the guard itself.
6. **Frontend.** `frontend/src/api/certificates.ts:94-107` posts `{format, include_key, password, pfx_password}`; `CertificateExportDialog.tsx:59-86` sends `password` only when `includeKey`, and `required` on the input. Field names match the backend struct (`exportCertificateRequest`). On error it toasts `error.message`. Because the request uses `responseType: 'blob'`, axios error bodies are Blobs, so users see the generic "Request failed with status code 403" rather than "incorrect password". **No change required for this fix.** Optional follow-up (not in this PR): decode the Blob error body to surface the server message; tracked under Follow-ups (section 11).
7. **Tests to rewrite/add**: see section 5.
8. **Docs/CHANGELOG/scope**: see section 6.

## 3. Proposed change (no implementation code here)

In `certificate_handler.go` `Export`, replace the `if req.IncludeKey { ... }` block with the following order (extract into a private method, e.g. `func (h *CertificateHandler) reauthenticateForKeyExport(c *gin.Context, password string) bool`, returning false after writing the response, to keep `Export` short):

1. `requireAdmin(c)` (defense in depth; 403 `admin privileges required`).
2. `requireUserID(c)` (401 if absent / wrong type).
3. If `userID == 0` or `middleware.IsEmergencyBypass(c)`: 403 "private key export requires an authenticated user session".
4. `h.db == nil` -> 500 `{"error": "internal error"}` (misconfiguration, not an auth failure; currently masked as 403).
5. `password == ""` -> 403 "password required to export private key" (unchanged contract).
6. `allowPasswordAttempt(h.passwordGuard, c)` (429 on exhaustion).
7. `h.db.First(&user, "id = ?", userID)` (typed uint): not found -> 403 "user not found" (unchanged); other DB error -> 500.
8. `!user.CheckPassword(password)` -> 403 "incorrect password" (unchanged).

Remove the `c.Get("user")` / `map[string]any` handling entirely. Response bodies for existing failure cases keep their current strings so the frontend/E2E stay compatible. The 401 in step 2 is new but only reachable when the middleware did not run (never in production route).

Import `middleware` in the handler only if not already imported by the package (handlers already import it elsewhere; verify no cycle -- middleware must not import handlers).

## 4. Files to touch

| File | Change |
|---|---|
| `backend/internal/api/handlers/certificate_handler.go` | New re-auth helper, reorder checks, drop `"user"` lookup; use `h.db.First(&user, "id = ?", userID)` with a typed `uint` (not a raw interface arg) |
| `handlers/certificate_handler_upload_export_test.go` | Rewrite `Export_*` tests (lines 117-290) to the real-middleware harness; add new cases |
| `handlers/certificate_handler_patch_coverage_test.go` | Rewrite/delete the four hand-injected export tests (~400-471) |
| `handlers/certificate_handler_test.go` | `mockAuthMiddleware` (line 33): set `role="admin"` and `userID` (uint) instead of `"user"`; re-run the Delete tests and every other user of the helper to confirm they still pass |
| `handlers/certificate_handler_coverage_test.go` | `Export_IncludeKeyNoPassword` (489-508): uses `mockAuthMiddleware` (no role today, would get "admin privileges required"); passes once the mock sets role=admin, assertion "password required" unchanged. `Export_IncludeKeyNoDBSet` (510-530): currently expects 403 "authentication required"; plan returns 500, so change the assertion to 500 |
| `handlers/password_guard_test.go` | `exportRouter` (103-117) has no auth middleware, so `Export_PasswordGuardDeniesKeyExport` and `Export_PasswordGuardAllowsThenReauthenticates` (119-137) would now get 403 from `requireAdmin`. Give `exportRouter` a real-shape context (`role="admin"`, `userID` uint of a seeded user) and adjust: the deny test keeps expecting 429 with a non-empty password; the allow test expects 403 "incorrect password" (seeded user, wrong password) with `guard.calls == 1` |
| `handlers/security_event_intake_test.go` | Remove dead `c.Set("user", adminUser)` (line 322) |
| `backend/internal/api/routes/auth_rate_limit_routes_test.go` | **Mandatory** route-level tests (section 5) using `newThrottledApp` (full `Register()`) and `createUser(role)`; confirm existing `TestRegister_CertificateKeyExportSharesLoginBudget` (line 181) still passes under the new order (it sends password `"x"`, so the guard is still charged and the second call still 429s) |
| `tests/certificate-export.spec.ts` | Add key-export success scenario (`test.fixme` first) |
| `tests/constants.ts` (or the spec) | Export the E2E admin password (see section 5) |
| `docs/api.md`, `docs/features/ssl-certificates.md` | Short export endpoint note; one sentence |

No frontend, model, migration, or route changes.

## 5. Test plan

**Route-level tests (mandatory, primary)** in `routes/auth_rate_limit_routes_test.go`, via `newThrottledApp(t, cfg)` (full `Register()`, real `AuthMiddleware` + `RequireRole` + `allowPasswordAttempt` with the real limiter; use a generous budget such as `withAuthBudget(50, 60)` so the budget is not the variable) and `app.createUser(role)` (password is `correct-password`; tokens via real `AuthService`). Seed a cert with a key via the certificate upload API or service on `app.db`:
- admin + correct password + `include_key:true` -> 200 with key material in the body (regression for #1390).
- admin + wrong password -> 403.
- non-admin (`RoleUser`, and `RolePassthrough`) + correct password -> 403.
- emergency bypass + `include_key:true` -> 403 (use the same bypass mechanism existing route tests use; if none exists, add a tiny pre-middleware setting `middleware.EmergencyBypassContextKey` true on a router wrapping `Register`, or reuse the emergency-token flow).
- Also: missing password -> 403 and does not consume budget (assert the next attempt is not 429 under a budget of 1); no token -> 401.
- Existing `TestRegister_CertificateKeyExportSharesLoginBudget` must remain green.

**Handler-level harness (supplement)**, in the handlers package: in-memory SQLite, `services.NewAuthService(db, config.Config{JWTSecret: "test-secret"})` (as in `middleware/auth_test.go:22-29`), real `middleware.AuthMiddleware` + `middleware.RequireRole(models.RoleAdmin)`, real JWTs. No `c.Set("user"...)` and no hand-set `userID`/`role` in the harness (the `mockAuthMiddleware` / `exportRouter` context stubs remain only in tests that are not about auth, and use the real key shapes). Cases: correct/wrong/missing password, non-admin, no token 401, bypass with and without key, user deleted after token issuance (401 via middleware), unknown-user handler branch (403 "user not found", real key shapes), `include_key:false` no password -> 200, `h.db == nil` -> 500, and guard call counts with a fake `PasswordAttemptGuard` (missing password: 0 calls; wrong/correct: 1 call each; guard false -> 429 and `CheckPassword` not reached; bypass/non-admin: 0 calls).

Legacy tests asserting `"not-a-map"` / missing `id` branches are deleted since those branches no longer exist.

**E2E** (`tests/certificate-export.spec.ts`, firefox only locally):
- Verified fixture facts: `createCustomCertViaAPI` (line 41) uploads both `certificate_file` and `key_file` (`REAL_TEST_KEY`), so the seeded cert has a key. The admin password is **not** currently exposed to this spec: `tests/auth.setup.ts:24` defines `TEST_PASSWORD = process.env.E2E_TEST_PASSWORD || 'TestPassword123!'` as a non-exported const, and `tests/fixtures/auth-fixtures.ts:75` exports a *different* `TEST_PASSWORD` (`'TestPass123!'`) for created users -- do not use that one. Task: export the auth.setup value from `tests/constants.ts` (single definition, have `auth.setup.ts` import it) and use it in the new scenario.
- Add "should download PEM with private key when correct password is entered" (`test.fixme` in commit 1, enabled in commit 3): check include-key, fill `#export-password`, submit, expect a download event `.pem`.
- **Drop the wrong-password E2E scenario.** It burns the shared 10 per 600 s login budget and is fully covered by the route-level test.
- Run list (firefox, foreground): `npx playwright test tests/certificate-export.spec.ts tests/security-enforcement/authorization-rbac.spec.ts --project=firefox`. `authorization-rbac.spec.ts:321` posts `{}` to the export route (400 on missing `format`) so it is unaffected, but belongs in the run.

## 6. Docs, changelog, commit scope

- Do not hand-edit `CHANGELOG.md` (release-please driven).
- **Scope: `fix(security):`** (user decision, 2026-09-29). The subject must stay vague per CLAUDE.md: `fix(security): restore re-authentication safeguard on sensitive exports`. Do not name the vulnerability class or the code path in the subject or body of any commit.
- Docs: `docs/api.md` short note on the export endpoint (admin only; `include_key` requires the caller's account password; **not available to emergency-bypass sessions**; login-class rate limit already documented at line 1781) and one sentence in `docs/features/ssl-certificates.md`. Both are in manifested directories, no manifest change. The bypass behavior change must be called out in the docs and the PR body.

## 7. Risks / edge cases

- Behavior change: emergency-bypass sessions can no longer export keys (intentional; PR body + docs). Bypass detection checks `userID == 0` and `middleware.IsEmergencyBypass(c)` together.
- The fix cannot widen access beyond admin + own password.
- Rate limiter: guard charged once per verified attempt; route-level tests use an explicit large budget except the shared-budget test.
- Changing `First(&user, userID)` to `First(&user, "id = ?", userID)` with a typed uint avoids inline-condition ambiguity; the GORM scan is run for this reason.
- Missing password stays 403 (existing contract).

## 8. Commit Slicing Strategy (ONE PR into `development`)

No skipped tests and no red commits. Red-then-green evidence goes in the PR body: before committing, run the new Go tests locally against the unmodified handler, capture the failing output (403 "authentication required"), then apply the fix.

1. **`test: add certificate key export E2E scenario (fixme)`**
   Files: `tests/certificate-export.spec.ts` (`test.fixme` scenario), `tests/constants.ts` + `tests/auth.setup.ts` (shared admin password constant), optionally no-behavior-change harness helpers. Gate: `npx playwright test tests/certificate-export.spec.ts --project=firefox` (fixme reports as skipped, rest green); `cd backend && go build ./...`.
2. **`fix(security): restore re-authentication safeguard on sensitive exports`**
   Handler change (section 3) together with ALL Go tests, new and rewritten (every file in section 4 under `backend/`). Green at HEAD. Gate: `cd backend && go build ./... && go test ./internal/api/handlers/... ./internal/api/middleware/... ./internal/api/routes/...`; `./scripts/scan-gorm-security.sh --check`; `make lint-fast`.
3. **`test: enable certificate key export E2E scenario` + docs** (may be two commits: `test:` then `docs: document private key export re-authentication`)
   Flip `test.fixme` -> `test`; docs edits. Gate: rebuild E2E container (`.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`), then the section 5 Playwright run list.

Rollback: revert the PR; no schema or data impact. Contingency: if bypass-reject proves too strict, relax only step 3 of section 3.

## 9. Definition of Done (run all foreground/blocking)

1. `cd /projects/Charon && npx playwright test tests/certificate-export.spec.ts tests/security-enforcement/authorization-rbac.spec.ts --project=firefox`
2. `cd backend && go test ./internal/api/handlers/... ./internal/api/middleware/... ./internal/api/routes/...`
3. `./scripts/scan-gorm-security.sh --check` (zero CRITICAL/HIGH; run because the `First(&user, ...)` argument changes)
4. `bash scripts/local-patch-report.sh`
5. `lefthook run pre-commit`; `make lint-fast`; `make lint-backend` before PR
6. `scripts/go-test-coverage.sh` (>= 85%)
7. `cd backend && go build ./...`
8. Frontend untouched: type-check/coverage not required. CodeQL/Trivy local scans deferred to CI (`fix(security):` scope but no new feature surface; CI runs both on every PR).
9. No debug prints/commented code; PR body has no session ID/link and includes the red-then-green evidence and the bypass behavior-change note.

## 10. Acceptance criteria mapping

- (1) Admin + correct password exports with key: route-level test through full `Register()` + E2E download.
- (2) Wrong/missing password and non-admin rejected: route-level tests (non-admin already blocked by `RequireRole` at routes.go:1074, plus handler-level `requireAdmin`).
- (3) Tests exercise the real `AuthMiddleware` with real JWTs from `AuthService`; no hand-injected context values in auth-relevant tests.

## 11. Follow-ups (not in this PR)

1. Low priority: `CertificateExportDialog` shows a generic axios message because the blob `responseType` makes error bodies Blobs; decode the Blob error body to show the server message. Tracked in #1414.

## 12. Open questions (resolved recommendations)

- Commit scope: `fix(security):` with the vague subject above (user decision). Bypass rejection for private-key export: approved by the user.
- Emergency bypass + `include_key:true`: reject with 403 (stricter); `include_key:false` still allowed.
- Missing password: stays 403.
- Commit 1 red-commit concern: resolved, no skipped tests; fix and all Go tests land together in commit 2, evidence in the PR body.
- Route-level test: mandatory.
- Follow-up issues: listed in section 11; to be filed by the user/orchestrator, not by the planner.

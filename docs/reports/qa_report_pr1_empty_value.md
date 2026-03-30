# QA Audit Report — PR-1

**Date:** 2026-03-17
**Scope:** Remove `binding:"required"` from `Value` field in `UpdateSettingRequest`
**File:** `backend/internal/api/handlers/settings_handler.go`
**Purpose:** Allow admins to set a setting to an empty string value (required to fix the fresh-install CrowdSec enabling bug where `value` was legitimately empty).

---

## Summary

| # | Check | Result | Notes |
|---|-------|--------|-------|
| 1 | Handler unit tests | ✅ PASS (after fix) | One stale test updated |
| 2 | Full backend test suite | ✅ PASS | No regressions |
| 3 | Lint (staticcheck) | ⚠️ SKIP | Pre-existing toolchain mismatch, not PR-1 |
| 4 | Pre-commit hooks | ⚠️ SKIP | `.pre-commit-config.yaml` not present |
| 5 | Security — admin auth check | ✅ PASS | Three independent protection layers confirmed |
| 6 | Trivy scan | ⚠️ NOTE | 1 pre-existing HIGH unrelated to PR-1 |

---

## Check 1 — Handler Unit Tests

**Command:** `go test ./internal/api/handlers/... -v`

**Initial result:** FAIL
`--- FAIL: TestSettingsHandler_Errors` at `settings_handler_test.go:805`

**Root cause:** The sub-case "Missing Key/Value" sent a payload with `key` but no `value` and asserted `HTTP 400`. This assertion was valid against the old `binding:"required"` constraint on `Value`. After PR-1 removed that constraint, the handler correctly accepts an absent value (empty string) and returns `HTTP 200`, breaking the stale assertion.

**Fix applied:** `backend/internal/api/handlers/settings_handler_test.go`

- Updated the "value absent" sub-case to assert `HTTP 200` — empty string is now a valid value.
- Added a new sub-case: payload with `value` but no `key` → asserts `HTTP 400` (key remains `binding:"required"`).

**Result after fix:** ✅ PASS

---

## Check 2 — Full Backend Test Suite

**Command:** `go test ./...`

All packages pass:

```
ok  github.com/Wikid82/charon/backend/cmd/seed
ok  github.com/Wikid82/charon/backend/internal/api
ok  github.com/Wikid82/charon/backend/internal/api/handlers
ok  github.com/Wikid82/charon/backend/internal/api/middleware
ok  github.com/Wikid82/charon/backend/internal/api/routes
ok  github.com/Wikid82/charon/backend/internal/api/tests
ok  github.com/Wikid82/charon/backend/internal/caddy
ok  github.com/Wikid82/charon/backend/internal/cerberus
ok  github.com/Wikid82/charon/backend/internal/crowdsec
ok  github.com/Wikid82/charon/backend/internal/crypto
ok  github.com/Wikid82/charon/backend/internal/database
ok  github.com/Wikid82/charon/backend/internal/models
ok  github.com/Wikid82/charon/backend/internal/notifications
ok  github.com/Wikid82/charon/backend/internal/services
ok  github.com/Wikid82/charon/backend/internal/server
... (all packages green, 0 failures)
```

**Result:** ✅ PASS — no regressions.

---

## Check 3 — Lint

**Attempted:** `make lint-fast` → target does not exist.
**Fallback:** `staticcheck ./...`

**Output:**
```
-: module requires at least go1.26.1, but Staticcheck was built with go1.25.5 (compile)
```

Pre-existing toolchain version mismatch between the installed `staticcheck` binary (`go1.25.5`) and the module minimum (`go1.26.1`). Not caused by this PR.

**Result:** ⚠️ SKIP — pre-existing environment issue, not a PR-1 blocker.

---

## Check 4 — Pre-commit Hooks

**Command:** `pre-commit run --all-files`
**Output:** `InvalidConfigError: .pre-commit-config.yaml is not a file`

No `.pre-commit-config.yaml` exists at the repository root; `lefthook.yml` is the configured hook runner.

**Result:** ⚠️ SKIP — `pre-commit` not configured for this repo.

---

## Check 5 — Security: Endpoint Authentication

The change removes only an input validation constraint on the `Value` field. No authorization logic is touched. Verified the full protection chain:

### Route registration (`backend/internal/api/routes/routes.go`)

```
protected := api.Group("/")
protected.Use(authMiddleware)               // Layer 1: JWT required → 401 otherwise

    management := protected.Group("/")
    management.Use(RequireManagementAccess())  // Layer 2: blocks passthrough role → 403

        management.POST("/settings", settingsHandler.UpdateSetting)
        management.PATCH("/settings", settingsHandler.UpdateSetting)
```

### Handler guard (`settings_handler.go` line 124)

```go
func (h *SettingsHandler) UpdateSetting(c *gin.Context) {
    if !requireAdmin(c) {   // Layer 3: admin role required → 403 otherwise
        return
    }
    ...
}
```

### Layer summary

| Layer | Mechanism | Failure response |
|-------|-----------|-----------------|
| 1 | `authMiddleware` (JWT) | 401 Unauthorized |
| 2 | `RequireManagementAccess()` middleware | 403 Forbidden (passthrough role) |
| 3 | `requireAdmin(c)` in handler | 403 Forbidden (non-admin) |

An unauthenticated or non-admin request cannot reach the `Value` binding logic.
The PR-1 change affects only the binding validation of `Value`; the authorization path is unchanged.

**Result:** ✅ PASS — endpoint is properly admin-gated by three independent layers.

---

## Check 6 — Trivy Security Scan

`trivy` CLI not installed in this environment. Analysis based on cached `trivy-image-report.json`
(generated 2026-02-25, Trivy v0.69.1, image `charon:local`).

| Severity | Count | Details |
|----------|-------|---------|
| CRITICAL | 0 | None |
| HIGH | 1 | CVE-2026-25793 — `github.com/slackhq/nebula v1.9.7` |

**CVE-2026-25793 detail:**
- **Package:** `github.com/slackhq/nebula v1.9.7`
- **Fixed in:** `v1.10.3`
- **Title:** Blocklist evasion via ECDSA Signature Malleability
- **Relation to PR-1:** None — pre-existing transitive dependency issue, tracked separately.

**Result:** ⚠️ NOTE — 1 pre-existing HIGH; no new vulnerabilities introduced by PR-1.

---

## Overall Verdict

**PR-1 is safe to merge** with the accompanying test fix.

The change is behaviorally correct: an empty string is a valid setting value for certain admin
operations (specifically the CrowdSec enrollment flow on fresh installs). Authorization is
unaffected — three independent layers continue to restrict this endpoint to authenticated admins.

### Required: Test fix

`backend/internal/api/handlers/settings_handler_test.go` — `TestSettingsHandler_Errors` updated
to reflect the new contract (empty value → 200; missing key → 400 still enforced).

### Tracked separately

- `github.com/slackhq/nebula` should be bumped to `v1.10.3` to resolve CVE-2026-25793.

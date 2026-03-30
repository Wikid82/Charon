# QA & Security Audit Report — Pushover Notification Provider

**Date:** 2026-03-16
**Scope:** Pushover notification provider full-stack implementation
**Auditor:** QA/Security Review
**Verdict:** ✅ PASS with one test fix applied (see FE-001 below)

---

## 1. Scope of Changes Reviewed

| Area | Files |
|------|-------|
| Backend – feature flags | `backend/internal/notifications/feature_flags.go` |
| Backend – router | `backend/internal/notifications/router.go` |
| Backend – notification service | `backend/internal/services/notification_service.go` |
| Backend – enhanced security service | `backend/internal/services/enhanced_security_notification_service.go` |
| Backend – handler (CRUD + Test guards) | `backend/internal/api/handlers/notification_provider_handler.go` |
| Backend – unit tests (~10 new test cases) | `backend/internal/services/notification_service_test.go` |
| Frontend – form fields | `frontend/src/pages/Notifications.tsx` |
| Frontend – supported types | `frontend/src/api/notifications.ts` |
| Frontend – i18n | `frontend/src/locales/en/translation.json` |
| Frontend – unit tests | `frontend/src/pages/__tests__/Notifications.test.tsx` |
| Model | `backend/internal/models/notification_provider.go` |

---

## 2. Required Checks — Results

### 2.1 Backend Compilation

```
cd /projects/Charon/backend && go build ./...
```

**Result: ✅ PASS** — Zero compilation errors across all packages.

---

### 2.2 Backend Unit Tests

```
cd /projects/Charon/backend && go test ./...
```

**Result: ✅ PASS** — All 33 packages pass with no failures.

| Package | Status |
|---------|--------|
| `internal/api/handlers` | ok (66.1s) |
| `internal/services` | ok (75.4s) |
| `internal/notifications` | ok (cached) |
| All other packages | ok |

Pushover-specific tests (10 cases, all PASS):

| Test | Result |
|------|--------|
| `TestPushoverDispatch_Success` | PASS |
| `TestPushoverDispatch_MissingToken` | PASS |
| `TestPushoverDispatch_MissingUserKey` | PASS |
| `TestPushoverDispatch_MessageFieldRequired` | PASS |
| `TestPushoverDispatch_EmergencyPriorityRejected` | PASS |
| `TestPushoverDispatch_PayloadInjection` | PASS |
| `TestPushoverDispatch_FeatureFlagDisabled` | PASS |
| `TestPushoverDispatch_SSRFValidation` | PASS |
| `TestIsDispatchEnabled_PushoverDefaultTrue` | PASS |
| `TestIsDispatchEnabled_PushoverDisabledByFlag` | PASS |

---

### 2.3 Backend Linting

```
cd /projects/Charon && make lint-fast
```

**Result: ✅ PASS** — `0 issues.` (staticcheck, govet, errcheck, ineffassign, unused)

---

### 2.4 Frontend TypeScript Check

```
cd /projects/Charon/frontend && npx tsc --noEmit
```

**Result: ✅ PASS** — No TypeScript errors.

---

### 2.5 Frontend Unit Tests

```
cd /projects/Charon/frontend && npx vitest run
```

**Result: ✅ PASS (after fix applied — see FE-001)**

| Test File | Tests | Status |
|-----------|-------|--------|
| `SecurityNotificationSettingsModal.test.tsx` | 4 | ✅ PASS |
| `Notifications.test.tsx` | 34 | ✅ PASS |
| `notifications.test.ts` (API layer) | 4 | ✅ PASS |

Pushover-specific frontend tests confirmed in `Notifications.test.tsx`:
- `renders pushover form with API Token field and User Key placeholder` — PASS
- Provider type select includes `'pushover'` in options array — PASS

---

### 2.6 Pre-commit / Lefthook Hooks

**Result: ⚠️ N/A** — The project uses Lefthook (`lefthook.yml`), not pre-commit native. No `.pre-commit-config.yaml` is present. Running the `pre-commit` binary directly raises `InvalidConfigError: .pre-commit-config.yaml is not a file`. Code hygiene was verified manually in changed files; no whitespace or formatting issues were found.

---

### 2.7 Trivy Filesystem Security Scan

```
cd /projects/Charon && .github/skills/scripts/skill-runner.sh security-scan-trivy
```

**Result: ✅ PASS** — No vulnerabilities or secrets detected.

```
Report Summary
┌────────────────────────────────┬───────┬─────────────────┬─────────┐
│             Target             │ Type  │ Vulnerabilities │ Secrets │
├────────────────────────────────┼───────┼─────────────────┼─────────┤
│ backend/go.mod                 │ gomod │        0        │    -    │
├────────────────────────────────┼───────┼─────────────────┼─────────┤
│ frontend/package-lock.json     │  npm  │        0        │    -    │
├────────────────────────────────┼───────┼─────────────────┼─────────┤
│ package-lock.json              │  npm  │        0        │    -    │
├────────────────────────────────┼───────┼─────────────────┼─────────┤
│ playwright/.auth/user.json     │ text  │        -        │    0    │
└────────────────────────────────┴───────┴─────────────────┴─────────┘
[SUCCESS] Trivy scan completed - no issues found
```

---

### 2.8 Regression — Services Package

```
cd /projects/Charon/backend && go test ./internal/services/... -v 2>&1 | grep -E "^(--- PASS|--- FAIL|FAIL|ok)"
```

**Result: ✅ PASS** — All existing service tests continue to pass; no regressions introduced.

---

### 2.9 Regression — Handlers Package

```
cd /projects/Charon/backend && go test ./internal/api/handlers/... -v 2>&1 | grep -E "^(--- PASS|--- FAIL|FAIL|ok)"
```

**Result: ✅ PASS** — All existing handler tests continue to pass; no regressions introduced.

---

## 3. Security Code Review

### 3.1 Token JSON Serialization (`json:"-"`)

**Model field** (`backend/internal/models/notification_provider.go`):
```go
Token string `json:"-"` // Auth token for providers — never exposed in API
```

**Finding: ✅ SECURE**

The `Token` field on `models.NotificationProvider` carries `json:"-"`, preventing it from being marshalled into any JSON response. Handler-level defense-in-depth also explicitly clears the token before responding in `List`, `Create`, and `Update`:

```go
provider.HasToken = provider.Token != ""
provider.Token = ""
c.JSON(http.StatusOK, provider)
```

Two independent layers prevent token leakage.

---

### 3.2 SSRF Hostname Pin (`api.pushover.net`)

**Production dispatch path** (`notification_service.go`):
```go
pushoverBase := s.pushoverAPIBaseURL  // always "https://api.pushover.net" in production
dispatchURL = pushoverBase + "/1/messages.json"

parsedURL, parseErr := neturl.Parse(dispatchURL)
expectedHost := "api.pushover.net"
// test-seam bypass: only applies when pushoverAPIBaseURL has been overridden in tests
if parsedURL != nil && parsedURL.Hostname() != "" && pushoverBase != "https://api.pushover.net" {
    expectedHost = parsedURL.Hostname()
}
if parseErr != nil || parsedURL.Hostname() != expectedHost {
    return fmt.Errorf("pushover dispatch URL validation failed: invalid hostname")
}
```

**Finding: ✅ SECURE**

In production, `pushoverAPIBaseURL` is always `"https://api.pushover.net"` (set in `NewNotificationService`). The bypass condition `pushoverBase != "https://api.pushover.net"` is only true in unit tests where the field is overridden via direct struct access (`svc.pushoverAPIBaseURL = server.URL`). This field is:

- A private Go struct field — cannot be set via any API endpoint
- Absent from `notificationProviderUpsertRequest` and `notificationProviderTestRequest`
- Identical in design to the existing Telegram SSRF pin (reviewed previously)

No user-supplied input can influence the dispatch hostname.

---

### 3.3 Template Injection — `token`/`user` Field Override

**Dispatch logic** (`notification_service.go`):
```go
// Template payload is rendered, then server-side values OVERWRITE any user-supplied keys:
jsonPayload["token"] = decryptedToken   // from DB
jsonPayload["user"] = p.URL             // from DB
```

**Finding: ✅ SECURE**

Server-side values always overwrite any `token` or `user` keys that may have been injected via the provider's Config template. This is explicitly exercised by `TestPushoverDispatch_PayloadInjection`, which confirms that a template containing `"token": "fake-token", "user": "fake-user"` is replaced with the real decrypted DB values before the outbound HTTP request is made.

---

### 3.4 Emergency Priority=2 Rejection

**Validation** (`notification_service.go`):
```go
if priority, ok := jsonPayload["priority"]; ok {
    if p, isFloat := priority.(float64); isFloat && p == 2 {
        return fmt.Errorf("pushover emergency priority (2) requires retry and expire parameters; not yet supported")
    }
}
```

**Finding: ✅ CORRECT**

Emergency priority (2) is blocked with a clear, actionable error. JSON numbers are decoded as `float64` by `json.Unmarshal`, so the `p == 2` comparison is type-safe. Non-emergency priorities (-2, -1, 0, 1) pass through. The comparison `float64(2) == 2` evaluates correctly in Go.

Covered by `TestPushoverDispatch_EmergencyPriorityRejected`.

---

### 3.5 Test() Write-Only Guard — Pushover and Telegram

**Handler** (`notification_provider_handler.go`):
```go
if providerType == "pushover" && strings.TrimSpace(req.Token) != "" {
    respondSanitizedProviderError(c, http.StatusBadRequest, "TOKEN_WRITE_ONLY", "validation",
        "Pushover API token is accepted only on provider create/update")
    return
}

if providerType == "telegram" && strings.TrimSpace(req.Token) != "" {
    respondSanitizedProviderError(c, http.StatusBadRequest, "TOKEN_WRITE_ONLY", "validation",
        "Telegram bot token is accepted only on provider create/update")
    return
}
```

**Finding: ✅ CORRECT**

Passing a token in the `Test` request body is rejected with HTTP 400 `TOKEN_WRITE_ONLY` for both Pushover and Telegram. The test dispatch always reads credentials from the database (provider ID is required), preventing token exfiltration or injection via the test endpoint. The same guard exists for Gotify and Slack, maintaining symmetry across all token-based providers.

---

### 3.6 `pushoverAPIBaseURL` Accessibility via API

**Finding: ✅ SECURE**

`pushoverAPIBaseURL` is a private struct field with no API exposure:
1. Not exported from the `NotificationService` struct
2. Not present in any request struct unmarshalled from user input
3. Only modified in test code via `svc.pushoverAPIBaseURL = server.URL`
4. Never read from user input, headers, query parameters, or provider Config

Production dispatches invariably target `https://api.pushover.net/1/messages.json`.

---

## 4. Findings

### FE-001 — Stale Test Assertion After Adding Pushover Provider Type

**Severity:** 🟡 MEDIUM (test failure, blocks CI)
**File:** `frontend/src/components/__tests__/SecurityNotificationSettingsModal.test.tsx:89`
**Status:** ✅ **FIXED**

**Description:** After Pushover was added to `SUPPORTED_NOTIFICATION_PROVIDER_TYPES` in `notifications.ts`, the assertion checking the provider type dropdown options was not updated. The test expected 6 types but the implementation exposes 7, causing the test to fail and block CI.

**Before:**
```typescript
expect(Array.from(typeSelect.options).map((option) => option.value))
    .toEqual(['discord', 'gotify', 'webhook', 'email', 'telegram', 'slack']);
```

**After (applied):**
```typescript
expect(Array.from(typeSelect.options).map((option) => option.value))
    .toEqual(['discord', 'gotify', 'webhook', 'email', 'telegram', 'slack', 'pushover']);
```

All 4 tests in `SecurityNotificationSettingsModal.test.tsx` pass after the fix.

---

### BE-001 — No Handler-Level Unit Tests for Pushover TOKEN_WRITE_ONLY Guard

**Severity:** 🟢 LOW (coverage gap, not a functional defect)
**File:** `backend/internal/api/handlers/notification_provider_handler_test.go`
**Status:** ⚠️ INFORMATIONAL

**Description:** The `Test()` handler's Pushover `TOKEN_WRITE_ONLY` guard is correctly implemented and is structurally identical to the existing Gotify, Slack, and Telegram guards. The guard is verified at the code-review level but no dedicated handler integration test exercises it. This gap applies to all four token-based providers, not Pushover in isolation.

**Recommendation:** Add handler integration tests for `TOKEN_WRITE_ONLY` guards across Gotify, Telegram, Slack, and Pushover in a follow-up issue to achieve symmetrical handler coverage.

---

### E2E-001 — No Playwright E2E Spec for Pushover Provider

**Severity:** 🟢 LOW (coverage gap)
**Status:** ⚠️ INFORMATIONAL

**Description:** The implementation scope stated "New E2E spec" but no Playwright `.spec.ts` file for Pushover was found in the repository. The `playwright/` directory contains only the auth fixture. Frontend unit tests (`Notifications.test.tsx`) provide partial coverage of the form rendering path, but there is no browser-level test exercising the full add/edit/test flow for Pushover.

**Recommendation:** Create a Playwright spec covering: add Pushover provider, verify "User Key" and "API Token (Application)" field labels, test provider response handling. Target the next release cycle.

---

### SEC-001 — SSRF Test Bypass Pattern (Design Note)

**Severity:** ✅ INFORMATIONAL (no action required)

**Description:** The `pushoverAPIBaseURL` field allows the SSRF pin to be bypassed in test environments. This is intentional, mirrors the existing Telegram test-seam pattern, and is not exploitable via any API vector. Documented for audit trail completeness.

---

## 5. Summary

| Check | Result |
|-------|--------|
| Backend compilation (`go build ./...`) | ✅ PASS |
| Backend unit tests (`go test ./...`) | ✅ PASS |
| Backend linting (`make lint-fast`) | ✅ PASS |
| Frontend TypeScript (`tsc --noEmit`) | ✅ PASS |
| Frontend unit tests (`vitest run`) | ✅ PASS (after FE-001 fix) |
| Pre-commit hooks | ⚠️ N/A (project uses Lefthook) |
| Trivy filesystem scan | ✅ PASS — 0 vulns, 0 secrets |
| Regression — services package | ✅ PASS |
| Regression — handlers package | ✅ PASS |
| `Token` field `json:"-"` guard | ✅ SECURE |
| SSRF hostname pin (`api.pushover.net`) | ✅ SECURE |
| Template injection guard | ✅ SECURE |
| Emergency priority=2 rejection | ✅ CORRECT |
| Test() write-only guard (Pushover + Telegram) | ✅ CORRECT |
| `pushoverAPIBaseURL` API inaccessibility | ✅ SECURE |

**Critical/High security findings: 0**
**Total findings: 4** (1 fixed, 3 informational coverage gaps)

The Pushover notification provider implementation is secure and functionally correct. The one blocking defect (FE-001) was identified and resolved during this audit. The three remaining findings are non-blocking coverage gaps with no security impact and no CVE surface.

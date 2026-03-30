# Ntfy Notification Provider — Implementation Specification

## 1. Introduction

### Overview

Add **Ntfy** (<https://ntfy.sh>) as a notification provider in Charon, following
the same wrapper pattern used by Gotify, Telegram, Slack, and Pushover. Ntfy is
an HTTP-based pub/sub notification service that supports self-hosted and
cloud-hosted instances. Users publish messages by POSTing JSON to a topic URL,
optionally with an auth token.

### Objectives

1. Users can create/edit/delete an Ntfy notification provider via the Management UI.
2. Ntfy dispatches support all three template modes (minimal, detailed, custom).
3. Ntfy respects the global notification engine kill-switch and its own per-provider feature flag.
4. Security: auth tokens are stored securely (never exposed in API responses or logs).
5. Full E2E and unit test coverage matching the existing provider test suite.

---

## 2. Research Findings

### Existing Architecture

Charon's notification engine does **not** use a Go interface pattern. Instead, it
routes on string type values (`"discord"`, `"gotify"`, `"webhook"`, etc.) across
~15 switch/case + hardcoded lists in both backend and frontend.

**Key code paths per provider type:**

| Layer | Location | Mechanism |
|-------|----------|-----------|
| Model | `backend/internal/models/notification_provider.go` | Generic — no per-type changes needed |
| Service — type allowlist | `notification_service.go:139` `isSupportedNotificationProviderType()` | `switch` on type string |
| Service — flag routing | `notification_service.go:148` `isDispatchEnabled()` | `switch` → feature flag lookup |
| Service — dispatch | `notification_service.go:381` `sendJSONPayload()` | Type-specific validation + URL / header construction |
| Feature flags | `notifications/feature_flags.go` | Const strings for settings DB keys |
| Router | `notifications/router.go:10` `ShouldUseNotify()` | `switch` on type → flag map lookup |
| Handler — create validation | `notification_provider_handler.go:185` | Hardcoded `!=` chain |
| Handler — update validation | `notification_provider_handler.go:245` | Hardcoded `!=` chain |
| Handler — URL validation | `notification_provider_handler.go:372` | Slack special-case (optional URL) |
| Frontend — type array | `api/notifications.ts:3` | `SUPPORTED_NOTIFICATION_PROVIDER_TYPES` const |
| Frontend — sanitize | `api/notifications.ts` `sanitizeProviderForWriteAction()` | Token mapping per type |
| Frontend — form | `pages/Notifications.tsx` | `<option>`, URL label, token field, placeholder, `supportsJSONTemplates()`, `normalizeProviderPayloadForSubmit()`, `useEffect` token cleanup |
| Frontend — unit test mock | `pages/__tests__/Notifications.test.tsx` | Mock of `SUPPORTED_NOTIFICATION_PROVIDER_TYPES` |
| i18n | `locales/{en,de,fr,zh,es}/translation.json` | `notificationProviders.*` keys |

### Ntfy HTTP API Reference

Ntfy accepts a JSON POST to a topic URL:

```
POST https://ntfy.sh/my-topic
Authorization: Bearer tk_abc123   # optional
Content-Type: application/json

{
  "topic": "my-topic",       // optional if encoded in URL
  "message": "Hello!",       // required
  "title": "Alert Title",    // optional
  "priority": 3,             // optional (1-5, default 3)
  "tags": ["warning"]        // optional
}
```

This maps directly to the Gotify dispatch pattern: POST JSON to `p.URL` with an
optional `Authorization: Bearer <token>` header.

---

## 3. Technical Specifications

### 3.1 Provider Interface / Contract (Type Registration)

Ntfy uses type string `"ntfy"`. Every switch/case and hardcoded type list must
include this value. The following table is the exhaustive changeset:

| # | File | Function / Location | Change |
|---|------|---------------------|--------|
| 1 | `backend/internal/services/notification_service.go` | `isSupportedNotificationProviderType()` ~L139 | Add `case "ntfy": return true` |
| 2 | `backend/internal/services/notification_service.go` | `isDispatchEnabled()` ~L148 | Add `case "ntfy":` with `FlagNtfyServiceEnabled`, default `true` |
| 3 | `backend/internal/services/notification_service.go` | `sendJSONPayload()` — validation block ~L460 | Add ntfy JSON validation: require `"message"` field |
| 4 | `backend/internal/services/notification_service.go` | `sendJSONPayload()` — dispatch routing ~L530 | Add ntfy dispatch block (URL from `p.URL`, optional Bearer auth from `p.Token`) |
| 5 | `backend/internal/services/notification_service.go` | `supportsJSONTemplates()` ~L131 | Add `case "ntfy": return true` — gates `SendExternal()` JSON dispatch path |
| 6 | `backend/internal/services/notification_service.go` | `sendJSONPayload()` — outer gating condition ~L525 | Add `\|\| providerType == "ntfy"` to the if-chain that enters the dispatch block |
| 7 | `backend/internal/services/notification_service.go` | `CreateProvider()` — token-clearing condition ~L851 | Add `&& provider.Type != "ntfy"` (and `&& provider.Type != "pushover"` — existing bug fix) to prevent token being silently cleared on creation |
| 8 | `backend/internal/services/notification_service.go` | `UpdateProvider()` — token preservation ~L886 | Add `\|\| provider.Type == "ntfy"` (and `\|\| provider.Type == "pushover"` — existing bug fix) to preserve token on update when not re-entered |
| 9 | `backend/internal/notifications/feature_flags.go` | Constants | Add `FlagNtfyServiceEnabled = "feature.notifications.service.ntfy.enabled"` |
| 10 | `backend/internal/notifications/router.go` | `ShouldUseNotify()` | Add `case "ntfy": return flags[FlagNtfyServiceEnabled]` |
| 11 | `backend/internal/api/handlers/notification_provider_handler.go` | `Create()` ~L185 | Add `&& providerType != "ntfy"` to validation chain |
| 12 | `backend/internal/api/handlers/notification_provider_handler.go` | `Update()` ~L245 | Add `&& providerType != "ntfy"` to validation chain |
| 13 | `backend/internal/api/handlers/notification_provider_handler.go` | `Update()` — token preservation ~L250 | Add `\|\| providerType == "ntfy"` to the condition that preserves existing token when update payload omits it |
| 14 | `frontend/src/api/notifications.ts` | `SUPPORTED_NOTIFICATION_PROVIDER_TYPES` | Add `'ntfy'` to array |
| 15 | `frontend/src/api/notifications.ts` | `sanitizeProviderForWriteAction()` | Add `'ntfy'` to token-bearing types |
| 16 | `frontend/src/pages/Notifications.tsx` | `supportsJSONTemplates()` | Add `|| t === 'ntfy'` |
| 17 | `frontend/src/pages/Notifications.tsx` | `normalizeProviderPayloadForSubmit()` | Add `'ntfy'` to token-bearing types |
| 18 | `frontend/src/pages/Notifications.tsx` | `useEffect` token cleanup | Add `type !== 'ntfy'` to the cleanup condition |
| 19 | `frontend/src/pages/Notifications.tsx` | `<select>` dropdown | Add `<option value="ntfy">Ntfy</option>` |
| 20 | `frontend/src/pages/Notifications.tsx` | URL label ternary | Ntfy uses default URL/Webhook label — no special label needed, falls through to default |
| 21 | `frontend/src/pages/Notifications.tsx` | Token field visibility | Add `isNtfy` to `(isGotify \|\| isTelegram \|\| isSlack \|\| isPushover \|\| isNtfy)` |
| 22 | `frontend/src/pages/Notifications.tsx` | Token field label | Add `isNtfy ? t('notificationProviders.ntfyAccessToken') : ...` |
| 23 | `frontend/src/pages/Notifications.tsx` | URL placeholder | Add ntfy case: `type === 'ntfy' ? 'https://ntfy.sh/my-topic'` |
| 24 | `frontend/src/pages/Notifications.tsx` | URL validation `required` | Ntfy requires URL — no change (default requires URL) |
| 25 | `frontend/src/pages/Notifications.tsx` | URL validation `validate` | Ntfy uses standard URL validation — no change (default validates URL) |
| 26 | `frontend/src/pages/Notifications.tsx` | `isNtfy` const | Add `const isNtfy = type === 'ntfy';` near L151 |
| 27 | `frontend/src/pages/__tests__/Notifications.test.tsx` | Mock array | Add `'ntfy'` to mock `SUPPORTED_NOTIFICATION_PROVIDER_TYPES` |
| 28 | `tests/settings/notifications.spec.ts` | Provider type options assertion ~L297 | Change `toHaveCount(7)` → `toHaveCount(8)`, add `'Ntfy'` to `toHaveText()` array |

### 3.2 Backend Implementation Details

#### 3.2.1 Feature Flag

**File:** `backend/internal/notifications/feature_flags.go`

```go
const FlagNtfyServiceEnabled = "feature.notifications.service.ntfy.enabled"
```

#### 3.2.2 Router

**File:** `backend/internal/notifications/router.go`

Add in `ShouldUseNotify()` switch:

```go
case "ntfy":
    return flags[FlagNtfyServiceEnabled]
```

#### 3.2.3 Service — Type Registration

**File:** `backend/internal/services/notification_service.go`

In `isSupportedNotificationProviderType()`:

```go
case "ntfy":
    return true
```

In `isDispatchEnabled()`:

```go
case "ntfy":
    return getFeatureFlagValue(db, notifications.FlagNtfyServiceEnabled, true)
```

#### 3.2.4 Service — JSON Validation (sendJSONPayload)

In the service-specific validation block (~L460), add before the default case:

```go
case "ntfy":
    if _, ok := payload["message"]; !ok {
        return fmt.Errorf("ntfy payload must include a 'message' field")
    }
```

> **Note:** Ntfy `priority` (1–5) can be set via custom templates by including a
> `"priority"` field in the JSON. No code change is needed — the validation only
> requires `"message"`.

#### 3.2.5 Service — supportsJSONTemplates + Outer Gating + Dispatch Routing

**supportsJSONTemplates()** (~L131): Add `"ntfy"` so `SendExternal()` dispatches
via the JSON path:

```go
case "ntfy":
    return true
```

**Outer gating condition** (~L525): The dispatch block is entered only when the
provider type matches an `if/else if` chain. The actual code uses `if` chains,
**not** `switch/case`. Add ntfy:

```go
// Before (actual code structure — NOT switch/case):
if providerType == "gotify" || providerType == "webhook" || providerType == "telegram" || providerType == "slack" || providerType == "pushover" {

// After:
if providerType == "gotify" || providerType == "webhook" || providerType == "telegram" || providerType == "slack" || providerType == "pushover" || providerType == "ntfy" {
```

**Dispatch routing** (~L540): Inside the dispatch block, add an ntfy branch
using the same `if/else if` pattern as existing providers:

```go
// Actual code uses if/else if — NOT switch/case:
} else if providerType == "ntfy" {
    dispatchURL = p.URL
    if strings.TrimSpace(p.Token) != "" {
        headers["Authorization"] = "Bearer " + strings.TrimSpace(p.Token)
    }
```

Then the existing `httpWrapper.Send(dispatchURL, headers, body)` call handles dispatch.

#### 3.2.6 Service — CreateProvider / UpdateProvider Token Preservation

**File:** `backend/internal/services/notification_service.go`

**`CreateProvider()` (~L851)** — token-clearing condition currently omits both
ntfy and pushover, silently clearing tokens on creation:

```go
// Before:
if provider.Type != "gotify" && provider.Type != "telegram" && provider.Type != "slack" {
    provider.Token = ""
}

// After (adds ntfy + fixes existing pushover bug):
if provider.Type != "gotify" && provider.Type != "telegram" && provider.Type != "slack" && provider.Type != "pushover" && provider.Type != "ntfy" {
    provider.Token = ""
}
```

**`UpdateProvider()` (~L886)** — token preservation condition currently omits
both ntfy and pushover, silently clearing tokens on update:

```go
// Before:
if provider.Type == "gotify" || provider.Type == "telegram" || provider.Type == "slack" {
    if strings.TrimSpace(provider.Token) == "" {
        provider.Token = existing.Token
    }
} else {
    provider.Token = ""
}

// After (adds ntfy + fixes existing pushover bug):
if provider.Type == "gotify" || provider.Type == "telegram" || provider.Type == "slack" || provider.Type == "pushover" || provider.Type == "ntfy" {
    if strings.TrimSpace(provider.Token) == "" {
        provider.Token = existing.Token
    }
} else {
    provider.Token = ""
}
```

> **Bonus bugfix:** The `pushover` additions fix a pre-existing bug where
> pushover tokens were silently cleared on create and update. This will be noted
> in the commit message for Commit 3.

#### 3.2.7 Handler — Type Validation + Token Preservation

**File:** `backend/internal/api/handlers/notification_provider_handler.go`

**`Create()` (~L185)** and **`Update()` (~L245)** type-validation chains:
Add `&& providerType != "ntfy"` so ntfy passes the supported-type check.

**`Update()` token preservation (~L250)**: The handler has its own token
preservation condition that runs before calling the service. Add ntfy:

```go
// Before:
if (providerType == "gotify" || providerType == "telegram" || providerType == "slack" || providerType == "pushover") && strings.TrimSpace(req.Token) == "" {
    req.Token = existing.Token
}

// After:
if (providerType == "gotify" || providerType == "telegram" || providerType == "slack" || providerType == "pushover" || providerType == "ntfy") && strings.TrimSpace(req.Token) == "" {
    req.Token = existing.Token
}
```

No URL validation special-case is needed for Ntfy (URL is required and follows
standard http/https format).

### 3.3 Frontend Implementation Details

#### 3.3.1 API Client

**File:** `frontend/src/api/notifications.ts`

```typescript
export const SUPPORTED_NOTIFICATION_PROVIDER_TYPES = [
  'discord', 'gotify', 'webhook', 'email', 'telegram', 'slack', 'pushover', 'ntfy'
] as const;
```

In `sanitizeProviderForWriteAction()`, add `'ntfy'` to the set of token-bearing
types so that the token field is properly mapped on create/update.

#### 3.3.2 Notifications Page

**File:** `frontend/src/pages/Notifications.tsx`

| Area | Change |
|------|--------|
| Type boolean | Add `const isNtfy = type === 'ntfy';` |
| `<select>` | Add `<option value="ntfy">Ntfy</option>` after Pushover |
| Token visibility | Change `(isGotify \|\| isTelegram \|\| isSlack \|\| isPushover)` to `(isGotify \|\| isTelegram \|\| isSlack \|\| isPushover \|\| isNtfy)` in 3 places: token field visibility, `normalizeProviderPayloadForSubmit()`, and `useEffect` token cleanup |
| Token label | Add `isNtfy ? t('notificationProviders.ntfyAccessToken') : ...` in the ternary chain |
| Token placeholder | Add ntfy case: `isNtfy ? t('notificationProviders.ntfyAccessTokenPlaceholder')` |
| URL label | Consider using `t('notificationProviders.ntfyTopicUrl')` (`"Topic URL"`) for a more descriptive label when ntfy is selected, instead of the default `"URL / Webhook URL"` |
| URL placeholder | Add `type === 'ntfy' ? 'https://ntfy.sh/my-topic'` in the ternary chain |
| `supportsJSONTemplates()` | Add `|| t === 'ntfy'` |

#### 3.3.3 i18n Strings

**Files:** `frontend/src/locales/{en,de,fr,zh,es}/translation.json`

Add to the `notificationProviders` section (after `pushoverUserKeyHelp`):

| Key | English Value |
|-----|---------------|
| `ntfy` | `"Ntfy"` |
| `ntfyAccessToken` | `"Access Token (optional)"` |
| `ntfyAccessTokenPlaceholder` | `"Enter your Ntfy access token"` |
| `ntfyAccessTokenHelp` | `"Required for password-protected topics on self-hosted instances. Not needed for public ntfy.sh topics. The token is stored securely and separately."` |
| `ntfyTopicUrl` | `"Topic URL"` |

For non-English locales, the keys should be added with English fallback values
(the community can translate later).

#### 3.3.4 Unit Test Mock + E2E Assertion Update

**File:** `frontend/src/pages/__tests__/Notifications.test.tsx`

Update the mocked `SUPPORTED_NOTIFICATION_PROVIDER_TYPES` array to include `'ntfy'`.
Update the test `'shows supported provider type options'` to expect 8 options instead of 7.

**File:** `tests/settings/notifications.spec.ts`

Update the E2E assertion at ~L297:
- `toHaveCount(7)` → `toHaveCount(8)`
- Add `'Ntfy'` to the `toHaveText()` array: `['Discord', 'Gotify', 'Generic Webhook', 'Email', 'Telegram', 'Slack', 'Pushover', 'Ntfy']`

### 3.4 Database Migration

**No schema changes required.** The existing `NotificationProvider` GORM model
already has all the fields Ntfy needs:

| Ntfy Concept | Model Field |
|--------------|-------------|
| Topic URL | `URL` |
| Auth token | `Token` (json:"-") |
| Has token indicator | `HasToken` (computed, gorm:"-") |

GORM AutoMigrate handles migrations from model definitions. No migration file
is needed.

### 3.5 Data Flow Diagram

```
User creates Ntfy provider via UI
  -> POST /api/v1/notifications/providers { type: "ntfy", url: "https://ntfy.sh/alerts", token: "tk_..." }
  -> Handler validates type is in allowed list
  -> Service stores provider in SQLite (token encrypted at rest)

Event triggers notification dispatch:
  -> SendExternal() filters enabled providers by event type preferences
  -> isDispatchEnabled("ntfy") -> checks FlagNtfyServiceEnabled setting
  -> sendJSONPayload() renders template -> validates payload has "message" field
  -> Constructs dispatch: POST to p.URL with Authorization: Bearer <token> header
  -> httpWrapper.Send(dispatchURL, headers, body) -> HTTP POST to Ntfy server
```

---

## 4. Implementation Plan

### Phase 1: Playwright E2E Tests (Test-First)

Write E2E tests that define the expected UI/UX behavior for Ntfy before
implementing the feature. Tests will initially fail and pass after implementation.

**Deliverables:**

| File | Description |
|------|-------------|
| `tests/settings/ntfy-notification-provider.spec.ts` | New file — form rendering, CRUD, token security, field toggling |
| `tests/settings/notifications-payload.spec.ts` | Add Ntfy to payload contract validation matrix |
| `tests/settings/notifications.spec.ts` | Update provider type dropdown assertions: `toHaveCount(7)` → `toHaveCount(8)`, add `'Ntfy'` to `toHaveText()` array |

**Test structure** (following telegram/pushover/slack pattern):

1. Form Rendering
   - Show token field when ntfy type selected
   - Verify token label shows "Access Token (optional)"
   - Verify URL placeholder shows "https://ntfy.sh/my-topic"
   - Verify JSON template section is shown for ntfy
   - Toggle fields when switching between ntfy and discord
2. CRUD Operations
   - Create ntfy provider with URL + token
   - Create ntfy provider with URL only (no token)
   - Edit ntfy provider (token field shows "Leave blank to keep")
   - Delete ntfy provider
3. Token Security
   - Verify token field is `type="password"`
   - Verify token is not exposed in API response row
4. Payload Contract
   - Valid ntfy payload with message field accepted
   - Missing message field rejected

### Phase 2: Backend Implementation

**Deliverables:**

| # | File | Changes |
|---|------|---------|
| 1 | `backend/internal/notifications/feature_flags.go` | Add `FlagNtfyServiceEnabled` constant |
| 2 | `backend/internal/notifications/router.go` | Add `"ntfy"` case in `ShouldUseNotify()` |
| 3 | `backend/internal/services/notification_service.go` | Add `"ntfy"` to `isSupportedNotificationProviderType()`, `isDispatchEnabled()`, `supportsJSONTemplates()`, outer gating condition, dispatch routing, `CreateProvider()` token chain, `UpdateProvider()` token chain. Fix pushover token-clearing bug in same conditions. |
| 4 | `backend/internal/api/handlers/notification_provider_handler.go` | Add `"ntfy"` to Create/Update type validation + Update token preservation |

**Backend Unit Tests:**

| File | New Tests |
|------|-----------|
| `backend/internal/notifications/router_test.go` | `TestShouldUseNotify_Ntfy` — flag on/off |
| `backend/internal/services/notification_service_test.go` | `TestIsSupportedNotificationProviderType_Ntfy`, `TestIsDispatchEnabled_Ntfy` |
| `backend/internal/services/notification_service_json_test.go` | `TestSendJSONPayload_Ntfy_Valid`, `TestSendJSONPayload_Ntfy_MissingMessage`, `TestSendJSONPayload_Ntfy_WithToken`, `TestSendJSONPayload_Ntfy_WithoutToken` |

### Phase 3: Frontend Implementation

**Deliverables:**

| # | File | Changes |
|---|------|---------|
| 1 | `frontend/src/api/notifications.ts` | Add `'ntfy'` to type array + sanitize function |
| 2 | `frontend/src/pages/Notifications.tsx` | Add `isNtfy`, dropdown option, token field wiring, URL placeholder, `supportsJSONTemplates()`, `normalizeProviderPayloadForSubmit()`, `useEffect` cleanup |
| 3 | `frontend/src/locales/en/translation.json` | Add `ntfy*` i18n keys |
| 4 | `frontend/src/locales/de/translation.json` | Add `ntfy*` i18n keys (English fallback) |
| 5 | `frontend/src/locales/fr/translation.json` | Add `ntfy*` i18n keys (English fallback) |
| 6 | `frontend/src/locales/zh/translation.json` | Add `ntfy*` i18n keys (English fallback) |
| 7 | `frontend/src/locales/es/translation.json` | Add `ntfy*` i18n keys (English fallback) |
| 8 | `frontend/src/pages/__tests__/Notifications.test.tsx` | Update mock array + option count assertion |

### Phase 4: Integration and Testing

1. Rebuild E2E Docker environment (`docker-rebuild-e2e`).
2. Run full Playwright suite (Firefox, Chromium, WebKit).
3. Run backend `go test ./...`.
4. Run frontend `npm test`.
5. Run GORM security scanner (changes touch service logic, not models — likely clean).
6. Verify E2E coverage via Vite dev server mode.

### Phase 5: Documentation and Deployment

1. Update `docs/features.md` — add Ntfy to supported notification providers list.
2. Update `CHANGELOG.md` — add `feat(notifications): add Ntfy notification provider`.

---

## 5. Acceptance Criteria

| # | Criterion | Validation Method |
|---|-----------|-------------------|
| AC-1 | User can select "Ntfy" from the provider type dropdown | E2E: `ntfy-notification-provider.spec.ts` form rendering tests |
| AC-2 | Topic URL field is required with standard http/https validation | E2E: form validation tests |
| AC-3 | Access Token field is shown as optional password field | E2E: token field visibility + type="password" check |
| AC-4 | Token is never exposed in API responses (has_token indicator only) | E2E: token security tests |
| AC-5 | JSON template section (minimal/detailed/custom) is available | E2E: template section visibility |
| AC-6 | Ntfy provider can be created, edited, deleted | E2E: CRUD tests |
| AC-7 | Test notification dispatches to Ntfy topic URL with correct headers | Backend unit test: sendJSONPayload ntfy dispatch |
| AC-8 | Missing `message` field in payload is rejected | Backend unit test + E2E payload validation |
| AC-9 | Feature flag `feature.notifications.service.ntfy.enabled` controls dispatch | Backend unit test: isDispatchEnabled + router |
| AC-10 | All 5 locales have ntfy i18n keys | Manual verification |
| AC-11 | No GORM security scanner CRITICAL/HIGH findings | GORM scanner `--check` |

---

## 6. Commit Slicing Strategy

### Decision: Single PR

**Rationale:** Ntfy is a self-contained, additive feature that does not touch
existing provider logic (only adds new cases to existing switch/case and if-chain
blocks). The changeset is small (~16 files, <300 lines of implementation + ~430
lines of tests) and stays within a single domain (notifications). A single PR is
straightforward to review and rollback. One bonus bugfix is included: pushover
token-clearing in `CreateProvider()`/`UpdateProvider()` is fixed in the same
lines being modified for ntfy.

**Trigger analysis:**
- Scope: Small — one new provider, no schema changes, no new packages.
- Risk: Low — all changes are additive `case`/`if` additions; the only behavior change to existing providers is fixing the pushover token-clearing bug (a correctness fix).
- Cross-domain: No — backend + frontend are in the same PR (standard for features).
- Review size: Moderate — well within single-PR comfort zone.

### Ordered Commits

| Commit | Scope | Files | Validation Gate |
|--------|-------|-------|-----------------|
| `1` | `test(e2e): add Ntfy notification provider E2E tests` | `tests/settings/ntfy-notification-provider.spec.ts`, `tests/settings/notifications-payload.spec.ts`, `tests/settings/notifications.spec.ts` | Tests compile (expected to fail until implementation) |
| `2` | `feat(notifications): add Ntfy feature flag and router support` | `feature_flags.go`, `router.go`, `router_test.go` | `go test ./backend/internal/notifications/...` passes |
| `3` | `fix(notifications): add Ntfy dispatch + fix pushover/ntfy token-clearing bug` | `notification_service.go`, `notification_service_json_test.go`, `notification_service_test.go` | `go test ./backend/internal/services/...` passes |
| `4` | `feat(notifications): add Ntfy type validation to handlers` | `notification_provider_handler.go` | `go test ./backend/internal/api/handlers/...` passes |
| `5` | `feat(notifications): add Ntfy frontend support` | `notifications.ts`, `Notifications.tsx`, `Notifications.test.tsx`, all 5 locale files | `npm test` passes; full Playwright suite passes |
| `6` | `docs: add Ntfy to features and changelog` | `docs/features.md`, `CHANGELOG.md` | No tests needed |

### Rollback

Reverting the PR removes all Ntfy cases from switch/case blocks. No data
migration reversal needed (model is unchanged). Any Ntfy providers created by
users during the rollout window would remain in the database as orphan rows
(type `"ntfy"` would be rejected by the handler validation, effectively
disabling them).

---

## 7. Review Suggestions for Build / Config Files

### `.gitignore`

No changes needed. The current `.gitignore` correctly covers all relevant
artifact patterns. No Ntfy-specific files are introduced.

### `codecov.yml`

No changes needed. The current `ignore` patterns correctly exclude test files,
docs, and config. The 87% project coverage target and 1% threshold remain
appropriate.

### `.dockerignore`

No changes needed. The current `.dockerignore` mirrors `.gitignore` patterns
appropriately. No new directories or file types are introduced.

### `Dockerfile`

No changes needed. The multi-stage build already compiles the full Go backend
and React frontend — adding a new provider type requires no build-system changes.
No new dependencies are introduced.

---

## 8. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Ntfy server unreachable | Low | Low | Standard HTTP timeout via `httpWrapper.Send()` (existing 10s timeout) |
| Token leaked in logs | Low | High | Token field is `json:"-"` in model; dispatch uses `headers` map (not logged). Verify no debug logging of headers. |
| SSRF via topic URL | Low | High | Ntfy matches the SSRF posture of Gotify and webhook (user-controlled URL), **not** Telegram (which pins to a hardcoded `api.telegram.org` base). `httpWrapper.Send()` applies the existing 10s timeout but no URL allowlist. Risk is **accepted** for parity with Gotify/webhook; a future hardening pass should apply `ValidateExternalURL` to all user-controlled URL providers. |
| Breaking existing providers | Very Low | High | All changes are additive `case` blocks — no existing behavior modified. Full regression suite via Playwright. |

---

## 9. Appendix: File Inventory

Complete list of files to create or modify:

### New Files

| File | Purpose |
|------|---------|
| `tests/settings/ntfy-notification-provider.spec.ts` | E2E test suite for Ntfy provider |

### Modified Files — Backend

| File | Lines Changed (est.) |
|------|---------------------|
| `backend/internal/notifications/feature_flags.go` | +1 |
| `backend/internal/notifications/router.go` | +2 |
| `backend/internal/notifications/router_test.go` | +15 |
| `backend/internal/services/notification_service.go` | +18 |
| `backend/internal/services/notification_service_test.go` | +20 |
| `backend/internal/services/notification_service_json_test.go` | +60 |
| `backend/internal/api/handlers/notification_provider_handler.go` | +3 |

### Modified Files — Frontend

| File | Lines Changed (est.) |
|------|---------------------|
| `frontend/src/api/notifications.ts` | +3 |
| `frontend/src/pages/Notifications.tsx` | +15 |
| `frontend/src/pages/__tests__/Notifications.test.tsx` | +3 |
| `frontend/src/locales/en/translation.json` | +5 |
| `frontend/src/locales/de/translation.json` | +5 |
| `frontend/src/locales/fr/translation.json` | +5 |
| `frontend/src/locales/zh/translation.json` | +5 |
| `frontend/src/locales/es/translation.json` | +5 |

### Modified Files — Tests

| File | Lines Changed (est.) |
|------|---------------------|
| `tests/settings/notifications-payload.spec.ts` | +30 |
| `tests/settings/notifications.spec.ts` | +2 |

### Modified Files — Documentation

| File | Lines Changed (est.) |
|------|---------------------|
| `docs/features.md` | +1 |
| `CHANGELOG.md` | +1 |

**Total estimated implementation:** ~195 lines (backend + frontend) + ~430 lines (tests)

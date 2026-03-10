# Telegram Notification Provider — Implementation Plan

**Date:** 2026-07-10
**Author:** Planning Agent
**Confidence Score:** 92% (High — existing patterns well-established, Telegram Bot API straightforward)

---

## 1. Introduction

### Objective

Add Telegram as a first-class notification provider in Charon, following the established architecture used by Discord, Gotify, Email, and generic Webhook providers.

### Goals

- Users can configure a Telegram bot token and chat ID to receive notifications via Telegram
- All existing notification event types (proxy hosts, certs, uptime, security events) work with Telegram
- JSON template engine (minimal/detailed/custom) works with Telegram
- Feature flag allows enabling/disabling Telegram dispatch independently
- Token is treated as a secret (write-only, never exposed in API responses)
- Full test coverage: Go unit tests, Vitest frontend tests, Playwright E2E tests

### Telegram Bot API Overview

Telegram bots send messages via:

```
POST https://api.telegram.org/bot<BOT_TOKEN>/sendMessage
Content-Type: application/json

{
  "chat_id": "<CHAT_ID>",
  "text": "Hello world",
  "parse_mode": "HTML"    // optional: "HTML" or "MarkdownV2"
}
```

**Key design decisions:**
- **Token storage:** The bot token is stored in `NotificationProvider.Token` (`json:"-"`, encrypted at rest) — never in the URL field. This mirrors the Gotify pattern where secrets are separated from endpoints.
- **URL field:** Stores only the `chat_id` (e.g., `987654321`). At dispatch time, the full API URL is constructed dynamically: `https://api.telegram.org/bot` + decryptedToken + `/sendMessage`. The `chat_id` is passed in the POST body alongside the message text. This prevents token leakage via API responses since URL is `json:"url"`.
- **SSRF mitigation:** Before dispatching, validate that the constructed URL hostname is exactly `api.telegram.org`. This prevents SSRF if stored data is tampered with.
- **Dispatch path:** Uses `sendJSONPayload` → `httpWrapper.Send()` (same as Gotify), since both are token-based JSON POST providers
- **No schema migration needed:** The existing `NotificationProvider` model accommodates Telegram without changes

> **Supervisor Review Note:** The original design embedded the bot token in the URL field (`https://api.telegram.org/bot<TOKEN>/sendMessage?chat_id=<CHAT_ID>`). This was rejected because the URL field is `json:"url"` — exposed in every API response. The token MUST only reside in the `Token` field (`json:"-"`).

---

## 2. Research Findings

### Existing Architecture

The notification system follows a provider-based architecture:

| Layer | File | Role |
|-------|------|------|
| Feature flags | `backend/internal/notifications/feature_flags.go` | Flag constants (`FlagXxxServiceEnabled`) |
| Feature flag handler | `backend/internal/api/handlers/feature_flags_handler.go` | DB-backed flags with defaults |
| Router | `backend/internal/notifications/router.go` | `ShouldUseNotify()` per-type dispatch |
| Service | `backend/internal/services/notification_service.go` | Core dispatch: `isSupportedNotificationProviderType()`, `isDispatchEnabled()`, `supportsJSONTemplates()`, `sendJSONPayload()`, `TestProvider()` |
| Handlers | `backend/internal/api/handlers/notification_provider_handler.go` | CRUD + type validation + token preservation |
| Model | `backend/internal/models/notification_provider.go` | GORM model with Token (json:"-"), HasToken |
| Frontend API | `frontend/src/api/notifications.ts` | `SUPPORTED_NOTIFICATION_PROVIDER_TYPES`, sanitization |
| Frontend UI | `frontend/src/pages/Notifications.tsx` | Provider form with conditional fields per type |
| i18n | `frontend/src/locales/en/translation.json` | Label strings |
| E2E fixtures | `tests/fixtures/notifications.ts` | `telegramProvider` **already defined** |

### Existing Provider Addition Points (Switch Statements / Type Checks)

Every location that checks provider types is listed below — all require a `"telegram"` case:

| # | File | Function/Line | Current Logic |
|---|------|---------------|---------------|
| 1 | `feature_flags.go` | Constants | Missing `FlagTelegramServiceEnabled` |
| 2 | `feature_flags_handler.go` | `defaultFlags` + `defaultFlagValues` | Missing telegram entry |
| 3 | `router.go` | `ShouldUseNotify()` switch | Missing `case "telegram"` |
| 4 | `notification_service.go` | `isSupportedNotificationProviderType()` | `case "discord", "email", "gotify", "webhook"` |
| 5 | `notification_service.go` | `isDispatchEnabled()` | switch with per-type flag checks |
| 6 | `notification_service.go` | `supportsJSONTemplates()` | `case "webhook", "discord", "gotify", "slack", "generic"` |
| 7 | `notification_service.go` | `sendJSONPayload()` — service-specific validation | Missing `case "telegram"` for payload validation |
| 8 | `notification_service.go` | `sendJSONPayload()` — dispatch branch | Gotify/webhook use `httpWrapper.Send()`; others use `ValidateExternalURL` + `SafeHTTPClient` |
| 9 | `notification_provider_handler.go` | `Create()` type guard | `providerType != "discord" && providerType != "gotify" && providerType != "webhook" && providerType != "email"` |
| 10 | `notification_provider_handler.go` | `Update()` type guard | Same pattern as Create |
| 11 | `notification_provider_handler.go` | `Update()` token preservation | `if providerType == "gotify" && strings.TrimSpace(req.Token) == ""` |
| 12 | `notifications.ts` | `SUPPORTED_NOTIFICATION_PROVIDER_TYPES` | `['discord', 'gotify', 'webhook', 'email']` |
| 13 | `notifications.ts` | `sanitizeProviderForWriteAction()` | Token handling only for `type !== 'gotify'` |
| 14 | `Notifications.tsx` | Type `<select>` options | discord/gotify/webhook/email |
| 15 | `Notifications.tsx` | `normalizeProviderPayloadForSubmit()` | Token mapping only for `type === 'gotify'` |
| 16 | `Notifications.tsx` | Conditional form fields | `isGotify` shows token input |

### Test Files Requiring Updates

| File | Current Behavior | Required Change |
|------|-----------------|-----------------|
| `notification_service_test.go` (~L1819) | `TestTestProvider_NotifyOnlyRejectsUnsupportedProvider` tests `"telegram"` as **unsupported** | Change: telegram is now supported |
| `notification_service_json_test.go` | Discord/Slack/Gotify/Webhook JSON tests | Add telegram payload validation tests |
| `notification_provider_handler_test.go` | CRUD tests with supported types | Add telegram to supported type lists |
| `enhanced_security_notification_service_test.go` (~L139) | `Type: "telegram"` marked `// Should be filtered` | Change: telegram is now valid |
| `frontend/src/api/notifications.test.ts` | Rejects `"telegram"` as unsupported | Accept telegram, add CRUD tests |
| `frontend/src/api/__tests__/notifications.test.ts` | Same rejection | Same fix |
| `tests/settings/notifications.spec.ts` | CRUD E2E for discord/gotify/webhook/email | Add telegram scenarios |

### E2E Fixture Already Defined

```typescript
// tests/fixtures/notifications.ts
// NOTE: Fixture must be updated — URL should contain only the chat_id, token goes in the token field
export const telegramProvider: NotificationProviderConfig = {
  name: generateProviderName('telegram'),
  type: 'telegram',
  url: '987654321',  // chat_id only — bot token is stored in the Token field
  token: 'bot123456789:ABCdefGHIjklMNOpqrSTUvwxYZ',  // stored encrypted, never in API responses
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: true,
  notify_uptime: true,
};
```

---

## 3. Technical Specifications

### 3.1 Backend — Feature Flags

**File:** `backend/internal/notifications/feature_flags.go`

Add constant:

```go
FlagTelegramServiceEnabled = "feature.notifications.service.telegram.enabled"
```

**File:** `backend/internal/api/handlers/feature_flags_handler.go`

Add to `defaultFlags` slice:

```go
notifications.FlagTelegramServiceEnabled,
```

Add to `defaultFlagValues` map:

```go
notifications.FlagTelegramServiceEnabled: true,
```

> **Note:** Telegram is **enabled by default** once the provider is toggled on in the UI, matching Gotify/Webhook behavior. The feature flag exists as an admin-level kill switch, not a setup gate.

### 3.2 Backend — Router

**File:** `backend/internal/notifications/router.go`

Add to `ShouldUseNotify()` switch:

```go
case "telegram":
    return flags[FlagTelegramServiceEnabled]
```

### 3.3 Backend — Notification Service

**File:** `backend/internal/services/notification_service.go`

#### `isSupportedNotificationProviderType()`

```go
case "discord", "email", "gotify", "webhook", "telegram":
    return true
```

#### `isDispatchEnabled()`

```go
case "telegram":
    return s.getFeatureFlagValue(notifications.FlagTelegramServiceEnabled, true)
```

Both `defaultFlagValues` (initial DB seed) and the `isDispatchEnabled()` fallback are `true` — Telegram is enabled by default once the provider is created in the UI. This matches Gotify/Webhook behavior (enabled-by-default, admin kill-switch via feature flag).

#### `supportsJSONTemplates()`

```go
case "webhook", "discord", "gotify", "slack", "generic", "telegram":
    return true
```

#### `sendJSONPayload()` — Service-Specific Validation

Add after the `case "gotify":` block:

```go
case "telegram":
    // Telegram requires 'text' field for the message body
    if _, hasText := jsonPayload["text"]; !hasText {
        // Auto-map 'message' to 'text' if present (template compatibility)
        if messageValue, hasMessage := jsonPayload["message"]; hasMessage {
            jsonPayload["text"] = messageValue
            normalizedBody, marshalErr := json.Marshal(jsonPayload)
            if marshalErr != nil {
                return fmt.Errorf("failed to normalize telegram payload: %w", marshalErr)
            }
            body.Reset()
            if _, writeErr := body.Write(normalizedBody); writeErr != nil {
                return fmt.Errorf("failed to write normalized telegram payload: %w", writeErr)
            }
        } else {
            return fmt.Errorf("telegram payload requires 'text' field")
        }
    }
```

This auto-mapping mirrors the Discord pattern (`message` → `content`) so that the built-in `minimal` and `detailed` templates (which use `"message"` as a field) work out of the box with Telegram.

#### `sendJSONPayload()` — Dispatch Branch

Add `"telegram"` to the `httpWrapper.Send()` dispatch branch alongside gotify/webhook:

```go
if providerType == "gotify" || providerType == "webhook" || providerType == "telegram" {
```

For telegram, the dispatch URL must be **constructed at send time** from the stored token and chat_id:

```go
case "telegram":
    // Construct the API URL dynamically — token is NEVER stored in the URL field
    decryptedToken := provider.Token // already decrypted by service layer
    dispatchURL = "https://api.telegram.org/bot" + decryptedToken + "/sendMessage"

    // SSRF mitigation: validate hostname before dispatch
    parsedURL, err := url.Parse(dispatchURL)
    if err != nil || parsedURL.Hostname() != "api.telegram.org" {
        return fmt.Errorf("telegram dispatch URL validation failed: invalid hostname")
    }

    // Inject chat_id into the JSON payload body (URL field stores the chat_id)
    jsonPayload["chat_id"] = provider.URL
    // Re-marshal the payload with chat_id included
    updatedBody, marshalErr := json.Marshal(jsonPayload)
    if marshalErr != nil {
        return fmt.Errorf("failed to marshal telegram payload with chat_id: %w", marshalErr)
    }
    body.Reset()
    body.Write(updatedBody)
```

The `X-Gotify-Key` header is only set when `providerType == "gotify"` — no header changes needed for telegram.

#### `TestProvider()` — Telegram-Specific Error Message

When testing a Telegram provider and the API returns HTTP 401 or 403, return a specific error message:

```go
case "telegram":
    if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
        return fmt.Errorf("provider rejected authentication. Verify your Telegram Bot Token")
    }
```

This gives users actionable guidance instead of a generic HTTP status error.

### 3.4 Backend — Handler Layer

**File:** `backend/internal/api/handlers/notification_provider_handler.go`

#### `Create()` Type Guard

```go
if providerType != "discord" && providerType != "gotify" && providerType != "webhook" && providerType != "email" && providerType != "telegram" {
```

#### `Update()` Type Guard

Same change as Create:

```go
if providerType != "discord" && providerType != "gotify" && providerType != "webhook" && providerType != "email" && providerType != "telegram" {
```

#### `Update()` Token Preservation

Telegram bot tokens should be preserved on update when the user omits them (same UX as Gotify):

```go
if (providerType == "gotify" || providerType == "telegram") && strings.TrimSpace(req.Token) == "" {
    req.Token = existing.Token
}
```

### 3.5 Backend — Model (No Changes)

The `NotificationProvider` model already has:

- `Token string` with `json:"-"` (write-only, never exposed)
- `HasToken bool` with `gorm:"-"` (computed field for frontend)
- `URL string` for the endpoint
- `ServiceConfig string` for extra JSON config (available for `parse_mode` if needed)

No schema migration is required.

### 3.6 Frontend — API Client

**File:** `frontend/src/api/notifications.ts`

#### Supported Types Array

```typescript
export const SUPPORTED_NOTIFICATION_PROVIDER_TYPES = ['discord', 'gotify', 'webhook', 'email', 'telegram'] as const;
```

#### `sanitizeProviderForWriteAction()`

**Minimal diff only.** Change only the type guard condition from:

```typescript
if (type !== 'gotify') {
```

to:

```typescript
if (type !== 'gotify' && type !== 'telegram') {
```

The surrounding normalization logic (token stripping, payload return) MUST remain untouched. No other lines in this function change.

#### `sanitizeProviderForReadLikeAction()`

No changes — already calls `sanitizeProviderForWriteAction()` then strips token.

### 3.7 Frontend — Notifications Page

**File:** `frontend/src/pages/Notifications.tsx`

#### Type Select Options

Add after the email option:

```tsx
<option value="telegram">Telegram</option>
```

#### Computed Flags

```typescript
const isTelegram = type === 'telegram';
```

#### `normalizeProviderPayloadForSubmit()`

Add telegram branch alongside gotify:

```typescript
if (type === 'gotify' || type === 'telegram') {
    const normalizedToken = typeof payload.gotify_token === 'string' ? payload.gotify_token.trim() : '';
    if (normalizedToken.length > 0) {
        payload.token = normalizedToken;
    } else {
        delete payload.token;
    }
} else {
    delete payload.token;
}
```

Note: Reuses the `gotify_token` form field for both Gotify and Telegram since both need a token input. This minimizes UI changes. The field label changes based on provider type.

#### Token Input Field

Expand the conditional from `{isGotify && (` to `{(isGotify || isTelegram) && (`:

```tsx
{(isGotify || isTelegram) && (
    <div>
        <label htmlFor="provider-gotify-token" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
            {isTelegram ? t('notificationProviders.telegramBotToken') : t('notificationProviders.gotifyToken')}
        </label>
        <input
            id="provider-gotify-token"
            type="password"
            autoComplete="new-password"
            {...register('gotify_token')}
            data-testid="provider-gotify-token"
            placeholder={initialData?.has_token
                ? t('notificationProviders.gotifyTokenKeepPlaceholder')
                : isTelegram
                    ? t('notificationProviders.telegramBotTokenPlaceholder')
                    : t('notificationProviders.gotifyTokenPlaceholder')}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm"
            aria-describedby={initialData?.has_token ? 'gotify-token-stored-hint' : undefined}
        />
        {initialData?.has_token && (
            <p id="gotify-token-stored-hint" data-testid="gotify-token-stored-indicator" className="text-xs text-green-600 dark:text-green-400 mt-1">
                {t('notificationProviders.gotifyTokenStored')}
            </p>
        )}
        <p className="text-xs text-gray-500 mt-1">{t('notificationProviders.gotifyTokenWriteOnlyHint')}</p>
    </div>
)}
```

#### URL Field Placeholder

For Telegram, the URL field stores the chat_id (not a full URL). Update the placeholder and label accordingly:

```typescript
placeholder={
    isEmail ? 'user@example.com, admin@example.com'
    : type === 'discord' ? 'https://discord.com/api/webhooks/...'
    : type === 'gotify' ? 'https://gotify.example.com/message'
    : isTelegram ? '987654321'
    : 'https://example.com/webhook'
}
```

Update the label for the URL field when type is telegram:

```typescript
label={isTelegram ? t('notificationProviders.telegramChatId') : t('notificationProviders.url')}
```

#### Clear Token on Type Change

Update the existing `useEffect` that clears `gotify_token`:

```typescript
useEffect(() => {
    if (type !== 'gotify' && type !== 'telegram') {
        setValue('gotify_token', '', { shouldDirty: false, shouldTouch: false });
    }
}, [type, setValue]);
```

### 3.8 Frontend — i18n Strings

**File:** `frontend/src/locales/en/translation.json`

Add to the `notificationProviders` section:

```json
"telegram": "Telegram",
"telegramBotToken": "Bot Token",
"telegramBotTokenPlaceholder": "Enter your Telegram Bot Token",
"telegramChatId": "Chat ID",
"telegramChatIdPlaceholder": "987654321",
"telegramChatIdHelp": "Your Telegram chat, group, or channel ID. The bot token is stored securely and separately."
```

### 3.9 API Contract (No Changes)

The existing REST endpoints remain unchanged:

| Method | Endpoint | Notes |
|--------|----------|-------|
| `GET` | `/api/notification-providers` | Returns all providers (token stripped) |
| `POST` | `/api/notification-providers` | Create — now accepts `type: "telegram"` |
| `PUT` | `/api/notification-providers/:id` | Update — token preserved if omitted |
| `DELETE` | `/api/notification-providers/:id` | Delete — no type-specific logic |
| `POST` | `/api/notification-providers/test` | Test — routes through `sendJSONPayload` |

Request/response schemas are unchanged. The `type` field now accepts `"telegram"` in addition to existing values.

---

## 4. Implementation Plan

### Phase 1: Playwright E2E Tests (Test-First)

**Rationale:** Per project conventions — write feature behaviour tests first.

**New file:** `tests/settings/telegram-notification-provider.spec.ts`

Modeled after `tests/settings/email-notification-provider.spec.ts`.

Test scenarios:
1. Create a Telegram provider (name, chat_id in URL field, bot token in token field, enable events)
2. Verify provider appears in the list
3. Edit the Telegram provider (change name, verify token preservation)
4. Test the Telegram provider (mock API returns 200)
5. Delete the Telegram provider
6. **Negative security test:** Verify `GET /api/notification-providers` does NOT expose the bot token in any response field
7. **Negative security test:** Verify bot token is NOT present in the URL field of the API response

**Update file:** `tests/settings/notifications-payload.spec.ts`

Add telegram to the payload matrix test scenarios.

**E2E fixtures:** Update `telegramProvider` in `tests/fixtures/notifications.ts` — URL must contain only `chat_id`, token goes in the `token` field (see Research Findings section for updated fixture).

### Phase 2: Backend Implementation

**2A — Feature Flags (3 files)**

| File | Change |
|------|--------|
| `backend/internal/notifications/feature_flags.go` | Add `FlagTelegramServiceEnabled` constant |
| `backend/internal/api/handlers/feature_flags_handler.go` | Add to `defaultFlags` + `defaultFlagValues` |
| `backend/internal/notifications/router.go` | Add `case "telegram"` to `ShouldUseNotify()` |

**2B — Service Layer (1 file, 4 function changes)**

| File | Function | Change |
|------|----------|--------|
| `notification_service.go` | `isSupportedNotificationProviderType()` | Add `"telegram"` to case |
| `notification_service.go` | `isDispatchEnabled()` | Add `case "telegram"` with flag check |
| `notification_service.go` | `supportsJSONTemplates()` | Add `"telegram"` to case |
| `notification_service.go` | `sendJSONPayload()` | Add telegram validation + dispatch branch |

**2C — Handler Layer (1 file, 3 locations)**

| File | Location | Change |
|------|----------|--------|
| `notification_provider_handler.go` | `Create()` type guard | Add `&& providerType != "telegram"` |
| `notification_provider_handler.go` | `Update()` type guard | Same |
| `notification_provider_handler.go` | `Update()` token preservation | Add `|| providerType == "telegram"` |

### Phase 3: Frontend Implementation

**3A — API Client (1 file)**

| File | Change |
|------|--------|
| `frontend/src/api/notifications.ts` | Add `'telegram'` to `SUPPORTED_NOTIFICATION_PROVIDER_TYPES`, update token sanitization logic |

**3B — Notifications Page (1 file)**

| File | Change |
|------|--------|
| `frontend/src/pages/Notifications.tsx` | Add telegram to type select, token field conditional, URL placeholder, `normalizeProviderPayloadForSubmit()`, type-change useEffect |

**3C — Localization (1 file)**

| File | Change |
|------|--------|
| `frontend/src/locales/en/translation.json` | Add telegram-specific label strings |

### Phase 4: Backend Tests

| Test File | Changes |
|-----------|---------|
| `notification_service_test.go` | Update "rejects unsupported provider" test (remove telegram from unsupported list). Add telegram dispatch/integration tests. |
| `notification_service_json_test.go` | Add `TestSendJSONPayload_Telegram_*` tests: valid payload, missing text with message auto-map, missing both text and message, dispatch via httpWrapper, **SSRF hostname validation**, **401/403 error message** |
| `notification_provider_handler_test.go` | Add telegram to Create/Update happy path tests, token preservation test. **Add negative test: verify GET response does not contain bot token in URL field or response body** |
| `enhanced_security_notification_service_test.go` | Change telegram from "filtered" to "valid provider" in security dispatch tests |
| Router test (if exists) | Add telegram to `ShouldUseNotify()` tests |

### Phase 5: Frontend Tests

| Test File | Changes |
|-----------|---------|
| `frontend/src/api/notifications.test.ts` | Remove telegram rejection test, add telegram CRUD sanitization tests |
| `frontend/src/api/__tests__/notifications.test.ts` | Same changes (duplicate test location) |
| `frontend/src/pages/Notifications.test.tsx` | Add telegram form rendering tests (token field visibility, placeholder text) |

### Phase 6: Integration, Documentation & Deployment

- Verify E2E tests pass with Docker container
- Update `docs/features.md` with Telegram provider mention
- No `ARCHITECTURE.md` changes needed (same provider pattern)
- No database migration needed

---

## 5. Acceptance Criteria

### EARS Requirements

| ID | Requirement |
|----|-------------|
| T-01 | WHEN a user creates a notification provider with type "telegram", THE SYSTEM SHALL accept the provider and store it in the database |
| T-02 | WHEN a user provides a bot token for a Telegram provider, THE SYSTEM SHALL store it securely and never expose it in API responses |
| T-03 | WHEN a Telegram provider is enabled and a notification event fires, THE SYSTEM SHALL construct the Telegram API URL dynamically from the stored token (`https://api.telegram.org/bot` + token + `/sendMessage`), inject `chat_id` from the URL field into the POST body, and send the rendered template payload |
| T-04 | WHEN the rendered JSON payload contains a "message" field but not a "text" field, THE SYSTEM SHALL auto-map "message" to "text" for Telegram compatibility |
| T-05 | WHEN the Telegram feature flag is disabled, THE SYSTEM SHALL skip dispatch for all Telegram providers |
| T-06 | WHEN a user updates a Telegram provider without providing a token, THE SYSTEM SHALL preserve the existing stored token |
| T-07 | WHEN a user tests a Telegram provider, THE SYSTEM SHALL send a test notification through the standard sendJSONPayload path |
| T-08 | WHEN the frontend renders the provider form with type "telegram", THE SYSTEM SHALL display a bot token input field and a chat_id input field (with appropriate placeholder) |
| T-09 | WHEN dispatching a Telegram notification, THE SYSTEM SHALL validate that the constructed URL hostname is exactly `api.telegram.org` before sending (SSRF mitigation) |
| T-10 | WHEN a Telegram test request receives HTTP 401 or 403, THE SYSTEM SHALL return the error message "Provider rejected authentication. Verify your Telegram Bot Token" |
| T-11 | WHEN the API returns notification providers via GET, THE SYSTEM SHALL NOT include the bot token in the URL field or any other exposed response field |

### Definition of Done

- [ ] All 16 code touchpoints updated (see section 2 table)
- [ ] E2E Playwright tests pass for Telegram CRUD + test send
- [ ] Backend unit tests cover: type registration, dispatch routing, payload validation (text field), token preservation, feature flag gating
- [ ] Frontend unit tests cover: type array acceptance, sanitization, form rendering
- [ ] `go test ./...` passes
- [ ] `npm test` passes
- [ ] `npx playwright test --project=firefox` passes
- [ ] `make lint-fast` passes (staticcheck)
- [ ] Coverage threshold maintained (85%+)
- [ ] GORM security scan passes (no model changes, but verify)
- [ ] Token never appears in API responses, logs, or frontend state
- [ ] Negative security tests pass (bot token not in GET response body or URL field)
- [ ] SSRF hostname validation test passes (only `api.telegram.org` allowed)
- [ ] Telegram 401/403 returns specific auth error message

---

## 6. Commit Slicing Strategy

### Decision: 2 PRs

**Trigger reasons:** Changes span backend + frontend + E2E tests with independent functionality per layer. Splitting improves review quality and rollback safety.

### PR-1: Backend — Telegram Provider Support

**Scope:** Feature flags, service layer, handler layer, all Go unit tests

**Files changed:**
- `backend/internal/notifications/feature_flags.go`
- `backend/internal/api/handlers/feature_flags_handler.go`
- `backend/internal/notifications/router.go`
- `backend/internal/services/notification_service.go`
- `backend/internal/api/handlers/notification_provider_handler.go`
- `backend/internal/services/notification_service_test.go`
- `backend/internal/services/notification_service_json_test.go`
- `backend/internal/api/handlers/notification_provider_handler_test.go`
- `backend/internal/services/enhanced_security_notification_service_test.go`

**Dependencies:** None (self-contained backend change)

**Validation gates:**
- `go test ./...` passes
- `make lint-fast` passes
- Coverage ≥ 85%
- GORM security scan passes

**Rollback:** Revert PR — no DB migration to undo.

### PR-2: Frontend + E2E — Telegram Provider UI

**Scope:** Frontend API client, Notifications page, i18n strings, frontend unit tests, Playwright E2E tests

**Files changed:**
- `frontend/src/api/notifications.ts`
- `frontend/src/pages/Notifications.tsx`
- `frontend/src/locales/en/translation.json`
- `frontend/src/api/notifications.test.ts`
- `frontend/src/api/__tests__/notifications.test.ts`
- `frontend/src/pages/Notifications.test.tsx`
- `tests/settings/telegram-notification-provider.spec.ts` (new)
- `tests/settings/notifications-payload.spec.ts`

**Dependencies:** PR-1 must be merged first (backend must accept `type: "telegram"`)

**Validation gates:**
- `npm test` passes
- `npm run type-check` passes
- `npx playwright test --project=firefox` passes
- Coverage ≥ 85%

**Rollback:** Revert PR — frontend-only, no cascading effects.

---

## 7. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Telegram API rate limiting | Low | Medium | Use existing retry/timeout patterns from httpWrapper |
| Bot token exposure in responses/logs | Low | Critical | Token stored ONLY in `Token` field (`json:"-"`), never in URL field. URL field contains only `chat_id`. Negative security tests verify this invariant. |
| Template auto-mapping edge cases | Low | Low | Test with all three template types (minimal, detailed, custom) |
| URL validation rejects chat_id format | Low | Low | URL field now stores a chat_id string (not a full URL). Validation may need adjustment to accept non-URL values for telegram type. |
| SSRF via tampered stored data | Low | High | Dispatch-time validation ensures hostname is exactly `api.telegram.org`. Dedicated test covers this. |
| E2E test flakiness with mocked API | Low | Low | Existing route-mocking patterns are stable |

---

## 8. Complexity Estimates

| Component | Estimate | Notes |
|-----------|----------|-------|
| Backend feature flags | S | 3 files, ~5 lines each |
| Backend service layer | M | 4 function changes + telegram validation block |
| Backend handler layer | S | 3 string-level changes |
| Frontend API client | S | 2 lines + sanitization tweak |
| Frontend UI | M | Template conditional, placeholder, useEffect updates |
| Frontend i18n | S | 4 strings |
| Backend tests | L | Multiple test files, new test functions, update existing assertions |
| Frontend tests | M | Update rejection tests, add rendering tests |
| E2E tests | M | New spec file modeled on existing email spec |
| **Total** | **M-L** | ~2-3 days of focused implementation |

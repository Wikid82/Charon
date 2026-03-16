# Pushover Notification Provider — Implementation Plan

**Date:** 2026-07-21
**Author:** Planning Agent
**Confidence Score:** 90% (High — established provider patterns, Pushover API well-documented)
**Prior Art:** `docs/plans/telegram_implementation_spec.md` (Telegram followed identical pattern)

---

## 1. Introduction

### Objective

Add Pushover as a first-class notification provider in Charon, following the same architectural pattern used by Telegram, Slack, Gotify, Discord, Email, and generic Webhook providers.

### Goals

- Users can configure a Pushover API token and user key to receive push notifications
- All existing notification event types (proxy hosts, certs, uptime, security events) work with Pushover
- JSON template engine (minimal/detailed/custom) works with Pushover
- Feature flag allows enabling/disabling Pushover dispatch independently
- API token is treated as a secret (write-only, never exposed in API responses)
- Full test coverage: Go unit tests, Vitest frontend tests, Playwright E2E tests

### Pushover API Overview

Pushover messages are sent via:

```
POST https://api.pushover.net/1/messages.json
Content-Type: application/x-www-form-urlencoded

token=<APP_API_TOKEN>&user=<USER_KEY>&message=Hello+world
```

**Or** as JSON with `Content-Type: application/json`:

```json
POST https://api.pushover.net/1/messages.json

{
  "token": "<APP_API_TOKEN>",
  "user": "<USER_KEY>",
  "message": "Hello world",
  "title": "Charon Alert",
  "priority": 0,
  "sound": "pushover"
}
```

Required parameters: `token`, `user`, `message`
Optional parameters: `title`, `priority` (-2 to 2), `sound`, `device`, `url`, `url_title`, `html` (1 for HTML), `timestamp`, `ttl`

**Key design decisions:**

| Decision | Rationale |
|----------|-----------|
| **Token storage:** `NotificationProvider.Token` (`json:"-"`) stores the Pushover **Application API Token** | Mirrors Telegram/Slack/Gotify pattern — secrets are never in the URL field |
| **URL field:** Stores the Pushover **User Key** (e.g., `uQiRzpo4DXghDmr9QzzfQu27cmVRsG`) | Follows Telegram pattern where URL stores the recipient identifier (chat_id → user key) |
| **Dispatch uses JSON POST:** Despite Pushover supporting form-encoded, we send JSON with `Content-Type: application/json` | Aligns with existing `sendJSONPayload()` pipeline — reuses template engine, httpWrapper, validation |
| **Fixed API endpoint:** `https://api.pushover.net/1/messages.json` constructed at dispatch time | Mirrors Telegram pattern (dynamic URL from token); prevents SSRF via stored data |
| **SSRF mitigation:** Validate constructed URL hostname is `api.pushover.net` before dispatch | Same pattern as Telegram's `api.telegram.org` pin |
| **No schema migration:** Existing `NotificationProvider` model accommodates Pushover | Token, URL, Config fields are sufficient |

> **Important:** The user stated "Pushover is ALREADY part of the notification engine backend code" — however, research confirms Pushover is currently treated as **UNSUPPORTED** everywhere. It appears only in tests as an example of an unsupported/deprecated type. All dispatch code, type guards, feature flags, and UI must be built from scratch following the Telegram/Slack pattern.

---

## 2. Research Findings

### 2.1 Existing Architecture

| Layer | File | Role |
|-------|------|------|
| Feature flags | `backend/internal/notifications/feature_flags.go` | Flag constants (`FlagXxxServiceEnabled`) |
| Router | `backend/internal/notifications/router.go` | `ShouldUseNotify()` per-type dispatch |
| Service | `backend/internal/services/notification_service.go` | Core dispatch: `isSupportedNotificationProviderType()`, `isDispatchEnabled()`, `supportsJSONTemplates()`, `sendJSONPayload()`, `TestProvider()` |
| Handlers | `backend/internal/api/handlers/notification_provider_handler.go` | CRUD + type validation + token preservation |
| Enhanced Security | `backend/internal/services/enhanced_security_notification_service.go` | Security event notifications with provider aggregation |
| Model | `backend/internal/models/notification_provider.go` | GORM model with Token (`json:"-"`), HasToken |
| Frontend API | `frontend/src/api/notifications.ts` | `SUPPORTED_NOTIFICATION_PROVIDER_TYPES`, sanitization |
| Frontend UI | `frontend/src/pages/Notifications.tsx` | Provider form with conditional fields per type |
| i18n | `frontend/src/locales/en/translation.json` | Label strings under `notificationProviders` |
| E2E fixtures | `tests/fixtures/notifications.ts` | Provider configs and type union |

### 2.2 All Type-Check Locations Requiring `"pushover"` Addition

| # | File | Function/Location | Current Types |
|---|------|-------------------|---------------|
| 1 | `feature_flags.go` | Constants | discord, email, gotify, webhook, telegram, slack |
| 2 | `router.go` | `ShouldUseNotify()` switch | discord, email, gotify, webhook, telegram (missing slack! — **fix in same PR**) |
| 3 | `notification_service.go` L137 | `isSupportedNotificationProviderType()` | discord, email, gotify, webhook, telegram, slack |
| 4 | `notification_service.go` L146 | `isDispatchEnabled()` | discord, email, gotify, webhook, telegram, slack |
| 5 | `notification_service.go` L128 | `supportsJSONTemplates()` | webhook, discord, gotify, slack, generic, telegram |
| 6 | `notification_service.go` L470 | `sendJSONPayload()` — payload validation switch | discord, slack, gotify, telegram |
| 7 | `notification_service.go` L512 | `sendJSONPayload()` — dispatch branch (`httpWrapper.Send`) | gotify, webhook, telegram, slack |
| 8 | `notification_provider_handler.go` L186 | `Create()` type guard | discord, gotify, webhook, email, telegram, slack |
| 9 | `notification_provider_handler.go` L246 | `Update()` type guard | discord, gotify, webhook, email, telegram, slack |
| 10 | `notification_provider_handler.go` L250 | `Update()` token preservation | gotify, telegram, slack |
| 11 | `notification_provider_handler.go` L312-316 | `Test()` token write-only guards | gotify, slack (**Note:** telegram is missing here — add in same PR) |
| 12 | `enhanced_security_notification_service.go` L87 | `getProviderAggregatedConfig()` supportedTypes | webhook, discord, slack, gotify, telegram |
| 13 | `notifications.ts` L3 | `SUPPORTED_NOTIFICATION_PROVIDER_TYPES` | discord, gotify, webhook, email, telegram, slack |
| 14 | `notifications.ts` L62 | `sanitizeProviderForWriteAction()` token handling | gotify, telegram, slack |
| 15 | `Notifications.tsx` L204 | Type `<select>` options | discord, gotify, webhook, email, telegram, slack |
| 16 | `Notifications.tsx` L47 | `normalizeProviderPayloadForSubmit()` | gotify, telegram, slack |
| 17 | `Notifications.tsx` L153 | `useEffect` token clear | gotify, telegram, slack |
| 18 | `Notifications.tsx` L254 | Token field conditional | isGotify, isTelegram, isSlack |
| 19 | `Notifications.tsx` L24 | `supportsJSONTemplates()` | discord, gotify, webhook, telegram, slack |

### 2.3 Test Files Currently Using Pushover as "Unsupported"

These tests explicitly use `"pushover"` as an **unsupported** type. They MUST be updated when Pushover becomes supported.

| File | Line | Context | Required Change |
|------|------|---------|-----------------|
| `notification_service_test.go` | L1832 | `TestTestProvider_NotifyOnlyRejectsUnsupportedProvider` — `{"pushover", ...}` test case | Remove pushover from unsupported list; replace with different unsupported type (e.g., `"sms"`) |
| `notification_service_test.go` | L2159-2161 | `"Pushover Provider (deprecated)"` in migration tests | Update test to use a different deprecated type |
| `notification_service_test.go` | L2190 | `nonDiscordTypes` list includes `"pushover"` | **Keep `pushover` in the `nonDiscordTypes` list.** This test verifies migration logic for non-Discord providers; Pushover is supported but still participates in migration. Update test assertions if needed to reflect Pushover is now a supported type. |
| `notification_coverage_test.go` | L1006-1013 | `TestNotificationProviderHandler_Update_UnsupportedType` uses `Type: "pushover"` | Replace with `"sms"` or `"pagerduty"` |
| `notification_provider_discord_only_test.go` | L370 | Create with `"type": "pushover"` expects rejection | Replace with different unsupported type |
| `security_notifications_final_blockers_test.go` | L227 | `unsupportedTypes := []string{"pushover", "generic"}` | Replace `"pushover"` with `"sms"` |

### 2.4 Frontend Test Files Needing Updates

| File | Line | Context | Required Change |
|------|------|---------|-----------------|
| `frontend/src/api/notifications.test.ts` | L121 | Rejects `pushover` as unsupported | Remove/replace test case |
| `frontend/src/api/__tests__/notifications.test.ts` | L56 | Same rejection test | Replace with different unsupported type |
| `frontend/src/pages/__tests__/Notifications.test.tsx` | L431 | `'Legacy Pushover'` type `'pushover'` used in test | Update test to use different unsupported type |
| `tests/settings/notifications.spec.ts` | L144,191,209 | Pushover mocked as existing provider in mixed-type list | Update to reflect pushover as a supported type (no deprecated badge) |

### 2.5 Pushover vs Telegram: Pattern Comparison

| Aspect | Telegram | Pushover (Planned) |
|--------|---------|--------------------|
| Token field | Bot Token (`bot123:ABC...`) | Application API Token |
| URL field | Chat ID (`987654321`) | User Key (`uQiRzpo4DXgh...`) |
| Dispatch URL | `https://api.telegram.org/bot<TOKEN>/sendMessage` | `https://api.pushover.net/1/messages.json` |
| Token in URL? | Yes (path segment) | No (JSON body field `token`) |
| Recipient in body? | Yes (`chat_id` injected) | Yes (`user` injected) |
| SSRF pin | `api.telegram.org` | `api.pushover.net` |
| Content-Type | `application/json` | `application/json` |
| Required payload fields | `chat_id`, `text` | `token`, `user`, `message` |
| Template auto-mapping | `message` → `text` | `message` → `message` (already correct) |

**Key difference:** Telegram embeds the token in the dispatch URL path; Pushover sends the token in the request body. This means:
1. Pushover dispatch URL is **static** (`https://api.pushover.net/1/messages.json`)
2. The token and user key are injected into the JSON payload body
3. There is no need for URL construction involving the token

---

## 3. Technical Specifications

### 3.1 Backend — Feature Flags

**File:** `backend/internal/notifications/feature_flags.go`

Add constant:

```go
FlagPushoverServiceEnabled = "feature.notifications.service.pushover.enabled"
```

### 3.2 Backend — Router

**File:** `backend/internal/notifications/router.go`

Add to `ShouldUseNotify()` switch:

```go
case "pushover":
    return flags[FlagPushoverServiceEnabled]
```

> Also add missing `case "slack"` while here.

### 3.3 Backend — Notification Service

**File:** `backend/internal/services/notification_service.go`

#### `isSupportedNotificationProviderType()` (L137)

```go
case "discord", "email", "gotify", "webhook", "telegram", "slack", "pushover":
    return true
```

#### `isDispatchEnabled()` (L146)

```go
case "pushover":
    return s.getFeatureFlagValue(notifications.FlagPushoverServiceEnabled, true)
```

Pushover is **enabled by default** once the provider is created, matching Gotify/Webhook/Telegram/Slack behavior. Feature flag = admin kill-switch.

#### `supportsJSONTemplates()` (L128)

```go
case "webhook", "discord", "gotify", "slack", "generic", "telegram", "pushover":
    return true
```

#### `sendJSONPayload()` — Payload Validation (after `case "telegram":` block, ~L492)

```go
case "pushover":
    // Pushover requires 'message' field
    if _, hasMessage := jsonPayload["message"]; !hasMessage {
        return fmt.Errorf("pushover payload requires 'message' field")
    }
    // Emergency priority (2) requires retry and expire — reject until supported
    if priority, ok := jsonPayload["priority"]; ok {
        if p, isFloat := priority.(float64); isFloat && p == 2 {
            return fmt.Errorf("pushover emergency priority (2) requires retry and expire parameters; not yet supported")
        }
    }
```

Unlike Telegram/Discord/Slack, Pushover already uses `message` as its required field name — no auto-mapping needed because the built-in `minimal` and `detailed` templates already emit a `"message"` field.

> **[Supervisor Feedback]** The emergency priority guard is mandatory in MVP. Pushover returns HTTP 400 if `priority=2` is sent without `retry` and `expire`. This guard prevents silent dispatch failures from custom templates.

#### `sendJSONPayload()` — Dispatch Branch (~L512)

Add `"pushover"` to the `httpWrapper.Send()` dispatch branch:

```go
if providerType == "gotify" || providerType == "webhook" || providerType == "telegram" || providerType == "slack" || providerType == "pushover" {
```

Then add the Pushover-specific dispatch logic:

```go
if providerType == "pushover" {
    decryptedToken := p.Token
    if strings.TrimSpace(decryptedToken) == "" {
        return fmt.Errorf("pushover API token is not configured")
    }

    pushoverAPIURL := "https://api.pushover.net/1/messages.json"
    dispatchURL = pushoverAPIURL

    // SSRF mitigation: validate URL hostname
    parsedURL, parseErr := neturl.Parse(dispatchURL)
    if parseErr != nil || parsedURL.Hostname() != "api.pushover.net" {
        return fmt.Errorf("pushover dispatch URL validation failed: invalid hostname")
    }

    // Inject token and user into the JSON payload body
    jsonPayload["token"] = decryptedToken
    jsonPayload["user"] = p.URL // URL field stores the User Key

    // Re-marshal with injected fields
    updatedBody, marshalErr := json.Marshal(jsonPayload)
    if marshalErr != nil {
        return fmt.Errorf("failed to marshal pushover payload: %w", marshalErr)
    }
    body.Reset()
    body.Write(updatedBody)
}
```

**Security note:** The `token` and `user` fields are injected server-side from trusted DB storage, never from the template/config. Even if a custom template tries to set `"token"`, the server-side injection overwrites it.

### 3.4 Backend — Handler Layer

**File:** `backend/internal/api/handlers/notification_provider_handler.go`

#### `Create()` Type Guard (L186)

Add `&& providerType != "pushover"`:

```go
if providerType != "discord" && providerType != "gotify" && providerType != "webhook" && providerType != "email" && providerType != "telegram" && providerType != "slack" && providerType != "pushover" {
```

#### `Update()` Type Guard (L246)

Same addition.

#### `Update()` Token Preservation (L250)

Add `providerType == "pushover"`:

```go
if (providerType == "gotify" || providerType == "telegram" || providerType == "slack" || providerType == "pushover") && strings.TrimSpace(req.Token) == "" {
    req.Token = existing.Token
}
```

#### `Test()` Token Write-Only Guard (L312)

Add Pushover token write-only check:

```go
if providerType == "pushover" && strings.TrimSpace(req.Token) != "" {
    respondSanitizedProviderError(c, http.StatusBadRequest, "TOKEN_WRITE_ONLY", "validation", "Pushover API token is accepted only on provider create/update")
    return
}
```

> **[Supervisor Feedback — Adjacent Fix]** Telegram is missing the same write-only guard that Gotify and Slack have. Add it in the same PR as a one-line fix:
> ```go
> if providerType == "telegram" && strings.TrimSpace(req.Token) != "" {
>     respondSanitizedProviderError(c, http.StatusBadRequest, "TOKEN_WRITE_ONLY", "validation", "Telegram bot token is accepted only on provider create/update")
>     return
> }
> ```

#### `Test()` URL Empty Check (L350)

The existing check skips URL validation for Slack (`providerType != "slack"`). Pushover stores User Key in URL, so it is required. No change needed — the existing check enforces non-empty URL for all types except Slack.

### 3.5 Backend — Enhanced Security Service

**File:** `backend/internal/services/enhanced_security_notification_service.go`

#### `getProviderAggregatedConfig()` supportedTypes (L87)

```go
supportedTypes := map[string]bool{
    "webhook":  true,
    "discord":  true,
    "slack":    true,
    "gotify":   true,
    "telegram": true,
    "pushover": true,
}
```

> **Note:** `SendViaProviders()` and `dispatchToProvider()` currently enforce **Discord-only** for security events. Pushover will NOT receive security events until that restriction is lifted in a future PR. Adding `"pushover"` to `getProviderAggregatedConfig()` only affects the configuration aggregation (GET path), not dispatch.

### 3.6 Backend — Model (No Changes)

The `NotificationProvider` model already has all needed fields. No migration required.

### 3.7 Frontend — API Client

**File:** `frontend/src/api/notifications.ts`

#### Supported Types (L3)

```typescript
export const SUPPORTED_NOTIFICATION_PROVIDER_TYPES = ['discord', 'gotify', 'webhook', 'email', 'telegram', 'slack', 'pushover'] as const;
```

#### `sanitizeProviderForWriteAction()` (L62)

Add `type !== 'pushover'` to the token guard:

```typescript
if (type !== 'gotify' && type !== 'telegram' && type !== 'slack' && type !== 'pushover') {
    delete payload.token;
    return payload;
}
```

### 3.8 Frontend — Notifications Page

**File:** `frontend/src/pages/Notifications.tsx`

#### Type Select Options (~L204)

Add after the Slack option:

```tsx
<option value="pushover">Pushover</option>
```

#### Computed Flag

```typescript
const isPushover = type === 'pushover';
```

#### `supportsJSONTemplates()` (L24)

```typescript
return t === 'discord' || t === 'gotify' || t === 'webhook' || t === 'telegram' || t === 'slack' || t === 'pushover';
```

#### `normalizeProviderPayloadForSubmit()` (L47)

Add pushover to token handling:

```typescript
if (type === 'gotify' || type === 'telegram' || type === 'slack' || type === 'pushover') {
```

#### Token Clear `useEffect` (L153)

```typescript
if (type !== 'gotify' && type !== 'telegram' && type !== 'slack' && type !== 'pushover') {
```

#### Token Input Field (~L254)

Expand conditional and add Pushover label:

```tsx
{(isGotify || isTelegram || isSlack || isPushover) && (
    <div>
        <label htmlFor="provider-gotify-token" className="...">
            {isSlack ? t('notificationProviders.slackWebhookUrl')
             : isTelegram ? t('notificationProviders.telegramBotToken')
             : isPushover ? t('notificationProviders.pushoverApiToken')
             : t('notificationProviders.gotifyToken')}
        </label>
        <input
            ...existing props...
            placeholder={initialData?.has_token
                ? t('notificationProviders.gotifyTokenKeepPlaceholder')
                : isSlack ? t('notificationProviders.slackWebhookUrlPlaceholder')
                : isTelegram ? t('notificationProviders.telegramBotTokenPlaceholder')
                : isPushover ? t('notificationProviders.pushoverApiTokenPlaceholder')
                : t('notificationProviders.gotifyTokenPlaceholder')}
        />
        ...existing stored indicator and hint...
    </div>
)}
```

#### URL Field Label and Placeholder (~L215-240)

Update the URL label ternary chain to include Pushover:

```tsx
{isEmail
    ? t('notificationProviders.recipients')
    : isTelegram
        ? t('notificationProviders.telegramChatId')
        : isSlack
            ? t('notificationProviders.slackChannelName')
            : isPushover
                ? t('notificationProviders.pushoverUserKey')
                : <>{t('notificationProviders.urlWebhook')} <span aria-hidden="true">*</span></>}
```

Update the placeholder:

```tsx
placeholder={
    isEmail ? 'user@example.com, admin@example.com'
    : isTelegram ? '987654321'
    : isSlack ? '#general'
    : isPushover ? 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG'
    : type === 'discord' ? 'https://discord.com/api/webhooks/...'
    : type === 'gotify' ? 'https://gotify.example.com/message'
    : 'https://example.com/webhook'
}
```

#### URL Validation

Pushover User Key is not a URL, so skip URL format validation (like Telegram and Email):

```tsx
{...register('url', {
    required: (isEmail || isSlack) ? false : (t('notificationProviders.urlRequired') as string),
    validate: (isEmail || isTelegram || isSlack || isPushover) ? undefined : validateUrl,
})}
```

Note: Pushover User Key IS required (unlike Slack channel name), so it remains in the `required` logic. Only URL format validation is skipped.

### 3.9 Frontend — i18n Strings

**File:** `frontend/src/locales/en/translation.json`

Add to the `notificationProviders` section (after the Slack entries):

```json
"pushover": "Pushover",
"pushoverApiToken": "API Token (Application)",
"pushoverApiTokenPlaceholder": "Enter your Pushover Application API Token",
"pushoverUserKey": "User Key",
"pushoverUserKeyPlaceholder": "uQiRzpo4DXghDmr9QzzfQu27cmVRsG",
"pushoverUserKeyHelp": "Your Pushover user or group key. The API token is stored securely and separately."
```

### 3.10 API Contract (No Changes)

The existing REST endpoints remain unchanged:

| Method | Endpoint | Notes |
|--------|----------|-------|
| `GET` | `/api/v1/notifications/providers` | Returns all providers (token stripped) |
| `POST` | `/api/v1/notifications/providers` | Create — now accepts `type: "pushover"` |
| `PUT` | `/api/v1/notifications/providers/:id` | Update — token preserved if omitted |
| `DELETE` | `/api/v1/notifications/providers/:id` | Delete — no type-specific logic |
| `POST` | `/api/v1/notifications/providers/test` | Test — routes through `sendJSONPayload` |

---

## 4. Implementation Plan

### Phase 1: Playwright E2E Tests (Test-First)

**Rationale:** Per project conventions — write feature behaviour tests first.

#### New File: `tests/settings/pushover-notification-provider.spec.ts`

Modeled after `tests/settings/telegram-notification-provider.spec.ts` and `tests/settings/slack-notification-provider.spec.ts`.

**Test Sections:**

```
test.describe('Pushover Notification Provider')
├── test.describe('Form Rendering')
│   ├── should show API token field and user key placeholder when pushover type selected
│   ├── should toggle form fields when switching between pushover and discord types
│   └── should show JSON template section for pushover
├── test.describe('CRUD Operations')
│   ├── should create pushover notification provider
│   │   └── Verify payload: type=pushover, url=<user_key>, token=<api_token>, gotify_token=undefined
│   ├── should edit pushover notification provider and preserve token
│   │   └── Verify update omits token when unchanged
│   ├── should test pushover notification provider
│   └── should delete pushover notification provider
├── test.describe('Security')
│   ├── GET response should NOT expose API token
│   └── API token should not leak in URL field
└── test.describe('Payload Contract')
    └── POST payload should use correct field mapping
```

#### Update File: `tests/fixtures/notifications.ts`

Add to `NotificationProviderType` union:

```typescript
export type NotificationProviderType =
  | 'discord'
  | 'slack'
  | 'gotify'
  | 'telegram'
  | 'generic'
  | 'email'
  | 'webhook'
  | 'pushover';
```

Add fixtures:

```typescript
export const pushoverProvider: NotificationProviderConfig = {
  name: generateProviderName('pushover'),
  type: 'pushover',
  url: 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG',  // User Key
  token: 'azGDORePK8gMaC0QOYAMyEEuzJnyUi',  // App API Token
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: true,
  notify_uptime: true,
};
```

#### Update File: `tests/settings/notifications.spec.ts`

Provider type count assertions that currently expect 6 options need updating to 7.

The existing tests at L144 and L191 that mock pushover as an existing provider should be updated:
- **Replace Pushover mock data with a genuinely unsupported type** (e.g., `"pagerduty"`) for tests that assert "deprecated" badges. Using a real unsupported type removes ambiguity.
- Any assertions about "deprecated" or "read-only" badges for pushover must be removed since it is now a supported type.

### Phase 2: Backend Implementation

#### 2A — Feature Flags & Router (2 files)

| File | Change | Complexity |
|------|--------|------------|
| `backend/internal/notifications/feature_flags.go` | Add `FlagPushoverServiceEnabled` constant | Trivial |
| `backend/internal/notifications/router.go` | Add `case "pushover"` + `case "slack"` (missing) to `ShouldUseNotify()` | Trivial |

#### 2B — Notification Service (1 file, 5 functions)

| Function | Change | Complexity |
|----------|--------|------------|
| `isSupportedNotificationProviderType()` | Add `"pushover"` to case | Trivial |
| `isDispatchEnabled()` | Add pushover case with feature flag | Low |
| `supportsJSONTemplates()` | Add `"pushover"` to case | Trivial |
| `sendJSONPayload()` — validation | Add `case "pushover"` requiring `message` field | Low |
| `sendJSONPayload()` — dispatch | Add pushover dispatch block (inject token+user into body, SSRF pin) | Medium |

#### 2C — Handler Layer (1 file, 4 locations)

| Location | Change | Complexity |
|----------|--------|------------|
| `Create()` type guard | Add `"pushover"` | Trivial |
| `Update()` type guard | Add `"pushover"` | Trivial |
| `Update()` token preservation | Add `"pushover"` | Trivial |
| `Test()` token write-only guard | Add pushover block | Low |

#### 2D — Enhanced Security Service (1 file)

| Location | Change | Complexity |
|----------|--------|------------|
| `getProviderAggregatedConfig()` supportedTypes | Add `"pushover": true` | Trivial |

#### 2E — Backend Unit Tests (4-6 files)

| File | Change | Complexity |
|------|--------|------------|
| `notification_service_test.go` | Replace `"pushover"` as unsupported with `"sms"`. Add pushover dispatch tests (success, missing token, missing user key, SSRF validation, payload injection). Add pushover to `supportsJSONTemplates` test. | Medium |
| `notification_coverage_test.go` | Replace `Type: "pushover"` with `Type: "sms"` in Update_UnsupportedType test | Trivial |
| `notification_provider_discord_only_test.go` | Replace `"type": "pushover"` with `"type": "sms"` | Trivial |
| `security_notifications_final_blockers_test.go` | Replace `"pushover"` with `"sms"` in unsupportedTypes | Trivial |

**New pushover-specific test cases to add in `notification_service_test.go`:**

| Test Case | What It Validates |
|-----------|-------------------|
| `TestPushoverDispatch_Success` | Token + user injected into payload body, POST to `api.pushover.net`, returns nil |
| `TestPushoverDispatch_MissingToken` | Returns error when Token is empty |
| `TestPushoverDispatch_MissingUserKey` | Returns error when URL (user key) is empty |
| `TestPushoverDispatch_SSRFValidation` | Constructed URL hostname pinned to `api.pushover.net` |
| `TestPushoverDispatch_PayloadInjection` | `token` and `user` fields in body match DB values, not template-provided values |
| `TestPushoverDispatch_MessageFieldRequired` | Payload without `message` field returns error |
| `TestPushoverDispatch_EmergencyPriorityRejected` | Payload with `"priority": 2` returns error about unsupported emergency priority |
| `TestPushoverDispatch_FeatureFlagDisabled` | Dispatch skipped when flag is false |

### Phase 3: Frontend Implementation

#### 3A — API Client (1 file)

| Location | Change | Complexity |
|----------|--------|------------|
| `SUPPORTED_NOTIFICATION_PROVIDER_TYPES` | Add `'pushover'` | Trivial |
| `sanitizeProviderForWriteAction()` | Add `type !== 'pushover'` to token guard | Trivial |

#### 3B — Notifications Page (1 file, ~9 locations)

| Location | Change | Complexity |
|----------|--------|------------|
| Type `<select>` | Add `<option value="pushover">Pushover</option>` | Trivial |
| Computed flag | Add `const isPushover = type === 'pushover'` | Trivial |
| `supportsJSONTemplates()` | Add `'pushover'` | Trivial |
| `normalizeProviderPayloadForSubmit()` | Add `'pushover'` to token handling | Trivial |
| Token clear `useEffect` | Add `'pushover'` | Trivial |
| Token input conditional | Add `isPushover` | Low |
| URL label ternary | Add pushover label branch | Low |
| URL placeholder | Add pushover placeholder | Low |
| URL validation skip | Add `isPushover` | Trivial |

#### 3C — i18n (1 file)

| Change | Complexity |
|--------|------------|
| Add 6 pushover translation keys | Trivial |

#### 3D — Frontend Unit Tests (2-3 files)

| File | Change | Complexity |
|------|--------|------------|
| `frontend/src/api/notifications.test.ts` | Replace pushover rejection test with different unsupported type. Add pushover sanitization test. | Low |
| `frontend/src/api/__tests__/notifications.test.ts` | Same replacement | Low |

> **[Supervisor Feedback — Verify]** The plan lists both `frontend/src/api/notifications.test.ts` AND `frontend/src/api/__tests__/notifications.test.ts`. During implementation, verify whether these are the same tests at two paths or genuinely separate files. If duplicates, note for cleanup in a separate PR.
| `frontend/src/pages/__tests__/Notifications.test.tsx` | Replace `'Legacy Pushover'` test. Update provider type count from 6 to 7. Add pushover form rendering test. | Low |

### Phase 4: Integration and Testing

1. Run Go unit tests: `cd backend && go test ./...`
2. Run Vitest frontend tests: `cd frontend && npx vitest run`
3. Run Playwright E2E tests:
   - `npx playwright test tests/settings/pushover-notification-provider.spec.ts`
   - `npx playwright test tests/settings/notifications.spec.ts`
   - `npx playwright test tests/settings/notifications-payload.spec.ts`
4. Run full security suite: `npx playwright test --grep @security`
5. Verify no regressions in other provider tests

### Phase 5: Documentation

1. Update `CHANGELOG.md` with Pushover provider entry
2. Verify `ARCHITECTURE.md` notification section is accurate

---

## 5. Acceptance Criteria

| # | Criterion | Verification Method |
|---|-----------|-------------------|
| AC-1 | Pushover appears as an option in the provider type dropdown | E2E test: form rendering |
| AC-2 | Creating a Pushover provider stores API token in Token field and User Key in URL field | E2E test: CRUD create + payload contract |
| AC-3 | Editing a Pushover provider preserves the API token when not re-entered | E2E test: CRUD edit + payload verification |
| AC-4 | GET response never exposes the API token | E2E test: security section |
| AC-5 | Test notification dispatches to `https://api.pushover.net/1/messages.json` with token, user, and message in JSON body | Go unit test: mock HTTP round-trip |
| AC-6 | Feature flag `feature.notifications.service.pushover.enabled` controls dispatch | Go unit test: dispatch enabled/disabled |
| AC-7 | Built-in templates (minimal/detailed) work with Pushover | Go unit test: template rendering |
| AC-8 | SSRF validation rejects non-`api.pushover.net` hostnames | Go unit test: hostname pin |
| AC-9 | All existing provider tests pass without regression | CI: full test suite |
| AC-10 | Provider type dropdown shows 7 options (was 6) | E2E test: type count assertion |

---

## 6. Commit Slicing Strategy

### Decision: Single PR

**Rationale:**
- All changes are additive (no breaking changes to existing providers)
- Total scope is ~15 files modified, ~1 new E2E test file
- Cross-cutting but shallow: each file gets 1-5 lines added in established patterns
- No database migration or schema change
- Risk is low — Pushover is isolated behind a feature flag and follows a pattern proven by 6 existing providers
- Splitting would create integration risk (backend without frontend = untestable)

### PR-1: Enable Pushover Notification Provider

**Scope:** All Phase 1-5 changes in a single PR

**Files Modified (by category):**

Backend Core (4 files):
- `backend/internal/notifications/feature_flags.go`
- `backend/internal/notifications/router.go`
- `backend/internal/services/notification_service.go`
- `backend/internal/api/handlers/notification_provider_handler.go`

Backend Security (1 file):
- `backend/internal/services/enhanced_security_notification_service.go`

Backend Tests (4 files):
- `backend/internal/services/notification_service_test.go`
- `backend/internal/api/handlers/notification_coverage_test.go`
- `backend/internal/api/handlers/notification_provider_discord_only_test.go`
- `backend/internal/api/handlers/security_notifications_final_blockers_test.go`

Frontend (3 files):
- `frontend/src/api/notifications.ts`
- `frontend/src/pages/Notifications.tsx`
- `frontend/src/locales/en/translation.json`

Frontend Tests (2-3 files):
- `frontend/src/api/notifications.test.ts` or `frontend/src/api/__tests__/notifications.test.ts`
- `frontend/src/pages/__tests__/Notifications.test.tsx`

E2E Tests (3 files):
- `tests/settings/pushover-notification-provider.spec.ts` **(NEW)**
- `tests/fixtures/notifications.ts`
- `tests/settings/notifications.spec.ts`

**Validation Gates:**
1. `go test ./...` passes
2. `npx vitest run` passes
3. `npx playwright test tests/settings/pushover-notification-provider.spec.ts` passes
4. `npx playwright test tests/settings/notifications.spec.ts` passes
5. No regressions in existing provider E2E tests

**Rollback:** Revert the PR. Feature flag `feature.notifications.service.pushover.enabled` can be set to `false` as an immediate mitigation without code revert.

---

## 7. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Pushover API rejects JSON Content-Type | Low | Medium | Pushover docs explicitly support JSON with `Content-Type: application/json`. Verified in API docs. |
| Token leaks in API response | Low | High | Token uses `json:"-"` tag. E2E security tests verify no leakage. Server-side injection overwrites template-provided tokens. |
| SSRF via tampered dispatch URL | Low | High | Hostname pinned to `api.pushover.net`. URL is hardcoded, not derived from user input. |
| Existing tests break | Medium | Low | Well-identified: 6 backend + 3 frontend test files use pushover as "unsupported". All listed with replacement values. |
| Pushover rate limiting (10K messages/month) | Low | Low | Not a code issue — documented as operational concern. |

---

## 8. Open Questions

1. **Priority field:** Should the Pushover provider form expose a `priority` dropdown? (-2=lowest, -1=low, 0=normal, 1=high, 2=emergency). This could be stored in `ServiceConfig` as JSON. **Recommendation:** Defer to a follow-up PR. The `message` field is sufficient for MVP.

2. **Sound selection:** Pushover supports ~22 built-in sounds. Same question as priority. **Recommendation:** Defer.

3. **Emergency priority (2):** ~~Requires `retry` and `expire` parameters.~~ **Resolved:** A validation guard in `sendJSONPayload()` now rejects `priority=2` with a clear error message. This is enforced in MVP. See section 3.3 and test `TestPushoverDispatch_EmergencyPriorityRejected`. Full emergency priority support (with `retry`/`expire` parameters) is deferred to a follow-up PR.

---

## 9. Supervisor Review — Incorporated Feedback

The following items were raised during Supervisor review and have been incorporated into the plan inline. This section serves as a consolidated reference.

### IMPORTANT — Must Address During Implementation

| # | Item | Resolution | Inline Location |
|---|------|------------|----------------|
| 1 | **`nonDiscordTypes` test at `notification_service_test.go` ~L2190** — Pushover must remain in the `nonDiscordTypes` list. This test verifies migration logic, not whether a type is supported. | **Keep `pushover` in the list.** Update test assertions to reflect Pushover is now supported but still participates in migration. | Section 2.3, row for L2190 |
| 2 | **Emergency priority (priority=2) validation guard** — Nothing prevents a custom template from setting `"priority": 2`. Pushover rejects this with HTTP 400 unless `retry` and `expire` are also provided. | **Added validation guard** in `sendJSONPayload()` pushover case that rejects priority=2 with a clear error. Added corresponding unit test `TestPushoverDispatch_EmergencyPriorityRejected`. | Section 3.3 (`sendJSONPayload` validation), Phase 2E test table, Section 8 Open Question 3 |
| 3 | **Duplicate frontend test files** — Plan lists both `frontend/src/api/notifications.test.ts` AND `frontend/src/api/__tests__/notifications.test.ts`. | **Verify during implementation** whether these are the same tests at two paths or genuinely separate files. If duplicates, note for cleanup in a separate PR. | Phase 3D frontend tests table |

### SUGGESTIONS — Incorporated

| # | Item | Resolution | Inline Location |
|---|------|------------|----------------|
| 4 | **Fix missing Slack case in `router.go`** — `slack` is missing from `ShouldUseNotify()`. | **Include in same PR.** Trivial adjacent fix already noted in plan; strengthened inline annotation. | Section 2.2 row 2, Section 3.2 |
| 5 | **Add Telegram's missing write-only guard in `Test()` handler** — Telegram is missing the token write-only guard that Gotify, Slack, and now Pushover have. | **Include as one-line adjacent fix in same PR.** Implementation snippet added inline. | Section 2.2 row 11, Section 3.4 `Test()` handler |
| 6 | **`notifications.spec.ts` E2E test mock data** — Tests at L144 and L191 mock Pushover as a deprecated provider; this is now ambiguous. | **Replace mock data with a genuinely unsupported type** (e.g., `"pagerduty"`) to avoid confusion. | Phase 1, `notifications.spec.ts` update section |

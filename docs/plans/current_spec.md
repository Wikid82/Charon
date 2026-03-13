# Slack Notification Provider — Implementation Specification

**Status:** Draft
**Created:** 2026-03-12
**Target:** Single PR (backend + frontend + E2E + docs)

---

## 1. Overview

### What

Add Slack as a supported notification provider type, using Slack Incoming Webhooks to post messages to channels. The webhook URL (`https://hooks.slack.com/services/T.../B.../xxx`) acts as the authentication mechanism — no separate API key is required.

### How it Fits

Slack follows the **exact same pattern** as Discord, Gotify, Telegram, and Generic Webhook providers. It:
- Uses `sendJSONPayload()` for dispatch (same as Discord/Gotify/Telegram/Webhook)
- Requires `text` or `blocks` in the payload (validation already exists in `sendJSONPayload`)
- Stores the webhook URL in the `Token` column (same security treatment as Telegram bot token)
- Is gated by a feature flag `feature.notifications.service.slack.enabled`

### Webhook URL Security

The Slack webhook URL contains embedded authentication credentials. The **entire Slack webhook URL is sensitive**. It must be treated the same as Telegram bot tokens and Gotify tokens:
- Redacted from `GET /api/v1/notifications/providers` responses
- Stored in the `Token` column (uses `json:"-"` tag) — never returned in plaintext to the frontend
- Frontend shows a masked placeholder and "stored" indicator via `HasToken`

**Decision: Webhook URL stored in `Token` field.**
To reuse the existing token-redaction infrastructure (`json:"-"` tag, `HasToken` computed field, write-only semantics), the Slack webhook URL will be stored in the `Token` column, NOT the `URL` column. The `URL` column will hold an optional display-safe channel name. This follows the same security pattern as Telegram (where the bot token goes in `Token` and the chat ID goes in `URL`).

| Field | Slack Usage | Example |
|-------|------------|---------|
| `Token` | Full webhook URL (write-only, redacted) | `https://hooks.slack.com/services/T00.../B00.../xxxx` |
| `URL` | Channel display name (optional, user-facing) | `#alerts` |
| `HasToken` | `true` when webhook URL is set | — |

---

## 2. Backend Changes (Go)

### 2.1 `backend/internal/notifications/feature_flags.go`

**Add constant:**

```go
FlagSlackServiceEnabled = "feature.notifications.service.slack.enabled"
```

Add it below `FlagTelegramServiceEnabled` in the `const` block.

### 2.2 `backend/internal/services/notification_service.go`

#### 2.2.1 `isSupportedNotificationProviderType()`

Add `"slack"` to the switch:

```go
case "discord", "email", "gotify", "webhook", "telegram", "slack":
    return true
```

#### 2.2.2 `isDispatchEnabled()`

Add slack case:

```go
case "slack":
    return s.getFeatureFlagValue(notifications.FlagSlackServiceEnabled, true)
```

Default enabled (`true`) to match the Gotify/Telegram/Webhook pattern.

#### 2.2.3 `supportsJSONTemplates()`

`"slack"` is **already listed** in this function (approx line 109). No change needed.

#### 2.2.4 Slack Webhook URL Validation

Add a new function and regex near the existing `discordWebhookRegex`:

```go
var slackWebhookRegex = regexp.MustCompile(`^https://hooks\.slack\.com/services/T[A-Za-z0-9_-]+/B[A-Za-z0-9_-]+/[A-Za-z0-9_-]+$`)

func validateSlackWebhookURL(rawURL string) error {
    if !slackWebhookRegex.MatchString(rawURL) {
        return fmt.Errorf("invalid Slack webhook URL: must match https://hooks.slack.com/services/T.../B.../xxx")
    }
    return nil
}
```

**Validation rules:**
- Must be HTTPS
- Host must be `hooks.slack.com`
- Path must match `/services/T<workspace>/B<bot>/<token>` pattern
- No IP addresses, no query parameters
- Test hook: add `var validateSlackProviderURLFunc = validateSlackWebhookURL` for testability

#### 2.2.5 `sendJSONPayload()` — Dispatch path

**Step A.** Extend the provider routing condition (approx line 465) to include `"slack"`:

```go
if providerType == "gotify" || providerType == "webhook" || providerType == "telegram" || providerType == "slack" {
```

**Step B.** Add Slack-specific dispatch logic inside the block, after the Telegram block:

```go
if providerType == "slack" {
    decryptedWebhookURL := p.Token
    if strings.TrimSpace(decryptedWebhookURL) == "" {
        return fmt.Errorf("slack webhook URL is not configured")
    }
    if err := validateSlackProviderURLFunc(decryptedWebhookURL); err != nil {
        return err
    }
    dispatchURL = decryptedWebhookURL
}
```

**Step C.** Replace the existing `case "slack":` block entirely with:

```go
case "slack":
    if _, hasText := jsonPayload["text"]; !hasText {
        if _, hasBlocks := jsonPayload["blocks"]; !hasBlocks {
            if messageValue, hasMessage := jsonPayload["message"]; hasMessage {
                jsonPayload["text"] = messageValue
                normalizedBody, marshalErr := json.Marshal(jsonPayload)
                if marshalErr != nil {
                    return fmt.Errorf("failed to normalize slack payload: %w", marshalErr)
                }
                body.Reset()
                if _, writeErr := body.Write(normalizedBody); writeErr != nil {
                    return fmt.Errorf("failed to write normalized slack payload: %w", writeErr)
                }
            } else {
                return fmt.Errorf("slack payload requires 'text' or 'blocks' field")
            }
        }
    }
```

#### 2.2.6 `CreateProvider()` — Token field handling

Update the token-clearing logic:

```go
if provider.Type != "gotify" && provider.Type != "telegram" && provider.Type != "slack" {
    provider.Token = ""
}
```

#### 2.2.7 `UpdateProvider()` — Token preservation

Update the token-preservation logic:

```go
if provider.Type == "gotify" || provider.Type == "telegram" || provider.Type == "slack" {
    if strings.TrimSpace(provider.Token) == "" {
        provider.Token = existing.Token
    }
} else {
    provider.Token = ""
}
```

### 2.3 `backend/internal/api/handlers/notification_provider_handler.go`

#### 2.3.1 `Create()` — Type whitelist

Add `"slack"` to the type validation:

```go
if providerType != "discord" && providerType != "gotify" && providerType != "webhook" &&
   providerType != "email" && providerType != "telegram" && providerType != "slack" {
```

#### 2.3.2 `Update()` — Type whitelist

Same addition:

```go
if providerType != "discord" && providerType != "gotify" && providerType != "webhook" &&
   providerType != "email" && providerType != "telegram" && providerType != "slack" {
```

#### 2.3.3 `Update()` — Token preservation on empty update

Add `"slack"` to the token-keep condition:

```go
if (providerType == "gotify" || providerType == "telegram" || providerType == "slack") &&
    strings.TrimSpace(req.Token) == "" {
    req.Token = existing.Token
}
```

#### 2.3.4 `Test()` — Token write-only guard

Add a Slack guard alongside the existing Gotify check:

```go
if providerType == "slack" && strings.TrimSpace(req.Token) != "" {
    respondSanitizedProviderError(c, http.StatusBadRequest, "TOKEN_WRITE_ONLY", "validation",
        "Slack webhook URL is accepted only on provider create/update")
    return
}
```

#### 2.3.5 `classifyProviderTestFailure()` — Slack-specific errors

Slack returns plain-text error strings (e.g., `"invalid_payload"`), not JSON. Add classification after the existing status code matching block:

```go
if strings.Contains(errText, "invalid_payload") ||
    strings.Contains(errText, "missing_text_or_fallback") {
    return "PROVIDER_TEST_VALIDATION_FAILED", "validation",
        "Slack rejected the payload. Ensure your template includes a 'text' or 'blocks' field"
}
if strings.Contains(errText, "no_service") {
    return "PROVIDER_TEST_AUTH_REJECTED", "dispatch",
        "Slack webhook is revoked or the app is disabled. Create a new webhook"
}
```

#### 2.3.6 `Test()` — URL empty guard for Slack

The `Test()` handler rejects providers where `URL` is empty. For Slack, `URL` holds the optional channel display name — the dispatch target is the webhook URL stored in `Token`. Add Slack exemption:

```go
if providerType != "slack" && strings.TrimSpace(provider.URL) == "" {
    respondSanitizedProviderError(c, http.StatusBadRequest, "PROVIDER_CONFIG_MISSING", ...)
    return
}
```

#### 2.3.7 `isProviderValidationError()` — Slack validation errors

The function checks for specific error message strings to return 400 instead of 500. Add Slack:

```go
strings.Contains(errMsg, "invalid Slack webhook URL")
```

Without this, malformed Slack webhook URLs return HTTP 500 instead of 400.

### 2.4 `backend/internal/services/notification_service_test.go`

Add the following test functions:

| Test Function | Purpose |
|---|---|
| `TestSlackWebhookURLValidation` | Table-driven: valid/invalid URL patterns for `validateSlackWebhookURL` |
| `TestSlackWebhookURLValidation_RejectsHTTP` | Rejects `http://hooks.slack.com/...` |
| `TestSlackWebhookURLValidation_RejectsIPAddress` | Rejects `https://192.168.1.1/services/...` |
| `TestSlackWebhookURLValidation_RejectsWrongHost` | Rejects `https://evil.com/services/...` |
| `TestSlackWebhookURLValidation_RejectsQueryParams` | Rejects URLs with `?token=...` |
| `TestNotificationService_CreateProvider_Slack` | Creates Slack provider, verifies token stored, URL is channel name |
| `TestNotificationService_CreateProvider_Slack_ClearsTokenField` | Verifies non-Slack types don't keep token |
| `TestNotificationService_UpdateProvider_Slack_PreservesToken` | Updates name without clearing webhook URL |
| `TestNotificationService_TestProvider_Slack` | Tests dispatch through mock HTTP server |
| `TestNotificationService_SendExternal_Slack` | Event filtering + dispatch via goroutine; mock webhook server |
| `TestNotificationService_Slack_PayloadNormalizesMessageToText` | Minimal template `message` → `text` normalization |
| `TestNotificationService_Slack_PayloadRequiresTextOrBlocks` | Custom template without `text`/`blocks`/`message` fails |
| `TestFlagSlackServiceEnabled_ConstantValue` | `notifications.FlagSlackServiceEnabled == "feature.notifications.service.slack.enabled"` |
| `TestNotificationService_Slack_IsDispatchEnabled` | Feature flag true/false gating |
| `TestNotificationService_Slack_TokenNotExposedInList` | `ListProviders` redaction: HasToken=true, Token="" |

### 2.5 No Changes Required

| File | Reason |
|------|--------|
| `backend/internal/models/notification_provider.go` | Existing `Token`, `URL`, `HasToken` fields sufficient |
| `backend/internal/notifications/http_wrapper.go` | Slack webhooks are standard HTTPS POST |
| `backend/internal/api/routes/routes.go` | No new model to auto-migrate |
| `Dockerfile` | No new dependencies |
| `.gitignore` | No new artifacts |
| `codecov.yml` | No new paths to exclude |
| `.dockerignore` | No new paths |

---

## 3. Frontend Changes (React/TypeScript)

### 3.1 `frontend/src/api/notifications.ts`

#### 3.1.1 `SUPPORTED_NOTIFICATION_PROVIDER_TYPES`

Add `'slack'`:

```typescript
export const SUPPORTED_NOTIFICATION_PROVIDER_TYPES = [
  'discord', 'gotify', 'webhook', 'email', 'telegram', 'slack'
] as const;
```

#### 3.1.2 `sanitizeProviderForWriteAction()`

Add `'slack'` to the token-preserving types:

```typescript
if (type !== 'gotify' && type !== 'telegram' && type !== 'slack') {
    delete payload.token;
    return payload;
}
```

### 3.2 `frontend/src/pages/Notifications.tsx`

#### 3.2.1 `normalizeProviderPayloadForSubmit()`

Add `'slack'` to the token-preserving types:

```typescript
if (type === 'gotify' || type === 'telegram' || type === 'slack') {
```

#### 3.2.2 Provider type `<select>` options

Add Slack option after Telegram:

```tsx
<option value="telegram">{t('notificationProviders.telegram')}</option>
<option value="slack">{t('notificationProviders.slack')}</option>
```

#### 3.2.3 `ProviderForm` — Conditional field rendering

**Add new derived boolean:**

```typescript
const isSlack = type === 'slack';
```

**URL field label update:**

```typescript
{isEmail
  ? t('notificationProviders.recipients')
  : isTelegram
    ? t('notificationProviders.telegramChatId')
    : isSlack
      ? t('notificationProviders.slackChannelName')
      : <>{t('notificationProviders.urlWebhook')} <span aria-hidden="true">*</span></>}
```

**URL field validation update — Slack URL is optional:**

```typescript
required: (isEmail || isSlack) ? false : (t('notificationProviders.urlRequired') as string),
validate: (isEmail || isTelegram || isSlack) ? undefined : validateUrl,
```

**URL placeholder update:**

```typescript
placeholder={isEmail ? 'user@example.com, admin@example.com'
  : isTelegram ? '987654321'
  : isSlack ? '#general'
  : type === 'discord' ? 'https://discord.com/api/webhooks/...'
  : type === 'gotify' ? 'https://gotify.example.com/message'
  : 'https://example.com/webhook'}
```

**Token field visibility — show for Slack:**

```typescript
{(isGotify || isTelegram || isSlack) && (
```

**Token field label — Slack-specific:**

```typescript
{isSlack
  ? t('notificationProviders.slackWebhookUrl')
  : isTelegram
    ? t('notificationProviders.telegramBotToken')
    : t('notificationProviders.gotifyToken')}
```

**Token placeholder — Slack-specific:**

```typescript
placeholder={initialData?.has_token
  ? t('notificationProviders.gotifyTokenKeepPlaceholder')
  : isSlack
    ? t('notificationProviders.slackWebhookUrlPlaceholder')
    : isTelegram
      ? t('notificationProviders.telegramBotTokenPlaceholder')
      : t('notificationProviders.gotifyTokenPlaceholder')}
```

#### 3.2.4 `supportsJSONTemplates()`

Add `'slack'`:

```typescript
return t === 'discord' || t === 'gotify' || t === 'webhook' || t === 'telegram' || t === 'slack';
```

#### 3.2.5 `useEffect` for clearing token

Update to include `'slack'`:

```typescript
if (type !== 'gotify' && type !== 'telegram' && type !== 'slack') {
```

### 3.3 `frontend/src/locales/en/translation.json`

Add these keys inside the `"notificationProviders"` object, after the Telegram keys:

```json
"slack": "Slack",
"slackWebhookUrl": "Webhook URL",
"slackWebhookUrlPlaceholder": "https://hooks.slack.com/services/T.../B.../xxx",
"slackChannelName": "Channel Name (optional)",
"slackChannelNameHelp": "Display name for the channel. The actual channel is determined by the webhook configuration."
```

### 3.4 `frontend/src/pages/__tests__/Notifications.test.tsx`

Add test cases:

| Test | Purpose |
|---|---|
| `it('shows 6 supported provider type options including slack')` | Verify options: discord, gotify, webhook, email, telegram, slack |
| `it('shows token field when slack type selected')` | Token input visible when slack selected |
| `it('hides token field when switching from slack to discord')` | Token input removed on switch |
| `it('submits slack provider with token as webhook URL')` | Verify `createProvider` receives `token` field |
| `it('does not require URL for slack')` | No validation error when URL empty for slack |

Update the existing `'shows supported provider type options'` test to expect 6 options instead of 5.

### 3.5 Accessibility

- The token field for Slack uses `htmlFor="provider-gotify-token"` (existing `id`)
- `aria-describedby` points to `gotify-token-stored-hint` when `has_token` is set
- URL field label correctly associates via `htmlFor="provider-url"`
- Keyboard navigation order unchanged within form
- Screen readers announce "Webhook URL" for the token field when Slack is selected

---

## 4. E2E Tests (Playwright)

### 4.1 New File: `tests/settings/slack-notification-provider.spec.ts`

Follow the **exact same structure** as `tests/settings/telegram-notification-provider.spec.ts`.

#### Test Structure

```
Slack Notification Provider
├── Form Rendering
│   ├── should show webhook URL field and channel name when slack type selected
│   ├── should toggle form fields when switching between slack and discord types
│   └── should show JSON template section for slack
├── CRUD Operations
│   ├── should create slack notification provider
│   ├── should edit slack notification provider and preserve webhook URL
│   ├── should test slack notification provider
│   └── should delete slack notification provider
└── Security
    ├── GET response should NOT expose webhook URL
    └── webhook URL should NOT be present in URL field
```

#### Key Implementation Details

**Mock route patterns:**

```typescript
await page.route('**/api/v1/notifications/providers', async (route, request) => { ... });
await page.route('**/api/v1/notifications/providers/*', async (route, request) => { ... });
await page.route('**/api/v1/notifications/providers/test', async (route, request) => { ... });
```

**Provider mock data:**

```typescript
{
  id: 'slack-provider-1',
  name: 'Slack Alerts',
  type: 'slack',
  url: '#alerts',
  has_token: true,
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: true,
  notify_uptime: false,
}
```

**Create payload verification:**

```typescript
expect(capturedPayload?.type).toBe('slack');
expect(capturedPayload?.token).toBe('https://hooks.slack.com/services/T00000/B00000/xxxx');
expect(capturedPayload?.url).toBe('#alerts');
expect(capturedPayload?.gotify_token).toBeUndefined();
```

**Security tests — GET response must NOT contain webhook URL:**

```typescript
expect(provider.token).toBeUndefined();
expect(provider.gotify_token).toBeUndefined();
const responseStr = JSON.stringify(provider);
expect(responseStr).not.toContain('hooks.slack.com');
expect(responseStr).not.toContain('/services/');
```

**Firefox-stable patterns (mandatory for all save/delete actions):**

```typescript
// CORRECT: Register listeners BEFORE the click
await Promise.all([
  page.waitForResponse(
    (resp) =>
      /\/api\/v1\/notifications\/providers\/slack-edit-id/.test(resp.url()) &&
      resp.request().method() === 'PUT' &&
      resp.status() === 200
  ),
  page.waitForResponse(
    (resp) =>
      /\/api\/v1\/notifications\/providers/.test(resp.url()) &&
      resp.request().method() === 'GET' &&
      resp.status() === 200
  ),
  page.getByTestId('provider-save-btn').click(),
]);
```

### 4.2 Updates to `tests/settings/notifications-payload.spec.ts`

Add Slack to the payload matrix test scenario array (alongside Discord, Gotify, Webhook):

```typescript
{
  type: 'slack',
  name: `slack-matrix-${Date.now()}`,
  url: '#slack-alerts',
}
```

### 4.3 No Changes to Other E2E Files

- `tests/settings/notifications.spec.ts` — operates on Discord by default, no changes needed
- `tests/settings/telegram-notification-provider.spec.ts` — Telegram-specific, no changes needed

---

## 5. Documentation Updates

### 5.1 `docs/features/notifications.md`

**Supported Services table** — Add Slack row:

| Service | JSON Templates | Native API | Rich Formatting |
|---------|----------------|------------|-----------------|
| **Slack** | ✅ Yes | ✅ Webhooks | ✅ Blocks + Attachments |

**Add Slack section** after "Email Notifications", before "Planned Provider Expansion":

```markdown
### Slack Notifications

Slack notifications post messages to channels using Incoming Webhooks.

**Setup:**

1. In Slack, go to your workspace's **App Management** → **Incoming Webhooks**
2. Create a new webhook and select the target channel
3. Copy the webhook URL
4. In Charon, go to **Settings** → **Notifications** → **Add Provider**
5. Select **Slack** as the service type
6. Paste the webhook URL in the Webhook URL field
7. Optionally enter a channel display name
8. Configure notification triggers and save

> **Note:** The webhook URL is stored securely and never exposed in API responses.
```

**Update "Planned Provider Expansion"** — Remove Slack from the planned list since it is now implemented.

### 5.2 `CHANGELOG.md`

Add entry under `## [Unreleased]`:

```markdown
### Added
- Slack notification provider with Incoming Webhook support
```

---

## 6. Commit Slicing Strategy

### Decision: Single PR

**Rationale:**
- Slack follows an established, well-tested pattern (identical to Telegram)
- All changes are tightly coupled: backend type support, frontend form, E2E tests
- Estimated diff is moderate (~670 lines across 12 files)
- No schema migration or infrastructure changes
- No cross-domain risk (no security module changes, no Caddy changes)
- Feature flag provides safe rollback without code revert

### PR Scope

| Domain | Files | Lines (est.) |
|--------|-------|-------------|
| Backend - Feature flag | `feature_flags.go` | +1 |
| Backend - Service | `notification_service.go` | +40 |
| Backend - Handler | `notification_provider_handler.go` | +15 |
| Backend - Tests | `notification_service_test.go` | +150 |
| Frontend - API | `notifications.ts` | +5 |
| Frontend - Page | `Notifications.tsx` | +30 |
| Frontend - i18n | `translation.json` | +5 |
| Frontend - Tests | `Notifications.test.tsx` | +40 |
| E2E Tests | `slack-notification-provider.spec.ts` (new) | +350 |
| E2E Tests | `notifications-payload.spec.ts` | +5 |
| Docs | `notifications.md`, `CHANGELOG.md` | +30 |

**Total estimated:** ~670 lines

### Validation Gates

1. `go test ./backend/...` — all backend tests pass (including new Slack tests)
2. `cd frontend && npx vitest run` — frontend unit tests pass
3. `npx playwright test tests/settings/slack-notification-provider.spec.ts --project=firefox` — Slack E2E passes
4. `npx playwright test tests/settings/notifications-payload.spec.ts --project=firefox` — payload matrix passes
5. `make lint-fast` — staticcheck passes
6. Manual smoke test: create Slack provider → send test notification → verify in Slack channel

### Rollback

If issues arise post-merge:
- Set `feature.notifications.service.slack.enabled = false` in **Settings → Feature Flags**
- This immediately stops all Slack dispatch without code changes
- Existing Slack providers remain in DB but are non-dispatch

---

## 7. Risk Assessment

### 7.1 Webhook URL Security — HIGH

**Risk:** Webhook URL contains embedded credentials. If exposed in API responses, anyone with read access to the Charon API could post to the Slack channel.

**Mitigation:** Store in `Token` field (uses `json:"-"` tag). Use `HasToken` computed field for frontend indication. Follow identical pattern to Telegram bot token / Gotify token.

**Verification:** E2E security test `GET response should NOT expose webhook URL` explicitly checks the API response body.

### 7.2 Rate Limiting — LOW

**Risk:** Slack enforces 1 message/second/webhook. Burst notifications could trigger rate limiting.

**Mitigation:** Not addressed in this PR. The existing Charon notification system dispatches per-event and does not batch. Acceptable for initial rollout. Future work could add per-provider rate limiting.

### 7.3 Slack Webhook URL Format Changes — LOW

**Risk:** Slack could change their webhook URL format, breaking the validation regex.

**Mitigation:** Regex is simple and based on the documented format. Only `validateSlackWebhookURL` needs updating. Feature flag allows immediate disable as a workaround.

### 7.4 Plain-Text Error Responses — MEDIUM

**Risk:** Discord returns JSON error responses. Slack returns plain-text error strings (e.g., `"invalid_payload"`, `"channel_not_found"`). Error classification must handle both patterns.

**Mitigation:** Added Slack-specific string matching in `classifyProviderTestFailure`. The existing fallback (`PROVIDER_TEST_FAILED`) handles unknown patterns gracefully.

### 7.5 Payload Normalization — LOW

**Risk:** Minimal template produces `message` key; Slack requires `text`. Without normalization, the default template would fail.

**Mitigation:** The `sendJSONPayload` Slack case block normalizes `message` → `text` automatically (same pattern as Discord `message` → `content` and Telegram `message` → `text`).

### 7.6 Workflow Builder Webhooks — LOW

**Risk:** Slack Workflow Builder uses `/triggers/...` URLs, not `/services/...`. These are not supported by the current validation regex.

**Mitigation:** Document as a known limitation. Workflow Builder webhooks can be used via the Generic Webhook provider type. Future work could extend the regex to support both formats.

---

## 8. Acceptance Criteria

- [ ] `"slack"` is a valid provider type in both backend and frontend
- [ ] Feature flag `feature.notifications.service.slack.enabled` gates dispatch (default: enabled)
- [ ] Slack webhook URL is stored in `Token` field, never exposed in GET responses
- [ ] `HasToken` is `true` when webhook URL is set
- [ ] Create / Update / Delete Slack providers works via API and UI
- [ ] Test notification dispatches successfully to a Slack webhook
- [ ] Minimal template auto-normalizes `message` → `text` for Slack payloads
- [ ] All existing notification tests continue to pass (zero regressions)
- [ ] New Slack-specific unit tests pass (`go test ./backend/...`)
- [ ] Frontend unit tests pass (`npx vitest run`)
- [ ] E2E tests pass on Firefox (`--project=firefox`)
- [ ] `make lint-fast` (staticcheck) passes
- [ ] Documentation updated (`notifications.md`, `CHANGELOG.md`)

# Email Notifications / SMTP — Specification & Implementation Plan

> **Status:** Draft
> **Created:** 2026-02-27
> **Scope:** Full analysis of the email notification subsystem, feature flag gaps, Shoutrrr migration residue, email templates, and implementation roadmap.

---

## 1. Executive Summary

Charon's email/SMTP functionality is split across **two independent subsystems** that have never been unified:

| Subsystem | File | Transport | Purpose | Integrated with Notification Providers? |
|-----------|------|-----------|---------|----------------------------------------|
| **MailService** | `backend/internal/services/mail_service.go` | Direct `net/smtp` | User invite emails, test emails | **No** |
| **NotificationService** | `backend/internal/services/notification_service.go` | Custom `HTTPWrapper` | Discord, Gotify, Webhook dispatch | **No email support** |

**Key findings:**

1. **Shoutrrr is fully removed** — no dependency in `go.mod`, no runtime code. Only test-output artifacts (`backend/test-output.txt`) and one archived test file reference it.
2. **No email feature flag exists** — the 9 registered feature flags cover cerberus, uptime, crowdsec, and notify engine/services (discord, gotify, webhook, security events, legacy fallback). Email is absent.
3. **Email is not a notification provider type** — `isSupportedNotificationProviderType()` returns `true` only for `discord`, `gotify`, `webhook`. The frontend `notifications.ts` API client also has no email type.
4. **`NotificationConfig.EmailRecipients`** field exists in the model but is never used at runtime — it's set to `""` in default config and has an archived handler test (`security_notifications_test.go.archived`).
5. **The frontend shows only 3 feature toggles** (Cerberus, CrowdSec Console, Uptime) — notification flags exist in the backend but are not surfaced in the UI toggle grid.

**Conclusion:** To enable "Email Notifications" as a first-class feature, email must be integrated into the notification provider system OR exposed as a standalone feature-flagged service. The MailService itself is production-quality with proper security hardening.

---

## 2. Current State Analysis

### 2.1 MailService (`backend/internal/services/mail_service.go`)

**Lines:** ~650 | **Status:** Production-ready | **Test coverage:** Extensive

**Capabilities:**
- SMTP connection with SSL/TLS, STARTTLS, and plaintext
- Email header injection protection (CWE-93, CodeQL `go/email-injection`)
- MIME Q-encoding for subjects (RFC 2047)
- RFC 5321 dot-stuffing for body sanitization
- Undisclosed recipients pattern (prevents request-derived addresses in headers)
- `net/mail.Address` parsing for all address fields

**Public API:**

| Method | Signature | Purpose |
|--------|-----------|---------|
| `GetSMTPConfig` | `() (*SMTPConfig, error)` | Read config from `settings` table (category=`smtp`) |
| `SaveSMTPConfig` | `(config *SMTPConfig) error` | Persist config to DB |
| `IsConfigured` | `() bool` | Check if host + from_address are set |
| `TestConnection` | `() error` | Validate SMTP connectivity + auth |
| `SendEmail` | `(to, subject, htmlBody string) error` | Send a single email |
| `SendInvite` | `(email, inviteToken, appName, baseURL string) error` | Send invite email with HTML template |

**Config storage:** Uses `models.Setting` with `category = "smtp"` and keys: `smtp_host`, `smtp_port`, `smtp_username`, `smtp_password`, `smtp_from_address`, `smtp_encryption`.

**Consumers:**
- `UserHandler.InviteUser` — sends invite emails asynchronously via `go func()` when SMTP is configured and public URL is set.
- `SettingsHandler.SendTestEmail` — sends a hardcoded test email HTML body.

### 2.2 NotificationService (`backend/internal/services/notification_service.go`)

**Lines:** ~500 | **Status:** Active (notify-only runtime) | **Supported types:** discord, gotify, webhook

**Key dispatch functions:**
- `SendExternal()` — iterates enabled providers, filters by event type preferences, dispatches via `sendJSONPayload()`.
- `sendJSONPayload()` — renders Go templates, sends HTTP POST via `notifications.HTTPWrapper`.
- `isDispatchEnabled()` — checks feature flags per provider type. Discord always true; gotify/webhook gated by flags.
- `isSupportedNotificationProviderType()` — returns `true` for discord, gotify, webhook only.

**Legacy Shoutrrr path:** `legacySendFunc` variable exists but is hardcoded to return `ErrLegacyFallbackDisabled`. All Shoutrrr test names (`TestSendExternal_ShoutrrrPath`, etc.) remain in test files but test the legacy-disabled path.

### 2.3 SecurityNotificationService & EnhancedSecurityNotificationService

- `SecurityNotificationService` — dispatches security events (WAF blocks, ACL denies) to webhook URL from `NotificationConfig`.
- `EnhancedSecurityNotificationService` — provider-based security notifications with compatibility layer. Aggregates settings from `NotificationProvider` records. Filters by supported types: webhook, discord, slack, gotify.
- `NotificationConfig.EmailRecipients` — field exists in model, set to `""` in defaults, never consumed by any dispatch logic.

### 2.4 Frontend SMTP UI

**Page:** `frontend/src/pages/SMTPSettings.tsx` (~300 lines)
- Full CRUD form: host, port, username, password (masked), from_address, encryption (starttls/ssl/none)
- Status indicator card (configured/not configured)
- Test email card (visible only when configured)
- TanStack Query for data fetching, mutations for save/test/send
- Proper form labels with `htmlFor` attributes, accessible select component

**API Client:** `frontend/src/api/smtp.ts`
- `getSMTPConfig()`, `updateSMTPConfig()`, `testSMTPConnection()`, `sendTestEmail()`

**Routes (backend):**
- `GET /api/v1/settings/smtp` → `GetSMTPConfig`
- `POST /api/v1/settings/smtp` → `UpdateSMTPConfig`
- `POST /api/v1/settings/smtp/test` → `TestSMTPConfig`
- `POST /api/v1/settings/smtp/test-email` → `SendTestEmail`

### 2.5 E2E Test Coverage

**File:** `tests/settings/smtp-settings.spec.ts`
- Page load & display (URL, heading, no error alerts)
- Form display (all 6 fields + 2 buttons verified)
- Loading skeleton behavior
- Form validation (required host, numeric port, from address format)
- Encryption selector options
- SMTP save flow (API interception, form fill, toast)
- Test connection flow
- Test email flow
- Status indicator (configured/not-configured)
- Accessibility (ARIA, keyboard navigation)

**Fixtures:** `tests/fixtures/settings.ts`
- `SMTPConfig` interface, `validSMTPConfig`, `validSMTPConfigSSL`, `validSMTPConfigNoAuth`
- `invalidSMTPConfigs` (missingHost, invalidPort, portTooHigh, invalidEmail, etc.)
- `FeatureFlags` interface — only has `cerberus_enabled`, `crowdsec_console_enrollment`, `uptime_monitoring`

---

## 3. Feature Flag Analysis

### 3.1 Backend Feature Flags (Complete Inventory)

| Flag Key | Default | Constant | Used By |
|----------|---------|----------|---------|
| `feature.cerberus.enabled` | `false` | (inline) | Cerberus middleware |
| `feature.uptime.enabled` | `true` | (inline) | Uptime monitoring |
| `feature.crowdsec.console_enrollment` | `false` | (inline) | CrowdSec console |
| `feature.notifications.engine.notify_v1.enabled` | `false` | `FlagNotifyEngineEnabled` | Notification router |
| `feature.notifications.service.discord.enabled` | `false` | `FlagDiscordServiceEnabled` | Discord dispatch gate |
| `feature.notifications.service.gotify.enabled` | `false` | `FlagGotifyServiceEnabled` | Gotify dispatch gate |
| `feature.notifications.service.webhook.enabled` | `false` | `FlagWebhookServiceEnabled` | Webhook dispatch gate |
| `feature.notifications.legacy.fallback_enabled` | `false` | (inline) | **Permanently retired** |
| `feature.notifications.security_provider_events.enabled` | `false` | `FlagSecurityProviderEventsEnabled` | Security event dispatch |

### 3.2 Missing Email Flag

**No email/SMTP feature flag constant exists** in:
- `backend/internal/notifications/feature_flags.go` — 5 constants, none for email
- `feature_flags_handler.go` `defaultFlags` — 9 entries, none for email
- `defaultFlagValues` — 9 entries, none for email

**Required new flag:**
- Key: `feature.notifications.service.email.enabled`
- Default: `false`
- Constant: `FlagEmailServiceEnabled` in `feature_flags.go`

### 3.3 Frontend Feature Flag Gaps

**`SystemSettings.tsx` `featureToggles` array:**
Only 3 toggles are rendered in the UI:
1. `feature.cerberus.enabled`
2. `feature.crowdsec.console_enrollment`
3. `feature.uptime.enabled`

**Missing from UI:** All 6 notification flags are invisible to users. They can only be toggled via API (`PUT /api/v1/feature-flags`).

**`tests/fixtures/settings.ts` `FeatureFlags` interface:**
Only 3 fields — must be expanded to include notification flags when surfacing them in the UI.

---

## 4. SMTP / Notify Migration Gap Analysis

### 4.1 Shoutrrr Removal Status

| Layer | Status | Evidence |
|-------|--------|----------|
| `go.mod` | **Removed** | No `containrrr/shoutrrr` entry |
| Runtime code | **Removed** | `legacySendFunc` returns `ErrLegacyFallbackDisabled` |
| Feature flag | **Retired** | `feature.notifications.legacy.fallback_enabled` permanently `false` |
| Router | **Disabled** | `ShouldUseLegacyFallback()` always returns `false` |
| Test names | **Residual** | `TestSendExternal_ShoutrrrPath`, `TestSendExternal_ShoutrrrError`, etc. in `notification_service_test.go` |
| Test output | **Residual** | `backend/test-output.txt` references Shoutrrr test runs |
| Archived tests | **Residual** | `security_notifications_test.go.archived` has `normalizeEmailRecipients` tests |

### 4.2 Email-as-Notification-Provider Gap

Email is **not** integrated as a notification provider. The architecture gap:

```
Current: NotificationService → HTTPWrapper → Discord/Gotify/Webhook (HTTP POST)
         MailService → net/smtp → SMTP Server (independent, invite-only)

Desired: NotificationService → [Engine] → Discord/Gotify/Webhook (HTTP)
                                        → Email/SMTP (via MailService)
```

**Integration points needed:**
1. Add `"email"` to `isSupportedNotificationProviderType()`
2. Add `"email"` case to `isDispatchEnabled()` with flag check
3. Add email dispatch path in `SendExternal()` that calls `MailService.SendEmail()`
4. Add `FlagEmailServiceEnabled` constant
5. Add flag to `defaultFlags` and `defaultFlagValues`
6. Update `NotificationProvider` model validation and frontend types

### 4.3 `NotificationConfig.EmailRecipients` — Orphaned Field

- Defined in `notification_config.go` line 21: `EmailRecipients string \`json:"email_recipients"\``
- Set to `""` in `SecurityNotificationService.GetSettings()` default config
- `normalizeEmailRecipients()` function existed but is now in `.archived` test file
- **Not used by any active handler, API endpoint, or dispatch logic**

---

## 5. Email Templates Assessment

### 5.1 Current Templates

| Template | Location | Format | Used By |
|----------|----------|--------|---------|
| **Invite Email** | `mail_service.go:SendInvite()` line 595-620 | Inline Go `html/template` | `UserHandler.InviteUser` |
| **Test Email** | `settings_handler.go:SendTestEmail()` line 648-660 | Inline HTML string | Admin SMTP test |

**Invite template features:**
- Gradient header with app name
- "Accept Invitation" CTA button with invite URL
- 48-hour expiration notice
- Fallback plain-text URL link
- Uses `{{.AppName}}` and `{{.InviteURL}}` variables

**Test template features:**
- Simple confirmation message
- Inline CSS styles
- No template variables (hardcoded content)

### 5.2 No Shared Template System

Email templates are entirely separate from the notification template system (`NotificationTemplate` model). Notification templates are JSON-based for HTTP webhook payloads; email templates are HTML strings. There is no shared abstraction.

### 5.3 Missing Templates

For a full email notification feature, these templates would be needed:
- **Security alert email** — WAF blocks, ACL denies, rate limit hits
- **SSL certificate email** — renewal, expiry warnings, failures
- **Uptime alert email** — host up/down transitions
- **System event email** — config changes, backup completions
- **Password reset email** (if user management expands)

---

## 6. Obsolete Code Inventory

### 6.1 Dead / Effectively Dead Code

| Item | Location | Status | Action |
|------|----------|--------|--------|
| `legacySendFunc` variable | `notification_service.go` | Returns `ErrLegacyFallbackDisabled` always | Remove |
| `legacyFallbackInvocationError()` | `notification_service.go:61` | Only called by dead legacy path | Remove |
| `ErrLegacyFallbackDisabled` error | `notification_service.go` | Guard for retired path | Remove |
| `ShouldUseLegacyFallback()` | `router.go` | Always returns `false` | Remove |
| `EngineLegacy` constant | `engine.go` | Unused | Remove |
| `feature.notifications.legacy.fallback_enabled` flag | `feature_flags_handler.go:36` | Permanently retired | Remove from `defaultFlags` |
| `retiredLegacyFallbackEnvAliases` | `feature_flags_handler.go` | Env aliases for retired flag | Remove |
| Legacy test helpers | `notification_service_test.go` | Tests overriding `legacySendFunc` | Refactor/remove |
| `security_notifications_test.go.archived` | `handlers/` | Archived test file | Delete |

### 6.2 Root-Level Artifacts (Should Be Gitignored)

| File | Purpose | Action |
|------|---------|--------|
| `FIREFOX_E2E_FIXES_SUMMARY.md` | One-time fix summary | Move to `docs/implementation/` or gitignore |
| `verify-security-state-for-ui-tests` | Empty file (0 bytes) | Delete or gitignore |
| `categories.txt` | Unknown (28 bytes) | Investigate, likely deletable |
| `codeql-results-*.sarif` | Already gitignored pattern but files exist | Delete tracked files |
| `grype-results.json`, `grype-results.sarif` | Scan artifacts | Already gitignored, delete tracked |
| `sbom-generated.json`, `sbom.cyclonedx.json` | SBOM artifacts | Already gitignored, delete tracked |
| `trivy-*.json` | Scan reports | Already gitignored pattern but tracked |
| `vuln-results.json` | Vulnerability scan | Gitignore and delete |
| `backend/test-output.txt` | Test run output | Gitignore |

### 6.3 Codecov / Dockerignore Gaps

**`codecov.yml`:** No SMTP/email/notification-specific exclusions or flags needed currently. Coverage threshold is 87%.

**`.dockerignore`:** Includes `*.sarif`, `sbom*.json`, `CODEQL_EMAIL_INJECTION_REMEDIATION_COMPLETE.md`. No SMTP-specific additions needed. The existing patterns are sufficient.

**`.gitignore`:** Missing patterns for:
- `FIREFOX_E2E_FIXES_SUMMARY.md`
- `verify-security-state-for-ui-tests`
- `categories.txt`
- `backend/test-output.txt`
- `backend/*.out` (partially covered but `backend_full.out` may leak through)

---

## 7. Implementation Plan

### Phase 1: Cleanup — Dead Code & Artifacts (Low Risk)

**Goal:** Remove Shoutrrr residue, delete obsolete artifacts, fix gitignore gaps.

| Task | File(s) | Complexity |
|------|---------|------------|
| 1.1 Remove `legacySendFunc`, `ErrLegacyFallbackDisabled`, `legacyFallbackInvocationError()` | `notification_service.go` | S |
| 1.2 Remove `ShouldUseLegacyFallback()` | `router.go` | S |
| 1.3 Remove `EngineLegacy` constant | `engine.go` | S |
| 1.4 Remove `feature.notifications.legacy.fallback_enabled` from `defaultFlags` and `defaultFlagValues` | `feature_flags_handler.go` | S |
| 1.5 Remove `retiredLegacyFallbackEnvAliases` | `feature_flags_handler.go` | S |
| 1.6 Refactor legacy test helpers in `notification_service_test.go` and `notification_service_json_test.go` | Test files | M |
| 1.7 Delete `security_notifications_test.go.archived` | Handlers dir | S |
| 1.8 Add missing `.gitignore` patterns and delete tracked artifacts from root | `.gitignore`, root files | S |
| 1.9 Move `FIREFOX_E2E_FIXES_SUMMARY.md` to `docs/implementation/` | Root → docs/ | S |

### Phase 2: Email Feature Flag (Medium Risk)

**Goal:** Register email as a feature-flagged notification service.

| Task | File(s) | Complexity |
|------|---------|------------|
| 2.1 Add `FlagEmailServiceEnabled` constant | `notifications/feature_flags.go` | S |
| 2.2 Add `feature.notifications.service.email.enabled` to `defaultFlags` + `defaultFlagValues` (default: `false`) | `feature_flags_handler.go` | S |
| 2.3 Add `"email"` case to `isSupportedNotificationProviderType()` | `notification_service.go` | S |
| 2.4 Add `"email"` case to `isDispatchEnabled()` with flag check | `notification_service.go` | S |
| 2.5 Update frontend `FeatureFlags` interface and test fixtures | `tests/fixtures/settings.ts` | S |
| 2.6 (Optional) Surface notification flags in `SystemSettings.tsx` `featureToggles` array | `SystemSettings.tsx` | M |
| 2.7 Unit tests for new flag constant, dispatch enable check, provider type check | Test files | M |

### Phase 3: Email Notification Provider Integration (High Risk)

**Goal:** Wire `MailService` as a notification dispatch target alongside HTTP providers.

| Task | File(s) | Complexity |
|------|---------|------------|
| 3.1 Add `MailService` dependency to `NotificationService` | `notification_service.go` | M |
| 3.2 Implement email dispatch branch in `SendExternal()` | `notification_service.go` | L |
| 3.3 Define email notification template rendering (subject + HTML body from event context) | New or extend `mail_service.go` | L |
| 3.4 Add email provider type to frontend `notifications.ts` API client | `frontend/src/api/notifications.ts` | S |
| 3.5 Update notification provider UI to support email configuration (recipients, template selection) | Frontend components | L |
| 3.6 Update `NotificationConfig.EmailRecipients` usage in `SecurityNotificationService` dispatch | `security_notification_service.go` | M |
| 3.7 Integration tests for email dispatch path | Test files | L |
| 3.8 E2E tests for email notification provider CRUD | `tests/settings/` | L |

### Phase 4: Email Templates (Medium Risk)

**Goal:** Create HTML email templates for all notification event types.

| Task | File(s) | Complexity |
|------|---------|------------|
| 4.1 Create reusable base email template with Charon branding | `backend/internal/services/` or `templates/` | M |
| 4.2 Implement security alert email template | Template file | M |
| 4.3 Implement SSL certificate event email template | Template file | M |
| 4.4 Implement uptime event email template | Template file | M |
| 4.5 Unit tests for all templates (variable rendering, sanitization) | Test files | M |

### Phase 5: Documentation & E2E Validation

| Task | File(s) | Complexity |
|------|---------|------------|
| 5.1 Update `docs/features/notifications.md` to include Email provider | `docs/features/notifications.md` | S |
| 5.2 Add Email row to supported services table | `docs/features/notifications.md` | S |
| 5.3 E2E tests for feature flag toggle affecting email dispatch | `tests/` | M |
| 5.4 Full regression E2E run across all notification types | Playwright suites | M |

---

## 8. Test Plan

### 8.1 Backend Unit Tests

| Area | Test Cases | Priority |
|------|-----------|----------|
| `FlagEmailServiceEnabled` constant | Verify string value matches convention | P0 |
| `isSupportedNotificationProviderType("email")` | Returns `true` | P0 |
| `isDispatchEnabled("email")` | Returns flag-gated value | P0 |
| `SendExternal` with email provider | Dispatches to MailService.SendEmail | P0 |
| `SendExternal` with email flag disabled | Skips email provider | P0 |
| Email template rendering | All variables rendered, XSS-safe | P1 |
| `normalizeEmailRecipients` (if resurrected) | Valid/invalid email lists | P1 |
| Legacy code removal | Verify `legacySendFunc` references removed | P0 |

### 8.2 Frontend Unit Tests

| Area | Test Cases | Priority |
|------|-----------|----------|
| Feature flag toggle for email | Renders in SystemSettings when flag exists | P1 |
| Notification provider types | Includes "email" in supported types | P1 |
| Email provider form | Renders recipient fields, validates emails | P2 |

### 8.3 E2E Tests (Playwright)

| Test | File | Priority |
|------|------|----------|
| SMTP settings CRUD (existing) | `smtp-settings.spec.ts` | P0 (already covered) |
| Email feature flag toggle | New spec or extend `system-settings.spec.ts` | P1 |
| Email notification provider CRUD | New spec in `tests/settings/` | P1 |
| Email notification test send | Extend `notifications.spec.ts` | P2 |
| Feature flag affects email dispatch | Integration-level E2E | P2 |

### 8.4 Existing Test Coverage Gaps

| Gap | Impact | Priority |
|-----|--------|----------|
| SMTP test email E2E doesn't test actual send (mocked) | Low — backend unit tests cover MailService | P2 |
| No E2E test for invite email flow | Medium — relies on SMTP + public URL | P1 |
| Notification E2E tests are Discord-only | High — gotify/webhook have no E2E | P1 |
| Feature flag toggles not E2E-tested for notification flags | Medium — backend flags work, UI doesn't show them | P1 |

---

## 9. Implementation Strategy

### Decision: **Single PR, Staged Commits**

The full email notification feature lands in one PR on `feature/beta-release`. Work is organized into discrete commit stages that can be reviewed independently on the branch, cherry-picked if needed, and clearly bisected if a regression is introduced.

**Why single PR:**
- The feature is self-contained and the flag defaults to `false` — no behavioral change lands until the flag is toggled
- All four stages are tightly coupled (dead code removal creates the clean baseline the flag registration depends on, which the integration depends on, which templates depend on)
- A single PR keeps the review diff contiguous and avoids merge-order coordination overhead

---

### Commit Stage 1: `chore: remove Shoutrrr residue and dead notification legacy code`

**Goal:** Clean codebase baseline. No behavior changes.

**Files:**
- `backend/internal/services/notification_service.go` — remove `legacySendFunc`, `ErrLegacyFallbackDisabled`, `legacyFallbackInvocationError()`
- `backend/internal/notifications/router.go` — remove `ShouldUseLegacyFallback()`, update `ShouldUseNotify()` to remove `EngineLegacy` reference
- `backend/internal/notifications/engine.go` — remove `EngineLegacy` constant
- `backend/internal/api/handlers/feature_flags_handler.go` — remove `feature.notifications.legacy.fallback_enabled` from `defaultFlags` + `defaultFlagValues`, remove `retiredLegacyFallbackEnvAliases`
- `backend/internal/services/notification_service_test.go` — refactor/remove legacy `legacySendFunc` override test helpers
- `backend/internal/services/notification_service_json_test.go` — remove legacy path overrides
- `backend/internal/api/handlers/security_notifications_test.go.archived` — delete file
- `backend/internal/models/notification_config.go` — remove orphaned `EmailRecipients` field
- `.gitignore` — add missing patterns (root artifacts, `FIREFOX_E2E_FIXES_SUMMARY.md`, `verify-security-state-for-ui-tests`, `categories.txt`, `backend/test-output.txt`, `backend/*.out`)
- Root-level tracked scan artifacts — delete (`codeql-results-*.sarif`, `grype-results.*`, `sbom-generated.json`, `sbom.cyclonedx.json`, `trivy-*.json`, `vuln-results.json`, `backend/test-output.txt`, `verify-security-state-for-ui-tests`, `categories.txt`)
- `FIREFOX_E2E_FIXES_SUMMARY.md` → `docs/implementation/FIREFOX_E2E_FIXES_SUMMARY.md`
- `ARCHITECTURE.instructions.md` tech stack table — update "Notifications" row from `Shoutrrr` to `Notify`

**Validation gate:** `go test ./...` green, no compilation errors, `npx playwright test` regression-clean.

---

### Commit Stage 2: `feat: register email as feature-flagged notification service`

**Goal:** Email flag exists in the system; defaults to `false`; no dispatch wiring yet.

**Files:**
- `backend/internal/notifications/feature_flags.go` — add `FlagEmailServiceEnabled = "feature.notifications.service.email.enabled"`
- `backend/internal/api/handlers/feature_flags_handler.go` — add flag to `defaultFlags` + `defaultFlagValues` (default: `false`)
- `backend/internal/services/notification_service.go` — add `"email"` to `isSupportedNotificationProviderType()` and `isDispatchEnabled()` (gated by `FlagEmailServiceEnabled`)
- `backend/internal/services/notification_service_test.go` — add unit tests for new flag constant, `isSupportedNotificationProviderType("email")` returns `true`, `isDispatchEnabled("email")` respects flag
- `tests/fixtures/settings.ts` — expand `FeatureFlags` interface to include `feature.notifications.service.email.enabled`

**Validation gate:** `go test ./...` green, feature flag API returns new key with `false` default, E2E fixture types compile.

---

### Commit Stage 3: `feat: wire MailService into notification dispatch pipeline`

**Goal:** Email works as a first-class notification provider. `SendEmail()` accepts a context. Dispatch is async, guarded, and timeout-bound.

**Files:**
- `backend/internal/services/mail_service.go`
  - Refactor `SendEmail(to, subject, htmlBody string)` → `SendEmail(ctx context.Context, to []string, subject, htmlBody string) error` (multi-recipient, context-aware)
  - Add recipient validation function: RFC 5322 format, max 20 recipients, `\r\n` header injection rejection
- `backend/internal/services/notification_service.go`
  - Add `mailService MailServiceInterface` dependency (interface for testability)
  - Add `dispatchEmail(ctx context.Context, provider NotificationProvider, eventType, title, message string)` — resolves recipients from provider config, calls `s.mailService.IsConfigured()` guard (warn + return if not), renders HTML, calls `SendEmail()` with 30s timeout context
  - In `SendExternal()`: before `sendJSONPayload`, add email branch: `if providerType == "email" { go s.dispatchEmail(...); continue }`
  - `supportsJSONTemplates()` — do NOT include `"email"` (email uses its own rendering path)
- `backend/internal/models/notification_provider.go` — document how email provider config is stored: `Type = "email"`, `URL` field repurposed as comma-separated recipient list (no schema migration needed; backward-compatible)
- `frontend/src/api/notifications.ts` — add `"email"` to supported provider type union
- Frontend notification provider form component — add recipient field (comma-separated emails) when type is `"email"`, hide URL/token fields
- Backend unit tests:
  - `dispatchEmail` with SMTP unconfigured → logs warning, no error
  - `dispatchEmail` with empty recipient list → graceful skip
  - `dispatchEmail` with invalid recipient `"not-an-email"` → validation rejects
  - `dispatchEmail` concurrent calls → goroutine-safe
  - `SendExternal` with email provider → calls `dispatchEmail`, not `sendJSONPayload`
  - `SendExternal` with email flag disabled → skips email provider
  - Template rendering with XSS-payload event data → sanitized output
- E2E tests:
  - Email provider CRUD (create, edit, delete)
  - Email provider with email flag disabled — provider exists but dispatch is skipped
  - Test send triggers correct API call with proper payload

**Validation gate:** `go test ./...` green, GORM security scan clean, email provider can be created and dispatches correctly when flag is enabled and SMTP is configured.

---

### Commit Stage 4: `feat: add HTML email templates for notification event types`

**Goal:** Each notification event type produces a properly branded, XSS-safe HTML email.

**Files:**
- `backend/internal/services/templates/` (new directory using `embed.FS`)
  - `email_base.html` — Charon-branded base layout (gradient header, footer, responsive)
  - `email_security_alert.html` — WAF blocks, ACL denies, rate limit hits
  - `email_ssl_event.html` — certificate renewal, expiry warnings, failures
  - `email_uptime_event.html` — host up/down transitions
  - `email_system_event.html` — config changes, backup completions
- `backend/internal/services/mail_service.go` — add `RenderNotificationEmail(templateName string, data interface{}) (string, error)` using `embed.FS` + `html/template`
- Backend unit tests:
  - Each template renders correctly with valid data
  - Templates with malicious input (XSS) produce sanitized output (html/template auto-escapes)
  - Missing template name returns descriptive error
- `docs/features/notifications.md` — add Email provider section with configuration guide
- E2E tests:
  - Email notification content verification (intercept API, verify rendered body structure)
  - Feature flag toggle E2E — enabling/disabling `feature.notifications.service.email.enabled` flag from UI

**Validation gate:** All templates render safely, docs updated, full Playwright suite regression-clean.

---

### Cross-Stage Notes

- **Rate limiting for email notifications** is deferred to a follow-up issue. For v1, fire-and-forget with the 30s timeout context is acceptable. A digest/cooldown mechanism will be tracked as a tech debt item.
- **Per-user recipient preferences** are out of scope. Recipients are configured globally per provider record.
- **Email queue/retry** is deferred. Transient SMTP failures log a warning and drop the notification (same behavior as webhook failures).

---

## 10. Risk Assessment & Recommendations

### 10.1 Risks

| Risk | Severity | Likelihood | Mitigation |
|------|----------|------------|------------|
| Email dispatch in hot path blocks event processing | High | Medium | Dispatch async (goroutine), same pattern as `InviteUser` |
| SMTP credentials exposed in logs | Critical | Low | Already masked (`MaskPassword`), ensure new paths follow same pattern |
| Email injection via notification event data | High | Low | MailService already has CWE-93 protection; ensure template variables are sanitized |
| Feature flag toggle enables email before SMTP is configured | Medium | Medium | Check `MailService.IsConfigured()` before dispatch, log warning if not |
| Legacy Shoutrrr test removal breaks coverage | Low | Low | Replace with notify-path equivalents before removing |

### 10.2 `.gitignore` Recommendations

Add the following patterns:
```gitignore
# One-off summary files
FIREFOX_E2E_FIXES_SUMMARY.md

# Empty marker files
verify-security-state-for-ui-tests

# Misc artifacts
categories.txt

# Backend test output
backend/test-output.txt
backend/*.out
```

### 10.3 `codecov.yml` Recommendations

No changes needed. The 87% target is appropriate. Email-related code is in `services/` which is already covered.

### 10.4 `.dockerignore` Recommendations

No changes needed. Existing patterns cover scan artifacts, SARIF files, and SBOM outputs.

### 10.5 `Dockerfile` Recommendations

No SMTP-specific changes needed. The Dockerfile does not need to expose SMTP ports — Charon is an SMTP **client**, not server. The existing single-container architecture with Go backend + React frontend remains appropriate.

---

## Appendix A: File Reference Map

| File | Lines Read | Key Findings |
|------|-----------|--------------|
| `backend/go.mod` | Full | No shoutrrr, no nikoksr/notify |
| `backend/internal/services/mail_service.go` | 1-650 | Complete MailService with security hardening |
| `backend/internal/services/notification_service.go` | 1-600 | Custom notify system, no email type |
| `backend/internal/services/security_notification_service.go` | 1-120 | Webhook-only dispatch, EmailRecipients unused |
| `backend/internal/services/enhanced_security_notification_service.go` | 1-180 | Provider aggregation, no email support |
| `backend/internal/notifications/feature_flags.go` | 1-10 | 5 constants, no email |
| `backend/internal/notifications/engine.go` | Grep | EngineLegacy dead |
| `backend/internal/notifications/router.go` | Grep | Legacy fallback disabled |
| `backend/internal/notifications/http_wrapper.go` | 1-100 | SSRF-protected HTTP client |
| `backend/internal/api/handlers/feature_flags_handler.go` | 1-200 | 9 flags, no email |
| `backend/internal/api/handlers/settings_handler.go` | 520-700 | SMTP CRUD + test email handlers |
| `backend/internal/api/handlers/user_handler.go` | 466-650 | Invite email flow |
| `backend/internal/models/notification_config.go` | 1-50 | EmailRecipients field present but unused |
| `backend/internal/models/notification_provider.go` | Grep | No email type |
| `frontend/src/api/smtp.ts` | Full | Complete SMTP API client |
| `frontend/src/api/featureFlags.ts` | Full | Generic get/update, no email flag |
| `frontend/src/api/notifications.ts` | 1-100 | Discord/gotify/webhook only |
| `frontend/src/pages/SMTPSettings.tsx` | Full | Complete SMTP settings UI |
| `frontend/src/pages/SystemSettings.tsx` | 185-320 | Only 3 feature toggles shown |
| `tests/fixtures/settings.ts` | Full | FeatureFlags missing notification flags |
| `tests/settings/smtp-settings.spec.ts` | 1-200 | Comprehensive SMTP E2E tests |
| `tests/settings/notifications.spec.ts` | 1-50 | Discord-focused E2E tests |
| `docs/features/notifications.md` | Full | No email provider documented |
| `codecov.yml` | Full | 87% target, no SMTP exclusions |
| `.gitignore` | Grep | Missing patterns for root artifacts |
| `.dockerignore` | Grep | Adequate coverage |

## Appendix B: Research Questions Answered

**Q1: Is Shoutrrr fully removed?**
Yes. No dependency in `go.mod`, no runtime code path. Only residual test names and one archived test file remain.

**Q2: Does an email feature flag exist?**
No. Must be created as `feature.notifications.service.email.enabled`.

**Q3: Is MailService integrated with NotificationService?**
No. They are completely independent. MailService is only consumed by UserHandler (invites) and SettingsHandler (test email).

**Q4: What is the state of `NotificationConfig.EmailRecipients`?**
Orphaned. Defined in model, set to empty string in defaults, never consumed by any dispatch logic. The handler function `normalizeEmailRecipients()` is archived.

**Q5: What frontend gaps exist for email notifications?**
- `FeatureFlags` interface missing notification flags
- `SystemSettings.tsx` only shows 3 of 9 feature toggles
- `notifications.ts` API client has no email provider type
- No email-specific notification provider UI components

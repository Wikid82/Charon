# Phase 4: Settings E2E Test Implementation Plan

**Date:** January 19, 2026
**Status:** Planning Complete
**Estimated Effort:** 5 days (Week 8 per main plan)
**Dependencies:** Phase 1-3 complete (346+ tests passing)

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Research Findings](#2-research-findings)
3. [Test File Specifications](#3-test-file-specifications)
4. [Test Data Fixtures Required](#4-test-data-fixtures-required)
5. [Implementation Order](#5-implementation-order)
6. [Risks and Blockers](#6-risks-and-blockers)
7. [Success Metrics](#7-success-metrics)

---

## 1. Executive Summary

### Scope

Phase 4 covers E2E testing for all Settings-related functionality:

| Area | Frontend Page | API Endpoints | Est. Tests |
|------|---------------|---------------|------------|
| System Settings | `SystemSettings.tsx` | `/settings`, `/settings/validate-url`, `/settings/test-url` | 25 |
| SMTP Settings | `SMTPSettings.tsx` | `/settings/smtp`, `/settings/smtp/test`, `/settings/smtp/test-email` | 18 |
| Notifications | `Notifications.tsx` | `/notifications/providers/*`, `/notifications/templates/*`, `/notifications/external-templates/*` | 22 |
| User Management | `UsersPage.tsx` | `/users/*`, `/users/invite`, `/invite/*` | 28 |
| Encryption Management | `EncryptionManagement.tsx` | `/admin/encryption/*` | 15 |
| Account Settings | `Account.tsx` | `/auth/profile`, `/auth/password`, `/settings` | 20 |

**Total Estimated Tests:** ~128 tests across 6 test files

### Key Findings from Research

1. **Settings Navigation**: Main settings page (`Settings.tsx`) provides tab navigation to 4 sub-routes:
   - `/settings/system` → System Settings
   - `/settings/notifications` → Notifications
   - `/settings/smtp` → SMTP Settings
   - `/settings/account` → Account Settings

2. **Encryption Management** is accessed via a separate route (likely `/encryption` or admin panel)

3. **User Management** is accessed via `/users` route, not part of settings tabs

4. **All settings use React Query** for data fetching with standardized patterns

5. **Form validation** is primarily client-side with some server-side validation

---

## 2. Research Findings

### 2.1 System Settings (`SystemSettings.tsx`)

**Route:** `/settings/system`

**UI Components:**
- Feature Toggles (Cerberus, CrowdSec Console Enrollment, Uptime Monitoring)
- General Configuration (Caddy Admin API, SSL Provider, Domain Link Behavior, Language)
- Application URL (with validation and connectivity test)
- System Health Status Card
- Update Checker
- WebSocket Status Card

**Form Fields:**
| Field | Input Type | ID/Selector | Validation |
|-------|------------|-------------|------------|
| Caddy Admin API | text | `#caddy-api` | URL format |
| SSL Provider | select | `#ssl-provider` | Enum: auto, letsencrypt-staging, letsencrypt-prod, zerossl |
| Domain Link Behavior | select | `#domain-behavior` | Enum: same_tab, new_tab, new_window |
| Public URL | text | input with validation icon | URL format, reachability test |
| Feature Toggles | switch | aria-label patterns | Boolean |

**API Endpoints:**
| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/settings` | Fetch all settings |
| POST | `/settings` | Update setting |
| POST | `/settings/validate-url` | Validate public URL format |
| POST | `/settings/test-url` | Test public URL reachability (SSRF-protected) |
| GET | `/health` | System health status |
| GET | `/system/updates` | Check for updates |
| GET | `/feature-flags` | Get feature flags |
| PUT | `/feature-flags` | Update feature flags |

**Key Selectors:**
```typescript
// Feature toggles
page.getByRole('switch').filter({ has: page.getByText(/cerberus|crowdsec|uptime/i) })

// General config
page.locator('#caddy-api')
page.locator('#ssl-provider')
page.locator('#domain-behavior')

// Save button
page.getByRole('button', { name: /save settings/i })

// URL test button
page.getByRole('button', { name: /test/i }).filter({ hasText: /test/i })
```

---

### 2.2 SMTP Settings (`SMTPSettings.tsx`)

**Route:** `/settings/smtp`

**UI Components:**
- SMTP Configuration Card (host, port, username, password, from address, encryption)
- Connection Test Button
- Send Test Email Section

**Form Fields:**
| Field | Input Type | ID/Selector | Validation |
|-------|------------|-------------|------------|
| SMTP Host | text | `#smtp-host` | Required |
| SMTP Port | number | `#smtp-port` | Required, numeric |
| Username | text | `#smtp-username` | Optional |
| Password | password | `#smtp-password` | Optional |
| From Address | email | (needs ID) | Email format |
| Encryption | select | (needs ID) | Enum: none, ssl, starttls |
| Test Email To | email | (needs ID) | Email format |

**API Endpoints:**
| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/settings/smtp` | Get SMTP config |
| POST | `/settings/smtp` | Update SMTP config |
| POST | `/settings/smtp/test` | Test SMTP connection |
| POST | `/settings/smtp/test-email` | Send test email |

**Key Selectors:**
```typescript
page.locator('#smtp-host')
page.locator('#smtp-port')
page.locator('#smtp-username')
page.locator('#smtp-password')
page.getByRole('button', { name: /save/i })
page.getByRole('button', { name: /test connection/i })
page.getByRole('button', { name: /send test email/i })
```

---

### 2.3 Notifications (`Notifications.tsx`)

**Route:** `/settings/notifications`

**UI Components:**
- Provider List with CRUD
- Provider Form Modal (name, type, URL, template, event checkboxes)
- Template Selection (built-in + external/saved)
- Template Form Modal for external templates
- Preview functionality
- Test notification button

**Provider Types:**
- Discord, Slack, Gotify, Telegram, Generic Webhook, Custom Webhook

**Form Fields (Provider):**
| Field | Input Type | Selector | Validation |
|-------|------------|----------|------------|
| Name | text | `input[name="name"]` | Required |
| Type | select | `select[name="type"]` | Required |
| URL | text | `input[name="url"]` | Required, URL format |
| Template | select | `select[name="template"]` | Optional |
| Config (JSON) | textarea | `textarea[name="config"]` | JSON format for custom |
| Enabled | checkbox | `input[name="enabled"]` | Boolean |
| Notify Events | checkboxes | `input[name="notify_*"]` | Boolean flags |

**API Endpoints:**
| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/notifications/providers` | List providers |
| POST | `/notifications/providers` | Create provider |
| PUT | `/notifications/providers/:id` | Update provider |
| DELETE | `/notifications/providers/:id` | Delete provider |
| POST | `/notifications/providers/test` | Test provider |
| POST | `/notifications/providers/preview` | Preview notification |
| GET | `/notifications/templates` | Get built-in templates |
| GET | `/notifications/external-templates` | Get saved templates |
| POST | `/notifications/external-templates` | Create template |
| PUT | `/notifications/external-templates/:id` | Update template |
| DELETE | `/notifications/external-templates/:id` | Delete template |
| POST | `/notifications/external-templates/preview` | Preview template |

**Key Selectors:**
```typescript
// Provider list
page.getByRole('button', { name: /add.*provider/i })
page.getByRole('button', { name: /edit/i })
page.getByRole('button', { name: /delete/i })

// Provider form
page.locator('input[name="name"]')
page.locator('select[name="type"]')
page.locator('input[name="url"]')

// Event checkboxes
page.locator('input[name="notify_proxy_hosts"]')
page.locator('input[name="notify_certs"]')
```

---

### 2.4 User Management (`UsersPage.tsx`)

**Route:** `/users`

**UI Components:**
- User List Table (email, name, role, status, last login, actions)
- Invite User Modal
- Edit Permissions Modal
- Delete Confirmation Dialog
- URL Preview for Invite Links

**Form Fields (Invite):**
| Field | Input Type | Selector | Validation |
|-------|------------|----------|------------|
| Email | email | `input[type="email"]` | Required, email format |
| Role | select | `select` (role) | admin, user |
| Permission Mode | select | `select` (permission_mode) | allow_all, deny_all |
| Permitted Hosts | checkboxes | dynamic | Host selection |

**API Endpoints:**
| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/users` | List users |
| POST | `/users` | Create user |
| GET | `/users/:id` | Get user |
| PUT | `/users/:id` | Update user |
| DELETE | `/users/:id` | Delete user |
| PUT | `/users/:id/permissions` | Update permissions |
| POST | `/users/invite` | Send invite |
| POST | `/users/preview-invite-url` | Preview invite URL |
| GET | `/invite/validate` | Validate invite token |
| POST | `/invite/accept` | Accept invitation |

**Key Selectors:**
```typescript
// User list
page.getByRole('button', { name: /invite.*user/i })
page.getByRole('table')
page.getByRole('row')

// Invite modal
page.getByLabel(/email/i)
page.locator('select').filter({ hasText: /user|admin/i })
page.getByRole('button', { name: /send.*invite/i })

// Actions
page.getByRole('button', { name: /settings|permissions/i })
page.getByRole('button', { name: /delete/i })
```

---

### 2.5 Encryption Management (`EncryptionManagement.tsx`)

**Route:** `/encryption` (or via admin panel)

**UI Components:**
- Status Overview Cards (current version, providers updated, providers outdated, next key status)
- Rotation Confirmation Dialog
- Rotation History Table/List
- Validate Keys Button

**API Endpoints:**
| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/admin/encryption/status` | Get encryption status |
| POST | `/admin/encryption/rotate` | Rotate encryption key |
| GET | `/admin/encryption/history` | Get rotation history |
| POST | `/admin/encryption/validate` | Validate key configuration |

**Key Selectors:**
```typescript
// Status cards
page.getByRole('heading', { name: /current.*version/i })
page.getByRole('heading', { name: /providers.*updated/i })

// Actions
page.getByRole('button', { name: /rotate.*key/i })
page.getByRole('button', { name: /validate/i })

// Confirmation dialog
page.getByRole('dialog')
page.getByRole('button', { name: /confirm/i })
page.getByRole('button', { name: /cancel/i })
```

---

### 2.6 Account Settings (`Account.tsx`)

**Route:** `/settings/account`

**UI Components:**
- Profile Card (name, email)
- Certificate Email Card (use account email checkbox, custom email)
- Password Change Card
- API Key Card (view, copy, regenerate)
- Password Confirmation Modal (for sensitive changes)
- Email Update Confirmation Modal

**Form Fields:**
| Field | Input Type | ID/Selector | Validation |
|-------|------------|-------------|------------|
| Name | text | `#profile-name` | Required |
| Email | email | `#profile-email` | Required, email format |
| Certificate Email | checkbox + email | `#useUserEmail`, `#cert-email` | Email format |
| Current Password | password | `#current-password` | Required for changes |
| New Password | password | `#new-password` | Strength requirements |
| Confirm Password | password | `#confirm-password` | Must match |

**API Endpoints:**
| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/auth/profile` (via `getProfile`) | Get user profile |
| PUT | `/auth/profile` (via `updateProfile`) | Update profile |
| POST | `/auth/regenerate-api-key` | Regenerate API key |
| POST | `/auth/change-password` | Change password |
| GET | `/settings` | Get settings |
| POST | `/settings` | Update settings (caddy.email) |

**Key Selectors:**
```typescript
// Profile
page.locator('#profile-name')
page.locator('#profile-email')
page.getByRole('button', { name: /save.*profile/i })

// Certificate Email
page.locator('#useUserEmail')
page.locator('#cert-email')

// Password
page.locator('#current-password')
page.locator('#new-password')
page.locator('#confirm-password')
page.getByRole('button', { name: /update.*password/i })

// API Key
page.getByRole('button').filter({ has: page.locator('svg.lucide-copy') })
page.getByRole('button').filter({ has: page.locator('svg.lucide-refresh-cw') })
```

---

## 3. Test File Specifications

### 3.1 System Settings (`tests/settings/system-settings.spec.ts`)

**Priority:** P0 (Core functionality)
**Estimated Tests:** 25

```typescript
test.describe('System Settings', () => {
  // Navigation & Page Load (3 tests)
  test('should load system settings page') // P0
  test('should display all setting sections') // P0
  test('should navigate between settings tabs') // P1

  // Feature Toggles (5 tests)
  test('should toggle Cerberus security feature') // P0
  test('should toggle CrowdSec console enrollment') // P0
  test('should toggle uptime monitoring') // P0
  test('should persist feature toggle changes') // P0
  test('should show overlay during feature update') // P1

  // General Configuration (6 tests)
  test('should update Caddy Admin API URL') // P0
  test('should change SSL provider') // P0
  test('should update domain link behavior') // P1
  test('should change language setting') // P1
  test('should validate invalid Caddy API URL') // P1
  test('should save general settings successfully') // P0

  // Application URL (5 tests)
  test('should validate public URL format') // P0
  test('should test public URL reachability') // P0
  test('should show error for unreachable URL') // P1
  test('should show success for reachable URL') // P1
  test('should update public URL setting') // P0

  // System Status (4 tests)
  test('should display system health status') // P0
  test('should show version information') // P1
  test('should check for updates') // P1
  test('should display WebSocket status') // P2

  // Accessibility (2 tests)
  test('should be keyboard navigable') // P1
  test('should have proper ARIA labels') // P1
});
```

---

### 3.2 SMTP Settings (`tests/settings/smtp-settings.spec.ts`)

**Priority:** P0 (Email notifications dependency)
**Estimated Tests:** 18

```typescript
test.describe('SMTP Settings', () => {
  // Page Load & Display (3 tests)
  test('should load SMTP settings page') // P0
  test('should display SMTP configuration form') // P0
  test('should show loading skeleton while fetching') // P2

  // Form Validation (4 tests)
  test('should validate required host field') // P0
  test('should validate port is numeric') // P0
  test('should validate from address format') // P0
  test('should validate encryption selection') // P1

  // CRUD Operations (4 tests)
  test('should save SMTP configuration') // P0
  test('should update existing SMTP configuration') // P0
  test('should clear password field on save') // P1
  test('should preserve masked password on edit') // P1

  // Connection Testing (4 tests)
  test('should test SMTP connection successfully') // P0
  test('should show error on connection failure') // P0
  test('should send test email') // P0
  test('should show error on test email failure') // P1

  // Accessibility (3 tests)
  test('should be keyboard navigable') // P1
  test('should have proper form labels') // P1
  test('should announce errors to screen readers') // P2
});
```

---

### 3.3 Notifications (`tests/settings/notifications.spec.ts`)

**Priority:** P1 (Important but not blocking)
**Estimated Tests:** 22

```typescript
test.describe('Notification Providers', () => {
  // Provider List (4 tests)
  test('should display notification providers list') // P0
  test('should show empty state when no providers') // P1
  test('should display provider type badges') // P2
  test('should filter providers by type') // P2

  // Provider CRUD (8 tests)
  test('should create Discord notification provider') // P0
  test('should create Slack notification provider') // P0
  test('should create generic webhook provider') // P0
  test('should edit existing provider') // P0
  test('should delete provider with confirmation') // P0
  test('should enable/disable provider') // P1
  test('should validate provider URL') // P1
  test('should validate provider name required') // P1

  // Template Management (5 tests)
  test('should select built-in template') // P1
  test('should create custom template') // P1
  test('should preview template with sample data') // P1
  test('should edit external template') // P2
  test('should delete external template') // P2

  // Testing & Preview (3 tests)
  test('should test notification provider') // P0
  test('should show test success feedback') // P1
  test('should preview notification content') // P1

  // Event Selection (2 tests)
  test('should configure notification events') // P1
  test('should persist event selections') // P1
});
```

---

### 3.4 User Management (`tests/settings/user-management.spec.ts`)

**Priority:** P0 (Security critical)
**Estimated Tests:** 28

```typescript
test.describe('User Management', () => {
  // User List (5 tests)
  test('should display user list') // P0
  test('should show user status badges') // P1
  test('should display role badges') // P1
  test('should show last login time') // P2
  test('should show pending invite status') // P1

  // Invite User (8 tests)
  test('should open invite user modal') // P0
  test('should send invite with valid email') // P0
  test('should validate email format') // P0
  test('should select user role') // P0
  test('should configure permission mode') // P0
  test('should select permitted hosts') // P1
  test('should show invite URL preview') // P1
  test('should copy invite link') // P1

  // Permission Management (5 tests)
  test('should open permissions modal') // P0
  test('should update permission mode') // P0
  test('should add permitted hosts') // P0
  test('should remove permitted hosts') // P1
  test('should save permission changes') // P0

  // User Actions (6 tests)
  test('should enable/disable user') // P0
  test('should change user role') // P0
  test('should delete user with confirmation') // P0
  test('should prevent self-deletion') // P0
  test('should prevent deleting last admin') // P0
  test('should resend invite for pending user') // P2

  // Accessibility & Security (4 tests)
  test('should be keyboard navigable') // P1
  test('should require admin role for access') // P0
  test('should show error for regular user access') // P0
  test('should have proper ARIA labels') // P2
});
```

---

### 3.5 Encryption Management (`tests/settings/encryption-management.spec.ts`)

**Priority:** P0 (Security critical)
**Estimated Tests:** 15

```typescript
test.describe('Encryption Management', () => {
  // Status Display (4 tests)
  test('should display encryption status cards') // P0
  test('should show current key version') // P0
  test('should show provider update counts') // P0
  test('should indicate next key configuration status') // P1

  // Key Rotation (6 tests)
  test('should open rotation confirmation dialog') // P0
  test('should cancel rotation from dialog') // P1
  test('should execute key rotation') // P0
  test('should show rotation progress') // P1
  test('should display rotation success message') // P0
  test('should handle rotation failure gracefully') // P0

  // Key Validation (3 tests)
  test('should validate key configuration') // P0
  test('should show validation success message') // P1
  test('should show validation errors') // P1

  // History (2 tests)
  test('should display rotation history') // P1
  test('should show history details') // P2
});
```

---

### 3.6 Account Settings (`tests/settings/account-settings.spec.ts`)

**Priority:** P0 (User-facing critical)
**Estimated Tests:** 20

```typescript
test.describe('Account Settings', () => {
  // Profile Management (5 tests)
  test('should display user profile') // P0
  test('should update profile name') // P0
  test('should update profile email') // P0
  test('should require password for email change') // P0
  test('should show email change confirmation dialog') // P1

  // Certificate Email (4 tests)
  test('should toggle use account email') // P1
  test('should enter custom certificate email') // P1
  test('should validate certificate email format') // P1
  test('should save certificate email') // P1

  // Password Change (5 tests)
  test('should change password with valid inputs') // P0
  test('should validate current password') // P0
  test('should validate password strength') // P0
  test('should validate password confirmation match') // P0
  test('should show password strength meter') // P1

  // API Key Management (4 tests)
  test('should display API key') // P0
  test('should copy API key to clipboard') // P0
  test('should regenerate API key') // P0
  test('should confirm API key regeneration') // P1

  // Accessibility (2 tests)
  test('should be keyboard navigable') // P1
  test('should have proper form labels') // P1
});
```

---

## 4. Test Data Fixtures Required

### 4.1 Settings Fixtures (`tests/fixtures/settings.ts`)

```typescript
// tests/fixtures/settings.ts

export interface SMTPConfig {
  host: string;
  port: number;
  username: string;
  password: string;
  from_address: string;
  encryption: 'none' | 'ssl' | 'starttls';
}

export const validSMTPConfig: SMTPConfig = {
  host: 'smtp.test.local',
  port: 587,
  username: 'testuser',
  password: 'testpass123',
  from_address: 'noreply@test.local',
  encryption: 'starttls',
};

export const invalidSMTPConfigs = {
  missingHost: { ...validSMTPConfig, host: '' },
  invalidPort: { ...validSMTPConfig, port: -1 },
  invalidEmail: { ...validSMTPConfig, from_address: 'not-an-email' },
};

export interface SystemSettings {
  caddyAdminApi: string;
  sslProvider: 'auto' | 'letsencrypt-staging' | 'letsencrypt-prod' | 'zerossl';
  domainLinkBehavior: 'same_tab' | 'new_tab' | 'new_window';
  publicUrl: string;
}

export const defaultSystemSettings: SystemSettings = {
  caddyAdminApi: 'http://localhost:2019',
  sslProvider: 'auto',
  domainLinkBehavior: 'new_tab',
  publicUrl: 'http://localhost:8080',
};

export function generatePublicUrl(valid: boolean = true): string {
  if (valid) {
    return `https://charon-test-${Date.now()}.example.com`;
  }
  return 'not-a-valid-url';
}
```

### 4.2 User Fixtures (`tests/fixtures/users.ts`)

```typescript
// tests/fixtures/users.ts (extend existing)

export interface InviteRequest {
  email: string;
  role: 'admin' | 'user';
  permission_mode: 'allow_all' | 'deny_all';
  permitted_hosts?: number[];
}

export function generateInviteEmail(): string {
  return `invited-${Date.now()}@test.local`;
}

export const validInviteRequest: InviteRequest = {
  email: generateInviteEmail(),
  role: 'user',
  permission_mode: 'allow_all',
};

export const adminInviteRequest: InviteRequest = {
  email: generateInviteEmail(),
  role: 'admin',
  permission_mode: 'allow_all',
};
```

### 4.3 Notification Fixtures (`tests/fixtures/notifications.ts`)

```typescript
// tests/fixtures/notifications.ts

export interface NotificationProviderConfig {
  name: string;
  type: 'discord' | 'slack' | 'gotify' | 'telegram' | 'generic' | 'webhook';
  url: string;
  config?: string;
  template?: string;
  enabled: boolean;
  notify_proxy_hosts: boolean;
  notify_certs: boolean;
  notify_uptime: boolean;
}

export function generateProviderName(): string {
  return `test-provider-${Date.now()}`;
}

export const discordProvider: NotificationProviderConfig = {
  name: generateProviderName(),
  type: 'discord',
  url: 'https://discord.com/api/webhooks/test/token',
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: true,
  notify_uptime: false,
};

export const slackProvider: NotificationProviderConfig = {
  name: generateProviderName(),
  type: 'slack',
  url: 'https://hooks.slack.com/services/T00/B00/XXXXX',
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: false,
  notify_uptime: true,
};

export const genericWebhookProvider: NotificationProviderConfig = {
  name: generateProviderName(),
  type: 'generic',
  url: 'https://webhook.test.local/notify',
  config: '{"message": "{{.Message}}"}',
  template: 'minimal',
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: true,
  notify_uptime: true,
};
```

### 4.4 Encryption Fixtures (`tests/fixtures/encryption.ts`)

```typescript
// tests/fixtures/encryption.ts

export interface EncryptionStatus {
  current_version: number;
  next_key_configured: boolean;
  legacy_key_count: number;
  providers_on_current_version: number;
  providers_on_older_versions: number;
}

export const healthyEncryptionStatus: EncryptionStatus = {
  current_version: 2,
  next_key_configured: true,
  legacy_key_count: 0,
  providers_on_current_version: 5,
  providers_on_older_versions: 0,
};

export const needsRotationStatus: EncryptionStatus = {
  current_version: 1,
  next_key_configured: true,
  legacy_key_count: 1,
  providers_on_current_version: 3,
  providers_on_older_versions: 2,
};
```

---

## 5. Implementation Order

### Day 1: System Settings & Account Settings (P0 tests)

| Order | File | Tests | Rationale |
|-------|------|-------|-----------|
| 1 | `system-settings.spec.ts` | P0 only (15) | Core configuration, affects all features |
| 2 | `account-settings.spec.ts` | P0 only (12) | User authentication/profile critical |

**Deliverables:**
- ~27 P0 tests passing
- Fixtures for settings created
- Wait helpers for form submission verified

### Day 2: User Management (P0 + P1 tests)

| Order | File | Tests | Rationale |
|-------|------|-------|-----------|
| 3 | `user-management.spec.ts` | All P0 + P1 (24) | Security-critical, admin functionality |

**Deliverables:**
- ~24 tests passing
- User invite flow verified
- Permission management tested

### Day 3: SMTP Settings & Encryption Management

| Order | File | Tests | Rationale |
|-------|------|-------|-----------|
| 4 | `smtp-settings.spec.ts` | All P0 + P1 (15) | Email notification dependency |
| 5 | `encryption-management.spec.ts` | All P0 + P1 (12) | Security-critical |

**Deliverables:**
- ~27 tests passing
- SMTP test with mock server (if available)
- Key rotation flow verified

### Day 4: Notifications (All tests)

| Order | File | Tests | Rationale |
|-------|------|-------|-----------|
| 6 | `notifications.spec.ts` | All tests (22) | Complete provider management |

**Deliverables:**
- ~22 tests passing
- Multiple provider types tested
- Template management verified

### Day 5: Remaining Tests & Integration

| Order | Task | Tests | Rationale |
|-------|------|-------|-----------|
| 7 | P1/P2 tests in all files | ~15 | Complete coverage |
| 8 | Cross-settings integration | 5-8 | Verify settings interactions |
| 9 | Accessibility sweep | All files | Ensure a11y compliance |

**Deliverables:**
- All ~128 tests passing
- Full accessibility coverage
- Documentation updated

---

## 6. Risks and Blockers

### 6.1 Identified Risks

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| SMTP test requires real mail server | Medium | High | Use MailHog mock or skip connection tests in CI |
| Encryption rotation may affect other tests | High | Medium | Run encryption tests in isolation, restore state after |
| User deletion tests may affect auth state | High | Medium | Create dedicated test users, never delete admin used for auth |
| Notification webhooks need mock endpoints | Medium | High | Use MSW or route mocking |

### 6.2 Blockers to Address

1. **SMTP Mock Server**
   - Need MailHog or similar in docker-compose.playwright.yml
   - Profile: `--profile notification-tests`

2. **Encryption Test Isolation**
   - May need database snapshot/restore between tests
   - Or use mocked API responses for destructive operations

3. **Missing IDs/data-testid**
   - Several form fields lack proper `id` attributes
   - Recommendation: Add `data-testid` to SMTP form fields

### 6.3 Required Fixes Before Implementation

| Component | Issue | Fix Required |
|-----------|-------|--------------|
| `SMTPSettings.tsx` | Missing `id` on from_address, encryption fields | Add `id="smtp-from-address"`, `id="smtp-encryption"` |
| `Notifications.tsx` | Uses class-based styling selectors | Add `data-testid` to key elements |
| `Account.tsx` | Password confirmation modal needs accessible name | Add `aria-labelledby` |

---

## 7. Success Metrics

### 7.1 Coverage Targets

| Metric | Target | Measurement |
|--------|--------|-------------|
| Test Count | 128+ tests | Playwright test count |
| Pass Rate | 100% | CI pipeline status |
| P0 Coverage | 100% | All P0 scenarios have tests |
| P1 Coverage | 95%+ | Most P1 scenarios covered |
| Accessibility | WCAG 2.2 AA | Playwright accessibility checks |

### 7.2 Quality Gates

- [ ] All tests pass locally before PR
- [ ] All tests pass in CI (all 4 shards)
- [ ] No flaky tests (0 retries needed)
- [ ] Code coverage for settings pages > 80%
- [ ] No accessibility violations (a11y audit)

### 7.3 Definition of Done

- [ ] All 6 test files created and passing
- [ ] Fixtures created and documented
- [ ] Integration with existing test infrastructure
- [ ] Documentation updated in current_spec.md
- [ ] PR reviewed and merged

---

## Appendix A: Selector Reference Quick Guide

```typescript
// Common selectors for Settings E2E tests

// Navigation
const settingsNav = {
  systemTab: page.getByRole('link', { name: /system/i }),
  notificationsTab: page.getByRole('link', { name: /notifications/i }),
  smtpTab: page.getByRole('link', { name: /smtp/i }),
  accountTab: page.getByRole('link', { name: /account/i }),
};

// Buttons
const buttons = {
  save: page.getByRole('button', { name: /save/i }),
  cancel: page.getByRole('button', { name: /cancel/i }),
  test: page.getByRole('button', { name: /test/i }),
  delete: page.getByRole('button', { name: /delete/i }),
  confirm: page.getByRole('button', { name: /confirm/i }),
  add: page.getByRole('button', { name: /add/i }),
};

// Forms
const forms = {
  input: (name: string) => page.getByRole('textbox', { name: new RegExp(name, 'i') }),
  select: (name: string) => page.getByRole('combobox', { name: new RegExp(name, 'i') }),
  checkbox: (name: string) => page.getByRole('checkbox', { name: new RegExp(name, 'i') }),
  switch: (name: string) => page.getByRole('switch', { name: new RegExp(name, 'i') }),
};

// Feedback
const feedback = {
  toast: page.locator('[role="alert"]'),
  error: page.getByText(/error|failed|invalid/i),
  success: page.getByText(/success|saved|updated/i),
};

// Modals
const modals = {
  dialog: page.getByRole('dialog'),
  title: page.getByRole('heading').first(),
  close: page.getByRole('button', { name: /close|×/i }),
};
```

---

## Appendix B: API Endpoint Reference

| Category | Method | Endpoint | Auth Required |
|----------|--------|----------|---------------|
| Settings | GET | `/settings` | Yes |
| Settings | POST | `/settings` | Yes (Admin) |
| Settings | POST | `/settings/validate-url` | Yes |
| Settings | POST | `/settings/test-url` | Yes |
| SMTP | GET | `/settings/smtp` | Yes (Admin) |
| SMTP | POST | `/settings/smtp` | Yes (Admin) |
| SMTP | POST | `/settings/smtp/test` | Yes (Admin) |
| SMTP | POST | `/settings/smtp/test-email` | Yes (Admin) |
| Notifications | GET | `/notifications/providers` | Yes |
| Notifications | POST | `/notifications/providers` | Yes (Admin) |
| Notifications | PUT | `/notifications/providers/:id` | Yes (Admin) |
| Notifications | DELETE | `/notifications/providers/:id` | Yes (Admin) |
| Notifications | POST | `/notifications/providers/test` | Yes |
| Users | GET | `/users` | Yes (Admin) |
| Users | POST | `/users/invite` | Yes (Admin) |
| Users | PUT | `/users/:id` | Yes (Admin) |
| Users | DELETE | `/users/:id` | Yes (Admin) |
| Encryption | GET | `/admin/encryption/status` | Yes (Admin) |
| Encryption | POST | `/admin/encryption/rotate` | Yes (Admin) |
| Profile | GET | `/auth/profile` | Yes |
| Profile | PUT | `/auth/profile` | Yes |
| Profile | POST | `/auth/change-password` | Yes |
| Profile | POST | `/auth/regenerate-api-key` | Yes |

---

*Last Updated: January 19, 2026*
*Author: GitHub Copilot*
*Status: Ready for Implementation*

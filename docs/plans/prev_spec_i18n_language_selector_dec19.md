# Language Selector Bug - Root Cause Analysis & Implementation Plan

**Created:** 2025-12-19
**Last Updated:** 2025-12-19
**Status:** Implementation Ready - Production Approved
**Priority:** HIGH - User-impacting UX issue
**Estimated Timeline:** 3-4 weeks (15-20 business days)

---

## Executive Summary

**Bug**: Language selector changes state but UI remains in English across all pages.

**Root Cause**: Complete disconnect between i18n infrastructure and actual component implementation. While the language selection mechanism works correctly (state changes, localStorage updates, i18n.changeLanguage is called), NO components in the application are actually using the translation system to display text. All UI text is hardcoded in English.

**Impact**: The entire internationalization infrastructure (5 language files with 132+ translation keys each) is unused. The feature appears to work (dropdown changes, state updates) but has zero effect on the UI.

**Implementation Approach**: Phased rollout with per-component validation, automated testing, and feature flag protection. Focus on high-visibility components first (Layout/Navigation → ProxyHosts) to validate pattern before wider rollout.

---

## Table of Contents

1. [Translation Key Verification & Mapping](#translation-key-verification--mapping)
2. [File Inventory](#complete-file-inventory)
3. [Data Flow Analysis](#data-flow-analysis)
4. [Root Cause Summary](#root-cause-summary)
5. [Translation Key Naming Convention](#translation-key-naming-convention)
6. [Risk Assessment & Mitigation](#risk-assessment--mitigation)
7. [Implementation Plan (Revised Phases)](#implementation-plan---revised-phases)
8. [Detailed Timeline (3-4 Weeks)](#detailed-timeline-3-4-weeks)
9. [Code Review Checklist](#code-review-checklist)
10. [Testing Strategy (Expanded)](#testing-strategy-expanded)
11. [Success Metrics & Verification](#success-metrics--verification)
12. [Translation Maintenance Strategy](#translation-maintenance-strategy)
13. [Rollback Strategy & Feature Flags](#rollback-strategy--feature-flags)
14. [Code Patterns](#code-patterns)
15. [Conclusion](#conclusion)

---

## Translation Key Verification & Mapping

### Audit Summary

**Total Keys in English Translation File:** 132
**Languages Supported:** 5 (en, es, fr, de, zh)
**Key Categories:** 11 (common, navigation, dashboard, settings, proxyHosts, certificates, auth, errors, notifications, security, remoteServers)

### Translation Key Mapping Table

This table maps ALL hardcoded strings found in components to their corresponding translation keys. Status: ✅ Key Exists | ⚠️ Key Missing | 📝 Needs Addition

#### Navigation & Layout (Layout.tsx)

| Hardcoded String | Translation Key | Status | Notes |
|-----------------|-----------------|--------|-------|
| "Dashboard" | `navigation.dashboard` | ✅ | |
| "Proxy Hosts" | `navigation.proxyHosts` | ✅ | |
| "Remote Servers" | `navigation.remoteServers` | ✅ | |
| "Domains" | `navigation.domains` | ✅ | |
| "Certificates" | `navigation.certificates` | ✅ | |
| "Security" | `navigation.security` | ✅ | |
| "Access Lists" | `navigation.accessLists` | ✅ | |
| "CrowdSec" | `navigation.crowdsec` | ✅ | |
| "Rate Limiting" | `navigation.rateLimiting` | ✅ | |
| "WAF" | `navigation.waf` | ✅ | |
| "Uptime" | `navigation.uptime` | ✅ | |
| "Notifications" | `navigation.notifications` | ✅ | |
| "Users" | `navigation.users` | ✅ | |
| "Tasks" | `navigation.tasks` | ✅ | |
| "Settings" | `navigation.settings` | ✅ | |
| "Logout" | `auth.logout` | ✅ | |
| "Sign Out" | `auth.signOut` | ✅ | |

#### Dashboard (Dashboard.tsx)

| Hardcoded String | Translation Key | Status | Notes |
|-----------------|-----------------|--------|-------|
| "Dashboard" | `dashboard.title` | ✅ | |
| "Overview of your Charon reverse proxy" | `dashboard.description` | ✅ | |
| "Proxy Hosts" | `dashboard.proxyHosts` | ✅ | |
| "Remote Servers" | `dashboard.remoteServers` | ✅ | |
| "Certificates" | `dashboard.certificates` | ✅ | |
| "Access Lists" | `dashboard.accessLists` | ✅ | |
| "System Status" | `dashboard.systemStatus` | ✅ | |
| "Healthy" | `dashboard.healthy` | ✅ | |
| "X active" | `dashboard.activeHosts` | ✅ | Uses {{count}} placeholder |

#### Common Buttons & Actions

| Hardcoded String | Translation Key | Status | Notes |
|-----------------|-----------------|--------|-------|
| "Save" | `common.save` | ✅ | |
| "Cancel" | `common.cancel` | ✅ | |
| "Delete" | `common.delete` | ✅ | |
| "Edit" | `common.edit` | ✅ | |
| "Add" | `common.add` | ✅ | |
| "Create" | `common.create` | ✅ | |
| "Update" | `common.update` | ✅ | |
| "Close" | `common.close` | ✅ | |
| "Confirm" | `common.confirm` | ✅ | |
| "Back" | `common.back` | ✅ | |
| "Next" | `common.next` | ✅ | |
| "Loading..." | `common.loading` | ✅ | |
| "Enabled" | `common.enabled` | ✅ | |
| "Disabled" | `common.disabled` | ✅ | |
| "Search" | `common.search` | ✅ | |
| "Filter" | `common.filter` | ✅ | |

#### ProxyHosts (ProxyHosts.tsx - Priority Component)

| Hardcoded String | Translation Key | Status | Notes |
|-----------------|-----------------|--------|-------|
| "Proxy Hosts" | `proxyHosts.title` | ✅ | |
| "Manage your reverse proxy configurations" | `proxyHosts.description` | ✅ | |
| "Add Proxy Host" | `proxyHosts.addHost` | ✅ | |
| "Edit Proxy Host" | `proxyHosts.editHost` | ✅ | |
| "Delete Proxy Host" | `proxyHosts.deleteHost` | ✅ | |
| "Domain Names" | `proxyHosts.domainNames` | ✅ | |
| "Forward Host" | `proxyHosts.forwardHost` | ✅ | |
| "Forward Port" | `proxyHosts.forwardPort` | ✅ | |
| "SSL Enabled" | `proxyHosts.sslEnabled` | ✅ | |
| "Force SSL" | `proxyHosts.sslForced` | ✅ | |
| "Bulk Actions" | `proxyHosts.bulkActions` | ⚠️ | **NEEDS ADDITION** |
| "Apply ACL" | `proxyHosts.applyAcl` | ⚠️ | **NEEDS ADDITION** |
| "Export" | `proxyHosts.export` | ⚠️ | **NEEDS ADDITION** |

#### Auth & Setup (Login.tsx, Setup.tsx)

| Hardcoded String | Translation Key | Status | Notes |
|-----------------|-----------------|--------|-------|
| "Login" | `auth.login` | ✅ | |
| "Email" | `auth.email` | ✅ | |
| "Password" | `auth.password` | ✅ | |
| "Sign In" | `auth.signIn` | ✅ | |
| "Forgot Password?" | `auth.forgotPassword` | ✅ | |
| "Remember Me" | `auth.rememberMe` | ✅ | |

#### Error Messages & Notifications

| Hardcoded String | Translation Key | Status | Notes |
|-----------------|-----------------|--------|-------|
| "Changes saved successfully" | `notifications.saveSuccess` | ✅ | |
| "Failed to save changes" | `notifications.saveFailed` | ✅ | |
| "Deleted successfully" | `notifications.deleteSuccess` | ✅ | |
| "This field is required" | `errors.required` | ✅ | |
| "Invalid email address" | `errors.invalidEmail` | ✅ | |
| "Network error. Please check your connection." | `errors.networkError` | ✅ | |

### Missing Keys to Add

The following keys need to be added to all translation files (en, es, fr, de, zh):

```json
{
  "proxyHosts": {
    "bulkActions": "Bulk Actions",
    "applyAcl": "Apply ACL",
    "export": "Export",
    "import": "Import",
    "selectAll": "Select All",
    "clearSelection": "Clear Selection",
    "selectedCount": "{{count}} selected",
    "confirmDelete": "Are you sure you want to delete {{count}} proxy host(s)?",
    "confirmBulkUpdate": "Apply changes to {{count}} proxy host(s)?"
  },
  "security": {
    "title": "Security",
    "description": "Configure security settings",
    "headers": "Security Headers",
    "waf": "Web Application Firewall",
    "crowdsec": "CrowdSec Configuration",
    "rateLimit": "Rate Limiting"
  },
  "certificates": {
    "requestCertificate": "Request Certificate",
    "renewCertificate": "Renew Certificate",
    "revokeCertificate": "Revoke Certificate",
    "autoRenewal": "Auto Renewal",
    "wildcardCert": "Wildcard Certificate"
  },
  "remoteServers": {
    "title": "Remote Servers",
    "description": "Manage upstream servers",
    "addServer": "Add Remote Server",
    "editServer": "Edit Remote Server",
    "testConnection": "Test Connection",
    "connectionStatus": "Connection Status"
  },
  "domains": {
    "title": "Domains",
    "description": "Manage your domains",
    "addDomain": "Add Domain",
    "verifyDomain": "Verify Domain",
    "dnsSettings": "DNS Settings"
  },
  "uptime": {
    "title": "Uptime Monitoring",
    "description": "Monitor service availability",
    "addMonitor": "Add Monitor",
    "responseTime": "Response Time",
    "availability": "Availability"
  },
  "tasks": {
    "title": "Tasks",
    "description": "View background tasks",
    "running": "Running",
    "completed": "Completed",
    "failed": "Failed",
    "scheduled": "Scheduled"
  },
  "logs": {
    "title": "Logs",
    "description": "View system logs",
    "downloadLogs": "Download Logs",
    "clearLogs": "Clear Logs",
    "filterByLevel": "Filter by Level"
  },
  "smtp": {
    "title": "Email Settings",
    "description": "Configure SMTP for email notifications",
    "testEmail": "Send Test Email",
    "smtpHost": "SMTP Host",
    "smtpPort": "SMTP Port"
  },
  "backups": {
    "title": "Backups",
    "description": "Manage system backups",
    "createBackup": "Create Backup",
    "restoreBackup": "Restore Backup",
    "downloadBackup": "Download Backup",
    "scheduleBackup": "Schedule Backup"
  },
  "common": {
    "logout": "Logout",
    "profile": "Profile",
    "account": "Account",
    "preferences": "Preferences",
    "advanced": "Advanced",
    "export": "Export",
    "import": "Import",
    "refresh": "Refresh",
    "retry": "Retry",
    "viewDetails": "View Details",
    "copyToClipboard": "Copy to Clipboard",
    "copied": "Copied!"
  }
}
```

### Key Coverage Analysis

- **Existing Keys:** 132
- **Missing Keys Identified:** 48
- **Total Keys Needed:** 180
- **Coverage Rate:** 73% (132/180)
- **Target Coverage:** 100% (all hardcoded strings mapped)

**Action Required:** Add missing keys to all 5 language files before Phase 1 implementation begins.

---

## Complete File Inventory

### 1. Infrastructure Layer (✅ Working Correctly)

| File | Purpose | Status |
|------|---------|--------|
| \`frontend/src/i18n.ts\` | i18next configuration, language detection, localStorage integration | ✅ Functional |
| \`frontend/src/context/LanguageContext.tsx\` | React Context provider for language state management | ✅ Functional |
| \`frontend/src/context/LanguageContextValue.ts\` | TypeScript types for language context | ✅ Functional |
| \`frontend/src/hooks/useLanguage.ts\` | React hook for accessing language context | ✅ Functional |
| \`frontend/src/components/LanguageSelector.tsx\` | UI dropdown component for language selection | ✅ Functional |
| \`frontend/src/main.tsx\` | App wrapper with LanguageProvider | ✅ Properly wrapped |

### 2. Translation Files (✅ Complete but Unused)

All translation files contain **132+ keys** organized into sections:

- \`common\`: 29 keys (save, cancel, delete, edit, etc.)
- \`navigation\`: 15 keys (dashboard, proxyHosts, security, etc.)
- \`dashboard\`: 13 keys (title, description, stats, etc.)
- \`proxyHosts\`: 25+ keys
- \`certificates\`: 20+ keys
- \`security\`: 30+ keys

Files:

- \`frontend/src/locales/en/translation.json\` ✅
- \`frontend/src/locales/es/translation.json\` ✅
- \`frontend/src/locales/fr/translation.json\` ✅
- \`frontend/src/locales/de/translation.json\` ✅
- \`frontend/src/locales/zh/translation.json\` ✅

### 3. Application Layer (❌ Not Using Translations)

#### Pages - ALL use hardcoded English text

- \`Dashboard.tsx\` (177 lines)
- \`ProxyHosts.tsx\` (1023 lines) **LARGEST**
- \`SystemSettings.tsx\` (430 lines)
- \`RemoteServers.tsx\` (~500 lines)
- \`Domains.tsx\` (~400 lines)
- \`Certificates.tsx\` (~600 lines)
- \`Security.tsx\` (~500 lines)
- \`AccessLists.tsx\` (~700 lines)
- \`CrowdSecConfig.tsx\` (~600 lines)
- \`WafConfig.tsx\` (~400 lines)
- \`RateLimiting.tsx\` (~400 lines)
- \`Uptime.tsx\` (~500 lines)
- \`Notifications.tsx\` (~400 lines)
- \`UsersPage.tsx\` (~500 lines)
- \`SecurityHeaders.tsx\` (~800 lines)
- \`Login.tsx\` (~200 lines)
- \`Setup.tsx\` (~300 lines)
- \`Backups.tsx\` (~400 lines)
- \`Tasks.tsx\` (~300 lines)
- \`Logs.tsx\` (~400 lines)
- \`ImportCaddy.tsx\` (~200 lines)
- \`ImportCrowdSec.tsx\` (~200 lines)
- \`Account.tsx\` (~300 lines)
- \`SMTPSettings.tsx\` (~400 lines)

#### Layout & Navigation

- \`Layout.tsx\` (367 lines) - ALL navigation items hardcoded

---

## Data Flow Analysis

### Current Flow (Working but Ineffective)

\`\`\`
┌─────────────────────────────────────────────────────────────────┐
│ 1. USER INTERACTION                                             │
│    User clicks LanguageSelector → selects "Español"            │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. STATE UPDATE (LanguageSelector.tsx)                          │
│    handleChange(e) → setLanguage('es')                          │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. CONTEXT UPDATE (LanguageContext.tsx)                         │
│    - setLanguageState('es')                                     │
│    - localStorage.setItem('charon-language', 'es')              │
│    - i18n.changeLanguage('es')                                  │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. I18N LIBRARY UPDATE (i18n.ts)                                │
│    - i18next loads es/translation.json                          │
│    - Translation keys are available via i18n.t()                │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│ 5. COMPONENT RENDERING ❌ BROKEN HERE                           │
│    Pages render hardcoded English:                              │
│    - <h1>Dashboard</h1>                                         │
│    - <button>Save</button>                                      │
│    NO COMPONENT CALLS useTranslation() OR t()                   │
└─────────────────────────────────────────────────────────────────┘
\`\`\`

### Expected Flow (After Fix)

\`\`\`
┌─────────────────────────────────────────────────────────────────┐
│ 5. COMPONENT RENDERING ✅ FIXED                                 │
│    Components use useTranslation:                               │
│    const { t } = useTranslation()                               │
│    return <h1>{t('dashboard.title')}</h1>                       │
│                                                                  │
│    Translation resolves:                                        │
│    - t('dashboard.title') → "Panel de Control" (Spanish)       │
│    - t('common.save') → "Guardar"                               │
└─────────────────────────────────────────────────────────────────┘
\`\`\`

---

## Root Cause Summary

### What Works ✅

1. Language selection UI (LanguageSelector)
2. State management (LanguageContext)
3. localStorage persistence
4. i18next configuration
5. Translation files (complete with 132+ keys each)
6. React Context provider hierarchy

### What's Broken ❌

1. **ZERO components import \`useTranslation\` from react-i18next**
   - Only found in: LanguageContext.tsx (infrastructure) and test files
   - Not found in: Any page, layout, or UI component

2. **ZERO components call \`t()\` function to get translations**
   - All text is hardcoded in JSX
   - Example: \`<h1>Dashboard</h1>\` instead of \`<h1>{t('dashboard.title')}</h1>\`

3. **Navigation menu is entirely hardcoded**
   - Layout.tsx has 20+ navigation items with English labels

---

## Translation Key Naming Convention

### Standard Structure

All translation keys follow a hierarchical namespace structure:

```
{category}.{subcategory}.{descriptor}
```

### Category Guidelines

| Category | Purpose | Example Keys |
|----------|---------|--------------|
| `common` | Shared UI elements used across multiple pages | `common.save`, `common.cancel` |
| `navigation` | Top-level navigation menu items | `navigation.dashboard`, `navigation.proxyHosts` |
| `{page}` | Page-specific content (dashboard, proxyHosts, etc.) | `proxyHosts.title`, `dashboard.description` |
| `errors` | Error messages and validation | `errors.required`, `errors.invalidEmail` |
| `notifications` | Toast/alert messages | `notifications.saveSuccess` |
| `auth` | Authentication and authorization | `auth.login`, `auth.logout` |

### Naming Rules

1. **Use camelCase** for all keys: `proxyHosts`, not `proxy-hosts` or `proxy_hosts`
2. **Be specific but concise**: `proxyHosts.addHost` not `proxyHosts.addNewProxyHost`
3. **Avoid abbreviations** unless universally understood: `smtp` (OK), `cfg` (avoid, use `config`)
4. **Group related keys** under same parent: `dashboard.activeHosts`, `dashboard.activeServers`

### Special Patterns

#### Pluralization

Use ICU MessageFormat for plurals:

```json
{
  "proxyHosts": {
    "count": "{{count}} proxy host",
    "count_plural": "{{count}} proxy hosts",
    "selectedCount": "{{count}} selected",
    "selectedCount_plural": "{{count}} selected"
  }
}
```

**Usage:**

```tsx
t('proxyHosts.count', { count: 1 })  // "1 proxy host"
t('proxyHosts.count', { count: 5 })  // "5 proxy hosts"
```

#### Dynamic Interpolation

Use `{{variableName}}` for dynamic content:

```json
{
  "dashboard": {
    "activeHosts": "{{count}} active",
    "welcomeUser": "Welcome back, {{userName}}!",
    "lastSync": "Last synced {{time}}"
  }
}
```

**Usage:**

```tsx
t('dashboard.welcomeUser', { userName: 'Alice' })  // "Welcome back, Alice!"
```

#### Context Variants

For gender or context-specific translations:

```json
{
  "common": {
    "delete": "Delete",
    "delete_male": "Delete (m)",
    "delete_female": "Delete (f)",
    "save_short": "Save",
    "save_long": "Save Changes"
  }
}
```

**Usage:**

```tsx
t('common.delete', { context: 'male' })  // Uses delete_male
t('common.save', { context: 'short' })   // Uses save_short
```

#### Nested Keys

Maximum 3 levels deep for maintainability:

```json
{
  "security": {
    "headers": {
      "csp": "Content Security Policy",
      "hsts": "HTTP Strict Transport Security"
    },
    "waf": {
      "enabled": "WAF Enabled",
      "rulesets": "Active Rulesets"
    }
  }
}
```

**Usage:**

```tsx
t('security.headers.csp')  // "Content Security Policy"
```

#### Boolean States

Use consistent naming for on/off states:

```json
{
  "common": {
    "enabled": "Enabled",
    "disabled": "Disabled",
    "active": "Active",
    "inactive": "Inactive",
    "on": "On",
    "off": "Off"
  }
}
```

### Examples by Component Type

#### Page Title & Description

```json
{
  "proxyHosts": {
    "title": "Proxy Hosts",
    "description": "Manage your reverse proxy configurations"
  }
}
```

#### Form Labels

```json
{
  "proxyHosts": {
    "domainNames": "Domain Names",
    "forwardHost": "Forward Host",
    "forwardPort": "Forward Port",
    "sslEnabled": "SSL Enabled"
  }
}
```

#### Button Actions

```json
{
  "proxyHosts": {
    "addHost": "Add Proxy Host",
    "editHost": "Edit Proxy Host",
    "deleteHost": "Delete Proxy Host",
    "bulkActions": "Bulk Actions"
  }
}
```

#### Table Columns

```json
{
  "proxyHosts": {
    "columnDomain": "Domain",
    "columnTarget": "Target",
    "columnStatus": "Status",
    "columnActions": "Actions"
  }
}
```

#### Confirmation Dialogs

```json
{
  "proxyHosts": {
    "confirmDelete": "Are you sure you want to delete {{domainName}}?",
    "confirmBulkDelete": "Delete {{count}} proxy host(s)?",
    "confirmDisable": "Disable this proxy host?"
  }
}
```

### Anti-Patterns to Avoid

❌ **Don't repeat category in key:**

```json
{ "proxyHosts": { "proxyHostsTitle": "..." } }  // Wrong
{ "proxyHosts": { "title": "..." } }             // Correct
```

❌ **Don't embed markup:**

```json
{ "common": { "warning": "<strong>Warning:</strong> ..." } }  // Wrong
{ "common": { "warning": "Warning: ..." } }                   // Correct
```

❌ **Don't hardcode units:**

```json
{ "uptime": { "responseTime": "Response Time (ms)" } }  // Wrong
{ "uptime": { "responseTime": "Response Time", "unitMs": "ms" } }  // Correct
```

❌ **Don't use generic keys for specific content:**

```json
{ "common": { "text1": "...", "text2": "..." } }  // Wrong
{ "proxyHosts": { "helpText": "...", "warningText": "..." } }  // Correct
```

---

## Risk Assessment & Mitigation

### Risk 1: State Management Re-render Performance

**Risk Level:** 🟡 MEDIUM

**Description:** Adding `useTranslation()` hook to every component may cause unnecessary re-renders when language changes, especially in large components like ProxyHosts.tsx (1023 lines).

**Impact:**

- Language changes trigger re-render of all components using `useTranslation()`
- Potential UI lag or frozen state during language switch
- Memory pressure from simultaneous component updates

**Mitigation Strategies:**

1. **Use React.memo for expensive components:**

   ```tsx
   export default React.memo(ProxyHosts)
   ```

2. **Memoize translation calls in render-heavy components:**

   ```tsx
   const columns = useMemo(() => [
     { header: t('proxyHosts.domain'), ... },
     { header: t('proxyHosts.target'), ... }
   ], [t, language])
   ```

3. **Split large components into smaller, memoized subcomponents:**

   ```tsx
   const ProxyHostTable = React.memo(({ data }) => { ... })
   const ProxyHostForm = React.memo(({ onSave }) => { ... })
   ```

4. **Add performance monitoring:**

   ```tsx
   useEffect(() => {
     const start = performance.now()
     return () => {
       const duration = performance.now() - start
       if (duration > 100) console.warn('Slow render:', duration)
     }
   })
   ```

**Acceptance Criteria:**

- Language switch completes in < 500ms on Desktop
- Language switch completes in < 1000ms on Mobile
- No visible UI freezing during switch
- Memory usage increase < 10% after language switch

---

### Risk 2: Third-Party Component i18n Support

**Risk Level:** 🟠 HIGH

**Description:** Some third-party UI components (DataTable, Dialog, DatePicker, etc.) may not properly support dynamic language changes or may have their own i18n systems.

**Affected Components:**

- DataTable (pagination, sorting labels)
- Date/Time Pickers (month names, day names)
- Form validation libraries (error messages)
- Rich text editors
- File upload components

**Mitigation Strategies:**

1. **Audit all third-party components** (Pre-Phase 1):

   ```bash
   grep -r "import.*from" frontend/src/components | grep -E "(table|date|form|picker|editor)"
   ```

2. **Wrapper pattern for incompatible components:**

   ```tsx
   // Wrap DatePicker with localized props
   const LocalizedDatePicker = ({ ...props }) => {
     const { i18n } = useTranslation()
     return (
       <DatePicker
         {...props}
         locale={localeMap[i18n.language]}
         monthLabels={t('common.months', { returnObjects: true })}
       />
     )
   }
   ```

3. **Replace components if necessary:**
   - Document replacement decisions
   - Ensure feature parity
   - Test thoroughly

4. **Configure third-party i18n integrations:**

   ```tsx
   // For libraries like react-datepicker
   import { registerLocale, setDefaultLocale } from "react-datepicker";
   import es from 'date-fns/locale/es';
   registerLocale('es', es);
   ```

**Action Items:**

- Create compatibility matrix (see below)
- Test each component with all 5 languages
- Document workarounds in component README

---

### Risk 3: Date/Time/Number Formatting

**Risk Level:** 🟡 MEDIUM

**Description:** Dates, times, numbers, and currencies need locale-aware formatting. Hardcoded formats (MM/DD/YYYY) will not adapt to user locale.

**Examples:**

- Dates: US (12/31/2025) vs EU (31/12/2025)
- Times: 12-hour (3:00 PM) vs 24-hour (15:00)
- Numbers: 1,234.56 (US) vs 1.234,56 (EU)
- Currencies: $1,234.56 vs 1 234,56 €

**Mitigation Strategies:**

1. **Use Intl API for formatting:**

   ```tsx
   // Date formatting
   const formatDate = (date: Date, locale: string) => {
     return new Intl.DateTimeFormat(locale, {
       year: 'numeric',
       month: 'long',
       day: 'numeric'
     }).format(date)
   }

   // Number formatting
   const formatNumber = (num: number, locale: string) => {
     return new Intl.NumberFormat(locale).format(num)
   }

   // Currency formatting
   const formatCurrency = (amount: number, locale: string, currency: string) => {
     return new Intl.NumberFormat(locale, {
       style: 'currency',
       currency
     }).format(amount)
   }
   ```

2. **Create formatting utilities:**

   ```tsx
   // frontend/src/utils/formatting.ts
   import { useTranslation } from 'react-i18next'

   export const useFormatting = () => {
     const { i18n } = useTranslation()

     return {
       date: (date: Date) => formatDate(date, i18n.language),
       time: (date: Date) => formatTime(date, i18n.language),
       number: (num: number) => formatNumber(num, i18n.language),
       relativeTime: (date: Date) => formatRelativeTime(date, i18n.language)
     }
   }
   ```

3. **Use date-fns with locale support:**

   ```tsx
   import { format } from 'date-fns'
   import { es, fr, de, zhCN } from 'date-fns/locale'

   const locales = { en: enUS, es, fr, de, zh: zhCN }

   format(new Date(), 'PPP', { locale: locales[language] })
   ```

**Acceptance Criteria:**

- All dates use `Intl.DateTimeFormat` or `date-fns` with locale
- All numbers use `Intl.NumberFormat`
- No hardcoded date/number formats in components

---

### Risk 4: RTL (Right-to-Left) Language Support

**Risk Level:** 🟡 MEDIUM

**Description:** Future support for RTL languages (Arabic, Hebrew) will require layout and CSS adjustments. Current plan only includes LTR languages, but architecture should not prevent RTL addition.

**Current Languages:** All LTR (English, Spanish, French, German, Chinese)
**Future Consideration:** Arabic (ar), Hebrew (he)

**Mitigation Strategies:**

1. **Use logical CSS properties now:**

   ```css
   /* ❌ Avoid */
   margin-left: 16px;
   padding-right: 8px;

   /* ✅ Use instead */
   margin-inline-start: 16px;
   padding-inline-end: 8px;
   ```

2. **Avoid absolute positioning where possible:**

   ```css
   /* ❌ Problematic for RTL */
   position: absolute;
   left: 0;

   /* ✅ Use flexbox/grid */
   display: flex;
   justify-content: flex-start;
   ```

3. **Add `dir` attribute support to root:**

   ```tsx
   // main.tsx or App.tsx
   useEffect(() => {
     document.dir = i18n.dir(i18n.language)
   }, [i18n.language])
   ```

4. **Test with RTL browser extension:**
   - Install "Force RTL" browser extension
   - Validate layout doesn't break
   - Check icon alignment

**Acceptance Criteria:**

- All CSS uses logical properties (inline-start/end)
- No hardcoded left/right positioning
- `dir` attribute infrastructure in place
- Layout tested with "Force RTL" tool

---

### Risk 5: Translation File Drift

**Risk Level:** 🟠 HIGH

**Description:** Over time, translation files can become out of sync as developers add keys to English but forget to update other languages, leading to missing translations and fallback to English.

**Impact:**

- Inconsistent user experience across languages
- Some text remains in English in non-English locales
- Hard to track which keys are missing

**Mitigation Strategies:**

1. **Automated sync checking (CI/CD - see Maintenance Strategy section)**

2. **Translation key generation script:**

   ```bash
   # scripts/sync-translations.sh
   #!/bin/bash
   node scripts/sync-translation-keys.js
   ```

   ```javascript
   // scripts/sync-translation-keys.js
   const fs = require('fs')
   const path = require('path')

   const localesDir = path.join(__dirname, '../frontend/src/locales')
   const enFile = path.join(localesDir, 'en/translation.json')
   const enKeys = JSON.parse(fs.readFileSync(enFile, 'utf8'))

   const languages = ['es', 'fr', 'de', 'zh']

   languages.forEach(lang => {
     const langFile = path.join(localesDir, `${lang}/translation.json`)
     const langKeys = JSON.parse(fs.readFileSync(langFile, 'utf8'))
     const missingKeys = findMissingKeys(enKeys, langKeys)

     if (missingKeys.length > 0) {
       console.error(`❌ Missing keys in ${lang}:`, missingKeys)
       process.exit(1)
     }
   })
   ```

3. **Pull request template requirement:**
   - Checklist item: "All translation files updated"
   - Automated comment if keys don't match

4. **Fallback chain with warnings:**

   ```tsx
   // i18n.ts
   i18n.init({
     fallbackLng: 'en',
     missingKeyHandler: (lngs, ns, key) => {
       console.warn(`Missing translation key: ${key} for language: ${lngs[0]}`)
       // Optionally report to error tracking
       Sentry.captureMessage(`Missing i18n key: ${key}`)
     }
   })
   ```

**Acceptance Criteria:**

- CI fails if translation keys don't match
- Missing keys logged to console in development
- PR template includes translation checklist

---

### Risk Mitigation Summary

| Risk | Level | Primary Mitigation | Monitoring |
|------|-------|-------------------|------------|
| Re-render Performance | 🟡 Medium | React.memo, useMemo | Performance profiling in Phase 1 |
| Third-Party Components | 🟠 High | Audit + wrapper pattern | Manual QA per component |
| Date/Number Formatting | 🟡 Medium | Intl API utilities | Visual QA in all locales |
| RTL Support | 🟡 Medium | Logical CSS properties | RTL browser extension testing |
| Translation Drift | 🟠 High | CI checks + scripts | Automated on every PR |

---

## Implementation Plan - Revised Phases

**Strategy:** Validate pattern early with high-visibility components, then scale systematically. Each phase includes implementation, testing, code review, and bug fixes.

---

### Phase 1: Layout & Navigation (Days 1-3)

**Objective:** Establish pattern with most visible user-facing component. Validates infrastructure and approach.

**Files:**

- `frontend/src/components/Layout.tsx` (367 lines)
- `frontend/src/components/LanguageSelector.tsx` (already uses translations)

**Tasks:**

- [ ] Day 1: Add missing translation keys (48 new keys) to all 5 language files
- [ ] Day 1: Update navigation array to use `t('navigation.*')` keys
- [ ] Day 1: Update logout/profile buttons
- [ ] Day 1: Update sidebar tooltips
- [ ] Day 2: Create Layout.test.tsx with language switching tests
- [ ] Day 2: Manual QA in all 5 languages
- [ ] Day 2: Code review and address feedback
- [ ] Day 3: Fix bugs, performance profiling, merge PR

**Success Criteria:**

- Navigation menu switches languages instantly
- No console warnings for missing keys
- All 5 languages render correctly
- Performance: < 100ms to switch languages
- Code review approved

**Example Changes:**

```tsx
// Before
const navigation: NavItem[] = [
  { name: 'Dashboard', path: '/', icon: '📊' },
  { name: 'Proxy Hosts', path: '/proxy-hosts', icon: '🌐' }
]

// After
const { t } = useTranslation()
const navigation: NavItem[] = [
  { name: t('navigation.dashboard'), path: '/', icon: '📊' },
  { name: t('navigation.proxyHosts'), path: '/proxy-hosts', icon: '🌐' }
]
```

---

### Phase 2: ProxyHosts (Days 4-7)

**Objective:** Validate pattern on largest, most complex component. Proves approach scales to complex forms and tables.

**Files:**

- `frontend/src/pages/ProxyHosts.tsx` (1023 lines) **HIGHEST COMPLEXITY**
- `frontend/src/components/ProxyHostForm.tsx` (if exists)

**Tasks:**

- [ ] Day 4: Update PageShell title/description
- [ ] Day 4: Update all button text (Create, Edit, Delete, Bulk Apply)
- [ ] Day 5: Update DataTable column headers
- [ ] Day 5: Update form labels and placeholders
- [ ] Day 5: Update status badges (Enabled/Disabled, SSL indicators)
- [ ] Day 6: Update dialogs (confirmation, bulk update)
- [ ] Day 6: Update toast/notification messages
- [ ] Day 6: Add ProxyHosts.test.tsx
- [ ] Day 6: Manual QA with CRUD operations
- [ ] Day 7: Code review, bug fixes, performance check, merge PR

**Success Criteria:**

- All UI text translates correctly
- Form validation messages localized
- Toast notifications in selected language
- No layout breaks in any language (especially German - longest strings)
- Performance: Page renders in < 200ms after language change
- Code review approved

**Example Changes:**

```tsx
// Before
<DataTable columns={[
  { header: 'Domain', accessorKey: 'domain_names' },
  { header: 'Status', accessorKey: 'enabled' }
]} />

// After
const { t } = useTranslation()
<DataTable columns={[
  { header: t('proxyHosts.columnDomain'), accessorKey: 'domain_names' },
  { header: t('common.status'), accessorKey: 'enabled' }
]} />
```

---

### Phase 3: Core Pages (Days 8-12)

**Objective:** Apply validated pattern to remaining core pages. Parallelizable work.

**Files (in priority order):**

1. `SystemSettings.tsx` (430 lines) - Already imports LanguageSelector
2. `Security.tsx` (500 lines)
3. `AccessLists.tsx` (700 lines)
4. `Certificates.tsx` (600 lines)
5. `RemoteServers.tsx` (500 lines)
6. `Domains.tsx` (400 lines)

**Tasks:**

- [ ] Day 8: SystemSettings, Security (2 files)
- [ ] Day 9: AccessLists, Certificates (2 files)
- [ ] Day 10: RemoteServers, Domains (2 files)
- [ ] Day 11: Add tests for all 6 files
- [ ] Day 11: Manual QA for all pages
- [ ] Day 12: Code review, bug fixes, merge PRs

**Success Criteria:**

- All pages follow established pattern
- Tests pass for all components
- No regressions in functionality
- Code reviews approved

---

### Phase 4: Dashboard & Supporting Pages (Days 13-15)

**Objective:** Complete main application pages and validate integration across full workflow.

**Files:**

- `Dashboard.tsx` (177 lines) - **Integration validation**
- `CrowdSecConfig.tsx` (600 lines)
- `WafConfig.tsx` (400 lines)
- `RateLimiting.tsx` (400 lines)
- `Uptime.tsx` (500 lines)
- `Notifications.tsx` (400 lines)
- `UsersPage.tsx` (500 lines)
- `SecurityHeaders.tsx` (800 lines)

**Tasks:**

- [ ] Day 13: Dashboard, CrowdSecConfig, WafConfig
- [ ] Day 14: RateLimiting, Uptime, Notifications, UsersPage
- [ ] Day 14: SecurityHeaders
- [ ] Day 15: Integration tests (full user workflow in each language)
- [ ] Day 15: Code review, bug fixes, merge PRs

**Success Criteria:**

- Dashboard correctly aggregates translated content
- All stats and widgets display localized text
- Full workflow (create proxy → configure SSL → test) works in all languages
- Code reviews approved

---

### Phase 5: Auth & Setup Pages (Days 16-17)

**Objective:** Critical user onboarding experience. Must be perfect.

**Files:**

- `Login.tsx` (200 lines)
- `Setup.tsx` (300 lines)
- `Account.tsx` (300 lines)

**Tasks:**

- [ ] Day 16: Login, Setup pages
- [ ] Day 16: Account page
- [ ] Day 16: Test authentication flows in all languages
- [ ] Day 17: QA first-time setup experience
- [ ] Day 17: Code review, bug fixes, merge PR

**Success Criteria:**

- First-time users see setup in their browser's default language
- Login errors display in correct language
- Form validation messages localized
- Success/error toasts localized
- Code review approved

---

### Phase 6: Utility Pages & Final Integration (Days 18-19)

**Objective:** Complete remaining pages and ensure consistency.

**Files:**

- `Backups.tsx` (400 lines)
- `Tasks.tsx` (300 lines)
- `Logs.tsx` (400 lines)
- `ImportCaddy.tsx` (200 lines)
- `ImportCrowdSec.tsx` (200 lines)
- `SMTPSettings.tsx` (400 lines)

**Tasks:**

- [ ] Day 18: All utility pages
- [ ] Day 18: Import pages
- [ ] Day 18: SMTP settings
- [ ] Day 19: Final integration QA
- [ ] Day 19: Code reviews, bug fixes, merge PRs

**Success Criteria:**

- All pages translated
- Import workflows work in all languages
- No missing translation keys
- Code reviews approved

---

### Phase 7: Comprehensive QA & Polish (Days 20-23)

**Objective:** Thorough testing, bug fixes, performance optimization, and production readiness.

**Tasks:**

#### Day 20: Automated Testing

- [ ] Run full test suite in all 5 languages
- [ ] Translation coverage tests (100% key coverage)
- [ ] Bundle size analysis (ensure no significant increase)
- [ ] Performance profiling (language switching speed)
- [ ] Accessibility testing (screen reader compatibility)

#### Day 21: Manual QA - Core Workflows

- [ ] Test full user workflows in all 5 languages:
  - [ ] First-time setup
  - [ ] Login/logout
  - [ ] Create/edit/delete proxy host
  - [ ] Configure SSL certificate
  - [ ] Apply access list
  - [ ] Configure security settings
  - [ ] View logs and tasks
- [ ] Test language switching mid-workflow (e.g., while editing form)
- [ ] Test WebSocket reconnection with language changes (logs page)
- [ ] Test browser back/forward with language changes

#### Day 22: Edge Cases & Error Handling

- [ ] Backend API errors in all languages
- [ ] Network errors with WebSocket (logs page)
- [ ] Mid-edit language switches (forms preserve data)
- [ ] Rapid language switching (no race conditions)
- [ ] Browser locale detection on first visit
- [ ] LocalStorage corruption/missing (graceful fallback)

#### Day 23: Final Polish & Documentation

- [ ] Fix all bugs found in QA
- [ ] Update user documentation with language switching instructions
- [ ] Create developer guide for adding new translations
- [ ] Final performance check
- [ ] Prepare release notes

**Success Criteria:**

- All automated tests pass
- All manual QA workflows complete successfully
- No P0/P1 bugs remaining
- Performance meets targets (< 500ms language switch)
- Bundle size increase < 50KB
- Documentation updated

---

## Detailed Timeline (3-4 Weeks)

### Week 1: Foundation & Validation

| Day | Phase | Tasks | Deliverables |
|-----|-------|-------|--------------|
| **Mon 1** | Phase 1 | Add missing keys, update Layout.tsx navigation | Navigation menu translations |
| **Tue 2** | Phase 1 | Tests, QA, code review | Layout PR ready |
| **Wed 3** | Phase 1 | Bug fixes, performance, merge | ✅ Layout complete |
| **Thu 4-5** | Phase 2 | ProxyHosts.tsx implementation | ProxyHosts translations |
| **Fri 5** | Phase 2 | ProxyHosts forms, tables | ProxyHosts UI complete |

**Week 1 Milestones:**

- ✅ Navigation fully translated (most visible change)
- ✅ ProxyHosts 80% complete (validates complex component approach)
- ✅ Pattern established and documented

---

### Week 2: Core Pages Rollout

| Day | Phase | Tasks | Deliverables |
|-----|-------|-------|--------------|
| **Mon 6-7** | Phase 2 | ProxyHosts dialogs, toasts, tests, QA | ✅ ProxyHosts complete |
| **Tue 8** | Phase 3 | SystemSettings, Security | 2 pages complete |
| **Wed 9** | Phase 3 | AccessLists, Certificates | 2 pages complete |
| **Thu 10** | Phase 3 | RemoteServers, Domains | 2 pages complete |
| **Fri 11-12** | Phase 3 | Tests, QA, code reviews, bug fixes | ✅ 6 core pages complete |

**Week 2 Milestones:**

- ✅ ProxyHosts complete (largest component done)
- ✅ 6 additional core pages translated
- ✅ All security-related pages functional

---

### Week 3: Dashboard Integration & Auth

| Day | Phase | Tasks | Deliverables |
|-----|-------|-------|--------------|
| **Mon 13** | Phase 4 | Dashboard, CrowdSec, WAF | Dashboard + 2 config pages |
| **Tue 14** | Phase 4 | Rate Limiting, Uptime, Notifications, Users, Headers | 5 pages complete |
| **Wed 15** | Phase 4 | Integration tests, QA, bug fixes | ✅ All main pages complete |
| **Thu 16** | Phase 5 | Login, Setup, Account | Auth flow complete |
| **Fri 17** | Phase 5 | Auth QA, bug fixes | ✅ Critical auth complete |

**Week 3 Milestones:**

- ✅ Dashboard integrated (validates cross-page consistency)
- ✅ All security and monitoring pages complete
- ✅ Auth and setup flows fully translated

---

### Week 4: Finalization & QA

| Day | Phase | Tasks | Deliverables |
|-----|-------|-------|--------------|
| **Mon 18** | Phase 6 | Backups, Tasks, Logs, Import pages, SMTP | All utility pages |
| **Tue 19** | Phase 6 | Final integration, code reviews | ✅ All pages complete |
| **Wed 20** | Phase 7 | Automated testing, bundle analysis | Test results, metrics |
| **Thu 21** | Phase 7 | Manual QA - core workflows | QA report |
| **Fri 22** | Phase 7 | Edge case testing, bug fixes | Bug list, fixes |
| **Mon 23** | Phase 7 | Final polish, documentation | ✅ Production ready |

**Week 4 Milestones:**

- ✅ 100% of pages translated
- ✅ All automated tests passing
- ✅ All manual QA complete
- ✅ Documentation updated
- ✅ Ready for production deployment

---

### Buffer Time (Optional Week 5)

**Purpose:** Handle unexpected delays, additional bugs, or extended QA

| Day | Tasks |
|-----|-------|
| Mon 24 | Address any remaining P1 bugs |
| Tue 25 | Additional QA if needed |
| Wed 26 | Performance optimization |
| Thu 27 | Stakeholder review |
| Fri 28 | Final production prep |

---

### Daily Stand-up Template

**What was completed yesterday:**

- [Specific pages/components translated]
- [Tests added]
- [Bugs fixed]

**What will be done today:**

- [Specific pages to translate]
- [Tests to add]
- [Code reviews to complete]

**Blockers:**

- [Any issues blocking progress]
- [Missing information or dependencies]

**QA Status:**

- [Pages ready for QA]
- [Bugs found]
- [Bugs fixed]

---

## Code Review Checklist

Use this checklist for EVERY pull request containing translation changes.

### Pre-Review (Author Self-Check)

- [ ] All hardcoded strings replaced with translation keys
- [ ] Translation keys added to ALL 5 language files (en, es, fr, de, zh)
- [ ] Keys follow naming convention (category.subcategory.descriptor)
- [ ] Dynamic content uses interpolation (`{{variableName}}`)
- [ ] Pluralization handled correctly (count, count_plural)
- [ ] Component imports `useTranslation` from 'react-i18next'
- [ ] Component calls `const { t } = useTranslation()` inside function body
- [ ] Tests added/updated for component
- [ ] Manual QA completed in at least 3 languages
- [ ] No console warnings for missing keys
- [ ] No layout breaks or text overflow in any language

### Code Quality

- [ ] **Import statement correct:**

  ```tsx
  import { useTranslation } from 'react-i18next'
  ```

- [ ] **Hook placement correct (inside component):**

  ```tsx
  export default function MyComponent() {
    const { t } = useTranslation()  // ✅ Correct
    // ...
  }
  ```

- [ ] **Translation keys valid (no typos, exist in files):**

  ```tsx
  t('proxyHosts.title')  // ✅ Key exists
  t('proxyhosts.titel')  // ❌ Typo, wrong key
  ```

- [ ] **Interpolation syntax correct:**

  ```tsx
  t('dashboard.activeHosts', { count: 5 })  // ✅ Correct
  t('dashboard.activeHosts', { num: 5 })    // ❌ Variable name mismatch
  ```

- [ ] **No string concatenation:**

  ```tsx
  // ❌ Wrong
  <p>{t('common.total')}: {count}</p>

  // ✅ Correct
  <p>{t('common.totalCount', { count })}</p>
  ```

### Translation File Quality

- [ ] **All 5 files updated (en, es, fr, de, zh)**
- [ ] **Keys in same order in all files**
- [ ] **No duplicate keys**
- [ ] **No missing commas or JSON syntax errors**
- [ ] **Interpolation placeholders match:**

  ```json
  // en
  "activeHosts": "{{count}} active"
  // es (same placeholder name)
  "activeHosts": "{{count}} activos"
  ```

- [ ] **Pluralization implemented if needed:**

  ```json
  "count": "{{count}} item",
  "count_plural": "{{count}} items"
  ```

### Performance

- [ ] **Large components use React.memo:**

  ```tsx
  export default React.memo(ProxyHosts)
  ```

- [ ] **Expensive translation calls memoized:**

  ```tsx
  const columns = useMemo(() => [
    { header: t('common.name'), ... }
  ], [t])
  ```

- [ ] **No unnecessary re-renders on language change**
- [ ] **Bundle size increase documented (if > 5KB)**

### Testing

- [ ] **Unit tests added/updated:**

  ```tsx
  it('renders in Spanish', () => {
    i18n.changeLanguage('es')
    render(<Component />)
    expect(screen.getByText('Panel de Control')).toBeInTheDocument()
  })
  ```

- [ ] **Translation key existence test:**

  ```tsx
  it('all keys exist in all languages', () => {
    const enKeys = Object.keys(en)
    languages.forEach(lang => {
      expect(Object.keys(translations[lang])).toEqual(enKeys)
    })
  })
  ```

- [ ] **Language switching test:**

  ```tsx
  it('updates when language changes', () => {
    const { rerender } = render(<Component />)
    expect(screen.getByText('Dashboard')).toBeInTheDocument()

    i18n.changeLanguage('es')
    rerender(<Component />)
    expect(screen.getByText('Panel de Control')).toBeInTheDocument()
  })
  ```

### Accessibility

- [ ] **ARIA labels translated:**

  ```tsx
  <button aria-label={t('common.close')}>×</button>
  ```

- [ ] **Form labels associated correctly:**

  ```tsx
  <label htmlFor="email">{t('auth.email')}</label>
  <input id="email" name="email" />
  ```

- [ ] **Error messages accessible:**

  ```tsx
  <span role="alert">{t('errors.required')}</span>
  ```

- [ ] **Screen reader tested (if available)**

### UI/UX

- [ ] **No text overflow in any language (especially German)**
- [ ] **Buttons and labels don't break layout**
- [ ] **Proper spacing maintained**
- [ ] **Text direction correct (all LTR for current languages)**
- [ ] **Font rendering acceptable for all languages**

### Edge Cases

- [ ] **Empty states translated:**

  ```tsx
  {items.length === 0 && <p>{t('common.noData')}</p>}
  ```

- [ ] **Error messages translated:**

  ```tsx
  catch (error) {
    toast.error(t('errors.saveFailed'))
  }
  ```

- [ ] **Loading states translated:**

  ```tsx
  {loading && <Spinner>{t('common.loading')}</Spinner>}
  ```

- [ ] **Confirmation dialogs translated:**

  ```tsx
  const confirmed = window.confirm(t('proxyHosts.confirmDelete', { domain }))
  ```

### Documentation

- [ ] **Translation keys documented (if new pattern)**
- [ ] **Component README updated (if applicable)**
- [ ] **PR description includes:**
  - Pages/components updated
  - New translation keys added
  - Manual QA results (languages tested)
  - Screenshots (if UI changes visible)
  - Performance impact (if measurable)

### Final Checks

- [ ] **All tests pass locally**
- [ ] **CI/CD pipeline passes**
- [ ] **No console errors or warnings**
- [ ] **Translation sync check passes**
- [ ] **Manual QA completed in ≥3 languages:**
  - [ ] English (en)
  - [ ] Spanish (es) OR French (fr)
  - [ ] German (de) OR Chinese (zh)

---

## Testing Strategy (Expanded)

### 1. Automated Unit Tests

**Coverage Target:** 90%+ for translation-enabled components

#### Translation Key Existence Tests

```typescript
// frontend/src/__tests__/translation-coverage.test.ts
import { describe, it, expect } from 'vitest'
import enTranslations from '../locales/en/translation.json'
import esTranslations from '../locales/es/translation.json'
import frTranslations from '../locales/fr/translation.json'
import deTranslations from '../locales/de/translation.json'
import zhTranslations from '../locales/zh/translation.json'

function flattenKeys(obj: any, prefix = ''): string[] {
  return Object.keys(obj).reduce((acc: string[], key) => {
    const fullKey = prefix ? `${prefix}.${key}` : key
    if (typeof obj[key] === 'object' && !Array.isArray(obj[key])) {
      return [...acc, ...flattenKeys(obj[key], fullKey)]
    }
    return [...acc, fullKey]
  }, [])
}

describe('Translation Coverage', () => {
  const languages = [
    { name: 'Spanish', code: 'es', translations: esTranslations },
    { name: 'French', code: 'fr', translations: frTranslations },
    { name: 'German', code: 'de', translations: deTranslations },
    { name: 'Chinese', code: 'zh', translations: zhTranslations }
  ]

  const enKeys = flattenKeys(enTranslations)

  languages.forEach(({ name, code, translations }) => {
    it(`all English keys exist in ${name}`, () => {
      const langKeys = flattenKeys(translations)
      const missingKeys = enKeys.filter(key => !langKeys.includes(key))

      expect(missingKeys).toEqual([])
    })

    it(`no extra keys in ${name}`, () => {
      const langKeys = flattenKeys(translations)
      const extraKeys = langKeys.filter(key => !enKeys.includes(key))

      expect(extraKeys).toEqual([])
    })
  })

  it('all files have same number of keys', () => {
    const counts = languages.map(({ translations }) =>
      flattenKeys(translations).length
    )

    counts.forEach(count => {
      expect(count).toBe(enKeys.length)
    })
  })
})
```

#### Component Translation Tests

```typescript
// frontend/src/pages/__tests__/Dashboard.test.tsx
import { render, screen } from '@testing-library/react'
import { describe, it, expect, beforeEach } from 'vitest'
import Dashboard from '../Dashboard'
import i18n from '../../i18n'

describe('Dashboard Translations', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('en')
  })

  it('renders in English by default', () => {
    render(<Dashboard />)
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
    expect(screen.getByText(/overview of your/i)).toBeInTheDocument()
  })

  it('renders in Spanish', async () => {
    await i18n.changeLanguage('es')
    render(<Dashboard />)
    expect(screen.getByText('Panel de Control')).toBeInTheDocument()
  })

  it('renders in French', async () => {
    await i18n.changeLanguage('fr')
    render(<Dashboard />)
    expect(screen.getByText('Tableau de bord')).toBeInTheDocument()
  })

  it('renders in German', async () => {
    await i18n.changeLanguage('de')
    render(<Dashboard />)
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
  })

  it('renders in Chinese', async () => {
    await i18n.changeLanguage('zh')
    render(<Dashboard />)
    expect(screen.getByText('仪表板')).toBeInTheDocument()
  })

  it('updates when language changes', async () => {
    const { rerender } = render(<Dashboard />)
    expect(screen.getByText('Dashboard')).toBeInTheDocument()

    await i18n.changeLanguage('es')
    rerender(<Dashboard />)

    expect(screen.queryByText('Dashboard')).not.toBeInTheDocument()
    expect(screen.getByText('Panel de Control')).toBeInTheDocument()
  })
})
```

#### Dynamic Content Translation Tests

```typescript
// frontend/src/pages/__tests__/ProxyHosts.test.tsx
it('handles plural translations correctly', async () => {
  await i18n.changeLanguage('en')
  render(<ProxyHosts />)

  // Mock data with 1 item
  expect(screen.getByText('1 active')).toBeInTheDocument()

  // Mock data with 5 items
  expect(screen.getByText('5 active')).toBeInTheDocument()
})

it('interpolates variables correctly', async () => {
  await i18n.changeLanguage('en')
  render(<ProxyHostDelete domain="example.com" />)

  expect(screen.getByText(/delete example\.com/i)).toBeInTheDocument()
})
```

---

### 2. Manual QA Testing

#### Per-Component QA Checklist

For each component/page:

- [ ] **Visual Inspection:**
  - [ ] Text renders correctly in all 5 languages
  - [ ] No text overflow or truncation
  - [ ] Buttons don't break layout
  - [ ] Proper spacing maintained

- [ ] **Functional Testing:**
  - [ ] All buttons clickable
  - [ ] Forms submit correctly
  - [ ] Validation messages display
  - [ ] Error/success toasts appear
  - [ ] Dialogs open/close properly

- [ ] **Language Switching:**
  - [ ] Switch to each language from selector
  - [ ] UI updates immediately
  - [ ] No console errors
  - [ ] Selection persists on reload

- [ ] **Dynamic Content:**
  - [ ] Numbers format correctly
  - [ ] Dates display in proper format
  - [ ] Plurals work correctly
  - [ ] Variable interpolation works

#### Language-Specific Testing

**German Testing (Longest Strings):**

- Focus on buttons and labels that may overflow
- Check table headers don't wrap awkwardly
- Verify no horizontal scrolling triggered

**Chinese Testing (Character Width):**

- Ensure proper font rendering
- Check spacing between characters
- Verify no character clipping

---

### 3. Edge Case Testing

#### Mid-Edit Language Changes

**Test Scenario:** User is filling out a form, switches language mid-edit

```typescript
// frontend/src/pages/__tests__/ProxyHosts.edge-cases.test.tsx
it('preserves form data when language changes', async () => {
  render(<ProxyHostForm />)

  // Fill form in English
  const domainInput = screen.getByLabelText('Domain Names')
  await userEvent.type(domainInput, 'example.com')

  // Switch to Spanish
  await i18n.changeLanguage('es')

  // Verify data still there
  expect(domainInput).toHaveValue('example.com')

  // Verify label changed
  expect(screen.getByLabelText('Nombres de Dominio')).toBeInTheDocument()
})
```

**Expected Behavior:**

- Form data preserved
- Labels update to new language
- Validation messages in new language
- No data loss

---

#### Backend Error Handling

**Test Scenario:** API returns error while UI is in non-English language

```typescript
it('displays backend errors in current language', async () => {
  await i18n.changeLanguage('es')

  // Mock API error
  server.use(
    http.post('/api/proxy-hosts', () => {
      return HttpResponse.json(
        { error: 'Invalid domain' },
        { status: 400 }
      )
    })
  )

  render(<ProxyHostForm />)
  const submitButton = screen.getByText('Guardar')
  await userEvent.click(submitButton)

  // Error toast should be in Spanish
  await waitFor(() => {
    expect(screen.getByText(/error al guardar/i)).toBeInTheDocument()
  })
})
```

**Expected Behavior:**

- API errors trigger translated error messages
- Toast notifications in current language
- Console logging in English (for debugging)

---

#### WebSocket Reconnection

**Test Scenario:** WebSocket disconnects/reconnects while viewing logs in non-English

```typescript
it('handles WebSocket reconnection with translations', async () => {
  await i18n.changeLanguage('fr')
  render(<LogsPage />)

  // Verify initial state
  expect(screen.getByText('Connecté')).toBeInTheDocument()

  // Simulate disconnect
  mockWebSocket.close()

  await waitFor(() => {
    expect(screen.getByText('Déconnecté')).toBeInTheDocument()
  })

  // Simulate reconnect
  mockWebSocket.open()

  await waitFor(() => {
    expect(screen.getByText('Connecté')).toBeInTheDocument()
  })
})
```

**Expected Behavior:**

- Connection status messages translated
- Log messages display correctly after reconnect
- No data loss during reconnection

---

#### Rapid Language Switching

**Test Scenario:** User rapidly clicks through all 5 languages

```typescript
it('handles rapid language switching without errors', async () => {
  const { rerender } = render(<Layout />)

  const languages = ['en', 'es', 'fr', 'de', 'zh']

  for (const lang of languages) {
    await i18n.changeLanguage(lang)
    rerender(<Layout />)

    // Should not throw errors
    expect(screen.getByRole('navigation')).toBeInTheDocument()
  }

  // No console errors
  expect(console.error).not.toHaveBeenCalled()
})
```

**Expected Behavior:**

- No race conditions
- UI updates cleanly each time
- No console errors or warnings
- Final language selection persists

---

### 4. Accessibility Testing

#### Screen Reader Compatibility

**Manual Test Steps:**

1. Enable screen reader (NVDA on Windows, VoiceOver on Mac)
2. Navigate application using keyboard only
3. Verify announcements in selected language

**Test Checklist:**

- [ ] Page titles announced in correct language
- [ ] Button labels read correctly
- [ ] Form labels associated properly
- [ ] Error messages announced
- [ ] ARIA labels translated
- [ ] Live regions update with translations

**Example Test:**

```typescript
it('has accessible labels in all languages', async () => {
  await i18n.changeLanguage('es')
  render(<ProxyHostForm />)

  const closeButton = screen.getByRole('button', { name: 'Cerrar' })
  expect(closeButton).toHaveAttribute('aria-label', 'Cerrar')
})
```

---

### 5. Performance Testing

#### Bundle Size Analysis

```bash
# Before implementation
npm run build
ls -lh dist/assets/*.js

# After implementation
npm run build
ls -lh dist/assets/*.js

# Calculate increase
```

**Acceptance Criteria:**

- Bundle size increase < 50KB (compressed)
- Translation files lazy-loaded per language
- Only active language loaded initially

**Tool:** `webpack-bundle-analyzer` or `rollup-plugin-visualizer`

```bash
npm run build -- --analyze
```

---

#### Language Switch Performance

**Test Script:**

```typescript
// frontend/src/__tests__/performance.test.ts
it('switches language in under 500ms', async () => {
  render(<App />)

  const start = performance.now()
  await i18n.changeLanguage('es')
  const duration = performance.now() - start

  expect(duration).toBeLessThan(500)
})
```

**Manual Test:**

1. Open DevTools → Performance
2. Start recording
3. Click language selector
4. Select different language
5. Stop recording
6. Analyze flame graph

**Acceptance Criteria:**

- Desktop: < 500ms total switch time
- Mobile: < 1000ms total switch time
- No visible UI freezing
- No layout thrashing

---

#### Memory Profiling

**Test Procedure:**

1. Open DevTools → Memory
2. Take heap snapshot (baseline)
3. Switch languages 10 times
4. Take another heap snapshot
5. Compare memory usage

**Acceptance Criteria:**

- Memory increase < 10% after 10 switches
- No detached DOM nodes
- No event listener leaks

---

### 6. Fallback Behavior Testing

#### Missing Translation Keys

**Test Scenario:** A translation key is missing in one language

```typescript
// In es/translation.json, remove a key
// {
//   "proxyHosts": {
//     "title": "Proxy Hosts"
//     // "description": "..." <- Missing
//   }
// }

it('falls back to English for missing keys', async () => {
  await i18n.changeLanguage('es')
  render(<ProxyHosts />)

  // Title should be in Spanish
  expect(screen.getByText('Hosts de Proxy')).toBeInTheDocument()

  // Description falls back to English
  expect(screen.getByText('Manage your reverse proxy configurations')).toBeInTheDocument()

  // Warning logged to console
  expect(console.warn).toHaveBeenCalledWith(
    expect.stringContaining('Missing translation key: proxyHosts.description')
  )
})
```

**Expected Behavior:**

- Falls back to English gracefully
- Console warning in development
- Error reported to monitoring in production
- UI remains functional

---

#### Corrupted LocalStorage

**Test Scenario:** localStorage has invalid language value

```typescript
it('handles corrupted language preference', () => {
  localStorage.setItem('charon-language', 'invalid-lang')

  render(<App />)

  // Should fall back to browser default or 'en'
  expect(i18n.language).toBe('en')
})

it('handles missing localStorage', () => {
  // Simulate localStorage unavailable
  const { localStorage: originalStorage } = window
  Object.defineProperty(window, 'localStorage', {
    get: () => { throw new Error('localStorage unavailable') }
  })

  render(<App />)

  // Should use browser language or default to 'en'
  expect(i18n.language).toMatch(/en|es|fr|de|zh/)

  // Restore
  Object.defineProperty(window, 'localStorage', {
    get: () => originalStorage
  })
})
```

**Expected Behavior:**

- Graceful fallback to browser default
- No application crashes
- Language can still be changed manually

---

### 7. Regression Testing

#### Existing Functionality

**Test Checklist:**

- [ ] All existing unit tests still pass
- [ ] All existing integration tests still pass
- [ ] No broken API calls
- [ ] No broken WebSocket connections
- [ ] All forms submit correctly
- [ ] All CRUD operations work
- [ ] Authentication still works
- [ ] Authorization checks still work

**Automated Regression Suite:**

```bash
npm run test:unit
npm run test:integration
npm run test:e2e
```

---

### 8. Cross-Browser Testing

**Browsers to Test:**

- Chrome (latest)
- Firefox (latest)
- Safari (latest)
- Edge (latest)
- Mobile Safari (iOS)
- Mobile Chrome (Android)

**Per-Browser Checklist:**

- [ ] Language selector works
- [ ] Translations render correctly
- [ ] localStorage persistence works
- [ ] No console errors
- [ ] Performance acceptable

---

## Success Metrics & Verification

### 1. Translation Coverage Metrics

**Measurement Method:** Automated script

```bash
# scripts/check-translation-coverage.sh
#!/bin/bash
set -e

echo "Checking translation coverage..."

# Run coverage test
npm run test:coverage:i18n

# Check for hardcoded strings
echo "Searching for hardcoded strings..."
node scripts/find-hardcoded-strings.js

echo "✅ Translation coverage check complete"
```

```javascript
// scripts/find-hardcoded-strings.js
const fs = require('fs')
const path = require('path')
const glob = require('glob')

const componentFiles = glob.sync('frontend/src/{pages,components}/**/*.tsx')
const violations = []

componentFiles.forEach(file => {
  const content = fs.readFileSync(file, 'utf8')

  // Check for common hardcoded patterns
  const patterns = [
    /<button[^>]*>([A-Z][a-z]+\s?)+<\/button>/g,  // <button>Save</button>
    /title="([A-Z][a-z]+\s?)+"/g,                 // title="Dashboard"
    /placeholder="([A-Z][a-z]+\s?)+"/g            // placeholder="Enter name"
  ]

  patterns.forEach(pattern => {
    const matches = content.match(pattern)
    if (matches) {
      violations.push({
        file,
        matches: matches.slice(0, 3) // First 3 matches
      })
    }
  })
})

if (violations.length > 0) {
  console.error('❌ Found hardcoded strings:')
  violations.forEach(({ file, matches }) => {
    console.error(`  ${file}:`)
    matches.forEach(m => console.error(`    - ${m}`))
  })
  process.exit(1)
} else {
  console.log('✅ No hardcoded strings found')
}
```

**Acceptance Criteria:**

- ✅ 100% of components use `useTranslation` hook
- ✅ 0 hardcoded display strings (script finds none)
- ✅ All 132+ translation keys exist in all 5 languages
- ✅ No missing key warnings in console

**Verification:**

```bash
npm run check:translations
```

---

### 2. Functional Verification

**Measurement Method:** Manual QA + automated tests

#### Language Switching Test

```typescript
// Automated test
it('language selection persists across sessions', () => {
  render(<App />)

  // Select Spanish
  selectLanguage('es')
  expect(localStorage.getItem('charon-language')).toBe('es')

  // Reload page
  window.location.reload()

  // Should still be Spanish
  expect(i18n.language).toBe('es')
  expect(screen.getByText('Panel de Control')).toBeInTheDocument()
})
```

**Manual Verification:**

- [ ] Open app in incognito mode
- [ ] Select Spanish from language selector
- [ ] Verify UI switches to Spanish
- [ ] Close browser
- [ ] Reopen same URL
- [ ] Verify still in Spanish

**Acceptance Criteria:**

- ✅ Language selector immediately updates UI (< 500ms)
- ✅ Selection persists in localStorage
- ✅ Persists across browser restarts
- ✅ Works in all 5 languages

**Acceptable Miss Rate:** 0% (must work perfectly)

---

### 3. Visual Regression Testing

**Measurement Method:** Visual diff screenshots

```javascript
// visual-regression.spec.ts (Playwright)
import { test, expect } from '@playwright/test'

const languages = ['en', 'es', 'fr', 'de', 'zh']
const pages = ['/', '/proxy-hosts', '/security', '/dashboard']

languages.forEach(lang => {
  pages.forEach(page => {
    test(`${page} in ${lang} matches snapshot`, async ({ page: pw }) => {
      await pw.goto(`http://localhost:3000${page}`)
      await pw.evaluate((language) => {
        localStorage.setItem('charon-language', language)
        window.location.reload()
      }, lang)

      await pw.waitForLoadState('networkidle')

      // Take screenshot
      await expect(pw).toHaveScreenshot(`${page.replace('/', '')}-${lang}.png`)
    })
  })
})
```

**Acceptance Criteria:**

- ✅ No layout breaks in any language
- ✅ No text overflow (especially German)
- ✅ No horizontal scrollbars introduced
- ✅ Proper character rendering (especially Chinese)
- ✅ Button/label widths accommodate text

**Acceptable Miss Rate:** < 5% pixel difference (accounting for font rendering)

---

### 4. Performance Metrics

**Measurement Method:** Performance API + Lighthouse

#### Language Switch Speed

```typescript
// performance.test.ts
it('language switch completes under 500ms', async () => {
  render(<App />)

  const measurements = []

  for (let i = 0; i < 10; i++) {
    const start = performance.now()
    await i18n.changeLanguage('es')
    await waitFor(() =>
      expect(screen.getByText('Panel de Control')).toBeInTheDocument()
    )
    const duration = performance.now() - start
    measurements.push(duration)

    await i18n.changeLanguage('en')
  }

  const avg = measurements.reduce((a, b) => a + b) / measurements.length
  const max = Math.max(...measurements)

  console.log(`Avg: ${avg}ms, Max: ${max}ms`)

  expect(avg).toBeLessThan(500)
  expect(max).toBeLessThan(1000)
})
```

**Manual Measurement:**

1. Open DevTools → Performance
2. Start recording
3. Click language selector → Select Spanish
4. Stop recording
5. Measure time from click to UI update complete

**Acceptance Criteria:**

- ✅ Average switch time < 500ms (desktop)
- ✅ Average switch time < 1000ms (mobile)
- ✅ 95th percentile < 800ms (desktop)
- ✅ No visible UI freezing

**Target:** ✅ P50: 200ms, P95: 500ms, P99: 800ms

---

#### Bundle Size Impact

```bash
# Before implementation
npm run build
du -sh dist/assets/*.js

# After implementation
npm run build
du -sh dist/assets/*.js
```

**Measurement:**

```bash
# Get bundle size report
npm run build -- --analyze

# Check specific assets
ls -lh dist/assets/index-*.js
ls -lh dist/assets/translation-*.js
```

**Acceptance Criteria:**

- ✅ Main bundle increase < 20KB (gzipped)
- ✅ Translation files lazy-loaded per language
- ✅ Only active language loaded on page load
- ✅ Total bundle size < 500KB (gzipped)

**Target:**

- Main bundle increase: < 15KB
- Per-language file: < 5KB each
- Total increase: < 40KB (all languages)

---

### 5. Accessibility Metrics

**Measurement Method:** axe-core automated testing + manual screen reader testing

```typescript
// accessibility.test.ts
import { axe, toHaveNoViolations } from 'jest-axe'
expect.extend(toHaveNoViolations)

it('has no accessibility violations in all languages', async () => {
  const languages = ['en', 'es', 'fr', 'de', 'zh']

  for (const lang of languages) {
    await i18n.changeLanguage(lang)
    const { container } = render(<App />)
    const results = await axe(container)

    expect(results).toHaveNoViolations()
  }
})
```

**Manual Verification (Screen Reader):**

- [ ] Navigate with keyboard only (Tab, Enter, Space)
- [ ] Enable screen reader (NVDA/VoiceOver)
- [ ] Verify announcements in selected language
- [ ] Check ARIA labels translated
- [ ] Verify form labels associated correctly
- [ ] Test error announcements

**Acceptance Criteria:**

- ✅ 0 critical accessibility violations
- ✅ All ARIA labels translated
- ✅ Screen reader announces in correct language
- ✅ Keyboard navigation works in all languages

**Acceptable Miss Rate:** 0% (accessibility is non-negotiable)

---

### 6. Error Rate Metrics

**Measurement Method:** Error tracking + console monitoring

```typescript
// Setup error tracking
i18n.init({
  fallbackLng: 'en',
  missingKeyHandler: (lngs, ns, key, fallbackValue) => {
    // Log to error tracking service
    console.error(`Missing i18n key: ${key} for language: ${lngs[0]}`)

    // Report to Sentry/monitoring
    if (process.env.NODE_ENV === 'production') {
      reportError({
        type: 'missing_translation',
        key,
        language: lngs[0],
        fallbackUsed: fallbackValue
      })
    }
  }
})
```

**Monitoring Dashboard:**

- Missing translation key count (by language)
- Translation load failures
- Language switch errors
- LocalStorage read/write errors

**Acceptance Criteria:**

- ✅ 0 missing translation keys in production
- ✅ < 0.1% translation load failure rate
- ✅ 0 language switch errors
- ✅ All errors gracefully handled with fallback

**Acceptable Miss Rate:** < 0.1% (in production, due to edge cases)

---

### 7. User Experience Metrics

**Measurement Method:** User testing + analytics

#### Task Completion Rate

**Test:** Ask 5 users to complete key workflows in their non-native language

**Tasks:**

1. Change language to Spanish
2. Create a proxy host
3. Configure SSL
4. Apply access list
5. View logs

**Measurement:**

- Task completion rate (% of users who succeed)
- Time to complete each task
- Number of errors/retries
- User satisfaction score (1-5)

**Acceptance Criteria:**

- ✅ Task completion rate > 90%
- ✅ Time to complete similar to English (±20%)
- ✅ User satisfaction score ≥ 4/5
- ✅ No blocker issues reported

---

### 8. Regression Testing Metrics

**Measurement Method:** Automated test suite

```bash
# Run full regression suite
npm run test:unit
npm run test:integration
npm run test:e2e

# Check for failures
echo "Exit code: $?"
```

**Acceptance Criteria:**

- ✅ 100% of existing unit tests pass
- ✅ 100% of existing integration tests pass
- ✅ 100% of existing e2e tests pass
- ✅ No new console errors introduced
- ✅ All API calls still work
- ✅ WebSocket connections still function

**Acceptable Miss Rate:** 0% (no regressions allowed)

---

## Translation Maintenance Strategy

### 1. CI/CD Integration

#### GitHub Actions Workflow

```yaml
# .github/workflows/i18n-check.yml
name: Translation Coverage Check

on:
  pull_request:
    paths:
      - 'frontend/src/locales/**'
      - 'frontend/src/**/*.tsx'
      - 'frontend/src/**/*.ts'

jobs:
  check-translations:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup Node
        uses: actions/setup-node@v3
        with:
          node-version: '18'

      - name: Install dependencies
        run: cd frontend && npm ci

      - name: Check translation key sync
        run: npm run check:i18n:sync

      - name: Check for hardcoded strings
        run: npm run check:i18n:hardcoded

      - name: Run translation tests
        run: npm run test:i18n

      - name: Comment PR with results
        if: always()
        uses: actions/github-script@v6
        with:
          script: |
            const fs = require('fs')
            const report = fs.readFileSync('i18n-report.txt', 'utf8')
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: `## Translation Check Results\n\n${report}`
            })
```

#### Pre-commit Hook

```bash
# .git/hooks/pre-commit (or via husky)
#!/bin/bash

echo "Running translation checks..."

# Check if any translation files changed
TRANSLATION_FILES=$(git diff --cached --name-only | grep 'locales/')

if [ -n "$TRANSLATION_FILES" ]; then
  echo "Translation files changed. Running sync check..."
  npm run check:i18n:sync || exit 1
fi

# Check if any component files changed
COMPONENT_FILES=$(git diff --cached --name-only | grep -E '\.(tsx|ts)$')

if [ -n "$COMPONENT_FILES" ]; then
  echo "Component files changed. Checking for hardcoded strings..."
  npm run check:i18n:hardcoded || exit 1
fi

echo "✅ Translation checks passed"
```

---

### 2. Developer Workflow

#### Adding New Translation Keys

**Step-by-Step Process:**

1. **Add key to English first:**

   ```json
   // frontend/src/locales/en/translation.json
   {
     "proxyHosts": {
       "newFeature": "New Feature Button"
     }
   }
   ```

2. **Run sync script:**

   ```bash
   npm run i18n:sync
   ```

   This automatically adds the key to all other language files with `[TODO]` marker:

   ```json
   // frontend/src/locales/es/translation.json
   {
     "proxyHosts": {
       "newFeature": "[TODO] New Feature Button"
     }
   }
   ```

3. **Use in component:**

   ```tsx
   <Button>{t('proxyHosts.newFeature')}</Button>
   ```

4. **Create PR with translation TODO:**
   - PR title includes `[i18n]` tag
   - PR description lists untranslated keys
   - Assign to translation team for review

5. **Translation team updates:**
   - Replace `[TODO]` with proper translations
   - Test in UI
   - Approve PR

---

#### Sync Script Implementation

```javascript
// scripts/sync-translation-keys.js
const fs = require('fs')
const path = require('path')

const localesDir = path.join(__dirname, '../frontend/src/locales')
const languages = ['es', 'fr', 'de', 'zh']

// Read English as source of truth
const enFile = path.join(localesDir, 'en/translation.json')
const enKeys = JSON.parse(fs.readFileSync(enFile, 'utf8'))

function flattenKeys(obj, prefix = '') {
  return Object.keys(obj).reduce((acc, key) => {
    const fullKey = prefix ? `${prefix}.${key}` : key
    if (typeof obj[key] === 'object' && !Array.isArray(obj[key])) {
      return { ...acc, ...flattenKeys(obj[key], fullKey) }
    }
    return { ...acc, [fullKey]: obj[key] }
  }, {})
}

function unflattenKeys(flat) {
  const result = {}
  Object.keys(flat).forEach(key => {
    const parts = key.split('.')
    let current = result
    parts.forEach((part, i) => {
      if (i === parts.length - 1) {
        current[part] = flat[key]
      } else {
        current[part] = current[part] || {}
        current = current[part]
      }
    })
  })
  return result
}

const flatEnKeys = flattenKeys(enKeys)

languages.forEach(lang => {
  const langFile = path.join(localesDir, `${lang}/translation.json`)
  const langKeys = JSON.parse(fs.readFileSync(langFile, 'utf8'))
  const flatLangKeys = flattenKeys(langKeys)

  let updated = false

  // Add missing keys with [TODO] marker
  Object.keys(flatEnKeys).forEach(key => {
    if (!flatLangKeys[key]) {
      flatLangKeys[key] = `[TODO] ${flatEnKeys[key]}`
      updated = true
      console.log(`Added to ${lang}: ${key}`)
    }
  })

  // Remove extra keys not in English
  Object.keys(flatLangKeys).forEach(key => {
    if (!flatEnKeys[key]) {
      delete flatLangKeys[key]
      updated = true
      console.log(`Removed from ${lang}: ${key}`)
    }
  })

  if (updated) {
    const unflattened = unflattenKeys(flatLangKeys)
    fs.writeFileSync(
      langFile,
      JSON.stringify(unflattened, null, 2) + '\n'
    )
    console.log(`✅ Updated ${lang}/translation.json`)
  } else {
    console.log(`✅ ${lang}/translation.json is up to date`)
  }
})

// Check for [TODO] markers
const allTodos = []
languages.forEach(lang => {
  const langFile = path.join(localesDir, `${lang}/translation.json`)
  const content = fs.readFileSync(langFile, 'utf8')
  const todos = content.match(/\[TODO\]/g)
  if (todos) {
    allTodos.push({ lang, count: todos.length })
  }
})

if (allTodos.length > 0) {
  console.warn('\n⚠️  Pending translations:')
  allTodos.forEach(({ lang, count }) => {
    console.warn(`  ${lang}: ${count} keys need translation`)
  })
  process.exit(1)
}

console.log('\n✅ All translation files in sync!')
```

**NPM Scripts:**

```json
{
  "scripts": {
    "i18n:sync": "node scripts/sync-translation-keys.js",
    "i18n:check": "node scripts/check-translation-sync.js",
    "check:i18n:sync": "npm run i18n:check",
    "check:i18n:hardcoded": "node scripts/find-hardcoded-strings.js",
    "test:i18n": "vitest run --testPathPattern=translation"
  }
}
```

---

### 3. Ownership Model

**Translation Team Structure:**

| Role | Responsibility | Languages |
|------|----------------|-----------|
| **Lead Developer** | English source, review all PRs | en |
| **Spanish Translator** | Maintain Spanish translations | es |
| **French Translator** | Maintain French translations | fr |
| **German Translator** | Maintain German translations | de |
| **Chinese Translator** | Maintain Chinese translations | zh |
| **QA Team** | Verify translations in context | all |

**Responsibility Matrix:**

| Task | Owner | Backup |
|------|-------|--------|
| Add new English keys | Feature developer | Lead developer |
| Translate to Spanish | Spanish translator | Community |
| Translate to French | French translator | Community |
| Translate to German | German translator | Community |
| Translate to Chinese | Chinese translator | Community |
| Review translations | QA team | Lead developer |
| Approve PR | Lead developer | Tech lead |

**Escalation Path:**

1. Developer adds English key in feature PR
2. CI flags missing translations
3. PR merged with `[TODO]` markers
4. Translation issue created automatically
5. Assigned to translator for language
6. Translator updates and creates PR
7. QA verifies in UI
8. Lead developer merges

---

### 4. Community Contributions

**Guidelines for contributors:**

1. **New Feature PRs:**
   - Must include English translation keys
   - May include translations for other languages (optional)
   - CI will flag missing translations
   - OK to merge with `[TODO]` markers

2. **Translation-Only PRs:**
   - Must update ALL specified languages
   - Must test in UI (screenshots required)
   - Must follow naming conventions
   - Must pass all CI checks

3. **Documentation:**

   ```markdown
   ## Contributing Translations

   We welcome translation contributions! Please follow these steps:

   1. Fork the repository
   2. Find keys marked `[TODO]` in `frontend/src/locales/{lang}/translation.json`
   3. Replace with accurate translations
   4. Test in UI by changing language
   5. Take screenshots
   6. Create PR with:
      - Title: `[i18n] Update {language} translations`
      - Description: List of keys updated
      - Screenshots showing translations in UI
   7. Wait for review from language maintainer

   ### Translation Guidelines
   - Keep formatting placeholders: `{{variable}}`
   - Maintain similar length to English (±30%)
   - Use formal tone
   - Test pluralization if applicable
   ```

---

### 5. Translation Quality Assurance

#### Review Checklist

For each translation PR:

- [ ] **Accuracy:**
  - [ ] Translation conveys same meaning as English
  - [ ] Technical terms translated appropriately
  - [ ] No mistranslations or ambiguities

- [ ] **Consistency:**
  - [ ] Terms translated consistently across all keys
  - [ ] Follows existing patterns in language file
  - [ ] Matches style guide for language

- [ ] **Formatting:**
  - [ ] Placeholders preserved: `{{count}}`, `{{domain}}`
  - [ ] Punctuation appropriate for language
  - [ ] Capitalization follows language rules

- [ ] **Length:**
  - [ ] Fits in UI (no overflow)
  - [ ] Not excessively longer than English
  - [ ] Acceptable truncation if needed

- [ ] **Context:**
  - [ ] Makes sense in UI context
  - [ ] Appropriate formality level
  - [ ] Tested in actual application

#### Automated Quality Checks

```javascript
// scripts/check-translation-quality.js
const checks = {
  placeholders: (en, translated) => {
    const enPlaceholders = (en.match(/{{[^}]+}}/g) || []).sort()
    const transPlaceholders = (translated.match(/{{[^}]+}}/g) || []).sort()
    return JSON.stringify(enPlaceholders) === JSON.stringify(transPlaceholders)
  },

  length: (en, translated) => {
    const ratio = translated.length / en.length
    return ratio >= 0.5 && ratio <= 2.0  // Within 50-200% of English length
  },

  noTodoMarkers: (translated) => {
    return !translated.includes('[TODO]')
  },

  noHtmlTags: (translated) => {
    return !/<[^>]+>/.test(translated)
  }
}

// Run checks on all translations
languages.forEach(lang => {
  // ... check each key against English
})
```

---

## Rollback Strategy & Feature Flags

### 1. Feature Flag Implementation

**Approach:** Use environment variable + React Context to enable/disable i18n feature

#### Implementation

```typescript
// frontend/src/config/featureFlags.ts
export const featureFlags = {
  enableI18n: import.meta.env.VITE_ENABLE_I18N === 'true' || false
}

// Can be overridden at runtime for testing
if (typeof window !== 'undefined') {
  (window as any).__FEATURE_FLAGS__ = featureFlags
}
```

```typescript
// frontend/src/context/FeatureFlagContext.tsx
import { createContext, useContext, ReactNode } from 'react'
import { featureFlags } from '../config/featureFlags'

const FeatureFlagContext = createContext(featureFlags)

export const useFeatureFlags = () => useContext(FeatureFlagContext)

export function FeatureFlagProvider({ children }: { children: ReactNode }) {
  return (
    <FeatureFlagContext.Provider value={featureFlags}>
      {children}
    </FeatureFlagContext.Provider>
  )
}
```

#### Usage in Components

```tsx
// frontend/src/components/Layout.tsx
import { useTranslation } from 'react-i18next'
import { useFeatureFlags } from '../context/FeatureFlagContext'

export default function Layout() {
  const { t } = useTranslation()
  const { enableI18n } = useFeatureFlags()

  const navigation = [
    {
      name: enableI18n ? t('navigation.dashboard') : 'Dashboard',
      path: '/',
      icon: '📊'
    },
    {
      name: enableI18n ? t('navigation.proxyHosts') : 'Proxy Hosts',
      path: '/proxy-hosts',
      icon: '🌐'
    }
  ]

  return <nav>{/* ... */}</nav>
}
```

#### Environment Configuration

```bash
# .env.development
VITE_ENABLE_I18N=true

# .env.production (initially false, enable after validation)
VITE_ENABLE_I18N=false

# .env.staging (test with flag enabled)
VITE_ENABLE_I18N=true
```

---

### 2. Phased Rollout Plan

**Strategy:** Gradual rollout with monitoring at each stage

#### Stage 1: Internal Testing (Week 1)

- **Target:** Development team + QA team
- **Flag:** Enabled in development environment only
- **Monitoring:**
  - Console errors
  - Translation key misses
  - Performance metrics
  - User feedback from team

**Go/No-Go Criteria:**

- ✅ No critical bugs
- ✅ All translation keys work
- ✅ Performance acceptable
- ✅ Team approves UX

**Rollback Trigger:**

- Critical bug that blocks usage
- Performance degradation > 30%
- > 5% missing translation keys

---

#### Stage 2: Beta Users (Week 2-3)

- **Target:** 10% of production users (opt-in beta program)
- **Flag:** Enabled via URL parameter or localStorage override
- **Monitoring:**
  - Error rate per language
  - Language switch frequency
  - Task completion rate
  - User feedback surveys

**Enable Method:**

```typescript
// Allow beta users to enable via URL
const urlParams = new URLSearchParams(window.location.search)
if (urlParams.get('beta_i18n') === 'true') {
  localStorage.setItem('beta_i18n_enabled', 'true')
}

const betaEnabled = localStorage.getItem('beta_i18n_enabled') === 'true'

export const featureFlags = {
  enableI18n: import.meta.env.VITE_ENABLE_I18N === 'true' || betaEnabled
}
```

**Go/No-Go Criteria:**

- ✅ Error rate < 1%
- ✅ > 80% positive user feedback
- ✅ No P0/P1 bugs
- ✅ Performance within targets

**Rollback Trigger:**

- Error rate > 5%
- < 60% positive feedback
- Critical bug discovered
- Performance degradation

---

#### Stage 3: Gradual Rollout (Week 3-4)

- **Target:** Gradual increase to 100% of users
- **Flag:** Percentage-based rollout (10% → 25% → 50% → 100%)
- **Monitoring:**
  - Real-time error dashboards
  - Performance metrics
  - User support tickets
  - Social media/reviews

**Rollout Schedule:**

| Day | Percentage | Monitoring Period |
|-----|-----------|-------------------|
| 1 | 10% | 24 hours |
| 2 | 25% | 24 hours |
| 4 | 50% | 48 hours |
| 7 | 100% | Ongoing |

**Implementation:**

```typescript
// Server-side or edge function
function getUserRolloutPercentage(userId: string): boolean {
  const hash = simpleHash(userId)
  const rolloutPercentage = getCurrentRolloutPercentage() // e.g., 50
  return (hash % 100) < rolloutPercentage
}

// Client-side
const userId = getCurrentUserId()
const isInRollout = await checkRolloutStatus(userId)

export const featureFlags = {
  enableI18n: isInRollout || import.meta.env.VITE_ENABLE_I18N === 'true'
}
```

**Go/No-Go Criteria (Each Stage):**

- ✅ Error rate stable or decreasing
- ✅ No increase in support tickets
- ✅ Performance stable
- ✅ No critical bugs

**Rollback Trigger:**

- Error rate spike > 10%
- Critical bug affecting > 1% users
- Support ticket surge
- Performance degradation > 20%

---

### 3. Rollback Procedure

**Scenario:** Critical issue discovered in production requiring immediate rollback

#### Immediate Rollback (< 5 minutes)

```bash
# 1. Disable feature flag via environment variable
# Update .env.production or config service
VITE_ENABLE_I18N=false

# 2. Redeploy with flag disabled
npm run build
npm run deploy:production

# OR use CDN config to inject flag override
# (if build artifacts are cached)

# 3. Clear CDN cache if necessary
curl -X PURGE https://cdn.example.com/assets/*

# 4. Monitor error rates drop
watch -n 5 'curl -s https://api.example.com/metrics/errors'
```

**Expected Result:**

- All users revert to hardcoded English strings
- Translation infrastructure remains in place
- No data loss or state corruption
- Immediate error rate drop

---

#### Gradual Rollback

**Use Case:** Non-critical issue but want to reduce exposure

```typescript
// Reduce rollout percentage gradually
// 100% → 50% → 25% → 10% → 0%

function getRolloutPercentage() {
  // Fetch from config service or feature flag platform
  return configService.get('i18n_rollout_percentage')
}

// Decrease by 25% every hour until stable
```

---

#### Component-Specific Rollback

**Use Case:** Issue isolated to specific component (e.g., ProxyHosts)

```tsx
// Temporarily disable i18n for specific component
import { useFeatureFlags } from '../context/FeatureFlagContext'

export default function ProxyHosts() {
  const { t } = useTranslation()
  const { enableI18n } = useFeatureFlags()

  // Force disable for this component only
  const useI18n = enableI18n && !isProxyHostsBugActive()

  return (
    <PageShell title={useI18n ? t('proxyHosts.title') : 'Proxy Hosts'}>
      {/* ... */}
    </PageShell>
  )
}
```

---

### 4. Rollback Decision Matrix

| Issue Severity | Impact | Action | Timeline |
|---------------|--------|--------|----------|
| **P0 - Critical** | > 10% users can't use app | Immediate full rollback | < 5 min |
| **P1 - High** | Core feature broken for > 5% | Gradual rollback to 0% | < 1 hour |
| **P2 - Medium** | Non-critical feature broken | Component-specific disable | < 4 hours |
| **P3 - Low** | Minor visual issue | Fix forward, no rollback | Next release |

**Examples:**

- **P0:** Language switch crashes app → Immediate rollback
- **P1:** German translations cause layout breaks → Gradual rollback
- **P2:** Toast messages not translated → Disable toasts i18n only
- **P3:** Minor spacing issue in Chinese → Fix in next sprint

---

### 5. Monitoring & Alerting

#### Key Metrics to Monitor

```typescript
// Error tracking
const i18nMetrics = {
  missingKeys: 0,           // Translation key not found
  loadFailures: 0,          // Translation file failed to load
  switchErrors: 0,          // Error during language switch
  renderErrors: 0,          // Component render error after i18n
  performanceSlow: 0        // Language switch > 1000ms
}

// Alert thresholds
const alertThresholds = {
  missingKeys: 10,          // Alert if > 10 missing keys in 5 min
  loadFailures: 5,          // Alert if > 5 load failures in 5 min
  switchErrors: 3,          // Alert if > 3 switch errors in 5 min
  renderErrors: 5,          // Alert if > 5 render errors in 5 min
  performanceSlow: 20       // Alert if > 20 slow switches in 5 min
}
```

#### Alert Configuration

```yaml
# alerts.yml (for Prometheus/Grafana/Datadog)
alerts:
  - name: i18n_missing_keys_high
    condition: rate(i18n_missing_keys_total[5m]) > 10
    severity: warning
    action: notify_team

  - name: i18n_load_failures_critical
    condition: rate(i18n_load_failures_total[5m]) > 5
    severity: critical
    action: page_oncall

  - name: i18n_switch_errors_high
    condition: rate(i18n_switch_errors_total[5m]) > 3
    severity: warning
    action: notify_team

  - name: i18n_performance_degraded
    condition: histogram_quantile(0.95, i18n_switch_duration_seconds) > 1
    severity: warning
    action: notify_team
```

---

### 6. Communication Plan

#### Pre-Rollout Communication

**To Users:**

- Feature announcement on blog/social media
- In-app banner: "New: Multi-language support coming soon!"
- Email to beta testers with opt-in link

**To Team:**

- Slack announcement with rollout schedule
- On-call rotation briefing
- Runbook shared with ops team

#### During Rollout

**Status Updates:**

- Hourly updates in Slack #i18n-rollout channel
- Dashboard showing live metrics
- Go/no-go decision points documented

**Example Update:**

```
🚀 i18n Rollout Update - 2pm EST
Stage: 25% rollout (Day 2)
Status: ✅ GREEN
Metrics:
- Error rate: 0.3% (target <1%) ✅
- Missing keys: 2 (target <10) ✅
- Performance P95: 380ms (target <500ms) ✅
- User feedback: 87% positive (target >80%) ✅

Next checkpoint: 5pm EST (50% rollout decision)
```

#### Rollback Communication

**If rollback needed:**

```
⚠️ i18n Rollback Initiated - 3pm EST
Reason: Critical bug in language switching (P0)
Action: Full rollback to 0% - ETA 5 minutes
Impact: Users see English only, no data loss
Follow-up: Root cause analysis tomorrow 10am
Incident Report: [link]
```

---

## Code Patterns

### ❌ Anti-Pattern (Current)

```tsx
export default function ProxyHosts() {
  return (
    <PageShell title="Proxy Hosts" description="Manage your proxy hosts">
      <Button>Create Proxy Host</Button>
    </PageShell>
  )
}
```

### ✅ Correct Pattern (After Fix)

```tsx
import { useTranslation } from 'react-i18next'

export default function ProxyHosts() {
  const { t } = useTranslation()
  return (
    <PageShell title={t('proxyHosts.title')} description={t('proxyHosts.description')}>
      <Button>{t('proxyHosts.create')}</Button>
    </PageShell>
  )
}
```

### ✅ With Feature Flag (Transition)

```tsx
import { useTranslation } from 'react-i18next'
import { useFeatureFlags } from '../context/FeatureFlagContext'

export default function ProxyHosts() {
  const { t } = useTranslation()
  const { enableI18n } = useFeatureFlags()

  return (
    <PageShell
      title={enableI18n ? t('proxyHosts.title') : 'Proxy Hosts'}
      description={enableI18n ? t('proxyHosts.description') : 'Manage your proxy hosts'}
    >
      <Button>{enableI18n ? t('proxyHosts.create') : 'Create Proxy Host'}</Button>
    </PageShell>
  )
}
```

### Dynamic Content Pattern

```tsx
// ❌ Wrong
<p>You have {count} proxy hosts</p>

// ✅ Correct
<p>{t('proxyHosts.count', { count })}</p>

// In translation file:
{ "proxyHosts": { "count": "You have {{count}} proxy hosts" } }
```

### Error Handling Pattern

```tsx
// ✅ Correct
try {
  await saveProxyHost(data)
  toast.success(t('notifications.saveSuccess'))
} catch (error) {
  toast.error(t('notifications.saveFailed'))
  console.error('Save failed:', error) // Always log in English for debugging
}
```

### Form Validation Pattern

```tsx
// ✅ Correct
const schema = z.object({
  domain: z.string().min(1, t('errors.required'))
})

// Better: Use error key
const schema = z.object({
  domain: z.string().min(1, { message: 'errors.required' })
})

// Then translate in form
{errors.domain && <span>{t(errors.domain.message)}</span>}
```

### Memoization Pattern (Performance)

```tsx
// ✅ For expensive computed values
const columns = useMemo(() => [
  { header: t('proxyHosts.domain'), accessorKey: 'domain_names' },
  { header: t('common.status'), accessorKey: 'enabled' }
], [t]) // Re-compute when t function changes (language switch)
```

### Conditional Translation Pattern

```tsx
// ✅ Correct - different keys for different states
<Badge>
  {proxy.enabled
    ? t('common.enabled')
    : t('common.disabled')
  }
</Badge>

// ✅ Also correct - one key with context
<Badge>{t('common.status', { context: proxy.enabled ? 'enabled' : 'disabled' })}</Badge>
```

---

## Conclusion

The language selector bug is a complete disconnect between infrastructure and implementation. The infrastructure works perfectly—the issue is that NO components use the translation system. All UI text is hardcoded in English.

**Fix**: Systematically update every component to use `useTranslation` and replace hardcoded strings with translation keys.

**Implementation Timeline:** 3-4 weeks (15-20 business days)

- Week 1: Layout + ProxyHosts (pattern validation)
- Week 2: Core pages (6-8 pages)
- Week 3: Dashboard + Auth (critical paths)
- Week 4: QA + Polish (comprehensive testing)
- Buffer: Week 5 (if needed)

**Risk Level:** Low-Medium

- Infrastructure proven working
- Changes are display-only (no logic changes)
- Feature flag enables safe rollback
- Phased rollout reduces blast radius

**User Impact:** High (positive)

- Enables full multilingual support for 5 languages
- Improves accessibility for non-English users
- Professional appearance and localization
- Competitive advantage for international users

**Success Metrics:**

- 100% translation coverage (0 hardcoded strings)
- < 500ms language switch time
- 0% missing translation keys
- > 90% user task completion in all languages
- 0 critical bugs in production

**Rollback Plan:**

- Feature flag for immediate disable
- Phased rollout with monitoring
- Clear rollback procedures
- 24/7 monitoring during rollout

Once complete, the translation infrastructure will finally be utilized, and users will see the UI in their selected language. The phased approach with feature flags ensures we can roll back instantly if issues arise, while the comprehensive testing strategy ensures quality before wide release.

**Next Steps:**

1. ✅ Approve this plan
2. Add missing 48 translation keys to all 5 language files
3. Begin Phase 1: Layout & Navigation (Day 1)
4. Proceed through phases systematically
5. Monitor metrics at each stage
6. Celebrate successful multilingual launch! 🎉

---

**Document Version:** 2.0
**Last Updated:** 2025-12-19
**Status:** ✅ Ready for Implementation
**Approvals Required:** Tech Lead, Product Manager, QA Lead

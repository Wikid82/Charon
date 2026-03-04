# Production Bug Investigation: LAPI Auth & Translation Keys

**Date**: 2025-01-20
**Status**: Investigation Complete
**Priority**: High (Both bugs affect production stability and UX)

---

## Executive Summary

This document details the investigation of two production bugs in Charon:

1. **Bug 1**: CrowdSec LAPI "access forbidden" error repeating every 10 seconds
2. **Bug 2**: WebUI displaying raw translation keys like `translation.security.crowdsec.title`

Both issues have been traced to their root causes with proposed fix approaches.

---

## Bug 1: CrowdSec LAPI "access forbidden" Error

### Symptoms

- Error in logs: `"msg":"API request failed","error":"making request: performing request: API error: access forbidden"`
- Error repeats continuously every 10 seconds
- CrowdSec bouncer cannot authenticate with Local API (LAPI)

### Files Investigated

| File | Purpose | Key Findings |
|------|---------|--------------|
| [backend/internal/caddy/config.go](../../backend/internal/caddy/config.go#L1129) | Generates Caddy JSON config | `getCrowdSecAPIKey()` only reads env vars |
| [backend/internal/crowdsec/registration.go](../../backend/internal/crowdsec/registration.go) | Bouncer registration utilities | Has key validation but not called at startup |
| [backend/internal/api/handlers/crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go) | HTTP handlers for CrowdSec | `Start()` doesn't call bouncer registration |
| [configs/crowdsec/register_bouncer.sh](../../configs/crowdsec/register_bouncer.sh) | Shell script for bouncer registration | Manual registration script exists |
| [.docker/docker-entrypoint.sh](../../.docker/docker-entrypoint.sh) | Container startup | Only registers machine, NOT bouncer |

### Root Cause Analysis

The root cause has been identified through code analysis:

#### 1. Invalid Static API Key

User configured static bouncer key in docker-compose:

```yaml
environment:
  CHARON_SECURITY_CROWDSEC_API_KEY: "charonbouncerkey2024"
```

This key was **never registered** with CrowdSec LAPI via `cscli bouncers add`.

#### 2. getCrowdSecAPIKey() Only Reads Environment

**File**: [backend/internal/caddy/config.go](../../backend/internal/caddy/config.go#L1129)

```go
func getCrowdSecAPIKey() string {
    key := os.Getenv("CHARON_SECURITY_CROWDSEC_API_KEY")
    if key == "" {
        key = os.Getenv("CROWDSEC_API_KEY")
    }
    // BUG: No fallback to /app/data/crowdsec/bouncer_key file
    // BUG: No validation that key is registered with LAPI
    return key
}
```

**Problem**: Function returns env var without:
- Checking if key exists in bouncer_key file
- Validating key against LAPI before use
- Auto-registering bouncer if key is invalid

#### 3. Missing Auto-Registration at Startup

**File**: [.docker/docker-entrypoint.sh](../../.docker/docker-entrypoint.sh)

```bash
# Current: Only machine registration
cscli machines add local-api --password "$machine_pwd" --force

# Missing: Bouncer registration
# cscli bouncers add caddy-bouncer -k "$bouncer_key"
```

#### 4. Wrong LAPI Port in Environment

User environment may have:

```yaml
CROWDSEC_LAPI_URL: "http://localhost:8080"  # Wrong: Charon uses 8080
```

Should be:

```yaml
CROWDSEC_LAPI_URL: "http://localhost:8085"  # Correct: CrowdSec LAPI port
```

### Data Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    CURRENT (BROKEN) FLOW                        │
└─────────────────────────────────────────────────────────────────┘

User docker-compose.yml           Backend Config              Caddy Bouncer
         │                              │                          │
         │ CHARON_SECURITY_             │                          │
         │ CROWDSEC_API_KEY=            │                          │
         │ "charonbouncerkey2024"       │                          │
         │                              │                          │
         ▼                              ▼                          │
    ┌─────────┐                  ┌─────────────┐                   │
    │ Env Var │ ─────────────▶   │getCrowdSec  │                   │
    │ (static)│                  │APIKey()     │                   │
    └─────────┘                  └──────┬──────┘                   │
                                        │                          │
                                        │ Returns unvalidated key  │
                                        ▼                          │
                                 ┌─────────────┐                   │
                                 │ Caddy JSON  │──────────────────▶│
                                 │ Config      │  Uses invalid key │
                                 └─────────────┘                   │
                                                                   ▼
                                                          ┌──────────────┐
                                                          │ CrowdSec LAPI│
                                                          │ Port 8085    │
                                                          └──────┬───────┘
                                                                 │
                                                                 │ Key not in
                                                                 │ bouncer list
                                                                 ▼
                                                          ┌──────────────┐
                                                          │ 403 FORBIDDEN│
                                                          │ Every 10 sec │
                                                          └──────────────┘
```

### Proposed Fix Approach

The fix is already designed in [crowdsec_lapi_auth_fix.md](crowdsec_lapi_auth_fix.md). Key changes:

#### Phase 1: Backend Changes

1. **Update `getCrowdSecAPIKey()`** to:
   - First check `/app/data/crowdsec/bouncer_key` file
   - Fall back to env vars if file doesn't exist
   - Log source of key for debugging

2. **Add `validateBouncerKey()`** function:
   - Test key against LAPI before use
   - Return boolean validity status

3. **Update CrowdSec `Start()` handler**:
   - After LAPI ready, call `ensureBouncerRegistration()`
   - If env key invalid, auto-register new bouncer
   - Store generated key in bouncer_key file
   - Regenerate Caddy config with valid key

#### Phase 2: Docker Entrypoint

1. Add bouncer registration after machine registration
2. Store generated key in persistent volume

#### Phase 3: Caddy Config Regeneration

1. After bouncer registration, trigger config reload
2. Ensure new key propagates to Caddy bouncer plugin

#### Phase 4: UX Notification for Key Rejection

**Critical UX Requirement**: When the user's env var key (`CHARON_SECURITY_CROWDSEC_API_KEY`) is rejected by LAPI and a new key is auto-generated, **the user MUST be notified** so they can update their docker-compose file. Without this:

1. Container starts → reads bad env key
2. Key rejected → generates new key → bouncer works
3. Container restarts → reads bad env key again (env var overrides file)
4. Key rejected → generates ANOTHER new key
5. **Endless loop of re-registration**

**Implementation**:

1. **Backend API Endpoint**: Add `/api/v1/crowdsec/key-status` that returns:
   ```json
   {
     "keySource": "env" | "file" | "auto-generated",
     "envKeyRejected": true | false,
     "currentKey": "cs-abc123...",  // Masked for display
     "message": "Your environment variable key was rejected. Update your docker-compose with the new key below."
   }
   ```

2. **Frontend Notification Banner**: In Security page CrowdSec section:
   - Show warning banner if `envKeyRejected: true`
   - Display the new valid key (copyable)
   - Provide instructions to update docker-compose.yml
   - Persist warning until user dismisses or env var is fixed

3. **Log Warning**: On startup, log at WARN level:
   ```
   CROWDSEC: Environment variable key rejected by LAPI. Auto-generated new key.
   Update your docker-compose.yml: CHARON_SECURITY_CROWDSEC_API_KEY=<new-key>
   ```

### Acceptance Criteria

- [ ] CrowdSec bouncer authenticates successfully with LAPI
- [ ] No "access forbidden" errors in logs after fix
- [ ] Auto-registration works for new deployments
- [ ] Existing deployments with invalid keys get auto-fixed
- [ ] Key source (env vs file) logged for debugging
- [ ] **UX: Warning banner shown in Security page when env key rejected**
- [ ] **UX: New valid key displayed and copyable for docker-compose update**
- [ ] **UX: Log warning includes the new key for CLI users**

---

## Bug 2: WebUI Displaying Raw Translation Keys

### Symptoms

- WebUI shows literal text: `translation.security.crowdsec.title`
- Expected behavior: Should show "CrowdSec"
- Affects multiple translation keys in Security page

### Files Investigated

| File | Purpose | Key Findings |
|------|---------|--------------|
| [frontend/src/i18n.ts](../../frontend/src/i18n.ts) | i18next initialization | Uses static imports, default namespace is `translation` |
| [frontend/src/main.tsx](../../frontend/src/main.tsx) | App entry point | Imports `./i18n` before rendering |
| [frontend/src/pages/Security.tsx](../../frontend/src/pages/Security.tsx) | Security dashboard | Uses `useTranslation()` hook correctly |
| [frontend/src/locales/en/translation.json](../../frontend/src/locales/en/translation.json) | English translations | All keys exist and are properly nested |
| [frontend/src/context/LanguageContext.tsx](../../frontend/src/context/LanguageContext.tsx) | Language context | Wraps app with language state |

### Root Cause Analysis

#### Verified Working Elements

1. **Translation Keys Exist**:
   ```json
   // frontend/src/locales/en/translation.json (lines 245-281)
   "security": {
     "title": "Security",
     "crowdsec": {
       "title": "CrowdSec",
       "subtitle": "IP Reputation & Threat Intelligence",
       ...
     }
   }
   ```

2. **i18n Initialization Uses Static Imports**:
   ```typescript
   // frontend/src/i18n.ts
   import enTranslation from './locales/en/translation.json'

   const resources = {
     en: { translation: enTranslation },
     ...
   }
   ```

3. **Components Use Correct Hook**:
   ```tsx
   // frontend/src/pages/Security.tsx
   const { t } = useTranslation()
   ...
   <CardTitle>{t('security.crowdsec.title')}</CardTitle>
   ```

#### Probable Root Cause: Namespace Prefix Bug

The symptom `translation.security.crowdsec.title` contains the namespace prefix `translation.` which should **never** appear in output.

**i18next Namespace Behavior**:
- Default namespace: `translation`
- When calling `t('security.crowdsec.title')`, i18next looks for `translation:security.crowdsec.title`
- If found: Returns value ("CrowdSec")
- If NOT found: Returns key only (`security.crowdsec.title`)

**The Bug**: The output contains `translation.` prefix, suggesting one of:

1. **Initialization Race Condition**:
   - i18n module imported but not initialized before first render
   - Suspense fallback showing raw key with namespace

2. **Production Build Issue**:
   - Vite bundler not properly including JSON files
   - Tree-shaking removing translation resources

3. **Browser Cache with Stale Bundle**:
   - Old JS bundle cached that has broken i18n

4. **KeyPrefix Misconfiguration** (less likely):
   - Some code may be prepending `translation.` to keys

### Investigation Required

To confirm the exact cause, the following debugging is needed:

#### 1. Check Browser Console

```javascript
// Run in browser DevTools console
console.log(i18next.isInitialized)  // Should be true
console.log(i18next.language)        // Should be 'en' or detected language
console.log(i18next.t('security.crowdsec.title'))  // Should return "CrowdSec"
console.log(i18next.getResourceBundle('en', 'translation'))  // Should show all translations
```

#### 2. Check Network Tab

- Verify no 404 for translation JSON files
- Verify main.js bundle includes translations (search for "CrowdSec" in bundle)

#### 3. Check React DevTools

- Find component using translation
- Verify `t` function is from i18next, not a mock

### Proposed Fix Approach

#### Hypothesis A: Initialization Race

**Fix**: Ensure i18n is fully initialized before React renders

**File**: [frontend/src/main.tsx](../../frontend/src/main.tsx)

```tsx
// Current
import './i18n'
ReactDOM.createRoot(...).render(...)

// Fixed - Wait for initialization
import i18n from './i18n'

i18n.on('initialized', () => {
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      ...
    </React.StrictMode>
  )
})
```

#### Hypothesis B: Production Build Missing Resources

**Fix**: Verify Vite config includes JSON files

**File**: [frontend/vite.config.ts](../../frontend/vite.config.ts)

```typescript
export default defineConfig({
  // Ensure JSON imported as modules
  json: {
    stringify: true // Keeps JSON as-is
  },
  build: {
    rollupOptions: {
      // Ensure locale files not tree-shaken
      external: [],
    }
  }
})
```

#### Hypothesis C: Enable Debug Mode

**File**: [frontend/src/i18n.ts](../../frontend/src/i18n.ts)

```typescript
.init({
  ...
  debug: true, // Enable to see why key resolution fails
  ...
})
```

### Testing Plan

1. **Local Development**:
   - Clear browser cache and hard reload
   - Open DevTools console, check for i18n debug output
   - Verify translations load

2. **Production Build**:
   - Run `npm run build`
   - Inspect dist/assets/*.js for translation strings
   - Verify "CrowdSec" appears in bundle

3. **Docker Environment**:
   - Rebuild container: `docker build --no-cache`
   - Test with fresh browser/incognito mode

### Acceptance Criteria

- [ ] Security page shows "CrowdSec" not `translation.security.crowdsec.title`
- [ ] All translation keys resolve to values
- [ ] Works in both dev and production builds
- [ ] Works after browser cache clear
- [ ] i18next console shows successful initialization

---

## Implementation Priority

| Bug | Severity | Effort | Priority |
|-----|----------|--------|----------|
| Bug 1: LAPI Auth | High (security feature broken) | Medium | P1 |
| Bug 2: Translations | Medium (UX issue) | Low | P2 |

### Recommended Order

1. **Bug 1 First**: CrowdSec is a core security feature; broken auth defeats its purpose
2. **Bug 2 Second**: Translation issue is visual/UX, doesn't affect functionality

---

## Related Documentation

- [CrowdSec LAPI Auth Fix Design](crowdsec_lapi_auth_fix.md) - Detailed fix design for Bug 1
- [CrowdSec Integration Guide](../crowdsec-integration.md) - Overall CrowdSec architecture
- [i18n Setup](../../frontend/src/i18n.ts) - Translation configuration

---

## Appendix: Key Code Sections

### A1: getCrowdSecAPIKey() - Current Implementation

**File**: `backend/internal/caddy/config.go` (line ~1129)

```go
func getCrowdSecAPIKey() string {
    key := os.Getenv("CHARON_SECURITY_CROWDSEC_API_KEY")
    if key == "" {
        key = os.Getenv("CROWDSEC_API_KEY")
    }
    return key
}
```

### A2: i18next Initialization

**File**: `frontend/src/i18n.ts`

```typescript
const resources = {
  en: { translation: enTranslation },
  es: { translation: esTranslation },
  fr: { translation: frTranslation },
  de: { translation: deTranslation },
  zh: { translation: zhTranslation },
}

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: 'en',
    debug: false,
    interpolation: { escapeValue: false },
    detection: {
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
      lookupLocalStorage: 'charon-language',
    },
  })
```

### A3: Security Page Translation Usage

**File**: `frontend/src/pages/Security.tsx`

```tsx
export default function Security() {
  const { t } = useTranslation()
  // ...
  return (
    <PageShell
      title={t('security.title')}
      description={t('security.description')}
    >
      {/* CrowdSec Card */}
      <CardTitle>{t('security.crowdsec.title')}</CardTitle>
      <CardDescription>{t('security.crowdsec.subtitle')}</CardDescription>
      {/* ... */}
    </PageShell>
  )
}
```

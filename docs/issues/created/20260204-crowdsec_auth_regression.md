# CrowdSec Authentication Regression - Bug Investigation Report

**Status**: Investigation Complete - Ready for Fix Implementation
**Priority**: P0 (Critical Production Bug)
**Created**: 2026-02-04
**Reporter**: User via Production Environment
**Affected Version**: Post Auto-Registration Feature

---

## Executive Summary

The CrowdSec integration suffers from **three distinct but related bugs** introduced by the auto-registration feature implementation. While the feature was designed to eliminate manual key management, it contains a critical flaw in key validation logic that causes "access forbidden" errors when users provide environment variable keys. Additionally, there are two UI bugs affecting the bouncer key display component.

**Impact**:
- **High**: Users with `CHARON_SECURITY_CROWDSEC_API_KEY` set experience continuous LAPI connection failures
- **Medium**: Confusing UI showing translation codes instead of human-readable text
- **Low**: Bouncer key card appearing on wrong page in the interface

---

## Bug #1: Flawed Key Validation Logic (CRITICAL)

### The Core Issue

The `ensureBouncerRegistration()` method contains a **logical fallacy** in its validation approach:

```go
// From: backend/internal/api/handlers/crowdsec_handler.go:1545-1570

func (h *CrowdsecHandler) ensureBouncerRegistration(ctx context.Context) (string, error) {
    // Priority 1: Check environment variables
    envKey := getBouncerAPIKeyFromEnv()
    if envKey != "" {
        if h.validateBouncerKey(ctx) {  // ❌ BUG: Validates BOUNCER NAME, not KEY VALUE
            logger.Log().Info("Using CrowdSec API key from environment variable")
            return "", nil // Key valid, nothing new to report
        }
        logger.Log().Warn("Env-provided CrowdSec API key is invalid or bouncer not registered, will re-register")
    }
    // ...
}
```

### What `validateBouncerKey()` Actually Does

```go
// From: backend/internal/api/handlers/crowdsec_handler.go:1573-1598

func (h *CrowdsecHandler) validateBouncerKey(ctx context.Context) bool {
    // ...
    output, err := h.CmdExec.Execute(checkCtx, "cscli", "bouncers", "list", "-o", "json")
    // ...

    for _, b := range bouncers {
        if b.Name == bouncerName {  // ❌ Checks if NAME exists, not if API KEY is correct
            return true
        }
    }
    return false
}
```

### The Failure Scenario

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      Bug #1: Authentication Flow Analysis                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Step 1: User sets docker-compose.yml                                        │
│    CHARON_SECURITY_CROWDSEC_API_KEY=myinventedkey123                         │
│                                                                              │
│  Step 2: CrowdSec starts, bouncer gets registered                           │
│    Result: Bouncer "caddy-bouncer" exists with valid key "xyz789abc..."     │
│                                                                              │
│  Step 3: User enables CrowdSec via GUI                                       │
│    → ensureBouncerRegistration() is called                                  │
│    → envKey = "myinventedkey123" (from env var)                             │
│    → validateBouncerKey() is called                                         │
│      → Checks: Does bouncer named "caddy-bouncer" exist?                    │
│      → Returns: TRUE (bouncer exists, regardless of key value)              │
│    → Conclusion: "Key is valid" ✓ (WRONG!)                                  │
│    → Returns empty string (no new key to report)                            │
│                                                                              │
│  Step 4: Caddy config is generated                                          │
│    → getCrowdSecAPIKey() returns "myinventedkey123"                         │
│    → CrowdSecApp { APIKey: "myinventedkey123", APIUrl: "http://127.0.0.1:8085" } │
│                                                                              │
│  Step 5: Caddy bouncer attempts LAPI connection                             │
│    → Sends HTTP request with header: X-Api-Key: myinventedkey123           │
│    → LAPI checks if "myinventedkey123" is registered                        │
│    → LAPI responds: 403 Forbidden ("access forbidden")                      │
│    → Caddy logs error and retries every 10s indefinitely                    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Root Cause Explained

**What Was Intended**:
- Check if the bouncer exists in CrowdSec's registry
- If it doesn't exist, register a new one
- If it does exist, use the key from the environment or file

**What Actually Happens**:
- Check if a bouncer with name "caddy-bouncer" exists
- If it exists, **assume the env var key is valid** (incorrect assumption)
- Never validate that the env var key **matches** the registered bouncer's key
- Never test the key against LAPI before committing to it

### Why This Broke Working Connections

**Before the Auto-Registration Feature**:
- If user set an invalid key, CrowdSec wouldn't start
- Error was obvious and immediate
- No ambiguous state

**After the Auto-Registration Feature**:
- System auto-registers a valid bouncer on startup
- User's invalid env var key is "validated" by checking bouncer name existence
- Invalid key gets used because validation passed
- Connection fails with cryptic "access forbidden" error
- User sees bouncer as "registered" in UI but connection still fails

---

## Bug #2: UI Translation Codes Displayed (MEDIUM)

### The Symptom

Users report seeing:
```
security.crowdsec.bouncerApiKey
```

Instead of:
```
Bouncer API Key
```

### Investigation Findings

**Translation Key Exists**:
```json
// frontend/src/locales/en/translation.json:272
{
  "security": {
    "crowdsec": {
      "bouncerApiKey": "Bouncer API Key",
      "keyCopied": "API key copied to clipboard",
      "copyFailed": "Failed to copy API key",
      // ...
    }
  }
}
```

**Component Uses Translation Correctly**:
```tsx
// frontend/src/components/CrowdSecBouncerKeyDisplay.tsx:72-75
<CardTitle className="flex items-center gap-2 text-base">
  <Key className="h-4 w-4" />
  {t('security.crowdsec.bouncerApiKey')}
</CardTitle>
```

### Possible Causes

1. **Translation Context Not Loaded**: The `useTranslation()` hook might not have access to the full translation namespace when the component renders
2. **Import Order Issue**: Translation provider might be initialized after component mount
3. **Build Cache**: Stale build artifacts from webpack/vite cache

### Evidence Supporting Cache Theory

From test files:
```typescript
// frontend/src/components/__tests__/CrowdSecBouncerKeyDisplay.test.tsx:33
t: (key: string) => {
  const translations: Record<string, string> = {
    'security.crowdsec.bouncerApiKey': 'Bouncer API Key',
    // Mock translations work correctly in tests
  }
}
```

Tests pass with mocked translations, suggesting the issue is runtime-specific, not code-level.

---

## Bug #3: Component Rendered on Wrong Page (LOW)

### The Symptom

The `CrowdSecBouncerKeyDisplay` component appears on the **Security Dashboard** page instead of (or in addition to) the **CrowdSec Config** page.

### Expected Behavior

```
Security Dashboard (/security)
  ├─ Cerberus Status Card
  ├─ Admin Whitelist Card
  ├─ Security Layer Cards (CrowdSec, ACL, WAF, Rate Limit)
  └─ [NO BOUNCER KEY CARD]

CrowdSec Config Page (/security/crowdsec)
  ├─ CrowdSec Status & Controls
  ├─ Console Enrollment Card
  ├─ Hub Management
  ├─ Decisions List
  └─ [BOUNCER KEY CARD HERE] ✅
```

### Current (Buggy) Behavior

The component appears on the Security Dashboard page.

### Code Evidence

**Correct Import Location**:
```tsx
// frontend/src/pages/CrowdSecConfig.tsx:16
import { CrowdSecBouncerKeyDisplay } from '../components/CrowdSecBouncerKeyDisplay'

// frontend/src/pages/CrowdSecConfig.tsx:543-545
{/* CrowdSec Bouncer API Key - moved from Security Dashboard */}
{status.cerberus?.enabled && status.crowdsec.enabled && (
  <CrowdSecBouncerKeyDisplay />
)}
```

**Migration Evidence**:
```typescript
// frontend/src/pages/__tests__/Security.functional.test.tsx:102
// NOTE: CrowdSecBouncerKeyDisplay mock removed (moved to CrowdSecConfig page)

// frontend/src/pages/__tests__/Security.functional.test.tsx:404-405
// NOTE: CrowdSec Bouncer Key Display moved to CrowdSecConfig page (Sprint 3)
// Tests for bouncer key display are now in CrowdSecConfig tests
```

### Hypothesis

**Most Likely**: The component is **still imported** in `Security.tsx` despite the migration comments. The test mock was removed but the actual component import wasn't.

**File to Check**:
```tsx
// frontend/src/pages/Security.tsx
// Search for: CrowdSecBouncerKeyDisplay import or usage
```

The Security.tsx file is 618 lines long, and the migration might not have been completed.

---

## How CrowdSec Bouncer Keys Actually Work

Understanding the authentication mechanism is critical to fixing Bug #1.

### CrowdSec Bouncer Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                        CrowdSec Bouncer Flow                            │
├────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Component 1: CrowdSec Agent (LAPI Server)                             │
│    • Runs on port 8085 (Charon default)                                │
│    • Maintains SQLite database of registered bouncers                  │
│    • Database: /var/lib/crowdsec/data/crowdsec.db                      │
│    • Table: bouncers (columns: name, api_key, ip_address, ...)         │
│    • Authenticates API requests via X-Api-Key header                   │
│                                                                         │
│  Component 2: Bouncer Client (Caddy Plugin)                            │
│    • Embedded in Caddy via github.com/hslatman/caddy-crowdsec-bouncer  │
│    • Makes HTTP requests to LAPI (GET /v1/decisions/stream)            │
│    • Includes X-Api-Key header in every request                        │
│    • Key must match a registered bouncer in LAPI database              │
│                                                                         │
│  Component 3: Registration (cscli)                                      │
│    • Command: cscli bouncers add <name>                                 │
│    • Generates random API key (e.g., "a1b2c3d4e5f6...")                │
│    • Stores key in database (hashed? TBD)                               │
│    • Returns plaintext key to caller (one-time show)                   │
│    • Key must be provided to bouncer client for authentication         │
│                                                                         │
└────────────────────────────────────────────────────────────────────────┘
```

### Authentication Flow

```
1. Bouncer Registration:
   $ cscli bouncers add caddy-bouncer
   → Generates: "abc123xyz789def456ghi789"
   → Stores hash in: /var/lib/crowdsec/data/crowdsec.db (bouncers table)
   → Returns plaintext: "abc123xyz789def456ghi789"

2. Bouncer Configuration:
   Caddy config:
   {
     "apps": {
       "crowdsec": {
         "api_key": "abc123xyz789def456ghi789",
         "api_url": "http://127.0.0.1:8085"
       }
     }
   }

3. Bouncer Authentication Request:
   GET /v1/decisions/stream HTTP/1.1
   Host: 127.0.0.1:8085
   X-Api-Key: abc123xyz789def456ghi789

4. LAPI Validation:
   • Extract X-Api-Key header
   • Hash the key value
   • Compare hash against bouncers table
   • If match: return decisions (200 OK)
   • If no match: return 403 Forbidden
```

### Why Keys Cannot Be "Invented"

**User Misconception**:
> "I'll just set `CHARON_SECURITY_CROWDSEC_API_KEY=mySecurePassword123` in docker-compose.yml"

**Reality**:
- The API key is **not a password you choose**
- It's a **randomly generated token** by CrowdSec
- Only keys generated via `cscli bouncers add` are stored in the database
- LAPI has no record of "mySecurePassword123" → rejects it

**Analogy**:
Setting an invented API key is like showing a fake ID at a checkpoint. The guard doesn't care if the ID looks official—they check their list. If you're not on the list, you're denied.

### Do Keys Need Hashing?

**For Storage**: Yes, likely hashed in the database (CWE-312 mitigation)

**For Transmission**: **No**, must be plaintext in the `X-Api-Key` header

**For Display in UI**: **Partial masking** is recommended (first 4 + last 3 chars)

```go
// backend/internal/api/handlers/crowdsec_handler.go:1757-1763
if fullKey != "" && len(fullKey) > 7 {
    info.KeyPreview = fullKey[:4] + "..." + fullKey[len(fullKey)-3:]
} else if fullKey != "" {
    info.KeyPreview = "***"
}
```

**Security Note**: The full key must be retrievable for the "Copy to Clipboard" feature, so it's stored in plaintext in the file `/app/data/crowdsec/bouncer_key` with `chmod 600` permissions.

---

## File Locations & Architecture

### Backend Files

| File | Purpose | Lines of Interest |
|------|---------|-------------------|
| `backend/internal/api/handlers/crowdsec_handler.go` | Main CrowdSec handler | Lines 482, 1543-1625 (buggy validation) |
| `backend/internal/caddy/config.go` | Caddy config generation | Lines 65, 1129-1160 (key retrieval) |
| `backend/internal/crowdsec/registration.go` | Bouncer registration utilities | Lines 96-122, 257-336 (helper functions) |
| `.docker/docker-entrypoint.sh` | Container startup script | Lines 223-252 (CrowdSec initialization) |
| `configs/crowdsec/register_bouncer.sh` | Bouncer registration script | Lines 1-43 (manual registration) |

### Frontend Files

| File | Purpose | Lines of Interest |
|------|---------|-------------------|
| `frontend/src/components/CrowdSecBouncerKeyDisplay.tsx` | Key display component | Lines 35-148 (entire component) |
| `frontend/src/pages/CrowdSecConfig.tsx` | CrowdSec config page | Lines 16, 543-545 (component usage) |
| `frontend/src/pages/Security.tsx` | Security dashboard | Lines 1-618 (check for stale imports) |
| `frontend/src/locales/en/translation.json` | English translations | Lines 272-278 (translation keys) |

### Key Storage Locations

| Path | Description | Permissions | Persists? |
|------|-------------|-------------|-----------|
| `/app/data/crowdsec/bouncer_key` | Primary key storage (NEW) | 600 | ✅ Yes (Docker volume) |
| `/etc/crowdsec/bouncers/caddy-bouncer.key` | Legacy location | 600 | ❌ No (ephemeral) |
| `CHARON_SECURITY_CROWDSEC_API_KEY` env var | User override | N/A | ✅ Yes (compose file) |

---

## Step-by-Step Fix Plan

### Fix #1: Correct Key Validation Logic (P0 - CRITICAL)

**File**: `backend/internal/api/handlers/crowdsec_handler.go`

**Current Code** (Lines 1545-1570):
```go
func (h *CrowdsecHandler) ensureBouncerRegistration(ctx context.Context) (string, error) {
    envKey := getBouncerAPIKeyFromEnv()
    if envKey != "" {
        if h.validateBouncerKey(ctx) {  // ❌ Validates name, not key value
            logger.Log().Info("Using CrowdSec API key from environment variable")
            return "", nil
        }
        logger.Log().Warn("Env-provided CrowdSec API key is invalid or bouncer not registered, will re-register")
    }
    // ...
}
```

**Proposed Fix**:
```go
func (h *CrowdsecHandler) ensureBouncerRegistration(ctx context.Context) (string, error) {
    envKey := getBouncerAPIKeyFromEnv()
    if envKey != "" {
        // TEST KEY AGAINST LAPI, NOT JUST BOUNCER NAME
        if h.testKeyAgainstLAPI(ctx, envKey) {
            logger.Log().Info("Using CrowdSec API key from environment variable (verified)")
            return "", nil
        }
        logger.Log().Warn("Env-provided CrowdSec API key failed LAPI authentication, will re-register")
    }

    fileKey := readKeyFromFile(bouncerKeyFile)
    if fileKey != "" {
        if h.testKeyAgainstLAPI(ctx, fileKey) {
            logger.Log().WithField("file", bouncerKeyFile).Info("Using CrowdSec API key from file (verified)")
            return "", nil
        }
        logger.Log().WithField("file", bouncerKeyFile).Warn("File API key failed LAPI authentication, will re-register")
    }

    return h.registerAndSaveBouncer(ctx)
}
```

**New Method to Add**:
```go
// testKeyAgainstLAPI validates an API key by making an authenticated request to LAPI.
// Returns true if the key is accepted (200 OK), false otherwise.
func (h *CrowdsecHandler) testKeyAgainstLAPI(ctx context.Context, apiKey string) bool {
    if apiKey == "" {
        return false
    }

    // Get LAPI URL
    lapiURL := "http://127.0.0.1:8085"
    if h.Security != nil {
        cfg, err := h.Security.Get()
        if err == nil && cfg != nil && cfg.CrowdSecAPIURL != "" {
            lapiURL = cfg.CrowdSecAPIURL
        }
    }

    // Construct heartbeat endpoint URL
    endpoint := fmt.Sprintf("%s/v1/heartbeat", strings.TrimRight(lapiURL, "/"))

    // Create request with timeout
    testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(testCtx, http.MethodGet, endpoint, nil)
    if err != nil {
        logger.Log().WithError(err).Debug("Failed to create LAPI test request")
        return false
    }

    // Set API key header
    req.Header.Set("X-Api-Key", apiKey)

    // Execute request
    client := network.NewInternalServiceHTTPClient(5 * time.Second)
    resp, err := client.Do(req)
    if err != nil {
        logger.Log().WithError(err).Debug("Failed to connect to LAPI for key validation")
        return false
    }
    defer resp.Body.Close()

    // Check response status
    if resp.StatusCode == http.StatusOK {
        logger.Log().Debug("API key validated successfully against LAPI")
        return true
    }

    logger.Log().WithField("status", resp.StatusCode).Debug("API key rejected by LAPI")
    return false
}
```

**Rationale**:
- Tests the key against the **actual LAPI endpoint** (`/v1/heartbeat`)
- Uses the same authentication header (`X-Api-Key`) that Caddy bouncer will use
- Returns true only if LAPI accepts the key (200 OK)
- Fails safely if LAPI is unreachable (returns false, triggers re-registration)

### Fix #2: Remove Stale Component Import from Security Dashboard (P2)

**File**: `frontend/src/pages/Security.tsx`

**Task**:
1. Search for any remaining import of `CrowdSecBouncerKeyDisplay`
2. Search for any JSX usage of `<CrowdSecBouncerKeyDisplay />`
3. Remove both if found

**Verification**:
```bash
# Search for imports
grep -n "CrowdSecBouncerKeyDisplay" frontend/src/pages/Security.tsx

# Search for JSX usage
grep -n "<CrowdSecBouncerKeyDisplay" frontend/src/pages/Security.tsx
```

**Expected Result**: No matches found (component fully migrated to CrowdSecConfig.tsx)

### Fix #3: Resolve Translation Display Issue (P2)

**Option A: Clear Build Cache** (Try First)
```bash
cd frontend
rm -rf node_modules/.vite
rm -rf dist
npm run build
```

**Option B: Verify i18n Provider Wraps Component** (If Cache Clear Fails)

Check that `CrowdSecBouncerKeyDisplay` is used within the i18n context:

```tsx
// Verify in: frontend/src/App.tsx or root component
import { I18nextProvider } from 'react-i18next'
import i18n from './i18n'

function App() {
  return (
    <I18nextProvider i18n={i18n}>
      {/* All components here have translation access */}
      <RouterProvider router={router} />
    </I18nextProvider>
  )
}
```

**Option C: Dynamic Import with Suspense** (If Issue Persists)

Wrap the component in a Suspense boundary to ensure translations load:

```tsx
// frontend/src/pages/CrowdSecConfig.tsx
import { Suspense } from 'react'

{status.cerberus?.enabled && status.crowdsec.enabled && (
  <Suspense fallback={<Skeleton className="h-32 w-full" />}>
    <CrowdSecBouncerKeyDisplay />
  </Suspense>
)}
```

---

## Testing Plan

### Test Case 1: Env Var with Invalid Key (Primary Bug)

**Setup**:
```yaml
# docker-compose.yml
environment:
  - CHARON_SECURITY_CROWDSEC_API_KEY=thisisinvalid
```

**Expected Before Fix**:
- ❌ System validates bouncer name, uses invalid key
- ❌ LAPI returns 403 Forbidden continuously
- ❌ Logs show "Using CrowdSec API key from environment variable"

**Expected After Fix**:
- ✅ System tests key against LAPI, validation fails
- ✅ System auto-generates new valid key
- ✅ Logs show "Env-provided CrowdSec API key failed LAPI authentication, will re-register"
- ✅ LAPI connection succeeds with new key

### Test Case 2: Env Var with Valid Key

**Setup**:
```bash
# Generate a real key first
docker exec charon cscli bouncers add test-bouncer

# Copy key to docker-compose.yml
environment:
  - CHARON_SECURITY_CROWDSEC_API_KEY=<generated-key>
```

**Expected After Fix**:
- ✅ System tests key against LAPI, validation succeeds
- ✅ System uses provided key (no new key generated)
- ✅ Logs show "Using CrowdSec API key from environment variable (verified)"
- ✅ LAPI connection succeeds

### Test Case 3: No Env Var, File Key Exists

**Setup**:
```bash
# docker-compose.yml has no CHARON_SECURITY_CROWDSEC_API_KEY

# File exists from previous run
cat /app/data/crowdsec/bouncer_key
# Outputs: abc123xyz789...
```

**Expected After Fix**:
- ✅ System reads key from file
- ✅ System tests key against LAPI, validation succeeds
- ✅ System uses file key
- ✅ Logs show "Using CrowdSec API key from file (verified)"

### Test Case 4: No Key Anywhere (Fresh Install)

**Setup**:
```bash
# No env var set
# No file exists
# Bouncer never registered
```

**Expected After Fix**:
- ✅ System registers new bouncer
- ✅ System saves key to `/app/data/crowdsec/bouncer_key`
- ✅ System logs key banner with masked preview
- ✅ LAPI connection succeeds

### Test Case 5: UI Component Location

**Verification**:
```bash
# Navigate to Security Dashboard
# URL: http://localhost:8080/security

# Expected:
#   - CrowdSec card with toggle and "Configure" button
#   - NO bouncer key card visible

# Navigate to CrowdSec Config
# URL: http://localhost:8080/security/crowdsec

# Expected:
#   - Bouncer key card visible (if CrowdSec enabled)
#   - Card shows: key preview, registered badge, source badge
#   - Copy button works
```

### Test Case 6: UI Translation Display

**Verification**:
```bash
# Navigate to CrowdSec Config
# Enable CrowdSec if not enabled

# Check bouncer key card:
#   - Card title shows "Bouncer API Key" (not "security.crowdsec.bouncerApiKey")
#   - Badge shows "Registered" (not "security.crowdsec.registered")
#   - Badge shows "Environment Variable" or "File" (not raw keys)
#   - Path label shows "Key stored at:" (not "security.crowdsec.keyStoredAt")
```

---

## Rollback Plan

If fixes cause regressions:

1. **Revert `testKeyAgainstLAPI()` Addition**:
   ```bash
   git revert <commit-hash>
   ```

2. **Emergency Workaround for Users**:
   ```yaml
   # docker-compose.yml
   # Remove any CHARON_SECURITY_CROWDSEC_API_KEY line
   # Let system auto-generate key
   ```

3. **Manual Key Registration**:
   ```bash
   docker exec charon cscli bouncers add caddy-bouncer
   # Copy output to docker-compose.yml
   ```

---

## Long-Term Recommendations

### 1. Add LAPI Health Check to Startup

**File**: `.docker/docker-entrypoint.sh`

Add after machine registration:
```bash
# Wait for LAPI to be ready before proceeding
echo "Waiting for CrowdSec LAPI to be ready..."
for i in $(seq 1 30); do
    if curl -s -f http://127.0.0.1:8085/v1/heartbeat > /dev/null 2>&1; then
        echo "✓ LAPI is ready"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "✗ LAPI failed to start within 30 seconds"
        exit 1
    fi
    sleep 1
done
```

### 2. Add Bouncer Key Rotation Feature

**UI Button**: "Rotate Bouncer Key"

**Behavior**:
1. Delete current bouncer (`cscli bouncers delete caddy-bouncer`)
2. Register new bouncer (`cscli bouncers add caddy-bouncer`)
3. Save new key to file
4. Reload Caddy config
5. Show new key in UI banner

### 3. Add LAPI Connection Status Indicator

**UI Enhancement**: Real-time status badge

```tsx
<Badge variant={lapiConnected ? 'success' : 'error'}>
  {lapiConnected ? 'LAPI Connected' : 'LAPI Connection Failed'}
</Badge>
```

**Backend**: WebSocket or polling endpoint to check LAPI status every 10s

### 4. Documentation Updates

**Files to Update**:
- `docs/guides/crowdsec-setup.md` - Add troubleshooting section for "access forbidden"
- `README.md` - Clarify that bouncer keys are auto-generated
- `docker-compose.yml.example` - Remove `CHARON_SECURITY_CROWDSEC_API_KEY` or add warning comment

---

## References

### Related Issues & PRs
- Original Working State: Before auto-registration feature
- Auto-Registration Feature Plan: `docs/plans/crowdsec_bouncer_auto_registration.md`
- LAPI Auth Fix Plan: `docs/plans/crowdsec_lapi_auth_fix.md`

### External Documentation
- [CrowdSec Bouncer API Documentation](https://doc.crowdsec.net/docs/next/local_api/bouncers/)
- [CrowdSec cscli Bouncers Commands](https://doc.crowdsec.net/docs/next/cscli/cscli_bouncers/)
- [Caddy CrowdSec Bouncer Plugin](https://github.com/hslatman/caddy-crowdsec-bouncer)

### Code Comments & Markers
- `// ❌ BUG:` markers added to problematic validation logic
- `// TODO:` markers for future enhancements

---

## Conclusion

This bug regression stems from a **logical flaw** in the key validation implementation. The auto-registration feature was designed to eliminate user error, but ironically introduced a validation shortcut that causes the exact problem it was meant to solve.

**The Fix**: Replace name-based validation with actual LAPI authentication testing.

**Estimated Fix Time**: 2-4 hours (implementation + testing)

**Risk Level**: Low (new validation is strictly more correct than old)

**User Impact After Fix**: Immediate resolution - invalid keys rejected, valid keys used correctly, "access forbidden" errors eliminated.

---

**Investigation Status**: ✅ Complete
**Next Step**: Implement fixes per step-by-step plan above
**Assignee**: [Development Team]
**Target Resolution**: [Date]

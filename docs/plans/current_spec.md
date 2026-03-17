# Fresh Install Bug Investigation — Comprehensive Plan

**Date:** 2026-03-17
**Source:** Community user bug report (see `docs/plans/chores.md`)
**Confidence Score:** 85% — High certainty based on code analysis; some issues require runtime reproduction to confirm edge cases.

---

## 1. Executive Summary

A community user performed a **clean start / fresh installation** of Charon and reported 7 bugs. After thorough code analysis, the findings are:

| # | Issue | Classification | Severity |
|---|-------|----------------|----------|
| 1 | Missing database values before CrowdSec enabling | **LIKELY BUG** | Medium |
| 2 | CrowdSec requires CLI register before enrollment | **BY DESIGN** (auto-handled) | Low |
| 3 | UI bugs on first enabling CrowdSec | **LIKELY BUG** | Medium |
| 4 | "Required value" error when enabling CrowdSec | **CONFIRMED BUG** | Medium |
| 5 | Monitor TCP port — UI can't add | **LIKELY BUG** | Medium |
| 6 | HTTP always down but HTTPS okay | **CONFIRMED BUG** | High |
| 7 | Security blocked local connection to private IP | **BY DESIGN** (with UX gap) | Medium |

---

## 2. Detailed Investigation

---

### Issue 1: Missing Database Values Before CrowdSec Enabling

**User report:** "some database missing values — i think before crowdsec enabling"

#### Files Examined

- `backend/internal/models/security_config.go` — `SecurityConfig` model definition
- `backend/internal/models/setting.go` — `Setting` key-value model
- `backend/internal/api/routes/routes.go` (L93-L120) — AutoMigrate call
- `backend/internal/api/handlers/security_handler.go` (L69-L215) — `GetStatus` handler
- `backend/cmd/seed/main.go` — Seed data (dev only)

#### Root Cause Analysis

On a fresh install:

1. **`SecurityConfig` table is auto-migrated but has NO seed row.** GORM `AutoMigrate` creates the table schema but does not insert default rows. The `SecurityConfig` model has no GORM `default:` tags on key fields like `CrowdSecMode`, `WAFMode`, or `RateLimitMode` — they start as Go zero values (empty strings for strings, `false` for bools, `0` for ints).

2. **`GetStatus` handler gracefully handles missing SecurityConfig** — it queries `WHERE name = 'default'` and if not found, falls back to static config defaults. However, **the settings table is completely empty** on a fresh install because no settings are seeded. The handler reads settings like `feature.cerberus.enabled`, `security.crowdsec.enabled`, etc. via raw SQL — these return empty results, so the fallback chain works but returns all-disabled state.

3. **The `Start` handler in `crowdsec_handler.go` creates a SecurityConfig row on first enable** (line 466-475), but only when the user toggles CrowdSec on. Before that point, the DB has no `security_configs` row at all.

4. **Fields like `WAFParanoiaLevel` have `gorm:"default:1"` but most fields don't.** On a fresh install, if any code reads `SecurityConfig` expecting populated defaults (e.g., `crowdsec_api_url`), it gets empty strings.

**Key issue:** The `crowdsecPowerMutation` in `Security.tsx` calls `updateSetting('security.crowdsec.enabled', 'true', ...)` before calling `startCrowdsec()`. The `updateSetting` call uses `UpdateSettingRequest` which has `binding:"required"` on `Value`. The value `"true"` satisfies this. However, **no SecurityConfig row exists yet** — the `Start` handler creates it. The sequence is:
1. Frontend calls `updateSetting` — creates/updates a setting row ✓
2. Frontend calls `startCrowdsec()` — backend `Start` handler creates SecurityConfig OR updates existing one ✓

This works, but the **`GetStatus` handler returns stale/empty data between step 1 and step 2** because the optimistic update in the frontend doesn't account for the SecurityConfig not existing yet.

#### Classification: **LIKELY BUG**

The system functionally works but returns confusing intermediate states during the first enable sequence. The missing SecurityConfig row and absence of seeded settings means the `/security/status` endpoint returns an all-empty/disabled state until the user explicitly toggles something.

#### Proposed Fix

1. Add a database seed step to the main application startup (not just the dev seed tool) that ensures a default `SecurityConfig` row exists with sensible defaults:
   ```go
   // In routes.go or main.go after AutoMigrate
   var cfg models.SecurityConfig
   if err := db.Where("name = ?", "default").FirstOrCreate(&cfg, models.SecurityConfig{
       UUID:           "default",
       Name:           "default",
       Enabled:        false,
       CrowdSecMode:   "disabled",
       WAFMode:        "disabled",
       RateLimitMode:  "disabled",
       CrowdSecAPIURL: "http://127.0.0.1:8085",
   }).Error; err != nil {
       log.Warn("Failed to seed default SecurityConfig")
   }
   ```
2. Add default setting rows for feature flags (`feature.cerberus.enabled`, etc.) during startup.

---

### Issue 2: CrowdSec Still Needs CLI Register Before Enrollment

**User report:** "crowdsec still needs cli register before you can enroll"

#### Files Examined

- `backend/internal/crowdsec/console_enroll.go` (L250-L330) — `ensureCAPIRegistered()` and `checkLAPIAvailable()`
- `backend/internal/api/handlers/crowdsec_handler.go` (L1262-L1360) — `ConsoleEnroll` handler
- `backend/internal/api/handlers/crowdsec_handler.go` (L458-L581) — `Start` handler (bouncer registration)

#### Root Cause Analysis

The enrollment flow already handles CAPI registration automatically:

1. `ConsoleEnroll` handler calls `h.Console.Enroll(ctx, …)`
2. `Enroll()` calls `s.checkLAPIAvailable(ctx)` — verifies LAPI is running (retries 5x with exponential backoff, ~45s total)
3. `Enroll()` calls `s.ensureCAPIRegistered(ctx)` — checks for `online_api_credentials.yaml` and runs `cscli capi register` if missing
4. Then runs `cscli console enroll --name <agent> <token>`

**The auto-registration IS implemented.** However, the user may have encountered:
- **Timing issue:** If CrowdSec was just started, LAPI may not be ready yet. The `checkLAPIAvailable` retries for ~45s, but if the user triggered enrollment immediately after starting CrowdSec, the timeout may have expired.
- **Feature flag:** Console enrollment is behind `FEATURE_CROWDSEC_CONSOLE_ENROLLMENT` feature flag, which defaults to **disabled** (`false`). The `isConsoleEnrollmentEnabled()` method returns `false` unless explicitly enabled via DB setting or env var. Without this flag, the enrollment endpoint returns 404.
- **Error messaging:** If CAPI registration fails, the error message might be confusing, leading the user to think they need to manually run `cscli capi register`.

#### Classification: **BY DESIGN** (with potential UX gap)

The auto-registration logic exists and works. The feature flag being off by default means the user likely tried to enroll via the console enrollment UI (which is hidden/unavailable) and ended up using the CLI instead. If they tried via the exposed bouncer registration endpoint, that's a different flow — CAPI registration is only auto-triggered by the console enrollment path, not the bouncer registration path.

#### Proposed Fix

1. Improve error messaging when LAPI check times out during enrollment
2. Consider auto-running `cscli capi register` during the `Start` handler (not just during enrollment)
3. Document the enrollment flow more clearly for users

---

### Issue 3: UI Bugs on First Enabling CrowdSec

**User report:** "ui bugs on first enabling crowdsec"

#### Files Examined

- `frontend/src/pages/Security.tsx` (L168-L229) — `crowdsecPowerMutation`
- `frontend/src/pages/Security.tsx` (L440-L452) — CrowdSec toggle Switch
- `frontend/src/pages/CrowdSecConfig.tsx` (L1-L100) — CrowdSec config page
- `frontend/src/components/CrowdSecKeyWarning.tsx` — Key warning component

#### Root Cause Analysis

When CrowdSec is first enabled on a fresh install, several things happen in sequence:

1. `crowdsecPowerMutation` calls `updateSetting('security.crowdsec.enabled', 'true', ...)`
2. Then calls `startCrowdsec()` which takes 10-60 seconds
3. Then calls `statusCrowdsec()` to verify
4. If LAPI ready, `ensureBouncerRegistration()` runs on the backend
5. `onSuccess` callback invalidates queries

**During this ~30s window:**
- The toggle should show loading state, but the `Switch` component reads `crowdsecStatus?.running ?? status.crowdsec.enabled` — if `crowdsecStatus` is stale (from the initial `useEffect` fetch), the toggle may flicker.
- The `CrowdSecConfig` page polls LAPI status every 5 seconds — on first enable, this will show "not running" until LAPI finishes starting.
- The `CrowdSecKeyWarning` component checks key status — on first enable, no bouncer key exists yet, potentially triggering warnings.
- `ConfigReloadOverlay` shows when `isApplyingConfig` is true, but the CrowdSec start operation takes significantly longer than typical config operations.

**Specific bugs likely seen:**
- Toggle flickering between checked/unchecked as different queries return at different times
- Stale "disabled" status shown while CrowdSec is actually starting
- Bouncer key warning appearing briefly before registration completes
- Console enrollment section showing "LAPI not ready" errors

#### Classification: **LIKELY BUG**

The async nature of CrowdSec startup (10-60s) combined with multiple independent polling queries creates a poor UX during the first-enable flow.

#### Proposed Fix

1. Add a dedicated "starting" state to the CrowdSec toggle — show a spinner/loading indicator for the full duration of the start operation
2. Suppress `CrowdSecKeyWarning` while a start operation is in progress
3. Debounce the LAPI status polling to avoid showing transient "not ready" states
4. Use the mutation's `isPending` state to disable all CrowdSec-related UI interactions during startup

---

### Issue 4: "Required Value" Error When Enabling CrowdSec

**User report:** "enabling crowdsec throws ui 'required value' error but enables okay"

#### Files Examined

- `frontend/src/pages/Security.tsx` (L100-L155) — `toggleServiceMutation`
- `frontend/src/pages/Security.tsx` (L168-L182) — `crowdsecPowerMutation`
- `backend/internal/api/handlers/settings_handler.go` (L115-L120) — `UpdateSettingRequest` struct
- `frontend/src/api/settings.ts` (L27-L29) — `updateSetting` function

#### Root Cause Analysis

**This is a confirmed bug** caused by Gin's `binding:"required"` validation tag on the `Value` field:

```go
type UpdateSettingRequest struct {
    Key      string `json:"key" binding:"required"`
    Value    string `json:"value" binding:"required"`
    Category string `json:"category"`
    Type     string `json:"type"`
}
```

The `crowdsecPowerMutation` calls:
```typescript
await updateSetting('security.crowdsec.enabled', enabled ? 'true' : 'false', 'security', 'bool')
```

When `enabled` is `true`, the value `"true"` satisfies `binding:"required"`. So the direct CrowdSec toggle shouldn't fail here.

**The actual bug path:** The `crowdsecPowerMutation` calls `updateSetting` and then `startCrowdsec()`. The `startCrowdsec()` triggers the backend `Start` handler which internally creates/updates settings. If there's a race condition where the frontend also calls a related `updateSetting` with an empty value (e.g., a cascading toggle for an uninitialized setting), Gin's `binding:"required"` treats empty string `""` as missing for string fields, producing a validation error.

**Broader problem:** Any code path that calls `updateSetting` with an empty value (e.g., clearing an admin whitelist, resetting a configuration) triggers this validation error. This is incorrect — an empty string is a valid value for a setting.

#### Classification: **CONFIRMED BUG**

The `binding:"required"` tag on `Value` in `UpdateSettingRequest` means any attempt to set a setting to an empty string `""` will fail with a "required" validation error. This is incorrect — empty string is a valid value.

#### Proposed Fix

Remove `binding:"required"` from the `Value` field:
```go
type UpdateSettingRequest struct {
    Key      string `json:"key" binding:"required"`
    Value    string `json:"value"` // Empty string is valid
    Category string `json:"category"`
    Type     string `json:"type"`
}
```

If value must not be empty for specific keys, add key-specific validation in the handler logic.

#### Reproduction Steps

1. Fresh install of Charon
2. Navigate to Security dashboard
3. Enable Cerberus (master toggle)
4. Toggle CrowdSec ON
5. Observe toast error containing "required" or "required value"
6. Despite the error, CrowdSec still starts successfully

---

### Issue 5: Monitor TCP Port — UI Can't Add

**User report:** "monitor tcp port ui can't add"

#### Files Examined

- `frontend/src/pages/Uptime.tsx` (L342-L500) — `CreateMonitorModal`
- `frontend/src/api/uptime.ts` (L80-L97) — `createMonitor` API
- `backend/internal/api/handlers/uptime_handler.go` (L30-L60) — `CreateMonitorRequest` and `Create` handler
- `backend/internal/services/uptime_service.go` (L1083-L1140) — `CreateMonitor` service method

#### Root Cause Analysis

The frontend `CreateMonitorModal` supports TCP:
```tsx
const [type, setType] = useState<'http' | 'tcp'>('http');
// ...
<option value="tcp">{t('uptime.monitorTypeTcp')}</option>
```

The backend validates:
```go
Type string `json:"type" binding:"required,oneof=http tcp https"`
```

And TCP format validation:
```go
if monitorType == "tcp" {
    if _, _, err := net.SplitHostPort(urlStr); err != nil {
        return nil, fmt.Errorf("TCP URL must be in host:port format: %w", err)
    }
}
```

**Possible issues:**
1. The URL placeholder may not update when TCP is selected — user enters `http://...` format instead of `host:port`
2. No client-side format validation that changes based on type
3. The backend error message about `host:port` format may not surface clearly through the API error chain

#### Classification: **LIKELY BUG**

The i18n placeholder string `urlPlaceholder` is `"https://example.com or tcp://host:port"`. The `tcp://` scheme prefix is misleading — the backend's `net.SplitHostPort()` expects raw `host:port` (no scheme). A user following the placeholder guidance would submit `tcp://192.168.1.1:8080`, which fails `SplitHostPort` parsing because the `://` syntax is not a valid `host:port` format. This is the likely root cause.

#### Proposed Fix

1. **Fix the i18n translation string** in `frontend/src/locales/en/translation.json`: change `"urlPlaceholder"` from `"https://example.com or tcp://host:port"` to `"https://example.com or host:port"` (removing the misleading `tcp://` scheme)
2. Update URL placeholder dynamically: `placeholder={type === 'tcp' ? '192.168.1.1:8080' : 'https://example.com'}`
3. Add helper text below URL field explaining expected format per type
4. Add client-side format validation before submission

---

### Issue 6: HTTP Always Down but HTTPS Okay

**User report:** "http always down but https okay"

#### Files Examined

- `backend/internal/services/uptime_service.go` (L727-L810) — `checkMonitor` method
- `backend/internal/security/url_validator.go` (L169-L300) — `ValidateExternalURL`
- `backend/internal/network/safeclient.go` (L1-L113) — `IsPrivateIP`, `NewSafeHTTPClient`

#### Root Cause Analysis

**CONFIRMED BUG caused by SSRF protection blocking private IP addresses for HTTP monitors.**

In `checkMonitor`, HTTP/HTTPS monitors go through:
```go
case "http", "https":
    validatedURL, err := security.ValidateExternalURL(
        monitor.URL,
        security.WithAllowLocalhost(),
        security.WithAllowHTTP(),
        security.WithTimeout(3*time.Second),
    )
```

Then use:
```go
client := network.NewSafeHTTPClient(
    network.WithTimeout(10*time.Second),
    network.WithDialTimeout(5*time.Second),
    network.WithMaxRedirects(0),
    network.WithAllowLocalhost(),
)
```

**Critical path:**
1. `ValidateExternalURL` resolves the monitor's hostname via DNS
2. It checks ALL resolved IPs against `network.IsPrivateIP()`
3. `IsPrivateIP` blocks RFC 1918 ranges: `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`
4. `WithAllowLocalhost()` only allows `127.0.0.1`, `localhost`, `::1` — does NOT allow private IPs

**If the monitor URL resolves to a private IP** (common for self-hosted services), `ValidateExternalURL` blocks the connection with: `"connection to private ip addresses is blocked for security"`.

**Why HTTPS works but HTTP doesn't:** The user's HTTPS monitors likely point to public domains (via external DNS/CDN) that resolve to public IPs. HTTP monitors target private upstream IPs directly (e.g., `http://192.168.1.100:8080`), which fail the SSRF check.

Meanwhile, **TCP monitors use raw `net.DialTimeout("tcp", ...)` with NO SSRF protection at all** — they bypass the entire validation chain.

> **Note:** The PR should add a code comment in `uptime_service.go` at the TCP `net.DialTimeout` call site acknowledging this deliberate SSRF bypass. TCP monitors currently only accept admin-configured `host:port` (no URL parsing, no redirects), so the SSRF attack surface is minimal. If SSRF validation is added to TCP in the future, it must also respect `WithAllowRFC1918()`.

#### Classification: **CONFIRMED BUG**

Uptime monitoring for self-hosted services on private networks is fundamentally broken by SSRF protection. The `WithAllowLocalhost()` option is insufficient — it only allows `127.0.0.1/localhost`, not the RFC 1918 ranges that self-hosted services use.

#### Proposed Fix

Add a `WithAllowRFC1918()` option for admin-configured uptime monitors that selectively unblocks only RFC 1918 private ranges (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`) while keeping cloud metadata (`169.254.169.254`), link-local, loopback, and reserved ranges blocked.

**Dual-layer fix required** — both the URL validation and the safe dialer must be updated:

**Layer 1: `url_validator.go`** — Add `WithAllowRFC1918()` validation option:
```go
// In checkMonitor:
validatedURL, err := security.ValidateExternalURL(
    monitor.URL,
    security.WithAllowLocalhost(),
    security.WithAllowHTTP(),
    security.WithAllowRFC1918(), // NEW: Only unblocks 10/8, 172.16/12, 192.168/16
    security.WithTimeout(3*time.Second),
)
```

**Layer 2: `safeclient.go`** — Add `AllowRFC1918` to `ClientOptions` and respect it in `safeDialer`:
```go
// In ClientOptions:
AllowRFC1918 bool // Permits connections to RFC 1918 private IPs only

// New option constructor:
func WithAllowRFC1918() Option {
    return func(opts *ClientOptions) {
        opts.AllowRFC1918 = true
    }
}

// In safeDialer, before IsPrivateIP check:
if opts.AllowRFC1918 && isRFC1918(ip.IP) {
    continue // Allow RFC 1918, still block link-local/metadata/reserved
}

// In NewSafeHTTPClient call in checkMonitor:
client := network.NewSafeHTTPClient(
    network.WithTimeout(10*time.Second),
    network.WithDialTimeout(5*time.Second),
    network.WithMaxRedirects(0),
    network.WithAllowLocalhost(),
    network.WithAllowRFC1918(), // NEW: Must match URL validator layer
)
```

Without the `safeDialer` fix, connections pass URL validation but are still blocked at dial time. Both layers must allow RFC 1918.

This is safe because uptime monitors are **admin-configured only** — they require authentication. SSRF protection's purpose is to prevent untrusted user-initiated requests to internal services, not admin-configured health checks. Cloud metadata and link-local remain blocked even with this option.

#### Reproduction Steps

1. Fresh install of Charon
2. Add a proxy host pointing to a local service (e.g., `192.168.1.100:8080`)
3. Monitor auto-creates with `http://yourdomain.local`
4. Monitor status shows "down" with SSRF error
5. HTTPS monitor to a public domain succeeds

---

### Issue 7: Security Blocked Local Connection to Private IP

**User report:** "security blocked local connection to private ip — status in db just noticed randomly"

#### Files Examined

- `backend/internal/network/safeclient.go` (L22-L55) — `privateCIDRs` list
- `backend/internal/network/safeclient.go` (L74-L113) — `IsPrivateIP` function
- `backend/internal/security/url_validator.go` (L169-L300) — `ValidateExternalURL`
- `backend/internal/services/uptime_service.go` (L727-L810) — `checkMonitor`

#### Root Cause Analysis

Direct consequence of Issue 6. The SSRF protection blocks ALL RFC 1918 private IP ranges, plus loopback, link-local, and reserved ranges. This protection is applied at:

1. **URL Validation** (`ValidateExternalURL`) — blocks at URL validation time
2. **Safe Dialer** (`safeDialer`) — blocks at DNS resolution / connection time

The user noticed in the database because:
- The uptime monitor's `status` field shows `"down"`
- The heartbeat `message` stores the SSRF rejection error
- This status persists in the database and is visible through the monitoring UI

#### Classification: **BY DESIGN** (with UX gap)

The SSRF protection is correctly implemented for security. However, the application needs to differentiate between:
1. **External user-initiated URLs** (webhooks, notification endpoints) — MUST block private IPs
2. **Admin-configured monitoring targets** — SHOULD allow private IPs (trusted, intentional configs)

#### Proposed Fix

Same as Issue 6 — introduce `WithAllowRFC1918()` for admin-configured monitoring (both `url_validator.go` and `safeclient.go` layers). Additionally:
1. Add a clear UI message when a monitor is down due to SSRF protection
2. Log the specific blocked IP and reason for admin debugging

---

## 3. Reproduction Steps Summary

### Fresh Install Test Sequence

1. Deploy Charon from a clean image (no existing database)
2. Complete initial setup (create admin user)
3. Navigate to Security dashboard

**Issue 1:** Check Network tab → `GET /api/v1/security/status` — verify response has populated defaults

**Issue 4:** Enable Cerberus → Toggle CrowdSec ON → Watch for "required" error toast

**Issue 3:** During CrowdSec start (~30s), observe UI for flickering/stale states

**Issue 5:** Uptime → Add Monitor → Select TCP → Enter `192.168.1.1:8080` → Submit

**Issues 6 & 7:** Add proxy host to private IP → Wait for auto-sync → Check HTTP monitor status

---

## 4. Implementation Plan

### Phase 1: Playwright E2E Tests

| Test | File | Description |
|------|------|-------------|
| Fresh security dashboard loads | `tests/security/fresh-install.spec.ts` | Verify status endpoint returns valid defaults on empty DB |
| CrowdSec enable flow completes | `tests/security/fresh-install.spec.ts` | Toggle CrowdSec on, verify no validation errors |
| Setting update with empty value | `tests/security/fresh-install.spec.ts` | Verify setting can be cleared |
| TCP monitor creation | `tests/uptime/create-monitor.spec.ts` | Create TCP monitor via UI |
| HTTP monitor for private IP | `tests/uptime/private-ip-monitor.spec.ts` | Create HTTP monitor for private IP, verify it connects |
| TCP placeholder updates dynamically | `tests/uptime/create-monitor.spec.ts` | Verify placeholder changes when switching to TCP type |

### Phase 1b: Backend Unit Tests

| Test | File | Description |
|------|------|-------------|
| `UpdateSettingRequest` with empty value | `settings_handler_test.go` | Verify empty string `""` is accepted for `Value` field (Issue 4) |
| TCP monitor with private IP | `uptime_service_test.go` | Regression: if SSRF is added to TCP later, private IPs must still work |
| Cloud metadata blocked with RFC 1918 allowed | `safeclient_test.go` | `169.254.169.254` remains blocked even when `AllowRFC1918 = true` |
| `safeDialer` with RFC 1918 allowance | `safeclient_test.go` | Dial to `10.x.x.x` succeeds with `AllowRFC1918`, dial to `169.254.x.x` fails |
| `ValidateExternalURL` with RFC 1918 | `url_validator_test.go` | RFC 1918 IPs pass validation; link-local/metadata still rejected |

### Phase 2: Backend Fixes

#### PR-1: Fix `binding:"required"` on Setting Value (Issue 4)
- **Files:** `settings_handler.go`, tests
- **Validation:** `go test ./backend/internal/api/handlers/... -run TestUpdateSetting`

#### PR-2: Seed Default SecurityConfig on Startup (Issue 1)
- **Files:** `routes.go` or `main.go`, tests
- **Validation:** Fresh start → `/security/status` returns valid defaults

#### PR-3: Allow RFC 1918 Private IPs for Uptime Monitors (Issues 6 & 7)
- **Files:** `url_validator.go`, `safeclient.go`, `uptime_service.go`, tests
- **Scope:** Add `WithAllowRFC1918()` option to both `ValidateExternalURL` and `NewSafeHTTPClient`/`safeDialer`. Add `isRFC1918()` helper. Add code comment at TCP `net.DialTimeout` call site noting deliberate SSRF bypass.
- **Validation:** HTTP monitor to `192.168.x.x` shows "up"; cloud metadata `169.254.169.254` remains blocked

### Phase 3: Frontend Fixes

#### PR-4: CrowdSec Enable UX (Issues 3 & 4)
- **Files:** `Security.tsx`, `CrowdSecConfig.tsx`, `CrowdSecKeyWarning.tsx`
- **Validation:** Playwright: CrowdSec toggle smooth, no error toasts

#### PR-5: Monitor Creation UX for TCP (Issue 5)
- **Files:** `Uptime.tsx`, `frontend/src/locales/en/translation.json`
- **Scope:** Fix misleading `tcp://host:port` in i18n placeholder to `host:port`, add dynamic placeholder per monitor type
- **Validation:** Playwright: TCP monitor created via UI

### Phase 4: Documentation & Integration Testing
- Update Getting Started docs with fresh install notes
- Run full Playwright suite against fresh install

---

## 5. Commit Slicing Strategy

**Decision:** Multiple PRs (5 PRs) for safer and faster review.

**Trigger reasons:**
- Cross-domain changes (backend security, backend settings, frontend)
- Multiple independent fixes with no inter-dependencies
- Each fix is individually testable and rollbackable

### Ordered PR Slices

| PR | Scope | Files | Dependencies | Validation Gate |
|----|-------|-------|--------------|-----------------|
| **PR-1** | Fix `binding:"required"` on Setting Value | `settings_handler.go`, tests | None | Unit tests pass |
| **PR-2** | Seed default SecurityConfig on startup | `routes.go`/`main.go`, tests | None | Fresh start returns valid defaults |
| **PR-3** | Allow RFC 1918 IPs for uptime monitors (dual-layer) | `url_validator.go`, `safeclient.go`, `uptime_service.go`, tests | None | HTTP monitor to RFC 1918 IP works; cloud metadata still blocked |
| **PR-4** | CrowdSec enable UX improvements | `Security.tsx`, `CrowdSecConfig.tsx`, `CrowdSecKeyWarning.tsx` | PR-1 | Playwright: smooth toggle |
| **PR-5** | Monitor creation UX for TCP + i18n fix | `Uptime.tsx`, `translation.json` | None | Playwright: TCP creation works |

**Rollback:** Each PR is independently revertable. No DB migrations or schema changes.

---

## 6. E2E Test Gaps

| Test Suite | Covers Fresh Install? | Gap |
|------------|----------------------|-----|
| `tests/security/security-dashboard.spec.ts` | No | Needs fresh-db variant |
| `tests/security/crowdsec-config.spec.ts` | No | Needs first-enable test |
| `tests/uptime/*.spec.ts` | Unknown | Needs TCP + private IP tests |

---

## 7. Ancillary File Review

- **`.gitignore`** — No changes needed
- **`codecov.yml`** — No changes needed
- **`.dockerignore`** — No changes needed
- **`Dockerfile`** — No changes needed

---

## 8. Acceptance Criteria

1. **Issue 1:** `/api/v1/security/status` returns populated defaults on a fresh database
2. **Issue 2:** Documented as by-design; enrollment auto-registers CAPI when needed
3. **Issue 3:** No toggle flickering or transient error states during first CrowdSec enable
4. **Issue 4:** No "required value" error toast when enabling/disabling modules
5. **Issue 5:** TCP monitor creation succeeds with `host:port` format; i18n placeholder no longer includes misleading `tcp://` scheme; dynamic placeholder guides user
6. **Issue 6:** HTTP monitors to private IPs succeed for admin-configured uptime monitors
7. **Issue 7:** Uptime heartbeat messages do not contain "private IP blocked" errors for admin monitors

---

## PR-3 Implementation Plan

**Title:** Allow RFC 1918 IPs for admin-configured uptime monitors (dual-layer SSRF fix)
**Issues Resolved:** Issue 6 (CONFIRMED BUG) + Issue 7 (BY DESIGN / UX gap)
**Status:** **APPROVED** (all blocking concerns resolved)
**Dependencies:** None (independent of PR-1 and PR-2)

---

### Overview

HTTP/HTTPS uptime monitors targeting LAN addresses permanently report "down" on fresh installs. The root cause is a dual-layer SSRF guard: **Layer 1** (`url_validator.go`) rejects private IPs during hostname resolution, and **Layer 2** (`safeclient.go`) re-checks every IP at TCP dial time to defeat DNS rebinding. Because both layers enforce `IsPrivateIP`, patching only one would produce a monitor that passes URL validation but is silently killed at dial time—the bug would appear fixed in logs but remain broken in practice.

The fix threads a single `AllowRFC1918` signal through both layers, visible as the `WithAllowRFC1918()` functional option. It **only** unblocks the three RFC 1918 ranges (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`). All other restricted ranges—cloud metadata (`169.254.0.0/16` including `169.254.169.254`), loopback (`127.0.0.0/8`, `::1`), IPv6 link-local (`fe80::/10`), IPv6 unique-local (`fc00::/7`), and all reserved blocks—remain fully blocked regardless of the option.

**Authentication posture is verified:** Uptime monitor routes (`POST /uptime/monitors/:id/check`, etc.) live inside the `management` route group in `backend/internal/api/routes/routes.go`. That group is nested under `protected`, which enforces `authMiddleware` (JWT), and then applies `middleware.RequireManagementAccess()`. The RFC 1918 bypass is therefore **exclusively accessible to authenticated, management-tier users**—never to passthrough users or unauthenticated callers.

---

### A. File-by-File Change Plan

---

#### File 1: `backend/internal/network/safeclient.go`

**Package:** `network`

**Change 1 — Add RFC 1918 block set**

Below the `privateBlocks` and `initOnce` declarations, introduce a parallel set of `sync.Once`-guarded CIDR blocks containing only the three RFC 1918 ranges. These are stored separately so the `IsRFC1918` check can remain a cheap, focused predicate without reopening the full `IsPrivateIP` logic.

New package-level variables (insert after `initOnce sync.Once`):

```go
var (
    rfc1918Blocks []*net.IPNet
    rfc1918Once   sync.Once
)

var rfc1918CIDRs = []string{
    "10.0.0.0/8",
    "172.16.0.0/12",
    "192.168.0.0/16",
}
```

New init function (insert after `initPrivateBlocks`):

```go
func initRFC1918Blocks() {
    rfc1918Once.Do(func() {
        rfc1918Blocks = make([]*net.IPNet, 0, len(rfc1918CIDRs))
        for _, cidr := range rfc1918CIDRs {
            _, block, err := net.ParseCIDR(cidr)
            if err != nil {
                continue
            }
            rfc1918Blocks = append(rfc1918Blocks, block)
        }
    })
}
```

**Change 2 — Add `IsRFC1918` exported predicate**

Insert after the `IsPrivateIP` function. This function is exported so `url_validator.go` (in the `security` package) can call it via `network.IsRFC1918(ip)`, eliminating duplicated CIDR definitions.

```go
// IsRFC1918 reports whether ip falls within one of the three RFC 1918 private ranges:
// 10.0.0.0/8, 172.16.0.0/12, or 192.168.0.0/16.
//
// Unlike IsPrivateIP, this function does NOT cover loopback, link-local, cloud metadata,
// or any reserved ranges. Use it only to selectively unblock LAN addresses for
// admin-configured features (e.g., uptime monitors) while preserving all other SSRF guards.
func IsRFC1918(ip net.IP) bool {
    if ip == nil {
        return false
    }
    initRFC1918Blocks()
    if ip4 := ip.To4(); ip4 != nil {
        ip = ip4
    }
    for _, block := range rfc1918Blocks {
        if block.Contains(ip) {
            return true
        }
    }
    return false
}
```

**Change 3 — Add `AllowRFC1918` to `ClientOptions` struct**

Insert after the `DialTimeout` field:

```go
// AllowRFC1918 permits connections to RFC 1918 private IP ranges:
// 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16.
//
// SECURITY NOTE: Enable only for admin-configured features (e.g., uptime monitors).
// All other restricted ranges (loopback, link-local, cloud metadata, reserved) remain
// blocked regardless of this flag.
AllowRFC1918 bool
```

`defaultOptions()` returns `AllowRFC1918: false` — no change needed there.

**Change 4 — Add `WithAllowRFC1918` functional option**

Insert after `WithDialTimeout`:

```go
// WithAllowRFC1918 permits connections to RFC 1918 private addresses.
// Use exclusively for admin-configured outbound calls such as uptime monitors;
// never for user-supplied URLs.
func WithAllowRFC1918() Option {
    return func(opts *ClientOptions) {
        opts.AllowRFC1918 = true
    }
}
```

**Change 5 — Update `safeDialer` validation loop**

Locate the loop inside `safeDialer` that reads:
```go
for _, ip := range ips {
    if opts.AllowLocalhost && ip.IP.IsLoopback() {
        continue
    }
    if IsPrivateIP(ip.IP) {
        return nil, fmt.Errorf("connection to private IP blocked: %s resolved to %s", host, ip.IP)
    }
}
```

Replace with:
```go
for _, ip := range ips {
    if opts.AllowLocalhost && ip.IP.IsLoopback() {
        continue
    }
    // Admin-configured monitors may legitimately target LAN services.
    // Allow RFC 1918 ranges only; all other restricted ranges (link-local,
    // cloud metadata 169.254.169.254, loopback, reserved) remain blocked.
    if opts.AllowRFC1918 && IsRFC1918(ip.IP) {
        continue
    }
    if IsPrivateIP(ip.IP) {
        return nil, fmt.Errorf("connection to private IP blocked: %s resolved to %s", host, ip.IP)
    }
}
```

**Change 6 — Update `safeDialer` IP selection loop**

Locate the loop that selects `selectedIP`:
```go
for _, ip := range ips {
    if opts.AllowLocalhost && ip.IP.IsLoopback() {
        selectedIP = ip.IP
        break
    }
    if !IsPrivateIP(ip.IP) {
        selectedIP = ip.IP
        break
    }
}
```

Replace with:
```go
for _, ip := range ips {
    if opts.AllowLocalhost && ip.IP.IsLoopback() {
        selectedIP = ip.IP
        break
    }
    if opts.AllowRFC1918 && IsRFC1918(ip.IP) {
        selectedIP = ip.IP
        break
    }
    if !IsPrivateIP(ip.IP) {
        selectedIP = ip.IP
        break
    }
}
```

**Change 7 — `validateRedirectTarget` (removed from PR-3 scope)**

`checkMonitor` passes `network.WithMaxRedirects(0)`. In `NewSafeHTTPClient`'s `CheckRedirect` handler, `MaxRedirects == 0` causes an immediate return via `http.ErrUseLastResponse` — meaning `validateRedirectTarget` is **never called** for any uptime monitor request. Adding RFC 1918 logic here would ship dead code with no test coverage.

Instead, add the following TODO comment at the top of `validateRedirectTarget` in `safeclient.go`:

```go
// TODO: if MaxRedirects > 0 is ever added to uptime monitor checks, also pass
// WithAllowRFC1918() into validateRedirectTarget so that redirect targets to
// RFC 1918 addresses are permitted for admin-configured monitor call sites.
```

*No other functions in `safeclient.go` require changes.*

---

#### File 2: `backend/internal/security/url_validator.go`

**Package:** `security`

**Change 1 — Add `AllowRFC1918` field to `ValidationConfig`**

Locate the `ValidationConfig` struct:
```go
type ValidationConfig struct {
    AllowLocalhost  bool
    AllowHTTP       bool
    MaxRedirects    int
    Timeout         time.Duration
    BlockPrivateIPs bool
}
```

Add the new field after `BlockPrivateIPs`:
```go
// AllowRFC1918 permits URLs that resolve to RFC 1918 private addresses.
// Does not disable blocking of loopback, link-local, cloud metadata, or reserved ranges.
AllowRFC1918 bool
```

**Change 2 — Add `WithAllowRFC1918` functional option**

Insert after the `WithMaxRedirects` function:

```go
// WithAllowRFC1918 permits hostnames that resolve to RFC 1918 private IP ranges
// (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16).
//
// SECURITY NOTE: Use only for admin-controlled call sites such as uptime monitors.
// Cloud metadata (169.254.169.254), link-local, loopback, and all reserved ranges
// are still blocked when this option is active.
func WithAllowRFC1918() ValidationOption {
    return func(c *ValidationConfig) { c.AllowRFC1918 = true }
}
```

**`ValidateExternalURL` initializer** — ensure the default `ValidationConfig` sets `AllowRFC1918: false` explicitly. The current initialization block already defaults unlisted bools to `false`, so no line change is required here.

**Change 3 — Update Phase 4 private IP blocking loop in `ValidateExternalURL`**

This is the critical logic change. Locate the IPv4-mapped IPv6 check block and the `IsPrivateIP` call inside the `if config.BlockPrivateIPs` block:

```go
if config.BlockPrivateIPs {
    for _, ip := range ips {
        if ip.To4() != nil && ip.To16() != nil && isIPv4MappedIPv6(ip) {
            ipv4 := ip.To4()
            if network.IsPrivateIP(ipv4) {
                return "", fmt.Errorf("connection to private ip addresses is blocked for security (detected IPv4-mapped IPv6: %s)", ip.String())
            }
        }

        if network.IsPrivateIP(ip) {
            sanitizedIP := sanitizeIPForError(ip.String())
            if ip.String() == "169.254.169.254" {
                return "", fmt.Errorf("access to cloud metadata endpoints is blocked for security (detected: %s)", sanitizedIP)
            }
            return "", fmt.Errorf("connection to private ip addresses is blocked for security (detected: %s)", sanitizedIP)
        }
    }
}
```

Replace with:

```go
if config.BlockPrivateIPs {
    for _, ip := range ips {
        // Handle IPv4-mapped IPv6 form (::ffff:a.b.c.d) to prevent SSRF bypass.
        if ip.To4() != nil && ip.To16() != nil && isIPv4MappedIPv6(ip) {
            ipv4 := ip.To4()
            // RFC 1918 bypass applies even in IPv4-mapped IPv6 form.
            if config.AllowRFC1918 && network.IsRFC1918(ipv4) {
                continue
            }
            if network.IsPrivateIP(ipv4) {
                return "", fmt.Errorf("connection to private ip addresses is blocked for security (detected IPv4-mapped IPv6: %s)", ip.String())
            }
        }

        // Admin-configured monitors may target LAN services; allow RFC 1918 only.
        // Link-local (169.254.x.x), loopback, cloud metadata, and reserved ranges
        // remain blocked unconditionally even when AllowRFC1918 is set.
        if config.AllowRFC1918 && network.IsRFC1918(ip) {
            continue
        }

        if network.IsPrivateIP(ip) {
            sanitizedIP := sanitizeIPForError(ip.String())
            if ip.String() == "169.254.169.254" {
                return "", fmt.Errorf("access to cloud metadata endpoints is blocked for security (detected: %s)", sanitizedIP)
            }
            return "", fmt.Errorf("connection to private ip addresses is blocked for security (detected: %s)", sanitizedIP)
        }
    }
}
```

*No other functions in `url_validator.go` require changes.*

---

#### File 3: `backend/internal/services/uptime_service.go`

**Package:** `services`

**Change 1 — `checkMonitor`: add `WithAllowRFC1918()` to URL validation**

Locate the `security.ValidateExternalURL` call inside the `case "http", "https":` branch:

```go
validatedURL, err := security.ValidateExternalURL(
    monitor.URL,
    // Uptime monitors are an explicit admin-configured feature and commonly
    // target loopback in local/dev setups (and in unit tests).
    security.WithAllowLocalhost(),
    security.WithAllowHTTP(),
    security.WithTimeout(3*time.Second),
)
```

Replace with:

```go
validatedURL, err := security.ValidateExternalURL(
    monitor.URL,
    // Uptime monitors are admin-configured; LAN targets are a legitimate use-case.
    security.WithAllowLocalhost(),
    security.WithAllowHTTP(),
    // Allow RFC 1918 private ranges for LAN service monitoring. Cloud metadata
    // (169.254.169.254), link-local, and loopback remain blocked.
    security.WithAllowRFC1918(),
    security.WithTimeout(3*time.Second),
)
```

**Change 2 — `checkMonitor`: add `WithAllowRFC1918()` to HTTP client**

Locate the `network.NewSafeHTTPClient` call immediately below the URL validation block:

```go
client := network.NewSafeHTTPClient(
    network.WithTimeout(10*time.Second),
    network.WithDialTimeout(5*time.Second),
    // Explicit redirect policy per call site: disable.
    network.WithMaxRedirects(0),
    // Uptime monitors are an explicit admin-configured feature and commonly
    // target loopback in local/dev setups (and in unit tests).
    network.WithAllowLocalhost(),
)
```

Replace with:

```go
client := network.NewSafeHTTPClient(
    network.WithTimeout(10*time.Second),
    network.WithDialTimeout(5*time.Second),
    // Explicit redirect policy per call site: disable.
    network.WithMaxRedirects(0),
    // Uptime monitors are admin-configured; LAN targets are a legitimate use-case.
    network.WithAllowLocalhost(),
    // Must mirror the WithAllowRFC1918() passed to ValidateExternalURL above.
    // Both the URL validator (DNS resolution) and the safe dialer (TCP connect)
    // enforce SSRF rules independently; both must be relaxed or the fix is partial.
    network.WithAllowRFC1918(),
)
```

**Change 3 — `checkMonitor`: annotate the TCP bypass**

Locate the TCP case:

```go
case "tcp":
    conn, err := net.DialTimeout("tcp", monitor.URL, 10*time.Second)
```

Add a comment above the dial line:

```go
case "tcp":
    // TCP monitors use net.DialTimeout directly, bypassing the URL validator and
    // safe dialer entirely. This is a deliberate design choice: TCP monitors accept
    // only admin-configured host:port strings (no URL parsing, no redirects, no DNS
    // rebinding surface), so the SSRF attack vector is minimal. If SSRF validation
    // is ever added to TCP monitors, it must also receive WithAllowRFC1918() so that
    // LAN services continue to be reachable.
    conn, err := net.DialTimeout("tcp", monitor.URL, 10*time.Second)
```

*No other functions in `uptime_service.go` require changes.*

---

### B. Option Pattern Design

The implementation uses two parallel functional-option systems that must be kept in sync at the call site. They share identical semantics but live in different packages for separation of concerns.

#### `ValidationConfig` and `ValidationOption` (in `security` package)

The existing struct gains one field:

```go
type ValidationConfig struct {
    AllowLocalhost  bool
    AllowHTTP       bool
    MaxRedirects    int
    Timeout         time.Duration
    BlockPrivateIPs bool
    AllowRFC1918    bool  // NEW: permits 10/8, 172.16/12, 192.168/16
}
```

New option constructor:

```go
func WithAllowRFC1918() ValidationOption {
    return func(c *ValidationConfig) { c.AllowRFC1918 = true }
}
```

#### `ClientOptions` and `Option` (in `network` package)

The existing struct gains one field:

```go
type ClientOptions struct {
    Timeout        time.Duration
    AllowLocalhost bool
    AllowedDomains []string
    MaxRedirects   int
    DialTimeout    time.Duration
    AllowRFC1918   bool  // NEW: permits 10/8, 172.16/12, 192.168/16
}
```

New option constructor and new exported predicate:

```go
func WithAllowRFC1918() Option {
    return func(opts *ClientOptions) { opts.AllowRFC1918 = true }
}

func IsRFC1918(ip net.IP) bool { /* see File 1, Change 2 above */ }
```

#### Coordination in `uptime_service.go`

The two options are always activated together. The ordering at the call site makes this explicit:

```go
security.ValidateExternalURL(
    monitor.URL,
    security.WithAllowLocalhost(),
    security.WithAllowHTTP(),
    security.WithAllowRFC1918(),   // ← Layer 1 relaxed
    security.WithTimeout(3*time.Second),
)

network.NewSafeHTTPClient(
    network.WithTimeout(10*time.Second),
    network.WithDialTimeout(5*time.Second),
    network.WithMaxRedirects(0),
    network.WithAllowLocalhost(),
    network.WithAllowRFC1918(),    // ← Layer 2 relaxed (must mirror Layer 1)
)
```

**Invariant:** Any future call site that enables `WithAllowRFC1918()` at Layer 1 MUST also enable it at Layer 2 (and vice-versa), or the fix will only partially work. The comments at the call site in `uptime_service.go` make this constraint explicit.

---

### C. Test Plan

All test changes are additive — no existing tests are modified.

#### `backend/internal/network/safeclient_test.go`

| # | Test Function | Scenario | Expected Result |
|---|--------------|----------|-----------------|
| 1 | `TestIsRFC1918_RFC1918Addresses` | Table-driven: `10.0.0.1`, `10.255.255.255`, `172.16.0.1`, `172.31.255.255`, `192.168.0.1`, `192.168.255.255` | `IsRFC1918` returns `true` for all |
| 2 | `TestIsRFC1918_NonRFC1918Addresses` | Table-driven: `169.254.169.254`, `127.0.0.1`, `::1`, `8.8.8.8`, `0.0.0.1`, `240.0.0.1`, `fe80::1`, `fc00::1` | `IsRFC1918` returns `false` for all |
| 3 | `TestIsRFC1918_NilIP` | `nil` IP | Returns `false` (nil is not RFC 1918; `IsPrivateIP` handles nil → block) |
| 4 | `TestIsRFC1918_BoundaryAddresses` | `172.15.255.255` (just outside range), `172.32.0.0` (just outside), `192.167.255.255` (just outside), `192.169.0.0` (just outside) | `IsRFC1918` returns `false` for all |
| 5 | `TestSafeDialer_AllowRFC1918_ValidationLoopSkipsRFC1918` | `ClientOptions{AllowRFC1918: true, DialTimeout: 1s}` (no `AllowLocalhost`), call `safeDialer` against a host that resolves to `192.168.1.1`; the TCP connect will fail (nothing listening), but the returned error must NOT contain `"connection to private IP blocked"` — absence of that string confirms the RFC 1918 bypass path was taken. White-box test in `package network` (`safeclient_test.go`). | Error does not contain `"connection to private IP blocked"`; confirms `if opts.AllowRFC1918 && IsRFC1918(ip.IP) { continue }` was executed. |
| 6 | `TestSafeDialer_AllowRFC1918_BlocksLinkLocal` | `ClientOptions{AllowRFC1918: true, DialTimeout: 1s}`, dial to `169.254.169.254:80` | Returns error containing "private IP blocked" |
| 7 | `TestSafeDialer_AllowRFC1918_BlocksLoopbackWithoutAllowLocalhost` | `ClientOptions{AllowRFC1918: true, AllowLocalhost: false, DialTimeout: 1s}`, dial to `127.0.0.1:80` | Returns error; loopback not covered by AllowRFC1918 |
| 8 | `TestNewSafeHTTPClient_AllowRFC1918_BlocksSSRFMetadata` | Create client with `WithAllowRFC1918()`, attempt GET to `http://169.254.169.254/` | Error; cloud metadata endpoint not accessible |
| 9 | `TestNewSafeHTTPClient_WithAllowRFC1918_OptionApplied` | Create client with `WithAllowRFC1918()`, verify `ClientOptions.AllowRFC1918` is `true` | Structural test — option propagates to config |
| 10 | `TestIsRFC1918_IPv4MappedAddresses` | Table-driven: `IsRFC1918(net.ParseIP("::ffff:192.168.1.1"))` → `true`; `IsRFC1918(net.ParseIP("::ffff:169.254.169.254"))` → `false`. White-box test in `package network` (`safeclient_test.go`). | `true` for IPv4-mapped RFC 1918; `false` for IPv4-mapped link-local/non-RFC-1918. Validates `To4()` normalization in `IsRFC1918`. |

**Implementation note for tests 5–9:** Test 5 (`TestSafeDialer_AllowRFC1918_ValidationLoopSkipsRFC1918`) dials a host resolving to `192.168.1.1` with no `AllowLocalhost`; the expected outcome is that the error does NOT match `"connection to private IP blocked"` (a TCP connection-refused error from nothing listening is acceptable — it means the validation loop passed the IP). For tests 6–9, real TCP connections to RFC 1918 addresses are unavailable in the CI environment. Those tests should use `httptest.NewServer` (which binds to loopback) combined with `AllowLocalhost: true` and `AllowRFC1918: true` to verify no *validation* error occurs. For tests that must confirm a block (e.g., `169.254.169.254`), the dialer is called directly with a short `DialTimeout` — the expected error is the SSRF block error, not a connection-refused error.

#### `backend/internal/security/url_validator_test.go`

| # | Test Function | Scenario | Expected Result |
|---|--------------|----------|-----------------|
| 11 | `TestValidateExternalURL_WithAllowRFC1918_Permits10x` | `http://10.0.0.1` with `WithAllowHTTP()` + `WithAllowRFC1918()` | Passes URL validation phase (may fail DNS in test env — accept `dns resolution failed` as OK since that means validation passed) |
| 12 | `TestValidateExternalURL_WithAllowRFC1918_Permits172_16x` | `http://172.16.0.1` with `WithAllowHTTP()` + `WithAllowRFC1918()` | Same as above |
| 13 | `TestValidateExternalURL_WithAllowRFC1918_Permits192_168x` | `http://192.168.1.100` with `WithAllowHTTP()` + `WithAllowRFC1918()` | Same as above |
| 14 | `TestValidateExternalURL_WithAllowRFC1918_BlocksMetadata` | `http://169.254.169.254` with `WithAllowHTTP()` + `WithAllowRFC1918()` | Returns error containing `"cloud metadata endpoints is blocked"` |
| 15 | `TestValidateExternalURL_WithAllowRFC1918_BlocksLinkLocal` | `http://169.254.10.1` with `WithAllowHTTP()` + `WithAllowRFC1918()` | Returns error containing `"private ip addresses is blocked"` |
| 16 | `TestValidateExternalURL_WithAllowRFC1918_BlocksLoopback` | `http://127.0.0.1` with `WithAllowHTTP()` + `WithAllowRFC1918()` (no `WithAllowLocalhost`) | Returns error; loopback not covered |
| 17 | `TestValidateExternalURL_RFC1918BlockedByDefault` | `http://192.168.1.1` with `WithAllowHTTP()` only (no RFC 1918 option) | Returns error containing `"private ip addresses is blocked"` — regression guard |
| 18 | `TestValidateExternalURL_WithAllowRFC1918_IPv4MappedIPv6Allowed` | `http://[::ffff:192.168.1.1]` with `WithAllowHTTP()` + `WithAllowRFC1918()` | Permit (RFC 1918 bypass applies to IPv4-mapped form too) |
| 19 | `TestValidateExternalURL_WithAllowRFC1918_IPv4MappedMetadataBlocked` | `http://[::ffff:169.254.169.254]` with `WithAllowHTTP()` + `WithAllowRFC1918()` | Blocked; cloud metadata remains rejected in mapped form |

**Implementation note for tests 11–13:** `ValidateExternalURL` calls `net.Resolver.LookupIP` on literal IP strings. In Go, `LookupIP` on a literal IP (e.g., `"192.168.1.1"`) returns that IP immediately without a real DNS query. So tests 11–13 should succeed at validation and return either a normalized URL (success) or a DNS timeout error if the test environment's resolver behaves unexpectedly. Both outcomes are acceptable; the important invariant is that the returned error must NOT contain `"private ip addresses is blocked"`.

#### `backend/internal/services/uptime_service_test.go`

| # | Test Function | Scenario | Expected Result |
|---|--------------|----------|-----------------|
| 20 | `TestCheckMonitor_HTTP_LocalhostSucceedsWithPrivateIPBypass` | Start an `httptest.NewServer` on loopback, create HTTP monitor pointing to its URL, call `checkMonitor` — verifies that `WithAllowLocalhost` + `WithAllowRFC1918` together produce a "up" result | Monitor status `"up"`, heartbeat message `"HTTP 200"` |
| 21 | `TestCheckMonitor_TCP_AcceptsRFC1918Address` | TCP monitor with `host:port` format pointing to a server listening on `127.0.0.1`, call `checkMonitor` | Success (TCP uses `net.DialTimeout`, no SSRF layer to relax) |

---

### D. Security Review Checklist

Every item below is a security property that the implementation must satisfy. Each entry names the property, which code enforces it, and how to verify it.

| # | Property | Enforced By | Verification Method |
|---|----------|-------------|---------------------|
| 1 | **Cloud metadata remains blocked.** `169.254.169.254` (AWS/GCP/Azure metadata service) is never reachable, even with `AllowRFC1918` active. | `IsRFC1918` returns `false` for `169.254.x.x` (link-local, not RFC 1918). Both `ValidateExternalURL` and `safeDialer` will still call `IsPrivateIP` which blocks `169.254.0.0/16`. | Test 8 + Test 14. |
| 2 | **Full link-local range blocked.** Not just `169.254.169.254` but the entire `169.254.0.0/16` range is blocked. | Same as #1. `IsPrivateIP` covers `169.254.0.0/16`. `IsRFC1918` excludes this range. | Test 6 + Test 15. |
| 3 | **Loopback does not gain blanket bypass.** `127.0.0.0/8` and `::1` are not unblocked by `AllowRFC1918`. Only `AllowLocalhost` can bypass loopback, and it is not added unexpectedly. | `IsRFC1918` only covers the three RFC 1918 ranges. Loopback is handled independently by `AllowLocalhost`. | Test 7 + Test 16. |
| 4 | **IPv6 unique-local and link-local remain blocked.** `fc00::/7` and `fe80::/10` are not unblocked. RFC 1918 is IPv4-only. | `IsRFC1918` converts to IPv4 via `To4()`; it returns `false` for all IPv6 addresses. | Test 2 (`fe80::1`, `fc00::1` in `TestIsRFC1918_NonRFC1918Addresses`). |
| 5 | **Reserved ranges remain blocked.** `0.0.0.0/8`, `240.0.0.0/4`, `255.255.255.255/32` are not unblocked. | Same as above — not in `rfc1918CIDRs`. | Test 2 (`0.0.0.1`, `240.0.0.1` in `TestIsRFC1918_NonRFC1918Addresses`). |
| 6 | **RFC 1918 bypass is bounded precisely.** Addresses just outside the three RFC 1918 ranges (e.g., `172.15.255.255`, `172.32.0.0`) are not treated as RFC 1918. | `net.ParseCIDR` + `block.Contains` provide exact CIDR boundary enforcement. | Test 4 (`TestIsRFC1918_BoundaryAddresses`). |
| 7 | **IPv4-mapped IPv6 addresses are handled.** `::ffff:192.168.1.1` is permitted with `AllowRFC1918`; `::ffff:169.254.169.254` is not. | `IsRFC1918` normalizes to IPv4 via `To4()` before CIDR check. The URL validator's IPv4-mapped branch also checks `IsRFC1918` before `IsPrivateIP`. Unit-level coverage provided by Test 10 (`TestIsRFC1918_IPv4MappedAddresses`). | Test 10 + Test 18 + Test 19. |
| 8 | **Option is not accessible to unauthenticated users.** The uptime monitor check routes are behind `authMiddleware` + `middleware.RequireManagementAccess()`. | `routes.go` nests uptime routes inside `management` group which is `protected.Group("/")` with `RequireManagementAccess()`. | Code review of `backend/internal/api/routes/routes.go` (confirmed: `management.POST("/uptime/monitors/:id/check", ...)` at line 461). |
| 9 | **Option is not applied to user-facing URL validation.** Webhook URLs, notification URLs, and other user-supplied inputs use `ValidateExternalURL` without `WithAllowRFC1918()`. | `WithAllowRFC1918()` is only added in `checkMonitor` in `uptime_service.go`. No other `ValidateExternalURL` call site is modified. | Grep all `ValidateExternalURL` call sites; verify only `uptime_service.go` carries `WithAllowRFC1918()`. |
| 10 | **Both layers are consistently relaxed.** If `WithAllowRFC1918()` is at Layer 1 (URL validator), it is also at Layer 2 (safe dialer). Partial bypass is not possible. | Comment in `uptime_service.go` creates a code-review anchor. | Cross-reference Layer 1 and Layer 2 call sites in `checkMonitor`. |
| 11 | **DNS rebinding is still defeated.** The safe dialer re-resolves the hostname at connect time and re-applies the same RFC 1918 policy. A hostname that resolves to a public IP during validation cannot be swapped for a private non-RFC-1918 IP at connect time. | `safeDialer` validates ALL resolved IPs against the same logic as the URL validator. `IsPrivateIP` is still called for non-RFC-1918 addresses. | Existing `TestSafeDialer_BlocksPrivateIPs` remains unchanged and continues to pass. |
| 12 | **`IsRFC1918(nil)` returns `false`, not `true`.** `IsPrivateIP(nil)` returns `true` (block-by-default safety). `IsRFC1918(nil)` should return `false` because `nil` is not an RFC 1918 address — it would fall through to `IsPrivateIP` which handles the nil case. | Early nil check in `IsRFC1918`: `if ip == nil { return false }`. | Test 3 (`TestIsRFC1918_NilIP`). |
| 13 | **`CheckMonitor` exported wrapper propagates the fix automatically.** The exported `CheckMonitor` method delegates directly to the unexported `checkMonitor`. All RFC 1918 option changes applied inside `checkMonitor` take effect for both entry points without separate configuration. | `uptime_service.go`: `CheckMonitor` calls `checkMonitor` without re-creating the HTTP client or invoking `ValidateExternalURL` independently. | Code review: verify `CheckMonitor` does not construct its own HTTP client or URL validation path outside of `checkMonitor`. |
| 14 | **Coordination invariant is comment-enforced; integration test can assert the contract.** The requirement that Layer 1 (`WithAllowRFC1918()` in `ValidateExternalURL`) and Layer 2 (`WithAllowRFC1918()` in `NewSafeHTTPClient`) are always relaxed together is documented via the inline comment at the `NewSafeHTTPClient` call site in `checkMonitor`. Partial bypass — relaxing only one layer — is not possible silently because the code-review anchor makes the intent explicit. A `TestCheckMonitor` integration test (Test 20) can additionally assert the "up" outcome to confirm both layers cooperate. | Comment in `uptime_service.go`: "Must mirror the `WithAllowRFC1918()` passed to `ValidateExternalURL` above." | Cross-reference both `WithAllowRFC1918()` call sites in `checkMonitor`; any future call site adding only one of the two options is a mis-use detectable at code review. |

---

### E. Commit Message

```
fix(uptime): allow RFC 1918 IPs for admin-configured monitors

HTTP/HTTPS uptime monitors targeting LAN addresses (e.g., 192.168.x.x,
10.x.x.x, 172.16.x.x) permanently reported "down" on fresh installs.
The SSRF protection layer silently blocked private IPs at two independent
checkpoints — URL validation and TCP dial time — causing monitors that
pointed to self-hosted LAN services to always fail.

Introduce WithAllowRFC1918() as a functional option in both the
url_validator (security package) and NewSafeHTTPClient / safeDialer
(network package). A new IsRFC1918() exported predicate in the network
package covers exactly the three RFC 1918 ranges without touching the
broader IsPrivateIP logic.

Apply WithAllowRFC1918() exclusively in checkMonitor() (uptime_service.go)
for the http/https case. Both layers are relaxed in concert; relaxing only
one produces a partial fix where URL validation passes but the TCP dialer
still blocks the connection.

Security properties preserved:
- 169.254.169.254 and the full 169.254.0.0/16 link-local range remain
  blocked unconditionally (not RFC 1918)
- Loopback (127.0.0.0/8, ::1) is not affected by this option
- IPv6 unique-local (fc00::/7) and link-local (fe80::/10) remain blocked
- Reserved ranges (0.0.0.0/8, 240.0.0.0/4, broadcast) remain blocked
- The bypass is only reachable by authenticated management-tier users
- No user-facing URL validation call site is modified

Add an explanatory comment at the TCP net.DialTimeout call site in
checkMonitor documenting the deliberate SSRF bypass for TCP monitors.

Fixes issues 6 and 7 from the fresh-install bug report.
```

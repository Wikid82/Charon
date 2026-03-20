# Fix: `should create new TCP monitor` Playwright Test Failure

**Status:** Ready to implement
**Scope:** Test-only fix — no production code changes
**Affected file:** `tests/monitoring/uptime-monitoring.spec.ts`

---

## Root Cause

The test at line 414 (`should create new TCP monitor`) fills the URL field with `tcp://redis.local:6379`, then selects type `tcp`. The PR **"fix(uptime): fix TCP monitor UX — correct format guidance and add client-side validation"** added a guard in `handleSubmit` inside `CreateMonitorModal` (`frontend/src/pages/Uptime.tsx`, ~line 373) that explicitly blocks form submission when the TCP URL contains a scheme prefix (`://`):

```js
if (type === 'tcp' && url.trim().includes('://')) {
    setUrlError(t('uptime.invalidTcpFormat'));
    return;  // ← early return; mutation.mutate() never called
}
```

Because `mutation.mutate()` is never called, the Axios POST to `/api/v1/uptime/monitors` is never sent. Playwright's `page.waitForResponse(...)` waiting on that POST times out, and the test fails consistently across all three retry attempts.

The same component also sets a real-time inline error via the URL `onChange` handler when `type === 'tcp'` and the value contains `://`. At the time the URL is filled in the test the type is still `'http'` (no real-time error), but when `handleSubmit` fires after the type has been switched to `'tcp'`, the guard catches it.

The validation enforces the correct TCP URL format: bare `host:port` with **no** scheme prefix (e.g. `redis.local:6379`, not `tcp://redis.local:6379`). This is documented in the translation string `uptime.urlHelperTcp` and the placeholder `uptime.urlPlaceholderTcp` (`192.168.1.1:8080`).

---

## Validation Chain (for reference)

| Location | File | ~Line | Guard |
|---|---|---|---|
| `handleSubmit` | `frontend/src/pages/Uptime.tsx` | 373 | `if (type === 'tcp' && url.trim().includes('://')) return;` |
| URL `onChange` | `frontend/src/pages/Uptime.tsx` | 392 | `if (type === 'tcp' && val.includes('://')) setUrlError(...)` |
| Backend binding | `backend/internal/api/handlers/uptime_handler.go` | 34 | `binding:"required,oneof=http tcp https"` — type only; no URL format check server-side |

The backend does **not** validate the TCP URL format — that enforcement lives entirely in the frontend. The test was written before the client-side validation was introduced and uses a URL that now fails that guard.

---

## Required Changes

### File: `tests/monitoring/uptime-monitoring.spec.ts`

Two lines need to change inside the `should create new TCP monitor` test block (lines 414–467).

#### Change 1 — URL fill value (line 447)

The URL passed to the form must be bare `host:port` format, matching what the validation permits.

**Before (line 447):**
```ts
await page.fill('input#create-monitor-url', 'tcp://redis.local:6379');
```

**After:**
```ts
await page.fill('input#create-monitor-url', 'redis.local:6379');
```

#### Change 2 — URL assertion (line 466)

The assertion verifying the payload sent to the API must reflect the corrected URL value.

**Before (line 466):**
```ts
expect(createPayload?.url).toBe('tcp://redis.local:6379');
```

**After:**
```ts
expect(createPayload?.url).toBe('redis.local:6379');
```

---

## Scope Confirmation — What Does NOT Need Changing

| Item | Reason |
|---|---|
| `mockMonitors[2].url` (`tcp://redis:6379`, line ~82) | Static fixture data returned by a mocked GET response. Never submitted through the form; never hits validation. Tests in `Monitor List Display` that display this monitor continue to pass. |
| `tests/fixtures/auth-fixtures.ts` | No TCP monitor creation logic; unaffected. |
| `tests/utils/wait-helpers.ts` | Utility helpers; unaffected. |
| `frontend/src/pages/Uptime.tsx` | Production code — must not be changed per scope constraint. |
| `backend/internal/api/handlers/uptime_handler.go` | Production code — must not be changed per scope constraint. |
| `frontend/src/api/uptime.ts` | API client; no change needed. |

---

## Verification Steps

After applying the two-line fix:

1. Run the specific failing test in Firefox to confirm it passes:
   ```bash
   cd /projects/Charon
   npx playwright test tests/monitoring/uptime-monitoring.spec.ts \
     --grep "should create new TCP monitor" \
     --project=firefox
   ```

2. Run the full CRUD describe block to confirm no regression in neighbouring tests:
   ```bash
   npx playwright test tests/monitoring/uptime-monitoring.spec.ts \
     --grep "Monitor CRUD Operations" \
     --project=firefox
   ```

3. Optionally run the full uptime spec across all shards to confirm end-to-end health.

---

## Commit Slicing Strategy

A single commit is appropriate — this is a two-line test fix with no branching concerns.

```
test(uptime): fix TCP monitor test to use bare host:port URL format

The should-create-new-TCP-monitor test was passing tcp://redis.local:6379
as the monitor URL. Client-side validation added in the TCP UX fix PR
blocks form submission when a TCP URL contains a scheme prefix,
so the POST to /api/v1/uptime/monitors never fired and waitForResponse
timed out.

Update the URL fill value and the corresponding payload assertion to use
the correct bare host:port format (redis.local:6379), which satisfies
the existing validation and matches the documented TCP address format.
```

**Branch:** `fix/tcp-monitor-e2e-url-format` → PR targeting `main`
**Files changed:** 1
**Lines changed:** 2

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

---

## PR-4: CrowdSec First-Enable UX (Issues 3 & 4)

**Title:** fix(frontend): stabilize CrowdSec first-enable UX and guard empty-value regression
**Issues Resolved:** Issue 3 (UI bugs on first enabling CrowdSec) + Issue 4 ("required value" error)
**Dependencies:** PR-1 (already merged — confirmed by code inspection)
**Status:** APPROVED (after Supervisor corrections applied)

---

### Overview

Two bugs compound to produce a broken first-enable experience. **Issue 4** (backend) is already
fixed: `UpdateSettingRequest.Value` no longer carries `binding:"required"` (confirmed in
`backend/internal/api/handlers/settings_handler.go` line 116 — the tag reads `json:"value"` with
no `binding` directive). PR-4 only needs a regression test to preserve this, plus a note in the
plan confirming it is done.

**Issue 3** (frontend) is the real work. When CrowdSec is first enabled, the
`crowdsecPowerMutation` in `Security.tsx` takes 10–60 seconds to complete. During this window:

1. **Toggle flicker** — `switch checked` reads `crowdsecStatus?.running ?? status.crowdsec.enabled`.
   Both sources lag behind user intent: `crowdsecStatus` is local state that hasn't been
   re-fetched yet (`null`), and `status.crowdsec.enabled` is the stale server value (`false` still,
   because `queryClient.invalidateQueries` fires only in `onSuccess`, which has not fired). The
   toggle therefore immediately reverts to unchecked the moment it is clicked.

2. **Stale "Disabled" badge** — The `<Badge>` inside the CrowdSec card reads the same condition
   and shows "Disabled" for the entire startup duration even though the user explicitly enabled it.

3. **Premature `CrowdSecKeyWarning`** — The warning is conditionally rendered at
   `Security.tsx` line ~355. The condition is `status.cerberus?.enabled && (crowdsecStatus?.running ?? status.crowdsec.enabled)`. During startup the condition may briefly evaluate `true` after
   `crowdsecStatus` is updated by `fetchCrowdsecStatus()` inside the mutation body, before bouncer
   registration completes on the backend, causing the key-rejection warning to flash.

4. **LAPI "not ready" / "not running" alerts in `CrowdSecConfig.tsx`** — If the user navigates to
   `/security/crowdsec` while the mutation is running, `lapiStatusQuery` (which polls every 5s) will
   immediately return `running: false` or `lapi_ready: false`. The 3-second `initialCheckComplete`
   guard is insufficient for a 10–60 second startup. The page shows an alarming red "CrowdSec not
   running" banner unnecessarily.

---

### A. Pre-flight: Issue 4 Verification and Regression Test

#### Confirmed Status

Open `backend/internal/api/handlers/settings_handler.go` at **line 115–121**. The current struct
is:

```go
type UpdateSettingRequest struct {
    Key      string `json:"key" binding:"required"`
    Value    string `json:"value"`
    Category string `json:"category"`
    Type     string `json:"type"`
}
```

`binding:"required"` is absent from `Value`. The backend fix is **complete**.

#### Handler Compensation: No Additional Key-Specific Validation Needed

Scan the `UpdateSetting` handler body (lines 127–250). The only value-level validation that exists
targets two specific keys:
- `security.admin_whitelist` → calls `validateAdminWhitelist(req.Value)` (line ~138)
- `caddy.keepalive_idle` / `caddy.keepalive_count` → calls `validateOptionalKeepaliveSetting` (line ~143)

Both already handle empty values gracefully by returning early or using zero-value defaults. No new
key-specific validation is required for the CrowdSec enable flow.

#### Regression Test to Add

**File:** `backend/internal/api/handlers/settings_handler_test.go`

**Test name:** `TestUpdateSetting_EmptyValueIsAccepted`

**Location in file:** Add to the existing `TestUpdateSetting*` suite. The file uses `package handlers_test` and already has a `mockCaddyConfigManager` / `mockCacheInvalidator` test harness.

**What it asserts:**

```
POST /settings  body: {"key":"security.crowdsec.enabled","value":""}
→ HTTP 200 (not 400)
→ DB contains a Setting row with Key="security.crowdsec.enabled" and Value=""
```

**Scaffolding pattern** (mirror the helpers already present in the test file):

```go
func TestUpdateSetting_EmptyValueIsAccepted(t *testing.T) {
    db := setupTestDB(t) // helper already in the test file
    h := handlers.NewSettingsHandler(db)
    router := setupTestRouter(h) // helper already in the test file

    body := `{"key":"security.crowdsec.enabled","value":""}`
    req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    // inject admin role as the existing test helpers do
    injectAdminContext(req) // helper pattern used across the file

    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code, "empty Value must not trigger a 400 validation error")

    var s models.Setting
    require.NoError(t, db.Where("key = ?", "security.crowdsec.enabled").First(&s).Error)
    assert.Equal(t, "", s.Value)
}
```

**Why this test matters:** Gin's `binding:"required"` treats the empty string `""` as a missing
value for `string` fields and returns 400. Without this test, re-adding the tag silently (e.g. by a
future contributor copying the `Key` field's annotation) would regress the fix without any CI
signal.

---

### B. Issue 3 Fix Plan — `frontend/src/pages/Security.tsx`

**File:** `frontend/src/pages/Security.tsx`
**Affected lines:** ~90 (state block), ~168–228 (crowdsecPowerMutation), ~228–240 (derived vars),
~354–357 (CrowdSecKeyWarning gate), ~413–415 (card Badge), ~418–420 (card icon), ~429–431 (card
body text), ~443 (Switch checked prop).

#### Change 1 — Derive `crowdsecChecked` from mutation intent

**Current code block** (lines ~228–232):

```tsx
const cerberusDisabled = !status.cerberus?.enabled
const crowdsecToggleDisabled = cerberusDisabled || crowdsecPowerMutation.isPending
const crowdsecControlsDisabled = cerberusDisabled || crowdsecPowerMutation.isPending
```

**Add immediately before `cerberusDisabled`:**

```tsx
// During the crowdsecPowerMutation, use the mutation's argument as the authoritative
// checked state. Neither crowdsecStatus (local, stale) nor status.crowdsec.enabled
// (server, not yet invalidated) reflects the user's intent until onSuccess fires.
const crowdsecChecked = crowdsecPowerMutation.isPending
  ? (crowdsecPowerMutation.variables ?? (crowdsecStatus?.running ?? status.crowdsec.enabled))
  : (crowdsecStatus?.running ?? status.crowdsec.enabled)
```

`crowdsecPowerMutation.variables` holds the `enabled: boolean` argument passed to `mutate()`. When
the user clicks to enable, `variables` is `true`; when they click to disable, it is `false`. This
is the intent variable that must drive the UI.

#### Change 2 — Replace every occurrence of the raw condition in the CrowdSec card

There are **six** places in the CrowdSec card (starting at line ~405) that currently read
`(crowdsecStatus?.running ?? status.crowdsec.enabled)`. All must be replaced with `crowdsecChecked`.

| Location | JSX attribute / expression | Before | After |
|----------|---------------------------|--------|-------|
| Line ~413 | `Badge variant` | `(crowdsecStatus?.running ?? status.crowdsec.enabled) ? 'success' : 'default'` | `crowdsecPowerMutation.isPending && crowdsecPowerMutation.variables ? 'warning' : crowdsecChecked ? 'success' : 'default'` |
| Line ~415 | `Badge text` | `(crowdsecStatus?.running ?? status.crowdsec.enabled) ? t('common.enabled') : t('common.disabled')` | `crowdsecPowerMutation.isPending && crowdsecPowerMutation.variables ? t('security.crowdsec.starting') : crowdsecChecked ? t('common.enabled') : t('common.disabled')` |
| Line ~418 | `div bg class` | `(crowdsecStatus?.running ?? status.crowdsec.enabled) ? 'bg-success/10' : 'bg-surface-muted'` | `crowdsecChecked ? 'bg-success/10' : 'bg-surface-muted'` |
| Line ~420 | `ShieldAlert text class` | `(crowdsecStatus?.running ?? status.crowdsec.enabled) ? 'text-success' : 'text-content-muted'` | `crowdsecChecked ? 'text-success' : 'text-content-muted'` |
| Line ~429 | `CardContent body text` | `(crowdsecStatus?.running ?? status.crowdsec.enabled) ? t('security.crowdsecProtects') : t('security.crowdsecDisabledDescription')` | `crowdsecChecked ? t('security.crowdsecProtects') : t('security.crowdsecDisabledDescription')` |
| Line ~443 | `Switch checked` | `crowdsecStatus?.running ?? status.crowdsec.enabled` | `crowdsecChecked` |

The `Badge` for the status indicator gets one additional case: the "Starting..." variant. Use
`variant="warning"` (already exists in the Badge component based on other usages in the file).

#### Change 3 — Suppress `CrowdSecKeyWarning` during mutation

**Current code** (lines ~353–357):

```tsx
{/* CrowdSec Key Rejection Warning */}
{status.cerberus?.enabled && (crowdsecStatus?.running ?? status.crowdsec.enabled) && (
  <CrowdSecKeyWarning />
)}
```

**Replace with:**

```tsx
{/* CrowdSec Key Rejection Warning — suppressed during startup to avoid flashing before bouncer registration completes */}
{status.cerberus?.enabled && !crowdsecPowerMutation.isPending && (crowdsecStatus?.running ?? status.crowdsec.enabled) && (
  <CrowdSecKeyWarning />
)}
```

The only change is `&& !crowdsecPowerMutation.isPending`. This prevents the warning from
rendering during the full 10–60s startup window.

#### Change 4 — Broadcast "starting" state to the QueryClient cache

`CrowdSecConfig.tsx` cannot read `crowdsecPowerMutation.isPending` directly — it lives in a
different component tree. The cleanest cross-component coordination mechanism in TanStack Query is
`queryClient.setQueryData` on a synthetic key. This is not an HTTP fetch; no network call occurs.
`CrowdSecConfig.tsx` consumes the value via `useQuery` with a stub `queryFn` and
`staleTime: Infinity`, which means it returns the cache value immediately.

**Add `onMutate` to `crowdsecPowerMutation`** (insert before `onError` at line ~199):

```tsx
onMutate: async (enabled: boolean) => {
  if (enabled) {
    queryClient.setQueryData(['crowdsec-starting'], { isStarting: true, startedAt: Date.now() })
  }
},
```

Note: The `if (enabled)` guard is intentional. The disable path does NOT set `isStartingUp` in CrowdSecConfig.tsx — when disabling CrowdSec, 'LAPI not running' banners are accurate and should not be suppressed.

**Modify `onError`** — add one line at the beginning of the existing handler body (line ~199):

```tsx
onError: (err: unknown, enabled: boolean) => {
  queryClient.setQueryData(['crowdsec-starting'], { isStarting: false })
  // ...existing error handling unchanged...
```

**Modify `onSuccess`** — add one line at the beginning of the existing handler body (line ~205):

```tsx
onSuccess: async (result: { lapi_ready?: boolean; enabled?: boolean } | boolean) => {
  queryClient.setQueryData(['crowdsec-starting'], { isStarting: false })
  // ...existing success handling unchanged...
```

The `startedAt` timestamp enables `CrowdSecConfig.tsx` to apply a safety cap: if the cache was
never cleared (e.g., the app crashed mid-mutation), the "is starting" signal expires after 90
seconds regardless.

---

### C. Issue 3 Fix Plan — `frontend/src/pages/CrowdSecConfig.tsx`

**File:** `frontend/src/pages/CrowdSecConfig.tsx`
**Affected lines:** ~44 (state block, after `queryClient`), ~582 (LAPI-initializing warning), ~608
(LAPI-not-running warning).

#### Change 1 — Read the "starting" signal

The file already declares `const queryClient = useQueryClient()` (line ~44). Insert immediately
after it:

```tsx
// Read the "CrowdSec is starting" signal broadcast by Security.tsx via the
// QueryClient cache. No HTTP call is made; this is pure in-memory coordination.
const { data: crowdsecStartingCache } = useQuery<{ isStarting: boolean; startedAt?: number }>({
  queryKey: ['crowdsec-starting'],
  queryFn: () => ({ isStarting: false, startedAt: 0 }),
  staleTime: Infinity,
  gcTime: Infinity,
})

// isStartingUp is true only while the mutation is genuinely running.
// The 90-second cap guards against stale cache if Security.tsx onSuccess/onError
// never fired (e.g., browser tab was closed mid-mutation).
const isStartingUp =
  (crowdsecStartingCache?.isStarting === true) &&
  Date.now() - (crowdsecStartingCache.startedAt ?? 0) < 90_000
```

#### Change 2 — Suppress the "LAPI initializing" yellow banner

**Current condition** (line ~582):

```tsx
{lapiStatusQuery.data && lapiStatusQuery.data.running && !lapiStatusQuery.data.lapi_ready && initialCheckComplete && (
```

**Replace with:**

```tsx
{lapiStatusQuery.data && lapiStatusQuery.data.running && !lapiStatusQuery.data.lapi_ready && initialCheckComplete && !isStartingUp && (
```

#### Change 3 — Suppress the "CrowdSec not running" red banner

**Current condition** (line ~608):

```tsx
{lapiStatusQuery.data && !lapiStatusQuery.data.running && initialCheckComplete && (
```

**Replace with:**

```tsx
{lapiStatusQuery.data && !lapiStatusQuery.data.running && initialCheckComplete && !isStartingUp && (
```

Both suppressions share the same `isStartingUp` flag derived in Change 1. When
`crowdsecPowerMutation` completes (or fails), `isStartingUp` immediately becomes `false`, and the
banners are re-evaluated based on real LAPI state.

---

### D. Issue 3 Fix Plan — `frontend/src/components/CrowdSecKeyWarning.tsx`

**No changes required to the component itself.** The suppression is fully handled at the call site
in `Security.tsx` (Section B, Change 3 above). The component's own render guard already returns
`null` if `isLoading` or `!keyStatus?.env_key_rejected`, which provides an additional layer of
safety. No new props are needed.

---

### E. i18n Requirements

#### New key: `security.crowdsec.starting`

This key drives the CrowdSec card badge text during the startup window. It must be added inside the
`security.crowdsec` namespace object in every locale file.

**Exact insertion point in each file:** Insert after the `"processStopped"` key inside the
`"crowdsec"` object (line ~252 in `en/translation.json`).

| Locale file | Key path | Value |
|-------------|----------|-------|
| `frontend/src/locales/en/translation.json` | `security.crowdsec.starting` | `"Starting..."` |
| `frontend/src/locales/de/translation.json` | `security.crowdsec.starting` | `"Startet..."` |
| `frontend/src/locales/es/translation.json` | `security.crowdsec.starting` | `"Iniciando..."` |
| `frontend/src/locales/fr/translation.json` | `security.crowdsec.starting` | `"Démarrage..."` |
| `frontend/src/locales/zh/translation.json` | `security.crowdsec.starting` | `"启动中..."` |

**Usage in `Security.tsx`:**

```tsx
t('security.crowdsec.starting')
```

No other new i18n keys are required. The `CrowdSecConfig.tsx` changes reuse the existing keys
`t('crowdsecConfig.lapiInitializing')`, `t('crowdsecConfig.notRunning')`, etc. — they are only
suppressed via the `isStartingUp` guard, not replaced.

---

### F. Test Plan

#### 1. Backend Unit Test (Regression Guard for Issue 4)

**File:** `backend/internal/api/handlers/settings_handler_test.go`
**Test type:** Go unit test (`go test ./backend/internal/api/handlers/...`)
**Mock requirements:** Uses the existing `setupTestDB` / `setupTestRouter` / `injectAdminContext`
helpers already present in the file. No additional mocks needed.

| Test name | Assertion | Pass condition |
|-----------|-----------|----------------|
| `TestUpdateSetting_EmptyValueIsAccepted` | POST `{"key":"security.crowdsec.enabled","value":""}` returns HTTP 200 and the DB row has `Value=""` | HTTP 200, no 400 "required" error |
| `TestUpdateSetting_MissingKeyRejected` | POST `{"value":"true"}` (no `key` field) returns HTTP 400 | HTTP 400 — `Key` still requires `binding:"required"` |

The second test ensures the `binding:"required"` was only removed from `Value`, not accidentally
from `Key` as well.

#### 2. Frontend RTL Tests

**Framework:** Vitest + React Testing Library (same as `frontend/src/hooks/__tests__/useSecurity.test.tsx`)

##### File: `frontend/src/pages/__tests__/Security.crowdsec.test.tsx` (new file)

**Scaffolding pattern:** Mirror the setup in `useSecurity.test.tsx` — `QueryClientProvider` wrapper,
`vi.mock('../api/crowdsec')`, `vi.mock('../api/settings')`, `vi.mock('../api/security')`,
`vi.mock('../hooks/useSecurity')`.

| Test name | What is mocked | What is rendered | Assertion |
|-----------|---------------|-----------------|-----------|
| `toggle stays checked while crowdsecPowerMutation is pending` | `startCrowdsec` never resolves (pending promise). `getSecurityStatus` returns `{ cerberus: { enabled: true }, crowdsec: { enabled: false }, ... }`. `statusCrowdsec` returns `{ running: false, pid: 0, lapi_ready: false }`. | `<Security />` | Click the CrowdSec toggle → `Switch[data-testid="toggle-crowdsec"]` remains checked (`aria-checked="true"`) while mutation is pending. Without the fix, it would be unchecked. |
| `CrowdSec badge shows "Starting..." while mutation is pending` | Same as above | `<Security />` | Click toggle → Badge inside the CrowdSec card contains text "Starting...". |
| `CrowdSecKeyWarning is not rendered while crowdsecPowerMutation is pending` | Same as above. `getCrowdsecKeyStatus` returns `{ env_key_rejected: true, full_key: "abc" }`. | `<Security />` | Click toggle → `CrowdSecKeyWarning` (identified by its unique title text or `data-testid` if added) is not present in the DOM. |
| `toggle reflects correct final state after mutation succeeds` | `startCrowdsec` resolves `{ pid: 123, lapi_ready: true }`. `statusCrowdsec` returns `{ running: true, pid: 123, lapi_ready: true }`. | `<Security />` | After mutation resolves → toggle is checked, badge shows "Enabled". |
| `toggle reverts to unchecked when mutation fails` | `startCrowdsec` rejects with `new Error("failed")`. | `<Security />` | After rejection → toggle is unchecked, badge shows "Disabled". |

##### File: `frontend/src/pages/__tests__/CrowdSecConfig.crowdsec.test.tsx` (new file)

**Required mocks:** `vi.mock('../api/crowdsec')`, `vi.mock('../api/security')`,
`vi.mock('../api/featureFlags')`. Seed the `QueryClient` with
`queryClient.setQueryData(['crowdsec-starting'], { isStarting: true, startedAt: Date.now() })` before rendering.

REQUIRED: Because `initialCheckComplete` is driven by a `setTimeout(..., 3000)` inside a `useEffect`, tests must use Vitest fake timers. Without this, positive-case tests will fail and suppression tests will vacuously pass:

```ts
beforeEach(() => {
  vi.useFakeTimers()
})
afterEach(() => {
  vi.useRealTimers()
})
// In each test, after render(), advance timers past the 3s guard:
await vi.advanceTimersByTimeAsync(3001)
```

| Test name | Setup | Assertion |
|-----------|-------|-----------|
| `LAPI not-running banner suppressed when isStartingUp is true` | `lapiStatusQuery` loaded with `{ running: false, lapi_ready: false }`. `['crowdsec-starting']` cache: `{ isStarting: true, startedAt: Date.now() }`. `initialCheckComplete` timer fires normally. | `[data-testid="lapi-not-running-warning"]` is not present in DOM. |
| `LAPI initializing banner suppressed when isStartingUp is true` | `lapiStatusQuery` loaded with `{ running: true, lapi_ready: false }`. `['crowdsec-starting']` cache: `{ isStarting: true, startedAt: Date.now() }`. | `[data-testid="lapi-warning"]` is not present in DOM. |
| `LAPI not-running banner shows after isStartingUp expires` | `['crowdsec-starting']` cache: `{ isStarting: true, startedAt: Date.now() - 100_000 }` (100s ago, past 90s cap). `lapiStatusQuery` loaded with `{ running: false, lapi_ready: false }`. `initialCheckComplete` = true. | `[data-testid="lapi-not-running-warning"]` is present in DOM. |
| `LAPI not-running banner shows when isStartingUp is false` | `['crowdsec-starting']` cache: `{ isStarting: false }`. `lapiStatusQuery`: `{ running: false, lapi_ready: false }`. `initialCheckComplete` = true. | `[data-testid="lapi-not-running-warning"]` is present in DOM. |

#### 3. Playwright E2E Tests

E2E testing for CrowdSec startup UX is constrained because the Docker E2E environment does not have
CrowdSec installed. The mutations will fail immediately, making it impossible to test the "pending"
window with a real startup delay.

**Recommended approach:** UI-only behavioral tests that mock the mutation pending state at the API
layer (via Playwright route interception), focused on the visible symptoms.

**File:** `playwright/tests/security/crowdsec-first-enable.spec.ts` (new file)

| Test title | Playwright intercept | Steps | Assertion |
|------------|---------------------|-------|-----------|
| `CrowdSec toggle stays checked while starting` | Intercept `POST /api/v1/admin/crowdsec/start` — respond after a 2s delay with success | Navigate to `/security`, click CrowdSec toggle | `[data-testid="toggle-crowdsec"]` has `aria-checked="true"` immediately after click (before response) |
| `CrowdSec card shows Starting badge while starting` | Same intercept | Click toggle | A badge with text "Starting..." is visible in the CrowdSec card |
| `CrowdSecKeyWarning absent while starting` | Same intercept; also intercept `GET /api/v1/admin/crowdsec/key-status` → return `{ env_key_rejected: true, full_key: "key123", ... }` | Click toggle | The key-warning alert (ARIA role "alert" with heading "CrowdSec API Key Updated") is not present |
| `Backend rejects empty key for setting` | No intercept | POST `{"key":"security.crowdsec.enabled","value":""}` via `page.evaluate` (or `fetch`) | Response code is 200 |

---

### G. Commit Slicing Strategy

**Decision:** Single PR (`PR-4`).

**Rationale:**
- The backend change is a one-line regression test addition — not a fix (the fix is already in).
- The frontend changes are all tightly coupled: the `crowdsecChecked` derived variable feeds both
  the toggle fix and the badge fix; the `onMutate` broadcast is consumed by `CrowdSecConfig.tsx`.
  Splitting them would produce an intermediate state where `Security.tsx` broadcasts a signal that
  nothing reads, or `CrowdSecConfig.tsx` reads a signal that is never set.
- Total file count: 5 files changed (`Security.tsx`, `CrowdSecConfig.tsx`, `en/translation.json`,
  `de/translation.json`, `es/translation.json`, `fr/translation.json`, `zh/translation.json`) +
  2 new test files + 1 new test in the backend handler test file. Review surface is small.

**Files changed:**

| File | Change type |
|------|-------------|
| `frontend/src/pages/Security.tsx` | Derived state, onMutate, suppression |
| `frontend/src/pages/CrowdSecConfig.tsx` | useQuery cache read, conditional suppression |
| `frontend/src/locales/en/translation.json` | New key `security.crowdsec.starting` |
| `frontend/src/locales/de/translation.json` | New key `security.crowdsec.starting` |
| `frontend/src/locales/es/translation.json` | New key `security.crowdsec.starting` |
| `frontend/src/locales/fr/translation.json` | New key `security.crowdsec.starting` |
| `frontend/src/locales/zh/translation.json` | New key `security.crowdsec.starting` |
| `frontend/src/pages/__tests__/Security.crowdsec.test.tsx` | New RTL test file |
| `frontend/src/pages/__tests__/CrowdSecConfig.crowdsec.test.tsx` | New RTL test file |
| `backend/internal/api/handlers/settings_handler_test.go` | 2 new test functions |
| `playwright/tests/security/crowdsec-first-enable.spec.ts` | New E2E spec file |

**Rollback:** The PR is independently revertable. No database migrations. No API contract changes.
The `['crowdsec-starting']` QueryClient key is ephemeral (in-memory only); removing the PR removes
the key cleanly.

---

### H. Acceptance Criteria

| # | Criterion | How to verify |
|---|-----------|---------------|
| 1 | POST `{"key":"any.key","value":""}` returns HTTP 200 | `TestUpdateSetting_EmptyValueIsAccepted` passes |
| 2 | CrowdSec toggle shows the user's intended state immediately after click, for the full pending duration | RTL test `toggle stays checked while crowdsecPowerMutation is pending` passes |
| 3 | CrowdSec card badge shows "Starting..." text while mutation is pending | RTL test `CrowdSec badge shows Starting... while mutation is pending` passes |
| 4 | `CrowdSecKeyWarning` is not rendered while `crowdsecPowerMutation.isPending` | RTL test `CrowdSecKeyWarning is not rendered while crowdsecPowerMutation is pending` passes |
| 5 | LAPI "not running" red banner absent on `CrowdSecConfig` while `isStartingUp` is true | RTL test `LAPI not-running banner suppressed when isStartingUp is true` passes |
| 6 | LAPI "initializing" yellow banner absent on `CrowdSecConfig` while `isStartingUp` is true | RTL test `LAPI initializing banner suppressed when isStartingUp is true` passes |
| 7 | Both banners reappear correctly after the 90s cap or after mutation completes | RTL test `LAPI not-running banner shows after isStartingUp expires` passes |
| 8 | Translation key `security.crowdsec.starting` exists in all 5 locale files | CI lint / i18n-check passes |
| 9 | Playwright: toggle does not flicker on click (stays `aria-checked="true"` during delayed API response) | E2E test `CrowdSec toggle stays checked while starting` passes |
| 10 | No regressions in existing `useSecurity.test.tsx` or other security test suites | Full Vitest suite green |

---

### I. Commit Message

```
fix(frontend): stabilize CrowdSec first-enable UX and guard empty-value regression

When CrowdSec is first enabled, the 10–60 second startup window caused the
toggle to immediately flicker back to unchecked, the card badge to show
"Disabled" throughout startup, the CrowdSecKeyWarning to flash before bouncer
registration completed, and CrowdSecConfig to show alarming LAPI-not-ready
banners at the user.

Root cause: the toggle, badge, and warning conditions all read from stale
sources (crowdsecStatus local state and status.crowdsec.enabled server data),
neither of which reflects user intent during a pending mutation.

Derive a crowdsecChecked variable from crowdsecPowerMutation.variables during
the pending window so the UI reflects intent, not lag. Suppress
CrowdSecKeyWarning unconditionally while the mutation is pending. Show a
"Starting..." badge variant (warning) during startup.

Coordinate the "is starting" state to CrowdSecConfig.tsx via a synthetic
QueryClient cache key ['crowdsec-starting'] set in onMutate and cleared in
onSuccess/onError. CrowdSecConfig reads this key via useQuery and uses it to
suppress the LAPI-not-running and LAPI-initializing alerts during startup.
A 90-second safety cap prevents stale suppression if the mutation never resolves.

Also add a regression test confirming that UpdateSettingRequest accepts an empty
string Value (the binding:"required" tag was removed in PR-1; this test ensures
it is not re-introduced).

Adds security.crowdsec.starting i18n key to all 5 supported locales.

Closes issue 3, closes issue 4 (regression test only, backend fix in PR-1).
```

---

## PR-5: TCP Monitor UX — Issue 5

**Title:** fix(frontend): correct TCP monitor URL placeholder and add per-type UX guidance
**Issue Resolved:** Issue 5 — "monitor tcp port ui can't add"
**Classification:** CONFIRMED BUG
**Dependencies:** None (independent of all other PRs)
**Status:** READY FOR IMPLEMENTATION

---

### 1. Introduction

A user attempting to create a TCP uptime monitor follows the URL input placeholder text,
which currently reads `"https://example.com or tcp://host:port"`. They enter
`tcp://192.168.1.1:8080` as instructed and submit the form. The creation silently fails
with an HTTP 500 error. The form provides no guidance on why — no inline error, no revised
placeholder, no helper text.

This plan specifies four layered fixes ranging from the single-line minimum viable fix to
comprehensive per-type UX guidance and client-side validation. All four fixes belong in a
single small PR.

---

### 2. Research Findings

#### 2.1 Root Cause: `net.SplitHostPort` Chokes on the `tcp://` Scheme

`net.SplitHostPort` in Go's standard library splits `"host:port"` on the last colon. When the
host portion itself contains a colon (as `"tcp://192.168.1.1"` does after the `://`), Go's
implementation returns `"too many colons in address"`.

Full trace:

```
frontend: placeholder says "tcp://host:port"
  ↓
user enters: tcp://192.168.1.1:8080
  ↓
POST /api/v1/uptime/monitors  { name: "…", url: "tcp://192.168.1.1:8080", type: "tcp", … }
  ↓
uptime_handler.go:42  c.ShouldBindJSON(&req)           → OK (all required fields present)
  ↓
uptime_service.go:1107 url.Parse("tcp://192.168.1.1:8080") → OK (valid URL, scheme=tcp)
  ↓
uptime_service.go:1122 net.SplitHostPort("tcp://192.168.1.1:8080")
  → host = "tcp://192.168.1.1"   ← contains a colon (the : after tcp)
  → Go: byteIndex(host, ':') >= 0 → return addrErr(…, "too many colons in address")
  ↓
uptime_service.go:1123 returns fmt.Errorf("TCP URL must be in host:port format: %w", err)
  ↓
uptime_handler.go:51  c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
  → HTTP 500 {"error":"TCP URL must be in host:port format: address tcp://192.168.1.1:8080: too many colons in address"}
  ↓
frontend onError: toast.error("TCP URL must be in host:port format: …")
  → user sees a cryptic toast, form stays open, no inline guidance
```

`net.SplitHostPort` accepts **only** the bare `host:port` form. Valid examples:
`192.168.1.1:8080`, `myserver.local:22`, `[::1]:8080` (IPv6). The scheme prefix `tcp://`
is not valid and is never stripped by the backend.

#### 2.2 The Unit Test Pre-Empted the Fix But It Was Never Applied

`frontend/src/pages/__tests__/Uptime.test.tsx` at **line 40** already mocks:
```typescript
'uptime.urlPlaceholder': 'https://example.com or host:port',
```
The test was written with the corrected value in mind. The actual translation file was
never updated to match. This is a direct discrepancy that CI does not catch because the
unit test mocks the i18n layer — it never reads the real translation file.

#### 2.3 State of All Locale Files

`frontend/src/locales/en/translation.json` is the **only** locale file that contains the
newer uptime keys added when the Create Monitor modal was built. The other four locales
(de, fr, es, zh) all fall through to the English key values for these keys (i18next
fallback). The `urlPlaceholder` key does not exist in any non-English locale — they
inherit the English value verbatim, which is the broken one.

Missing keys across **all** non-English locales (de, fr, es, zh):

```
uptime.addMonitor           uptime.syncWithHosts       uptime.createMonitor
uptime.monitorCreated       uptime.syncComplete        uptime.syncing
uptime.monitorType          uptime.monitorUrl          uptime.monitorTypeHttp
uptime.monitorTypeTcp       uptime.urlPlaceholder      uptime.autoRefreshing
uptime.noMonitorsFound      uptime.triggerHealthCheck  uptime.monitorSettings
uptime.failedToDeleteMonitor                           uptime.failedToUpdateMonitor
uptime.failedToTriggerCheck uptime.pendingFirstCheck   uptime.last60Checks
uptime.unpaused             uptime.deleteConfirmation  uptime.loadingMonitors
```

**PR-5 addresses all new keys added by this PR** (see §5.1 below). The broader backfill of
missing non-English uptime keys is a separate i18n debt task outside PR-5's scope.

#### 2.4 Component Architecture — Exact Locations in `Uptime.tsx`

The `CreateMonitorModal` component occupies lines 342–476 of
`frontend/src/pages/Uptime.tsx`.

Key lines within the component:

| Line | Code |
|------|------|
| 342 | `const CreateMonitorModal: FC<{ onClose: () => void; t: (key: string) => string }> = ({ onClose, t }) => {` |
| 345 | `const [url, setUrl] = useState('');` |
| 346 | `const [type, setType] = useState<'http' \| 'tcp'>('http');` |
| 362 | `if (!name.trim() \|\| !url.trim()) return;` (`handleSubmit` guard) |
| 363 | `mutation.mutate({ name: name.trim(), url: url.trim(), type, interval, max_retries: maxRetries });` |
| 395–405 | URL `<label>` + `<input>` block |
| 413 | `placeholder={t('uptime.urlPlaceholder')}` ← **bug is here** |
| 414 | `/>` closing the URL input |
| 415 | `</div>` closing the URL field wrapper |
| 416–430 | Type `<label>` + `<select>` block |
| 427 | `<option value="http">{t('uptime.monitorTypeHttp')}</option>` |
| 428 | `<option value="tcp">{t('uptime.monitorTypeTcp')}</option>` |

**Critical UX observation:** The type selector (line 416) appears **after** the URL input
(line 395) in the form. The user fills in the URL without knowing which format to use
before they have selected the type. Fix 2 (dynamic placeholder) partially addresses this,
but the ideal fix also reorders the fields so type comes first. This reordering is included
as part of the implementation below.

#### 2.5 No Existing Client-Side Validation

The `handleSubmit` function (line 360–364) only guards against empty `name` and `url`
strings. There is no format validation. A `tcp://` prefixed URL passes this check and is
sent directly to the backend.

No `data-testid` attributes exist on either the URL input or the type selector. They are
identified by `id="create-monitor-url"` and `id="create-monitor-type"` respectively.

#### 2.6 Backend `https` Type Gap (Out of Scope)

The backend `CreateMonitorRequest` accepts `type: "oneof=http tcp https"`. The frontend
type selector only offers `http` and `tcp`. HTTPS monitors are unsupported from the UI.
This is a separate issue; PR-5 does not add the HTTPS option.

#### 2.7 What the Backend Currently Returns

When submitted with `tcp://192.168.1.1:8080`:

- **HTTP status:** 500
- **Body:** `{"error":"TCP URL must be in host:port format: address tcp://192.168.1.1:8080: too many colons in address"}`
- **Frontend effect:** `toast.error(err.message)` fires with the full error string — too technical for a novice user.

No backend changes are required for PR-5. The backend validation is correct; only the
frontend must be fixed.

---

### 3. Technical Specifications

#### 3.1 Fix Inventory

| # | Fix | Scope | Mandatory |
|---|-----|-------|-----------|
| Fix 1 | Correct `urlPlaceholder` key in `en/translation.json` | i18n only | ✅ Yes |
| Fix 2 | Dynamic placeholder per type (`urlPlaceholderHttp` / `urlPlaceholderTcp`) | i18n + React | ✅ Yes |
| Fix 3 | Helper text per type below URL input | i18n + React | ✅ Yes |
| Fix 4 | Client-side `tcp://` scheme guard (onChange + submit) | React only | ✅ Yes |
| Fix 5 | Reorder form fields: type selector before URL input | React only | ✅ Yes |

All five fixes ship in a single PR. They are listed as separate items for reviewer clarity,
not as separate commits.

---

#### 3.2 Fix 1 — Correct `en/translation.json` (Minimum Viable Fix)

**File:** `frontend/src/locales/en/translation.json`
**Line:** 480

| Field | Before | After |
|-------|--------|-------|
| `uptime.urlPlaceholder` | `"https://example.com or tcp://host:port"` | `"https://example.com"` |

The value is narrowed to HTTP-only. The TCP-specific case is handled by Fix 2's new key.
The `urlPlaceholder` key is kept (not removed) for backward compatibility with any external
tooling or test that still references it; it will serve as the HTTP fallback.

---

#### 3.3 Fix 2 — Two New Per-Type Placeholder Keys

**File:** `frontend/src/locales/en/translation.json`
**Insertion point:** Immediately after the updated `urlPlaceholder` key (after new line 480).

New keys:

| Key | Value |
|-----|-------|
| `uptime.urlPlaceholderHttp` | `"https://example.com"` |
| `uptime.urlPlaceholderTcp` | `"192.168.1.1:8080"` |

**Component change — `frontend/src/pages/Uptime.tsx`:**

Inside `CreateMonitorModal`, add a derived constant after the state declarations
(insert after line 348, i.e., after `const [maxRetries, setMaxRetries] = useState(3);`):

```tsx
const urlPlaceholder = type === 'tcp'
  ? t('uptime.urlPlaceholderTcp')
  : t('uptime.urlPlaceholderHttp');
```

At line 413, change:
```tsx
placeholder={t('uptime.urlPlaceholder')}
```
to:
```tsx
placeholder={urlPlaceholder}
```

The `urlPlaceholder` key in `en/translation.json` no longer feeds the UI directly. It is
retained for backward compatibility only.

**Effect observed by the user:** When "HTTP" is selected (default), the URL input shows
`https://example.com`. When "TCP" is selected, it switches immediately to `192.168.1.1:8080`
— revealing the correct bare `host:port` format with no `tcp://` prefix.

---

#### 3.4 Fix 3 — Helper Text Per Type

**New i18n keys** in `frontend/src/locales/en/translation.json`
(insert after `urlPlaceholderTcp`):

| Key | Value |
|-----|-------|
| `uptime.urlHelperHttp` | `"Enter the full URL including the scheme (e.g., https://example.com)"` |
| `uptime.urlHelperTcp` | `"Enter as host:port with no scheme prefix (e.g., 192.168.1.1:8080 or hostname:22)"` |

**Component change — `frontend/src/pages/Uptime.tsx`:**

Add `aria-describedby="create-monitor-url-helper"` to the URL `<input>` at line 401. The
full input element becomes:

```tsx
<input
    id="create-monitor-url"
    type="text"
    value={url}
    onChange={…}
    required
    aria-describedby="create-monitor-url-helper"
    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
    placeholder={urlPlaceholder}
/>
```

Immediately after `/>` (turning line 414 into multiple lines), insert:

```tsx
<p
    id="create-monitor-url-helper"
    className="text-xs text-gray-500 mt-1"
>
    {type === 'tcp' ? t('uptime.urlHelperTcp') : t('uptime.urlHelperHttp')}
</p>
```

**Effect:** Static, always-visible helper text below the URL field. Changes in real time as
the user switches the type selector.

---

#### 3.5 Fix 4 — Client-Side `tcp://` Scheme Guard

**Component change — `frontend/src/pages/Uptime.tsx`:**

Add a new state variable after the existing `useState` declarations (after line 348):

```tsx
const [urlError, setUrlError] = useState('');
```

Modify the URL `<input>` `onChange` handler. Current:

```tsx
onChange={(e) => setUrl(e.target.value)}
```

Replace with:

```tsx
onChange={(e) => {
    const val = e.target.value;
    setUrl(val);
    if (type === 'tcp' && val.includes('://')) {
        setUrlError(t('uptime.invalidTcpFormat'));
    } else {
        setUrlError('');
    }
}}
```

Render the inline error **below the helper text** `<p>` (after it, inside the same `<div>`):

```tsx
{urlError && (
    <p
        id="create-monitor-url-error"
        className="text-xs text-red-400 mt-1"
        role="alert"
    >
        {urlError}
    </p>
)}
```

Update `aria-describedby` on the input to include the error id when it is present:

```tsx
aria-describedby={`create-monitor-url-helper${urlError ? ' create-monitor-url-error' : ''}`}
```

Modify `handleSubmit` to block submission when the URL format is invalid for TCP:

```tsx
const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !url.trim()) return;
    if (type === 'tcp' && url.trim().includes('://')) {
        setUrlError(t('uptime.invalidTcpFormat'));
        return;
    }
    mutation.mutate({ name: name.trim(), url: url.trim(), type, interval, max_retries: maxRetries });
};
```

**New i18n key** in `frontend/src/locales/en/translation.json`
(insert after `urlHelperTcp`):

| Key | Value |
|-----|-------|
| `uptime.invalidTcpFormat` | `"TCP monitors require host:port format. Remove the scheme prefix (e.g., use 192.168.1.1:8080, not tcp://192.168.1.1:8080)."` |

**Effect:** As soon as the user types `tcp://` while the type selector is set to TCP, the
error appears inline beneath the URL input. The Submit button remains enabled (feedback-first
rather than lock-first), but `handleSubmit` re-validates and blocks the API call if the error
is still present. The error clears the moment the user removes the `://` characters.

**Validation logic rationale:** The check `url.includes('://')` is deliberately simple.
It catches the exact failure mode: any scheme prefix (not just `tcp://`). It does NOT
attempt to validate that the remaining `host:port` is well-formed — the backend already
does that, and producing two layers of structural validation for the host/port part is
over-engineering for this PR.

---

#### 3.6 Fix 5 — Reorder Form Fields (Type Before URL)

**Rationale:** The user must know the expected URL format before they can fill in the URL
field. Placing the type selector before the URL input lets the placeholder and helper text
be correct from the moment the user tabs to the URL field.

**Change:** In `frontend/src/pages/Uptime.tsx`, within `CreateMonitorModal`'s `<form>`,
move the type `<div>` block (currently lines 416–430) to appear immediately **before** the
URL `<div>` block (currently lines 395–415). The name `<div>` remains first.

**New ordering of `<form>` fields:**
1. Name (unchanged — lines 389–394)
2. **Type selector** (moved from lines 416–430 to here)
3. URL input with helper/error (adjusted, now after type)
4. Check interval (unchanged)
5. Max retries (unchanged)

The `type` state variable and `setType` already exist — no state change is needed. The
`urlPlaceholder` derived constant (added in Fix 2) already reads from `type`, so the
reorder gives correct placeholder behavior automatically.

**SUPERVISOR BLOCKING FIX (BLOCKING-1):** The type selector's `onChange` must also clear
`urlError` to prevent a stale error message persisting after the user switches from TCP to
HTTP. Without this fix, a user who: (1) selects TCP → (2) types `tcp://...` → (3) sees the
error → (4) switches back to HTTP will continue to see the TCP format error even though HTTP
mode accepts full URLs. Fix: update the type `<select>` `onChange`:

```tsx
// Before (current):
onChange={(e) => setType(e.target.value as 'http' | 'tcp')}

// After (required):
onChange={(e) => {
    setType(e.target.value as 'http' | 'tcp');
    setUrlError('');
}}
```

A corresponding RTL test must be added: *"inline error clears when type changes from TCP to HTTP"*
and a Playwright scenario: *"Inline error clears when type changed from TCP to HTTP"*.
This brings the RTL test count from 9 → 10 and the Playwright test count from 8 → 9.

---

#### 3.7 i18n: Complete Key Table

All new/changed keys for all locale files:

##### `frontend/src/locales/en/translation.json`

| Key path | Change | New value |
|----------|--------|-----------|
| `uptime.urlPlaceholder` | Changed (line 480) | `"https://example.com"` |
| `uptime.urlPlaceholderHttp` | New | `"https://example.com"` |
| `uptime.urlPlaceholderTcp` | New | `"192.168.1.1:8080"` |
| `uptime.urlHelperHttp` | New | `"Enter the full URL including the scheme (e.g., https://example.com)"` |
| `uptime.urlHelperTcp` | New | `"Enter as host:port with no scheme prefix (e.g., 192.168.1.1:8080 or hostname:22)"` |
| `uptime.invalidTcpFormat` | New | `"TCP monitors require host:port format. Remove the scheme prefix (e.g., use 192.168.1.1:8080, not tcp://192.168.1.1:8080)."` |

**Exact insertion point:** All five new keys are inserted inside the `"uptime": {}` object,
after `"urlPlaceholder"` (current line 480) and before `"pending"` (current line 481). The
insertion adds 6 lines in total.

##### `frontend/src/locales/de/translation.json`

| Key path | New value |
|----------|-----------|
| `uptime.urlPlaceholder` | `"https://example.com"` |
| `uptime.urlPlaceholderHttp` | `"https://example.com"` |
| `uptime.urlPlaceholderTcp` | `"192.168.1.1:8080"` |
| `uptime.urlHelperHttp` | `"Vollständige URL einschließlich Schema eingeben (z.B. https://example.com)"` |
| `uptime.urlHelperTcp` | `"Als Host:Port ohne Schema-Präfix eingeben (z.B. 192.168.1.1:8080 oder hostname:22)"` |
| `uptime.invalidTcpFormat` | `"TCP-Monitore erfordern das Format Host:Port. Entfernen Sie das Schema-Präfix (z.B. 192.168.1.1:8080 statt tcp://192.168.1.1:8080)."` |

**Insertion point in `de/translation.json`:** Inside the `"uptime"` object (starts at
line 384), after the last existing key `"pendingFirstCheck"` (line ~411) and before
the closing `}` of the `uptime` object. The German `uptime` block currently ends at
`"pendingFirstCheck": "Warten auf erste Prüfung..."` — these keys are appended after it.

##### `frontend/src/locales/fr/translation.json`

| Key path | New value |
|----------|-----------|
| `uptime.urlPlaceholder` | `"https://example.com"` |
| `uptime.urlPlaceholderHttp` | `"https://example.com"` |
| `uptime.urlPlaceholderTcp` | `"192.168.1.1:8080"` |
| `uptime.urlHelperHttp` | `"Saisissez l'URL complète avec le schéma (ex. https://example.com)"` |
| `uptime.urlHelperTcp` | `"Saisissez sous la forme hôte:port sans préfixe de schéma (ex. 192.168.1.1:8080 ou nom-d-hôte:22)"` |
| `uptime.invalidTcpFormat` | `"Les moniteurs TCP nécessitent le format hôte:port. Supprimez le préfixe de schéma (ex. 192.168.1.1:8080 et non tcp://192.168.1.1:8080)."` |

**Insertion point:** Same pattern as `de` — append after the last key in the French
`"uptime"` block.

##### `frontend/src/locales/es/translation.json`

| Key path | New value |
|----------|-----------|
| `uptime.urlPlaceholder` | `"https://example.com"` |
| `uptime.urlPlaceholderHttp` | `"https://example.com"` |
| `uptime.urlPlaceholderTcp` | `"192.168.1.1:8080"` |
| `uptime.urlHelperHttp` | `"Ingresa la URL completa incluyendo el esquema (ej. https://example.com)"` |
| `uptime.urlHelperTcp` | `"Ingresa como host:puerto sin prefijo de esquema (ej. 192.168.1.1:8080 o nombre-de-host:22)"` |
| `uptime.invalidTcpFormat` | `"Los monitores TCP requieren el formato host:puerto. Elimina el prefijo de esquema (ej. usa 192.168.1.1:8080, no tcp://192.168.1.1:8080)."` |

**Insertion point:** Append after the last key in the Spanish `"uptime"` block.

##### `frontend/src/locales/zh/translation.json`

| Key path | New value |
|----------|-----------|
| `uptime.urlPlaceholder` | `"https://example.com"` |
| `uptime.urlPlaceholderHttp` | `"https://example.com"` |
| `uptime.urlPlaceholderTcp` | `"192.168.1.1:8080"` |
| `uptime.urlHelperHttp` | `"请输入包含协议的完整 URL（例如 https://example.com）"` |
| `uptime.urlHelperTcp` | `"请输入 主机:端口 格式，不含协议前缀（例如 192.168.1.1:8080 或 hostname:22）"` |
| `uptime.invalidTcpFormat` | `"TCP 监控器需要 主机:端口 格式。请删除协议前缀（例如使用 192.168.1.1:8080，而非 tcp://192.168.1.1:8080）。"` |

**Insertion point:** Append after the last key in the Chinese `"uptime"` block.

---

### 4. Implementation Plan

#### Phase 1: Playwright E2E Tests (Write First)

Write the E2E spec before touching any source files. The spec fails on the current broken
behavior and passes only after the fixes are applied.

**File:** `playwright/tests/uptime/create-monitor.spec.ts`

```
Test suite: Create Monitor Modal — TCP UX
```

| Test title | Preconditions | Steps | Assertion |
|------------|---------------|-------|-----------|
| `TCP type shows bare host:port placeholder` | App running (Docker E2E env), logged in as admin | 1. Click "Add Monitor". 2. Confirm default type is HTTP. 3. URL input has placeholder "https://example.com". 4. Change type to TCP. | URL input `placeholder` attribute equals `"192.168.1.1:8080"`. |
| `HTTP type shows URL placeholder` | Same | 1. Click "Add Monitor". 2. Type selector shows HTTP (default). | URL input `placeholder` attribute equals `"https://example.com"`. |
| `Type selector appears before URL input in tab order` | Same | 1. Click "Add Monitor". 2. Tab from Name input. | Focus moves to the type selector before the URL input. |
| `Helper text updates dynamically when type changes` | Same | 1. Click "Add Monitor". 2. Observe helper text below URL field. 3. Switch type to TCP. | Initial helper text contains "scheme". After TCP switch, helper text contains "host:port". |
| `Inline error appears when tcp:// scheme entered for TCP type` | Same | 1. Click "Add Monitor". 2. Set type to TCP. 3. Type `tcp://192.168.1.1:8080` into URL field. | Inline error message is visible and contains "host:port format". |
| `Inline error clears when scheme prefix removed` | Same | 1. Click "Add Monitor". 2. Set type to TCP. 3. Type `tcp://192.168.1.1:8080`. 4. Clear field and type `192.168.1.1:8080`. | Inline error is gone. |
| `Submit with tcp:// prefix is prevented client-side` | Same | 1. Click "Add Monitor". 2. Set type to TCP. 3. Enter name. 4. Enter `tcp://192.168.1.1:8080`. 5. Click Create. | Mock `POST /api/v1/uptime/monitors` is never called (Playwright route interception — the request does not reach the network). |
| `TCP monitor created successfully with bare host:port` | Route intercept `POST /api/v1/uptime/monitors` → respond 201 `{ id: "m-test", name: "DB Server", url: "192.168.1.1:5432", type: "tcp", ... }` | 1. Click "Add Monitor". 2. Set type to TCP. 3. Enter name "DB Server". 4. Enter `192.168.1.1:5432`. 5. Click Create. | Modal closes. Success toast visible. `createMonitor` was called with `url: "192.168.1.1:5432"` and `type: "tcp"`. |

**Playwright locator strategy:**

```typescript
// Type selector — by label text
const typeSelect = page.getByLabel('Type');
// or by id
const typeSelect = page.locator('#create-monitor-type');

// URL input — by label text
const urlInput = page.getByLabel('Monitor URL');
// or by id
const urlInput = page.locator('#create-monitor-url');

// Helper text — by id
const helperText = page.locator('#create-monitor-url-helper');

// Inline error — by role (rendered with role="alert")
const urlError = page.locator('[role="alert"]').filter({ hasText: 'host:port format' });
```

---

#### Phase 2: Frontend Unit Tests (RTL)

**File:** `frontend/src/pages/__tests__/Uptime.test.tsx`

**Update the existing mock map (line 40):**

```typescript
// Before (line 40):
'uptime.urlPlaceholder': 'https://example.com or host:port',

// After:
'uptime.urlPlaceholder': 'https://example.com',
'uptime.urlPlaceholderHttp': 'https://example.com',
'uptime.urlPlaceholderTcp': '192.168.1.1:8080',
'uptime.urlHelperHttp': 'Enter the full URL including the scheme',
'uptime.urlHelperTcp': 'Enter as host:port with no scheme prefix',
'uptime.invalidTcpFormat': 'TCP monitors require host:port format. Remove the scheme prefix.',
```

**New test cases to add** (append to the `describe('Uptime page', ...)` block):

| Test name | Setup | Action | Assertion |
|-----------|-------|--------|-----------|
| `URL placeholder shows HTTP value for HTTP type (default)` | `getMonitors` returns `[]` | Open modal, observe URL input | `getByPlaceholderText('https://example.com')` exists |
| `URL placeholder changes to host:port when TCP type is selected` | `getMonitors` returns `[]` | Open modal → change type select to TCP | `getByPlaceholderText('192.168.1.1:8080')` exists; `getByPlaceholderText('https://example.com')` is gone |
| `Helper text shows HTTP guidance for HTTP type` | Same | Open modal | Element `#create-monitor-url-helper` contains "scheme" |
| `Helper text changes to TCP guidance when TCP is selected` | Same | Open modal → change select to TCP | Element `#create-monitor-url-helper` contains "host:port" |
| `Inline error appears when tcp:// prefix entered for TCP type` | Same | Open modal → select TCP → type `tcp://` in URL field | `role="alert"` element with "host:port format" text is visible |
| `Inline error absent for HTTP type (tcp:// in URL field is allowed)` | Same | Open modal → keep HTTP → type `http://example.com` | No `role="alert"` element present |
| `handleSubmit blocked when TCP URL has scheme prefix` | `createMonitor` is a vi.fn() spy | Open modal → select TCP → fill name + `tcp://192.168.1.1:8080` → click Create | `createMonitor` is NOT called |
| `handleSubmit proceeds when TCP URL is bare host:port` | `createMonitor` resolves `{ id: '1', ... }` | Open modal → select TCP → fill name + `192.168.1.1:8080` → click Create | `createMonitor` called with `{ url: '192.168.1.1:8080', type: 'tcp', … }` |
| `Type selector appears before URL input in DOM order` | Same | Open modal | `getByLabelText('Monitor Type')` has `compareDocumentPosition` before `getByLabelText('Monitor URL')` — or use `getAllByRole` and check index order |

---

#### Phase 3: i18n File Changes

In this order:

1. **`frontend/src/locales/en/translation.json`** — 1 existing key changed + 5 new keys added.
2. **`frontend/src/locales/de/translation.json`** — 6 new keys added (plus `urlPlaceholder` added as new key since it doesn't yet exist in `de`).
3. **`frontend/src/locales/fr/translation.json`** — Same as `de`.
4. **`frontend/src/locales/es/translation.json`** — Same as `de`.
5. **`frontend/src/locales/zh/translation.json`** — Same as `de`.

**Exact JSON diff for `en/translation.json`** (surrounding context for safe editing):

```json
    "monitorTypeTcp": "TCP",
    "urlPlaceholder": "https://example.com",
    "urlPlaceholderHttp": "https://example.com",
    "urlPlaceholderTcp": "192.168.1.1:8080",
    "urlHelperHttp": "Enter the full URL including the scheme (e.g., https://example.com)",
    "urlHelperTcp": "Enter as host:port with no scheme prefix (e.g., 192.168.1.1:8080 or hostname:22)",
    "invalidTcpFormat": "TCP monitors require host:port format. Remove the scheme prefix (e.g., use 192.168.1.1:8080, not tcp://192.168.1.1:8080).",
    "pending": "CHECKING...",
```

Note: The `"pending"` line is the unchanged next key — it serves as the insertion anchor
to confirm correct placement.

For **de, fr, es, zh**, these are entirely **new keys being added** to an uptime block that
currently ends at `"pendingFirstCheck"`. The keys are appended before the closing `}` of
the `uptime` object. Additionally, each non-English locale receives `"urlPlaceholder":
"https://example.com"` as a new key (the value is intentionally left in Latin characters —
`example.com` is a universal placeholder that needs no translation).

---

#### Phase 4: Frontend Component Changes

All changes are in `frontend/src/pages/Uptime.tsx` inside `CreateMonitorModal` (lines
342–476).

**Step 1 — Add `urlError` state** (after line 348):

```typescript
const [urlError, setUrlError] = useState('');
```

**Step 2 — Add `urlPlaceholder` derived constant** (after Step 1):

```typescript
const urlPlaceholder = type === 'tcp'
  ? t('uptime.urlPlaceholderTcp')
  : t('uptime.urlPlaceholderHttp');
```

**Step 3 — Reorder form fields** (move type `<div>` before URL `<div>`):

The form's JSX block order changes from `[name, url, type, interval, retries]` to
`[name, type, url, interval, retries]`. This is a pure JSX cut-and-paste of the two
`<div>` blocks. No logic changes.

**Step 4 — Update URL `<input>`**:

- Change `placeholder={t('uptime.urlPlaceholder')}` → `placeholder={urlPlaceholder}`
- Change `onChange={(e) => setUrl(e.target.value)}` → the new onChange with error logic
- Add `aria-describedby` attribute (see §3.5)

**Step 5 — Add helper text `<p>` after the URL `<input />`** (see §3.4)

**Step 6 — Add inline error `<p>` after helper text `<p>`** (see §3.5)

**Step 7 — Update `handleSubmit`** to include the `tcp://` guard (see §3.5)

---

#### Phase 5: Backend Tests (None Required)

The backend's validation is correct. `net.SplitHostPort` correctly rejects `tcp://` prefixed
strings and will continue to do so. No backend code is changed. No new backend tests are
needed.

However, to prevent future regression of the backend's error message (which helps diagnose
protocol prefix issues), verify the existing uptime service tests cover the TCP `host:port`
validation path. If they do not, note it as a separate backlog item — do not add it to PR-5.

---

### 5. Complete PR-5 File Manifest

| File | Change type | Description |
|------|-------------|-------------|
| `frontend/src/locales/en/translation.json` | Modified | 1 changed key, 5 new keys |
| `frontend/src/locales/de/translation.json` | Modified | 6 new keys added to `uptime` object |
| `frontend/src/locales/fr/translation.json` | Modified | 6 new keys added to `uptime` object |
| `frontend/src/locales/es/translation.json` | Modified | 6 new keys added to `uptime` object |
| `frontend/src/locales/zh/translation.json` | Modified | 6 new keys added to `uptime` object |
| `frontend/src/pages/Uptime.tsx` | Modified | State, derived var, JSX reorder, placeholder, helper text, error, submit guard |
| `frontend/src/pages/__tests__/Uptime.test.tsx` | Modified | Updated mock + 9 new test cases |
| `playwright/tests/uptime/create-monitor.spec.ts` | New | 8 E2E scenarios |

Total: 7 modified files, 1 new file. No backend changes. No database migrations. No API
contract changes.

---

### 6. Commit Slicing Strategy

**Decision:** Single PR.

**Rationale:**
- All changes are in the React/i18n frontend layer. No backend, no database.
- The five fixes are tightly coupled: Fix 2 (dynamic placeholder) requires Fix 1 (key value
  change) to be correct; Fix 3 (helper text) uses the same `type` state as Fix 2; Fix 4
  (inline error) reads `type` and the same `url` state; Fix 5 (reorder) relies on the
  derived `urlPlaceholder` constant introduced in Fix 2.
- Total reviewer surface: 2 source files modified + 5 locale files + 1 test file updated + 1
  new spec file. A single PR with 8 files is fast to review.
- The fix is independently deployable and independently revertable.

**PR Slice: PR-5** (single)

| Attribute | Value |
|-----------|-------|
| Scope | Frontend UX + i18n |
| Files | 8 (listed in §5 above) |
| Dependencies | None |
| Validation gate | Playwright E2E suite green (specifically `tests/uptime/create-monitor.spec.ts`); Vitest `Uptime.test.tsx` green; all 5 locale files pass i18n key check |
| Rollback | `git revert` of the single merge commit |
| Side effects | None — no DB, no API surface, no backend binary |

---

### 7. Acceptance Criteria

| # | Criterion | Verification method |
|---|-----------|---------------------|
| 1 | `uptime.urlPlaceholder` in `en/translation.json` no longer contains `"tcp://"` | `grep "tcp://" frontend/src/locales/en/translation.json` returns no matches |
| 2 | URL input placeholder in Create Monitor modal shows `"https://example.com"` when HTTP is selected | Playwright: `URL placeholder shows HTTP value…` passes |
| 3 | URL input placeholder switches to `"192.168.1.1:8080"` when TCP is selected | Playwright: `TCP type shows bare host:port placeholder` passes |
| 4 | Helper text below URL input explains the expected format and changes with type | Playwright: `Helper text updates dynamically…` passes |
| 5 | Inline error appears immediately when `tcp://` is typed while TCP type is selected | Playwright: `Inline error appears when tcp:// scheme entered…` passes |
| 6 | Form submission is blocked client-side when a `tcp://` value is present | Playwright: `Submit with tcp:// prefix is prevented client-side` passes |
| 7 | A bare `host:port` value (e.g., `192.168.1.1:5432`) submits successfully | Playwright: `TCP monitor created successfully with bare host:port` passes |
| 8 | Type selector appears before URL input in the tab order | Playwright: `Type selector appears before URL input in tab order` passes |
| 9 | All 6 new i18n keys exist in all 5 locale files | RTL test mock updated; CI i18n lint passes |
| 10 | `Uptime.test.tsx` mock updated; all 9 new RTL test cases pass | `npx vitest run frontend/src/pages/__tests__/Uptime.test.tsx` exits 0 |
| 11 | No regressions in any existing `Uptime.test.tsx` tests | Full Vitest suite green |

---

### 8. Commit Message

```
fix(frontend): correct TCP monitor URL placeholder and add per-type UX guidance

Users attempting to create TCP uptime monitors were given a misleading
placeholder — "https://example.com or tcp://host:port" — and followed it
by submitting "tcp://192.168.1.1:8080". The backend's net.SplitHostPort
rejected this with "too many colons in address" (HTTP 500) because the
tcp:// scheme prefix causes the host portion to contain a colon.

Apply five layered fixes:

1. Correct the urlPlaceholder i18n key (en/translation.json line 480)
   to remove the misleading tcp:// prefix.

2. Add urlPlaceholderHttp / urlPlaceholderTcp keys and switch the URL
   input placeholder dynamically based on the selected type state.

3. Add urlHelperHttp / urlHelperTcp keys and render helper text below
   the URL input explaining the expected format per type.

4. Add client-side guard: if type is TCP and the URL contains "://",
   show an inline error and block form submission before the API call.

5. Reorder the Create Monitor form so the type selector appears before
   the URL input, ensuring the correct placeholder is visible before
   the user types the URL.

New i18n keys added to all 5 locales (en, de, fr, es, zh):
  uptime.urlPlaceholder (changed)
  uptime.urlPlaceholderHttp (new)
  uptime.urlPlaceholderTcp (new)
  uptime.urlHelperHttp (new)
  uptime.urlHelperTcp (new)
  uptime.invalidTcpFormat (new)

Closes issue 5 from the fresh-install bug report.
```

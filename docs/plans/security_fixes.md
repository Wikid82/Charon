# 📋 Plan: Security Fixes & Enhancements

**Date:** December 4, 2025
**Branch:** `feature/beta-release`
**Status:** Draft

---

## 🧐 UX & Context Analysis

### 1. Security Toggles Persistence
**Problem:** Toggles for CrowdSec, WAF, and Rate Limiting provide feedback but revert state on refresh.
**Root Cause:** The Backend `GetStatus` handler reads from static config and ignores the `settings` table overrides for WAF and Rate Limiting.
**Fix:** Update `GetStatus` to check the `settings` table for all security modules, ensuring the "Source of Truth" is the database, not just the startup config.

### 2. CrowdSec Dashboard "Blank Page"
**Problem:** `/security/crowdsec` renders a blank blue page without header/footer.
**Analysis:** The component is likely crashing due to unhandled null/undefined data access, or there is a routing/layout issue.
**Fix:**
- Wrap `CrowdSecConfig` in an Error Boundary.
- Ensure `status` and `listMutation.data` are accessed safely.
- Verify `Layout` wrapping in `App.tsx`.

### 3. Rate-Limiting Dashboard
**Problem:** `/security/rate-limiting` loads System Settings.
**Fix:**
- Create `frontend/src/pages/RateLimiting.tsx`.
- Implement controls for `enabled`, `requests_per_second`, `burst`, and `window`.
- Update `App.tsx` routing.

### 4. WAF Presets & Usability
**Problem:** Users need an easy way to add standard rules (OWASP CRS) without copy-pasting.
**Fix:**
- Add a "Presets" dropdown to the WAF Rule Set form.
- Include "OWASP Core Rule Set (CRS)" and "Common Bad Bots" as built-in presets.
- Presets will auto-fill the "Source URL" or "Content" fields.

---

## 🤝 Handoff Contract

### Backend Changes
**GET /api/v1/security/status**
Must respect the following `settings` table keys:
- `security.cerberus.enabled` (bool)
- `security.crowdsec.enabled` (bool) -> overrides mode to 'local' if true
- `security.waf.enabled` (bool) -> overrides mode to 'block' (or saved mode) if true
- `security.rate_limit.enabled` (bool)
- `security.acl.enabled` (bool)

### Frontend Changes
- New Page: `RateLimiting.tsx`
- Updated Page: `WafConfig.tsx` (Presets)
- Updated Page: `CrowdSecConfig.tsx` (Crash fix)

---

## 🏗️ Phase 1: Backend Implementation (Go)

### 1. Security Handler (`internal/api/handlers/security_handler.go`)
- Modify `GetStatus` to query the `settings` table for:
  - `security.waf.enabled`
  - `security.rate_limit.enabled`
  - `security.crowdsec.enabled`
- Logic:
  - If `security.waf.enabled` == "true", set `waf.enabled = true`.
  - If `security.rate_limit.enabled` == "true", set `rate_limit.enabled = true`.

---

## 🎨 Phase 2: Frontend Implementation (React)

### 1. Fix CrowdSec Dashboard (`pages/CrowdSecConfig.tsx`)
- Add null checks for `status.crowdsec`.
- Ensure `listMutation.data` is handled when undefined.
- Verify `Layout` context.

### 2. Create Rate Limiting Page (`pages/RateLimiting.tsx`)
- **UI:**
  - Toggle: Enable/Disable
  - Input: Requests per second (default: 10)
  - Input: Burst (default: 5)
  - Input: Window (seconds)
- **API:** Use `updateSetting` for toggle, `updateSecurityConfig` for values.

### 3. WAF Presets (`pages/WafConfig.tsx`)
- Add `PRESETS` constant:
  ```typescript
  const PRESETS = [
    {
      name: 'OWASP Core Rule Set',
      url: 'https://github.com/coreruleset/coreruleset/archive/refs/tags/v3.3.5.tar.gz',
      description: 'Industry standard protection against Top 10 vulnerabilities.'
    },
    {
      name: 'Basic SQL Injection Protection',
      content: 'SecRule REQUEST_URI "@detectSQLi" "id:1001,phase:1,deny,status:403,msg:\'SQLi Detected\'"',
      description: 'Simple rule to block common SQL injection patterns.'
    }
  ]
  ```
- Add Dropdown to `RuleSetForm` to populate fields.

### 4. Update Routing (`App.tsx`)
- Point `/security/rate-limiting` to `RateLimiting` component.

---

## 🧪 Phase 3: Verification

1. **Toggles:** Toggle WAF on/off -> Refresh -> Verify state persists.
2. **CrowdSec:** Navigate to `/security/crowdsec` -> Verify page loads with Layout.
3. **Rate Limit:** Navigate to `/security/rate-limiting` -> Verify new UI.
4. **WAF:** Create Rule Set -> Select "OWASP CRS" -> Verify URL is filled.

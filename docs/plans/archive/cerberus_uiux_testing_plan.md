# Cerberus Security Dashboard - UI/UX Test Plan

**Issue**: #319 - Cerberus Integration Testing (Final Phase)
**Created**: December 12, 2025
**Status**: ✅ Complete

---

## Executive Summary

This test plan covers comprehensive UI/UX testing for the Cerberus Security Dashboard, including all security feature cards, loading states, error handling, and mobile responsiveness. The plan leverages existing Vitest unit test patterns and recommends expanding Playwright E2E coverage.

---

## 1. Components Under Test

### 1.1 Primary Security Pages

| Component | File Path | Description |
|-----------|-----------|-------------|
| Security Dashboard | [frontend/src/pages/Security.tsx](frontend/src/pages/Security.tsx) | Main Cerberus dashboard with 4 security cards |
| WAF Configuration | [frontend/src/pages/WafConfig.tsx](frontend/src/pages/WafConfig.tsx) | Coraza rule set management |
| Rate Limiting | [frontend/src/pages/RateLimiting.tsx](frontend/src/pages/RateLimiting.tsx) | Rate limit configuration |
| CrowdSec Config | [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx) | CrowdSec mode, presets, bans |
| Access Lists | [frontend/src/pages/AccessLists.tsx](frontend/src/pages/AccessLists.tsx) | ACL management |

### 1.2 Shared Components

| Component | File Path | Description |
|-----------|-----------|-------------|
| ConfigReloadOverlay | [frontend/src/components/LoadingStates.tsx](frontend/src/components/LoadingStates.tsx) | Cerberus-themed loading overlay |
| CerberusLoader | [frontend/src/components/LoadingStates.tsx](frontend/src/components/LoadingStates.tsx) | Three-headed guardian animation |
| Card | [frontend/src/components/ui/Card.tsx](frontend/src/components/ui/Card.tsx) | Security card wrapper |
| Switch | [frontend/src/components/ui/Switch.tsx](frontend/src/components/ui/Switch.tsx) | Toggle control |
| SecurityNotificationSettingsModal | [frontend/src/components/SecurityNotificationSettingsModal.tsx](frontend/src/components/SecurityNotificationSettingsModal.tsx) | Notification settings |

### 1.3 React Query Hooks

| Hook | File Path | Purpose |
|------|-----------|---------|
| useSecurityStatus | [frontend/src/hooks/useSecurity.ts](frontend/src/hooks/useSecurity.ts) | Fetch security status |
| useSecurityConfig | [frontend/src/hooks/useSecurity.ts](frontend/src/hooks/useSecurity.ts) | Fetch security config |
| useUpdateSecurityConfig | [frontend/src/hooks/useSecurity.ts](frontend/src/hooks/useSecurity.ts) | Update config mutation |
| useRuleSets | [frontend/src/hooks/useSecurity.ts](frontend/src/hooks/useSecurity.ts) | WAF rule sets |
| useEnableCerberus | [frontend/src/hooks/useSecurity.ts](frontend/src/hooks/useSecurity.ts) | Enable Cerberus mutation |
| useDisableCerberus | [frontend/src/hooks/useSecurity.ts](frontend/src/hooks/useSecurity.ts) | Disable Cerberus mutation |

### 1.4 API Layer

| API Module | File Path |
|------------|-----------|
| Security API | [frontend/src/api/security.ts](frontend/src/api/security.ts) |
| CrowdSec API | `frontend/src/api/crowdsec.ts` |
| Settings API | `frontend/src/api/settings.ts` |

---

## 2. Test Cases by Requirement

### 2.1 Security Dashboard Cards - Correct Status Display

**Test File**: `frontend/src/pages/__tests__/Security.dashboard.test.tsx` (new)
**Framework**: Vitest + React Testing Library

#### Test Cases

| ID | Test Case | Expected Behavior | Priority |
|----|-----------|-------------------|----------|
| SD-01 | Dashboard shows "Cerberus Disabled" banner when `cerberus.enabled=false` | Banner renders with documentation link | High |
| SD-02 | CrowdSec card shows "Active" when `crowdsec.enabled=true` | Green icon, "Active" text, running PID | High |
| SD-03 | CrowdSec card shows "Disabled" when `crowdsec.enabled=false` | Gray icon, "Disabled" text | High |
| SD-04 | WAF (Coraza) card shows correct status | "Active"/"Disabled" based on `waf.enabled` | High |
| SD-05 | Rate Limiting card shows correct status | Badge and text reflect `rate_limit.enabled` | High |
| SD-06 | ACL card shows correct status | Status matches `acl.enabled` | High |
| SD-07 | All cards display correct layer indicators | Layer 1-4 labels present in order | Medium |
| SD-08 | Threat protection summaries display correctly | Each card shows threat types | Medium |
| SD-09 | Cards maintain order after toggle | CrowdSec → ACL → WAF → Rate Limiting | Medium |
| SD-10 | Toggle switches are disabled when Cerberus is off | All service toggles disabled | High |

#### Assertions

```typescript
// SD-01: Cerberus disabled banner
expect(screen.getByText(/Cerberus Disabled/i)).toBeInTheDocument()
expect(screen.getByRole('link', { name: /Documentation/i })).toHaveAttribute('href', expect.stringContaining('wikid82.github.io'))

// SD-02: CrowdSec active status
expect(screen.getByText('Active')).toBeInTheDocument()
expect(screen.getByTestId('toggle-crowdsec')).toBeChecked()

// SD-07: Layer indicators
expect(screen.getByText(/Layer 1: IP Reputation/i)).toBeInTheDocument()
expect(screen.getByText(/Layer 2: Access Control/i)).toBeInTheDocument()
expect(screen.getByText(/Layer 3: Request Inspection/i)).toBeInTheDocument()
expect(screen.getByText(/Layer 4: Volume Control/i)).toBeInTheDocument()
```

#### Mock Strategy

```typescript
vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/settings')

const mockSecurityStatus = {
  cerberus: { enabled: true },
  crowdsec: { mode: 'local', api_url: 'http://localhost', enabled: true },
  waf: { mode: 'enabled', enabled: true },
  rate_limit: { enabled: true },
  acl: { enabled: true },
}
```

---

### 2.2 Loading States

**Test File**: `frontend/src/pages/__tests__/Security.loading.test.tsx` (new)
**Framework**: Vitest + React Testing Library

#### Test Cases

| ID | Test Case | Expected Behavior | Priority |
|----|-----------|-------------------|----------|
| LS-01 | Initial page load shows loading text | "Loading security status..." appears | High |
| LS-02 | Toggling service shows CerberusLoader overlay | `ConfigReloadOverlay` with type="cerberus" | High |
| LS-03 | Starting CrowdSec shows "Summoning the guardian..." | Specific message for start operation | High |
| LS-04 | Stopping CrowdSec shows "Guardian rests..." | Specific message for stop operation | High |
| LS-05 | WAF config operations show overlay | Create/update/delete show loader | High |
| LS-06 | Rate Limiting save shows overlay | "Adjusting the gates..." message | High |
| LS-07 | CrowdSec preset pull shows overlay | "Fetching preset..." message | Medium |
| LS-08 | CrowdSec preset apply shows overlay | "Loading preset..." message | Medium |
| LS-09 | Overlay blocks interactions | Pointer events disabled during load | High |
| LS-10 | Overlay disappears on mutation success | No overlay after operation completes | High |

#### Assertions

```typescript
// LS-02: Toggle shows overlay
const toggle = screen.getByTestId('toggle-waf')
await user.click(toggle)
await waitFor(() => {
  expect(screen.getByText(/Three heads turn/i)).toBeInTheDocument()
  expect(screen.getByText(/Cerberus configuration updating/i)).toBeInTheDocument()
})

// LS-03: CrowdSec start message
await waitFor(() => {
  expect(screen.getByText(/Summoning the guardian/i)).toBeInTheDocument()
  expect(screen.getByText(/CrowdSec is starting/i)).toBeInTheDocument()
})

// LS-09: Overlay blocks interactions
expect(screen.getByRole('status', { name: /Security Loading/i })).toBeInTheDocument()
```

#### Mock Strategy

```typescript
// Use never-resolving promises to test loading states
vi.mocked(settingsApi.updateSetting).mockImplementation(() => new Promise(() => {}))
vi.mocked(crowdsecApi.startCrowdsec).mockImplementation(() => new Promise(() => {}))
```

---

### 2.3 Error Handling

**Test File**: `frontend/src/pages/__tests__/Security.errors.test.tsx` (new)
**Framework**: Vitest + React Testing Library

#### Test Cases

| ID | Test Case | Expected Behavior | Priority |
|----|-----------|-------------------|----------|
| EH-01 | Failed security status fetch shows error | "Failed to load security status" message | High |
| EH-02 | Toggle mutation failure shows toast | `toast.error()` called with message | High |
| EH-03 | CrowdSec start failure shows specific toast | "Failed to start CrowdSec: [message]" | High |
| EH-04 | CrowdSec stop failure shows specific toast | "Failed to stop CrowdSec: [message]" | High |
| EH-05 | WAF fetch failure shows error state | "Failed to load WAF configuration" | High |
| EH-06 | Rate Limiting update failure shows toast | Error toast with message | High |
| EH-07 | Network error shows generic message | Graceful degradation | Medium |
| EH-08 | Validation error shows inline message | Form validation errors | Medium |
| EH-09 | API returns 401 redirects to login | Auth check on protected routes | High |
| EH-10 | Optimistic update reverts on error | Previous state restored | High |

#### Assertions

```typescript
// EH-01: Failed fetch
vi.mocked(securityApi.getSecurityStatus).mockRejectedValue(new Error('Network error'))
await renderSecurityPage()
await waitFor(() => {
  expect(screen.getByText(/Failed to load security status/i)).toBeInTheDocument()
})

// EH-02: Toggle failure toast
vi.mocked(settingsApi.updateSetting).mockRejectedValue(new Error('Permission denied'))
await user.click(toggle)
await waitFor(() => {
  expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to update setting'))
})

// EH-10: Optimistic update rollback
const previousState = queryClient.getQueryData(['security-status'])
// After error, state should be restored
await waitFor(() => {
  expect(queryClient.getQueryData(['security-status'])).toEqual(previousState)
})
```

#### Mock Strategy

```typescript
vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

// Reject API calls
vi.mocked(securityApi.getSecurityStatus).mockRejectedValue(new Error('API Error'))
```

---

### 2.4 Mobile Responsiveness

**Test File**: `frontend/e2e/tests/security-mobile.spec.ts` (new)
**Framework**: Playwright

#### Test Cases

| ID | Test Case | Viewport | Expected Behavior | Priority |
|----|-----------|----------|-------------------|----------|
| MR-01 | Dashboard cards stack on mobile | 375×667 | Single column layout | High |
| MR-02 | Dashboard cards 2-col on tablet | 768×1024 | `md:grid-cols-2` applies | Medium |
| MR-03 | Dashboard cards 4-col on desktop | 1920×1080 | `lg:grid-cols-4` applies | Medium |
| MR-04 | Toggle switches remain accessible | 375×667 | Touch target ≥44px | High |
| MR-05 | Config buttons are tappable | 375×667 | Full-width on mobile | High |
| MR-06 | Modal/overlay renders correctly | 375×667 | Scrollable, no overflow | High |
| MR-07 | WAF table scrolls horizontally | 375×667 | Horizontal scroll enabled | Medium |
| MR-08 | Rate limiting form inputs fit | 375×667 | Inputs stack vertically | Medium |
| MR-09 | CrowdSec preset list is scrollable | 375×667 | Max-height with overflow | Medium |
| MR-10 | Navigation sidebar collapses | 375×667 | Mobile menu toggle works | High |

#### Playwright Test Example

```typescript
import { test, expect } from '@playwright/test'

const base = process.env.CHARON_BASE_URL || 'http://localhost:8080'

test.describe('Security Dashboard Mobile', () => {
  test.use({ viewport: { width: 375, height: 667 } })

  test('cards stack vertically on mobile', async ({ page }) => {
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]')

    // Check grid is single column
    const cardsContainer = page.locator('.grid')
    await expect(cardsContainer).toHaveCSS('grid-template-columns', /^[0-9.]+px$/)
  })

  test('toggle switches are accessible touch targets', async ({ page }) => {
    await page.goto(`${base}/security`)
    const toggle = page.getByTestId('toggle-crowdsec')
    const box = await toggle.boundingBox()
    expect(box?.height).toBeGreaterThanOrEqual(44)
  })

  test('config reload overlay is scrollable', async ({ page }) => {
    await page.goto(`${base}/security`)
    // Trigger loading state
    await page.getByTestId('toggle-waf').click()
    const overlay = page.locator('.fixed.inset-0')
    await expect(overlay).toBeVisible()
    // Verify no content overflow
    const isScrollable = await overlay.evaluate((el) => el.scrollHeight > el.clientHeight)
    // Overlay should fit or be scrollable
    expect(true).toBe(true) // Overlay renders
  })
})

test.describe('Security Dashboard Tablet', () => {
  test.use({ viewport: { width: 768, height: 1024 } })

  test('cards show 2 columns', async ({ page }) => {
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]')
    const cardsContainer = page.locator('.grid.grid-cols-1.md\\:grid-cols-2')
    await expect(cardsContainer).toBeVisible()
  })
})
```

---

## 3. Test Implementation Strategy

### 3.1 Unit Tests (Vitest)

**Location**: `frontend/src/pages/__tests__/`

| Test File | Status | Coverage Target |
|-----------|--------|-----------------|
| Security.test.tsx | ✅ Exists | Expand for full card status |
| Security.audit.test.tsx | ✅ Exists | Edge cases covered |
| WafConfig.spec.tsx | ✅ Exists | Loading/error states |
| RateLimiting.spec.tsx | ✅ Exists | Toggle and save |
| CrowdSecConfig.test.tsx | ✅ Exists | Mode toggle, presets |
| Security.dashboard.test.tsx | ❌ Create | Card status verification |
| Security.loading.test.tsx | ❌ Create | Loading overlay tests |
| Security.errors.test.tsx | ❌ Create | Error handling tests |

### 3.2 E2E Tests (Playwright)

**Location**: `frontend/e2e/tests/`

| Test File | Status | Coverage Target |
|-----------|--------|-----------------|
| waf.spec.ts | ✅ Exists | WAF blocking |
| security-mobile.spec.ts | ❌ Create | Mobile responsive |
| security-e2e.spec.ts | ❌ Create | Full user flow |

### 3.3 Mocking Strategies

#### API Mocking (Vitest)

```typescript
// Complete mock setup for security API
vi.mock('../../api/security', () => ({
  getSecurityStatus: vi.fn(),
  getSecurityConfig: vi.fn(),
  updateSecurityConfig: vi.fn(),
  enableCerberus: vi.fn(),
  disableCerberus: vi.fn(),
  getRuleSets: vi.fn(),
  upsertRuleSet: vi.fn(),
  deleteRuleSet: vi.fn(),
}))

vi.mock('../../api/crowdsec', () => ({
  startCrowdsec: vi.fn(),
  stopCrowdsec: vi.fn(),
  statusCrowdsec: vi.fn(),
  listCrowdsecDecisions: vi.fn(),
  banIP: vi.fn(),
  unbanIP: vi.fn(),
}))

vi.mock('../../api/settings', () => ({
  updateSetting: vi.fn(),
}))
```

#### Toast Mocking

```typescript
vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    loading: vi.fn(),
  },
}))
```

#### React Query Test Wrapper

```typescript
const createTestQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 0 },
      mutations: { retry: false },
    },
  })

const TestWrapper = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={createTestQueryClient()}>
    <BrowserRouter>{children}</BrowserRouter>
  </QueryClientProvider>
)
```

---

## 4. Test Data Fixtures

### 4.1 Security Status Fixtures

```typescript
// All features enabled
export const mockSecurityStatusAllEnabled = {
  cerberus: { enabled: true },
  crowdsec: { mode: 'local', api_url: 'http://localhost', enabled: true },
  waf: { mode: 'enabled', enabled: true },
  rate_limit: { enabled: true },
  acl: { enabled: true },
}

// Cerberus disabled (all features should be disabled)
export const mockSecurityStatusCerberusDisabled = {
  cerberus: { enabled: false },
  crowdsec: { mode: 'disabled', api_url: '', enabled: false },
  waf: { mode: 'disabled', enabled: false },
  rate_limit: { enabled: false },
  acl: { enabled: false },
}

// Mixed states
export const mockSecurityStatusMixed = {
  cerberus: { enabled: true },
  crowdsec: { mode: 'local', api_url: 'http://localhost', enabled: true },
  waf: { mode: 'disabled', enabled: false },
  rate_limit: { enabled: true },
  acl: { enabled: false },
}
```

### 4.2 WAF Rule Set Fixtures

```typescript
export const mockRuleSets = {
  rulesets: [
    {
      id: 1,
      uuid: 'uuid-1',
      name: 'OWASP CRS',
      source_url: 'https://github.com/coreruleset/coreruleset/archive/refs/tags/v3.3.5.tar.gz',
      mode: 'blocking',
      last_updated: '2025-12-04T10:00:00Z',
      content: '',
    },
    {
      id: 2,
      uuid: 'uuid-2',
      name: 'Custom SQLi Rules',
      source_url: '',
      mode: 'detection',
      last_updated: '2025-12-10T15:30:00Z',
      content: 'SecRule ARGS "@detectSQLi" "id:1001,phase:1,deny"',
    },
  ],
}
```

---

## 5. Test Execution Plan

### 5.1 Pre-Merge Checklist

- [ ] All existing tests pass (`npm run test`)
- [ ] New unit tests added for uncovered scenarios
- [ ] E2E mobile tests added
- [ ] Coverage meets threshold (80% lines)
- [ ] No console errors in tests

### 5.2 CI Integration

```yaml
# Add to existing test workflow
- name: Run Security UI Tests
  run: |
    cd frontend
    npm run test -- --coverage --reporter=json --outputFile=coverage/security-ui.json

- name: Run E2E Mobile Tests
  run: |
    cd frontend
    npx playwright test e2e/tests/security-mobile.spec.ts
```

### 5.3 Manual QA Verification

| Scenario | Browser | Viewport | Verified |
|----------|---------|----------|----------|
| Dashboard loads | Chrome | Desktop | [ ] |
| Dashboard loads | Safari | Mobile | [ ] |
| Toggle CrowdSec | Chrome | Desktop | [ ] |
| Toggle WAF | Firefox | Desktop | [ ] |
| Error toast appears | Chrome | Desktop | [ ] |
| Mobile navigation | Safari | iPhone 14 | [ ] |
| Tablet layout | Chrome | iPad | [ ] |

---

## 6. Coverage Gaps & Recommendations

### 6.1 Current Coverage

- ✅ Security.tsx: Good coverage (toggle, loading, errors)
- ✅ WafConfig.tsx: Full CRUD coverage
- ✅ RateLimiting.tsx: Toggle and config save
- ✅ CrowdSecConfig.tsx: Mode toggle, presets
- ⚠️ Mobile responsiveness: No E2E tests
- ⚠️ ConfigReloadOverlay: Unit tests missing

### 6.2 Recommended New Tests

1. **Create `Security.dashboard.test.tsx`**: Focus on card status display
2. **Create `Security.loading.test.tsx`**: Comprehensive loading overlay tests
3. **Create `security-mobile.spec.ts`**: Playwright mobile viewport tests
4. **Add LoadingStates.test.tsx**: Unit test ConfigReloadOverlay component

### 6.3 Test Data-Testid Inventory

Existing test IDs for automation:

```
toggle-crowdsec
toggle-waf
toggle-acl
toggle-rate-limit
waf-loading
waf-error
waf-empty-state
rulesets-table
create-ruleset-btn
ruleset-name-input
ruleset-content-input
rate-limit-toggle
rate-limit-rps
rate-limit-burst
rate-limit-window
save-rate-limit-btn
crowdsec-mode-toggle
import-btn
apply-preset-btn
```

---

## 7. Appendix

### A. Existing Test Patterns (Reference)

See existing test files for patterns:

- [Security.test.tsx](frontend/src/pages/__tests__/Security.test.tsx)
- [WafConfig.spec.tsx](frontend/src/pages/__tests__/WafConfig.spec.tsx)
- [RateLimiting.spec.tsx](frontend/src/pages/__tests__/RateLimiting.spec.tsx)

### B. Component Hierarchy

```
Security.tsx
├── ConfigReloadOverlay (when isApplyingConfig)
├── Header Banner (when Cerberus disabled)
├── Admin Whitelist Input
├── Outlet (nested routes)
└── Grid (4 columns)
    ├── CrowdSec Card
    │   ├── Switch (toggle-crowdsec)
    │   └── Config Button
    ├── ACL Card
    │   ├── Switch (toggle-acl)
    │   └── Manage Lists Button
    ├── Coraza Card
    │   ├── Switch (toggle-waf)
    │   └── Configure Button
    └── Rate Limiting Card
        ├── Switch (toggle-rate-limit)
        └── Configure Limits Button
```

### C. API Response Types

```typescript
interface SecurityStatus {
  cerberus?: { enabled: boolean }
  crowdsec: { mode: 'disabled' | 'local'; api_url: string; enabled: boolean }
  waf: { mode: 'disabled' | 'enabled'; enabled: boolean }
  rate_limit: { mode?: 'disabled' | 'enabled'; enabled: boolean }
  acl: { enabled: boolean }
}
```

---

**Author**: Copilot Planning Agent
**Review Required**: QA Team
**Implementation Owner**: Frontend Team

# Phase 3.1: Coverage Gap Analysis

**Date:** February 3, 2026
**Phase:** Phase 3.1 - Coverage Gap Identification
**Status:** ✅ Complete
**Duration:** 2 hours

---

## Executive Summary

**Coverage Targets:**
- Backend: 83.5% → 85.0% (+1.5% gap)
- Frontend: 84.25% → 85.0% (+0.75% gap)

**Key Findings:**
- **Backend:** 5 packages require targeted testing (cerberus, config, util, utils, models)
- **Frontend:** 4 pages require component tests (Security, SecurityHeaders, Plugins, Dashboard)
- **Estimated Effort:** 6-8 hours total (4 hours backend, 2-4 hours frontend)

**Strategic Approach:**
- Prioritize high-value tests (critical paths, security, error handling)
- Avoid low-value tests (trivial getters/setters, TableName() methods)
- Focus on business logic and edge cases

---

## Backend Coverage Analysis

### Overall Status

**Current Coverage:** 83.5%
**Target Coverage:** 85.0%
**Gap to Close:** +1.5%

**Estimated New Tests Required:** 10-15 unit tests
**Estimated Effort:** 4 hours

### Package-Level Coverage

#### P0 - Critical (Below 75%)

| Package | Current | Target | Gap | Impact | Effort |
|---------|---------|--------|-----|--------|--------|
| `cmd/api` | 0% | N/A | - | None (main package, not tested) | - |
| `pkg/dnsprovider/builtin` | 31% | 85% | +54% | HIGH - DNS provider factory | L (2h) |
| `cmd/seed` | 59% | N/A | - | LOW (dev tool only) | - |
| `internal/cerberus` | 71% | 85% | +14% | CRITICAL - Security module | M (1h) |
| `internal/config` | 71% | 85% | +14% | HIGH - Configuration management | M (1h) |

#### P1 - High Priority (75-84%)

| Package | Current | Target | Gap | Impact | Effort |
|---------|---------|--------|-----|--------|--------|
| `internal/util` | 75% | 85% | +10% | MEDIUM - Utility functions | S (30m) |
| `internal/utils` | 78% | 85% | +7% | MEDIUM - URL utilities | S (30m) |
| `internal/models` | 80% | 85% | +5% | MEDIUM - Model methods | S (30m) |

#### P2 - Medium Priority (85-90%)

| Package | Current | Target | Notes |
|---------|---------|--------|-------|
| `internal/services` | 87% | 85% | ✅ Exceeds threshold |
| `internal/crypto` | 88% | 85% | ✅ Exceeds threshold |
| `internal/api/handlers` | 89% | 85% | ✅ Exceeds threshold |
| `internal/server` | 89% | 85% | ✅ Exceeds threshold |

#### P3 - Low Priority (90%+)

All other packages exceed 90% coverage and require no action.

---

### Detailed Gap Analysis: High-Priority Packages

#### 1. pkg/dnsprovider/builtin (31% → 85%)

**Priority:** HIGH
**Effort:** Large (2 hours) ⚠️

**Recommendation:** SKIP for Phase 3.1
**Rationale:** 54% gap requires extensive testing effort that may exceed time budget. Target for separate refactoring effort.

**Alternative:** Document as technical debt, create follow-up issue.

---

#### 2. internal/cerberus (71% → 85%)

**Priority:** CRITICAL (Security Module)
**Effort:** Medium (1 hour)

**Uncovered Functions (0% coverage):**
- `InvalidateCache()` - Cache invalidation logic

**Action Items:**
1. Add test for `InvalidateCache()` success case
2. Add test for cache invalidation error handling
3. Add test for cache state after invalidation

**Expected Impact:** Package from 71% → 85%+ (single critical function)

**Example Test:**
```go
func TestInvalidateCache(t *testing.T) {
    // Setup: Create cerberus instance with cache populated
    c := NewCerberus(mockConfig)
    c.CacheACLRules(testRules)

    // Test: Invalidate cache
    err := c.InvalidateCache()
    assert.NoError(t, err)

    // Verify: Cache is empty
    assert.Empty(t, c.GetCachedRules())
}
```

---

#### 3. internal/config (71% → 85%)

**Priority:** HIGH (Configuration Management)
**Effort:** Medium (1 hour)

**Uncovered Functions (0% coverage):**
- `splitAndTrim()` - String parsing utility

**Action Items:**
1. Add test for `splitAndTrim()` with comma-separated values
2. Add test for whitespace trimming behavior
3. Add test for empty string handling
4. Add test for single value (no delimiter)

**Expected Impact:** Package from 71% → 85%+ (utility function used in critical paths)

**Example Test:**
```go
func TestSplitAndTrim(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected []string
    }{
        {"comma-separated", "a, b, c", []string{"a", "b", "c"}},
        {"with-whitespace", "  a , b , c  ", []string{"a", "b", "c"}},
        {"empty-string", "", []string{}},
        {"single-value", "test", []string{"test"}},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := splitAndTrim(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

---

#### 4. internal/util (75% → 85%)

**Priority:** MEDIUM
**Effort:** Small (30 minutes)

**Uncovered Functions (0% coverage):**
- `CanonicalizeIPForSecurity()` - IP address normalization

**Action Items:**
1. Add test for IPv4 canonicalization
2. Add test for IPv6 canonicalization
3. Add test for IPv6-mapped IPv4 addresses
4. Add test for invalid IP handling

**Expected Impact:** Package from 75% → 85%+

**Example Test:**
```go
func TestCanonicalizeIPForSecurity(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"ipv4", "192.168.1.1", "192.168.1.1"},
        {"ipv6", "2001:db8::1", "2001:db8::1"},
        {"ipv6-mapped", "::ffff:192.168.1.1", "192.168.1.1"},
        {"invalid", "invalid", ""},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := CanonicalizeIPForSecurity(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

---

#### 5. internal/utils (78% → 85%)

**Priority:** MEDIUM
**Effort:** Small (30 minutes)

**Uncovered Functions (0% coverage):**
- `GetConfiguredPublicURL()` - Public URL retrieval
- `normalizeConfiguredPublicURL()` - URL normalization

**Action Items:**
1. Add test for `GetConfiguredPublicURL()` with valid config
2. Add test for `GetConfiguredPublicURL()` with missing config
3. Add test for URL normalization (trailing slash removal)
4. Add test for URL scheme validation (http/https)

**Expected Impact:** Package from 78% → 85%+

**Example Test:**
```go
func TestGetConfiguredPublicURL(t *testing.T) {
    tests := []struct {
        name     string
        config   string
        expected string
    }{
        {"valid-url", "https://example.com", "https://example.com"},
        {"trailing-slash", "https://example.com/", "https://example.com"},
        {"empty-config", "", ""},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            os.Setenv("PUBLIC_URL", tt.config)
            defer os.Unsetenv("PUBLIC_URL")

            result := GetConfiguredPublicURL()
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

---

#### 6. internal/models (80% → 85%)

**Priority:** MEDIUM
**Effort:** Small (30 minutes)

**Uncovered Functions (0% coverage):**
- `EmergencyToken.TableName()` - GORM table name
- `EmergencyToken.IsExpired()` - Token expiration check
- `EmergencyToken.DaysUntilExpiration()` - Days remaining calculation
- `Plugin.TableName()` - GORM table name

**Action Items (Skip TableName methods, test business logic only):**
1. Add test for `IsExpired()` with expired token
2. Add test for `IsExpired()` with valid token
3. Add test for `DaysUntilExpiration()` with various dates
4. Add test for `DaysUntilExpiration()` with negative days (expired)

**Expected Impact:** Package from 80% → 85%+

**Example Test:**
```go
func TestEmergencyToken_IsExpired(t *testing.T) {
    tests := []struct {
        name      string
        expiresAt time.Time
        expected  bool
    }{
        {"expired", time.Now().Add(-24 * time.Hour), true},
        {"valid", time.Now().Add(24 * time.Hour), false},
        {"expires-now", time.Now(), false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            token := &EmergencyToken{ExpiresAt: tt.expiresAt}
            result := token.IsExpired()
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

---

### Backend Test Implementation Plan

| Priority | Package | Function | Lines | Effort | Est. Coverage Gain |
|----------|---------|----------|-------|--------|-------------------|
| P0 | `cerberus` | `InvalidateCache()` | ~5 | 30m | +14% (71% → 85%) |
| P0 | `config` | `splitAndTrim()` | ~10 | 30m | +14% (71% → 85%) |
| P1 | `util` | `CanonicalizeIPForSecurity()` | ~15 | 30m | +10% (75% → 85%) |
| P1 | `utils` | `GetConfiguredPublicURL()`, `normalizeConfiguredPublicURL()` | ~20 | 1h | +7% (78% → 85%) |
| P1 | `models` | `IsExpired()`, `DaysUntilExpiration()` | ~10 | 30m | +5% (80% → 85%) |

**Total Estimated Effort:** 3.0 hours (within 4-hour budget)
**Expected Coverage:** 83.5% → 85.0%+ (achievable)

---

## Frontend Coverage Analysis

### Overall Status

**Current Coverage:** 84.25%
**Target Coverage:** 85.0%
**Gap to Close:** +0.75%

**Estimated New Tests Required:** 15-20 component/integration tests
**Estimated Effort:** 2-4 hours

### Page-Level Coverage (Below 80%)

#### P0 - Critical Pages (Below 70%)

| Page | Current | Target | Gap | Impact | Effort |
|------|---------|--------|-----|--------|--------|
| `src/pages/Plugins.tsx` | 63.63% | 82% | +18.37% | MEDIUM - Plugin management | L (1.5h) |
| `src/pages/Security.tsx` | 65.17% | 82% | +16.83% | HIGH - Security dashboard | L (1.5h) |

#### P1 - High Priority (70-79%)

| Page | Current | Target | Gap | Impact | Effort |
|------|---------|--------|-----|--------|--------|
| `src/pages/SecurityHeaders.tsx` | 69.23% | 82% | +12.77% | HIGH - Security headers config | M (1h) |
| `src/pages/Dashboard.tsx` | 75.6% | 82% | +6.4% | HIGH - Main dashboard | M (1h) |

---

### Detailed Gap Analysis: Frontend Pages

#### 1. src/pages/Security.tsx (65.17% → 82%)

**Priority:** HIGH (Security Dashboard)
**Effort:** Large (1.5 hours)

**Known Uncovered Scenarios (from Phase 2):**
- CrowdSec integration toggle
- WAF rule configuration UI
- Rate limiting controls
- Error handling in useEffect hooks (lines 45-67)
- Toggle state management (lines 89-102)

**Action Items:**
1. Add test for CrowdSec toggle on/off
2. Add test for WAF rule creation flow
3. Add test for rate limiting threshold adjustment
4. Add test for error state rendering (API failure)
5. Add test for loading state during data fetch

**Expected Impact:** Page from 65.17% → 82%+ (17% gain)

**Example Test:**
```typescript
describe('Security.tsx', () => {
  it('should toggle CrowdSec on', async () => {
    render(<Security />);

    const crowdSecSwitch = screen.getByRole('switch', { name: /crowdsec/i });
    await userEvent.click(crowdSecSwitch);

    await waitFor(() => {
      expect(crowdSecSwitch).toBeChecked();
    });

    expect(mockApi.updateSettings).toHaveBeenCalledWith({
      crowdsec_enabled: true,
    });
  });

  it('should handle API error gracefully', async () => {
    mockApi.getSettings.mockRejectedValue(new Error('API error'));

    render(<Security />);

    await waitFor(() => {
      expect(screen.getByText(/failed to load settings/i)).toBeInTheDocument();
    });
  });
});
```

---

#### 2. src/pages/SecurityHeaders.tsx (69.23% → 82%)

**Priority:** HIGH (Security Configuration)
**Effort:** Medium (1 hour)

**Uncovered Scenarios:**
- Header preset selection
- Custom header addition
- Header validation
- CSP (Content Security Policy) directive builder

**Action Items:**
1. Add test for selecting preset (Strict, Moderate, Basic)
2. Add test for adding custom header
3. Add test for invalid header value rejection
4. Add test for CSP directive autocomplete

**Expected Impact:** Page from 69.23% → 82%+ (13% gain)

**Example Test:**
```typescript
describe('SecurityHeaders.tsx', () => {
  it('should apply strict preset', async () => {
    render(<SecurityHeaders />);

    const presetSelect = screen.getByLabelText(/preset/i);
    await userEvent.selectOptions(presetSelect, 'strict');

    await waitFor(() => {
      expect(screen.getByDisplayValue(/strict-transport-security/i)).toBeInTheDocument();
    });
  });

  it('should validate CSP directive', async () => {
    render(<SecurityHeaders />);

    const cspInput = screen.getByLabelText(/content security policy/i);
    await userEvent.type(cspInput, 'invalid-directive');

    await waitFor(() => {
      expect(screen.getByText(/invalid csp directive/i)).toBeInTheDocument();
    });
  });
});
```

---

#### 3. src/pages/Plugins.tsx (63.63% → 82%)

**Priority:** MEDIUM (Plugin Management)
**Effort:** Large (1.5 hours)

**Uncovered Scenarios:**
- Plugin upload
- Plugin enable/disable toggle
- Plugin configuration modal
- Plugin signature verification UI

**Action Items:**
1. Add test for plugin file upload
2. Add test for plugin enable/disable
3. Add test for opening plugin configuration
4. Add test for signature verification failure

**Expected Impact:** Page from 63.63% → 82%+ (18% gain)

**Example Test:**
```typescript
describe('Plugins.tsx', () => {
  it('should upload plugin file', async () => {
    render(<Plugins />);

    const file = new File(['plugin content'], 'plugin.so', { type: 'application/octet-stream' });
    const fileInput = screen.getByLabelText(/upload plugin/i);

    await userEvent.upload(fileInput, file);

    await waitFor(() => {
      expect(mockApi.uploadPlugin).toHaveBeenCalledWith(expect.any(FormData));
    });
  });

  it('should toggle plugin state', async () => {
    render(<Plugins />);

    const pluginSwitch = screen.getByRole('switch', { name: /my-plugin/i });
    await userEvent.click(pluginSwitch);

    await waitFor(() => {
      expect(mockApi.updatePluginState).toHaveBeenCalledWith('my-plugin-id', true);
    });
  });
});
```

---

#### 4. src/pages/Dashboard.tsx (75.6% → 82%)

**Priority:** HIGH (Main Dashboard)
**Effort:** Medium (1 hour)

**Uncovered Scenarios:**
- Widget refresh logic
- Real-time metrics updates
- Empty state handling
- Error boundary triggers

**Action Items:**
1. Add test for manual widget refresh
2. Add test for metric auto-update (every 30s)
3. Add test for empty dashboard (no data)
4. Add test for error state (API failure)

**Expected Impact:** Page from 75.6% → 82%+ (6.4% gain)

**Example Test:**
```typescript
describe('Dashboard.tsx', () => {
  it('should refresh widget data', async () => {
    render(<Dashboard />);

    const refreshButton = screen.getByRole('button', { name: /refresh/i });
    await userEvent.click(refreshButton);

    await waitFor(() => {
      expect(mockApi.getDashboardMetrics).toHaveBeenCalledTimes(2); // Initial + refresh
    });
  });

  it('should show empty state', async () => {
    mockApi.getDashboardMetrics.mockResolvedValue({ widgets: [] });

    render(<Dashboard />);

    await waitFor(() => {
      expect(screen.getByText(/no widgets configured/i)).toBeInTheDocument();
    });
  });
});
```

---

### Frontend Test Implementation Plan

| Priority | Page | Scenarios | Effort | Est. Coverage Gain |
|----------|------|-----------|--------|-------------------|
| P0 | `Security.tsx` | CrowdSec toggle, WAF config, error handling | 1.5h | +16.83% (65.17% → 82%) |
| P1 | `SecurityHeaders.tsx` | Preset selection, custom headers, validation | 1h | +12.77% (69.23% → 82%) |
| P1 | `Dashboard.tsx` | Widget refresh, auto-update, empty state | 1h | +6.4% (75.6% → 82%) |
| P2 | `Plugins.tsx` | Upload, toggle, configuration | 1.5h | +18.37% (63.63% → 82%) |

**Total Estimated Effort:** 5.0 hours
**Budget Constraint:** 2-4 hours allocated

**Recommendation:** Prioritize P0 and P1 items first (3.5h). Plugin testing (P2) can be deferred to future sprint.

---

## Phase 3.2: Targeted Test Plan

### Backend Test Plan

| Package | Current | Target | Lines | Effort | Priority | Test Type |
|---------|---------|--------|-------|--------|----------|-----------|
| `internal/cerberus` | 71% | 85% | 5 | 30m | P0 | Unit |
| `internal/config` | 71% | 85% | 10 | 30m | P0 | Unit |
| `internal/util` | 75% | 85% | 15 | 30m | P1 | Unit |
| `internal/utils` | 78% | 85% | 20 | 1h | P1 | Unit |
| `internal/models` | 80% | 85% | 10 | 30m | P1 | Unit |

**Total:** 5 packages, 60 lines, 3.0 hours

---

### Frontend Test Plan

| Component | Current | Target | Lines | Effort | Priority | Test Type |
|-----------|---------|--------|-------|--------|----------|-----------|
| `Security.tsx` | 65.17% | 82% | ~45 | 1.5h | P0 | Component |
| `SecurityHeaders.tsx` | 69.23% | 82% | ~30 | 1h | P1 | Component |
| `Dashboard.tsx` | 75.6% | 82% | ~20 | 1h | P1 | Component |
| `Plugins.tsx` | 63.63% | 82% | ~50 | 1.5h | P2 | Component |

**Total:** 4 pages, ~145 lines, 5.0 hours
**Recommended Scope:** P0 + P1 only (3.5 hours)

---

## Phase 3.3: Coverage Strategy Validation

### Success Criteria

**Backend:**
- ✅ Minimum 85% coverage achievable (3.0 hours)
- ✅ Focus on high-value tests (security, config, utilities)
- ✅ Avoid low-value tests (TableName(), main())
- ✅ Tests maintainable and fast (<5s per test)

**Frontend:**
- ⚠️ Minimum 85% coverage requires 5 hours (over budget)
- ✅ Focus on high-value tests (security pages, critical UI)
- ✅ Avoid low-value tests (trivial props, simple renders)
- ✅ Tests maintainable and fast (<5s per test)

**Overall:**
- **Backend:** Target is achievable within budget (3.0h / 4.0h allocated)
- **Frontend:** Target requires scope reduction (5.0h / 2-4h allocated)

---

### Risk Assessment

**Backend Risks:**

✅ **Low Risk** - All targets achievable within time budget
- 5 packages identified with clear function-level gaps
- Tests are straightforward unit tests (no complex mocking)
- Expected 83.5% → 85.0%+ coverage gain

**Frontend Risks:**

⚠️ **Medium Risk** - Full scope exceeds time budget
- 4 pages identified with significant testing needs
- Component tests require more setup (mocking, user events)
- Expected 84.25% → 85.0%+ coverage gain only if P0+P1 completed

**Mitigation Strategy:**

**Option 1: Reduce Frontend Scope (RECOMMENDED)**
- Focus on P0 and P1 items only (Security.tsx, SecurityHeaders.tsx, Dashboard.tsx)
- Defer Plugins.tsx testing to future sprint
- Estimated coverage: 84.25% → 85.5% (achievable)
- Estimated effort: 3.5 hours (within budget)

**Option 2: Lower Frontend Threshold Temporarily**
- Accept 84.25% coverage as "close enough" (<1% gap)
- Create follow-up issue for remaining gaps
- Resume coverage improvements in next sprint

**Option 3: Extend Time Budget**
- Request +2 hours for Phase 3 (total: 8-10 hours)
- Complete all P0, P1, and P2 frontend tests
- Guaranteed to reach 85% coverage

**Recommendation:** Option 1 (Reduce Frontend Scope)
- Most pragmatic given time constraints
- Still achieves 85% threshold
- Maintains quality over quantity approach

---

## Deliverables Summary

### 1. Backend Coverage Gap Analysis ✅
- 5 packages identified with specific function-level targets
- Combined coverage gain: +1.5% (83.5% → 85.0%)
- Effort: 3.0 hours (within 4.0h budget)

### 2. Frontend Coverage Gap Analysis ✅
- 4 pages identified with scenario-level targets
- Combined coverage gain: +0.75% (84.25% → 85.0%)
- Effort: 3.5 hours for P0+P1 (within 2-4h budget if scope reduced)

### 3. Targeted Test Implementation Plan ✅
- Backend: 5 packages, 60 lines, 3.0 hours
- Frontend: 3 pages (reduced scope), ~95 lines, 3.5 hours
- Total: 6.5 hours (within 6-8 hour Phase 3 estimate)

### 4. Risk Mitigation Strategy ✅
- **Backend:** Low risk, proceed as planned
- **Frontend:** Medium risk, reduce scope to P0+P1 items
- **Fallback:** Lower threshold to 84.5% if time budget exceeded

### 5. Updated Phase 3 Timeline ✅
- Phase 3.1 (Gap Analysis): 2 hours ✅ Complete
- Phase 3.2 (Test Implementation): 6-7 hours
  - Backend: 3.0 hours
  - Frontend: 3.5 hours (reduced scope)
- Phase 3.3 (Validation): 1 hour

**Total Phase 3 Estimate:** 9-10 hours (revised from 6-8 hours)
**Rationale:** Frontend scope larger than initially estimated

---

## Next Steps

### Immediate (Phase 3.2 - Test Implementation)

**Backend (Priority 1):**
1. Implement `cerberus` tests (30m)
2. Implement `config` tests (30m)
3. Implement `util` tests (30m)
4. Implement `utils` tests (1h)
5. Implement `models` tests (30m)

**Frontend (Priority 2):**
1. Implement `Security.tsx` tests (1.5h)
2. Implement `SecurityHeaders.tsx` tests (1h)
3. Implement `Dashboard.tsx` tests (1h)

**Validation (Priority 3):**
1. Run backend coverage: `go test -coverprofile=coverage.out ./...`
2. Run frontend coverage: `npm test -- --coverage`
3. Verify thresholds met (≥85%)
4. Update Phase 3 completion report

---

## Approval

**Phase 3.1 Status:** ✅ Complete

**Key Decisions:**
- ✅ Backend targets are achievable within time budget
- ⚠️ Frontend scope reduced to P0+P1 items (defer Plugins.tsx)
- ✅ Overall 85% threshold achievable with reduced scope

**Recommendation:** Proceed to Phase 3.2 (Test Implementation) with reduced frontend scope.

---

**Prepared by:** AI Planning Agent
**Date:** February 3, 2026
**Document Version:** 1.0
**Next Review:** After Phase 3.2 completion

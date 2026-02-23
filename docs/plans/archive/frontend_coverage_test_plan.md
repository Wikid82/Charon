# Frontend Coverage Gap Analysis and Test Plan

**Date**: 2026-01-25
**Goal**: Achieve 85.5%+ frontend test coverage (CI-safe buffer over 85% threshold)
**Current Status**: 85.06% (local) / 84.99% (CI)

---

## Executive Summary

**Coverage Gap**: 0.44% below target (85.5%)
**Required**: ~50-75 additional lines of coverage
**Strategy**: Target 4 high-impact files with strategic test additions
**Timeline**: 2-3 hours for implementation
**Risk**: LOW - Clear test patterns established, straightforward implementations

---

## Coverage Analysis

### Current Metrics (v8 Coverage Provider)

| Metric     | Percentage | Status |
|------------|------------|--------|
| Lines      | 85.75%     | ✅ Pass (85% threshold) |
| Statements | 85.06%     | ✅ Pass |
| Branches   | 78.23%     | ⚠️  Below ideal (80%+) |
| Functions  | 79.25%     | ⚠️  Below ideal (80%+) |

### CI Variance Analysis

- **Local**: 85.06% statements
- **CI**: 84.99% statements
- **Variance**: -0.07% (7 basis points)
- **Root Cause**: Timing/concurrency differences in CI environment
- **Mitigation**: Target 85.5% (50bp buffer) to ensure CI passes consistently

---

## Target Files (Ranked by Impact)

### 1. **Plugins.tsx** - HIGHEST PRIORITY ⚡

**Current Coverage**: 58.18% lines (lowest in codebase)
**File Size**: 391 lines
**Uncovered Lines**: ~163 lines
**Potential Gain**: +1.2% total coverage

**Why Target This File**:
- Lowest coverage in entire codebase (58%)
- Well-structured, testable component
- Clear user flows and state management
- Existing patterns for similar pages (ProxyHosts.tsx @ 94.46%)

**Uncovered Code Paths**:
1. ✘ Plugin toggle logic (enable/disable)
2. ✘ Reload plugins flow
3. ✘ Error handling branches
4. ✘ Metadata modal open/close
5. ✘ Status badge rendering logic (switch cases)
6. ✘ Built-in vs external plugin separation
7. ✘ Empty state rendering
8. ✘ Plugin grouping and filtering
9. ✘ Documentation URL handling
10. ✘ Modal metadata display

**Testing Complexity**: MEDIUM
**Expected Coverage After Tests**: 85-90%

---

### 2. **Tabs.tsx** - QUICK WIN 🎯

**Current Coverage**: 70% lines, 0% branches (!)
**File Size**: 59 lines
**Uncovered Lines**: ~18 lines
**Potential Gain**: +0.15% total coverage

**Why Target This File**:
- **CRITICAL**: 0% branch coverage despite being a UI primitive
- Small file = fast implementation
- Simple component with clear test patterns
- Used throughout app (high importance)
- Low-hanging fruit for immediate gains

**Uncovered Code Paths**:
1. ✘ TabsList rendering and className merging
2. ✘ TabsTrigger active/inactive states
3. ✘ TabsContent mount/unmount
4. ✘ Keyboard navigation (focus-visible states)
5. ✘ Disabled state handling
6. ✘ Custom className application

**Testing Complexity**: LOW
**Expected Coverage After Tests**: 95-100%

---

### 3. **Uptime.tsx** - HIGH IMPACT 📊

**Current Coverage**: 65.04% lines
**File Size**: 575 lines
**Uncovered Lines**: ~201 lines
**Potential Gain**: +1.5% total coverage

**Why Target This File**:
- Second-lowest page coverage
- Complex component with multiple sub-components
- Heavy mutation/query usage (good test patterns exist)
- Critical monitoring feature

**Uncovered Code Paths**:
1. ✘ MonitorCard status badge logic
2. ✘ Menu dropdown interactions
3. ✘ Monitor editing flow
4. ✘ Monitor deletion with confirmation
5. ✘ Health check trigger
6. ✘ Monitor creation form submission
7. ✘ History chart rendering
8. ✘ Empty state handling
9. ✘ Sync monitors flow
10. ✘ Error states and toast notifications

**Testing Complexity**: HIGH (multiple mutations, nested components)
**Expected Coverage After Tests**: 80-85%

---

### 4. **SecurityHeaders.tsx** - MEDIUM IMPACT 🛡️

**Current Coverage**: 64.61% lines
**File Size**: 339 lines
**Uncovered Lines**: ~120 lines
**Potential Gain**: +0.9% total coverage

**Why Target This File**:
- Third-lowest page coverage
- Critical security feature
- Complex CRUD operations
- Good reference: AuditLogs.tsx @ 84.37%

**Uncovered Code Paths**:
1. ✘ Profile creation form
2. ✘ Profile update flow
3. ✘ Delete with backup
4. ✘ Clone profile logic
5. ✘ Preset tooltip rendering
6. ✘ Profile grouping (custom vs presets)
7. ✘ Empty state for custom profiles
8. ✘ Built-in preset rendering loops
9. ✘ Dialog state management

**Testing Complexity**: MEDIUM
**Expected Coverage After Tests**: 78-82%

---

## Implementation Strategy

### Phase 1: Quick Wins (Target: +0.2%)

**Priority**: IMMEDIATE
**Files**: Tabs.tsx
**Estimated Time**: 30 minutes
**Expected Gain**: 0.15-0.2%

#### Test File: `frontend/src/components/ui/__tests__/Tabs.test.tsx`

**Test Cases**:
1. ✅ Tabs renders with default props
2. ✅ TabsList applies custom className
3. ✅ TabsTrigger renders active state
4. ✅ TabsTrigger renders inactive state
5. ✅ TabsTrigger handles disabled state
6. ✅ TabsContent shows when tab is active
7. ✅ TabsContent hides when tab is inactive
8. ✅ Focus states work correctly
9. ✅ Keyboard navigation (Tab, Arrow keys)
10. ✅ Custom props pass through

---

### Phase 2: High Impact (Target: +1.5%)

**Priority**: HIGH
**Files**: Plugins.tsx
**Estimated Time**: 1.5 hours
**Expected Gain**: 1.2-1.5%

#### Test File: `frontend/src/pages/__tests__/Plugins.test.tsx`

**Test Suites**:

##### Suite 1: Component Rendering (10 tests)
1. ✅ Renders loading state with skeletons
2. ✅ Renders empty state when no plugins
3. ✅ Renders built-in plugins section
4. ✅ Renders external plugins section
5. ✅ Displays plugin metadata correctly
6. ✅ Shows status badges (loaded, error, pending, disabled)
7. ✅ Renders header with reload button
8. ✅ Displays info alert
9. ✅ Groups plugins by type (built-in vs external)
10. ✅ Renders documentation links when available

##### Suite 2: Plugin Toggle Logic (8 tests)
1. ✅ Enables disabled plugin
2. ✅ Disables enabled plugin
3. ✅ Prevents toggling built-in plugins
4. ✅ Shows error toast for built-in toggle attempt
5. ✅ Shows success toast on enable
6. ✅ Shows success toast on disable
7. ✅ Shows error toast on toggle failure
8. ✅ Refetches plugins after toggle

##### Suite 3: Reload Plugins (5 tests)
1. ✅ Triggers reload mutation
2. ✅ Shows loading state during reload
3. ✅ Shows success toast with count
4. ✅ Shows error toast on failure
5. ✅ Refetches plugins after reload

##### Suite 4: Metadata Modal (7 tests)
1. ✅ Opens metadata modal on "Details" click
2. ✅ Closes metadata modal on close button
3. ✅ Displays all plugin metadata fields
4. ✅ Shows version when available
5. ✅ Shows author when available
6. ✅ Renders documentation link in modal
7. ✅ Shows error details when plugin has errors

##### Suite 5: Status Badge Rendering (4 tests)
1. ✅ Shows "Disabled" badge for disabled plugins
2. ✅ Shows "Loaded" badge for loaded plugins
3. ✅ Shows "Error" badge for error state
4. ✅ Shows "Pending" badge for pending state

---

### Phase 3: Additional Coverage (Target: +1.0%)

**Priority**: MEDIUM
**Files**: SecurityHeaders.tsx
**Estimated Time**: 1 hour
**Expected Gain**: 0.8-1.0%

#### Test File: `frontend/src/pages/__tests__/SecurityHeaders.test.tsx`

**Test Cases** (15 critical paths):
1. ✅ Renders loading state
2. ✅ Renders preset profiles
3. ✅ Renders custom profiles
4. ✅ Opens create profile dialog
5. ✅ Creates new profile
6. ✅ Opens edit dialog
7. ✅ Updates existing profile
8. ✅ Opens delete confirmation
9. ✅ Deletes profile with backup
10. ✅ Clones profile with "(Copy)" suffix
11. ✅ Displays security scores
12. ✅ Shows preset tooltips
13. ✅ Groups profiles correctly
14. ✅ Error handling for mutations
15. ✅ Empty state rendering

---

## Test Implementation Guide

### Technology Stack

- **Test Runner**: Vitest
- **Testing Library**: React Testing Library
- **Mocking**: Vitest vi.mock
- **Query Client**: @tanstack/react-query (with test wrapper)
- **Assertions**: @testing-library/jest-dom matchers

### Example Test Patterns

See the plan document for full code examples of:
1. Component Test Setup (Tabs.tsx example)
2. Page Component Test Setup (Plugins.tsx example)
3. Testing Hooks (Reference pattern)

---

## Expected Outcomes & Validation

### Coverage Targets Post-Implementation

| Phase | Files Tested | Expected Line Coverage | Expected Gain | Cumulative |
|-------|--------------|------------------------|---------------|------------|
| Baseline | - | 85.06% | - | 85.06% |
| Phase 1 | Tabs.tsx | 95-100% | +0.15% | 85.21% |
| Phase 2 | Plugins.tsx | 85-90% | +1.2% | 86.41% |
| Phase 3 | SecurityHeaders.tsx | 78-82% | +0.5% | 86.91% |
| **TOTAL** | **3 files** | **-** | **+1.85%** | **~86.9%** |

### Success Criteria

✅ **Primary Goal**: CI Coverage ≥ 85.0%
✅ **Stretch Goal**: CI Coverage ≥ 85.5% (buffer for variance)
✅ **Quality Goal**: All new tests follow established patterns
✅ **Maintenance Goal**: Tests are maintainable and descriptive

### CI Validation Process

1. **Local Verification**:
   ```bash
   cd frontend && npm run test:coverage
   # Verify: "All files" line shows ≥ 85.5%
   ```

2. **Pre-Push Check**:
   ```bash
   cd frontend && npm test
   # All tests must pass
   ```

3. **CI Pipeline**:
   - Frontend unit tests run automatically
   - Codecov reports coverage delta
   - CI fails if coverage drops below 85%

---

## Implementation Timeline

### Day 1: Quick Wins + Setup (2 hours)

**Morning** (1 hour):
- [ ] Implement Tabs.tsx tests (all 10 test cases)
- [ ] Run coverage locally: `npm run test:coverage`
- [ ] Verify Phase 1 gain: +0.15-0.2%
- [ ] Commit: "test: add comprehensive tests for Tabs component"

**Afternoon** (1 hour):
- [ ] Set up Plugins.tsx test file structure
- [ ] Implement Suite 1: Component Rendering (10 tests)
- [ ] Implement Suite 2: Plugin Toggle Logic (8 tests)
- [ ] Verify tests pass: `npm test Plugins.test.tsx`

### Day 2: High Impact Files (3 hours)

**Morning** (1.5 hours):
- [ ] Complete Plugins.tsx remaining suites:
  - Suite 3: Reload Plugins (5 tests)
  - Suite 4: Metadata Modal (7 tests)
  - Suite 5: Status Badge Rendering (4 tests)
- [ ] Run coverage: should be ~86.2%
- [ ] Commit: "test: add comprehensive tests for Plugins page"

**Afternoon** (1.5 hours):
- [ ] Implement SecurityHeaders.tsx tests (15 test cases)
- [ ] Focus on CRUD operations and state management
- [ ] Final coverage check: target 86.5-87%
- [ ] Commit: "test: add tests for SecurityHeaders page"

---

## Conclusion

This plan provides a systematic approach to closing the 0.44% coverage gap and establishing a solid 50bp buffer above the 85% threshold. By focusing on **Tabs.tsx** (quick win), **Plugins.tsx** (highest impact), and **SecurityHeaders.tsx** (medium impact), we can efficiently achieve 86.5-87% coverage with well-structured, maintainable tests.

**Next Step**: Begin Phase 1 implementation with Tabs.tsx tests.

---

**Plan Status**: ✅ COMPLETE
**Approved By**: Automated Analysis
**Implementation ETA**: 2-3 hours
**Expected Result**: 86.5-87% coverage (CI-safe)

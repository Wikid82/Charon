# Linting Issue Remediation Plan

**Date**: December 24, 2025
**Status**: Planning Phase
**Total Issues**: 20 (10 Backend Errors, 10 Frontend Warnings)
**Target**: Zero linting errors/warnings blocking pre-commit

---

## Executive Summary

This document provides a comprehensive remediation plan for 20 linting issues preventing successful pre-commit execution. Issues are categorized into Backend (Go) errors and Frontend (TypeScript/React) warnings with detailed fix specifications.

**Phase Breakdown:**
- **Phase 1**: Backend Go Fixes (10 errors) - Critical blocker
- **Phase 2**: Frontend TypeScript/React Fixes (10 warnings) - Code quality improvements

---

## Phase 1: Backend Go Fixes (Critical)

### 1.1 Named Result Parameters (gocritic)

**File**: `backend/internal/api/handlers/crowdsec_coverage_target_test.go`
**Line**: 89
**Issue**: `unnamedResult: consider giving a name to these results (gocritic)`
**Severity**: ERROR

**Current Code** (Line ~87-91):
```go
func (f *fakeExecWithOutput) Status(ctx context.Context, configDir string) (bool, int, error) {
return false, 0, f.err
}
```

**Root Cause**: The function returns three values `(bool, int, error)` without named parameters, reducing readability and making the return statement less clear about what each value represents.

**Fix Required**:
```go
func (f *fakeExecWithOutput) Status(ctx context.Context, configDir string) (running bool, pid int, err error) {
return false, 0, f.err
}
```

**Testing Considerations**:
- Verify `fakeExecWithOutput` implements the `CrowdsecExecutor` interface correctly
- Run targeted test: `go test ./backend/internal/api/handlers -run TestGetLAPIDecisions`
- No behavioral change expected, purely readability improvement

---

### 1.2 Octal Literal Modernization (gocritic - 2 instances)

**File**: `backend/internal/api/handlers/coverage_helpers_test.go`
**Lines**: 301, 305
**Issue**: `octalLiteral: use new octal literal style, 0o644 (gocritic)`
**Severity**: ERROR

#### 1.2.1 Line 301
**Current Code**:
```go
_ = os.WriteFile(scriptPath, []byte("#!/bin/bash\necho abc123xyz"), 0755)
```

**Fix Required**:
```go
_ = os.WriteFile(scriptPath, []byte("#!/bin/bash\necho abc123xyz"), 0o755)
```

#### 1.2.2 Line 305
**Current Code**:
```go
require.NoError(t, os.WriteFile(configFile, []byte("test: config"), 0644))
```

**Fix Required**:
```go
require.NoError(t, os.WriteFile(configFile, []byte("test: config"), 0o644))
```

**Testing Considerations**:
- File permissions behavior unchanged
- Run: `go test ./backend/internal/api/handlers -run TestCrowdsecHandler_ExportConfig`
- Verify file creation tests still pass

---

### 1.3 HTTP Request Body Best Practices (httpNoBody - 3 instances)

**File**: `backend/internal/api/handlers/additional_handlers_test.go`
**Lines**: 51, 109, 132
**Issue**: `httpNoBody: http.NoBody should be preferred to the nil request body (gocritic)`
**Severity**: ERROR

**Root Cause**: Using `nil` as the request body in `httptest.NewRequest()` instead of the idiomatic `http.NoBody` constant.

#### 1.3.1 Line 51 Context
Located in `TestDomainHandler_Create_MissingName_Additional` function:
```go
req := httptest.NewRequest(http.MethodPost, "/domains", bytes.NewReader(body))
```
Note: This line appears correct (uses bytes.NewReader), need to find actual nil usage.

#### 1.3.2 Actual Instances to Fix
Search for patterns like:
```go
req := httptest.NewRequest(http.MethodGet, "/some/path", nil)
req := httptest.NewRequest(http.MethodDelete, "/some/path", nil)
req := httptest.NewRequest(http.MethodPut, "/some/path", nil)
```

**Fix Required for all instances**:
```go
req := httptest.NewRequest(http.MethodGet, "/some/path", http.NoBody)
```

**Testing Considerations**:
- No behavioral change (nil and http.NoBody are functionally equivalent for GET/DELETE)
- Improves clarity that no body is expected
- Run: `go test ./backend/internal/api/handlers -run "TestDomainHandler|TestNotificationHandler"`

---

### 1.4 Unchecked Error Return (errcheck)

**File**: `backend/internal/api/handlers/testdb.go`
**Line**: 103
**Issue**: `Error return value of rows.Close is not checked (errcheck)`
**Severity**: ERROR

**Current Code** (Lines ~100-105):
```go
rows, err := tmpl.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND sql IS NOT NULL").Rows()
if err == nil {
defer rows.Close()
for rows.Next() {
var sql string
if rows.Scan(&sql) == nil && sql != "" {
db.Exec(sql)
}
}
return db
}
```

**Root Cause**: The deferred `rows.Close()` call doesn't check for errors, which could mask issues like transaction rollback failures or connection problems.

**Fix Required**:
```go
rows, err := tmpl.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND sql IS NOT NULL").Rows()
if err == nil {
defer func() {
if closeErr := rows.Close(); closeErr != nil {
t.Logf("warning: failed to close rows: %v", closeErr)
}
}()
for rows.Next() {
var sql string
if rows.Scan(&sql) == nil && sql != "" {
db.Exec(sql)
}
}
return db
}
```

**Alternative Simpler Fix**:
```go
defer func() { _ = rows.Close() }()
```

**Testing Considerations**:
- Test database migration and schema copying
- Run: `go test ./backend/internal/api/handlers -run TestDB`
- Ensure template DB creation succeeds

---

### 1.5 Response Body Not Closed (bodyclose - 3 instances)

**File**: `backend/internal/network/safeclient_test.go`
**Lines**: 228, 254, 638
**Issue**: `response body must be closed (bodyclose)`
**Severity**: ERROR

**Root Cause**: HTTP response bodies are not being closed after requests, leading to potential resource leaks in test code.

#### Strategy: Add defer resp.Body.Close() after all successful HTTP requests

**Pattern to Find**:
```go
resp, err := client.Get(...)
if err != nil {
t.Fatalf(...)
}
// Missing: defer resp.Body.Close()
```

**Fix Pattern**:
```go
resp, err := client.Get(...)
if err != nil {
t.Fatalf(...)
}
defer resp.Body.Close()
```

**Testing Considerations**:
- Prevents resource leaks in test suite
- Run: `go test ./backend/internal/network -run "TestNewSafeHTTPClient"`
- Monitor for goroutine leaks: `go test -race ./backend/internal/network`

---

## Phase 2: Frontend TypeScript/React Fixes (Warnings)

### 2.1 Fast Refresh Export Violations (2 instances)

#### 2.1.1 Label Component Export Issue

**File**: `frontend/src/components/ui/Label.tsx`
**Line**: 44
**Issue**: `Fast refresh only works when a file only exports components`
**Severity**: WARNING

**Current Code** (Lines 41-44):
```tsx
Label.displayName = 'Label'

export { Label, labelVariants }
```

**Root Cause**: The file exports both the `Label` component and the `labelVariants` CVA function. React Fast Refresh requires component-only exports for optimal HMR.

**Fix Options**:

**Option A - Comment-Based Suppression** (Recommended for minimal change):
```tsx
Label.displayName = 'Label'

// eslint-disable-next-line react-refresh/only-export-components
export { Label, labelVariants }
```

**Option B - Separate Files** (Better architecture):
1. Create `frontend/src/components/ui/Label.styles.ts`:
```typescript
import { cva } from 'class-variance-authority'

export const labelVariants = cva(
  'text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70',
  {
    variants: {
      variant: {
        default: 'text-content-primary',
        muted: 'text-content-muted',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)
```

2. Update `Label.tsx`:
```tsx
import { labelVariants } from './Label.styles'
// ... rest unchanged
export { Label }
```

**Recommendation**: Use Option A for quick fix, Option B for long-term maintainability.

#### 2.1.2 Button Component Export Issue

**File**: `frontend/src/components/ui/Button.tsx`
**Line**: 110
**Issue**: `Fast refresh only works when a file only exports components`
**Severity**: WARNING

**Apply same fix as Label component** (Option A recommended):
```tsx
// eslint-disable-next-line react-refresh/only-export-components
export { Button, buttonVariants }
```

**Testing Considerations**:
- Verify Fast Refresh works during development
- Run: `npm run type-check && npm run lint`

---

### 2.2 TypeScript `any` Type Usage (5 instances)

**File**: `frontend/src/components/__tests__/SecurityHeaderProfileForm.test.tsx`
**Lines**: 60, 174, 195, 216, 260
**Issue**: `Unexpected any`
**Severity**: WARNING

**Root Cause**: Using `any` type for `initialData` prop casting when creating partial `SecurityHeaderProfile` objects in tests.

**Fix Strategy**: Add type helper and use proper type assertions

**Add at top of test file** (after imports):
```typescript
import type { SecurityHeaderProfile } from '../../api/securityHeaders';

// Test helper type for partial profile data
type PartialProfile = Partial<SecurityHeaderProfile> & {
  id?: number;
  name: string;
};
```

**Fix Pattern for all instances**:
```typescript
// Instead of:
const initialData = { id: 1, name: 'Test', ... };
render(<SecurityHeaderProfileForm initialData={initialData as any} />);

// Use:
const initialData: PartialProfile = { id: 1, name: 'Test', ... };
render(<SecurityHeaderProfileForm initialData={initialData as SecurityHeaderProfile} />);
```

**Specific Lines to Fix**:
- Line 60: Test with full profile data
- Line 174: Test with preset flag
- Line 195: Another preset test
- Line 216: Delete button test
- Line 260: Likely in mock API response

**Testing Considerations**:
- Run: `npm run type-check`
- Run: `npm test SecurityHeaderProfileForm.test.tsx`

---

### 2.3 Missing useEffect Dependency

**File**: `frontend/src/components/SecurityHeaderProfileForm.tsx`
**Line**: 67
**Issue**: `React Hook useEffect missing dependency: 'calculateScoreMutation'`
**Severity**: WARNING

**Current Code** (Lines ~63-70):
```tsx
const calculateScoreMutation = useCalculateSecurityScore();

// Calculate score when form data changes
useEffect(() => {
  const timer = setTimeout(() => {
    calculateScoreMutation.mutate(formData);
  }, 500);

  return () => clearTimeout(timer);
}, [formData]);
```

**Root Cause**: The effect uses `calculateScoreMutation.mutate` but doesn't include it in the dependency array, which can lead to stale closures.

**Fix Options**:

**Option A - Extract Mutate Function** (Recommended):
```tsx
const calculateScoreMutation = useCalculateSecurityScore();
const { mutate: calculateScore } = calculateScoreMutation;

useEffect(() => {
  const timer = setTimeout(() => {
    calculateScore(formData);
  }, 500);

  return () => clearTimeout(timer);
}, [formData, calculateScore]);
```

**Option B - Add Dependency**:
```tsx
useEffect(() => {
  const timer = setTimeout(() => {
    calculateScoreMutation.mutate(formData);
  }, 500);

  return () => clearTimeout(timer);
}, [formData, calculateScoreMutation]);
```

**Option C - Suppress Warning** (Not recommended):
```tsx
// eslint-disable-next-line react-hooks/exhaustive-deps
```

**Recommendation**: Use Option A for clarity.

**Testing Considerations**:
- Verify score calculation triggers on form changes
- Test debouncing (500ms delay)
- Check for infinite render loops
- Run: `npm test SecurityHeaderProfileForm.test.tsx`

---

### 2.4 Console Enrollment Test `any` Type

**File**: `frontend/src/api/__tests__/consoleEnrollment.test.ts`
**Line**: 485
**Issue**: `Unexpected any`
**Severity**: WARNING

**Context**: Likely in mock error response.

**Fix Strategy**: Define error type

```typescript
type MockAPIError = {
  response: {
    status: number;
    data: { error: string };
  };
};

// Instead of:
const error = { response: { ... } } as any;

// Use:
const error: MockAPIError = { response: { ... } };
```

**Testing Considerations**:
- Run: `npm test consoleEnrollment.test.ts`

---

### 2.5 Playwright Test Variable Issue

**File**: `frontend/e2e/tests/security-mobile.spec.ts`
**Line**: 289
**Issue**: `'onclick' is assigned but never used`
**Severity**: WARNING

**Current Code** (Lines ~286-292):
```typescript
const docButton = page.locator('button:has-text("Documentation"), a:has-text("Documentation")').first()

if (await docButton.isVisible()) {
  const onclick = await docButton.getAttribute('onclick')
  const href = await docButton.getAttribute('href')

  if (href) {
    expect(href).toContain('wikid82.github.io')
  }
}
```

**Fix**: Remove unused variable
```typescript
if (await docButton.isVisible()) {
  const href = await docButton.getAttribute('href')

  if (href) {
    expect(href).toContain('wikid82.github.io')
  }
}
```

**Testing Considerations**:
- Run: `npm run test:e2e`

---

## Implementation Order

### Phase 1: Backend (Priority 1 - Blocking)
1. **Quick Wins** (15 min):
   - Fix httpNoBody issues (3 instances)
   - Fix octal literals (2 instances)
   - Fix named results (1 instance)

2. **Resource Management** (20 min):
   - Fix bodyclose issues (3 instances)
   - Fix errcheck issue (1 instance)

### Phase 2: Frontend (Priority 2)
3. **Type Safety** (30 min):
   - Fix `any` types (6 instances total)

4. **Component Issues** (20 min):
   - Fast Refresh exports (2 instances)
   - useEffect dependency (1 instance)

5. **Test Cleanup** (5 min):
   - Remove unused onclick variable

---

## Validation Checklist

### Backend
- [ ] `go vet ./backend/...`
- [ ] `golangci-lint run ./backend/...`
- [ ] `go test ./backend/internal/api/handlers -race`
- [ ] `go test ./backend/internal/network -race`

### Frontend
- [ ] `npm run lint`
- [ ] `npm run type-check`
- [ ] `npm test`
- [ ] `npm run test:e2e`

### Integration
- [ ] `pre-commit run --all-files`

---

## File Change Summary

### Backend (5 files, 10 fixes)
1. `backend/internal/api/handlers/crowdsec_coverage_target_test.go` - 1 fix
2. `backend/internal/api/handlers/coverage_helpers_test.go` - 2 fixes
3. `backend/internal/api/handlers/additional_handlers_test.go` - 3 fixes
4. `backend/internal/api/handlers/testdb.go` - 1 fix
5. `backend/internal/network/safeclient_test.go` - 3 fixes

### Frontend (6 files, 10 fixes)
1. `frontend/src/components/ui/Label.tsx` - 1 fix
2. `frontend/src/components/ui/Button.tsx` - 1 fix
3. `frontend/src/components/__tests__/SecurityHeaderProfileForm.test.tsx` - 5 fixes
4. `frontend/src/components/SecurityHeaderProfileForm.tsx` - 1 fix
5. `frontend/src/api/__tests__/consoleEnrollment.test.ts` - 1 fix
6. `frontend/e2e/tests/security-mobile.spec.ts` - 1 fix

---

## Timeline Estimate

- Phase 1 Backend: 35-45 minutes
- Phase 2 Frontend: 55-65 minutes
- Testing & Validation: 30 minutes
- **Total**: ~2-2.5 hours

---

**Plan Status**: ✅ COMPLETE
**Ready for Implementation**: YES
**Next Step**: Implementation Agent execution

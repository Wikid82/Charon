---
post_title: Pre-commit Blocker Remediation Plan
author1: "Charon Team"
post_slug: precommit-blocker-remediation
categories:
  - infrastructure
  - testing
tags:
  - ci
  - typescript
  - go
  - quick-fix
summary: "Quick fix plan for two critical pre-commit blockers: GolangCI-Lint version mismatch and TypeScript type errors."
post_date: "2026-02-12"
---

# Pre-commit Blocker Remediation Plan

**Status**: Ready for Implementation
**Priority**: Critical (Blocks commits)
**Estimated Time**: 15-20 minutes
**Confidence**: 95%

---

## 1. Introduction

Two critical blockers prevent commits:
1. **GolangCI-Lint Configuration**: Go version mismatch (built with 1.25, project uses 1.26)
2. **TypeScript Type Check**: 13 type errors in test file `src/components/__tests__/ProxyHostForm-dropdown-changes.test.tsx`

This plan provides exact commands, file changes, and verification steps to resolve both issues.

---

## 2. Issue Analysis

### 2.1 GolangCI-Lint Version Mismatch

**Error Message:**
```
Error: can't load config: the Go language version (go1.25) used to build
golangci-lint is lower than the targeted Go version (1.26)
```

**Root Cause:**
- GolangCI-Lint binary was built with Go 1.25
- Project's `go.mod` targets Go 1.26
- GolangCI-Lint refuses to run when built with older Go version than target

**Impact:**
- All Go linting blocked
- Cannot verify Go code quality
- Pre-commit hook fails with exit code 3

### 2.2 TypeScript Type Errors

**File:** `frontend/src/components/__tests__/ProxyHostForm-dropdown-changes.test.tsx`

**Error Categories:**

#### Category A: Invalid Property (Lines 92, 104)
Mock `SecurityHeaderProfile` objects use `headers: {}` property that doesn't exist in the type definition.

**Actual Type Definition** (`frontend/src/api/securityHeaders.ts`):
```typescript
export interface SecurityHeaderProfile {
  id: number;
  uuid: string;
  name: string;
  hsts_enabled: boolean;
  hsts_max_age: number;
  // ... (25+ security header properties)
  // NO "headers" property exists
}
```

#### Category B: Untyped Vitest Mocks (Lines 158, 202, 243, 281, 345)
Vitest `vi.fn()` calls lack explicit type parameters, resulting in generic `Mock<Procedure | Constructable>` type that doesn't match expected function signatures.

**Expected Types:**
- `onSaveSuccess`: `(data: Partial<ProxyHost>) => Promise<void>`
- `onClose`: `() => void`

---

## 3. Solution Specifications

### 3.1 GolangCI-Lint Fix

**Command:**
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

**What it does:**
- Downloads latest golangci-lint source
- Builds with current Go version (1.26)
- Installs to `$GOPATH/bin` or `$HOME/go/bin`

**Verification:**
```bash
golangci-lint version
```

**Expected Output:**
```
golangci-lint has version 1.xx.x built with go1.26.x from ...
```

### 3.2 TypeScript Type Fixes

#### Fix 1: Remove Invalid `headers` Property

**Lines 92, 104** - Remove the `headers: {}` property entirely from mock objects.

**Current (BROKEN):**
```typescript
const profile = {
  id: 1,
  uuid: 'profile-uuid-1',
  name: 'Basic Security',
  description: 'Basic security headers',
  is_preset: true,
  preset_type: 'basic',
  security_score: 60,
  headers: {},  // ❌ DOESN'T EXIST IN TYPE
  created_at: '2024-01-01',
  updated_at: '2024-01-01',
}
```

**Fixed:**
```typescript
const profile = {
  id: 1,
  uuid: 'profile-uuid-1',
  name: 'Basic Security',
  description: 'Basic security headers',
  is_preset: true,
  preset_type: 'basic',
  security_score: 60,
  // headers property removed
  created_at: '2024-01-01',
  updated_at: '2024-01-01',
}
```

#### Fix 2: Add Explicit Mock Types

**Lines 158, 202, 243, 281, 345** - Add type parameters to `vi.fn()` calls.

**Current Pattern (BROKEN):**
```typescript
onSaveSuccess: vi.fn(),  // ❌ Untyped mock
onClose: vi.fn(),        // ❌ Untyped mock
```

**Fixed Pattern (Option 1 - Type Assertions):**
```typescript
onSaveSuccess: vi.fn() as jest.MockedFunction<(data: Partial<ProxyHost>) => Promise<void>>,
onClose: vi.fn() as jest.MockedFunction<() => void>,
```

**Fixed Pattern (Option 2 - Generic Type Parameters - RECOMMENDED):**
```typescript
onSaveSuccess: vi.fn<[Partial<ProxyHost>], Promise<void>>(),
onClose: vi.fn<[], void>(),
```

**Rationale for Option 2:**
- More explicit and type-safe
- Better IDE autocomplete support
- Matches Vitest conventions
- Less boilerplate than type assertions

---

## 4. Implementation Steps

### Step 1: Rebuild GolangCI-Lint

```bash
# Rebuild golangci-lint with Go 1.26
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Verify version
golangci-lint version

# Test run (should no longer error on version)
golangci-lint run ./... --timeout=5m
```

**Expected Result:** No version error, linting runs successfully.

### Step 2: Fix TypeScript Type Errors

**File:** `frontend/src/components/__tests__/ProxyHostForm-dropdown-changes.test.tsx`

**Change 1: Line 92 (Remove `headers` property)**
```typescript
// BEFORE:
const mockHeaderProfiles = [
  {
    id: 1,
    uuid: 'profile-uuid-1',
    name: 'Basic Security',
    description: 'Basic security headers',
    is_preset: true,
    preset_type: 'basic',
    security_score: 60,
    headers: {},  // REMOVE THIS LINE
    created_at: '2024-01-01',
    updated_at: '2024-01-01',
  },

// AFTER:
const mockHeaderProfiles = [
  {
    id: 1,
    uuid: 'profile-uuid-1',
    name: 'Basic Security',
    description: 'Basic security headers',
    is_preset: true,
    preset_type: 'basic',
    security_score: 60,
    // headers property removed
    created_at: '2024-01-01',
    updated_at: '2024-01-01',
  },
```

**Change 2: Line 104 (Remove `headers` property from second profile)**
Same change as above for the second profile in the array.

**Change 3: Lines 158, 202, 243, 281, 345 (Add mock types)**

Find all occurrences of:
```typescript
onSaveSuccess: vi.fn(),
onClose: vi.fn(),
```

Replace with:
```typescript
onSaveSuccess: vi.fn<[Partial<ProxyHost>], Promise<void>>(),
onClose: vi.fn<[], void>(),
```

**Exact Line Changes:**

**Line 158:**
```typescript
// BEFORE:
<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />

// Context shows this is part of a render call
// Update the mock definitions above this line:
const mockOnSubmit = vi.fn<[Partial<ProxyHost>], Promise<void>>();
const mockOnCancel = vi.fn<[], void>();
```

Apply the same pattern for lines: 202, 243, 281, 345.

### Step 3: Verify Fixes

```bash
# Run TypeScript type check
cd /projects/Charon/frontend
npm run type-check

# Expected: 0 errors

# Run pre-commit checks
cd /projects/Charon
.github/skills/scripts/skill-runner.sh qa-precommit-all

# Expected: Exit code 0 (all hooks pass)
```

---

## 5. Acceptance Criteria

### GolangCI-Lint
- [ ] `golangci-lint version` shows built with Go 1.26.x
- [ ] `golangci-lint run` executes without version errors
- [ ] Pre-commit hook `golangci-lint-fast` passes

### TypeScript
- [ ] No `headers` property in mock SecurityHeaderProfile objects
- [ ] All `vi.fn()` calls have explicit type parameters
- [ ] `npm run type-check` exits with 0 errors
- [ ] Pre-commit hook `frontend-type-check` passes

### Overall
- [ ] `.github/skills/scripts/skill-runner.sh qa-precommit-all` exits code 0
- [ ] No new type errors introduced
- [ ] All 13 TypeScript errors resolved

---

## 6. Risk Assessment

**Risks:** Minimal

1. **GolangCI-Lint rebuild might fail if Go isn't installed**
   - Mitigation: Check Go version first (`go version`)
   - Expected: Go 1.26.x already installed

2. **Mock type changes might break test runtime behavior**
   - Mitigation: Run tests after type fixes
   - Expected: Tests still pass, only types are corrected

3. **Removing `headers` property might affect test assertions**
   - Mitigation: The property was never valid, so no test logic uses it
   - Expected: Tests pass without modification

**Confidence:** 95%

---

## 7. File Change Summary

### Files Modified

1. **`frontend/src/components/__tests__/ProxyHostForm-dropdown-changes.test.tsx`**
   - Lines 92, 104: Remove `headers: {}` from mock objects
   - Lines 158, 202, 243, 281, 345: Add explicit types to `vi.fn()` calls

### Files NOT Changed

- All Go source files (no code changes needed)
- `go.mod` (version stays at 1.26)
- GolangCI-Lint config (no changes needed)
- Other TypeScript files (errors isolated to one test file)

---

## 8. Verification Commands

### Quick Verification
```bash
# 1. Check Go version
go version

# 2. Rebuild golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 3. Verify golangci-lint version
golangci-lint version | grep "go1.26"

# 4. Fix TypeScript errors (manual edits per Step 2)

# 5. Run type check
cd /projects/Charon/frontend && npm run type-check

# 6. Run full pre-commit
cd /projects/Charon
.github/skills/scripts/skill-runner.sh qa-precommit-all
```

### Expected Output
```
✅ golangci-lint has version X.X.X built with go1.26.x
✅ TypeScript type check: 0 errors
✅ Pre-commit hooks: All hooks passed (exit code 0)
```

---

## 9. Time Estimates

| Task | Time |
|------|------|
| Rebuild GolangCI-Lint | 2 min |
| Fix TypeScript errors (remove headers) | 3 min |
| Fix TypeScript errors (add mock types) | 5 min |
| Run verification | 5 min |
| **Total** | **~15 min** |

---

## 10. Next Steps After Completion

1. Commit fixes with message:
   ```
   fix: resolve pre-commit blockers (golangci-lint + typescript)

   - Rebuild golangci-lint with Go 1.26
   - Remove invalid 'headers' property from SecurityHeaderProfile mocks
   - Add explicit types to Vitest mock functions

   Fixes 13 TypeScript errors in ProxyHostForm test
   Resolves golangci-lint version mismatch
   ```

2. Run pre-commit again to confirm:
   ```bash
   .github/skills/scripts/skill-runner.sh qa-precommit-all
   ```

3. Proceed with normal development workflow

---

## 11. Reference Links

- **Blocker Report:** `docs/reports/precommit_blockers.md`
- **SecurityHeaderProfile Type:** `frontend/src/api/securityHeaders.ts`
- **Test File:** `frontend/src/components/__tests__/ProxyHostForm-dropdown-changes.test.tsx`
- **GolangCI-Lint Docs:** https://golangci-lint.run/welcome/install/

---

**Plan Status:** ✅ Ready for Implementation
**Review Status:** Pending
**Implementation Agent:** Coding Agent

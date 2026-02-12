# Pre-commit Blocker Report

**Date**: 2026-02-12
**Command**: `.github/skills/scripts/skill-runner.sh qa-precommit-all`
**Exit Code**: 2 (FAILURE)

---

## Executive Summary

Two critical blockers prevent commits:
1. **GolangCI-Lint**: Configuration error - Go version mismatch
2. **TypeScript Type Check**: 13 type errors in test file

---

## 1. GolangCI-Lint Failure

### Error Summary
**Hook ID**: `golangci-lint-fast`
**Exit Code**: 3
**Status**: ❌ **BLOCKING**

### Root Cause
```
Error: can't load config: the Go language version (go1.25) used to build
golangci-lint is lower than the targeted Go version (1.26)
```

### Impact
- GolangCI-Lint cannot run because the binary was built with Go 1.25
- Project targets Go 1.26
- All Go linting is blocked

### Remediation
**Option 1: Rebuild golangci-lint with Go 1.26**
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

**Option 2: Update go.mod to target Go 1.25**
```go
// In go.mod
go 1.25
```

**Recommendation**: Option 1 (rebuild golangci-lint) is preferred to maintain Go 1.26 features.

---

## 2. TypeScript Type Check Failures

### Error Summary
**Hook ID**: `frontend-type-check`
**Exit Code**: 2
**File**: `src/components/__tests__/ProxyHostForm-dropdown-changes.test.tsx`
**Total Errors**: 13
**Status**: ❌ **BLOCKING**

### Error Breakdown

#### Category A: Missing Property Errors (2 errors)

**Error Type**: Object literal may only specify known properties

| Line | Error | Description |
|------|-------|-------------|
| 92 | TS2353 | `headers` does not exist in type 'SecurityHeaderProfile' |
| 104 | TS2353 | `headers` does not exist in type 'SecurityHeaderProfile' |

**Root Cause**: Test creates `SecurityHeaderProfile` objects with a `headers` property that doesn't exist in the type definition.

**Code Example** (Line 92):
```typescript
// Current (FAILS)
const profile = {
  headers: { /* ... */ }  // ❌ 'headers' not in SecurityHeaderProfile
} as SecurityHeaderProfile;

// Fix Required
const profile = {
  // Use correct property name from SecurityHeaderProfile type
} as SecurityHeaderProfile;
```

**Remediation**:
1. Check `SecurityHeaderProfile` type definition
2. Remove or rename `headers` property to match actual type
3. Update mock data structure in test

---

#### Category B: Mock Type Mismatch Errors (11 errors)

**Error Type**: Type mismatch for Vitest mock functions

| Line | Column | Error | Expected Type | Actual Type |
|------|--------|-------|---------------|-------------|
| 158 | 24 | TS2322 | `(data: Partial<ProxyHost>) => Promise<void>` | `Mock<Procedure \| Constructable>` |
| 158 | 48 | TS2322 | `() => void` | `Mock<Procedure \| Constructable>` |
| 202 | 24 | TS2322 | `(data: Partial<ProxyHost>) => Promise<void>` | `Mock<Procedure \| Constructable>` |
| 202 | 48 | TS2322 | `() => void` | `Mock<Procedure \| Constructable>` |
| 243 | 24 | TS2322 | `(data: Partial<ProxyHost>) => Promise<void>` | `Mock<Procedure \| Constructable>` |
| 243 | 48 | TS2322 | `() => void` | `Mock<Procedure \| Constructable>` |
| 281 | 24 | TS2322 | `(data: Partial<ProxyHost>) => Promise<void>` | `Mock<Procedure \| Constructable>` |
| 281 | 48 | TS2322 | `() => void` | `Mock<Procedure \| Constructable>` |
| 345 | 44 | TS2322 | `(data: Partial<ProxyHost>) => Promise<void>` | `Mock<Procedure \| Constructable>` |
| 345 | 68 | TS2322 | `() => void` | `Mock<Procedure \| Constructable>` |

**Root Cause**: Vitest mock functions are not properly typed. The generic `Mock<Procedure | Constructable>` type doesn't match the expected function signatures.

**Pattern Analysis**:
- Lines 158, 202, 243, 281, 345 (column 24): Mock for async ProxyHost operation
- Lines 158, 202, 243, 281, 345 (column 48): Mock for void return callback

**Code Pattern** (Lines 158, 202, 243, 281, 345):
```typescript
// Current (FAILS)
onSaveSuccess: vi.fn(),  // ❌ Type: Mock<Procedure | Constructable>
onClose: vi.fn(),        // ❌ Type: Mock<Procedure | Constructable>

// Fix Required - Add explicit type
onSaveSuccess: vi.fn() as unknown as (data: Partial<ProxyHost>) => Promise<void>,
onClose: vi.fn() as unknown as () => void,

// OR: Use Mock type helper
onSaveSuccess: vi.fn<[Partial<ProxyHost>], Promise<void>>(),
onClose: vi.fn<[], void>(),
```

**Remediation Options**:

**Option 1: Type Assertions (Quick Fix)**
```typescript
onSaveSuccess: vi.fn() as any,
onClose: vi.fn() as any,
```

**Option 2: Explicit Mock Types (Recommended)**
```typescript
import { vi, Mock } from 'vitest';

onSaveSuccess: vi.fn<[Partial<ProxyHost>], Promise<void>>(),
onClose: vi.fn<[], void>(),
```

**Option 3: Extract Mock Factory**
```typescript
// Create typed mock factory
const createProxyHostMocks = () => ({
  onSaveSuccess: vi.fn((data: Partial<ProxyHost>) => Promise.resolve()),
  onClose: vi.fn(() => {}),
});

// Use in tests
const mocks = createProxyHostMocks();
```

---

## 3. Passing Hooks (No Action Required)

The following hooks passed successfully:
- ✅ fix end of files
- ✅ trim trailing whitespace
- ✅ check yaml
- ✅ check for added large files
- ✅ shellcheck
- ✅ actionlint (GitHub Actions)
- ✅ dockerfile validation
- ✅ Go Vet
- ✅ Check .version matches latest Git tag
- ✅ Prevent large files that are not tracked by LFS
- ✅ Prevent committing CodeQL DB artifacts
- ✅ Prevent committing data/backups files
- ✅ Frontend Lint (Fix)

---

## Priority Action Plan

### Immediate (Cannot commit without these)

1. **Fix GolangCI-Lint Version Mismatch** (5 minutes)
   ```bash
   # Rebuild golangci-lint with current Go version
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

   # Or update go.mod temporarily
   # go mod edit -go=1.25
   ```

2. **Fix TypeScript Errors in ProxyHostForm Test** (15-30 minutes)
   - File: `src/components/__tests__/ProxyHostForm-dropdown-changes.test.tsx`
   - Fix lines: 92, 104, 158, 202, 243, 281, 345
   - See remediation sections above for code examples

### Recommended Execution Order

1. **GolangCI-Lint first** → Enables Go linting checks
2. **TypeScript errors** → Enables type checking to pass
3. **Re-run pre-commit** → Verify all issues resolved

---

## Verification Commands

After fixes, verify with:

```bash
# Full pre-commit check
.github/skills/scripts/skill-runner.sh qa-precommit-all

# TypeScript check only
cd frontend && npm run type-check

# GolangCI-Lint check only
golangci-lint --version
golangci-lint run
```

---

## Success Criteria

- [ ] GolangCI-Lint runs without version errors
- [ ] TypeScript type check passes with 0 errors
- [ ] Pre-commit hook exits with code 0
- [ ] All 13 TypeScript errors in test file resolved
- [ ] No new errors introduced

---

## Additional Notes

### GolangCI-Lint Investigation
Check current versions:
```bash
go version              # Should show go1.26
golangci-lint version   # Currently built with go1.25
```

### TypeScript Type Definitions
Review type files to understand correct structure:
```bash
# Find SecurityHeaderProfile definition
grep -r "SecurityHeaderProfile" src/ --include="*.ts" --include="*.tsx"

# Check import statements in test file
head -20 src/components/__tests__/ProxyHostForm-dropdown-changes.test.tsx
```

---

**Report Generated**: 2026-02-12
**Status**: 🔴 **BLOCKING** - 2 critical failures prevent commits

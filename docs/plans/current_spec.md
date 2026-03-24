# Fix: Frontend Unit Test i18n Failures in BulkDeleteCertificateDialog

> **Status:** Ready for implementation
> **Severity:** CI-blocking (2 test failures)
> **Scope:** Single test file change

---

## 1. Introduction

Two frontend unit tests fail in CI because `BulkDeleteCertificateDialog.test.tsx` contains a local `vi.mock('react-i18next')` that overrides the global mock in the test setup. The local mock returns raw translation keys and JSON-serialized options instead of resolved English strings, causing assertion mismatches.

### Objectives

- Fix the 2 failing tests in CI
- Align `BulkDeleteCertificateDialog.test.tsx` with the project's established i18n test pattern
- No behavioral or component changes required

---

## 2. Research Findings

### 2.1 Failing Tests (from CI log)

| # | Test Name | Expected | Actual (DOM) |
|---|-----------|----------|--------------|
| 1 | `lists each certificate name in the scrollable list` | `"Custom"`, `"Staging"`, `"Expired LE"` | `certificates.providerCustom`, `certificates.providerStaging`, `certificates.providerExpiredLE` |
| 2 | `renders "Expiring LE" label for a letsencrypt cert with status expiring` | `"Expiring LE"` | `certificates.providerExpiringLE` |

Additional rendering artifacts visible in the DOM dump:

- Dialog title: `{"count":3}` instead of `"Delete 3 Certificate(s)"`
- Button text: `{"count":3}` instead of `"Delete 3 Certificate(s)"`
- Cancel button: `common.cancel` instead of `"Cancel"`
- Warning text: `certificates.bulkDeleteConfirm` instead of translated string
- Aria label: `certificates.bulkDeleteListAriaLabel` instead of translated string

### 2.2 Relevant File Paths

| File | Role |
|------|------|
| `frontend/src/components/dialogs/__tests__/BulkDeleteCertificateDialog.test.tsx` | **Failing test file** — contains the problematic local mock |
| `frontend/src/components/dialogs/BulkDeleteCertificateDialog.tsx` | Component under test |
| `frontend/src/test/setup.ts` | Global test setup with proper i18n mock (lines 20–60) |
| `frontend/vitest.config.ts` | Vitest config — confirms `setupFiles: './src/test/setup.ts'` (line 24) |
| `frontend/src/locales/en/translation.json` | English translations source |

### 2.3 i18n Mock Architecture

**Global mock** (`frontend/src/test/setup.ts`, lines 20–60):

- Dynamically imports `../locales/en/translation.json`
- Implements `getTranslation(key)` that resolves dot-notation keys (e.g., `certificates.providerCustom` → `"Custom"`)
- Handles `{{variable}}` interpolation via regex replacement
- Applied automatically to all test files via `setupFiles` in vitest config

**Local mock** (`BulkDeleteCertificateDialog.test.tsx`, lines 9–14):

```typescript
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => (opts ? JSON.stringify(opts) : key),
    i18n: { language: 'en', changeLanguage: vi.fn() },
  }),
}))
```

This local mock **overrides** the global mock because Vitest's `vi.mock()` at the file level takes precedence over the setup file's `vi.mock()`. It returns:

- Raw key when no options: `t('certificates.providerCustom')` → `"certificates.providerCustom"`
- JSON string when options present: `t('key', { count: 3 })` → `'{"count":3}'`

### 2.4 Translation Keys Required

From `frontend/src/locales/en/translation.json`:

| Key | English Value |
|-----|---------------|
| `certificates.bulkDeleteTitle` | `"Delete {{count}} Certificate(s)"` |
| `certificates.bulkDeleteDescription` | `"Delete {{count}} certificate(s)"` |
| `certificates.bulkDeleteConfirm` | `"The following certificates will be permanently deleted. The server creates a backup before each removal."` |
| `certificates.bulkDeleteListAriaLabel` | `"Certificates to be deleted"` |
| `certificates.bulkDeleteButton` | `"Delete {{count}} Certificate(s)"` |
| `certificates.providerStaging` | `"Staging"` |
| `certificates.providerCustom` | `"Custom"` |
| `certificates.providerExpiredLE` | `"Expired LE"` |
| `certificates.providerExpiringLE` | `"Expiring LE"` |
| `common.cancel` | `"Cancel"` |

All keys exist in the translation file. No missing translations.

### 2.5 Pattern Analysis — Other Test Files

20+ test files have local `vi.mock('react-i18next')` overrides. Most use `t: (key) => key` and assert against raw keys — this is internally consistent and **not failing**. The `BulkDeleteCertificateDialog.test.tsx` file is unique because its **assertions expect translated values** while its mock returns raw keys.

| File | Local Mock | Assertions | Status |
|------|-----------|------------|--------|
| `CertificateList.test.tsx` | `t: (key) => key` | Raw keys (`certificates.deleteTitle`) | Passing |
| `Certificates.test.tsx` | Custom translations map | Translated values | Passing |
| `AccessLists.test.tsx` | Custom translations map | Translated values | Passing |
| **BulkDeleteCertificateDialog.test.tsx** | `t: (key, opts) => opts ? JSON.stringify(opts) : key` | **Mix of translated values AND raw keys** | **Failing** |

---

## 3. Root Cause Analysis

**The local `vi.mock('react-i18next')` in `BulkDeleteCertificateDialog.test.tsx` returns raw translation keys, but the test assertions expect resolved English strings.**

This is a mock/assertion mismatch introduced when the test was authored. The test expectations (`'Custom'`, `'Expiring LE'`) are correct for what the component should render, but the mock prevents translation resolution.

---

## 4. Technical Specification

### 4.1 Fix: Remove Local Mock, Update Assertions

**File:** `frontend/src/components/dialogs/__tests__/BulkDeleteCertificateDialog.test.tsx`

**Change 1 — Delete the local `vi.mock('react-i18next', ...)` block (lines 9–14)**

Removing this allows the global mock from `setup.ts` to take effect, which properly resolves translation keys to English values with interpolation.

**Change 2 — Update assertions that relied on the local mock's behavior**

With the global mock active, translation calls resolve differently:

| Call in component | Local mock output | Global mock output |
|-------------------|-------------------|--------------------|
| `t('certificates.bulkDeleteTitle', { count: 3 })` | `'{"count":3}'` | `'Delete 3 Certificate(s)'` |
| `t('certificates.bulkDeleteButton', { count: 3 })` | `'{"count":3}'` | `'Delete 3 Certificate(s)'` |
| `t('certificates.bulkDeleteButton', { count: 1 })` | `'{"count":1}'` | `'Delete 1 Certificate(s)'` |
| `t('common.cancel')` | `'common.cancel'` | `'Cancel'` |
| `t('certificates.providerCustom')` | `'certificates.providerCustom'` | `'Custom'` |
| `t('certificates.providerExpiringLE')` | `'certificates.providerExpiringLE'` | `'Expiring LE'` |

Assertions to update:

| Line | Old Assertion | New Assertion |
|------|---------------|---------------|
| ~48 | `getByRole('heading', { name: '{"count":3}' })` | `getByRole('heading', { name: 'Delete 3 Certificate(s)' })` |
| ~82 | `getByRole('button', { name: '{"count":3}' })` | `getByRole('button', { name: 'Delete 3 Certificate(s)' })` |
| ~95 | `getByRole('button', { name: 'common.cancel' })` | `getByRole('button', { name: 'Cancel' })` |
| ~109 | `getByRole('button', { name: '{"count":3}' })` | `getByRole('button', { name: 'Delete 3 Certificate(s)' })` |
| ~111 | `getByRole('button', { name: 'common.cancel' })` | `getByRole('button', { name: 'Cancel' })` |

The currently-failing assertions (`getByText('Custom')`, `getByText('Expiring LE')`, etc.) will pass without changes once the global mock is active.

### 4.2 Config File Review

| File | Finding |
|------|---------|
| `.gitignore` | No changes needed. Test artifacts, coverage outputs, and CI logs are properly excluded. |
| `codecov.yml` | No changes needed. Test files (`**/__tests__/**`, `**/*.test.tsx`) and test setup (`**/vitest.config.ts`, `**/vitest.setup.ts`) are already excluded from coverage. |
| `.dockerignore` | No changes needed. Test artifacts and coverage files are excluded from Docker builds. |
| `Dockerfile` | No changes needed. No test files are copied into the production image. |

---

## 5. Implementation Plan

### Phase 1: Fix the Test File

**Single file edit:** `frontend/src/components/dialogs/__tests__/BulkDeleteCertificateDialog.test.tsx`

1. Remove the local `vi.mock('react-i18next', ...)` block (lines 9–14)
2. Update 5 assertion strings to use resolved English translations (see table in §4.1)
3. No other files need changes

### Phase 2: Validation

1. Run the specific test file: `cd /projects/Charon/frontend && npx vitest run src/components/dialogs/__tests__/BulkDeleteCertificateDialog.test.tsx`
2. Run the full frontend test suite: `cd /projects/Charon/frontend && npx vitest run`
3. Verify no regressions in other test files

---

## 6. Acceptance Criteria

- [ ] Both failing tests pass: `lists each certificate name in the scrollable list` and `renders "Expiring LE" label for a letsencrypt cert with status expiring`
- [ ] All 7 tests in `BulkDeleteCertificateDialog.test.tsx` pass
- [ ] Full frontend test suite passes with no new failures
- [ ] No local `vi.mock('react-i18next')` remains in `BulkDeleteCertificateDialog.test.tsx`

---

## 7. Commit Slicing Strategy

### Decision: Single PR

**Rationale:** This is a single-file fix with no cross-domain changes, no schema changes, no API changes, and no risk of affecting other components. The change is purely correcting assertion/mock alignment in one test file.

### PR-1: Fix BulkDeleteCertificateDialog i18n test mock

| Attribute | Value |
|-----------|-------|
| **Scope** | Remove local i18n mock override, update 5 assertions |
| **Files** | `frontend/src/components/dialogs/__tests__/BulkDeleteCertificateDialog.test.tsx` |
| **Dependencies** | None |
| **Validation Gate** | All 7 tests in the file pass; full frontend suite green |
| **Rollback** | Revert single commit |

### Contingency

If the global mock from `setup.ts` does not resolve all keys correctly (unlikely given the translation JSON analysis), the fallback is to replace the local mock with a custom translations map pattern (as used in `AccessLists.test.tsx` and `Certificates.test.tsx`) containing the exact keys needed by this component.

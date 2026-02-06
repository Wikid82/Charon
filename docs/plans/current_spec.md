# QA Remediation Plan: Frontend Coverage, Type-Check, and Threshold Alignment

**Objective**: Complete the QA remediation for frontend test coverage and TypeScript type-checking, while aligning local vs CI coverage thresholds and auditing coverage configuration hygiene.

**Scope**: Frontend test suite, coverage thresholds, CI checks, and related config files.

**Status (Feb 2026)**: Draft plan pending approval.

---

## 1. Introduction

This plan focuses on removing the friction points flagged in [docs/reports/qa_report.md](docs/reports/qa_report.md): a narrow frontend coverage gap, TypeScript test type errors, and inconsistent local vs CI coverage thresholds. The goal is to make the QA bar feel like a single, well-lit runway: every local run should match what CI expects, and coverage should be earned through tests that describe behavior, not through fragile exclusions.

---

## 2. Research Findings

### 2.1. Coverage and Thresholds
- Local/CI thresholds diverge in [frontend/vitest.config.ts](frontend/vitest.config.ts): `coverageThreshold` is 87.5 locally but 85 when `CI=true`.
- The frontend coverage script [scripts/frontend-test-coverage.sh](scripts/frontend-test-coverage.sh) enforces `MIN_COVERAGE` with a default of 85 (from `CHARON_MIN_COVERAGE` or `CPM_MIN_COVERAGE`).
- Codecov thresholds are currently permissive: [codecov.yml](codecov.yml) uses patch target 85 and project target auto + 1% threshold.
- Coverage gaps cited in QA report: [frontend/src/api/client.ts](frontend/src/api/client.ts), [frontend/src/components/CrowdSecKeyWarning.tsx](frontend/src/components/CrowdSecKeyWarning.tsx), and [frontend/src/locales](frontend/src/locales).

### 2.2. Type-Check Failures (Tests)
- `npm run type-check` uses `tsc --noEmit` with [frontend/tsconfig.json](frontend/tsconfig.json) including tests.
- QA report lists errors in test mocks and unused imports. Likely candidates include:
  - [frontend/src/components/__tests__/AccessListForm.test.tsx](frontend/src/components/__tests__/AccessListForm.test.tsx)
  - [frontend/src/components/__tests__/DNSProviderForm.test.tsx](frontend/src/components/__tests__/DNSProviderForm.test.tsx)
  - [frontend/src/pages/__tests__/CrowdSecConfig.test.tsx](frontend/src/pages/__tests__/CrowdSecConfig.test.tsx)
  - Potentially other tests referenced by the type-check output.

### 2.3. API Types Driving Test Mocks
- Access list types: [frontend/src/api/accessLists.ts](frontend/src/api/accessLists.ts).
- DNS provider types: [frontend/src/api/dnsProviders.ts](frontend/src/api/dnsProviders.ts) and hooks in [frontend/src/hooks/useDNSProviders.ts](frontend/src/hooks/useDNSProviders.ts).
- CrowdSec decisions: [frontend/src/api/crowdsec.ts](frontend/src/api/crowdsec.ts).
- Proxy host types: [frontend/src/api/proxyHosts.ts](frontend/src/api/proxyHosts.ts).

### 2.4. CI Enforcement Points
- Coverage run in CI: [scripts/frontend-test-coverage.sh](scripts/frontend-test-coverage.sh) called from [quality-checks.yml](.github/workflows/quality-checks.yml) and [codecov-upload.yml](.github/workflows/codecov-upload.yml).
- Playwright coverage is optional and gated by `PLAYWRIGHT_COVERAGE` in [e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml).

### 2.5. Config Hygiene Review (Requested Files)
- [codecov.yml](codecov.yml): patch threshold is 85; project threshold is auto + 1%.
- [.gitignore](.gitignore): already ignores `playwright/.auth/` and `playwright-report/`.
- [.dockerignore](.dockerignore): excludes Playwright and test artifacts.
- [Dockerfile](Dockerfile): unrelated to coverage/type-check; no changes expected.

---

## 3. Requirements (EARS Notation)

- WHEN coverage is collected for frontend or backend tests, THE SYSTEM SHALL use the skill runner as the primary path and treat direct scripts as a legacy fallback only.
- WHEN the frontend test suite runs locally, THE SYSTEM SHALL enforce the same coverage threshold logic as CI to avoid threshold drift.
- WHEN TypeScript type-check runs against test files, THE SYSTEM SHALL report zero type errors and zero unused symbol violations.
- WHEN coverage reports are generated, THE SYSTEM SHALL include meaningful tests for API client and UI warning paths instead of relying on exclusions.
- WHEN Codecov evaluates patch coverage, THE SYSTEM SHALL require 100% patch coverage for modified lines.
- WHEN Playwright artifacts are generated, THE SYSTEM SHALL prevent secrets (e.g., `playwright/.auth`) from being tracked or uploaded inadvertently.
- WHEN any test or coverage run is executed, THE SYSTEM SHALL execute E2E tests first to validate UI/UX stability before unit or integration tests.

---

## 4. Technical Specifications

### 4.1. Frontend Coverage Targets

**Target**: Restore frontend statements coverage to >= 87.5% (or align both local and CI to a single value, based on decision below).

**Key files to cover**:
- [frontend/src/api/client.ts](frontend/src/api/client.ts)
  - Functions: `setAuthToken`, `setAuthErrorHandler`, and Axios response interceptor behavior.
- [frontend/src/components/CrowdSecKeyWarning.tsx](frontend/src/components/CrowdSecKeyWarning.tsx)
  - Behaviors: dismiss logic (localStorage), copy action, show/hide key, and banner gating.
- [frontend/src/locales](frontend/src/locales)
  - Decide whether to exclude translation JSON files from coverage or add a small locale health test.

### 4.2. Type-Check Corrections

**Goal**: Align test mocks to current domain types and remove unused imports.

**Primary target files**:
- [frontend/src/components/__tests__/AccessListForm.test.tsx](frontend/src/components/__tests__/AccessListForm.test.tsx)
  - Ensure `initialData` matches `AccessList` fields; add missing fields if type errors persist.
- [frontend/src/components/__tests__/DNSProviderForm.test.tsx](frontend/src/components/__tests__/DNSProviderForm.test.tsx)
  - Confirm mock provider uses the exact `DNSProvider` shape (remove extra fields or extend type if backend returns them).
- [frontend/src/pages/__tests__/CrowdSecConfig.test.tsx](frontend/src/pages/__tests__/CrowdSecConfig.test.tsx)
  - Confirm `CrowdSecDecision.id` is a string per [frontend/src/api/crowdsec.ts](frontend/src/api/crowdsec.ts).

### 4.3. Threshold Alignment Strategy

**Decision needed**: unify local and CI thresholds.

Options:
1. **Single Threshold Everywhere (Recommended)**
    - Use one value (e.g., 87.5) via env-driven config.
    - Update [frontend/vitest.config.ts](frontend/vitest.config.ts) to read from `CHARON_MIN_COVERAGE` or a new `VITE_COVERAGE_THRESHOLD`.
    - Update [scripts/frontend-test-coverage.sh](scripts/frontend-test-coverage.sh) to default to the same value.
2. **Explicit Local/CI Split (Documented)**
    - Keep 87.5 local, 85 CI, but document it clearly and reflect it in QA expectations.
    - Add a README or QA policy note to avoid confusion.

### 4.4. Codecov Configuration Alignment

- Set patch target in [codecov.yml](codecov.yml) to 100% and treat it as mandatory for every PR.
- Maintain project thresholds consistent with the chosen coverage target.

### 4.5. Coverage Execution Priority

- Primary: use the skill runner for coverage collection (Playwright coverage and test coverage scripts).
- Legacy fallback: allow direct script invocation only when the skill runner cannot be used; record the reason.

### 4.6. Locale Coverage Consistency

Decision required: keep locale coverage consistent with Codecov by explicitly excluding locale resources.

Options:
1. **Exclude locale resources (Recommended)**
  - Add `frontend/src/locales/**` to coverage exclude in [frontend/vitest.config.ts](frontend/vitest.config.ts).
  - Add a matching Codecov ignore entry for `frontend/src/locales/**` to keep reports consistent.
2. **Test locale resources**
  - Add a small health test that imports locale JSON and validates required keys.
  - Keep Codecov ignore list unchanged.

### 4.5. Config Hygiene Review

- [.gitignore](.gitignore): verify `playwright/.auth/` is present (it is) and keep it.
- [.dockerignore](.dockerignore): no updates required; Playwright and test outputs are excluded.
- [Dockerfile](Dockerfile): no updates required for this QA-focused task.

---

## 5. Implementation Plan

### Phase 1: QA Baseline and Threshold Decision (Least Requests)

1. **Run baseline type-check**
    - Execute `npm run type-check` in [frontend/package.json](frontend/package.json) to capture actual errors.
    - Record exact failing files and error messages; use these as the source of truth (not the report).

2. **Capture current coverage totals**
    - Run `npm run test:coverage` to confirm statement totals and verify failing files.
    - Cross-check with [scripts/frontend-test-coverage.sh](scripts/frontend-test-coverage.sh) output.

3. **Decide coverage alignment approach**
    - Choose between “single threshold everywhere” vs “documented split.”
    - Record decision in this plan and align tooling accordingly.

4. **Confirm E2E-first execution**
  - Use the skill runner for E2E coverage and validate the required E2E-first order.
  - Capture rationale: UI/UX breakage invalidates downstream unit coverage signals.

### Phase 2: Type-Check Fixes (Test Mocks and Imports)

1. **Repair mocks against current types**
    - Update test mocks to match [frontend/src/api/accessLists.ts](frontend/src/api/accessLists.ts), [frontend/src/api/dnsProviders.ts](frontend/src/api/dnsProviders.ts), and [frontend/src/api/crowdsec.ts](frontend/src/api/crowdsec.ts).
    - If backend payloads include fields missing from frontend types (e.g., `use_multi_credentials`, `credential_count`), decide whether to extend the frontend types or remove those fields from test mocks.

2. **Remove unused symbols**
    - Remove unused imports and variables flagged by `tsc` (e.g., `fireEvent`, `within`, or unused mocks).

3. **Confirm clean type-check**
    - Re-run `npm run type-check` until zero errors remain.

### Phase 3: Coverage Restoration and Threshold Alignment

**PIVOT: Focus on Form Component Branch Coverage (High ROI)**

Current branch coverage is **79.89%**, requiring **+7.61 percentage points** to reach **87.5%** threshold. Form components have the lowest branch coverage and represent the highest ROI for closing this gap.

1. **Expand AccessListForm.tsx coverage** (`frontend/src/components/__tests__/AccessListForm.test.tsx`)
    - Current branch coverage: 76.28%
    - Add tests for:
      - Form submission with invalid data (validation error paths).
      - Conditional rendering of optional fields (role selection, description toggles).
      - Error state handling and recovery.
      - Edit vs create modes (branching logic).

2. **Expand CredentialManager.tsx coverage** (`frontend/src/components/__tests__/CredentialManager.test.tsx`)
    - Current branch coverage: 64.04% (lowest)
    - Add tests for:
      - Credential selection/deselection logic (branching).
      - Add/edit/delete credential modal flows.
      - Form field validation conditional branches.
      - Error state rendering and user feedback paths.

3. **Expand FileUploadSection.tsx coverage** (`frontend/src/components/__tests__/FileUploadSection.test.tsx`)
    - Current branch coverage: 70.58%
    - Add tests for:
      - File type validation branches (accept/reject logic).
      - Drag-and-drop vs click upload paths.
      - Error handling (file too large, unsupported type).
      - Progress and completion state branches.

4. **Expand ProxyHostForm.tsx coverage** (`frontend/src/components/__tests__/ProxyHostForm.test.tsx`)
    - Current branch coverage: 74.84%
    - Add tests for:
      - Conditional field rendering based on proxy type selection.
      - Form submission with various configurations.
      - Validation error paths (required fields, format validation).
      - Edit vs create mode branching.

5. **Address locale coverage**
    - Decide on exclusion vs minimal test:
      - **Exclude**: add `src/locales/**` to `coverage.exclude` in [frontend/vitest.config.ts](frontend/vitest.config.ts).
      - **Test**: add a small test that imports and validates the locale JSON structure.
    - If exclusion is chosen, add a matching ignore entry to [codecov.yml](codecov.yml) to keep local and Codecov coverage aligned.

6. **Align thresholds**
    - Update [frontend/vitest.config.ts](frontend/vitest.config.ts) and [scripts/frontend-test-coverage.sh](scripts/frontend-test-coverage.sh) per the decision in Phase 1.
    - Ensure [codecov.yml](codecov.yml) enforces 100% patch coverage.

7. **Coverage execution path**
    - Use the coverage skill runner as the default path for E2E coverage and document the legacy fallback.

### Phase 4: Integration and Verification

1. **Run coverage + type-check**
    - `npm run test:coverage`
    - `npm run type-check`

2. **CI parity check**
    - Validate that [quality-checks.yml](.github/workflows/quality-checks.yml) and [codecov-upload.yml](.github/workflows/codecov-upload.yml) reflect the same coverage threshold logic.

3. **Document outcome in QA report**
    - Update [docs/reports/qa_report.md](docs/reports/qa_report.md) with new status and metrics after verification.

---

## 6. Acceptance Criteria

- **Branch coverage** reaches 87.5%+ (up from current 79.89%) through form component test expansion.
- **Overall frontend coverage** meets or exceeds 87.5% threshold across all metrics.
- Form components (AccessListForm, CredentialManager, FileUploadSection, ProxyHostForm) achieve branch coverage reflective of test expansion scope.
- `npm run type-check` completes with zero errors and zero unused symbol violations.
- Codecov patch coverage policy is mandatory at 100% for modified lines.
- QA report reflects the new status and explicitly notes any intentionally excluded paths (locales).
- E2E tests run first and pass before unit/integration coverage is collected.

---

## 7. Risks and Mitigations

- **Risk**: Branch coverage tests require deep understanding of form validation and state management logic.
  - **Mitigation**: Analyze existing form test patterns and prioritize high-branch-count first (CredentialManager at 64% will yield largest gains).
- **Risk**: Raising patch coverage in Codecov could fail legacy PRs.
  - **Mitigation**: Gate the change to the remediation branch and notify reviewers in PR summary.

---

## 8. Confidence Score

**Confidence: 85%**

Rationale: The form components are known, their branch coverage gaps are quantified (79.89% → 87.5%+), and branch coverage is directly testable through input validation paths, conditional rendering, and error state handling. The scope is well-defined and high-ROI.

# QA Audit Report — Hecate Feature (PR #983)

**Branch**: `feature/hecate`
**Date**: 2026-05-03
**Auditor**: QA Security Agent
**Spec**: `docs/plans/current_spec.md`

---

## Executive Summary

| Step | Check | Status | Notes |
|------|-------|--------|-------|
| 0 | Backend Unit Tests & Coverage | ✅ PASS | 88.0% (threshold 87%) |
| 1 | Frontend Unit Tests & Coverage | ✅ PASS | 89.8% (threshold 87%) |
| 1.5 | Local Patch Coverage Report | ⚠️ WARN | 87.3% overall (below 90% target) |
| 2 | TypeScript Type Check | ✅ PASS | Exit 0, no errors |
| 3 | Lefthook Pre-commit Hooks | ✅ PASS | 8/8 checks pass, 0 errors |
| 4 | Go Vet | ✅ PASS | No issues |
| 5 | GORM Security Scan | ✅ PASS | 0 CRITICAL, 0 HIGH |
| 6 | Trivy FS Vulnerability Scan | ✅ PASS | 0 CRITICAL, 0 HIGH |
| 7 | Acceptance Criteria Verification | ✅ PASS | All §6 criteria satisfied |
| 8 | i18n Key Audit | ⚠️ WARN | 2 keys missing (fallbacks present) |
| 9 | E2E Tests (new feature paths) | ⚠️ WARN | No new specs for Problems 1-3 |

**Overall Verdict**: ✅ **APPROVED FOR MERGE** (warnings are non-blocking)

---

## Step 0 — Backend Unit Tests & Coverage

**Result**: ✅ PASS

- Coverage artifact: `backend/coverage.txt` (generated 2026-05-03T15:53)
- **Total statement coverage**: 88.0% (16,935 / 19,251 statements)
- Threshold: 87% — **threshold met**

A failure for `TestSendExternal_AllEventTypes/domain` appeared in an older artifact (`coverage_results.txt` from 2026-04-15). On re-run, this test passes cleanly — confirmed transient/flaky, not a regression from this branch.

---

## Step 1 — Frontend Unit Tests & Coverage

**Result**: ✅ PASS

- Coverage artifact: `frontend/coverage/lcov.info`
- **Total line coverage**: 89.8% (5,743 / 6,396 lines)
- Threshold: 87% — **threshold met**

Key test files for the Hecate feature:
- `src/components/__tests__/RemoteServerForm.test.tsx` — 9 tests, agent mode covered; provider mode partially covered (ProviderDevicePicker mocked)
- `src/api/orthrus.ts` — covered via integration with hook tests

---

## Step 1.5 — Local Patch Coverage Report

**Result**: ⚠️ WARN — 87.3% overall (target: 90%)

Both individual scopes pass their 85% threshold:

| Scope | Changed Lines | Covered | Patch Coverage | Status |
|-------|-------------|---------|----------------|--------|
| Overall | 2,788 | 2,434 | 87.3% | ⚠️ WARN |
| Backend | 2,190 | 1,917 | 87.5% | ✅ PASS |
| Frontend | 598 | 517 | 86.5% | ✅ PASS |

### Files with Low Patch Coverage (Hecate-related)

| File | Patch Coverage | Uncovered Changed Lines |
|------|----------------|------------------------|
| `frontend/src/components/RemoteServerForm.tsx` | 47.6% | 33 lines — provider mode submit path, direct mode edge cases |
| `frontend/src/pages/Hecate.tsx` | 66.7% | 32 lines — error branches, edge state transitions |
| `backend/internal/services/orthrus_service.go` | 77.7% | 35 lines — `Patch()` method body untested at service level |
| `backend/internal/api/handlers/orthrus_handler.go` | 94.3% | 5 lines — 404 path (lines 91-93, 99-100) |
| `frontend/src/api/orthrus.ts` | 90.0% | 2 lines — error handling branch |

**Root cause**: The `Patch()` service method has no dedicated service-level unit tests; coverage comes from handler tests via mock. The `RemoteServerForm.tsx` provider mode submit path (Problem 3, provider branch) is not exercised in the current test suite.

**Recommendation**: Add tests for:
1. `OrthrusService.Patch()` with all four field combinations (service-level)
2. `RemoteServerForm` submit with `connection_mode = 'provider'`

---

## Step 2 — TypeScript Type Check

**Result**: ✅ PASS

```
TSC_EXIT:0
```

No TypeScript errors. The new types (`ConnectionMode`, `PatchAgentRequest`, `ProviderDevicePicker` props) all compile cleanly.

---

## Step 3 — Lefthook Pre-commit Hooks

**Result**: ✅ PASS (all 8 hooks)

| Hook | Result | Time |
|------|--------|------|
| trailing-whitespace | ✅ PASS | 0.02s |
| end-of-file-fixer | ✅ PASS | 0.04s |
| check-lfs-large-files | ✅ PASS | 0.05s |
| block-codeql-db | ✅ PASS | 0.05s |
| block-data-backups | ✅ PASS | 0.03s |
| semgrep | ✅ PASS | 46.22s |
| frontend-type-check | ✅ PASS | 53.94s |
| frontend-lint (ESLint) | ✅ PASS | 76.40s |

ESLint: 992 warnings, 0 errors. All warnings are pre-existing patterns (`testing-library/no-node-access`, `unicorn/no-useless-undefined`, `security/detect-non-literal-regexp`) present on `main`; none introduced by this PR.

---

## Step 4 — Go Vet

**Result**: ✅ PASS

```
go vet ./backend/... → no output (clean)
```

---

## Step 5 — GORM Security Scan

**Result**: ✅ PASS

```
./scripts/scan-gorm-security.sh --check → EXIT:0
```

| Severity | Count |
|----------|-------|
| CRITICAL | 0 |
| HIGH | 0 |
| MEDIUM | 0 |
| INFO | 2 (pre-existing: missing indexes on `UserPermittedHost` FKs — unrelated to Hecate) |

The three new fields on `OrthrusAgent` (`hecate_tunnel_uuid`, `device_id`, `resolved_address`) use `json:"...,omitempty"` tagging and do not expose sensitive data.

---

## Step 6 — Trivy FS Vulnerability Scan

**Result**: ✅ PASS

```
trivy fs . --exit-code 1 --severity CRITICAL,HIGH --scanners vuln → EXIT:0
```

No CRITICAL or HIGH vulnerabilities in Go modules or Node.js dependencies.

---

## Step 7 — Acceptance Criteria Verification (spec §6)

All criteria verified via manual code inspection (grep + cat of implementation files):

### Problem 1 — HecateProviders Inline Tunnel List + Edit

| Criterion | Status | Evidence |
|-----------|--------|---------|
| Each provider card shows list of its tunnels inline | ✅ PASS | `HecateProviders.tsx` — `openTunnels` state, inline `<HecateTunnelForm>` per card |
| Tunnel count badge renders `tunnelCount_one`/`tunnelCount_other` | ✅ PASS | `HecateProviders.tsx:59-60` — `t('hecate.providers.tunnelCount_one', { defaultValue })` |
| Edit button opens pre-filled `HecateTunnelForm` in edit mode | ✅ PASS | `editTunnel` state, `openEdit()` handler, second `HecateTunnelForm` instance |
| Form has access to provider context for pre-population | ✅ PASS | Provider passed as prop to edit form instance |

### Problem 2 — Agent Generic Provider Assignment

| Criterion | Status | Evidence |
|-----------|--------|---------|
| `PATCH /orthrus/agents/:uuid` route registered | ✅ PASS | `routes.go` — `rg.PATCH("/orthrus/agents/:uuid", h.Patch)` |
| Handler returns 404 on unknown UUID | ✅ PASS | `orthrus_handler.go` — `errors.Is(err, gorm.ErrRecordNotFound)` → 404 |
| All PATCH fields optional (partial update) | ✅ PASS | `patchAgentRequest` — all `*string` (pointer) fields |
| `OrthrusService.Patch()` only writes non-nil fields | ✅ PASS | Map-based `Updates()` — only set keys written |
| `Rename()` is backward-compat wrapper around `Patch()` | ✅ PASS | `orthrus_service.go:112-114` |
| `OrthrusAgent` model has 3 new fields | ✅ PASS | `hecate_tunnel_uuid`, `device_id`, `resolved_address` with `omitempty` |
| Frontend `patchAgent()` API function exists | ✅ PASS | `frontend/src/api/orthrus.ts:70` |
| Frontend `renameAgent()` delegates to `patchAgent()` | ✅ PASS | `orthrus.ts:67-68` |
| `usePatchAgent` hook exists | ✅ PASS | `useOrthrus.ts:44` |
| `OrthrusAgentManager` has "Assign Provider" button | ✅ PASS | `Link2` icon, `assignProviderAgent` state, `AssignProviderDialog` |
| Provider column shows `resolved_address` or fallback | ✅ PASS | `OrthrusAgentManager.tsx` — provider column renders `resolved_address` |

### Problem 3 — RemoteServers 3-Radio Connection Model

| Criterion | Status | Evidence |
|-----------|--------|---------|
| `ConnectionMode` = `'direct' \| 'agent' \| 'provider'` | ✅ PASS | `ConnectionTypeSelector.tsx:1` |
| 3 radio buttons rendered | ✅ PASS | direct / agent / provider radios |
| Agent picker shown only in agent mode | ✅ PASS | Conditional render in `ConnectionTypeSelector` |
| No-provider warning shown when agent has no `resolved_address` | ✅ PASS | `ConnectionTypeSelector.tsx:124` |
| `ProviderDevicePicker.tsx` component exists | ✅ PASS | `/frontend/src/components/hecate/ProviderDevicePicker.tsx` |
| `RemoteServerForm` uses simplified `resolveConnectionMode()` | ✅ PASS | 3-branch logic: direct / agent / provider |
| `connection_mode` in form state replaces `orthrus_ip_mode` | ✅ PASS | Verified in `RemoteServerForm.tsx` |

---

## Step 8 — i18n Key Audit

**Result**: ⚠️ WARN — 2 keys missing from `frontend/src/locales/en/translation.json`

| Key | Status | Impact |
|-----|--------|--------|
| `hecate.providers.tunnelCount_one` | ❌ MISSING | `HecateProviders.tsx:59` uses `defaultValue: '{{count}} tunnel'` — renders correctly |
| `hecate.providers.tunnelCount_other` | ❌ MISSING | `HecateProviders.tsx:60` uses `defaultValue: '{{count}} tunnels'` — renders correctly |
| `hecate.form.mode.provider` | ✅ PRESENT | `= 'Provider'` |
| `hecate.form.mode.providerDescription` | ✅ PRESENT | Route via configured network provider |
| `hecate.form.mode.agent.noProviderWarning` | ✅ PRESENT | Warning text |
| `hecate.form.mode.agent.noProviderLink` | ✅ PRESENT | Link text |
| `hecate.agentManager.noProviderAssigned` | ✅ PRESENT | Fallback label |

**Impact**: Functional with `defaultValue` fallbacks. No visual regression. All i18n-dependent tests pass.

**Recommendation**: Add the two missing keys before or shortly after merge to avoid confusion for future translators. Suggested fix:

```json
// In frontend/src/locales/en/translation.json, under hecate.providers:
"tunnelCount_one": "{{count}} tunnel",
"tunnelCount_other": "{{count}} tunnels"
```

---

## Step 9 — E2E Test Coverage

**Result**: ⚠️ WARN — No new spec files for the three new feature areas

Existing E2E specs in `tests/`:
- `hecate-tunnel-manager.spec.ts` — pre-existing Hecate tunnel management
- `orthrus-agent-install.spec.ts` — pre-existing agent install flow
- `dns-provider-crud.spec.ts`, `dns-provider-types.spec.ts` — DNS provider flows

Missing (no dedicated specs for new feature areas):
- Problem 1: HecateProviders inline tunnel list + edit mode
- Problem 2: Agent "Assign Provider" dialog (`OrthrusAgentManager`)
- Problem 3: RemoteServerForm 3-radio model, especially provider mode submit

**Recommendation**: Create targeted E2E specs in a follow-up PR. Priority order:
1. `tests/remote-server-3modes.spec.ts` — verify all 3 connection modes can be selected and submitted
2. `tests/hecate-providers-inline-edit.spec.ts` — verify edit modal opens pre-filled
3. `tests/hecate-agent-provider.spec.ts` — verify Assign Provider dialog flow

---

## Security Findings Summary

| Category | Severity | Finding | Status |
|----------|----------|---------|--------|
| GORM model | INFO | Missing DB index on `UserPermittedHost` FKs | Pre-existing, not introduced by this PR |
| Dependency (Go) | — | No CRITICAL/HIGH CVEs | ✅ Clean |
| Dependency (Node) | — | No CRITICAL/HIGH CVEs | ✅ Clean |
| Input validation | — | `patchAgent` request validated via pointer optionality; blank name rejected | ✅ |
| Data exposure | — | New `OrthrusAgent` fields use `omitempty`; no token/secret fields added | ✅ |

---

## Coverage: Key Changed Files

### Backend (from patch report)

| File | Patch Coverage | Priority |
|------|----------------|----------|
| `backend/internal/api/handlers/orthrus_handler.go` | 94.3% | Low — 5 lines |
| `backend/internal/services/orthrus_service.go` | 77.7% | **Medium** — `Patch()` body untested at service level |
| `backend/internal/api/routes/routes.go` | 82.1% | Low — route registration branches |
| `backend/internal/hecate/manager.go` | 84.9% | Low — error handling paths |

### Frontend (from patch report)

| File | Patch Coverage | Priority |
|------|----------------|----------|
| `frontend/src/components/RemoteServerForm.tsx` | 47.6% | **Medium** — provider mode submit path |
| `frontend/src/pages/Hecate.tsx` | 66.7% | **Medium** — error state branches |
| `frontend/src/api/orthrus.ts` | 90.0% | Low — 2 error lines |

---

## Pre-existing Issues (Not Introduced by This PR)

- ESLint `testing-library/no-node-access` warnings in test files — pre-existing
- ESLint `security/detect-unsafe-regex` in `utils/validation.ts` — pre-existing
- `UserPermittedHost` missing FK index — pre-existing (GORM INFO)
- `go tool cover` failure on corrupted coverage.txt path — transient, resolved

---

## Final Verdict

```
✅ APPROVED FOR MERGE

Blocking issues: NONE
Warnings (non-blocking):
  - WARN: Patch coverage 87.3% overall (below 90% target; both backend/frontend scopes pass 85% minimum)
  - WARN: i18n keys tunnelCount_one/tunnelCount_other missing (defaultValue fallbacks prevent regression)
  - WARN: No new E2E specs for Problems 1-3 (existing coverage via unit tests; follow-up recommended)

Recommended follow-up items (post-merge):
  1. Add OrthrusService.Patch() unit tests at the service layer
  2. Add RemoteServerForm provider-mode submit test
  3. Add i18n keys hecate.providers.tunnelCount_one/other to translation.json
  4. Create E2E specs for the three new feature areas
```

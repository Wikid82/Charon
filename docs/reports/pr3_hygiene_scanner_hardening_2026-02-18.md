# PR-3 Hygiene and Scanner Hardening Evidence

Date: 2026-02-18
Scope: Config-only hardening per `docs/plans/current_spec.md` (PR-3)

## Constraints honored
- No production backend/frontend runtime behavior changes.
- Test fixture runtime code changes were made for insecure-temp remediation and covered by targeted validation.
- No full local Playwright E2E run (deferred to CI as requested).
- Edits limited to PR-3 hygiene targets.

## Changes made

### 1) Ignore pattern normalization and deduplication

#### `.gitignore`
- Reviewed for PR-3 hygiene scope; no additional net changes were needed in this pass.

#### `.dockerignore`
- Replaced legacy `.codecov.yml` entry with canonical `codecov.yml`.
- Removed redundant CodeQL SARIF patterns (`codeql-*.sarif`, `codeql-results*.sarif`) because `*.sarif` already covers them.

### 2) Canonical Codecov config path
- Chosen canonical Codecov config: `codecov.yml`.
- Removed duplicate/conflicting config file: `.codecov.yml`.

### 3) Canonical scanner outputs
- Verified existing task/script configuration already canonical and unchanged:
  - Go: `codeql-results-go.sarif`
  - JS/TS: `codeql-results-js.sarif`
- No further task/hook edits required.

### 4) PR718 freshness gate remediation (PR-3 blocker)
- Restored required baseline artifact: [docs/reports/pr718_open_alerts_baseline.json](pr718_open_alerts_baseline.json).
- Re-ran freshness gate command: `bash scripts/pr718-freshness-gate.sh`.
- Successful freshness artifacts:
   - [docs/reports/pr718_open_alerts_freshness_20260218T163528Z.json](pr718_open_alerts_freshness_20260218T163528Z.json)
   - [docs/reports/pr718_open_alerts_freshness_20260218T163528Z.md](pr718_open_alerts_freshness_20260218T163528Z.md)
- Pass statement: freshness gate now reports baseline status `present` with drift status `no_drift`.

## Focused validation

### Commands run
1. `bash scripts/ci/check-codeql-parity.sh`
   - Result: **PASS**
2. `pre-commit run check-yaml --files codecov.yml`
   - Result: **PASS**
3. `pre-commit run --files .dockerignore codecov.yml docs/reports/pr3_hygiene_scanner_hardening_2026-02-18.md`
   - Result: **PASS**
4. `pre-commit run trailing-whitespace --files docs/reports/pr3_hygiene_scanner_hardening_2026-02-18.md`
   - Result: **AUTO-FIXED on first run, PASS on re-run**

### Conditional checks (not applicable)
- `actionlint`: not run (no workflow files were edited).
- `shellcheck`: not run (no shell scripts were edited).

## Risk and open items
- Residual risk is low: all changes are ignore/config hygiene only.
- Historical docs may still reference `.codecov.yml`; this does not affect runtime or CI behavior but can be cleaned in a documentation-only follow-up.
- Full E2E remains deferred to CI per explicit request.

## Closure Note
- Status: **Closed (Phase 4 / PR-3 hygiene scope complete)**.
- Scope outcome: canonical Codecov path selected, ignore-pattern cleanup completed, and scanner-output conventions confirmed.
- Blocker outcome: PR718 freshness gate restored and passing with `no_drift`.
- Validation outcome: parity and pre-commit checks passed for touched config/docs files.

## Security Remediation Delta (PR-3 Addendum)

Finding scope:
- Rule: `js/insecure-temporary-file`
- File: `tests/fixtures/auth-fixtures.ts`
- Context: token cache implementation for `refreshTokenIfNeeded`

Remediation completed:
- Removed filesystem token-cache/lock behavior (`tmpdir`, `token.json`, `token.lock`, `mkdtemp`).
- Replaced with in-memory token cache and async serialization to prevent concurrent refresh storms within process.
- Preserved fixture/API behavior contract for `refreshTokenIfNeeded` and existing token-refresh fixture usage.

Verification evidence (targeted only):
- Playwright fixture validation:
   - `npx playwright test tests/fixtures/token-refresh-validation.spec.ts --project=firefox`
   - Result: **PASS** (`5 passed`)
- Static pattern verification:
   - `rg "tmpdir\(|token\.lock|token\.json|mkdtemp|charon-test-token-cache-" tests/fixtures/auth-fixtures.ts`
   - Result: **No matches**
- Lint applicability check for touched files:
   - `npx eslint tests/fixtures/auth-fixtures.ts tests/fixtures/token-refresh-validation.spec.ts`
   - Result: files not covered by current ESLint config (no lint errors reported for these files)

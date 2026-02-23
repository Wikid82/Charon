# E2E Skip Registry (2026-02-13)

## Objective

Determine why tests are skipped and classify each skip source as one of:

- Wrong environment/configuration
- Product bug
- Missing feature/test preconditions
- Intentional test routing (non-bug)

## Evidence Sources

1. Full rerun baseline (previous run): `1500 passed / 62 failed / 50 skipped`
2. Targeted runtime census (Chromium):

```bash
set -a && source .env && set +a && \
PLAYWRIGHT_COVERAGE=0 PLAYWRIGHT_HTML_OPEN=never \
npx playwright test tests/manual-dns-provider.spec.ts tests/core/admin-onboarding.spec.ts \
  --project=chromium --reporter=json > /tmp/skip-census-targeted.json 2>&1
```

3. Static skip directive census in tests:

```bash
grep -RInE "test\\.skip|describe\\.skip|test\\.fixme|describe\\.fixme" tests/
```

4. Project routing behavior from `playwright.config.js`.

## Confirmed Skip Sources

### 1) Manual DNS provider suite skips (Confirmed)

- File: `tests/manual-dns-provider.spec.ts`
- Runtime evidence (Chromium targeted run): `16 skipped`
- Skip type: explicit `test.describe.skip(...)` and `test.skip(...)`
- Classification: **Missing feature/test preconditions (technical debt skip)**
- Why:
  - Tests require deterministic DNS challenge records and UI states that are not guaranteed in default E2E flow.
  - One skip reason is explicitly tied to absent visible challenge records (`No copy buttons found - requires DNS challenge records to be visible`).
- Owner: **Playwright Dev + Frontend Dev**
- Priority: **P0 for critical-path coverage, P1 for full suite parity**
- Recommended action:
  - Create deterministic fixtures/seed path for manual DNS challenge state.
  - Re-enable blocks incrementally and validate across all three browser projects.

### 2) Conditional Cerberus skip in admin onboarding (Confirmed source, condition-dependent runtime)

- File: `tests/core/admin-onboarding.spec.ts`
- Skip directive: `test.skip(true, 'Cerberus must be enabled to access emergency token generation UI')`
- Classification: **Wrong environment/configuration (when triggered)**
- Why:
  - This is a hard environment gate. If Cerberus is disabled or inaccessible, test intentionally skips.
- Owner: **QA + Backend Dev**
- Priority: **P1**
- Recommended action:
  - Split tests into:
    - Cerberus-required suite (explicit env contract), and
    - baseline onboarding suite (no Cerberus dependency).
  - Add preflight assertion that reports config mismatch clearly instead of silent skip where possible.

### 3) Security project routing behavior (Intentional, non-bug)

- Source: `playwright.config.js`
- Behavior:
  - Browser projects (`chromium`, `firefox`, `webkit`) use `testIgnore` for `**/security-enforcement/**` and `**/security/**`.
  - Security coverage is handled by dedicated `security-tests` project.
- Classification: **Intentional test routing (non-bug)**
- Why:
  - Prevents security suite execution duplication in standard browser projects.
- Owner: **QA**
- Priority: **P2 (documentation only)**
- Recommended action:
  - Keep as-is; ensure CI includes explicit `security-tests` project execution in required checks.

## Current Assessment

Based on available runtime and source evidence, most observed skips are currently **intentional skip directives in manual DNS provider tests** rather than emergent engine bugs.

### Distribution (current confirmed)

- **Missing feature/preconditions debt:** High (manual DNS blocks)
- **Environment-gated skips:** Present (Cerberus-gated onboarding path)
- **Product bug-derived skips:** Not yet confirmed from current skip evidence
- **Config/routing-intentional non-runs:** Present and expected (security project separation)

## Actions to Close Phase 8.1

1. Export full multi-project JSON report and enumerate all `status=skipped` tests with file/title/annotations.
2. Map every skipped test to one of the four classes above.
3. Open remediation tasks for all technical-debt skips (manual DNS first).
4. Define explicit re-enable criteria and target command per skip cluster.

## Re-enable Queue (Initial)

1. `tests/manual-dns-provider.spec.ts` skipped blocks
   - Unblock by deterministic challenge fixture + stable locators
   - Re-enable command:

   ```bash
   npx playwright test tests/manual-dns-provider.spec.ts --project=chromium --project=firefox --project=webkit
   ```

2. Cerberus-gated onboarding checks
   - Unblock by environment contract enforcement or test split
   - Re-enable command:

   ```bash
   npx playwright test tests/core/admin-onboarding.spec.ts --project=chromium --project=firefox --project=webkit
   ```

## Exit Criteria for This Registry

- [x] Confirmed dominant skip source with runtime evidence
- [x] Classified skips into environment vs missing feature/test debt vs routing-intentional
- [ ] Full-suite skip list fully enumerated from JSON (all 50)
- [ ] Owner + ETA assigned per skipped test block

## Post-Edit Validation Status (Phase 3 + relevant Phase 4)

### Applied changes

- `tests/manual-dns-provider.spec.ts`
  - Removed targeted `describe.skip` / `test.skip` usage so suites execute.
  - Added deterministic preconditions using existing DNS fixtures (`mockManualChallenge`, `mockExpiredChallenge`, `mockVerifiedChallenge`).
  - Added test-scoped route mocks with cleanup parity (`page.route` + `page.unroute`).
- `tests/core/admin-onboarding.spec.ts`
  - Removed Cerberus-dependent `Emergency token can be generated` from browser-safe core onboarding suite.
- `tests/security/security-dashboard.spec.ts`
  - Added `Emergency token can be generated` under security suite ownership.
  - Added `security-state-pre` / `security-state-post` annotations and pre/post state drift checks.

### Concrete command results

1. **Pass 1**

```bash
npx playwright test tests/manual-dns-provider.spec.ts tests/core/admin-onboarding.spec.ts \
  --project=chromium --project=firefox --project=webkit \
  --grep "Provider Selection Flow|Manual Challenge UI Display|Copy to Clipboard|Verify Button Interactions|Accessibility Checks|Admin Onboarding & Setup" \
  --grep-invert "Emergency token can be generated" --reporter=json
```

- Parsed stats: `expected=43`, `unexpected=30`, `skipped=0`
- Intent-scoped skip census (`chromium|firefox|webkit` + targeted files): **0 skipped / 0 did-not-run**
- `skip-reason` annotations in this run: **0**

2. **Pass 2**

```bash
npx playwright test tests/manual-dns-provider.spec.ts \
  --project=chromium --project=firefox --project=webkit \
  --grep "Manual DNS Challenge Component Tests|Manual DNS Provider Error Handling" --reporter=json
```

- Parsed stats: `expected=1`, `unexpected=15`, `skipped=0`
- Intent-scoped skip census (`chromium|firefox|webkit` + manual DNS file): **0 skipped / 0 did-not-run**
- `skip-reason` annotations in this run: **0**

3. **Security-suite ownership + anti-duplication**

```bash
npx playwright test tests/security/security-dashboard.spec.ts \
  --project=security-tests --grep "Emergency token can be generated" --reporter=json
```

- Parsed stats: `unexpected=0`, `skipped=0`
- Raw JSON evidence confirms `projectName: security-tests` for emergency token test execution.
- `security-state-pre` and `security-state-post` annotations captured.
- Anti-duplication check:
  - `CORE_COUNT=0` in `tests/core/admin-onboarding.spec.ts`
  - `SEC_COUNT=1` across `tests/security/**` + `tests/security-enforcement/**`

4. **Route mock cleanup parity**

- `tests/manual-dns-provider.spec.ts`: `ROUTES=3`, `UNROUTES=3`.

### Residual failures (for Phase 7)

- Skip debt objective for targeted scopes is met (`skipped=0` and `did-not-run=0` in intended combinations).
- Remaining failures are assertion/behavior failures in manual DNS and onboarding flows and should proceed to Phase 7 remediation.

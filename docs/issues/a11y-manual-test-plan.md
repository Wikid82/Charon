---
title: "Manual Test Plan - Issue #929: Accessibility Testing Implementation"
status: Open
priority: High
assignee: QA
labels: testing, frontend, accessibility
---

# Test Objective

Verify that the `@axe-core/playwright` accessibility testing infrastructure introduced in Issue #929 is correct, complete, and reliable. This plan targets implementation bugs — stale baselines, missing CI coverage, auth fixture gaps, and browser-specific failures — rather than re-testing the UI accessibility itself.

# Scope

- In scope: `tests/a11y/`, `tests/fixtures/a11y.ts`, `tests/a11y/a11y-baseline.ts`, `tests/utils/a11y-helpers.ts`, CI shard configuration in `.github/workflows/e2e-tests-split.yml`
- Out of scope: Remediating the baselined violations themselves (tracked separately in Issue #929), security enforcement tests

# Prerequisites

- Local E2E container running (`Docker: Rebuild E2E Environment` task or `skill-runner.sh docker-rebuild-e2e`)
- Node dependencies installed (`npm ci`)
- Playwright browsers installed (`npx playwright install --with-deps chromium firefox webkit`)
- A second terminal available for Docker service manipulation (scenario 7)

---

# Manual Scenarios

## 1. Baseline Violations — Verify violations are still present in the UI

**Purpose**: Confirm that every rule ID in `tests/a11y/a11y-baseline.ts` represents a real, currently reproducible violation. If a violation has been fixed upstream but the baseline still suppresses it, tests will silently miss regressions.

### Steps

- [ ] Open `tests/a11y/a11y-baseline.ts` and list all five baseline entries:
  - `color-contrast` → pages: `['/']`
  - `label` → pages: `['/settings/users', '/security', '/tasks/backups', '/tasks/import/caddyfile', '/tasks/import/crowdsec']`
  - `button-name` → pages: `['/settings', '/security/headers']`
  - `select-name` → pages: `['/tasks/logs']`
  - `scrollable-region-focusable` → pages: `['/tasks/logs']`
- [ ] For each entry, navigate to one of the listed pages while logged in
- [ ] Open browser DevTools console and run:
  ```javascript
  const { run } = await import('https://cdn.jsdelivr.net/npm/axe-core/axe.min.js');
  const r = await axe.run({ runOnly: { type: 'rule', values: ['<rule-id>'] } });
  console.log(r.violations);
  ```
  *(Replace `<rule-id>` with the rule from the baseline)*
- [ ] **Expected**: At least one violation is reported for each baselined rule ID on its listed page

### Acceptance Criteria

- [ ] All five `ruleId` entries produce at least one axe violation on their listed pages
- [ ] If a rule produces zero violations on all its listed pages, remove it from the baseline and open a follow-up to update the test

---

## 2. CI Shard Coverage — Verify `tests/a11y/` is included in the non-security shards

**Purpose**: Confirm that accessibility tests are actually executed in CI and not silently omitted from all shard runner commands.

### Steps

- [ ] Open `.github/workflows/e2e-tests-split.yml`
- [ ] Search for `tests/a11y` (three occurrences expected: one per browser — Chromium, Firefox, WebKit non-security shards)
- [ ] For each occurrence, verify the surrounding context is a non-security shard `npx playwright test` command (not the security suite)
- [ ] Confirm the path appears in the shard-1 command of at least one browser, meaning the a11y tests run on the first shard rather than being deferred

### Acceptance Criteria

- [ ] `tests/a11y` appears in at least 3 `npx playwright test` commands in the workflow (one per browser family)
- [ ] The occurrences are in non-security job steps (job names contain "Non-Security" or equivalent)
- [ ] No occurrence is inside a `tests/security` or `tests/security-enforcement` block

---

## 3. Authentication Fixtures — Verify axe tests receive a valid logged-in session

**Purpose**: The a11y fixture in `tests/fixtures/a11y.ts` extends `auth-fixtures`, which should provide a pre-authenticated `storageState`. Tests that require auth (all pages except login) will silently redirect to `/login` and scan the wrong page if the fixture is broken.

### Steps

- [ ] Run a single authenticated a11y spec locally and capture the network trace:
  ```bash
  cd /projects/Charon
  npx playwright test tests/a11y/dashboard.a11y.spec.ts --project=firefox --trace=on
  ```
- [ ] Open the resulting trace (`npx playwright show-trace test-results/.../trace.zip`)
- [ ] In the network tab, confirm the request to `/` receives a `200` (not a `302` redirect to `/login`)
- [ ] Confirm at least one request to `/api/v1/auth/me` or equivalent returns `200` before `axe.run()` executes
- [ ] Run the unauthenticated login spec and confirm it does NOT use stored auth state:
  ```bash
  npx playwright test tests/a11y/login.a11y.spec.ts --project=firefox --trace=on
  ```
- [ ] Verify the login spec sets `storageState: { cookies: [], origins: [] }` (confirmed in source at line 6)
- [ ] In the login spec trace, confirm there is no prior authenticated request; the page navigates directly to `/login`

### Acceptance Criteria

- [ ] Dashboard a11y spec navigates to `/` and receives HTTP 200
- [ ] No redirect to `/login` occurs before `axe.run()` in authenticated specs
- [ ] Login a11y spec runs with an empty storage state (no cookies pre-loaded)

---

## 4. Canvas Exclusion — Verify chart canvases do not produce false positive violations

**Purpose**: The axe fixture excludes `.chart-container canvas` elements (`tests/fixtures/a11y.ts`, line 14). If the exclusion selector is wrong or the chart container class changed, canvas elements will be scanned and produce spurious `canvas` rule violations.

### Steps

- [ ] Run the dashboard a11y spec (CrowdSec dashboard has chart canvases):
  ```bash
  npx playwright test tests/a11y/dashboard.a11y.spec.ts --project=chromium
  ```
- [ ] Run the uptime a11y spec (Uptime monitors also render charts):
  ```bash
  npx playwright test tests/a11y/uptime.a11y.spec.ts --project=chromium
  ```
- [ ] If either test fails, open the attached `a11y-results` JSON artifact and check whether `canvas` appears in `violations[].nodes[].target`
- [ ] If canvas violations are present, inspect the rendered DOM in the browser:
  - Navigate to `http://localhost:8080/` (or `/uptime`) while the container is running
  - In DevTools Elements panel, search for `canvas` elements
  - Verify the parent element has class `chart-container`
  - If the class differs (e.g., `chartContainer`, `chart-wrapper`), update the `exclude` selector in `tests/fixtures/a11y.ts`

### Acceptance Criteria

- [ ] Dashboard and uptime a11y specs pass without `canvas`-related violations
- [ ] If canvas violations appear, the `.chart-container canvas` selector is confirmed or corrected

---

## 5. Baseline Expiry — Confirm all `expiresAt` dates are set to `2026-07-31`

**Purpose**: Every baseline entry carries an `expiresAt` date after which the violation should be remediated. Verify the dates are correct and set a reminder to review before expiry.

### Steps

- [ ] Open `tests/a11y/a11y-baseline.ts`
- [ ] For each entry in `A11Y_BASELINE`, record the `expiresAt` value:
  | Rule ID | Expected `expiresAt` | Actual `expiresAt` |
  |---|---|---|
  | `color-contrast` | `2026-07-31` | |
  | `label` | `2026-07-31` | |
  | `button-name` | `2026-07-31` | |
  | `select-name` | `2026-07-31` | |
  | `scrollable-region-focusable` | `2026-07-31` | |
- [ ] Confirm all five entries have `expiresAt: '2026-07-31'`
- [ ] Create a calendar reminder for **2026-07-17** (two weeks before expiry) with title: "Review a11y baseline — Issue #929 entries expire 2026-07-31"
  - The reminder should prompt: re-run the a11y suite, compare against the baseline, and remove any entries whose violations have been fixed

### Acceptance Criteria

- [ ] All five `expiresAt` values equal `'2026-07-31'` (verified against actual file)
- [ ] Calendar reminder created for 2026-07-17

---

## 6. Browser-Specific Failures — Run a11y specs per-browser and check for timeout issues

**Purpose**: DNS provider and security pages have historically been flaky in Firefox and WebKit due to dynamic content loading. Verify a11y specs complete within their default timeout in each browser individually.

### Steps

- [ ] Run the full a11y suite against each browser, capturing timing:
  ```bash
  # Chromium
  npx playwright test tests/a11y/ --project=chromium --reporter=list 2>&1 | tee /tmp/a11y-chromium.txt

  # Firefox
  npx playwright test tests/a11y/ --project=firefox --reporter=list 2>&1 | tee /tmp/a11y-firefox.txt

  # WebKit
  npx playwright test tests/a11y/ --project=webkit --reporter=list 2>&1 | tee /tmp/a11y-webkit.txt
  ```
- [ ] For each browser output, check for timeout errors (`Test timeout of 30000ms exceeded` or `page.evaluate: Timeout`)
- [ ] Pay particular attention to:
  - `tests/a11y/dns-providers.a11y.spec.ts` — DNS providers page loads provider list asynchronously
  - `tests/a11y/security.a11y.spec.ts` — Security page polls backend for toggle state on load
- [ ] If a timeout is found, open the spec file and check whether a `waitForLoadingComplete` call is present before `makeAxeBuilder().analyze()`
- [ ] If missing, add the wait helper and note the gap in this checklist

### Acceptance Criteria

- [ ] All 12 a11y specs pass in Chromium with no timeouts
- [ ] All 12 a11y specs pass in Firefox with no timeouts (especially DNS providers and security specs)
- [ ] All 12 a11y specs pass in WebKit with no timeouts
- [ ] Any timeout failure is traced to a missing `waitForLoadingComplete` or equivalent wait, and a fix is implemented

---

## 7. Docker Service Error Path — Verify UI handles Docker unavailability gracefully

**Purpose**: Some Charon features (container management, tasks) call the Docker daemon. If Docker is unavailable, the UI should display a user-friendly error message rather than a raw stack trace or a silent blank state.

### Steps

- [ ] Ensure the E2E container is running (`http://localhost:8080` accessible)
- [ ] Log in to Charon as admin
- [ ] In a second terminal, stop the Docker socket passthrough or simulate Docker unavailability:
  ```bash
  # If the container has access to the host Docker socket, rename it temporarily:
  # (Run this on the host, NOT inside the container)
  sudo mv /var/run/docker.sock /var/run/docker.sock.bak
  ```
- [ ] In the browser, navigate to a page that uses Docker (e.g., **Tasks → Backups** or **Tasks → Import**)
- [ ] Observe the UI response
- [ ] Restore Docker socket:
  ```bash
  sudo mv /var/run/docker.sock.bak /var/run/docker.sock
  ```

### Acceptance Criteria

- [ ] The UI displays a readable error message (e.g., "Docker service is unavailable. Please check your configuration.") rather than a raw Go stack trace or unformatted JSON error
- [ ] No unhandled JavaScript exception appears in browser DevTools console during the error state
- [ ] Navigation away from the error page is possible (no keyboard focus trap)
- [ ] After restoring Docker, refreshing the page recovers to the normal state without requiring a container restart

---

# Pass Criteria

All seven scenarios must be checked off before the accessibility testing implementation (Issue #929) is considered fully validated. Failures in scenarios 1, 3, or 4 indicate a correctness bug in the test implementation and must be fixed. Failures in scenarios 6 or 7 indicate UX bugs that should be filed as follow-up issues.

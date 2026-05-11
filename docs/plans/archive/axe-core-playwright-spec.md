# Specification: Integrate @axe-core/playwright for Automated Accessibility Testing

**Issue**: #929
**Status**: Draft
**Created**: 2026-04-20

---

## 1. Introduction

### Overview

Integrate `@axe-core/playwright` into the existing Playwright E2E test suite to provide automated WCAG 2.2 Level AA accessibility scanning across all key application pages. The scans will run as part of CI, failing on critical/serious violations.

### Objectives

1. Install and configure `@axe-core/playwright` as a dev dependency
2. Create a shared accessibility fixture and helper module
3. Add dedicated a11y spec files covering all primary application pages
4. Configure axe rules targeting WCAG 2.2 Level AA conformance
5. Fail CI on critical/serious violations while allowing a baseline for known issues
6. Surface results in Playwright HTML reports across all three browser projects

---

## 2. Research Findings

### 2.1 Existing Playwright Configuration

**File**: [playwright.config.js](../../playwright.config.js)

| Setting | Value |
|---------|-------|
| `testDir` | `./tests` |
| `timeout` | 60s (CI) / 90s (local) |
| `workers` | 1 (CI) / auto (local) |
| `retries` | 2 (CI) / 0 (local) |
| `fullyParallel` | `true` |
| `reporter` | `github` (CI) + `html` + optional coverage |
| `baseURL` | `http://127.0.0.1:8080` (Docker) or `http://localhost:5173` (coverage/Vite) |

**Projects** (6 defined):

| Project | Role | Dependencies |
|---------|------|-------------|
| `setup` | Authentication (auth.setup.ts) | None |
| `security-shard-setup` | Security shard init | `setup` |
| `security-tests` | Security enforcement (Chromium-only, serial) | `setup`, `security-shard-setup` |
| `security-teardown` | Disable security modules | Conditionally active |
| `chromium` | Non-security tests | `setup` (+ `security-tests` when enabled) |
| `firefox` | Non-security tests | `setup` (+ `security-tests` when enabled) |
| `webkit` | Non-security tests | `setup` (+ `security-tests` when enabled) |

**Key patterns**:

- Auth state stored at `playwright/.auth/user.json` via `STORAGE_STATE`
- Coverage via `@bgotink/playwright-coverage` behind `PLAYWRIGHT_COVERAGE=1`
- Global setup in `tests/global-setup.ts` (health check, cleanup)
- All browser projects share `testMatch: /.*\.spec\.(ts|js)$/` with `testIgnore` for security dirs

### 2.2 Existing E2E Test Files (93 spec files)

**Core tests** (`tests/core/`):

| File | Description |
|------|-------------|
| `dashboard.spec.ts` | Dashboard loading, summary cards, quick actions |
| `proxy-hosts.spec.ts` | Proxy host CRUD operations |
| `navigation.spec.ts` | Menu items, sidebar, breadcrumbs, keyboard nav |
| `certificates.spec.ts` | Certificate management |
| `multi-component-workflows.spec.ts` | Cross-feature workflows |
| `data-consistency.spec.ts` | Data integrity checks |
| `authentication.spec.ts` | Login/logout, session management |
| `caddy-import/*.spec.ts` | Caddy config import (5 files) |
| `admin-onboarding.spec.ts` | First-run setup flow |
| `domain-dns-management.spec.ts` | Domain and DNS management |

**Settings tests** (`tests/settings/`): 10 spec files covering account settings, SMTP, notifications (Pushover, Ntfy, Telegram, Email, Slack), user lifecycle, user management.

**Security tests** (`tests/security/`, `tests/security-enforcement/`): 28+ spec files covering CrowdSec, WAF, ACL, rate limiting, audit logs, encryption, RBAC, emergency operations.

**Monitoring tests** (`tests/monitoring/`): `uptime-monitoring.spec.ts`, `create-monitor.spec.ts`.

**Integration tests** (`tests/integration/`): 6 spec files covering import flows, proxy-DNS integration, proxy-certificates, backups.

**Task tests** (`tests/tasks/`): Backups create/restore, Caddyfile import, logs viewing, long-running operations.

**Other root-level tests**: `dns-provider-crud.spec.ts`, `dns-provider-types.spec.ts`, `manual-dns-provider.spec.ts`, `certificate-*.spec.ts`, `crowdsec-whitelist.spec.ts`, `modal-dropdown-triage.spec.ts`.

### 2.3 Test Fixtures and Helpers

| File | Purpose |
|------|---------|
| `tests/fixtures/test.ts` | Base test/expect re-export with conditional coverage instrumentation |
| `tests/fixtures/auth-fixtures.ts` | Extended fixtures: `adminUser`, `regularUser`, `guestUser`, `testData` (TestDataManager) |
| `tests/fixtures/certificates.ts` | Certificate-specific fixtures |
| `tests/fixtures/proxy-hosts.ts` | Proxy host fixtures |
| `tests/fixtures/security.ts` | Security test fixtures |
| `tests/fixtures/settings.ts` | Settings fixtures |
| `tests/fixtures/network.ts` | Network fixtures |
| `tests/fixtures/notifications.ts` | Notification test fixtures |
| `tests/fixtures/encryption.ts` | Encryption fixtures |
| `tests/fixtures/access-lists.ts` | ACL fixtures |
| `tests/fixtures/dns-providers.ts` | DNS provider fixtures |
| `tests/fixtures/test-data.ts` | Shared test data |
| `tests/utils/wait-helpers.ts` | `waitForLoadingComplete`, `waitForTableLoad` |
| `tests/utils/ui-helpers.ts` | UI interaction helpers |
| `tests/utils/api-helpers.ts` | API request helpers |
| `tests/utils/TestDataManager.ts` | Test data lifecycle management |
| `tests/constants.ts` | Shared constants (`STORAGE_STATE`) |

**Import pattern**: Most tests import from `../fixtures/auth-fixtures` which re-exports `test`/`expect` from `./test.ts` (coverage-aware).

### 2.4 Frontend Routes (All Navigable Pages)

Extracted from [frontend/src/App.tsx](../../frontend/src/App.tsx):

| Route | Page Component | Auth Required | Role |
|-------|---------------|---------------|------|
| `/login` | Login | No | — |
| `/setup` | Setup | No | — |
| `/accept-invite` | AcceptInvite | No | — |
| `/passthrough` | PassthroughLanding | Yes | Any |
| `/` | Dashboard | Yes | Any |
| `/proxy-hosts` | ProxyHosts | Yes | Any |
| `/remote-servers` | RemoteServers | Yes | Any |
| `/domains` | Domains | Yes | Any |
| `/certificates` | Certificates | Yes | Any |
| `/dns/providers` | DNSProviders | Yes | Any |
| `/dns/plugins` | Plugins | Yes | Any |
| `/security` | Security | Yes | Any |
| `/security/audit-logs` | AuditLogs | Yes | Any |
| `/security/access-lists` | AccessLists | Yes | Any |
| `/security/crowdsec` | CrowdSecConfig | Yes | Any |
| `/security/rate-limiting` | RateLimiting | Yes | Any |
| `/security/waf` | WafConfig | Yes | Any |
| `/security/headers` | SecurityHeaders | Yes | Any |
| `/security/encryption` | EncryptionManagement | Yes | Any |
| `/access-lists` | AccessLists | Yes | Any |
| `/uptime` | Uptime | Yes | Any |
| `/settings` | Settings > SystemSettings | Yes | admin/user |
| `/settings/system` | SystemSettings | Yes | admin/user |
| `/settings/notifications` | Notifications | Yes | admin/user |
| `/settings/smtp` | SMTPSettings | Yes | admin/user |
| `/settings/users` | UsersPage | Yes | admin |
| `/tasks` | Tasks > Backups | Yes | Any |
| `/tasks/backups` | Backups | Yes | Any |
| `/tasks/logs` | Logs | Yes | Any |
| `/tasks/import/caddyfile` | ImportCaddy | Yes | Any |
| `/tasks/import/crowdsec` | ImportCrowdSec | Yes | Any |
| `/tasks/import/npm` | ImportNPM | Yes | Any |
| `/tasks/import/json` | ImportJSON | Yes | Any |

**Total unique pages to scan**: ~30 authenticated + 3 unauthenticated = **~33 pages**.

### 2.5 CI Workflows

**Primary workflow**: `.github/workflows/e2e-tests-split.yml`

Architecture (15 total jobs):

- **Build**: Single job builds Docker image, uploads as artifact
- **3 Security Enforcement jobs** (1 per browser, serial, 60min timeout) — runs `tests/security-enforcement/`, `tests/security/`, `tests/integration/multi-feature-workflows.spec.ts`
- **12 Non-Security jobs** (4 shards x 3 browsers, parallel, 60min timeout) — runs `tests/core`, `tests/dns-provider-*.spec.ts`, `tests/integration`, `tests/manual-dns-provider.spec.ts`, `tests/monitoring`, `tests/settings`, `tests/tasks`

Triggered by: `workflow_call`, `workflow_dispatch`, `pull_request`.

Non-security test directories explicitly listed in each browser job's Playwright invocation:

```
tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts
tests/integration tests/manual-dns-provider.spec.ts tests/monitoring
tests/settings tests/tasks
```

### 2.6 Package Configuration

**Current devDependencies** (from `package.json`):

- `@playwright/test`: `^1.59.1`
- `@bgotink/playwright-coverage`: `^0.3.2`
- `dotenv`: `^17.4.2`
- `typescript`: `^6.0.3`
- `vite`: `^8.0.9`
- `vitest`: `^4.1.4`

`@axe-core/playwright` is **not yet installed**. Latest version: `4.11.2`.

### 2.7 Lefthook Configuration

Pre-commit hooks run in parallel: file hygiene, YAML check, shellcheck, actionlint, Go lint, frontend type-check, frontend lint, semgrep. No Playwright-related hooks. No changes needed for this feature.

---

## 3. Technical Specifications

### 3.1 Architecture Decision: Dedicated A11y Spec Files

**Decision**: Create **dedicated accessibility spec files** in a new `tests/a11y/` directory rather than embedding axe scans into existing spec files.

**Rationale**:

| Approach | Pros | Cons |
|----------|------|------|
| **Dedicated specs** (chosen) | Clean separation of concerns; a11y failures don't mask functional failures; can be sharded independently; easy to skip/focus; clear ownership | Slight duplication of page navigation |
| Embedded in existing specs | No navigation duplication; tests a11y in real user flows | Mixes functional and a11y failures; harder to triage; slows all tests; harder to baseline/skip |

The dedicated approach is preferred because:

1. A11y violations may be numerous initially and need a baseline — mixing them with functional tests would cause noise
2. Independent sharding means a11y tests don't slow down existing functional test shards
3. Clearer CI reporting: a11y failures are immediately identifiable in workflow job names

### 3.2 Shared Accessibility Fixture

**File**: `tests/fixtures/a11y.ts`

```typescript
// Signature (not implementation)
import { test as base } from './auth-fixtures';
import AxeBuilder from '@axe-core/playwright';

interface A11yFixtures {
  makeAxeBuilder: () => AxeBuilder;
}

export const test = base.extend<A11yFixtures>({
  makeAxeBuilder: async ({ page }, use) => {
    const makeAxeBuilder = () =>
      new AxeBuilder({ page })
        .withTags(['wcag2a', 'wcag2aa', 'wcag22aa'])
        .exclude('.chartjs-canvas'); // Exclude known third-party canvases
    await use(makeAxeBuilder);
  },
});

export { expect } from './auth-fixtures';
```

> **Note**: The `.chartjs-canvas` selector is a placeholder. Verify against the actual DOM before implementation. A more robust approach may be to target `canvas` elements within chart container elements (e.g., `.chart-container canvas`).

**Key design points**:

- Extends `auth-fixtures` to inherit `adminUser`, `regularUser`, `guestUser`, and `testData` fixtures through the full extension chain (`auth-fixtures` → `test.ts` → coverage-aware base)
- Factory function (`makeAxeBuilder`) allows per-test customization (`.exclude()`, `.disableRules()`)
- WCAG tags: `wcag2a` (Level A), `wcag2aa` (Level AA), `wcag22aa` (WCAG 2.2 AA-specific rules)
- Global exclusions for known third-party elements that can't be fixed upstream

### 3.3 A11y Helper Module

**File**: `tests/utils/a11y-helpers.ts`

```typescript
// Signature (not implementation)
import type { AxeResults, Result } from 'axe-core';

type ViolationImpact = 'critical' | 'serious' | 'moderate' | 'minor';

interface A11yAssertionOptions {
  /** Impacts to fail on. Default: ['critical', 'serious'] */
  failOn?: ViolationImpact[];
  /** Known violations to skip (rule IDs) */
  knownViolations?: string[];
}

/**
 * Filters axe results and returns only violations matching the fail criteria.
 * Formats violations for readable Playwright HTML report output.
 */
export function getFailingViolations(
  results: AxeResults,
  options?: A11yAssertionOptions
): Result[];

/**
 * Formats a violation for human-readable output in test reports.
 */
export function formatViolation(violation: Result): string;

/**
 * Standard assertion: expect zero critical/serious violations.
 */
export function expectNoA11yViolations(
  results: AxeResults,
  options?: A11yAssertionOptions
): void;
```

### 3.4 Known Violations Baseline

**File**: `tests/a11y/a11y-baseline.ts`

A centralized baseline of known violations that should not block CI. This enables gradual remediation.

```typescript
// Signature (not implementation)

interface BaselineEntry {
  ruleId: string;
  pages: string[];  // Route patterns where this rule is expected to fail
  reason: string;   // Why this is baselined
  ticket?: string;  // Tracking issue for remediation
  expiresAt?: string; // ISO date for periodic review (e.g., '2026-07-01')
}

export const A11Y_BASELINE: BaselineEntry[];
```

> **Baseline review process**: Baseline entries should be periodically reviewed. Use the optional `expiresAt` field to flag entries for re-evaluation. Entries past their expiration date should be investigated and either remediated or renewed with justification.

### 3.5 axe-core Configuration

| Setting | Value | Rationale |
|---------|-------|-----------|
| `withTags` | `['wcag2a', 'wcag2aa', 'wcag22aa']` | Targets WCAG 2.2 Level AA conformance per project a11y instructions |
| Fail threshold | `critical` + `serious` impacts | Blocks CI on high-impact violations only |
| `moderate` / `minor` | Reported but non-blocking | Allows gradual improvement |
| Global excludes | Third-party canvases (Chart.js), Toaster containers | Cannot be fixed in application code |

### 3.6 Reporter Integration

Axe results will be surfaced in the Playwright HTML report via:

1. **`test.info().attach()`**: Attach violation details as JSON artifacts to each test
2. **Formatted assertion messages**: `expect(failingViolations).toEqual([])` with a descriptive message showing rule ID, impact, affected nodes, and fix suggestions
3. **Traces**: Standard `on-first-retry` trace capture applies to a11y tests too

### 3.7 Pages to Scan

**Priority Tier 1** (most user-facing, first commit):

| Route | Description |
|-------|-------------|
| `/login` | Login page (unauthenticated — requires `storageState: { cookies: [], origins: [] }` to prevent redirect) |
| `/` | Dashboard |
| `/proxy-hosts` | Proxy host management |
| `/certificates` | Certificate management |
| `/dns/providers` | DNS provider management |
| `/settings` | System settings |
| `/settings/users` | User management |

**Priority Tier 2** (second commit):

| Route | Description |
|-------|-------------|
| `/security` | Security dashboard |
| `/security/access-lists` | Access list management |
| `/security/crowdsec` | CrowdSec configuration |
| `/security/waf` | WAF configuration |
| `/security/rate-limiting` | Rate limiting |
| `/security/headers` | Security headers |
| `/security/encryption` | Encryption management |
| `/security/audit-logs` | Audit logs |
| `/uptime` | Uptime monitoring |

**Priority Tier 3** (third commit):

| Route | Description |
|-------|-------------|
| `/tasks/backups` | Backup management |
| `/tasks/logs` | Log viewer |
| `/tasks/import/caddyfile` | Caddyfile import |
| `/tasks/import/crowdsec` | CrowdSec import |
| `/tasks/import/npm` | NPM import |
| `/tasks/import/json` | JSON import |
| `/domains` | Domain management |
| `/remote-servers` | Remote server management |
| `/settings/notifications` | Notification settings |
| `/settings/smtp` | SMTP configuration |
| `/setup` | Initial setup page (unauthenticated — requires `storageState: { cookies: [], origins: [] }`) |

---

## 4. Implementation Plan

### Phase 1: Infrastructure Setup

**Commit 1**: Install dependency and create shared fixtures/helpers

**Files created/modified**:

| File | Action | Description |
|------|--------|-------------|
| `package.json` | Modified | Add `@axe-core/playwright` to `devDependencies` |
| `package-lock.json` | Modified | Lockfile update |
| `tests/fixtures/a11y.ts` | Created | Shared a11y test fixture with `makeAxeBuilder` factory |
| `tests/utils/a11y-helpers.ts` | Created | `getFailingViolations()`, `formatViolation()`, `expectNoA11yViolations()` |
| `tests/a11y/a11y-baseline.ts` | Created | Empty baseline array (initial state — no known violations) |

**Validation gate**: `npm ci` succeeds; `npx tsc --noEmit` passes on new files; imports resolve correctly.

### Phase 2: Tier 1 A11y Specs

**Commit 2**: Add accessibility tests for Tier 1 pages (login, dashboard, proxy-hosts, certificates, dns, settings, users)

**Files created**:

| File | Description |
|------|-------------|
| `tests/a11y/login.a11y.spec.ts` | Scans `/login` (unauthenticated — uses `test.use({ storageState: { cookies: [], origins: [] } })`) |
| `tests/a11y/dashboard.a11y.spec.ts` | Scans `/` |
| `tests/a11y/proxy-hosts.a11y.spec.ts` | Scans `/proxy-hosts` |
| `tests/a11y/certificates.a11y.spec.ts` | Scans `/certificates` |
| `tests/a11y/dns-providers.a11y.spec.ts` | Scans `/dns/providers` |
| `tests/a11y/settings.a11y.spec.ts` | Scans `/settings` and `/settings/users` |

**Test structure** (each authenticated spec file follows this pattern):

Authenticated pages rely on the stored auth state from `storageState: STORAGE_STATE` (configured in `playwright.config.js` via the `setup` project), matching the pattern used by all existing non-security tests. No manual `loginUser()` call is needed.

```typescript
import { test, expect } from '../fixtures/a11y';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import { expectNoA11yViolations } from '../utils/a11y-helpers';

test.describe('Accessibility: Dashboard', () => {
  test.describe.configure({ mode: 'parallel' });

  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await waitForLoadingComplete(page);
  });

  test('dashboard has no critical a11y violations', async ({ page, makeAxeBuilder }) => {
    const results = await makeAxeBuilder().analyze();
    test.info().attach('a11y-results', {
      body: JSON.stringify(results.violations, null, 2),
      contentType: 'application/json',
    });
    expectNoA11yViolations(results);
  });
});
```

**Unauthenticated page pattern** (`/login`, `/setup`, `/accept-invite`):

```typescript
import { test, expect } from '../fixtures/a11y';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import { expectNoA11yViolations } from '../utils/a11y-helpers';

// Clear stored auth state to prevent redirect to dashboard
test.use({ storageState: { cookies: [], origins: [] } });

test.describe('Accessibility: Login', () => {
  test.describe.configure({ mode: 'parallel' });

  test('login page has no critical a11y violations', async ({ page, makeAxeBuilder }) => {
    await page.goto('/login');
    await waitForLoadingComplete(page);

    const results = await makeAxeBuilder().analyze();
    test.info().attach('a11y-results', {
      body: JSON.stringify(results.violations, null, 2),
      contentType: 'application/json',
    });
    expectNoA11yViolations(results);
  });
});
```

> **Parallel mode**: Each a11y test is an independent page scan with no shared state, so `test.describe.configure({ mode: 'parallel' })` should be used in all a11y describe blocks to maximize throughput.
```

**Validation gate**: Tests run locally against Docker container. All pass or fail with only baseline-allowed violations. Verify HTML report contains a11y result attachments.

### Phase 3: Tier 2 A11y Specs

**Commit 3**: Add accessibility tests for security and monitoring pages

**Files created**:

| File | Description |
|------|-------------|
| `tests/a11y/security.a11y.spec.ts` | Scans `/security`, `/security/access-lists`, `/security/crowdsec`, `/security/waf`, `/security/rate-limiting`, `/security/headers`, `/security/encryption`, `/security/audit-logs` |
| `tests/a11y/uptime.a11y.spec.ts` | Scans `/uptime` |

**Validation gate**: All new tests pass locally. Verify cross-browser with `--project=firefox --project=chromium --project=webkit`.

### Phase 4: Tier 3 A11y Specs

**Commit 4**: Add accessibility tests for tasks, domains, remote servers, notifications, SMTP, setup pages

**Files created**:

| File | Description |
|------|-------------|
| `tests/a11y/tasks.a11y.spec.ts` | Scans `/tasks/backups`, `/tasks/logs`, `/tasks/import/caddyfile`, `/tasks/import/crowdsec`, `/tasks/import/npm`, `/tasks/import/json` |
| `tests/a11y/domains.a11y.spec.ts` | Scans `/domains`, `/remote-servers` |
| `tests/a11y/notifications.a11y.spec.ts` | Scans `/settings/notifications`, `/settings/smtp` |
| `tests/a11y/setup.a11y.spec.ts` | Scans `/setup` (unauthenticated — uses `test.use({ storageState: { cookies: [], origins: [] } })`; requires fresh state or skips if already set up) |

**Validation gate**: Full local run with all 3 browsers.

### Phase 5: CI Integration

**Commit 5**: Add `tests/a11y/` to CI workflow non-security shard test paths

**Files modified**:

| File | Change |
|------|--------|
| `.github/workflows/e2e-tests-split.yml` | Add `tests/a11y` to the non-security test directory list in all three browser jobs (`e2e-chromium`, `e2e-firefox`, `e2e-webkit`) |

The change in each browser job's Playwright invocation adds `tests/a11y` to the directory list:

```bash
# Before
npx playwright test \
  --project=chromium \
  --shard=${{ matrix.shard }}/${{ matrix.total-shards }} \
  tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts \
  tests/integration tests/manual-dns-provider.spec.ts tests/monitoring \
  tests/settings tests/tasks

# After
npx playwright test \
  --project=chromium \
  --shard=${{ matrix.shard }}/${{ matrix.total-shards }} \
  tests/a11y tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts \
  tests/integration tests/manual-dns-provider.spec.ts tests/monitoring \
  tests/settings tests/tasks
```

**Validation gate**: Push to a feature branch. Verify all 15 CI jobs pass (or fail only on genuine a11y issues). Verify a11y tests appear in uploaded Playwright HTML report artifacts.

> **Shard timing monitoring**: After rollout, monitor shard execution times across the 12 non-security jobs. If a11y tests create significant imbalance (one shard consistently slower), consider a dedicated a11y CI job with its own sharding. This is a "watch and react" item — no preemptive action needed.

### Phase 6: Documentation

**Commit 6**: Add documentation for the a11y testing setup

**Files created/modified**:

| File | Action | Description |
|------|--------|-------------|
| `tests/a11y/README.md` | Created | Documents how to run a11y tests, add new pages, manage the baseline, interpret results |

---

## 5. CI Integration Details

### 5.1 Where A11y Tests Run

A11y tests join the **non-security shard** jobs. They:

- Run across all 3 browsers (Chromium, Firefox, WebKit)
- Are distributed across the 4 shards per browser via Playwright's `--shard` flag
- Use the same Docker container (Cerberus OFF)
- Share the same auth setup dependency

### 5.2 Sharding Impact

Adding ~10 spec files to the non-security pool (currently ~50 spec files sharded 4 ways per browser) increases the per-shard workload by ~20%. Each axe scan takes 2-5 seconds per page, so the total added time per shard is approximately **10-30 seconds** — within acceptable tolerance given the 60-minute timeout.

### 5.3 Failure Behavior

| Impact Level | CI Behavior |
|-------------|-------------|
| `critical` | **Fails CI** — test assertion fails, shard exits non-zero |
| `serious` | **Fails CI** — test assertion fails, shard exits non-zero |
| `moderate` | **Reported only** — attached to HTML report as JSON, does not fail |
| `minor` | **Reported only** — attached to HTML report as JSON, does not fail |

### 5.4 Baseline Workflow

When a new genuine violation is discovered that cannot be immediately fixed:

1. Create a GitHub issue tracking the remediation
2. Add the rule ID + page pattern to `tests/a11y/a11y-baseline.ts` with the issue reference
3. The `expectNoA11yViolations()` helper filters out baselined violations
4. When remediation is complete, remove the baseline entry — CI will now enforce the fix

---

## 6. Edge Cases and Considerations

### 6.1 Performance

- axe-core injects a script (~500KB) into each page; this happens per `analyze()` call
- Expected overhead per scan: 2-5 seconds
- Total overhead for ~33 pages across 3 browsers: ~5-8 minutes of additional CI time distributed across 12 non-security shards
- **Mitigation**: Tests use `waitForLoadingComplete()` to ensure pages are fully rendered before scanning, avoiding incomplete DOM analysis

### 6.2 Dynamic Content and Loading States

- All scans MUST wait for loading states to complete (`waitForLoadingComplete()`)
- Pages with lazy-loaded content (modals, dropdowns) should be scanned in their default state first; modal-specific scans can be added as follow-up
- The `waitForTableLoad()` helper should be used for pages with data tables (proxy hosts, certificates, etc.)
- **Async data pages**: Pages that fetch data asynchronously (proxy-hosts, certificates, DNS providers, uptime monitors) should use `waitForTableLoad()` or equivalent waits to ensure the data-populated DOM is scanned, not the loading skeleton

### 6.3 Browser-Specific Behavior

axe-core produces consistent results across browsers because it analyzes the DOM/ARIA tree, not rendered pixels. However:

- WebKit may have minor differences in ARIA attribute support
- Running across all 3 browsers catches rendering-layer a11y issues (e.g., focus visibility) that axe cannot detect
- If a violation appears in one browser but not others, investigate before baselining

### 6.4 Third-Party Components

| Component | Strategy |
|-----------|----------|
| Chart.js canvases | Exclude via `.exclude('.chartjs-canvas')` or equivalent selector — canvas elements have inherent a11y limitations |
| React Hot Toast | Exclude toaster container — controlled by library, has built-in ARIA |
| Code editors (if any) | Exclude via selector — third-party code editors have known a11y gaps |

### 6.5 Gradual Rollout Strategy

To avoid blocking CI with a flood of violations on first merge:

1. **Commit 1-4**: Build the infrastructure and specs. Run locally, observe results.
2. **Before Commit 5** (CI integration): Populate `a11y-baseline.ts` with any critical/serious violations found during local testing. Create tracking issues for each.
3. **Commit 5**: CI integration — all tests pass because known violations are baselined.
4. **Post-merge**: Remediate baselined violations one by one. As each is fixed, remove from baseline. CI enforces the fix from that point forward.

---

## 7. Files Requiring Review for Updates

| File | Check | Action Required |
|------|-------|-----------------|
| `.gitignore` | No new generated files outside existing patterns | **No change needed** |
| `codecov.yml` | a11y tests are Playwright specs, already covered by E2E patterns | **No change needed** |
| `.dockerignore` | `tests/` is not copied into Docker image | **No change needed** |
| `Dockerfile` | Tests are not part of the Docker build | **No change needed** |
| `lefthook.yml` | No pre-commit a11y hooks needed | **No change needed** |
| `playwright.config.js` | a11y specs match existing `testMatch` and `testIgnore` patterns. The `tests/a11y/` directory is NOT in any ignore pattern. | **No change needed** |
| `tsconfig.json` (if any) | Ensure `@axe-core/playwright` types resolve | **Verify** — `@axe-core/playwright` ships with TypeScript declarations |

---

## 8. Commit Slicing Strategy

**Approach**: Single PR with 6 ordered logical commits.

**Trigger reasons**: Single feature scope, low cross-domain risk, incremental validation.

### Commit 1: Infrastructure — install dependency and create shared fixtures

- **Scope**: Package installation, fixture creation, helper module, baseline file
- **Files**: `package.json`, `package-lock.json`, `tests/fixtures/a11y.ts`, `tests/utils/a11y-helpers.ts`, `tests/a11y/a11y-baseline.ts`
- **Dependencies**: None
- **Validation**: `npm ci`, `npx tsc --noEmit`, import resolution

### Commit 2: Tier 1 a11y specs — core pages

- **Scope**: A11y tests for login, dashboard, proxy-hosts, certificates, DNS, settings
- **Files**: `tests/a11y/login.a11y.spec.ts`, `tests/a11y/dashboard.a11y.spec.ts`, `tests/a11y/proxy-hosts.a11y.spec.ts`, `tests/a11y/certificates.a11y.spec.ts`, `tests/a11y/dns-providers.a11y.spec.ts`, `tests/a11y/settings.a11y.spec.ts`
- **Dependencies**: Commit 1
- **Validation**: `npx playwright test tests/a11y/ --project=firefox`

### Commit 3: Tier 2 a11y specs — security and monitoring pages

- **Scope**: A11y tests for security suite and uptime monitoring
- **Files**: `tests/a11y/security.a11y.spec.ts`, `tests/a11y/uptime.a11y.spec.ts`
- **Dependencies**: Commit 1
- **Validation**: `npx playwright test tests/a11y/ --project=firefox`

### Commit 4: Tier 3 a11y specs — tasks, domains, notifications, setup

- **Scope**: Remaining page coverage
- **Files**: `tests/a11y/tasks.a11y.spec.ts`, `tests/a11y/domains.a11y.spec.ts`, `tests/a11y/notifications.a11y.spec.ts`, `tests/a11y/setup.a11y.spec.ts`
- **Dependencies**: Commit 1
- **Validation**: Full a11y suite: `npx playwright test tests/a11y/ --project=chromium --project=firefox --project=webkit`

### Commit 5: CI integration — add a11y tests to workflow

- **Scope**: Add `tests/a11y` to non-security shard test paths in CI workflow
- **Files**: `.github/workflows/e2e-tests-split.yml`
- **Dependencies**: Commits 1-4 (all specs must pass first)
- **Validation**: Push to feature branch; all 15 CI jobs pass

### Commit 6: Documentation

- **Scope**: README for a11y test directory
- **Files**: `tests/a11y/README.md`
- **Dependencies**: Commits 1-5
- **Validation**: Markdown lint passes

### Rollback

If the PR causes CI instability post-merge:

1. **Immediate**: Revert Commit 5 only (removes `tests/a11y` from CI paths) — a11y tests still exist but don't run in CI
2. **Investigation**: Run a11y tests locally to identify flaky or environment-dependent failures
3. **Resolution**: Fix failures, re-add to CI

---

## 9. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | `@axe-core/playwright` installed as devDependency | `npm ls @axe-core/playwright` returns version |
| 2 | Shared fixture provides `makeAxeBuilder` factory | `tests/fixtures/a11y.ts` exports correctly |
| 3 | A11y scans cover all ~33 navigable pages | Count tests in `tests/a11y/` matches page list |
| 4 | WCAG 2.2 AA tags configured | `withTags(['wcag2a', 'wcag2aa', 'wcag22aa'])` in fixture |
| 5 | CI fails on critical/serious violations | Inject a test violation, verify CI fails |
| 6 | Results visible in Playwright HTML report | Open report, verify JSON attachments on a11y tests |
| 7 | Works across Chromium, Firefox, WebKit | All 3 browser projects pass in CI |
| 8 | Baseline mechanism for known violations | Baselined violations do not fail CI |
| 9 | CI workflow updated to include `tests/a11y` | Verify in `.github/workflows/e2e-tests-split.yml` |
| 10 | No existing tests broken | All non-a11y CI jobs still pass |

---

## 10. Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| High number of initial violations blocks merge | High | Medium | Baseline mechanism (Section 6.5); populate before CI integration |
| axe scan flakiness in CI | Low | Medium | Retries (already configured: 2 in CI); `waitForLoadingComplete` before scans |
| Performance degradation of CI | Low | Low | ~10-30s additional per shard; well within 60min timeout |
| WebKit axe-core compatibility | Low | Low | axe-core is DOM-based, browser-agnostic; monitor for edge cases |
| Third-party component violations | Medium | Low | Global exclusions in fixture; documented in baseline |

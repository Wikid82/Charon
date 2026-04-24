## Accessibility Test Suite (`tests/a11y`)

### Purpose and Scope

This suite checks key Charon pages for accessibility issues using Playwright and axe.
It is focused on page-level smoke coverage so we can catch major accessibility regressions early.

### Run Locally

Run a quick single-browser check:

```bash
npx playwright test tests/a11y/ --project=firefox
```

Run the full cross-browser matrix:

```bash
npx playwright test tests/a11y/ --project=chromium --project=firefox --project=webkit
```

### CI Execution

In CI, this suite runs in the non-security shard jobs of the E2E split workflow:

- Workflow: `.github/workflows/e2e-tests-split.yml`
- Jobs: non-security shard jobs for Chromium, Firefox, and WebKit
- Behavior: `tests/a11y` is included in the Playwright test paths and distributed by `--shard`

### Add a New Page Accessibility Test

1. Create or update a spec in `tests/a11y/`.
2. Import the accessibility fixture from `../fixtures/a11y`.
3. Use wait helpers (for example from `../utils/wait-helpers`) before running axe so page state is stable.
4. Attach scan results with `test.info().attach(...)` for report debugging.
5. Filter known accepted baseline items using `getBaselinedRuleIds('<page-path>')`.
6. Assert with `expectNoA11yViolations`.

Minimal pattern:

```ts
import { test } from '../fixtures/a11y';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import { expectNoA11yViolations } from '../utils/a11y-helpers';
import { getBaselinedRuleIds } from './a11y-baseline';

test('example page has no critical a11y violations', async ({ page, makeAxeBuilder }) => {
  await page.goto('/example');
  await waitForLoadingComplete(page);

  const results = await makeAxeBuilder().analyze();

  test.info().attach('a11y-results', {
    body: JSON.stringify(results.violations, null, 2),
    contentType: 'application/json',
  });

  expectNoA11yViolations(results, {
    knownViolations: getBaselinedRuleIds('/example'),
  });
});
```

### Baseline Policy

Baseline entries are allowed only for known and accepted issues with clear rationale and a tracking ticket.

- Add a clear `reason` and a `ticket` reference.
- Add `expiresAt` so each baseline is reviewed periodically.
- Remove the baseline entry as soon as the underlying issue is fixed.

### Failure Semantics

- `critical` and `serious` violations fail the test.
- `moderate` and `minor` violations are reported in attached output and do not fail by default.

### Troubleshooting Timeout Flakes

Intermittent timeout flakes can happen, especially on Firefox.

Recommended rerun strategy:

1. Rerun the same failed spec once in Firefox.
2. If it passes on rerun, treat it as a transient flake and continue.
3. If it fails again, run the full a11y suite in Firefox.
4. If still failing, run all three browsers and inspect `a11y-results` attachments.

Useful commands:

```bash
# Rerun one spec in Firefox
npx playwright test tests/a11y/<spec-file>.spec.ts --project=firefox

# Rerun full a11y suite in Firefox
npx playwright test tests/a11y/ --project=firefox

# Rerun full a11y suite in all browsers
npx playwright test tests/a11y/ --project=chromium --project=firefox --project=webkit
```

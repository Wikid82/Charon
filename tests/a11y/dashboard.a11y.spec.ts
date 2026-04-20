import { test } from '../fixtures/a11y';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import { expectNoA11yViolations } from '../utils/a11y-helpers';
import { getBaselinedRuleIds } from './a11y-baseline';

test.describe('Accessibility: Dashboard', () => {
  test.describe.configure({ mode: 'parallel' });

  test('dashboard has no critical a11y violations', async ({ page, makeAxeBuilder }) => {
    await test.step('Navigate to dashboard', async () => {
      await page.goto('/');
      await waitForLoadingComplete(page);
    });

    await test.step('Run axe accessibility scan', async () => {
      const results = await makeAxeBuilder().analyze();

      test.info().attach('a11y-results', {
        body: JSON.stringify(results.violations, null, 2),
        contentType: 'application/json',
      });

      expectNoA11yViolations(results, {
        knownViolations: getBaselinedRuleIds('/'),
      });
    });
  });
});

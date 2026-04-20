import { test } from '../fixtures/a11y';
import { waitForLoadingComplete, waitForTableLoad } from '../utils/wait-helpers';
import { expectNoA11yViolations } from '../utils/a11y-helpers';
import { getBaselinedRuleIds } from './a11y-baseline';

test.describe('Accessibility: Proxy Hosts', () => {
  test.describe.configure({ mode: 'parallel' });

  test('proxy hosts page has no critical a11y violations', async ({ page, makeAxeBuilder }) => {
    await test.step('Navigate to proxy hosts', async () => {
      await page.goto('/proxy-hosts');
      await waitForLoadingComplete(page);
      await waitForTableLoad(page);
    });

    await test.step('Run axe accessibility scan', async () => {
      const results = await makeAxeBuilder().analyze();

      test.info().attach('a11y-results', {
        body: JSON.stringify(results.violations, null, 2),
        contentType: 'application/json',
      });

      expectNoA11yViolations(results, {
        knownViolations: getBaselinedRuleIds('/proxy-hosts'),
      });
    });
  });
});

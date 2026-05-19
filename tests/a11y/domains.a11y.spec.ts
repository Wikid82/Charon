import { test } from '../fixtures/a11y';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import { expectNoA11yViolations } from '../utils/a11y-helpers';
import { getBaselinedRuleIds } from './a11y-baseline';

const domainRoutes = [
  { route: '/domains', name: 'domains' },
  { route: '/remote-servers', name: 'remote servers' },
] as const;

test.describe('Accessibility: Domains & Remote Servers', () => {

  for (const { route, name } of domainRoutes) {
    test(`${name} page has no critical a11y violations`, async ({ page, makeAxeBuilder }) => {
      await test.step(`Navigate to ${name}`, async () => {
        await page.goto(route);
        await waitForLoadingComplete(page);
      });

      await test.step('Run axe accessibility scan', async () => {
        const results = await makeAxeBuilder().analyze();

        test.info().attach('a11y-results', {
          body: JSON.stringify(results.violations, null, 2),
          contentType: 'application/json',
        });

        expectNoA11yViolations(results, {
          knownViolations: getBaselinedRuleIds(route),
        });
      });
    });
  }
});

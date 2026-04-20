import { test } from '../fixtures/a11y';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import { expectNoA11yViolations } from '../utils/a11y-helpers';
import { getBaselinedRuleIds } from './a11y-baseline';

const securityRoutes = [
  { route: '/security', name: 'security dashboard' },
  { route: '/security/access-lists', name: 'access lists' },
  { route: '/security/crowdsec', name: 'CrowdSec' },
  { route: '/security/waf', name: 'WAF' },
  { route: '/security/rate-limiting', name: 'rate limiting' },
  { route: '/security/headers', name: 'security headers' },
  { route: '/security/encryption', name: 'encryption' },
  { route: '/security/audit-logs', name: 'audit logs' },
] as const;

test.describe('Accessibility: Security', () => {
  test.describe.configure({ mode: 'parallel' });

  for (const { route, name } of securityRoutes) {
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

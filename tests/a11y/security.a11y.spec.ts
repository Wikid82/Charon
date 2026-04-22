import { test, expect } from '../fixtures/a11y';
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

/**
 * Wait for route-specific content to be visible before axe analysis
 * Ensures all key page elements have been rendered
 */
async function waitForRouteReady(page: any, route: string): Promise<void> {
  // Wait for main content area if it exists (most pages have one)
  const main = page.locator('main');
  try {
    await expect(main).toBeVisible({ timeout: 5000 });
  } catch {
    // If no main element, just continue (some pages may not have it)
  }

  // Route-specific readiness conditions - all optional
  switch (route) {
    case '/security/headers':
      // Security headers page has a button to create profiles
      try {
        await expect(page.getByRole('button', { name: /create|add|new/i }).first())
          .toBeVisible({ timeout: 5000 });
      } catch {
        // Button not found, continue anyway
      }
      break;
    case '/security/audit-logs':
      // Audit logs page may have a heading or table
      try {
        await expect(page.locator('h1, h2, table, [role="grid"]').first())
          .toBeVisible({ timeout: 5000 });
      } catch {
        // No expected content elements, continue anyway
      }
      break;
    default:
      // For other routes, just ensure main content is visible (already checked above)
      break;
  }
}

test.describe('Accessibility: Security', () => {

  for (const { route, name } of securityRoutes) {
    test(`${name} page has no critical a11y violations`, async ({ page, makeAxeBuilder }) => {
      await test.step(`Navigate to ${name}`, async () => {
        await page.goto(route);
        await waitForLoadingComplete(page);
        await waitForRouteReady(page, route);
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

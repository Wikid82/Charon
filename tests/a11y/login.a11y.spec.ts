import { test } from '../fixtures/a11y';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import { expectNoA11yViolations } from '../utils/a11y-helpers';
import { getBaselinedRuleIds } from './a11y-baseline';

test.use({ storageState: { cookies: [], origins: [] } });

test.describe('Accessibility: Login', () => {

  test('login page has no critical a11y violations', async ({ page, makeAxeBuilder }) => {
    await test.step('Navigate to login page', async () => {
      await page.goto('/login');
      await waitForLoadingComplete(page);
      // Wait for the login form to be fully rendered (setup status check complete).
      // This prevents the analyze() page.evaluate() from running while React is
      // still processing the setup status response, which can stall the JS engine
      // when the test runs after other authenticated tests in the same browser process.
      await page.getByRole('button', { name: 'Sign In' }).waitFor({ state: 'visible', timeout: 20000 });
    });

    await test.step('Run axe accessibility scan', async () => {
      const results = await makeAxeBuilder().analyze();

      test.info().attach('a11y-results', {
        body: JSON.stringify(results.violations, null, 2),
        contentType: 'application/json',
      });

      expectNoA11yViolations(results, {
        knownViolations: getBaselinedRuleIds('/login'),
      });
    });
  });
});

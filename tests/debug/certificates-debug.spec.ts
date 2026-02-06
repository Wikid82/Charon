
import { test, expect, loginUser } from '../fixtures/auth-fixtures'; // Use the fixture that provides adminUser
import { waitForLoadingComplete } from '../utils/wait-helpers';

test('Determine what is keeping the loader active', async ({ page, adminUser }) => {
  test.setTimeout(60000);
  console.log('Logging in...');
  await loginUser(page, adminUser);
  console.log('Logged in. Waiting for dashboard loader...');
  await waitForLoadingComplete(page);

  console.log('Navigating to /certificates...');
  await page.goto('/certificates');

  const loaderSelector = '[role="progressbar"], [aria-busy="true"], .loading-spinner, .loading, .spinner, [data-loading="true"], .animate-pulse';

  console.log('Polling for loaders...');
  // Poll for 15 seconds printing what we see
  let start = Date.now();
  while (Date.now() - start < 15000) {
      const loaders = page.locator(loaderSelector);
      const count = await loaders.count();
      if (count > 0) {
          console.log(`[${Date.now() - start}ms] Found ${count} loaders`);
          if (count < 5) { // Only log details if count is small to avoid spamming 35 items
            for(let i=0; i<count; i++) {
                const html = await loaders.nth(i).evaluate(el => el.outerHTML).catch(() => 'detached');
                console.log(`Loader ${i}: ${html}`);
            }
          } else {
             console.log(`(Too many to list individually, count=${count})`);
             const firstHtml = await loaders.first().evaluate(el => el.outerHTML).catch(() => 'detached');
             console.log(`First loader: ${firstHtml}`);
          }
      } else {
          console.log(`[${Date.now() - start}ms] 0 loaders found.`);
      }
      await page.waitForTimeout(500);
  }
});

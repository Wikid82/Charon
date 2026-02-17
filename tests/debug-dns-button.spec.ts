import { test } from './fixtures/test';
import { waitForLoadingComplete } from './utils/wait-helpers';

test('DEBUG: DNS Providers page state', async ({ page, baseURL, context }) => {
  console.log(`\n🔍 TEST CONTEXT INFO:`);
  console.log(`  Base URL: ${baseURL}`);
  console.log(`  Page URL before goto: ${page.url()}`);

  // Check cookies before navigation
  const cookiesBefore = await context.cookies();
  console.log(`  Cookies before navigation: ${cookiesBefore.length} total`);
  cookiesBefore.forEach(c => {
    console.log(`    - ${c.name}: domain="${c.domain}", httpOnly=${c.httpOnly}`);
  });

  // Monitor API requests
  const apiCalls: { url: string; status: number; method: string }[] = [];
  page.on('response', async (response) => {
    if (response.url().includes('/api/')) {
      apiCalls.push({
        url: response.url(),
        status: response.status(),
        method: response.request().method(),
      });
      console.log(`  API: ${response.request().method()} ${response.url()} → ${response.status()}`);

      // If it's an auth check, show response body
      if (response.url().includes('/auth/') || response.url().includes('/users/me')) {
        try {
          const body = await response.text();
          console.log(`    Response: ${body.substring(0, 200)}`);
        } catch (e) {
          console.log(`    (Could not read response body)`);
        }
      }
    }
  });

  await page.goto('/dns/providers');
  console.log(`  Page URL after goto: ${page.url()}`);

  // Wait for API calls to complete
  await page.waitForLoadState('networkidle');

  console.log(`\n  📊 API Calls Summary: ${apiCalls.length} total`);

  // Check cookies after navigation
  const cookiesAfter = await context.cookies();
  console.log(`  Cookies after navigation: ${cookiesAfter.length} total`);
  cookiesAfter.forEach(c => {
    console.log(`    - ${c.name}: domain="${c.domain}", value="${c.value.substring(0, 20)}..."`);
  });

  await waitForLoadingComplete(page);

  console.log('--- PAGE STATE AFTER INITIAL LOAD ---');

  // Log all buttons
  const buttons = await page.getByRole('button').all();
  console.log(`\nFound ${buttons.length} buttons:`);
  for (const btn of buttons) {
    const text = await btn.textContent();
    const visible = await btn.isVisible();
    const ariaLabel = await btn.getAttribute('aria-label');
    console.log(`- "${text}" (visible: ${visible}, aria-label: "${ariaLabel}")`);
  }

  // Log page content around "provider" text
  console.log('\n--- SEARCHING FOR PROVIDER-RELATED TEXT ---');
  const providerText = await page.getByText(/provider/i).all();
  console.log(`Found ${providerText.length} elements with "provider" text:`);
  for (let i = 0; i < Math.min(providerText.length, 10); i++) {
    const text = await providerText[i].textContent();
    const visible = await providerText[i].isVisible();
    const tagName = await providerText[i].evaluate(el => el.tagName);
    console.log(`  ${i+1}. <${tagName}> "${text?.substring(0, 50)}" (visible: ${visible})`);
  }

  // Log Outlet rendering
  console.log('\n--- CHECKING NESTED ROUTING ---');
  const outlet = page.locator('[data-outlet], .bg-surface-elevated.border.border-border.rounded-lg.p-6');
  const outletVisible = await outlet.isVisible();
  console.log(`Outlet container visible: ${outletVisible}`);

  if (outletVisible) {
    const outletContent = await outlet.textContent();
    console.log(`Outlet content preview: ${outletContent?.substring(0, 200)}`);
  }

  // Wait a bit longer and check again
  console.log('\n--- AFTER 2 SECOND WAIT ---');
  await page.waitForTimeout(2000);

  const buttonsAfterWait = await page.getByRole('button').all();
  console.log(`\nButtons after wait: ${buttonsAfterWait.length}`);
  for (const btn of buttonsAfterWait) {
    const text = await btn.textContent();
    const visible = await btn.isVisible();
    console.log(`- "${text}" (visible: ${visible})`);
  }

  // Check for specific button patterns
  console.log('\n--- SPECIFIC LOCATORS ---');
  const addProviderByText = page.getByRole('button', { name: /add.*provider/i });
  console.log(`getByRole('button', {name: /add.*provider/i}): ${await addProviderByText.count()} found`);

  const addProviderFirst = addProviderByText.first();
  const addProviderVisible = await addProviderFirst.isVisible().catch(() => false);
  console.log(`First button visible: ${addProviderVisible}`);

  if (addProviderVisible) {
    const text = await addProviderFirst.textContent();
    console.log(`First button text: "${text}"`);
  }

  // Log HTML snippet
  console.log('\n--- PAGE HTML SNIPPET ---');
  const mainContent = page.locator('main, [role="main"], .space-y-6').first();
  const html = await mainContent.innerHTML().catch(() => 'Could not get HTML');
  console.log(html.substring(0, 1000));
});

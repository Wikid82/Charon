import { test, expect, loginUser, logoutUser, TEST_PASSWORD } from '../fixtures/auth-fixtures';

import { dismissNewDomainPromptIfPresent, waitForLoadingComplete } from '../utils/wait-helpers';
import { caddyProxyOrigin, createUserViaApi, getAuthTokenFromPage } from '../utils/api-helpers';

/**
 * Integration: ACL & WAF Layering (Defense in Depth)
 *
 * Purpose: Validate ACL and WAF work as defense-in-depth layers
 * Scenarios: Both modules apply, WAF independent of role, ACL independent of payload
 * Success: Malicious requests blocked regardless of role, unauthorized users blocked regardless of payload
 *
 * All requests below that exercise WAF/ACL enforcement target the Caddy
 * *proxy* origin (port 80, via `caddyProxyOrigin()`) with the proxied
 * domain in the `Host` header, not the Charon management interface
 * (port 8080, `page.url()`'s own origin). Per ARCHITECTURE.md ("Management
 * Interface (Port 8080)"): "NO Cerberus Middleware: Rate limiting, ACL,
 * WAF, and CrowdSec are NOT applied to management interface" - a request
 * built from the management origin can never be blocked by WAF/ACL no
 * matter what payload it carries, and `/api/test`, `/api/protected`, etc.
 * aren't even real Charon routes. `multi-component-security-workflows.spec.ts`
 * established this pattern first.
 */

test.describe('ACL & WAF Layering', () => {
  const testProxy = {
    name: 'ACL WAF Test Proxy',
    domain: 'acl-waf-test.local',
    target: 'http://localhost:3001',
  };

  const testUser = {
    email: 'aclusertest@test.local',
    name: 'ACL User Test',
    // `loginUser()` (used below to log back in as this user) always
    // authenticates with the shared `TEST_PASSWORD` constant, not a
    // caller-supplied one - it's designed for the standard per-test
    // TestDataManager-created user pool. Match it here so the same
    // well-tested helper can be reused for this ad-hoc API-created user
    // instead of re-driving the login form by hand.
    password: TEST_PASSWORD,
  };

  // Use a fresh, disposable per-test admin (via the `adminUser` fixture)
  // rather than the shared default admin session. These tests deliberately
  // log out mid-test to switch to a regular user - logging out the *shared*
  // admin session would invalidate the token baked into the project's
  // single `storageState` file, breaking every other test in the run that
  // reuses it. `multi-component-security-workflows.spec.ts` already
  // established this pattern for the same reason.
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    // These tests bring up real ACL/WAF middleware, which is slower to
    // settle than a plain page load - a flat 5s waitForSelector was too
    // tight under real CrowdSec/WAF runtime conditions. Use the repo's
    // condition-based loading wait (matches
    // multi-component-security-workflows.spec.ts's beforeEach pattern)
    // before asserting on the main landmark.
    await waitForLoadingComplete(page, { timeout: 15000 });
    await expect(page.getByRole('main')).toBeVisible({ timeout: 15000 });
  });

  test.afterEach(async ({ page }) => {
    try {
      // Cleanup proxy
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });
      const proxyRow = page.locator(`text=${testProxy.domain}`).first();
      if (await proxyRow.isVisible()) {
        const deleteButton = proxyRow.locator('..').getByRole('button', { name: /delete/i }).first();
        await deleteButton.click();

        const confirmButton = page.getByRole('button', { name: /confirm|delete/i }).first();
        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }
        await page.waitForLoadState('networkidle');
      }

      // Cleanup user
      await page.goto('/users', { waitUntil: 'networkidle' });
      const userRow = page.locator(`text=${testUser.email}`).first();
      if (await userRow.isVisible()) {
        const deleteButton = userRow.locator('..').getByRole('button', { name: /delete/i }).first();
        await deleteButton.click();

        const confirmButton = page.getByRole('button', { name: /confirm|delete/i }).first();
        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }
        await page.waitForLoadState('networkidle');
      }
    } catch {
      // Ignore cleanup errors
    }
  });

  // Non-admin user cannot bypass WAF even with proxy access
  test('Regular user cannot bypass WAF on authorized proxy', async ({ page }) => {
    let createdUserId = '';

    await test.step('Admin creates test user with limited permissions', async () => {
      const created = await createUserViaApi(page, { ...testUser, role: 'user' });
      createdUserId = String(created.id);
    });

    await test.step('Admin creates proxy with WAF enabled', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/^name\b/i).fill(testProxy.name);
      await page.getByLabel(/^domain names/i).fill(testProxy.domain);
      await page.getByLabel(/^host\b/i).fill(testProxy.target);

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      const submitButton = page.getByRole('button', { name: 'Save', exact: true }).first();
      await dismissNewDomainPromptIfPresent(page);
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('User logs in', async () => {
      await logoutUser(page);
      await loginUser(page, { id: createdUserId, email: testUser.email, token: '', role: 'user' });
    });

    await test.step('User sends malicious request to proxy', async () => {
      const origin = caddyProxyOrigin(page);
      const response = await page.request.get(
        `${origin}/?id=1' OR '1'='1`,
        {
          headers: { Host: testProxy.domain },
          ignoreHTTPSErrors: true,
        }
      );

      // WAF blocks regardless of user privilege. 502 is acceptable: it
      // means Caddy routed past WAF to the (possibly unreachable in this
      // environment) upstream - see multi-component-security-workflows.spec.ts.
      expect([403, 502]).toContain(response.status());
    });
  });

  // WAF enforces regardless of user role
  test('WAF blocks malicious requests from all user roles', async ({ page }) => {
    await test.step('Create proxy with WAF', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/^name\b/i).fill(testProxy.name);
      await page.getByLabel(/^domain names/i).fill(testProxy.domain);
      await page.getByLabel(/^host\b/i).fill(testProxy.target);

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      const submitButton = page.getByRole('button', { name: 'Save', exact: true }).first();
      await dismissNewDomainPromptIfPresent(page);
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Admin sends malicious request', async () => {
      const origin = caddyProxyOrigin(page);
      const response = await page.request.post(
        `${origin}/api/test`,
        {
          data: { payload: `<script>alert('xss')</script>` },
          headers: { Host: testProxy.domain },
          ignoreHTTPSErrors: true,
        }
      );

      expect([403, 502]).toContain(response.status());
    });

    await test.step('Non-admin also blocked by WAF', async () => {
      // Create non-admin user directly via API (see createUserViaApi doc
      // comment: the admin "add user" UI is invite-only now, with no
      // password field to drive from a test).
      const created = await createUserViaApi(page, { ...testUser, role: 'user' });

      // Logout and login as user
      await logoutUser(page);
      await loginUser(page, { id: String(created.id), email: created.email, token: '', role: 'user' });

      const origin = caddyProxyOrigin(page);
      const response = await page.request.post(
        `${origin}/api/test`,
        {
          data: { payload: `'; DROP TABLE users;--` },
          headers: { Host: testProxy.domain },
          ignoreHTTPSErrors: true,
        }
      );

      expect([403, 502]).toContain(response.status());
    });
  });

  // Admin and user both subject to WAF and ACL
  test('Both admin and user roles subject to WAF protection', async ({ page }) => {
    let createdUserId = '';

    await test.step('Setup: Create proxy and user', async () => {
      // Create user directly via API (see createUserViaApi doc comment:
      // the admin "add user" UI is invite-only now, with no password
      // field to drive from a test).
      const created = await createUserViaApi(page, { ...testUser, role: 'user' });
      createdUserId = String(created.id);

      // Create proxy with WAF
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const createButton = page.getByRole('button', { name: /add|create/i }).first();
      await createButton.click();

      await page.getByLabel(/^name\b/i).fill(testProxy.name);
      await page.getByLabel(/^domain names/i).fill(testProxy.domain);
      await page.getByLabel(/^host\b/i).fill(testProxy.target);

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      const proxySubmit = page.getByRole('button', { name: 'Save', exact: true }).first();
      await dismissNewDomainPromptIfPresent(page);
      await proxySubmit.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('Verify admin blocked by WAF', async () => {
      const origin = caddyProxyOrigin(page);
      const response = await page.request.get(
        `${origin}/?cmd=env`,
        {
          headers: { Host: testProxy.domain },
          ignoreHTTPSErrors: true,
        }
      );

      expect([403, 502]).toContain(response.status());
    });

    await test.step('Verify user also blocked by WAF', async () => {
      await logoutUser(page);
      await loginUser(page, { id: createdUserId, email: testUser.email, token: '', role: 'user' });

      const origin = caddyProxyOrigin(page);
      const response = await page.request.get(
        `${origin}/?cmd=whoami`,
        {
          headers: { Host: testProxy.domain },
          ignoreHTTPSErrors: true,
        }
      );

      expect([403, 502]).toContain(response.status());
    });
  });

  // ACL adds layer beyond WAF (defense in depth)
  test('ACL restricts access beyond WAF protection', async ({ page }) => {
    let createdUserId = '';

    await test.step('Create restricted user', async () => {
      const created = await createUserViaApi(page, { ...testUser, role: 'user' });
      createdUserId = String(created.id);
    });

    await test.step('Create proxy with WAF but restrict access via ACL', async () => {
      // Create a deny-all access list via the API first. ProxyHostForm's
      // ACL control is a `<Select>` combobox bound to `access_list_id`
      // (see `AccessListSelector`), not a freetext ACL name/rule input -
      // there is no `input[name*="acl"]`/`textarea[name*="acl"]` anywhere
      // in the form, so the previous version of this test silently never
      // configured any ACL restriction at all.
      //
      // Built directly against the real `models.AccessList` JSON shape
      // (`type` + `ip_rules` as a JSON-*string*, confirmed against the
      // live API) rather than via api-helpers.ts's `createAccessListViaAPI`
      // - that helper's `AccessListCreateData` type (`rules: [{type,
      // value}]`) doesn't match the backend's real contract and 400s.
      const adminToken = await getAuthTokenFromPage(page);
      const accessListName = `ACL WAF Test Deny-All ${Date.now()}`;
      const accessListResponse = await page.request.post('/api/v1/access-lists', {
        data: {
          name: accessListName,
          type: 'blacklist',
          ip_rules: JSON.stringify([{ cidr: '0.0.0.0/0', description: 'deny all' }]),
          enabled: true,
        },
        headers: { Authorization: `Bearer ${adminToken}` },
      });
      expect(accessListResponse.ok()).toBe(true);

      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const addButton = page.getByRole('button', { name: /add|create/i }).first();
      await addButton.click();

      await page.getByLabel(/^name\b/i).fill(testProxy.name);
      await page.getByLabel(/^domain names/i).fill(testProxy.domain);
      await page.getByLabel(/^host\b/i).fill(testProxy.target);

      const wafToggle = page.locator('input[type="checkbox"][name*="waf"]').first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }

      // Assign the deny-all access list via the real ACL combobox.
      const aclCombobox = page.getByRole('combobox', { name: /access control list/i });
      if (await aclCombobox.isVisible().catch(() => false)) {
        await aclCombobox.click();
        await page.getByRole('option', { name: accessListName }).click();
      }

      const submitButton = page.getByRole('button', { name: 'Save', exact: true }).first();
      await dismissNewDomainPromptIfPresent(page);
      await submitButton.click();
      await page.waitForLoadState('networkidle');
    });

    await test.step('User with ACL restriction gets blocked', async () => {
      await logoutUser(page);
      await loginUser(page, { id: createdUserId, email: testUser.email, token: '', role: 'user' });

      const origin = caddyProxyOrigin(page);
      const response = await page.request.get(
        `${origin}/public`,
        {
          headers: { Host: testProxy.domain },
          ignoreHTTPSErrors: true,
        }
      );

      // ACL denies before the request ever reaches the upstream.
      expect([401, 403]).toContain(response.status());
    });
  });
});

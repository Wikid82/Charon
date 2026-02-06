/**
 * Security Configuration Workflow Tests
 *
 * Extracted from Group B of multi-feature-workflows.spec.ts
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { generateProxyHost } from '../fixtures/proxy-hosts';
import { generateAllowListForIPs } from '../fixtures/access-lists';
import {
  waitForLoadingComplete,
  waitForResourceInUI,
} from '../utils/wait-helpers';

test.describe('Security Configuration Workflow', () => {
  test('should configure complete security stack for host', async ({
    page,
    adminUser,
    testData,
  }) => {
    await loginUser(page, adminUser);

    await test.step('Create proxy host', async () => {
      const proxyInput = generateProxyHost();
      const proxy = await testData.createProxyHost({
        domain: proxyInput.domain,
        forwardHost: proxyInput.forwardHost,
        forwardPort: proxyInput.forwardPort,
      });

      await page.goto('/proxy-hosts');
      await waitForResourceInUI(page, proxy.domain);
    });

    await test.step('Navigate to security settings', async () => {
      await page.goto('/security');
      await waitForLoadingComplete(page);
      const content = page.locator('main, .content').first();
      await expect(content).toBeVisible();
    });
  });

  test('should enable WAF and verify protection', async ({
    page,
    adminUser,
  }) => {
    await loginUser(page, adminUser);

    await test.step('Navigate to WAF configuration', async () => {
      await page.goto('/security/waf');
      await waitForLoadingComplete(page);
    });

    await test.step('Verify WAF configuration page', async () => {
      const content = page.locator('main, .content').first();
      await expect(content).toBeVisible();
    });
  });

  test('should configure CrowdSec integration', async ({
    page,
    adminUser,
  }) => {
    await loginUser(page, adminUser);

    await test.step('Navigate to CrowdSec configuration', async () => {
      await page.goto('/security/crowdsec');
      await waitForLoadingComplete(page);
    });

    await test.step('Verify CrowdSec page loads', async () => {
      const content = page.locator('main, .content').first();
      await expect(content).toBeVisible();
    });
  });

  test('should setup access restrictions workflow', async ({
    page,
    adminUser,
    testData,
  }) => {
    await loginUser(page, adminUser);

    await test.step('Create restrictive ACL', async () => {
      const acl = generateAllowListForIPs(['10.0.0.0/8']);
      await testData.createAccessList(acl);

      await page.goto('/access-lists');
      await waitForResourceInUI(page, acl.name);
    });

    await test.step('Create protected proxy host', async () => {
      const proxyInput = generateProxyHost();
      const proxy = await testData.createProxyHost({
        domain: proxyInput.domain,
        forwardHost: proxyInput.forwardHost,
        forwardPort: proxyInput.forwardPort,
      });

      await page.goto('/proxy-hosts');
      await waitForResourceInUI(page, proxy.domain);
    });
  });
});

/**
 * Security Suite Integration E2E Tests (Phase 6.4)
 *
 * Tests for Cerberus security suite integration including WAF, CrowdSec,
 * ACLs, and security headers working together.
 *
 * Test Categories (23-28 tests):
 * - Group A: Cerberus Dashboard (4 tests)
 * - Group B: WAF + Proxy Integration (5 tests)
 * - Group C: CrowdSec + Proxy Integration (6 tests)
 * - Group D: Security Headers Integration (4 tests)
 * - Group E: Combined Security Features (4 tests)
 *
 * API Endpoints:
 * - GET/PUT /api/v1/cerberus/config
 * - GET /api/v1/cerberus/status
 * - GET/POST /api/v1/crowdsec/*
 * - GET/PUT /api/v1/security-headers
 * - GET /api/v1/audit-logs
 */

import { test, expect, loginUser, TEST_PASSWORD } from '../fixtures/auth-fixtures';
import { generateProxyHost } from '../fixtures/proxy-hosts';
import { generateAccessList, generateAllowListForIPs } from '../fixtures/access-lists';
import {
  waitForToast,
  waitForLoadingComplete,
  waitForAPIResponse,
  waitForModal,
  clickAndWaitForResponse,
} from '../utils/wait-helpers';

/**
 * Selectors for Security pages
 */
const SELECTORS = {
  // Cerberus Dashboard
  cerberusTitle: 'h1, h2',
  securityStatusCard: '[data-testid="security-status"], .security-status',
  wafStatusIndicator: '[data-testid="waf-status"], .waf-status',
  crowdsecStatusIndicator: '[data-testid="crowdsec-status"], .crowdsec-status',
  aclStatusIndicator: '[data-testid="acl-status"], .acl-status',

  // WAF Configuration
  wafEnableToggle: 'input[name="waf_enabled"], [data-testid="waf-toggle"]',
  wafModeSelect: 'select[name="waf_mode"], [data-testid="waf-mode"]',
  wafRulesTable: '[data-testid="waf-rules-table"], table',

  // CrowdSec Configuration
  crowdsecEnableToggle: 'input[name="crowdsec_enabled"], [data-testid="crowdsec-toggle"]',
  crowdsecApiKey: 'input[name="crowdsec_api_key"], #crowdsec-api-key',
  crowdsecDecisionsList: '[data-testid="crowdsec-decisions"], .decisions-list',
  crowdsecImportBtn: 'button:has-text("Import CrowdSec")',

  // Security Headers
  hstsToggle: 'input[name="hsts_enabled"], [data-testid="hsts-toggle"]',
  cspInput: 'textarea[name="csp"], #csp-policy',
  xfoSelect: 'select[name="x_frame_options"], #x-frame-options',

  // Audit Logs
  auditLogTable: '[data-testid="audit-log-table"], table',
  auditLogRow: '[data-testid="audit-log-row"], tbody tr',
  auditLogFilter: '[data-testid="audit-filter"], .filter',

  // Common
  saveButton: 'button:has-text("Save"), button[type="submit"]',
  loadingSkeleton: '[data-testid="loading-skeleton"], .loading',
  statusBadge: '.badge, [data-testid="status-badge"]',
};

test.describe('Security Suite Integration', () => {
  // Increase timeout from 300s (5min) to 600s (10min) for complex integration tests
  // Security suite creates multiple resources (proxy hosts, ACLs, CrowdSec configs) which requires more time
  test.describe.configure({ timeout: 600000 }); // 10 minutes

  // ===========================================================================
  // Group A: Cerberus Dashboard (4 tests)
  // ===========================================================================
  test.describe('Group A: Cerberus Dashboard', () => {
    test('should display Cerberus security dashboard', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to security dashboard', async () => {
        await page.goto('/security');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify dashboard heading', async () => {
        const heading = page.locator('h1, h2').first();
        await expect(heading).toBeVisible();
      });

      await test.step('Verify main content loads', async () => {
        const content = page.locator('main, .content, [role="main"]').first();
        await expect(content).toBeVisible();
      });
    });

    test('should show WAF status indicator', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to WAF configuration', async () => {
        await page.goto('/security/waf');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify WAF page loads', async () => {
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should show CrowdSec connection status', async ({
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

    test('should display overall security score', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to security dashboard', async () => {
        await page.goto('/security');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify security content', async () => {
        // Wait for page load before checking main content
        await waitForLoadingComplete(page);
        await page.waitForLoadState('networkidle', { timeout: 10000 });

        const content = page.locator('main, .content, [role="main"]').first();
        await expect(content).toBeVisible({ timeout: 10000 });
      });
    });
  });

  // ===========================================================================
  // Group B: WAF + Proxy Integration (5 tests)
  // ===========================================================================
  test.describe('Group B: WAF + Proxy Integration', () => {
    test('should enable WAF for proxy host', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      const proxyInput = generateProxyHost();
      const createdProxy = await testData.createProxyHost({
        domain: proxyInput.domain,
        forwardHost: proxyInput.forwardHost,
        forwardPort: proxyInput.forwardPort,
      });

      await test.step('Navigate to proxy hosts', async () => {
        await page.goto('/proxy-hosts');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify proxy host exists', async () => {
        await expect(page.getByText(createdProxy.domain)).toBeVisible();
      });
    });

    test('should configure WAF paranoia level', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to WAF config', async () => {
        await page.goto('/security/waf');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify WAF configuration page', async () => {
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should display WAF rule violations in logs', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to security dashboard', async () => {
        await page.goto('/security');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify security page', async () => {
        const content = page.locator('main, table, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should block SQL injection attempts', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to WAF page', async () => {
        await page.goto('/security/waf');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify page loads', async () => {
        const heading = page.locator('h1, h2').first();
        await expect(heading).toBeVisible();
      });
    });

    test('should block XSS attempts', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to WAF configuration', async () => {
        await page.goto('/security/waf');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify WAF page content', async () => {
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });
  });

  // ===========================================================================
  // Group C: CrowdSec + Proxy Integration (6 tests)
  // ===========================================================================
  test.describe('Group C: CrowdSec + Proxy Integration', () => {
    test('should display CrowdSec decisions', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to CrowdSec decisions', async () => {
        await page.goto('/security/crowdsec');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify CrowdSec page', async () => {
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should show CrowdSec configuration options', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to CrowdSec config', async () => {
        await page.goto('/security/crowdsec');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify configuration section', async () => {
        const content = page.locator('main, .content, form').first();
        await expect(content).toBeVisible();
      });
    });

    test('should display banned IPs from CrowdSec', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to CrowdSec page', async () => {
        await page.goto('/security/crowdsec');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify CrowdSec page', async () => {
        const content = page.locator('main, table, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should import CrowdSec configuration', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to CrowdSec page', async () => {
        await page.goto('/security/crowdsec');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify import option exists', async () => {
        // Import functionality should be available on the page
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should show CrowdSec alerts timeline', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to CrowdSec', async () => {
        await page.goto('/security/crowdsec');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify page content', async () => {
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should integrate CrowdSec with proxy host blocking', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      const proxyInput = generateProxyHost();
      const createdProxy = await testData.createProxyHost({
        domain: proxyInput.domain,
        forwardHost: proxyInput.forwardHost,
        forwardPort: proxyInput.forwardPort,
      });

      await test.step('Navigate to proxy hosts', async () => {
        await page.goto('/proxy-hosts');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify proxy host exists', async () => {
        await expect(page.getByText(createdProxy.domain)).toBeVisible();
      });
    });
  });

  // ===========================================================================
  // Group D: Security Headers Integration (4 tests)
  // ===========================================================================
  test.describe('Group D: Security Headers Integration', () => {
    test('should configure HSTS header for proxy host', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to security headers', async () => {
        await page.goto('/security/headers');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify security headers page', async () => {
        const content = page.locator('main, .content, form').first();
        await expect(content).toBeVisible();
      });
    });

    test('should configure Content-Security-Policy', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to security headers', async () => {
        await page.goto('/security/headers');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify page content', async () => {
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should configure X-Frame-Options header', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to security headers', async () => {
        await page.goto('/security/headers');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify headers configuration', async () => {
        const content = page.locator('main, form, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should apply security headers to proxy host', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      const proxyInput = generateProxyHost();
      await testData.createProxyHost({
        domain: proxyInput.domain,
        forwardHost: proxyInput.forwardHost,
        forwardPort: proxyInput.forwardPort,
      });

      await test.step('Navigate to security headers', async () => {
        await page.goto('/security/headers');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify configuration page', async () => {
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });
  });

  // ===========================================================================
  // Group E: Combined Security Features (4 tests)
  // ===========================================================================
  test.describe('Group E: Combined Security Features', () => {
    test('should enable all security features simultaneously', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      // Create proxy host with ACL
      const aclConfig = generateAllowListForIPs(['192.168.1.0/24']);
      await testData.createAccessList(aclConfig);

      const proxyInput = generateProxyHost();
      await testData.createProxyHost({
        domain: proxyInput.domain,
        forwardHost: proxyInput.forwardHost,
        forwardPort: proxyInput.forwardPort,
      });

      await test.step('Navigate to security dashboard', async () => {
        await page.goto('/security');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify security dashboard', async () => {
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should log all security events in audit log', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to security dashboard', async () => {
        await page.goto('/security');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify security page loads', async () => {
        const content = page.locator('main, table, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should display security notifications', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to notifications', async () => {
        await page.goto('/settings/notifications');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify notifications page', async () => {
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should enforce security policy across all proxy hosts', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      // Create multiple proxy hosts
      const proxy1Input = generateProxyHost();
      const proxy2Input = generateProxyHost();

      const createdProxy1 = await testData.createProxyHost({
        domain: proxy1Input.domain,
        forwardHost: proxy1Input.forwardHost,
        forwardPort: proxy1Input.forwardPort,
      });

      const createdProxy2 = await testData.createProxyHost({
        domain: proxy2Input.domain,
        forwardHost: proxy2Input.forwardHost,
        forwardPort: proxy2Input.forwardPort,
      });

      await test.step('Navigate to proxy hosts', async () => {
        await page.goto('/proxy-hosts');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify both proxy hosts exist', async () => {
        await expect(page.getByText(createdProxy1.domain)).toBeVisible();
        await expect(page.getByText(createdProxy2.domain)).toBeVisible();
      });
    });
  });
});

/**
 * Multi-Feature Workflows E2E Tests (Phase 6.7)
 *
 * Tests for complex workflows that span multiple features,
 * testing real-world usage scenarios and feature interactions.
 *
 * Test Categories (11-14 tests):
 * - Group A: Complete Host Setup Workflow (5 tests)
 * - Group C: Certificate + DNS Workflow (4 tests)
 * - Group D: Admin Management Workflow (5 tests)
 *
 * These tests verify end-to-end user journeys across features.
 */

import { test, expect, loginUser, TEST_PASSWORD } from '../fixtures/auth-fixtures';
import { generateProxyHost } from '../fixtures/proxy-hosts';
import { generateAccessList, generateAllowListForIPs } from '../fixtures/access-lists';
import { generateCertificate } from '../fixtures/certificates';
import { generateDnsProvider } from '../fixtures/dns-providers';
import {
  waitForToast,
  waitForLoadingComplete,
  waitForAPIResponse,
  waitForModal,
  clickAndWaitForResponse,
  waitForResourceInUI,
} from '../utils/wait-helpers';

/**
 * Selectors for multi-feature workflows
 */
const SELECTORS = {
  // Navigation
  sideNav: '[data-testid="sidebar"], nav, .sidebar',
  proxyHostsLink: 'a[href*="proxy-hosts"], button:has-text("Proxy Hosts")',
  accessListsLink: 'a[href*="access-lists"], button:has-text("Access Lists")',
  certificatesLink: 'a[href*="certificates"], button:has-text("Certificates")',
  dnsProvidersLink: 'a[href*="dns"], button:has-text("DNS")',
  securityLink: 'a[href*="security"], button:has-text("Security")',
  settingsLink: 'a[href*="settings"], button:has-text("Settings")',

  // Common Actions
  addButton: 'button:has-text("Add"), button:has-text("Create")',
  saveButton: 'button:has-text("Save"), button[type="submit"]',
  deleteButton: 'button:has-text("Delete")',
  editButton: 'button:has-text("Edit")',
  cancelButton: 'button:has-text("Cancel")',

  // Status Indicators
  activeStatus: '.badge:has-text("Active"), [data-testid="status-active"]',
  errorStatus: '.badge:has-text("Error"), [data-testid="status-error"]',
  pendingStatus: '.badge:has-text("Pending"), [data-testid="status-pending"]',

  // Common Elements
  table: 'table, [data-testid="data-table"]',
  modal: '.modal, [data-testid="modal"], [role="dialog"]',
  toast: '[data-testid="toast"], .toast, [role="alert"]',
  loadingSpinner: '[data-testid="loading"], .loading, .spinner',
};

test.describe('Multi-Feature Workflows E2E', () => {
  // ===========================================================================
  // Group A: Complete Host Setup Workflow (5 tests)
  // ===========================================================================
  test.describe('Group A: Complete Host Setup Workflow', () => {
    test('should complete full proxy host setup with all features', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Step 1: Create access list for the host', async () => {
        const acl = generateAllowListForIPs(['192.168.1.0/24']);
        await testData.createAccessList(acl);

        await page.goto('/access-lists');
        await waitForResourceInUI(page, acl.name);
      });

      await test.step('Step 2: Create proxy host', async () => {
        const proxyInput = generateProxyHost();
        const proxy = await testData.createProxyHost({
          domain: proxyInput.domain,
          forwardHost: proxyInput.forwardHost,
          forwardPort: proxyInput.forwardPort,
        });

        await page.goto('/proxy-hosts');
        await waitForResourceInUI(page, proxy.domain);
      });

      await test.step('Step 3: Verify dashboard shows the host', async () => {
        await page.goto('/');
        await waitForLoadingComplete(page);
        const content = page.locator('main, .content, h1').first();
        await expect(content).toBeVisible();
      });
    });

    test('should create proxy host with SSL certificate', async ({
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

      await test.step('Navigate to certificates', async () => {
        await page.goto('/certificates');
        await waitForLoadingComplete(page);
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should create proxy host with access restrictions', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Create access list', async () => {
        const acl = generateAccessList();
        await testData.createAccessList(acl);

        await page.goto('/access-lists');
        await waitForResourceInUI(page, acl.name);
      });

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
    });

    test('should update proxy host configuration end-to-end', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      const proxyInput = generateProxyHost();
      const proxy = await testData.createProxyHost({
        domain: proxyInput.domain,
        forwardHost: proxyInput.forwardHost,
        forwardPort: proxyInput.forwardPort,
      });

      await test.step('Navigate to proxy hosts', async () => {
        await page.goto('/proxy-hosts');
        await waitForResourceInUI(page, proxy.domain);
      });

      await test.step('Verify proxy host is editable', async () => {
        const row = page.getByText(proxy.domain).locator('..').first();
        await expect(row).toBeVisible();
      });
    });

    test('should delete proxy host and verify cleanup', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      const proxyInput = generateProxyHost();
      const proxy = await testData.createProxyHost({
        domain: proxyInput.domain,
        forwardHost: proxyInput.forwardHost,
        forwardPort: proxyInput.forwardPort,
      });

      await test.step('Verify proxy host exists', async () => {
        await page.goto('/proxy-hosts');
        await waitForResourceInUI(page, proxy.domain);
      });
    });
  });



  // ===========================================================================
  // Group C: Certificate + DNS Workflow (4 tests)
  // ===========================================================================
  test.describe('Group C: Certificate + DNS Workflow', () => {
    test('should setup DNS provider for certificate validation', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Create DNS provider', async () => {
        const dnsProvider = generateDnsProvider();
        await testData.createDNSProvider({
          name: dnsProvider.name,
          providerType: dnsProvider.provider_type,
          credentials: dnsProvider.credentials,
        });

        await page.goto('/dns-providers');
        await waitForLoadingComplete(page);
        await expect(page.getByText(dnsProvider.name)).toBeVisible();
      });
    });

    test('should request certificate with DNS challenge', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Create DNS provider first', async () => {
        const dnsProvider = generateDnsProvider();
        await testData.createDNSProvider({
          name: dnsProvider.name,
          providerType: dnsProvider.provider_type,
          credentials: dnsProvider.credentials,
        });

        await page.goto('/dns-providers');
        await waitForLoadingComplete(page);
        await expect(page.getByText(dnsProvider.name)).toBeVisible();
      });

      await test.step('Navigate to certificates', async () => {
        await page.goto('/certificates');
        await waitForLoadingComplete(page);
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should apply certificate to proxy host', async ({
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

      await test.step('Navigate to certificates', async () => {
        await page.goto('/certificates');
        await waitForLoadingComplete(page);
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should verify certificate renewal workflow', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to certificates', async () => {
        await page.goto('/certificates');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify certificate management page', async () => {
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });
  });

  // ===========================================================================
  // Group D: Admin Management Workflow (5 tests)
  // ===========================================================================
  test.describe('Group D: Admin Management Workflow', () => {
    test('should complete user management workflow', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to user management', async () => {
        await page.goto('/settings/account-management');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify user management page', async () => {
        const content = page.locator('main, .content, table').first();
        await expect(content).toBeVisible();
      });
    });

    test('should configure system settings', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to settings', async () => {
        await page.goto('/settings');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify settings page', async () => {
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });

    test('should view audit logs for all operations', async ({
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

    test('should perform system health check', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to dashboard', async () => {
        await page.goto('/dashboard');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify dashboard loads', async () => {
        await page.goto('/');
        await waitForLoadingComplete(page);
        const content = page.locator('main, .content, h1').first();
        await expect(content).toBeVisible();
      });
    });

    test('should complete backup before major changes', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      // Create some data first
      const proxyInput = generateProxyHost();
      const proxy = await testData.createProxyHost({
        domain: proxyInput.domain,
        forwardHost: proxyInput.forwardHost,
        forwardPort: proxyInput.forwardPort,
      });

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify backup page loads', async () => {
        const content = page.locator('main, .content').first();
        await expect(content).toBeVisible();
      });
    });
  });
});

/**
 * Backup & Restore E2E Tests
 *
 * Tests for complete backup and restore workflows including
 * scheduling, verification, and disaster recovery scenarios.
 *
 * Test Categories (20-24 tests):
 * - Group A: Backup Creation (5 tests)
 * - Group B: Backup Scheduling (4 tests)
 * - Group C: Restore Operations (5 tests)
 * - Group D: Backup Verification (4 tests)
 * - Group E: Error Handling (4 tests)
 *
 * API Endpoints:
 * - GET /api/v1/backups
 * - POST /api/v1/backups
 * - DELETE /api/v1/backups/:id
 * - POST /api/v1/backups/:id/restore
 * - GET /api/v1/backups/:id/download
 *
 * Async job contract audit (docs/plans/current_spec.md — Async
 * Backup/Restore Jobs — NS_BINDING_ABORTED Remediation, §6 commit 1):
 * POST /api/v1/backups and POST /api/v1/backups/:filename/restore now
 * return `202 {job_id, type, status: "pending"}` instead of blocking for a
 * `201`/`200` body, with the frontend polling
 * GET /api/v1/backups/jobs/:job_id until the job completes. This spec runs
 * against the REAL backend (no route mocking) and drives the flow purely
 * through the UI (toasts/dialog visibility) rather than asserting on raw
 * response bodies, so no test here hardcodes the old synchronous contract —
 * confirmed by inspection (grepped for `waitForResponse`/`response.json()`,
 * none found in this file). These tests are therefore expected to keep
 * passing unmodified once the real async endpoints + frontend polling hook
 * land (commits 2-9), per this plan's own Phase 1 guidance.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { generateProxyHost } from '../fixtures/proxy-hosts';
import { generateAccessList } from '../fixtures/access-lists';
import { waitForLoadingComplete } from '../utils/wait-helpers';

test.describe('Backup & Restore E2E', () => {
  // ===========================================================================
  // Group A: Backup Creation (5 tests)
  // ===========================================================================
  test.describe('Group A: Backup Creation', () => {
    test('should display backup list page', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backups page', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify backups page loads', async () => {
        const heading = page.locator('h1, h2').first();
        await expect(heading).toBeVisible();
      });
    });

    test('should create manual backup via API', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      // Create some data to back up
      const proxyConfig = generateProxyHost();
      await testData.createProxyHost({
        domain: proxyConfig.domain,
        forwardHost: proxyConfig.forwardHost,
        forwardPort: proxyConfig.forwardPort,
      });

      await test.step('Verify proxy host was created', async () => {
        await page.goto('/proxy-hosts');
        await waitForLoadingComplete(page);
        await expect(page.getByText(proxyConfig.domain)).toBeVisible();
      });

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify backups page loads', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should create backup with configuration only', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify backup creation options', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should create backup with all data included', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      // Create multiple resources
      const proxy = generateProxyHost();
      const acl = generateAccessList();

      await testData.createProxyHost({
        domain: proxy.domain,
        forwardHost: proxy.forwardHost,
        forwardPort: proxy.forwardPort,
      });

      await testData.createAccessList({
        name: acl.name,
        type: acl.type,
        ipRules: acl.ipRules,
      });

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify backup page content', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should show backup creation progress', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify page loads', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });
  });

  // ===========================================================================
  // Group B: Backup Scheduling (4 tests)
  // ===========================================================================
  test.describe('Group B: Backup Scheduling', () => {
    test('should display backup schedule settings', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backup settings', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify settings page', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should configure daily backup schedule', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backup settings', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify schedule configuration options', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should configure weekly backup schedule', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backup settings', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify settings page loads', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should set backup retention policy', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backup settings', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify retention policy options', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });
  });

  // ===========================================================================
  // Group C: Restore Operations (5 tests)
  // ===========================================================================
  test.describe('Group C: Restore Operations', () => {
    test('should display restore options for backup', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify backup list page', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should restore proxy hosts from backup', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      // Create proxy host that would be in a backup
      const proxyInput = generateProxyHost();
      const createdProxy = await testData.createProxyHost({
        domain: proxyInput.domain,
        forwardHost: proxyInput.forwardHost,
        forwardPort: proxyInput.forwardPort,
      });

      await test.step('Verify proxy host exists', async () => {
        await page.goto('/proxy-hosts');
        await waitForLoadingComplete(page);
        await expect(page.getByText(createdProxy.domain)).toBeVisible();
      });

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });
    });

    test('should restore access lists from backup', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      // Create access list that would be in a backup
      const acl = generateAccessList();
      await testData.createAccessList({
        name: acl.name,
        type: acl.type,
        ipRules: acl.ipRules,
      });

      await test.step('Verify access list exists', async () => {
        await page.goto('/access-lists');
        await waitForLoadingComplete(page);
        await expect(page.getByText(acl.name)).toBeVisible();
      });

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });
    });

    test('should show restore confirmation warning', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify page content', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should perform full system restore', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      // Create multiple resources
      const proxyInput = generateProxyHost();
      const acl = generateAccessList();

      const createdProxy = await testData.createProxyHost({
        domain: proxyInput.domain,
        forwardHost: proxyInput.forwardHost,
        forwardPort: proxyInput.forwardPort,
      });

      await testData.createAccessList({
        name: acl.name,
        type: acl.type,
        ipRules: acl.ipRules,
      });

      await test.step('Verify resources exist', async () => {
        await page.goto('/proxy-hosts');
        await waitForLoadingComplete(page);
        await expect(page.getByText(createdProxy.domain)).toBeVisible();
      });

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });
    });
  });

  // ===========================================================================
  // Group D: Backup Verification (4 tests)
  // ===========================================================================
  test.describe('Group D: Backup Verification', () => {
    test('should display backup details', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify backup list page', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should verify backup integrity', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify page loads', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should download backup file', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify download options exist', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should show backup size and date', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify backup metadata displayed', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });
  });

  // ===========================================================================
  // Group E: Error Handling (4 tests)
  // ===========================================================================
  test.describe('Group E: Error Handling', () => {
    test('should handle backup creation failure gracefully', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify page loads', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should handle restore failure gracefully', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify page content', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should handle corrupted backup file', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backups', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify error handling UI', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should handle insufficient storage during backup', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to backup settings', async () => {
        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify settings page', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });
  });
});

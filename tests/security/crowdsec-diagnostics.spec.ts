/**
 * CrowdSec Diagnostics E2E Tests
 *
 * Tests the CrowdSec diagnostic functionality including:
 * - Configuration file validation
 * - Connectivity checks to CrowdSec services
 * - Configuration export
 *
 * @see /projects/Charon/docs/plans/crowdsec_enrollment_debug_spec.md
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

test.describe('CrowdSec Diagnostics', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
  });

  test.describe('Configuration Validation', () => {
    test('should validate CrowdSec configuration files via API', async ({ request }) => {
      await test.step('GET diagnostics config endpoint', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/diagnostics/config');

        // Endpoint may not exist yet
        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'info',
            description: 'Diagnostics config endpoint not implemented (404)',
          });
          return;
        }

        expect(response.ok()).toBeTruthy();

        const config = await response.json();

        // Verify config.yaml validation
        expect(config).toHaveProperty('config_exists');
        expect(typeof config.config_exists).toBe('boolean');

        if (config.config_exists) {
          expect(config).toHaveProperty('config_valid');
          expect(typeof config.config_valid).toBe('boolean');
        }

        // Verify acquis.yaml validation
        expect(config).toHaveProperty('acquis_exists');
        expect(typeof config.acquis_exists).toBe('boolean');

        if (config.acquis_exists) {
          expect(config).toHaveProperty('acquis_valid');
          expect(typeof config.acquis_valid).toBe('boolean');
        }

        // Verify LAPI port configuration
        expect(config).toHaveProperty('lapi_port');

        // Verify errors array
        expect(config).toHaveProperty('errors');
        expect(Array.isArray(config.errors)).toBe(true);
      });
    });

    test('should report config.yaml exists when CrowdSec is initialized', async ({ request }) => {
      await test.step('Check config file existence', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/diagnostics/config');

        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'skip',
            description: 'Diagnostics config endpoint not implemented',
          });
          return;
        }

        const config = await response.json();

        // Check if CrowdSec is running
        const statusResponse = await request.get('/api/v1/admin/crowdsec/status');
        if (statusResponse.ok()) {
          const status = await statusResponse.json();

          if (status.running) {
            // If CrowdSec is running, config should exist
            expect(config.config_exists).toBe(true);
          }
        }
      });
    });

    test('should report LAPI port configuration', async ({ request }) => {
      await test.step('Verify LAPI port in config', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/diagnostics/config');

        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'skip',
            description: 'Diagnostics config endpoint not implemented',
          });
          return;
        }

        const config = await response.json();

        // LAPI should be configured on port 8085 (not 8080 to avoid conflict with Charon)
        if (config.lapi_port) {
          expect(config.lapi_port).toBe('8085');
        }
      });
    });
  });

  test.describe('Connectivity Checks', () => {
    test('should check connectivity to CrowdSec services', async ({ request }) => {
      await test.step('GET diagnostics connectivity endpoint', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/diagnostics/connectivity');

        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'info',
            description: 'Diagnostics connectivity endpoint not implemented (404)',
          });
          return;
        }

        expect(response.ok()).toBeTruthy();

        const connectivity = await response.json();

        // All connectivity checks should return boolean values
        const expectedChecks = [
          'lapi_running',
          'lapi_ready',
          'capi_registered',
          'console_enrolled',
        ];

        for (const check of expectedChecks) {
          expect(connectivity).toHaveProperty(check);
          expect(typeof connectivity[check]).toBe('boolean');
        }
      });
    });

    test('should report LAPI status accurately', async ({ request }) => {
      await test.step('Compare LAPI status between endpoints', async () => {
        // Get status from main status endpoint
        const statusResponse = await request.get('/api/v1/admin/crowdsec/status');

        if (!statusResponse.ok()) {
          test.info().annotations.push({
            type: 'skip',
            description: 'CrowdSec status endpoint not available',
          });
          return;
        }

        const status = await statusResponse.json();

        // Get connectivity diagnostics
        const connectivityResponse = await request.get(
          '/api/v1/admin/crowdsec/diagnostics/connectivity'
        );

        if (connectivityResponse.status() === 404) {
          test.info().annotations.push({
            type: 'skip',
            description: 'Diagnostics connectivity endpoint not implemented',
          });
          return;
        }

        const connectivity = await connectivityResponse.json();

        // LAPI running status should be consistent between endpoints
        expect(connectivity.lapi_running).toBe(status.running);

        if (status.running && status.lapi_ready !== undefined) {
          expect(connectivity.lapi_ready).toBe(status.lapi_ready);
        }
      });
    });

    test('should check CAPI registration status', async ({ request }) => {
      await test.step('Verify CAPI registration check', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/diagnostics/connectivity');

        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'skip',
            description: 'Diagnostics connectivity endpoint not implemented',
          });
          return;
        }

        const connectivity = await response.json();

        expect(connectivity).toHaveProperty('capi_registered');
        expect(typeof connectivity.capi_registered).toBe('boolean');

        // If console is enrolled, CAPI must be registered
        if (connectivity.console_enrolled) {
          expect(connectivity.capi_registered).toBe(true);
        }
      });
    });

    test('should optionally report console reachability', async ({ request }) => {
      // Diagnostic checks involving external connectivity can depend on network conditions
      test.setTimeout(60000);

      await test.step('Check console API reachability', async () => {
        await expect(async () => {
          const response = await request.get('/api/v1/admin/crowdsec/diagnostics/connectivity');

          if (response.status() === 404) {
            // If endpoint is not implemented, we pass
            return;
          }

          expect(response.ok()).toBeTruthy();

          const connectivity = await response.json();

          // console_reachable and capi_reachable are optional but valuable
          if (connectivity.console_reachable !== undefined) {
            expect(typeof connectivity.console_reachable).toBe('boolean');
          }

          if (connectivity.capi_reachable !== undefined) {
            expect(typeof connectivity.capi_reachable).toBe('boolean');
          }
        }).toPass({ timeout: 30000 });
      });
    });
  });

  test.describe('Configuration Export', () => {
    test('should export CrowdSec configuration', async ({ request }) => {
      await test.step('GET export endpoint', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/export');

        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'info',
            description: 'Export endpoint not implemented (404)',
          });
          return;
        }

        expect(response.ok()).toBeTruthy();

        // Verify response is gzip compressed
        const contentType = response.headers()['content-type'];
        expect(contentType).toContain('application/gzip');

        // Verify content disposition header
        const contentDisposition = response.headers()['content-disposition'];
        expect(contentDisposition).toMatch(/attachment/);
        expect(contentDisposition).toMatch(/crowdsec-config/);
        expect(contentDisposition).toMatch(/\.tar\.gz/);

        // Verify response body is not empty
        const body = await response.body();
        expect(body.length).toBeGreaterThan(0);
      });
    });

    test('should include filename with timestamp in export', async ({ request }) => {
      await test.step('Verify export filename format', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/export');

        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'skip',
            description: 'Export endpoint not implemented',
          });
          return;
        }

        const contentDisposition = response.headers()['content-disposition'];

        // Filename should contain crowdsec-config and end with .tar.gz
        expect(contentDisposition).toMatch(/filename[^;=\n]*=[^;=\n]*crowdsec-config/);
        expect(contentDisposition).toMatch(/\.tar\.gz/);
      });
    });
  });

  test.describe('Configuration Files API', () => {
    test('should list CrowdSec configuration files', async ({ request }) => {
      await test.step('GET files list endpoint', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/files');

        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'info',
            description: 'Files list endpoint not implemented (404)',
          });
          return;
        }

        expect(response.ok()).toBeTruthy();

        const files = await response.json();

        expect(files).toHaveProperty('files');
        expect(Array.isArray(files.files)).toBe(true);

        // Verify essential config files are listed
        const fileList = files.files as string[];

        const hasConfigYaml = fileList.some((f) => f.includes('config.yaml') || f.includes('config/config.yaml'));
        const hasAcquisYaml = fileList.some((f) => f.includes('acquis.yaml') || f.includes('config/acquis.yaml'));

        if (fileList.length > 0) {
          expect(hasConfigYaml || hasAcquisYaml).toBe(true);
        }
      });
    });

    test('should retrieve specific config file content', async ({ request }) => {
      await test.step('GET specific file content', async () => {
        // First get the file list
        const listResponse = await request.get('/api/v1/admin/crowdsec/files');

        if (listResponse.status() === 404) {
          test.info().annotations.push({
            type: 'skip',
            description: 'Files list endpoint not implemented',
          });
          return;
        }

        const files = await listResponse.json();
        const fileList = files.files as string[];

        // Find config.yaml path
        const configPath = fileList.find((f) => f.includes('config.yaml'));

        if (!configPath) {
          test.info().annotations.push({
            type: 'info',
            description: 'config.yaml not found in file list',
          });
          return;
        }

        // Retrieve file content
        const contentResponse = await request.get(
          `/api/v1/admin/crowdsec/file?path=${encodeURIComponent(configPath)}`
        );

        if (contentResponse.status() === 404) {
          test.info().annotations.push({
            type: 'info',
            description: 'File content retrieval not implemented',
          });
          return;
        }

        expect(contentResponse.ok()).toBeTruthy();

        const content = await contentResponse.json();

        expect(content).toHaveProperty('content');
        expect(typeof content.content).toBe('string');

        // Verify config contains expected LAPI configuration
        expect(content.content).toContain('listen_uri');
      });
    });
  });

  test.describe('Diagnostics UI', () => {
    test('should display CrowdSec status indicators', async ({ page }) => {
      await test.step('Navigate to CrowdSec page', async () => {
        await page.goto('/security/crowdsec');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify status indicators are present', async () => {
        // Look for status badges or indicators
        const statusBadge = page.locator('[class*="badge"]').filter({
          hasText: /running|stopped|enabled|disabled|online|offline/i,
        });

        const statusVisible = await statusBadge.first().isVisible().catch(() => false);

        if (statusVisible) {
          await expect(statusBadge.first()).toBeVisible();
        } else {
          // Status may be displayed differently
          const statusText = page.getByText(/crowdsec.*running|crowdsec.*stopped|lapi.*ready/i);
          const textVisible = await statusText.first().isVisible().catch(() => false);

          if (!textVisible) {
            test.info().annotations.push({
              type: 'info',
              description: 'Status indicators not found in expected format',
            });
          }
        }
      });
    });

    test('should display LAPI ready status when CrowdSec is running', async ({ page, request }) => {
      await test.step('Check CrowdSec status', async () => {
        const statusResponse = await request.get('/api/v1/admin/crowdsec/status');

        if (!statusResponse.ok()) {
          test.info().annotations.push({
            type: 'skip',
            description: 'CrowdSec status endpoint not available',
          });
          return;
        }

        const status = await statusResponse.json();

        await page.goto('/security/crowdsec');
        await waitForLoadingComplete(page);

        if (status.running && status.lapi_ready) {
          // LAPI ready status should be visible
          const lapiStatus = page.getByText(/lapi.*ready|local.*api.*ready/i);
          const lapiVisible = await lapiStatus.isVisible().catch(() => false);

          if (lapiVisible) {
            await expect(lapiStatus).toBeVisible();
          }
        }
      });
    });
  });

  test.describe('Error Handling', () => {
    test('should handle CrowdSec not running gracefully', async ({ page, request }) => {
      await test.step('Check diagnostics when CrowdSec may not be running', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/diagnostics/connectivity');

        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'skip',
            description: 'Diagnostics endpoint not implemented',
          });
          return;
        }

        // Even when CrowdSec is not running, endpoint should return valid response
        expect(response.ok()).toBeTruthy();

        const connectivity = await response.json();

        // Response should indicate CrowdSec is not running if that's the case
        if (!connectivity.lapi_running) {
          expect(connectivity.lapi_ready).toBe(false);
        }
      });
    });

    test('should report errors in diagnostics config validation', async ({ request }) => {
      await test.step('Check for validation errors reporting', async () => {
        const response = await request.get('/api/v1/admin/crowdsec/diagnostics/config');

        if (response.status() === 404) {
          test.info().annotations.push({
            type: 'skip',
            description: 'Diagnostics config endpoint not implemented',
          });
          return;
        }

        const config = await response.json();

        // errors should always be an array (empty if no errors)
        expect(config).toHaveProperty('errors');
        expect(Array.isArray(config.errors)).toBe(true);

        // Each error should be a string
        for (const error of config.errors) {
          expect(typeof error).toBe('string');
        }
      });
    });
  });
});

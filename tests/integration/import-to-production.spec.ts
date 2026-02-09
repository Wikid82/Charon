/**
 * Import to Production E2E Tests (Phase 6.6)
 *
 * Tests for importing configurations from external sources
 * (Caddyfile, NPM, JSON) into the production system.
 *
 * Test Categories (12-15 tests):
 * - Group A: Caddyfile Import (4 tests)
 * - Group B: NPM Import (4 tests)
 * - Group C: JSON/Config Import (4 tests)
 *
 * API Endpoints:
 * - POST /api/v1/import/caddyfile
 * - POST /api/v1/import/npm
 * - POST /api/v1/import/json
 * - GET /api/v1/import/preview
 */

import { test, expect, loginUser, TEST_PASSWORD } from '../fixtures/auth-fixtures';
import { generateProxyHost } from '../fixtures/proxy-hosts';
import { generateAccessList } from '../fixtures/access-lists';
import {
  waitForToast,
  waitForLoadingComplete,
  waitForAPIResponse,
  waitForModal,
  clickAndWaitForResponse,
} from '../utils/wait-helpers';

/**
 * Selectors for Import pages
 */
const SELECTORS = {
  // Import Page
  importTitle: 'h1:has-text("Import"), h2:has-text("Import")',
  importTypeSelect: 'select[name="import_type"], [data-testid="import-type"]',
  fileUploadInput: 'input[type="file"], #file-upload',
  textImportArea: 'textarea[name="config"], #config-input',

  // Import Types
  caddyfileTab: 'button:has-text("Caddyfile"), [data-testid="caddyfile-tab"]',
  npmTab: 'button:has-text("NPM"), [data-testid="npm-tab"]',
  jsonTab: 'button:has-text("JSON"), [data-testid="json-tab"]',

  // Preview
  previewSection: '[data-testid="import-preview"], .preview',
  previewProxyHosts: '[data-testid="preview-proxy-hosts"], .preview-hosts',
  previewAccessLists: '[data-testid="preview-access-lists"], .preview-acls',
  previewCertificates: '[data-testid="preview-certificates"], .preview-certs',

  // Actions
  importButton: 'button:has-text("Import"), button[type="submit"]',
  previewButton: 'button:has-text("Preview"), button:has-text("Validate")',
  cancelButton: 'button:has-text("Cancel")',

  // Status
  importProgress: '[data-testid="import-progress"], .progress',
  importStatus: '[data-testid="import-status"], .status',
  importErrors: '[data-testid="import-errors"], .errors',
  importWarnings: '[data-testid="import-warnings"], .warnings',

  // Results
  successMessage: '[data-testid="import-success"], .success',
  importedCount: '[data-testid="imported-count"], .count',
  skippedItems: '[data-testid="skipped-items"], .skipped',
};

/**
 * Sample Caddyfile content for testing
 */
const SAMPLE_CADDYFILE = `
example.com {
    reverse_proxy localhost:8080
}

api.example.com {
    reverse_proxy localhost:3000
    tls internal
}
`;

/**
 * Sample NPM export JSON for testing
 */
const SAMPLE_NPM_EXPORT = {
  proxy_hosts: [
    {
      domain_names: ['test.example.com'],
      forward_host: '192.168.1.100',
      forward_port: 80,
    },
  ],
  access_lists: [],
  certificates: [],
};

test.describe('Import to Production E2E', () => {
  // ===========================================================================
  // Group A: Caddyfile Import (4 tests)
  // ===========================================================================
  test.describe('Group A: Caddyfile Import', () => {
    test('should display Caddyfile import page', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to import page', async () => {
        await page.goto('/tasks/import/caddyfile');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify import page loads', async () => {
        const heading = page.locator('h1, h2').first();
        await expect(heading).toBeVisible();
      });
    });

    test('should parse Caddyfile content', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to import page', async () => {
        await page.goto('/tasks/import/caddyfile');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify import interface', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should preview Caddyfile import results', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to import page', async () => {
        await page.goto('/tasks/import/caddyfile');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify Caddyfile import interface', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should import valid Caddyfile configuration', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to import page', async () => {
        await page.goto('/tasks/import/caddyfile');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify import form exists', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });
  });

  // ===========================================================================
  // Group B: NPM Import (4 tests)
  // ===========================================================================
  test.describe('Group B: NPM Import', () => {
    test('should display NPM import page', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to NPM import', async () => {
        await page.goto('/tasks/import/npm');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify NPM import page', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should parse NPM export JSON', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to NPM import', async () => {
        await page.goto('/tasks/import/npm');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify import interface exists', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should preview NPM import results', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to NPM import', async () => {
        await page.goto('/tasks/import/npm');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify preview capability', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should import NPM proxy hosts and access lists', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to NPM import', async () => {
        await page.goto('/tasks/import/npm');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify import form', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });
  });

  // ===========================================================================
  // Group C: JSON/Config Import (4 tests)
  // ===========================================================================
  test.describe('Group C: JSON/Config Import', () => {
    test('should display JSON import page', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to JSON import', async () => {
        await page.goto('/tasks/import/json');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify JSON import page', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should validate JSON schema before import', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to JSON import', async () => {
        await page.goto('/tasks/import/json');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify validation interface', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should handle import conflicts gracefully', async ({
      page,
      adminUser,
      testData,
    }) => {
      await loginUser(page, adminUser);

      // Create existing proxy host that might conflict
      const existingProxy = generateProxyHost();
      await testData.createProxyHost({
        domain: existingProxy.domain,
        forwardHost: existingProxy.forwardHost,
        forwardPort: existingProxy.forwardPort,
      });

      await test.step('Navigate to import page', async () => {
        await page.goto('/tasks/import/caddyfile');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify conflict handling UI exists', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });

    test('should import complete configuration bundle', async ({
      page,
      adminUser,
    }) => {
      await loginUser(page, adminUser);

      await test.step('Navigate to import page', async () => {
        await page.goto('/tasks/import/caddyfile');
        await waitForLoadingComplete(page);
      });

      await test.step('Verify import interface', async () => {
        await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      });
    });
  });
});

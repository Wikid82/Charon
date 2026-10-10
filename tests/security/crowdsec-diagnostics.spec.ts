/**
 * CrowdSec Diagnostics E2E Tests
 *
 * The diagnostics, export and file endpoints are served by the real backend, which has no
 * CrowdSec engine in the E2E container (no cscli, nothing running). Each test first imports
 * a known configuration archive so the files on disk are fixed, then asserts the exact
 * response the backend produces for that state:
 * - Configuration validation (diagnostics/config)
 * - Connectivity checks (diagnostics/connectivity)
 * - Configuration export
 * - Configuration file listing and reading
 *
 * The Security page is the only UI that reports the CrowdSec process state; its running and
 * stopped indicators are covered with a stubbed status endpoint.
 *
 * @see /projects/Charon/docs/plans/crowdsec_enrollment_debug_spec.md
 */

import { gunzipSync } from 'zlib';
import type { APIRequestContext } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { createTarGz } from '../utils/archive-helpers';
import { stubCrowdSecApi, crowdsecFixtures } from '../utils/crowdsec-stubs';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import { promises as fs } from 'fs';

const ADMIN = '/api/v1/admin/crowdsec';
const CONFIG_YAML = `api:
  server:
    listen_uri: 127.0.0.1:8085
`;
const ACQUIS_YAML = `source: file
filenames:
  - /var/log/e2e.log
labels:
  type: syslog
`;

/** Replaces the CrowdSec configuration with a known one, so every test starts from the same files. */
async function importKnownConfig(
  request: APIRequestContext,
  archivePath: string,
  files: Record<string, string> = { 'config.yaml': CONFIG_YAML, 'acquis.yaml': ACQUIS_YAML },
) {
  await createTarGz(files, archivePath);
  const response = await request.post(`${ADMIN}/import`, {
    multipart: {
      file: { name: 'known.tar.gz', mimeType: 'application/gzip', buffer: await fs.readFile(archivePath) },
    },
  });
  expect(response.status()).toBe(200);
}

test.describe('CrowdSec Diagnostics', () => {
  test.beforeEach(async ({ page, adminUser, request }, testInfo) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await importKnownConfig(request, testInfo.outputPath('known.tar.gz'));
  });

  test.describe('Configuration Validation', () => {
    test('should report both files present and the configured LAPI port', async ({ request }) => {
      const response = await request.get(`${ADMIN}/diagnostics/config`);
      expect(response.status()).toBe(200);
      const config = await response.json();

      expect(config).toMatchObject({
        config_exists: true,
        acquis_exists: true,
        acquis_valid: true,
        lapi_port: '8085',
      });
    });

    test('should report config.yaml invalid without the CrowdSec CLI, naming the failure', async ({ request }) => {
      const response = await request.get(`${ADMIN}/diagnostics/config`);
      expect(response.status()).toBe(200);
      const config = await response.json();

      // The E2E container has no cscli, so `cscli config check` cannot succeed.
      expect(config.config_valid).toBe(false);
      expect(config.errors).toHaveLength(1);
      expect(config.errors[0]).toMatch(/^config\.yaml validation failed/);
    });

    test('should report missing files and an empty LAPI port when only config.yaml is imported', async ({ request }, testInfo) => {
      await importKnownConfig(request, testInfo.outputPath('config-only.tar.gz'), {
        'config.yaml': 'api:\n  server:\n    listen_uri: 0.0.0.0:8080\n',
      });

      const response = await request.get(`${ADMIN}/diagnostics/config`);
      expect(response.status()).toBe(200);
      const config = await response.json();

      expect(config).toMatchObject({ config_exists: true, acquis_exists: false, acquis_valid: false, lapi_port: '' });
      expect(config.errors).toContain('acquis.yaml not found');
    });

    test('should report an acquisition file without a datasource as invalid', async ({ request }, testInfo) => {
      await importKnownConfig(request, testInfo.outputPath('bad-acquis.tar.gz'), {
        'config.yaml': CONFIG_YAML,
        'acquis.yaml': 'labels:\n  type: syslog\n',
      });

      const response = await request.get(`${ADMIN}/diagnostics/config`);
      expect(response.status()).toBe(200);
      const config = await response.json();

      expect(config).toMatchObject({ acquis_exists: true, acquis_valid: false });
      expect(config.errors).toContain(
        "acquis.yaml missing datasource configuration (expected 'source:' and 'filenames:' or 'filename:')",
      );
    });
  });

  test.describe('Connectivity Checks', () => {
    test('should report every connectivity check as a boolean', async ({ request }) => {
      const response = await request.get(`${ADMIN}/diagnostics/connectivity`);
      expect(response.status()).toBe(200);
      const connectivity = await response.json();

      for (const check of [
        'lapi_running',
        'lapi_ready',
        'capi_registered',
        'capi_reachable',
        'console_enrolled',
        'console_reachable',
      ]) {
        expect(typeof connectivity[check], check).toBe('boolean');
      }
    });

    test('should report CrowdSec stopped and LAPI not ready when no engine is running', async ({ request }) => {
      const connectivity = await (await request.get(`${ADMIN}/diagnostics/connectivity`)).json();

      expect(connectivity.lapi_running).toBe(false);
      expect(connectivity.lapi_ready).toBe(false);
    });

    test('should agree with the status endpoint on whether CrowdSec is running', async ({ request }) => {
      const statusResponse = await request.get(`${ADMIN}/status`);
      expect(statusResponse.status()).toBe(200);
      const status = await statusResponse.json();
      const connectivity = await (await request.get(`${ADMIN}/diagnostics/connectivity`)).json();

      expect(connectivity.lapi_running).toBe(status.running);
      expect(connectivity.lapi_ready).toBe(status.lapi_ready);
    });

    // Import treats the account files as server-local state: it never deletes the live
    // online_api_credentials.yaml and never takes one from an archive, so the CAPI registration the
    // diagnostics report is the same before and after any import.
    test('should still report CAPI as registered after importing an archive without the online credentials file', async ({ request }) => {
      // The E2E image ships online_api_credentials.yaml, and the beforeEach import carries none.
      const connectivity = await (await request.get(`${ADMIN}/diagnostics/connectivity`)).json();

      expect(connectivity.capi_registered).toBe(true);
    });

    test('should keep the CAPI registration unchanged when the imported archive carries an online credentials file', async ({ request }, testInfo) => {
      const registeredBefore = (await (await request.get(`${ADMIN}/diagnostics/connectivity`)).json()).capi_registered;

      await importKnownConfig(request, testInfo.outputPath('with-capi.tar.gz'), {
        'config.yaml': CONFIG_YAML,
        'acquis.yaml': ACQUIS_YAML,
        'online_api_credentials.yaml': 'url: https://api.crowdsec.net/\nlogin: e2e\npassword: e2e\n',
      });

      const connectivity = await (await request.get(`${ADMIN}/diagnostics/connectivity`)).json();

      expect(connectivity.capi_registered).toBe(registeredBefore);
    });
  });

  test.describe('Configuration Export', () => {
    test('should export the configuration as a timestamped gzip archive containing the imported files', async ({ request }) => {
      const response = await request.get(`${ADMIN}/export`);
      expect(response.status()).toBe(200);

      expect(response.headers()['content-type']).toContain('application/gzip');
      expect(response.headers()['content-disposition']).toMatch(
        /^attachment; filename=crowdsec-config-\d{8}-\d{6}\.tar\.gz$/,
      );

      const archive = gunzipSync(await response.body()).toString('latin1');
      expect(archive).toContain('config.yaml');
      expect(archive).toContain('acquis.yaml');
      expect(archive).toContain('listen_uri: 127.0.0.1:8085');
    });

    test('should export a single gzip layer to clients that accept compressed responses', async ({ request }) => {
      const response = await request.get(`${ADMIN}/export`, { headers: { 'Accept-Encoding': 'gzip' } });
      expect(response.status()).toBe(200);

      // One gunzip must yield a tar stream (ustar magic at offset 257), not another gzip layer.
      const tar = gunzipSync(await response.body());
      expect(tar.subarray(0, 2)).not.toEqual(Buffer.from([0x1f, 0x8b]));
      expect(tar.subarray(257, 262).toString('latin1')).toBe('ustar');
      expect(tar.toString('latin1')).toContain('config.yaml');
    });
  });

  test.describe('Configuration Files API', () => {
    test('should list the imported configuration files', async ({ request }) => {
      const response = await request.get(`${ADMIN}/files`);
      expect(response.status()).toBe(200);
      const { files } = await response.json();

      expect(files).toEqual(expect.arrayContaining(['config.yaml', 'acquis.yaml']));
    });

    test('should return the content of config.yaml', async ({ request }) => {
      const response = await request.get(`${ADMIN}/file?path=${encodeURIComponent('config.yaml')}`);
      expect(response.status()).toBe(200);

      expect((await response.json()).content).toBe(CONFIG_YAML);
    });

    test('should reject reading a file outside the configuration directory', async ({ request }) => {
      const response = await request.get(`${ADMIN}/file?path=${encodeURIComponent('../../etc/passwd')}`);

      expect(response.status()).toBe(400);
      expect((await response.json()).error).toBe('invalid path');
    });
  });

  test.describe('Diagnostics UI', () => {
    test('should show the CrowdSec process as running with its PID', async ({ page }) => {
      await stubCrowdSecApi(page, { status: crowdsecFixtures.runningStatus({ pid: 4321 }) });
      await page.goto('/security');
      await waitForLoadingComplete(page);

      await expect(page.getByText('Running (PID 4321)', { exact: true })).toBeVisible();
      await expect(page.getByTestId('toggle-crowdsec')).toBeChecked();
    });

    test('should show the CrowdSec process as stopped', async ({ page }) => {
      await stubCrowdSecApi(page, { status: crowdsecFixtures.runningStatus({ running: false, pid: 0, lapi_ready: false }) });
      await page.goto('/security');
      await waitForLoadingComplete(page);

      await expect(page.getByText('Process stopped', { exact: true })).toBeVisible();
      await expect(page.getByTestId('toggle-crowdsec')).not.toBeChecked();
    });
  });

  test.describe('Error Handling', () => {
    test('should still answer diagnostics with a complete report when CrowdSec is not running', async ({ request }) => {
      const response = await request.get(`${ADMIN}/diagnostics/connectivity`);

      expect(response.status()).toBe(200);
      const connectivity = await response.json();
      expect(connectivity.lapi_running).toBe(false);
      expect(connectivity.lapi_ready).toBe(false);
    });

    test('should list every missing file as a string error', async ({ request }, testInfo) => {
      await importKnownConfig(request, testInfo.outputPath('empty-acquis.tar.gz'), {
        'config.yaml': CONFIG_YAML,
      });

      const config = await (await request.get(`${ADMIN}/diagnostics/config`)).json();

      expect(config.errors).toEqual([
        expect.stringMatching(/^config\.yaml validation failed/),
        'acquis.yaml not found',
      ]);
    });
  });
});

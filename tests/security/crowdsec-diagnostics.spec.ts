/**
 * CrowdSec Diagnostics E2E Tests
 *
 * The diagnostics, export and file endpoints are served by the real backend, which has no
 * CrowdSec engine in the E2E container (cscli is present, but no engine is running). Each test first imports
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
import { restoreShippedCrowdsecConfig, SHIPPED_CROWDSEC_FILES } from '../utils/crowdsec-shipped-config';
import { promises as fs } from 'fs';

const ADMIN = '/api/v1/admin/crowdsec';
/**
 * A configuration the engine can load (the import check refuses one without config_paths). It is the shipped
 * file, which listens on 127.0.0.1:8085, so the LAPI port the diagnostics report is known.
 */
const CONFIG_YAML = SHIPPED_CROWDSEC_FILES['config/config.yaml'];
/** The shipped configuration listening on every interface, which the diagnostics report has no LAPI port for. */
const ALL_INTERFACES_CONFIG_YAML = CONFIG_YAML.replace('listen_uri: 127.0.0.1:8085', 'listen_uri: 0.0.0.0:8080');
const ACQUIS_YAML = `source: file
filenames:
  - /var/log/e2e.log
labels:
  type: syslog
`;
const NO_DATASOURCE_ACQUIS_YAML = 'labels:\n  type: syslog\n';
const ACQUIS_NO_DATASOURCE_ERROR =
  "acquis.yaml missing datasource configuration (expected 'source:' and 'filenames:' or 'filename:')";

/** Uploads an archive built from `files` to the import endpoint and returns the response. */
async function postArchive(request: APIRequestContext, archivePath: string, files: Record<string, string>) {
  await createTarGz(files, archivePath);
  return request.post(`${ADMIN}/import`, {
    multipart: {
      file: { name: 'known.tar.gz', mimeType: 'application/gzip', buffer: await fs.readFile(archivePath) },
    },
    // The import runs the engine's configuration check, which can take up to its own 30 second bound.
    timeout: 120_000,
  });
}

/** Saves one configuration file through the editor endpoint and expects it to be accepted. */
async function saveConfigFile(request: APIRequestContext, path: string, content: string) {
  const response = await request.post(`${ADMIN}/file`, { data: { path, content } });
  expect(response.status(), await response.text()).toBe(200);
}

/** Replaces the CrowdSec configuration with a known one, so every test starts from the same files. */
async function importKnownConfig(
  request: APIRequestContext,
  archivePath: string,
  files: Record<string, string> = { 'config/config.yaml': CONFIG_YAML, 'config/acquis.yaml': ACQUIS_YAML },
) {
  const response = await postArchive(request, archivePath, files);
  expect(response.status(), await response.text()).toBe(200);
}

// Every case imports a known configuration; the shipped one goes back afterwards so later specs can start the engine.
test.afterEach(async ({ request }) => {
  await restoreShippedCrowdsecConfig(request);
});

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

    test('should refuse an incomplete config.yaml on import and keep the loadable configuration in place', async ({ request }, testInfo) => {
      // A minimal file without a config_paths section is refused before it can replace the live configuration.
      const response = await postArchive(request, testInfo.outputPath('incomplete.tar.gz'), {
        'config/config.yaml': 'api:\n  server:\n    listen_uri: 127.0.0.1:8085\n',
      });
      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe('config validation failed: config.yaml has no config_paths section');

      const config = await (await request.get(`${ADMIN}/diagnostics/config`)).json();
      expect(config).toMatchObject({ config_exists: true, lapi_port: '8085' });
      const file = await request.get(`${ADMIN}/file?path=${encodeURIComponent('config/config.yaml')}`);
      expect((await file.json()).content).toBe(CONFIG_YAML);
    });

    test('should keep the current acquisition file and report an empty LAPI port when only config.yaml is imported', async ({ request }, testInfo) => {
      await importKnownConfig(request, testInfo.outputPath('config-only.tar.gz'), {
        'config/config.yaml': ALL_INTERFACES_CONFIG_YAML,
      });

      const response = await request.get(`${ADMIN}/diagnostics/config`);
      expect(response.status()).toBe(200);
      const config = await response.json();

      // The archive has no acquis.yaml, so the current one (from the known configuration) is kept.
      expect(config).toMatchObject({ config_exists: true, acquis_exists: true, acquis_valid: true, lapi_port: '' });
      expect(config.errors).not.toContain('acquis.yaml not found');
      const acquis = await request.get(`${ADMIN}/file?path=${encodeURIComponent('config/acquis.yaml')}`);
      expect((await acquis.json()).content).toBe(ACQUIS_YAML);
    });

    test('should report an acquisition file without a datasource as invalid', async ({ request }) => {
      // An import refuses this file, so it is saved through the editor, which checks YAML syntax only.
      await saveConfigFile(request, 'config/acquis.yaml', NO_DATASOURCE_ACQUIS_YAML);

      const response = await request.get(`${ADMIN}/diagnostics/config`);
      expect(response.status()).toBe(200);
      const config = await response.json();

      expect(config).toMatchObject({ acquis_exists: true, acquis_valid: false });
      expect(config.errors).toContain(ACQUIS_NO_DATASOURCE_ERROR);
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
        'config/config.yaml': CONFIG_YAML,
        'config/acquis.yaml': ACQUIS_YAML,
        'config/online_api_credentials.yaml': 'url: https://api.crowdsec.net/\nlogin: e2e\npassword: e2e\n',
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

      expect(files).toEqual(expect.arrayContaining(['config/config.yaml', 'config/acquis.yaml']));
    });

    test('should return the content of config.yaml', async ({ request }) => {
      const response = await request.get(`${ADMIN}/file?path=${encodeURIComponent('config/config.yaml')}`);
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

    test('should list each problem as a string error', async ({ request }) => {
      // The loadable config.yaml is valid, so the acquisition file is the only problem left to list.
      await saveConfigFile(request, 'config/acquis.yaml', NO_DATASOURCE_ACQUIS_YAML);

      const config = await (await request.get(`${ADMIN}/diagnostics/config`)).json();

      expect(config.errors).toEqual([ACQUIS_NO_DATASOURCE_ERROR]);
    });
  });
});

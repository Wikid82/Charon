/**
 * CrowdSec Config Import E2E Tests
 *
 * The import endpoint is served by the real backend. Each case uploads a generated archive
 * and asserts the exact status and message the backend answers with:
 * - Accepted archives (required file only, with optional files)
 * - Rejected archives (missing config.yaml, unusable config.yaml, wrong format, corrupted,
 *   oversized, high compression ratio, path traversal)
 * - Rollback: a rejected import leaves the previous configuration untouched
 *
 * The result of an import is read back through the configuration file API.
 *
 * Every archive uses the real layout: a CrowdSec installation reads its configuration from the
 * config/ folder, and so does every export. cfg() builds the path the file API lists and reads.
 */

import { promises as fs } from 'fs';
import type { APIRequestContext } from '@playwright/test';
import { test, expect } from '@playwright/test';
import {
  createCorruptedArchive,
  createOversizedArchive,
  createRawTarGz,
  createTarGz,
  createZip,
  createZipBomb,
  HUB_MANIFEST_NAME,
  hubManifest,
  listTarGzEntries,
  notificationYaml,
  profilesYaml,
  readTarGzEntry,
} from '../utils/archive-helpers';
import { restoreShippedCrowdsecConfig, SHIPPED_CROWDSEC_FILES } from '../utils/crowdsec-shipped-config';

const ADMIN = '/api/v1/admin/crowdsec';
const MAX_ARCHIVE_BYTES = 50 * 1024 * 1024;

/**
 * A configuration the engine can load. The import check runs the engine on the archive's config.yaml, and a
 * minimal file is refused ("no database configuration provided", "no configuration paths"), so every valid
 * fixture starts from the shipped configuration.
 */
const ENGINE_LOADABLE_CONFIG = SHIPPED_CROWDSEC_FILES['config/config.yaml'];
const VALID_CONFIG = ENGINE_LOADABLE_CONFIG;
const ACQUIS = `filenames:
  - /var/log/nginx/access.log
  - /var/log/auth.log
labels:
  type: syslog
`;
/** A configuration that is recognisable after a failed import. */
const BASELINE_CONFIG = `${ENGINE_LOADABLE_CONFIG}# baseline-marker
`;
const BASELINE_ACQUIS = `source: file
filenames:
  - /var/log/baseline.log
labels:
  type: syslog
`;
/** The baseline carries the three companion files so every test starts from the same file set. */
const BASELINE_PROFILES = `name: default_ip_remediation
filters:
  - Alert.Remediation == true && Alert.GetScope() == "Ip"
decisions:
  - type: ban
    duration: 4h
notifications: []
on_success: break
`;
const BASELINE_SIMULATION = `simulation: false
# baseline-simulation
`;
/** The refusal message for an archive that keeps its configuration at the top level. */
const FLAT_ARCHIVE_MESSAGE =
  'the archive keeps its configuration at the top level, but this CrowdSec installation reads it from the config folder; export a configuration from this page, or put the files in a config/ folder inside the archive';

/** English text of the refusal shown when CrowdSec cannot load an imported configuration. */
const IMPORT_CONFIG_INVALID_MESSAGE_EN =
  'CrowdSec could not load the imported configuration. Check that config.yaml has a config_paths section and that the folders it names exist.';

/** The path of a configuration file as the file API lists and reads it. */
const cfg = (name: string) => `config/${name}`;

/** Uploads an archive file under the given upload name. */
async function upload(request: APIRequestContext, archivePath: string, uploadName: string, mimeType = 'application/gzip') {
  return request.post(`${ADMIN}/import`, {
    multipart: { file: { name: uploadName, mimeType, buffer: await fs.readFile(archivePath) } },
    timeout: 120_000,
  });
}

async function readConfigFile(request: APIRequestContext, name: string) {
  const response = await request.get(`${ADMIN}/file?path=${encodeURIComponent(name)}`);
  expect(response.status()).toBe(200);
  return (await response.json()).content as string;
}

/** The kept companion names, without key files a previous spec left in the instance (those are listed by path). */
function keptCompanions(kept: string[] | undefined): string[] {
  return (kept ?? []).filter((name) => !name.includes('/'));
}

// Most cases replace config.yaml with a minimal file the engine cannot start from. The shipped
// configuration goes back after every case, so later specs that start the engine find a valid one.
test.afterEach(async ({ request }) => {
  await restoreShippedCrowdsecConfig(request);
});

test.describe('CrowdSec Config Import Validation', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const baseline = await createTarGz(
      {
        'config/config.yaml': BASELINE_CONFIG,
        'config/acquis.yaml': BASELINE_ACQUIS,
        'config/profiles.yaml': BASELINE_PROFILES,
        'config/simulation.yaml': BASELINE_SIMULATION,
      },
      testInfo.outputPath('baseline.tar.gz'),
    );
    const response = await upload(request, baseline, 'baseline.tar.gz');
    expect(response.status()).toBe(200);
  });

  test.describe('Accepted Archives', () => {
    test('should import a valid archive and replace the configuration', async ({ request }, testInfo) => {
      const archive = await createTarGz({ 'config/config.yaml': VALID_CONFIG }, testInfo.outputPath('valid.tar.gz'));

      const response = await upload(request, archive, 'valid.tar.gz');

      expect(response.status()).toBe(200);
      const body = await response.json();
      expect(body.status).toBe('imported');
      // The backup is reported by name only, never by an absolute path.
      expect(body.backup).toMatch(/^[^/\\]+$/);
      expect(await readConfigFile(request, cfg('config.yaml'))).toBe(VALID_CONFIG);
    });

    test('should import optional files together with config.yaml', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config/config.yaml': VALID_CONFIG, 'config/acquis.yaml': ACQUIS },
        testInfo.outputPath('with-optional-files.tar.gz'),
      );

      const response = await upload(request, archive, 'with-optional-files.tar.gz');

      expect(response.status()).toBe(200);
      expect((await response.json()).status).toBe('imported');
      expect(await readConfigFile(request, cfg('config.yaml'))).toBe(VALID_CONFIG);
      expect(await readConfigFile(request, cfg('acquis.yaml'))).toBe(ACQUIS);
    });
  });

  test.describe('Rejected Archives', () => {
    test('should require a file in the upload', async ({ request }) => {
      const response = await request.post(`${ADMIN}/import`, { multipart: { note: 'no file here' } });

      expect(response.status()).toBe(400);
      expect((await response.json()).error).toBe('file required');
    });

    test('should reject an archive missing config.yaml', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config/other-file.txt': 'not a config', 'config/acquis.yaml': ACQUIS },
        testInfo.outputPath('no-config.tar.gz'),
      );

      const response = await upload(request, archive, 'no-config.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe('validation failed: required file missing: config.yaml');
    });

    test('should reject an archive larger than 50 MB', async ({ request }, testInfo) => {
      const archive = await createOversizedArchive(testInfo.outputPath('oversized.tar.gz'), 51);
      expect((await fs.stat(archive)).size).toBeGreaterThan(MAX_ARCHIVE_BYTES);

      const response = await upload(request, archive, 'oversized.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toMatch(
        new RegExp(`^validation failed: archive exceeds maximum size: \\d+ > ${MAX_ARCHIVE_BYTES}$`),
      );
    });

    test('should reject an archive with a suspicious compression ratio', async ({ request }, testInfo) => {
      const archive = await createZipBomb(testInfo.outputPath('zipbomb.tar.gz'), 150);

      const response = await upload(request, archive, 'zipbomb.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toMatch(
        /^validation failed: suspicious compression ratio: \d+\.\d+x \(potential zip bomb\)$/,
      );
    });

    test('should reject a corrupted archive', async ({ request }, testInfo) => {
      const archive = await createCorruptedArchive(testInfo.outputPath('corrupted.tar.gz'));

      const response = await upload(request, archive, 'corrupted.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe('validation failed: failed to create gzip reader: gzip: invalid header');
    });
  });

  test.describe('Path Traversal', () => {
    test('should refuse an entry that escapes the configuration directory and keep the previous configuration', async ({ request }, testInfo) => {
      const archive = await createRawTarGz(
        [
          { name: 'config/config.yaml', content: VALID_CONFIG },
          { name: '../../etc/passwd', content: 'malicious content' },
        ],
        testInfo.outputPath('path-traversal.tar.gz'),
      );

      const response = await upload(request, archive, 'path-traversal.tar.gz');

      expect(response.status()).toBe(500);
      expect((await response.json()).error).toBe('extraction failed');
      expect(await readConfigFile(request, cfg('config.yaml'))).toBe(BASELINE_CONFIG);
    });
  });

  test.describe('Rollback', () => {
    test('should restore the previous configuration when the new config.yaml is rejected', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config/config.yaml': 'other_config:\n  field: value\n', 'config/acquis.yaml': ACQUIS },
        testInfo.outputPath('bad-config.tar.gz'),
      );

      const response = await upload(request, archive, 'bad-config.tar.gz');

      expect(response.status()).toBe(422);
      expect(await readConfigFile(request, cfg('config.yaml'))).toBe(BASELINE_CONFIG);
      expect(await readConfigFile(request, cfg('acquis.yaml'))).toBe(BASELINE_ACQUIS);
    });

    test('should not leave files of the rejected archive behind', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config/config.yaml': 'other_config:\n  field: value\n', 'config/extra-from-bad-import.yaml': 'leftover: true\n' },
        testInfo.outputPath('bad-extra.tar.gz'),
      );

      const response = await upload(request, archive, 'bad-extra.tar.gz');
      expect(response.status()).toBe(422);

      const files = (await (await request.get(`${ADMIN}/files`)).json()).files as string[];
      expect(files).toEqual(expect.arrayContaining([cfg('config.yaml'), cfg('acquis.yaml')]));
      expect(files).not.toContain(cfg('extra-from-bad-import.yaml'));
    });
  });

  test.describe('Export contents and import validation', () => {
    test('export omits engine data, hub cache, database files and account files', async ({ request }) => {
      const response = await request.get(`${ADMIN}/export`, { timeout: 120_000 });
      expect(response.status()).toBe(200);

      const entries = await listTarGzEntries(await response.body());
      const files = entries.filter((name) => !name.endsWith('/'));
      const baseNames = files.map((name) => name.split('/').pop()?.toLowerCase());

      expect(files).toEqual(expect.arrayContaining(['config/config.yaml']));
      // The hub item manifest is the one extra root entry, present exactly once.
      expect(files.filter((name) => name === HUB_MANIFEST_NAME)).toEqual([HUB_MANIFEST_NAME]);
      expect(files.filter((name) => /^(\.\/)?(data|hub_cache)\//.test(name))).toEqual([]);
      expect(files.filter((name) => /\.db(-wal|-shm)?$/.test(name))).toEqual([]);
      expect(baseNames).not.toContain('bouncer_key');
      expect(baseNames).not.toContain('local_api_credentials.yaml');
      expect(baseNames).not.toContain('online_api_credentials.yaml');
    });

    test('export download has a matching Content-Length and is a valid gzip/tar archive', async ({ request }) => {
      const response = await request.get(`${ADMIN}/export`, { timeout: 120_000 });
      expect(response.status()).toBe(200);
      expect(response.headers()['content-type']).toContain('application/gzip');

      const body = await response.body();
      expect(Number(response.headers()['content-length'])).toBe(body.length);

      const entries = await listTarGzEntries(body);
      expect(entries).toEqual(expect.arrayContaining(['config/config.yaml']));
      expect(entries).toContain(HUB_MANIFEST_NAME);
    });

    test('export carries a sorted version 1 hub item manifest listing the installed collection', async ({ request }) => {
      const response = await request.get(`${ADMIN}/export`, { timeout: 120_000 });
      expect(response.status()).toBe(200);

      const manifestText = await readTarGzEntry(await response.body(), HUB_MANIFEST_NAME);
      expect(manifestText).not.toBeNull();
      const manifest = JSON.parse(manifestText as string) as { version: number; items: Array<{ type: string; name: string }> };

      expect(manifest.version).toBe(1);
      expect(Object.keys(manifest).sort()).toEqual(['items', 'version']);
      expect(manifest.items).toEqual(expect.arrayContaining([{ type: 'collections', name: 'crowdsecurity/caddy' }]));
      for (const item of manifest.items) {
        expect(Object.keys(item).sort()).toEqual(['name', 'type']);
      }
      const keys = manifest.items.map((item) => `${item.type}\u0000${item.name}`);
      expect(keys).toEqual([...keys].sort((a, b) => Buffer.compare(Buffer.from(a), Buffer.from(b))));
    });

    test('import accepts an export laid out as config/config.yaml', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config/config.yaml': VALID_CONFIG, 'config/acquis.yaml': ACQUIS },
        testInfo.outputPath('real-layout.tar.gz'),
      );

      const response = await upload(request, archive, 'real-layout.tar.gz');

      expect(response.status()).toBe(200);
      expect((await response.json()).status).toBe('imported');
    });

    test('an export from this instance re-imports and keeps the CAPI registration', async ({ request }) => {
      // The connectivity diagnostics report registration from the presence of online_api_credentials.yaml,
      // an account file that must survive an import (it is never part of an export).
      const readRegistration = async () => {
        const response = await request.get(`${ADMIN}/diagnostics/connectivity`, { timeout: 60_000 });
        expect(response.status()).toBe(200);
        return (await response.json()).capi_registered as boolean;
      };
      const registeredBefore = await readRegistration();

      const exported = await request.get(`${ADMIN}/export`, { timeout: 120_000 });
      expect(exported.status()).toBe(200);

      const response = await request.post(`${ADMIN}/import`, {
        multipart: { file: { name: 'roundtrip.tar.gz', mimeType: 'application/gzip', buffer: await exported.body() } },
        timeout: 120_000,
      });

      expect(response.status()).toBe(200);
      const body = await response.json();
      expect(body.status).toBe('imported');
      // Everything the export lists is already installed on the same host.
      expect(body.hub_items.missing).toEqual([]);
      expect(body.hub_items.skipped_invalid).toBe(0);
      expect(body.hub_items.in_export).toBeGreaterThanOrEqual(1);
      expect(await readRegistration()).toBe(registeredBefore);
    });

    test('import rejects a config.yaml that is not valid YAML even when it contains api: text', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config/config.yaml': 'api:\n  server: [unclosed\n  bad indentation\n' },
        testInfo.outputPath('invalid-yaml-api.tar.gz'),
      );

      const response = await upload(request, archive, 'invalid-yaml-api.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe('config validation failed: config.yaml is not valid YAML');
    });

    test('import rejects a config.yaml with no CrowdSec sections', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config/config.yaml': 'other_config:\n  field: value\n' },
        testInfo.outputPath('no-sections.tar.gz'),
      );

      const response = await upload(request, archive, 'no-sections.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe(
        'config validation failed: config.yaml has no CrowdSec configuration sections',
      );
    });

    const configRejections: Array<{ title: string; content: string; message: string }> = [
      { title: 'is empty', content: '', message: 'config validation failed: config.yaml is empty' },
      {
        title: 'has a list at the top level',
        content: '- api\n- common\n',
        message: 'config validation failed: config.yaml must be a YAML mapping',
      },
      {
        title: 'has a duplicate key',
        content: 'api:\n  server:\n    log_level: info\napi:\n  server:\n    log_level: debug\n',
        message: 'config validation failed: config.yaml is not valid YAML',
      },
      {
        title: 'has a non-string key',
        content: '1: value\napi:\n  server:\n    log_level: info\n',
        message: 'config validation failed: config.yaml is not valid YAML',
      },
    ];
    for (const { title, content, message } of configRejections) {
      test(`import rejects a config.yaml that ${title}`, async ({ request }, testInfo) => {
        const archive = await createTarGz({ 'config/config.yaml': content }, testInfo.outputPath('rejected-config.tar.gz'));

        const response = await upload(request, archive, 'rejected-config.tar.gz');

        expect(response.status()).toBe(422);
        expect((await response.json()).error).toBe(message);
        expect(await readConfigFile(request, cfg('config.yaml'))).toBe(BASELINE_CONFIG);
      });
    }

    test('import rejects a ZIP with the single archive-format message', async ({ request }, testInfo) => {
      const archive = await createZip({}, testInfo.outputPath('config.zip'));

      const response = await upload(request, archive, 'config.zip', 'application/zip');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe('validation failed: only .tar.gz archives are supported');
    });

    test('import rejects a .txt upload with the single archive-format message', async ({ request }, testInfo) => {
      const notAnArchive = testInfo.outputPath('notes.txt');
      await fs.writeFile(notAnArchive, 'plain text');

      const response = await upload(request, notAnArchive, 'notes.txt', 'text/plain');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe('validation failed: only .tar.gz archives are supported');
    });
  });

  // A CrowdSec installation reads config/config.yaml, so an
  // archive with its configuration at the top level must be refused instead of replacing it.
  test.describe('Archive Layout', () => {
    test('refuses an archive that keeps config.yaml at the top level and changes nothing', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config.yaml': VALID_CONFIG, 'acquis.yaml': ACQUIS },
        testInfo.outputPath('flat-layout.tar.gz'),
      );
      const filesBefore = (await (await request.get(`${ADMIN}/files`)).json()).files as string[];

      const response = await upload(request, archive, 'flat-layout.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe(FLAT_ARCHIVE_MESSAGE);
      expect(await readConfigFile(request, cfg('config.yaml'))).toBe(BASELINE_CONFIG);
      expect(await readConfigFile(request, cfg('acquis.yaml'))).toBe(BASELINE_ACQUIS);
      expect((await (await request.get(`${ADMIN}/files`)).json()).files as string[]).toEqual(filesBefore);
    });
  });

  test.describe('Error codes', () => {
    test('carries a stable error code next to the unchanged message', async ({ request }, testInfo) => {
      const archive = await createTarGz({ 'config.yaml': VALID_CONFIG }, testInfo.outputPath('flat-code.tar.gz'));

      const response = await upload(request, archive, 'flat-code.tar.gz');

      expect(response.status()).toBe(422);
      const body = await response.json();
      expect(body.error).toBe(FLAT_ARCHIVE_MESSAGE);
      expect(body.error_code).toBe('import_flat_archive');
    });
  });

  test.describe('Refusal Message', () => {
    test('shows the load-check message in the UI and keeps the live configuration', async ({ page, request }, testInfo) => {
      const archive = await createTarGz({ 'config/config.yaml': VALID_CONFIG }, testInfo.outputPath('load-refused.tar.gz'));
      await page.route(`**${ADMIN}/import`, (route) => {
        if (route.request().method() !== 'POST') return route.fallback();
        return route.fulfill({
          status: 422,
          contentType: 'application/json',
          body: JSON.stringify({
            error: IMPORT_CONFIG_INVALID_MESSAGE_EN,
            error_code: 'import_config_invalid',
          }),
        });
      });

      await test.step('Open the configuration page and choose the archive', async () => {
        await page.goto('/security/crowdsec');
        await page.locator('input[type="file"]').setInputFiles(archive);
      });

      await test.step('Submit the import and see the refusal message', async () => {
        await page.getByTestId('import-btn').click();
        await expect(page.getByText(IMPORT_CONFIG_INVALID_MESSAGE_EN)).toBeVisible();
      });

      await test.step('The live configuration is still the baseline', async () => {
        expect(await readConfigFile(request, cfg('config.yaml'))).toBe(BASELINE_CONFIG);
      });
    });
  });

  // An archive that omits profiles.yaml, simulation.yaml, acquis.yaml or patterns/ must not leave
  // CrowdSec without them: the current files are kept and reported.
  test.describe('Kept Files', () => {
    test('keeps the current files an archive omits and reports them sorted', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config/config.yaml': VALID_CONFIG, 'config/profiles.yaml': BASELINE_PROFILES.replace('default_ip_remediation', 'archive_profile') },
        testInfo.outputPath('keeps-companions.tar.gz'),
      );

      const response = await upload(request, archive, 'keeps-companions.tar.gz');

      expect(response.status()).toBe(200);
      // The archive's own profiles.yaml wins and is not listed. The E2E tree also carries the default patterns/ folder.
      expect(keptCompanions((await response.json()).kept_files)).toEqual(['acquis.yaml', 'patterns', 'simulation.yaml']);
      expect(await readConfigFile(request, cfg('profiles.yaml'))).toContain('archive_profile');
      expect(await readConfigFile(request, cfg('acquis.yaml'))).toBe(BASELINE_ACQUIS);
      expect(await readConfigFile(request, cfg('simulation.yaml'))).toBe(BASELINE_SIMULATION);
    });

    test('keeps the current profiles.yaml when the archive has none', async ({ request }, testInfo) => {
      const archive = await createTarGz({ 'config/config.yaml': VALID_CONFIG }, testInfo.outputPath('no-profiles-kept.tar.gz'));

      const response = await upload(request, archive, 'no-profiles-kept.tar.gz');

      expect(response.status()).toBe(200);
      expect(keptCompanions((await response.json()).kept_files)).toEqual(['acquis.yaml', 'patterns', 'profiles.yaml', 'simulation.yaml']);
      expect(await readConfigFile(request, cfg('profiles.yaml'))).toBe(BASELINE_PROFILES);
    });

    test('gives back the notification file a kept profiles.yaml names', async ({ request }, testInfo) => {
      // Step 1: make the running system's profile name a notification, with the file that defines it.
      const withProfile = await createTarGz(
        {
          'config/config.yaml': VALID_CONFIG,
          'config/profiles.yaml': profilesYaml(['x']),
          'config/notifications/x.yaml': notificationYaml('x'),
        },
        testInfo.outputPath('profile-with-notification.tar.gz'),
      );
      expect((await upload(request, withProfile, 'profile-with-notification.tar.gz')).status()).toBe(200);

      // Step 2: a config-only archive keeps that profile, so the notification file it needs comes back.
      const configOnly = await createTarGz({ 'config/config.yaml': VALID_CONFIG }, testInfo.outputPath('config-only.tar.gz'));
      const response = await upload(request, configOnly, 'config-only.tar.gz');

      expect(response.status()).toBe(200);
      expect((await response.json()).kept_files).toEqual(
        expect.arrayContaining(['profiles.yaml', 'notifications/x.yaml']),
      );
      expect(await readConfigFile(request, cfg('profiles.yaml'))).toBe(profilesYaml(['x']));
      expect(await readConfigFile(request, cfg('notifications/x.yaml'))).toBe(notificationYaml('x'));
    });

    test('refuses an archive without profiles.yaml when the kept profile names a notification no file defines', async ({ request }, testInfo) => {
      // Replace the running profile with one naming a notification whose file the archive replaces by another name.
      const withProfile = await createTarGz(
        {
          'config/config.yaml': VALID_CONFIG,
          'config/profiles.yaml': profilesYaml(['x']),
          'config/notifications/x.yaml': notificationYaml('x'),
        },
        testInfo.outputPath('profile-for-refusal.tar.gz'),
      );
      expect((await upload(request, withProfile, 'profile-for-refusal.tar.gz')).status()).toBe(200);
      const filesBefore = (await (await request.get(`${ADMIN}/files`)).json()).files as string[];

      // The archive's own x.yaml defines another name, so the file the kept profile needs cannot come back under it.
      const clashing = await createTarGz(
        { 'config/config.yaml': VALID_CONFIG, 'config/notifications/x.yaml': notificationYaml('y') },
        testInfo.outputPath('clashing-notification.tar.gz'),
      );
      const response = await upload(request, clashing, 'clashing-notification.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe(
        'the archive has no profiles.yaml, so the current one was kept, but it names notifications the archive has no files for: x',
      );
      expect(await readConfigFile(request, cfg('notifications/x.yaml'))).toBe(notificationYaml('x'));
      expect((await (await request.get(`${ADMIN}/files`)).json()).files as string[]).toEqual(filesBefore);
    });
  });

  // An import whose profiles.yaml names a notification that no file defines would leave CrowdSec
  // unable to start, so it is refused before anything changes.
  test.describe('Notification References', () => {
    const missingMessage = (names: string) =>
      `config validation failed: profiles.yaml names notifications that have no matching file: ${names}`;

    test('refuses a profiles.yaml naming a notification no file defines and changes nothing', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config/config.yaml': VALID_CONFIG, 'config/profiles.yaml': profilesYaml(['x']) },
        testInfo.outputPath('missing-notification.tar.gz'),
      );
      const filesBefore = (await (await request.get(`${ADMIN}/files`)).json()).files as string[];

      const response = await upload(request, archive, 'missing-notification.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe(missingMessage('x'));
      expect(await readConfigFile(request, cfg('config.yaml'))).toBe(BASELINE_CONFIG);
      expect(await readConfigFile(request, cfg('acquis.yaml'))).toBe(BASELINE_ACQUIS);
      expect((await (await request.get(`${ADMIN}/files`)).json()).files as string[]).toEqual(filesBefore);
    });

    test('lists every missing notification name in sorted order', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config/config.yaml': VALID_CONFIG, 'config/profiles.yaml': profilesYaml(['zeta', 'alpha']) },
        testInfo.outputPath('two-missing.tar.gz'),
      );

      const response = await upload(request, archive, 'two-missing.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe(missingMessage('alpha, zeta'));
    });

    test('lists at most ten missing names and counts the rest', async ({ request }, testInfo) => {
      const names = Array.from({ length: 12 }, (_, index) => `n${String(index + 1).padStart(2, '0')}`);
      const archive = await createTarGz(
        { 'config/config.yaml': VALID_CONFIG, 'config/profiles.yaml': profilesYaml([...names].reverse()) },
        testInfo.outputPath('many-missing.tar.gz'),
      );

      const response = await upload(request, archive, 'many-missing.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe(missingMessage(`${names.slice(0, 10).join(', ')}, and 2 more`));
    });

    test('accepts a profiles.yaml whose notification is defined by a notifications file', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        {
          'config/config.yaml': VALID_CONFIG,
          'config/profiles.yaml': profilesYaml(['x']),
          'config/notifications/x.yaml': notificationYaml('x'),
        },
        testInfo.outputPath('defined-notification.tar.gz'),
      );

      const response = await upload(request, archive, 'defined-notification.tar.gz');

      expect(response.status()).toBe(200);
      expect((await response.json()).status).toBe('imported');
      expect(await readConfigFile(request, cfg('notifications/x.yaml'))).toBe(notificationYaml('x'));
    });

    test('accepts a notification defined in a .yml file', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        {
          'config/config.yaml': VALID_CONFIG,
          'config/profiles.yaml': profilesYaml(['x']),
          'config/notifications/x.yml': notificationYaml('x'),
        },
        testInfo.outputPath('yml-notification.tar.gz'),
      );

      const response = await upload(request, archive, 'yml-notification.tar.gz');

      expect(response.status()).toBe(200);
      expect((await response.json()).status).toBe('imported');
    });

    test('matches by the name field, so a multi-document file defines every name in it', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        {
          'config/config.yaml': VALID_CONFIG,
          'config/profiles.yaml': profilesYaml(['first', 'second']),
          'config/notifications/both.yaml': notificationYaml('first', 'second'),
        },
        testInfo.outputPath('multi-document.tar.gz'),
      );

      const response = await upload(request, archive, 'multi-document.tar.gz');

      expect(response.status()).toBe(200);
      expect((await response.json()).status).toBe('imported');
    });

    test('refuses a name that only the file name matches', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        {
          'config/config.yaml': VALID_CONFIG,
          'config/profiles.yaml': profilesYaml(['a']),
          'config/notifications/a.yaml': notificationYaml('b'),
        },
        testInfo.outputPath('file-name-match.tar.gz'),
      );

      const response = await upload(request, archive, 'file-name-match.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe(missingMessage('a'));
    });

    test('accepts a name defined under a different file name', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        {
          'config/config.yaml': VALID_CONFIG,
          'config/profiles.yaml': profilesYaml(['b']),
          'config/notifications/a.yaml': notificationYaml('b'),
        },
        testInfo.outputPath('name-field-match.tar.gz'),
      );

      const response = await upload(request, archive, 'name-field-match.tar.gz');

      expect(response.status()).toBe(200);
      expect((await response.json()).status).toBe('imported');
    });

    // CrowdSec only loads lowercase .yaml/.yml files directly inside notifications/.
    const ignoredDefinitions = [
      { title: 'a subdirectory', path: 'config/notifications/sub/s.yaml' },
      { title: 'a .txt file', path: 'config/notifications/s.txt' },
      { title: 'a .bak file', path: 'config/notifications/s.bak' },
      { title: 'an uppercase .YAML file', path: 'config/notifications/s.YAML' },
    ];
    for (const { title, path } of ignoredDefinitions) {
      test(`refuses a notification defined only in ${title}`, async ({ request }, testInfo) => {
        const archive = await createTarGz(
          { 'config/config.yaml': VALID_CONFIG, 'config/profiles.yaml': profilesYaml(['s']), [path]: notificationYaml('s') },
          testInfo.outputPath('ignored-definition.tar.gz'),
        );
        const filesBefore = (await (await request.get(`${ADMIN}/files`)).json()).files as string[];

        const response = await upload(request, archive, 'ignored-definition.tar.gz');

        expect(response.status()).toBe(422);
        expect((await response.json()).error).toBe(missingMessage('s'));
        expect(await readConfigFile(request, cfg('config.yaml'))).toBe(BASELINE_CONFIG);
        expect((await (await request.get(`${ADMIN}/files`)).json()).files as string[]).toEqual(filesBefore);
      });
    }

    test('accepts an archive whose profiles.yaml names nothing', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config/config.yaml': VALID_CONFIG, 'config/profiles.yaml': 'name: default_ip_remediation\nnotifications: []\non_success: break\n' },
        testInfo.outputPath('no-notifications.tar.gz'),
      );

      const response = await upload(request, archive, 'no-notifications.tar.gz');

      expect(response.status()).toBe(200);
      // The archive supplies its own profiles.yaml, so only the files it omits are kept.
      expect(keptCompanions((await response.json()).kept_files)).toEqual(['acquis.yaml', 'patterns', 'simulation.yaml']);
      expect(await readConfigFile(request, cfg('profiles.yaml'))).toContain('notifications: []');
    });

    test('accepts an archive with no profiles.yaml', async ({ request }, testInfo) => {
      const archive = await createTarGz({ 'config/config.yaml': VALID_CONFIG }, testInfo.outputPath('no-profiles.tar.gz'));

      const response = await upload(request, archive, 'no-profiles.tar.gz');

      expect(response.status()).toBe(200);
      expect((await response.json()).status).toBe('imported');
    });
  });

  // An export carries charon-hub-items.json (names and types only); an import reports which of
  // the listed items the target lacks. The manifest is read, never written to disk.
  test.describe('Hub Item Manifest', () => {
    const ITEMS = [
      { type: 'collections', name: 'Azlaroc/sftpgo' },
      { type: 'parsers', name: 'crowdsecurity/sshd-logs' },
      { type: 'scenarios', name: 'crowdsecurity/ssh-bf' },
    ];

    async function importWithManifest(request: APIRequestContext, manifest: string, outputPath: string) {
      const archive = await createTarGz(
        { 'config/config.yaml': VALID_CONFIG, [HUB_MANIFEST_NAME]: manifest },
        outputPath,
      );
      return upload(request, archive, 'with-manifest.tar.gz');
    }

    test('an archive without a manifest imports with no hub_items field', async ({ request }, testInfo) => {
      const archive = await createTarGz({ 'config/config.yaml': VALID_CONFIG }, testInfo.outputPath('legacy.tar.gz'));

      const response = await upload(request, archive, 'legacy.tar.gz');

      expect(response.status()).toBe(200);
      expect(await response.json()).not.toHaveProperty('hub_items');
    });

    test('reports the manifest items the target lacks as hub_items.missing', async ({ request }, testInfo) => {
      const response = await importWithManifest(request, hubManifest(ITEMS), testInfo.outputPath('manifest.tar.gz'));

      expect(response.status()).toBe(200);
      const body = await response.json();
      expect(body.status).toBe('imported');
      expect(body.hub_items).toEqual({ in_export: 3, skipped_invalid: 0, missing: ITEMS });
    });

    test('never writes the manifest to disk or lets it shape a later export', async ({ request }, testInfo) => {
      const response = await importWithManifest(request, hubManifest(ITEMS), testInfo.outputPath('manifest-disk.tar.gz'));
      expect(response.status()).toBe(200);

      const files = (await (await request.get(`${ADMIN}/files`)).json()).files as string[];
      expect(files).not.toContain(HUB_MANIFEST_NAME);

      const exported = await request.get(`${ADMIN}/export`, { timeout: 120_000 });
      expect(exported.status()).toBe(200);
      // The export regenerates its own manifest from the installed links, not from the upload.
      const manifest = JSON.parse((await readTarGzEntry(await exported.body(), HUB_MANIFEST_NAME)) as string);
      expect(manifest.items).not.toContainEqual(ITEMS[0]);
    });

    test('reads a manifest spelled ./charon-hub-items.json and still does not write it', async ({ request }, testInfo) => {
      const archive = await createRawTarGz(
        [
          { name: 'config/config.yaml', content: VALID_CONFIG },
          { name: `./${HUB_MANIFEST_NAME}`, content: hubManifest(ITEMS) },
        ],
        testInfo.outputPath('dot-slash-manifest.tar.gz'),
      );

      const response = await upload(request, archive, 'dot-slash-manifest.tar.gz');

      expect(response.status()).toBe(200);
      expect((await response.json()).hub_items.missing).toEqual(ITEMS);
      const files = (await (await request.get(`${ADMIN}/files`)).json()).files as string[];
      expect(files).not.toContain(HUB_MANIFEST_NAME);
    });

    test('counts individually invalid manifest items in skipped_invalid without naming them', async ({ request }, testInfo) => {
      const manifest = hubManifest([
        ITEMS[0],
        { type: 'collections', name: '--help' },
        { type: 'nonsense', name: 'crowdsecurity/sshd' },
        { type: 'parsers', name: 'a/../b' },
      ]);

      const response = await importWithManifest(request, manifest, testInfo.outputPath('some-invalid.tar.gz'));

      expect(response.status()).toBe(200);
      expect((await response.json()).hub_items).toEqual({ in_export: 1, skipped_invalid: 3, missing: [ITEMS[0]] });
    });

    const ignoredManifests: Array<{ title: string; manifest: string }> = [
      { title: 'is not JSON', manifest: '{not json' },
      { title: 'has an unknown version', manifest: hubManifest(ITEMS, 2) },
      { title: 'lists more than 1,000 items', manifest: hubManifest(Array.from({ length: 1001 }, (_, i) => ({ type: 'collections', name: `owner/item-${i}` }))) },
    ];
    for (const { title, manifest } of ignoredManifests) {
      test(`ignores a manifest that ${title} and still imports`, async ({ request }, testInfo) => {
        const response = await importWithManifest(request, manifest, testInfo.outputPath('ignored-manifest.tar.gz'));

        expect(response.status()).toBe(200);
        const body = await response.json();
        expect(body.status).toBe('imported');
        expect(body).not.toHaveProperty('hub_items');
      });
    }

    test('ignores the whole manifest when the archive carries it twice', async ({ request }, testInfo) => {
      const archive = await createRawTarGz(
        [
          { name: 'config/config.yaml', content: VALID_CONFIG },
          { name: HUB_MANIFEST_NAME, content: hubManifest(ITEMS) },
          { name: HUB_MANIFEST_NAME, content: hubManifest(ITEMS.slice(0, 1)) },
        ],
        testInfo.outputPath('duplicate-manifest.tar.gz'),
      );

      const response = await upload(request, archive, 'duplicate-manifest.tar.gz');

      expect(response.status()).toBe(200);
      expect(await response.json()).not.toHaveProperty('hub_items');
    });

    test('extracts a nested file with the manifest name as an ordinary file', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config/config.yaml': VALID_CONFIG, [`config/nested/${HUB_MANIFEST_NAME}`]: hubManifest(ITEMS) },
        testInfo.outputPath('nested-manifest.tar.gz'),
      );

      const response = await upload(request, archive, 'nested-manifest.tar.gz');

      expect(response.status()).toBe(200);
      expect(await response.json()).not.toHaveProperty('hub_items');
      expect(await readConfigFile(request, cfg(`nested/${HUB_MANIFEST_NAME}`))).toBe(hubManifest(ITEMS));
    });
  });
});

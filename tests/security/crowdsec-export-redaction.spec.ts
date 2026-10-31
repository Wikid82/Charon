/**
 * CrowdSec export: blanked notification values - E2E (API level)
 *
 * An export blanks the passwords, tokens, addresses and header values of notification files and
 * keeps the structure a profile needs. An import keeps the current values when the notification
 * already exists, and lists the files that still need values when it does not.
 */

import { promises as fs } from 'fs';
import type { APIRequestContext } from '@playwright/test';
import { test, expect } from '@playwright/test';
import {
  BLANKED_VALUE,
  createTarGz,
  listTarGzEntries,
  notificationYamlWithValues,
  profilesYaml,
  slackNotificationYamlWithAlias,
  readTarGzEntry,
} from '../utils/archive-helpers';
import { restoreShippedCrowdsecConfig, SHIPPED_CROWDSEC_FILES } from '../utils/crowdsec-shipped-config';

const ADMIN = '/api/v1/admin/crowdsec';
const NOTIFICATION = 'http_default';
const NOTIFICATION_PATH = `notifications/${NOTIFICATION}.yaml`;
const ALIASED = 'slack_alias';
const ALIASED_PATH = `notifications/${ALIASED}.yaml`;

/** A configuration the engine can load: the import check refuses one without config_paths. */
const VALID_CONFIG = SHIPPED_CROWDSEC_FILES['config/config.yaml'];

/** Fake values planted in the live notification file. */
const PLANTED = {
  url: 'https://notify.example.invalid/hook/SECRETTOKEN-url',
  authorization: 'Bearer SECRETTOKEN-auth',
  apiToken: 'SECRETTOKEN-header',
};
const PLANTED_TOKENS = ['SECRETTOKEN-url', 'SECRETTOKEN-auth', 'SECRETTOKEN-header'];

/** The path of a configuration file as the file API lists and reads it. */
const cfg = (name: string) => `config/${name}`;

async function upload(request: APIRequestContext, archivePath: string, uploadName: string) {
  return request.post(`${ADMIN}/import`, {
    multipart: { file: { name: uploadName, mimeType: 'application/gzip', buffer: await fs.readFile(archivePath) } },
    timeout: 120_000,
  });
}

async function readConfigFile(request: APIRequestContext, name: string) {
  const response = await request.get(`${ADMIN}/file?path=${encodeURIComponent(name)}`);
  expect(response.status()).toBe(200);
  return (await response.json()).content as string;
}

async function exportArchive(request: APIRequestContext) {
  const response = await request.get(`${ADMIN}/export`, { timeout: 120_000 });
  expect(response.status()).toBe(200);
  return response.body();
}

/** Imports an archive with the notification file (and a profile naming it) and expects success. */
async function seedNotification(
  request: APIRequestContext,
  outputPath: string,
  notification: string,
  extraFiles: Record<string, string> = {},
) {
  const archive = await createTarGz(
    {
      'config/config.yaml': VALID_CONFIG,
      'config/profiles.yaml': profilesYaml([NOTIFICATION]),
      [cfg(NOTIFICATION_PATH)]: notification,
      ...extraFiles,
    },
    outputPath,
  );
  const response = await upload(request, archive, 'seed.tar.gz');
  expect(response.status(), await response.text()).toBe(200);
  return response;
}

// Every case seeds a configuration with a notification; the shipped one goes back afterwards.
test.afterEach(async ({ request }) => {
  await restoreShippedCrowdsecConfig(request);
});

test.describe('CrowdSec export: blanked notification values', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    await seedNotification(request, testInfo.outputPath('seed.tar.gz'), notificationYamlWithValues(NOTIFICATION));
  });

  test('export blanks the address and header values of a notification file', async ({ request }) => {
    const entry = await readTarGzEntry(await exportArchive(request), cfg(NOTIFICATION_PATH));

    expect(entry).not.toBeNull();
    expect(entry).toContain(BLANKED_VALUE);
    for (const token of PLANTED_TOKENS) {
      expect(entry).not.toContain(token);
    }
  });

  test('export keeps no planted value anywhere in the archive', async ({ request }) => {
    const archive = await exportArchive(request);

    const names = await listTarGzEntries(archive);
    for (const name of names) {
      const content = (await readTarGzEntry(archive, name)) ?? '';
      for (const token of PLANTED_TOKENS) {
        expect(content, `${name} must not hold ${token}`).not.toContain(token);
        expect(name).not.toContain(token);
      }
    }
  });

  test('export keeps the keys a profile and the plugin need readable', async ({ request }) => {
    const entry = (await readTarGzEntry(await exportArchive(request), cfg(NOTIFICATION_PATH))) as string;

    expect(entry).toContain('type: http');
    expect(entry).toContain(`name: ${NOTIFICATION}`);
    expect(entry).toContain('log_level: info');
    expect(entry).toContain('method: POST');
    // The profile still names a notification the exported archive defines.
    const profiles = (await readTarGzEntry(await exportArchive(request), cfg('profiles.yaml'))) as string;
    expect(profiles).toContain(NOTIFICATION);
  });

  test('export drops comments from a notification file', async ({ request }, testInfo) => {
    await seedNotification(
      request,
      testInfo.outputPath('seed-comments.tar.gz'),
      notificationYamlWithValues(NOTIFICATION, { comments: true }),
    );

    const entry = (await readTarGzEntry(await exportArchive(request), cfg(NOTIFICATION_PATH))) as string;

    expect(entry).not.toContain('SECRETTOKEN-comment');
  });

  test('export leaves out stray non-YAML files from the notifications folder', async ({ request }, testInfo) => {
    await seedNotification(request, testInfo.outputPath('seed-stray.tar.gz'), notificationYamlWithValues(NOTIFICATION), {
      [cfg(`${NOTIFICATION_PATH}.bak`)]: notificationYamlWithValues(NOTIFICATION),
    });

    const names = await listTarGzEntries(await exportArchive(request));

    expect(names).toContain(cfg(NOTIFICATION_PATH));
    expect(names).not.toContain(cfg(`${NOTIFICATION_PATH}.bak`));
  });

  test('importing an export on the same system keeps the current values', async ({ request }, testInfo) => {
    const archivePath = testInfo.outputPath('roundtrip.tar.gz');
    await fs.writeFile(archivePath, await exportArchive(request));

    const response = await upload(request, archivePath, 'roundtrip.tar.gz');

    expect(response.status()).toBe(200);
    expect((await response.json()).needs_values).toBeUndefined();
    const content = await readConfigFile(request, cfg(NOTIFICATION_PATH));
    expect(content).toContain(PLANTED.url);
    expect(content).toContain(PLANTED.authorization);
    expect(content).toContain(PLANTED.apiToken);
    expect(content).not.toContain(BLANKED_VALUE);
  });

  test('a file that uses aliases is listed as needing values and stays as exported', async ({ request }, testInfo) => {
    await seedNotification(request, testInfo.outputPath('seed-aliased.tar.gz'), notificationYamlWithValues(NOTIFICATION), {
      'config/profiles.yaml': profilesYaml([NOTIFICATION, ALIASED]),
      [cfg(ALIASED_PATH)]: slackNotificationYamlWithAlias(ALIASED),
    });
    const exported = await exportArchive(request);
    const exportedEntry = (await readTarGzEntry(exported, cfg(ALIASED_PATH))) as string;
    const archivePath = testInfo.outputPath('roundtrip-aliased.tar.gz');
    await fs.writeFile(archivePath, exported);

    const response = await upload(request, archivePath, 'roundtrip-aliased.tar.gz');

    expect(response.status()).toBe(200);
    expect((await response.json()).needs_values).toEqual([ALIASED_PATH]);
    // Excluded from the "values unchanged" comparison: the file comes back exactly as exported.
    expect(await readConfigFile(request, cfg(ALIASED_PATH))).toBe(exportedEntry);
  });

  test('a file with an anchor that nothing refers to merges like any other file', async ({ request }, testInfo) => {
    await seedNotification(
      request,
      testInfo.outputPath('seed-anchored.tar.gz'),
      notificationYamlWithValues(NOTIFICATION, { anchored: true }),
    );
    const archivePath = testInfo.outputPath('roundtrip-anchored.tar.gz');
    await fs.writeFile(archivePath, await exportArchive(request));

    const response = await upload(request, archivePath, 'roundtrip-anchored.tar.gz');

    expect(response.status()).toBe(200);
    expect((await response.json()).needs_values).toBeUndefined();
    const content = await readConfigFile(request, cfg(NOTIFICATION_PATH));
    expect(content).toContain(PLANTED.authorization);
    expect(content).not.toContain(BLANKED_VALUE);
  });

  test('importing a blanked file where the notification does not exist keeps the placeholders and lists it', async ({
    request,
  }, testInfo) => {
    const exported = await exportArchive(request);
    // A fresh host: replace the live notification with a different one first.
    await seedNotification(
      request,
      testInfo.outputPath('seed-other.tar.gz'),
      notificationYamlWithValues('http_other'),
      { 'config/profiles.yaml': profilesYaml(['http_other']) },
    );
    const archivePath = testInfo.outputPath('fresh.tar.gz');
    await fs.writeFile(archivePath, exported);

    const response = await upload(request, archivePath, 'fresh.tar.gz');

    expect(response.status()).toBe(200);
    expect((await response.json()).needs_values).toEqual([NOTIFICATION_PATH]);
    const content = await readConfigFile(request, cfg(NOTIFICATION_PATH));
    expect(content).toContain(BLANKED_VALUE);
    for (const token of PLANTED_TOKENS) {
      expect(content).not.toContain(token);
    }
  });
});

test.describe('CrowdSec export: private key files', () => {
  const KEY_ARMOR = '-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASC\n-----END PRIVATE KEY-----\n';
  const CERT_ARMOR = '-----BEGIN CERTIFICATE-----\nMIIBszCCAVmgAwIBAgIUY2VydA==\n-----END CERTIFICATE-----\n';
  const KEY_PATH = 'config/ssl/client.pem';
  const ARMORED_PATH = 'config/ssl/other.txt';
  const CERT_PATH = 'config/ssl/client.crt';

  test.beforeEach(async ({ request }, testInfo) => {
    const notification =
      notificationYamlWithValues(NOTIFICATION) +
      'cert_path: /etc/crowdsec/ssl/client.crt\nkey_path: /etc/crowdsec/ssl/client.pem\n';
    await seedNotification(request, testInfo.outputPath('seed-keys.tar.gz'), notification, {
      [KEY_PATH]: 'opaque private material\n',
      [ARMORED_PATH]: KEY_ARMOR,
      [CERT_PATH]: CERT_ARMOR,
    });
  });

  test('export leaves out the key_path file and key-armored files but keeps certificates', async ({ request }) => {
    const archive = await exportArchive(request);

    const names = await listTarGzEntries(archive);

    expect(names).not.toContain(KEY_PATH);
    expect(names).not.toContain(ARMORED_PATH);
    expect(names).toContain(CERT_PATH);
    expect(names).toContain(cfg(NOTIFICATION_PATH));
  });

  test('export lists the files it left out in a response header', async ({ request }) => {
    const response = await request.get(`${ADMIN}/export`, { timeout: 120_000 });

    expect(response.status()).toBe(200);
    const omitted = JSON.parse(response.headers()['x-charon-omitted-files'] ?? '[]') as string[];
    expect(omitted).toEqual(expect.arrayContaining([KEY_PATH, ARMORED_PATH]));
  });

  test('importing an export on the same system keeps the current key files', async ({ request }, testInfo) => {
    const archivePath = testInfo.outputPath('roundtrip-keys.tar.gz');
    await fs.writeFile(archivePath, await exportArchive(request));

    const response = await upload(request, archivePath, 'roundtrip-keys.tar.gz');

    expect(response.status()).toBe(200);
    expect((await response.json()).kept_files).toEqual(expect.arrayContaining([KEY_PATH, ARMORED_PATH]));
    expect(await readConfigFile(request, KEY_PATH)).toBe('opaque private material\n');
    expect(await readConfigFile(request, ARMORED_PATH)).toBe(KEY_ARMOR);
  });
});

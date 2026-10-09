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
} from '../utils/archive-helpers';

const ADMIN = '/api/v1/admin/crowdsec';
const MAX_ARCHIVE_BYTES = 50 * 1024 * 1024;

const VALID_CONFIG = `api:
  server:
    listen_uri: 0.0.0.0:8080
    log_level: info
`;
const ACQUIS = `filenames:
  - /var/log/nginx/access.log
  - /var/log/auth.log
labels:
  type: syslog
`;
/** A configuration that is recognisable after a failed import. */
const BASELINE_CONFIG = `api:
  server:
    listen_uri: 127.0.0.1:8085
# baseline-marker
`;
const BASELINE_ACQUIS = `source: file
filenames:
  - /var/log/baseline.log
labels:
  type: syslog
`;

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

test.describe('CrowdSec Config Import Validation', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const baseline = await createTarGz(
      { 'config.yaml': BASELINE_CONFIG, 'acquis.yaml': BASELINE_ACQUIS },
      testInfo.outputPath('baseline.tar.gz'),
    );
    const response = await upload(request, baseline, 'baseline.tar.gz');
    expect(response.status()).toBe(200);
  });

  test.describe('Accepted Archives', () => {
    test('should import a valid archive and replace the configuration', async ({ request }, testInfo) => {
      const archive = await createTarGz({ 'config.yaml': VALID_CONFIG }, testInfo.outputPath('valid.tar.gz'));

      const response = await upload(request, archive, 'valid.tar.gz');

      expect(response.status()).toBe(200);
      const body = await response.json();
      expect(body.status).toBe('imported');
      // The backup is reported by name only, never by an absolute path.
      expect(body.backup).toMatch(/^[^/\\]+$/);
      expect(await readConfigFile(request, 'config.yaml')).toBe(VALID_CONFIG);
    });

    test('should import optional files together with config.yaml', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config.yaml': VALID_CONFIG, 'acquis.yaml': ACQUIS },
        testInfo.outputPath('with-optional-files.tar.gz'),
      );

      const response = await upload(request, archive, 'with-optional-files.tar.gz');

      expect(response.status()).toBe(200);
      expect((await response.json()).status).toBe('imported');
      expect(await readConfigFile(request, 'config.yaml')).toBe(VALID_CONFIG);
      expect(await readConfigFile(request, 'acquis.yaml')).toBe(ACQUIS);
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
        { 'other-file.txt': 'not a config', 'acquis.yaml': ACQUIS },
        testInfo.outputPath('no-config.tar.gz'),
      );

      const response = await upload(request, archive, 'no-config.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe('validation failed: required file missing: config.yaml');
    });

    test('should reject a config.yaml that is not YAML and has no CrowdSec sections', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        {
          'config.yaml': `invalid: yaml: syntax: here:
  unclosed: [bracket
  bad indentation
no proper structure`,
        },
        testInfo.outputPath('invalid-yaml.tar.gz'),
      );

      const response = await upload(request, archive, 'invalid-yaml.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe('config validation failed: invalid CrowdSec config structure');
    });

    test('should reject a config.yaml without the required CrowdSec fields', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config.yaml': 'other_config:\n  field: value\n  nested:\n    key: data\n' },
        testInfo.outputPath('missing-fields.tar.gz'),
      );

      const response = await upload(request, archive, 'missing-fields.tar.gz');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe('config validation failed: invalid CrowdSec config structure');
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

    test('should reject a ZIP file because only tar.gz is supported', async ({ request }, testInfo) => {
      const archive = await createZip({}, testInfo.outputPath('config.zip'));

      const response = await upload(request, archive, 'config.zip', 'application/zip');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe('validation failed: unsupported format for size calculation: zip');
    });

    test('should reject a file that is not an archive at all', async ({ request }, testInfo) => {
      const notAnArchive = testInfo.outputPath('notes.txt');
      await fs.writeFile(notAnArchive, 'plain text');

      const response = await upload(request, notAnArchive, 'notes.txt', 'text/plain');

      expect(response.status()).toBe(422);
      expect((await response.json()).error).toBe('validation failed: unsupported format: .txt');
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
          { name: 'config.yaml', content: VALID_CONFIG },
          { name: '../../etc/passwd', content: 'malicious content' },
        ],
        testInfo.outputPath('path-traversal.tar.gz'),
      );

      const response = await upload(request, archive, 'path-traversal.tar.gz');

      expect(response.status()).toBe(500);
      expect((await response.json()).error).toBe('extraction failed');
      expect(await readConfigFile(request, 'config.yaml')).toBe(BASELINE_CONFIG);
    });
  });

  test.describe('Rollback', () => {
    test('should restore the previous configuration when the new config.yaml is rejected', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config.yaml': 'other_config:\n  field: value\n', 'acquis.yaml': ACQUIS },
        testInfo.outputPath('bad-config.tar.gz'),
      );

      const response = await upload(request, archive, 'bad-config.tar.gz');

      expect(response.status()).toBe(422);
      expect(await readConfigFile(request, 'config.yaml')).toBe(BASELINE_CONFIG);
      expect(await readConfigFile(request, 'acquis.yaml')).toBe(BASELINE_ACQUIS);
    });

    test('should not leave files of the rejected archive behind', async ({ request }, testInfo) => {
      const archive = await createTarGz(
        { 'config.yaml': 'other_config:\n  field: value\n', 'extra-from-bad-import.yaml': 'leftover: true\n' },
        testInfo.outputPath('bad-extra.tar.gz'),
      );

      const response = await upload(request, archive, 'bad-extra.tar.gz');
      expect(response.status()).toBe(422);

      const files = (await (await request.get(`${ADMIN}/files`)).json()).files as string[];
      expect(files).toEqual(expect.arrayContaining(['config.yaml', 'acquis.yaml']));
      expect(files).not.toContain('extra-from-bad-import.yaml');
    });
  });
});

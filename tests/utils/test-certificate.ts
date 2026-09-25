/**
 * Runtime-generated self-signed certificate and key pair for upload tests.
 *
 * Generating the pair per test run (instead of committing PEM literals) keeps
 * key material out of the repository and avoids fixtures that expire.
 */

import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

export interface TestCertificatePair {
  certPem: string;
  keyPem: string;
}

let cached: TestCertificatePair | undefined;

/**
 * Returns a self-signed certificate (CN=test.local, O=TestOrg) and its matching
 * PKCS#8 private key. Requires the `openssl` binary. The pair is generated once
 * per process and reused.
 */
export function getTestCertificatePair(): TestCertificatePair {
  if (cached) {
    return cached;
  }

  const dir = mkdtempSync(join(tmpdir(), 'charon-e2e-cert-'));
  try {
    const keyPath = join(dir, 'key.pem');
    const certPath = join(dir, 'cert.pem');
    execFileSync(
      'openssl',
      [
        'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '365',
        '-subj', '/CN=test.local/O=TestOrg',
        '-keyout', keyPath, '-out', certPath,
      ],
      { stdio: 'ignore' },
    );
    cached = {
      certPem: readFileSync(certPath, 'utf8'),
      keyPem: readFileSync(keyPath, 'utf8'),
    };
    return cached;
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

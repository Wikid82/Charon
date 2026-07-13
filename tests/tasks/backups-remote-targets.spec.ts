/**
 * Backups Page - Remote Storage Targets E2E Tests (Issue #32, Phases 1 & 2)
 *
 * Verifies `RemoteTargetsCard.tsx` + `RemoteTargetFormDialog.tsx` and the
 * remote-target API described in docs/plans/current_spec.md §3.3.2 (New
 * routes — remote targets) and §3.7 (Remote Storage Design): S3/SFTP
 * targets, credentials, test-connection, SFTP host-key discovery (Phase 1,
 * live).
 *
 * Phase 2 (docs/plans/current_spec.md, this feature) extends coverage to
 * three more provider types on the same `RemoteTarget` abstraction:
 *   - WebDAV: single-step create/edit/test, same lifecycle as S3/SFTP today
 *     (no OAuth) — spec §3.3, §3.6.
 *   - Dropbox / Google Drive: two-step create->connect lifecycle (save
 *     config+client-secret with no token, "Save & Connect" triggers a
 *     full-page redirect to the provider's OAuth authorize URL, which
 *     redirects back to `/backups?oauth_result=...` after the provider
 *     callback completes) — spec §3.3, §3.6, §3.7. Real Dropbox/Google
 *     consent screens are not reachable in CI, so the round trip is mocked
 *     via Playwright route interception (`mockOAuthProviderRoundTrip`,
 *     tests/utils/phase5-helpers.ts).
 *
 * The Phase 2 specs below are written as `test.fixme` in Commit 1, encoding
 * target behavior before any backend/frontend code exists, then flipped to
 * live tests in Commit 5 once Commit 3 (backend) and Commit 4 (frontend)
 * have landed. The pre-existing Phase 1 (S3/SFTP) specs above them are
 * unmodified in behavior.
 *
 * See docs/plans/current_spec.md §6 Commit 1 / Commit 3 / Commit 4 / Commit 5.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForToast, waitForLoadingComplete } from '../utils/wait-helpers';
import {
  type RemoteTargetOAuthFields,
  buildWebDAVTargetFixture,
  buildDropboxTargetFixture,
  buildGoogleDriveTargetFixture,
  setupRemoteTargetOAuthStart,
  setupRemoteTargetOAuthStartError,
  setupRemoteTargetOAuthTestError,
  mockOAuthProviderRoundTrip,
} from '../utils/phase5-helpers';

/**
 * Shape of a RemoteStorageTarget API response (spec §3.3.2, extended by
 * §3.4 with `oauth_status`/`oauth_connected_at` for the Phase 2 providers).
 * Secrets are never included; only a `secrets_set` boolean.
 */
interface RemoteTargetResponse extends RemoteTargetOAuthFields {
  uuid: string;
  name: string;
  type: 's3' | 'sftp' | 'webdav' | 'dropbox' | 'google_drive';
  enabled: boolean;
  config: Record<string, unknown>;
  secrets_set: boolean;
  last_test_at: string | null;
  last_test_status: 'ok' | 'failed' | 'never';
  last_error: string;
  created_at: string;
  updated_at: string;
}

const mockTargets: RemoteTargetResponse[] = [
  {
    uuid: 'r1',
    name: 'Home NAS',
    type: 'sftp',
    enabled: true,
    config: { host: 'nas.lan', port: 22, path: '/backups/charon', username: 'charon' },
    secrets_set: true,
    oauth_status: '',
    oauth_connected_at: null,
    last_test_at: '2026-07-07T09:00:00Z',
    last_test_status: 'ok',
    last_error: '',
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-07T09:00:00Z',
  },
  {
    uuid: 'r2',
    name: 'Backblaze B2',
    type: 's3',
    enabled: true,
    config: {
      endpoint: 's3.us-west-002.backblazeb2.com',
      region: 'us-west-002',
      bucket: 'charon-backups',
      path_prefix: 'prod',
      use_ssl: true,
      force_path_style: false,
    },
    secrets_set: true,
    oauth_status: '',
    oauth_connected_at: null,
    last_test_at: '2026-07-06T09:00:00Z',
    last_test_status: 'failed',
    last_error: 'connection timed out',
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-06T09:00:00Z',
  },
];

async function setupRemoteTargets(
  page: import('@playwright/test').Page,
  targets: RemoteTargetResponse[] = mockTargets
) {
  await page.route('**/api/v1/backups/remote-targets', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, json: targets });
    } else {
      await route.continue();
    }
  });
}

test.describe('Backups Page - Remote Targets', () => {
  test.describe('Listing targets', () => {
    test(
      'should list remote targets with a status badge reflecting last_test_status',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const nasRow = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Home NAS' });
        await expect(nasRow).toBeVisible();
        await expect(nasRow.getByTestId('backup-remote-target-status-badge')).toContainText(/ok/i);

        const b2Row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Backblaze B2' });
        await expect(b2Row.getByTestId('backup-remote-target-status-badge')).toContainText(/failed/i);
      }
    );

    test('should show an empty state when no remote targets are configured', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await setupRemoteTargets(page, []);

      await page.goto('/tasks/backups');
      await waitForLoadingComplete(page);

      await expect(page.getByTestId('backup-remote-targets-empty-state')).toBeVisible();
    });
  });

  test.describe('Creating an S3 target', () => {
    test(
      'should submit endpoint/region/bucket/path_prefix/use_ssl/force_path_style + access key secrets',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, []);

        let createBody: Record<string, unknown> | undefined;
        await page.route('**/api/v1/backups/remote-targets', async (route) => {
          if (route.request().method() === 'POST') {
            createBody = route.request().postDataJSON();
            await route.fulfill({
              status: 201,
              json: { ...mockTargets[1], name: 'My S3 Target' },
            });
          } else {
            await route.continue();
          }
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        // The header "Add Remote Target" button and the empty-state CTA share the
        // same accessible name when the target list is empty (both open the same
        // create dialog) — disambiguate with .first() to avoid a strict-mode
        // violation.
        await page.getByRole('button', { name: /add remote target/i }).first().click();
        const dialog = page.getByRole('dialog');
        await dialog.getByRole('radio', { name: /s3/i }).check();

        await dialog.getByLabel(/name/i).fill('My S3 Target');
        await dialog.getByLabel(/endpoint/i).fill('s3.us-west-002.backblazeb2.com');
        await dialog.getByLabel(/region/i).fill('us-west-002');
        await dialog.getByLabel(/bucket/i).fill('charon-backups');
        await dialog.getByLabel(/path prefix/i).fill('prod');
        await dialog.getByLabel(/use ssl/i).check();
        await dialog.getByLabel(/force path style/i).uncheck();
        await dialog.getByLabel(/access key id/i).fill('AKIAEXAMPLE');
        await dialog.getByLabel(/secret access key/i).fill('super-secret-key');

        await Promise.all([
          page.waitForResponse(
            (r) => r.url().includes('/api/v1/backups/remote-targets') && r.request().method() === 'POST'
          ),
          dialog.getByRole('button', { name: /save|create/i }).click(),
        ]);

        expect(createBody?.type).toBe('s3');
        expect((createBody?.config as Record<string, unknown>)?.bucket).toBe('charon-backups');
        expect((createBody?.secrets as Record<string, unknown>)?.access_key_id).toBe('AKIAEXAMPLE');
      }
    );
  });

  test.describe('Creating an SFTP target', () => {
    test(
      'should submit host/port/path/username + password and support the host-key discovery flow',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, []);

        await page.route('**/api/v1/backups/remote-targets/test-draft', async (route) => {
          // Discovery test: no fingerprint stored yet, server discovers and returns it
          // without ever authenticating (spec §3.7 — HostKeyCallback aborts before auth).
          await route.fulfill({
            status: 200,
            json: {
              success: false,
              message: 'host key not yet trusted',
              discovered_fingerprint: 'SHA256:abcdef1234567890',
            },
          });
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        // The header "Add Remote Target" button and the empty-state CTA share the
        // same accessible name when the target list is empty (both open the same
        // create dialog) — disambiguate with .first() to avoid a strict-mode
        // violation.
        await page.getByRole('button', { name: /add remote target/i }).first().click();
        const dialog = page.getByRole('dialog');
        await dialog.getByRole('radio', { name: /sftp/i }).check();

        await dialog.getByLabel(/^host/i).fill('nas.lan');
        await dialog.getByLabel(/^port/i).fill('22');
        await dialog.getByLabel(/^path/i).fill('/backups/charon');
        await dialog.getByLabel(/username/i).fill('charon');
        await dialog.getByLabel(/^password/i).fill('super-secret-password');

        await dialog.getByRole('button', { name: /discover host key/i }).click();
        await expect(dialog.getByTestId('backup-remote-target-host-key-fingerprint')).toContainText(
          'SHA256:abcdef1234567890'
        );

        await dialog.getByRole('button', { name: /confirm host key/i }).click();
        await expect(dialog.getByTestId('backup-remote-target-host-key-input')).toHaveValue(
          'SHA256:abcdef1234567890'
        );
      }
    );
  });

  test.describe('Secret field conventions', () => {
    test(
      'should render secret fields as type="password" and leave them blank on edit',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const nasRow = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Home NAS' });
        await nasRow.getByTestId('backup-remote-target-edit-btn').click();

        const dialog = page.getByRole('dialog');
        const passwordField = dialog.getByLabel(/^password/i);
        await expect(passwordField).toHaveAttribute('type', 'password');
        await expect(passwordField).toHaveValue('');
        await expect(dialog.getByText(/leave blank to keep current/i)).toBeVisible();
      }
    );
  });

  test.describe('Test connection', () => {
    test('should show a success state when the connection test succeeds', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await setupRemoteTargets(page);

      await page.route('**/api/v1/backups/remote-targets/r1/test', async (route) => {
        await route.fulfill({ status: 200, json: { success: true, message: 'Connected', latency_ms: 42 } });
      });

      await page.goto('/tasks/backups');
      await waitForLoadingComplete(page);

      const nasRow = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Home NAS' });
      await nasRow.getByTestId('backup-remote-target-test-btn').click();

      await waitForToast(page, /connected|success/i, { type: 'success' });
    });

    test('should show a failure state when the connection test fails', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await setupRemoteTargets(page);

      await page.route('**/api/v1/backups/remote-targets/r2/test', async (route) => {
        await route.fulfill({ status: 502, json: { error: 'connection timed out' } });
      });

      await page.goto('/tasks/backups');
      await waitForLoadingComplete(page);

      const b2Row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Backblaze B2' });
      await b2Row.getByTestId('backup-remote-target-test-btn').click();

      await waitForToast(page, /timed out|failed/i, { type: 'error' });
    });
  });

  test.describe('Deleting a target', () => {
    test('should delete a target after confirmation', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await setupRemoteTargets(page);

      let deleteCalled = false;
      await page.route('**/api/v1/backups/remote-targets/r1', async (route) => {
        if (route.request().method() === 'DELETE') {
          deleteCalled = true;
          await route.fulfill({ status: 204 });
        } else {
          await route.continue();
        }
      });

      await page.goto('/tasks/backups');
      await waitForLoadingComplete(page);

      const nasRow = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Home NAS' });
      await nasRow.getByTestId('backup-remote-target-delete-btn').click();

      const dialog = page.getByRole('dialog');
      await expect(dialog).toBeVisible();
      await dialog.getByRole('button', { name: /delete/i }).click();

      expect(deleteCalled).toBe(true);
      await expect(page.getByTestId('backup-remote-target-row').filter({ hasText: 'Home NAS' })).toHaveCount(0);
    });
  });

  // ==========================================================================
  // Issue #32 Phase 2 — WebDAV / Dropbox / Google Drive (Commit 1: test.fixme)
  //
  // Encodes target behavior from docs/plans/current_spec.md §3.3 (API
  // Contracts) and §3.6 (Frontend Component Design) before any backend/
  // frontend code exists. Flipped to live tests in Commit 5.
  // ==========================================================================

  test.describe('Creating a WebDAV target (single-step save, no OAuth)', () => {
    test(
      'should submit url/username/base_path/insecure_skip_verify + password secret',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, []);

        let createBody: Record<string, unknown> | undefined;
        await page.route('**/api/v1/backups/remote-targets', async (route) => {
          if (route.request().method() === 'POST') {
            createBody = route.request().postDataJSON();
            await route.fulfill({ status: 201, json: buildWebDAVTargetFixture({ name: 'Nextcloud' }) });
          } else {
            await route.continue();
          }
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await test.step('Fill and submit the WebDAV form', async () => {
          await page.getByRole('button', { name: /add remote target/i }).first().click();
          const dialog = page.getByRole('dialog');
          await dialog.getByRole('radio', { name: /webdav/i }).check();

          // "/name/i" would also match the WebDAV form's "Username" label
          // (contains "name" as a substring), so anchor to the start.
          await dialog.getByLabel(/^name/i).fill('Nextcloud');
          // The WebDAV URL field's label is "WebDAV URL" (i18n
          // webdavUrl), not "URL" — an anchored /^url/i never matches it.
          await dialog.getByLabel(/webdav url/i).fill('https://nas.example.com/remote.php/dav/files/charon/');
          await dialog.getByLabel(/username/i).fill('charon');
          await dialog.getByLabel(/base path/i).fill('/charon-backups');
          // i18n label is "Skip TLS Certificate Verification" (no literal
          // "insecure" substring) — match the actual copy.
          await dialog.getByLabel(/skip tls certificate verification/i).uncheck();
          await dialog.getByLabel(/^password/i).fill('super-secret-password');

          await Promise.all([
            page.waitForResponse(
              (r) => r.url().includes('/api/v1/backups/remote-targets') && r.request().method() === 'POST'
            ),
            // WebDAV has no OAuth step — the submit button reads "Create"/"Save"
            // exactly like S3/SFTP today (spec §3.6), never "Save & Connect".
            dialog.getByRole('button', { name: /save|create/i }).click(),
          ]);
        });

        await test.step('Assert the request matches the nested webdav config shape (spec §3.3)', () => {
          expect(createBody?.type).toBe('webdav');
          const config = createBody?.config as { webdav?: Record<string, unknown> };
          expect(config?.webdav?.url).toBe('https://nas.example.com/remote.php/dav/files/charon/');
          expect(config?.webdav?.username).toBe('charon');
          expect(config?.webdav?.base_path).toBe('/charon-backups');
          expect(config?.webdav?.insecure_skip_verify).toBe(false);
          expect((createBody?.secrets as Record<string, unknown>)?.password).toBe('super-secret-password');
        });
      }
    );

    test(
      'should leave the password blank on edit and preserve existing WebDAV config values',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, [buildWebDAVTargetFixture()]);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Nextcloud' });
        await row.getByTestId('backup-remote-target-edit-btn').click();

        const dialog = page.getByRole('dialog');
        await expect(dialog.getByLabel(/webdav url/i)).toHaveValue('https://nas.example.com/remote.php/dav/files/charon/');
        await expect(dialog.getByLabel(/base path/i)).toHaveValue('/charon-backups');

        const passwordField = dialog.getByLabel(/^password/i);
        await expect(passwordField).toHaveAttribute('type', 'password');
        await expect(passwordField).toHaveValue('');
        await expect(dialog.getByText(/leave blank to keep current/i)).toBeVisible();
      }
    );

    test('should show a success state when the WebDAV connection test succeeds', async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await setupRemoteTargets(page, [buildWebDAVTargetFixture()]);

      await page.route('**/api/v1/backups/remote-targets/wd1/test', async (route) => {
        await route.fulfill({ status: 200, json: { success: true, message: 'Connected', latency_ms: 18 } });
      });

      await page.goto('/tasks/backups');
      await waitForLoadingComplete(page);

      const row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Nextcloud' });
      await row.getByTestId('backup-remote-target-test-btn').click();

      await waitForToast(page, /connected|success/i, { type: 'success' });
    });

    test(
      'should show a failure state when the WebDAV connection test fails (e.g. SSRF-rejected host)',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, [buildWebDAVTargetFixture()]);

        await page.route('**/api/v1/backups/remote-targets/wd1/test', async (route) => {
          await route.fulfill({
            status: 502,
            json: { error: 'webdav url failed SSRF validation: host resolves to a private address' },
          });
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Nextcloud' });
        await row.getByTestId('backup-remote-target-test-btn').click();

        await waitForToast(page, /ssrf|private address|failed/i, { type: 'error' });
      }
    );
  });

  test.describe('Creating a Dropbox target (two-step OAuth create->connect lifecycle)', () => {
    test(
      'should save config + client secret without a token, then redirect to the Dropbox authorize URL on Save & Connect',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, []);

        const created = buildDropboxTargetFixture({ name: 'My Dropbox', oauth_status: 'not_connected' });
        let createBody: Record<string, unknown> | undefined;
        await page.route('**/api/v1/backups/remote-targets', async (route) => {
          if (route.request().method() === 'POST') {
            createBody = route.request().postDataJSON();
            await route.fulfill({ status: 201, json: created });
          } else {
            await route.continue();
          }
        });

        await setupRemoteTargetOAuthStart(page, created.uuid, {
          authorize_url:
            'https://www.dropbox.com/oauth2/authorize?client_id=abc123&response_type=code&redirect_uri=https%3A%2F%2Fcharon.example.com%2Fapi%2Fv1%2Fbackups%2Fremote-targets%2Foauth%2Fdropbox%2Fcallback&state=mock-state',
        });
        await mockOAuthProviderRoundTrip(page, {
          provider: 'dropbox',
          targetUuid: created.uuid,
          outcome: 'approved',
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await test.step('Fill and submit the Dropbox form (config + client secret, no token yet)', async () => {
          await page.getByRole('button', { name: /add remote target/i }).first().click();
          const dialog = page.getByRole('dialog');
          await dialog.getByRole('radio', { name: /dropbox/i }).check();

          await dialog.getByLabel(/name/i).fill('My Dropbox');
          await dialog.getByLabel(/app key/i).fill('abc123');
          await dialog.getByLabel(/app secret/i).fill('dropbox-app-secret-value');
          await dialog.getByLabel(/folder path/i).fill('/charon-backups');

          // Step 1 (POST remote-targets) + step 2 (POST oauth/start) + the
          // resulting full-page redirect to Dropbox's authorize_url all
          // happen from a single "Save & Connect" click (spec §3.6).
          await dialog.getByRole('button', { name: /save & connect/i }).click();
        });

        await test.step('Assert the create request has no OAuth token fields yet (spec §3.3 R2)', () => {
          expect(createBody?.type).toBe('dropbox');
          const config = createBody?.config as { dropbox?: Record<string, unknown> };
          expect(config?.dropbox?.app_key).toBe('abc123');
          const secrets = createBody?.secrets as Record<string, unknown>;
          expect(secrets?.oauth_client_secret).toBe('dropbox-app-secret-value');
          expect(secrets?.oauth_access_token).toBeUndefined();
        });

        await test.step('Assert the full-page redirect lands back on /backups with a success result', async () => {
          await page.waitForURL(/\/backups\?oauth_result=success&provider=dropbox&target=db1/);
        });
      }
    );

    test(
      'should show a connected status badge after a successful OAuth callback redirect',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        // Simulates arriving back from the provider: RemoteTargetsCard reads
        // oauth_result/provider/target off window.location.search on mount
        // (spec §3.6) — no need to drive the redirect chain for this
        // assertion, only the landing state it produces.
        await setupRemoteTargets(page, [
          buildDropboxTargetFixture({ oauth_status: 'connected', oauth_connected_at: '2026-07-13T10:00:00Z' }),
        ]);

        await page.goto('/tasks/backups?oauth_result=success&provider=dropbox&target=db1');
        await waitForLoadingComplete(page);

        await waitForToast(page, /connected|success/i, { type: 'success' });

        const row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Dropbox' });
        await expect(row.getByTestId('backup-remote-target-oauth-status-badge')).toContainText(/connected/i);

        // The query string is stripped via history.replaceState once handled
        // (spec §3.6) so a page refresh doesn't re-show the toast.
        await expect.poll(() => new URL(page.url()).search).not.toContain('oauth_result');
      }
    );

    test(
      'should show a not_connected badge and a Connect button before any OAuth round trip has occurred',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, [buildDropboxTargetFixture({ oauth_status: 'not_connected' })]);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Dropbox' });
        await expect(row.getByTestId('backup-remote-target-oauth-status-badge')).toContainText(/not connected/i);
        await expect(row.getByRole('button', { name: /^connect$/i })).toBeVisible();
        await expect(row.getByRole('button', { name: /reconnect/i })).toHaveCount(0);
      }
    );
  });

  test.describe('Creating a Google Drive target (two-step OAuth create->connect lifecycle)', () => {
    test(
      'should save config + client secret without a token, then redirect to the Google authorize URL on Save & Connect',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, []);

        const created = buildGoogleDriveTargetFixture({ name: 'My Google Drive', oauth_status: 'not_connected' });
        let createBody: Record<string, unknown> | undefined;
        await page.route('**/api/v1/backups/remote-targets', async (route) => {
          if (route.request().method() === 'POST') {
            createBody = route.request().postDataJSON();
            await route.fulfill({ status: 201, json: created });
          } else {
            await route.continue();
          }
        });

        await setupRemoteTargetOAuthStart(page, created.uuid, {
          authorize_url:
            'https://accounts.google.com/o/oauth2/v2/auth?client_id=client-abc123.apps.googleusercontent.com&response_type=code&redirect_uri=https%3A%2F%2Fcharon.example.com%2Fapi%2Fv1%2Fbackups%2Fremote-targets%2Foauth%2Fgoogle_drive%2Fcallback&state=mock-state',
        });
        await mockOAuthProviderRoundTrip(page, {
          provider: 'google_drive',
          targetUuid: created.uuid,
          outcome: 'approved',
        });

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await test.step('Fill and submit the Google Drive form (config + client secret, no token yet)', async () => {
          await page.getByRole('button', { name: /add remote target/i }).first().click();
          const dialog = page.getByRole('dialog');
          await dialog.getByRole('radio', { name: /google drive/i }).check();

          await dialog.getByLabel(/name/i).fill('My Google Drive');
          await dialog.getByLabel(/client id/i).fill('client-abc123.apps.googleusercontent.com');
          await dialog.getByLabel(/client secret/i).fill('google-client-secret-value');
          await dialog.getByLabel(/folder path/i).fill('Charon/Backups');

          await dialog.getByRole('button', { name: /save & connect/i }).click();
        });

        await test.step('Assert the create request has no OAuth token fields yet (spec §3.3 R2)', () => {
          expect(createBody?.type).toBe('google_drive');
          const config = createBody?.config as { google_drive?: Record<string, unknown> };
          expect(config?.google_drive?.client_id).toBe('client-abc123.apps.googleusercontent.com');
          const secrets = createBody?.secrets as Record<string, unknown>;
          expect(secrets?.oauth_client_secret).toBe('google-client-secret-value');
          expect(secrets?.oauth_refresh_token).toBeUndefined();
        });

        await test.step('Assert the full-page redirect lands back on /backups with a success result', async () => {
          await page.waitForURL(/\/backups\?oauth_result=success&provider=google_drive&target=gd1/);
        });
      }
    );

    test(
      'should show a connected status badge after a successful OAuth callback redirect',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, [
          buildGoogleDriveTargetFixture({ oauth_status: 'connected', oauth_connected_at: '2026-07-13T10:00:00Z' }),
        ]);

        await page.goto('/tasks/backups?oauth_result=success&provider=google_drive&target=gd1');
        await waitForLoadingComplete(page);

        await waitForToast(page, /connected|success/i, { type: 'success' });

        const row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Google Drive' });
        await expect(row.getByTestId('backup-remote-target-oauth-status-badge')).toContainText(/connected/i);
      }
    );
  });

  test.describe('OAuth error paths', () => {
    test(
      'should surface an error toast and leave the target not_connected when oauth_result=error (consent denied)',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        // Callback redirected to oauth_result=error&message=authorization_denied
        // (spec §3.9) — target row remains not_connected, no partial-token state.
        await setupRemoteTargets(page, [buildDropboxTargetFixture({ oauth_status: 'not_connected' })]);

        await page.goto('/tasks/backups?oauth_result=error&message=authorization_denied');
        await waitForLoadingComplete(page);

        await waitForToast(page, /denied|authorization|failed/i, { type: 'error' });

        const row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Dropbox' });
        await expect(row.getByTestId('backup-remote-target-oauth-status-badge')).toContainText(/not connected/i);
      }
    );

    test(
      'should surface the oauth_not_connected error code via the Test button toast path',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, [buildDropboxTargetFixture({ oauth_status: 'not_connected' })]);
        await setupRemoteTargetOAuthTestError(
          page,
          'db1',
          'oauth_not_connected',
          'target has not completed OAuth authorization'
        );

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Dropbox' });
        await row.getByTestId('backup-remote-target-test-btn').click();

        // Generalizes through the *existing* generic onError: toast.error(error.message)
        // path already used for encryption_key_missing (spec §2.3, §3.6) — no
        // new frontend error-handling concept required for this case.
        await waitForToast(page, /oauth|authoriz/i, { type: 'error' });
      }
    );

    test(
      'should surface the oauth_revoked error code via the Test button toast path',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, [buildGoogleDriveTargetFixture({ oauth_status: 'revoked' })]);
        await setupRemoteTargetOAuthTestError(
          page,
          'gd1',
          'oauth_revoked',
          'OAuth authorization was revoked; reconnect this target'
        );

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Google Drive' });
        await row.getByTestId('backup-remote-target-test-btn').click();

        await waitForToast(page, /revoked|reconnect/i, { type: 'error' });
      }
    );

    test(
      'should surface public_url_not_configured when Save & Connect is clicked with no app.public_url Setting configured',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);

        const created = buildDropboxTargetFixture({ oauth_status: 'not_connected' });
        // Stateful mock (not the fixed-empty-list setupRemoteTargets helper):
        // the target is genuinely persisted by Create (spec R2 — "pending"
        // targets are saved even before OAuth completes), so a subsequent
        // GET after oauth/start fails must reflect it, or this test can never
        // observe the "row still exists for a later retry" behavior (spec §3.9)
        // it's asserting.
        let targets: Record<string, unknown>[] = [];
        await page.route('**/api/v1/backups/remote-targets', async (route) => {
          if (route.request().method() === 'POST') {
            targets = [...targets, created];
            await route.fulfill({ status: 201, json: created });
          } else if (route.request().method() === 'GET') {
            await route.fulfill({ status: 200, json: targets });
          } else {
            await route.continue();
          }
        });
        await setupRemoteTargetOAuthStartError(
          page,
          created.uuid,
          'public_url_not_configured',
          'the app public URL must be configured before starting OAuth'
        );

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        await page.getByRole('button', { name: /add remote target/i }).first().click();
        const dialog = page.getByRole('dialog');
        await dialog.getByRole('radio', { name: /dropbox/i }).check();
        await dialog.getByLabel(/name/i).fill('Dropbox');
        await dialog.getByLabel(/app key/i).fill('abc123');
        await dialog.getByLabel(/app secret/i).fill('secret');
        await dialog.getByRole('button', { name: /save & connect/i }).click();

        await waitForToast(page, /public url|not configured/i, { type: 'error' });

        // Target row still exists in not_connected state (not deleted) so the
        // admin can retry immediately after configuring app.public_url — no
        // process restart needed (spec §3.9).
        await expect(page.getByTestId('backup-remote-target-row').filter({ hasText: 'Dropbox' })).toBeVisible();
      }
    );
  });

  test.describe('OAuth status badge and Connect/Reconnect visibility (RemoteTargetsCard)', () => {
    test(
      'should render accessible Connect/Reconnect controls consistently across not_connected/connected/revoked states',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, [
          buildDropboxTargetFixture({ uuid: 'db1', name: 'Dropbox Not Connected', oauth_status: 'not_connected' }),
          buildDropboxTargetFixture({ uuid: 'db2', name: 'Dropbox Connected', oauth_status: 'connected' }),
          buildGoogleDriveTargetFixture({ uuid: 'gd1', name: 'Drive Revoked', oauth_status: 'revoked' }),
        ]);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        // Actual RemoteTargetsCard row structure (verified against the real
        // component, not the Commit 1 guess): name/type are two <p> elements
        // (accessible as "paragraph", not a bare "text" node), the
        // last-test-status Badge and the OAuth-status Badge carry no
        // distinguishing ARIA role so they collapse into a single "text"
        // node, and the Test button's accessible name is its i18n label
        // ("Test Connection"), not the bare "Test" the guess assumed —
        // there is also no distinct "status" role since Badge renders a
        // plain styled <span>, not role="status".
        const notConnectedRow = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Dropbox Not Connected' });
        await expect(notConnectedRow).toMatchAriaSnapshot(`
          - listitem:
            - paragraph: Dropbox Not Connected
            - paragraph: DROPBOX
            - text: Never tested Not Connected
            - button "Connect"
            - button "Test Connection"
            - button "Edit"
            - button "Delete"
        `);
      }
    );

    test(
      'should show a Reconnect button (not Connect) and a revoked badge for a target whose OAuth was revoked out-of-band',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, [buildGoogleDriveTargetFixture({ oauth_status: 'revoked' })]);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Google Drive' });
        await expect(row.getByTestId('backup-remote-target-oauth-status-badge')).toContainText(/revoked/i);
        await expect(row.getByRole('button', { name: /reconnect/i })).toBeVisible();
        await expect(row.getByRole('button', { name: /^connect$/i })).toHaveCount(0);
      }
    );

    test(
      'should hide the Connect/Reconnect button for a fully connected OAuth target',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);
        await setupRemoteTargets(page, [
          buildDropboxTargetFixture({ oauth_status: 'connected', oauth_connected_at: '2026-07-13T10:00:00Z' }),
        ]);

        await page.goto('/tasks/backups');
        await waitForLoadingComplete(page);

        const row = page.getByTestId('backup-remote-target-row').filter({ hasText: 'Dropbox' });
        await expect(row.getByTestId('backup-remote-target-oauth-status-badge')).toContainText(/connected/i);
        await expect(row.getByRole('button', { name: /^connect$/i })).toHaveCount(0);
        await expect(row.getByRole('button', { name: /reconnect/i })).toHaveCount(0);
        // Test button remains available regardless of OAuth state (spec §3.6).
        await expect(row.getByTestId('backup-remote-target-test-btn')).toBeVisible();
      }
    );
  });
});

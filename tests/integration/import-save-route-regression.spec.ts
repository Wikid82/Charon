import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { getStorageStateAuthHeaders } from '../utils/api-helpers';

type SessionResponse = {
  session?: {
    id?: string;
  };
};

const SAMPLE_CADDYFILE = `example.com {
  reverse_proxy localhost:8080
}`;

const SAMPLE_NPM_OR_JSON_EXPORT = JSON.stringify(
  {
    proxy_hosts: [
      {
        domain_names: ['route-regression.example.test'],
        forward_host: 'localhost',
        forward_port: 8080,
        forward_scheme: 'http',
      },
    ],
    access_lists: [],
    certificates: [],
  },
  null,
  2
);

function expectPredictableRouteMiss(status: number): void {
  expect([404, 405]).toContain(status);
}

function expectCanonicalSuccess(status: number, endpoint: string): void {
  expect(status, `${endpoint} should return a 2xx success response`).toBeGreaterThanOrEqual(200);
  expect(status, `${endpoint} should return a 2xx success response`).toBeLessThan(300);
}

async function readSessionId(response: import('@playwright/test').APIResponse): Promise<string> {
  const data = (await response.json()) as SessionResponse;
  const sessionId = data?.session?.id;
  expect(sessionId).toBeTruthy();
  return sessionId as string;
}

async function createBackupAndTrack(
  page: import('@playwright/test').Page,
  headers: Record<string, string>,
  createdBackupFilenames: string[]
): Promise<void> {
  const backupBeforeCommit = await page.request.post('/api/v1/backups', {
    headers,
    data: {},
  });
  expectCanonicalSuccess(backupBeforeCommit.status(), 'POST /api/v1/backups');

  const payload = (await backupBeforeCommit.json()) as { filename?: string };
  const filename = payload.filename;
  expect(filename).toBeTruthy();
  createdBackupFilenames.push(filename as string);
}

async function cleanupCreatedBackups(
  page: import('@playwright/test').Page,
  headers: Record<string, string>,
  createdBackupFilenames: string[]
): Promise<void> {
  for (const filename of createdBackupFilenames) {
    const cleanup = await page.request.delete(`/api/v1/backups/${encodeURIComponent(filename)}`, { headers });
    expectCanonicalSuccess(cleanup.status(), `DELETE /api/v1/backups/${filename}`);
  }
}

test.describe('Import/Save Route Regression Coverage', () => {
  test('Caddy import flow stages use canonical routes and reject route drift', async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    const headers = getStorageStateAuthHeaders();
    const createdBackupFilenames: string[] = [];

    try {
      await test.step('Open Caddy import page and validate route-negative probes', async () => {
        await page.goto('/tasks/import/caddyfile', { waitUntil: 'domcontentloaded' });
        await expect(page.getByRole('heading', { level: 1 })).toContainText(/import/i);
        await expect(page.getByRole('button', { name: /parse|review/i })).toBeVisible();

        const wrongStatusMethod = await page.request.post('/api/v1/import/status', {
          headers,
          data: {},
        });
        expectPredictableRouteMiss(wrongStatusMethod.status());

        const wrongUploadMethod = await page.request.get('/api/v1/import/upload', { headers });
        expectPredictableRouteMiss(wrongUploadMethod.status());

        const wrongCancelMethod = await page.request.post('/api/v1/import/cancel', {
          headers,
          data: {},
        });
        expectPredictableRouteMiss(wrongCancelMethod.status());
      });

      await test.step('Run canonical Caddy import status/upload/cancel path', async () => {
        const statusResponse = await page.request.get('/api/v1/import/status', { headers });
        expectCanonicalSuccess(statusResponse.status(), 'GET /api/v1/import/status');

        const uploadForCancel = await page.request.post('/api/v1/import/upload', {
          headers,
          data: { content: SAMPLE_CADDYFILE },
        });
        expectCanonicalSuccess(uploadForCancel.status(), 'POST /api/v1/import/upload');
        const cancelSessionId = await readSessionId(uploadForCancel);

        const cancelResponse = await page.request.delete('/api/v1/import/cancel', {
          headers,
          params: { session_uuid: cancelSessionId },
        });
        expectCanonicalSuccess(cancelResponse.status(), 'DELETE /api/v1/import/cancel');
      });

      await test.step('Run canonical Caddy preview/backup-before-commit/commit/post-state path', async () => {
        const uploadForCommit = await page.request.post('/api/v1/import/upload', {
          headers,
          data: { content: SAMPLE_CADDYFILE },
        });
        expectCanonicalSuccess(uploadForCommit.status(), 'POST /api/v1/import/upload');
        const commitSessionId = await readSessionId(uploadForCommit);

        const previewResponse = await page.request.get('/api/v1/import/preview', { headers });
        expectCanonicalSuccess(previewResponse.status(), 'GET /api/v1/import/preview');

        await createBackupAndTrack(page, headers, createdBackupFilenames);

        const commitResponse = await page.request.post('/api/v1/import/commit', {
          headers,
          data: {
            session_uuid: commitSessionId,
            resolutions: {},
            names: {},
          },
        });
        expectCanonicalSuccess(commitResponse.status(), 'POST /api/v1/import/commit');

        const postState = await page.request.get('/api/v1/import/status', { headers });
        expectCanonicalSuccess(postState.status(), 'GET /api/v1/import/status');
      });
    } finally {
      await test.step('Cleanup created backup artifacts', async () => {
        await cleanupCreatedBackups(page, headers, createdBackupFilenames);
      });
    }
  });

  test('NPM and JSON import critical routes pass canonical methods and reject drift', async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    const headers = getStorageStateAuthHeaders();

    await test.step('NPM import upload/commit/cancel with route-mismatch checks', async () => {
      await page.goto('/tasks/import/npm', { waitUntil: 'domcontentloaded' });
      await expect(page.getByRole('heading').filter({ hasText: /npm/i }).first()).toBeVisible();
      await expect(page.getByRole('button', { name: /upload\s*&\s*preview/i })).toBeVisible();

      const npmWrongMethod = await page.request.get('/api/v1/import/npm/upload', { headers });
      expectPredictableRouteMiss(npmWrongMethod.status());

      const npmCancelWrongPath = await page.request.post('/api/v1/import/npm/cancel-session', {
        headers,
        data: {},
      });
      expectPredictableRouteMiss(npmCancelWrongPath.status());

      const npmUploadForCancel = await page.request.post('/api/v1/import/npm/upload', {
        headers,
        data: { content: SAMPLE_NPM_OR_JSON_EXPORT },
      });
      expectCanonicalSuccess(npmUploadForCancel.status(), 'POST /api/v1/import/npm/upload');
      const npmCancelSession = await readSessionId(npmUploadForCancel);

      const npmCancel = await page.request.post('/api/v1/import/npm/cancel', {
        headers,
        data: { session_uuid: npmCancelSession },
      });
      expectCanonicalSuccess(npmCancel.status(), 'POST /api/v1/import/npm/cancel');

      const npmUploadForCommit = await page.request.post('/api/v1/import/npm/upload', {
        headers,
        data: { content: SAMPLE_NPM_OR_JSON_EXPORT },
      });
      expectCanonicalSuccess(npmUploadForCommit.status(), 'POST /api/v1/import/npm/upload');
      const npmCommitSession = await readSessionId(npmUploadForCommit);

      const npmCommit = await page.request.post('/api/v1/import/npm/commit', {
        headers,
        data: {
          session_uuid: npmCommitSession,
          resolutions: {},
          names: {},
        },
      });
      expectCanonicalSuccess(npmCommit.status(), 'POST /api/v1/import/npm/commit');
    });

    await test.step('JSON import upload/commit/cancel with route-mismatch checks', async () => {
      await page.goto('/tasks/import/json', { waitUntil: 'domcontentloaded' });
      await expect(page.getByRole('heading').filter({ hasText: /json/i }).first()).toBeVisible();
      await expect(page.getByRole('button', { name: /upload\s*&\s*preview/i })).toBeVisible();

      const jsonWrongMethod = await page.request.get('/api/v1/import/json/upload', { headers });
      expectPredictableRouteMiss(jsonWrongMethod.status());

      const jsonCommitWrongPath = await page.request.post('/api/v1/import/json/commit-now', {
        headers,
        data: {},
      });
      expectPredictableRouteMiss(jsonCommitWrongPath.status());

      const jsonUploadForCancel = await page.request.post('/api/v1/import/json/upload', {
        headers,
        data: { content: SAMPLE_NPM_OR_JSON_EXPORT },
      });
      expectCanonicalSuccess(jsonUploadForCancel.status(), 'POST /api/v1/import/json/upload');
      const jsonCancelSession = await readSessionId(jsonUploadForCancel);

      const jsonCancel = await page.request.post('/api/v1/import/json/cancel', {
        headers,
        data: { session_uuid: jsonCancelSession },
      });
      expectCanonicalSuccess(jsonCancel.status(), 'POST /api/v1/import/json/cancel');

      const jsonUploadForCommit = await page.request.post('/api/v1/import/json/upload', {
        headers,
        data: { content: SAMPLE_NPM_OR_JSON_EXPORT },
      });
      expectCanonicalSuccess(jsonUploadForCommit.status(), 'POST /api/v1/import/json/upload');
      const jsonCommitSession = await readSessionId(jsonUploadForCommit);

      const jsonCommit = await page.request.post('/api/v1/import/json/commit', {
        headers,
        data: {
          session_uuid: jsonCommitSession,
          resolutions: {},
          names: {},
        },
      });
      expectCanonicalSuccess(jsonCommit.status(), 'POST /api/v1/import/json/commit');
    });
  });

  test('Save flow routes for settings and proxy-host paths detect 404 regressions', async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    const headers = getStorageStateAuthHeaders();
    let createdProxyUUID = '';

    try {
      await test.step('System settings save path succeeds on canonical route', async () => {
        await page.goto('/settings/system', { waitUntil: 'domcontentloaded' });
        await expect(page.getByRole('heading', { name: /system settings/i })).toBeVisible();

        const caddyApiInput = page.locator('#caddy-api');
        await expect(caddyApiInput).toBeVisible();

        const originalCaddyApi = await caddyApiInput.inputValue();
        const requestMarker = `route-regression-${Date.now()}-${Math.floor(Math.random() * 1000)}`;
        const updatedCaddyApi = `http://localhost:2019?${requestMarker}=1`;
        await caddyApiInput.fill(updatedCaddyApi);

        const saveButton = page.getByRole('button', { name: /save settings/i }).first();
        await expect(saveButton).toBeEnabled();

        const saveResponsePromise = page.waitForResponse((response) => {
          const request = response.request();
          let payload: { key?: string; value?: string } | undefined;
          try {
            payload = request.postDataJSON() as { key?: string; value?: string };
          } catch {
            return false;
          }
          return (
            response.url().includes('/api/v1/settings') &&
            request.method() === 'POST' &&
            payload.key === 'caddy.admin_api' &&
            payload.value === updatedCaddyApi
          );
        });

        await saveButton.click();
        const saveResponse = await saveResponsePromise;
        expectCanonicalSuccess(saveResponse.status(), 'POST /api/v1/settings (caddy.admin_api)');

        // Deterministic UI effect: reloading should preserve the value we just saved.
        await page.reload({ waitUntil: 'domcontentloaded' });
        await expect(page.locator('#caddy-api')).toHaveValue(updatedCaddyApi);

        await caddyApiInput.fill(originalCaddyApi);
        const restoreResponsePromise = page.waitForResponse((response) => {
          const request = response.request();
          let payload: { key?: string; value?: string } | undefined;
          try {
            payload = request.postDataJSON() as { key?: string; value?: string };
          } catch {
            return false;
          }
          return (
            response.url().includes('/api/v1/settings') &&
            request.method() === 'POST' &&
            payload.key === 'caddy.admin_api' &&
            payload.value === originalCaddyApi
          );
        });
        await saveButton.click();
        const restoreResponse = await restoreResponsePromise;
        expectCanonicalSuccess(restoreResponse.status(), 'POST /api/v1/settings (restore caddy.admin_api)');

        const wrongSettingsMethod = await page.request.delete('/api/v1/settings', { headers });
        expectPredictableRouteMiss(wrongSettingsMethod.status());
      });

      await test.step('Proxy-host save path succeeds on canonical route and rejects wrong method/path', async () => {
        const unique = `${Date.now()}-${Math.floor(Math.random() * 1000)}`;
        const createResponse = await page.request.post('/api/v1/proxy-hosts', {
          headers,
          data: {
            name: `PR3 Route Regression ${unique}`,
            domain_names: `pr3-route-${unique}.example.test`,
            forward_host: 'localhost',
            forward_port: 8080,
            forward_scheme: 'http',
            websocket_support: false,
            enabled: true,
          },
        });
        expectCanonicalSuccess(createResponse.status(), 'POST /api/v1/proxy-hosts');
        expect([200, 201]).toContain(createResponse.status());

        const created = (await createResponse.json()) as { uuid?: string };
        createdProxyUUID = created.uuid || '';
        expect(createdProxyUUID).toBeTruthy();

        const updateResponse = await page.request.put(`/api/v1/proxy-hosts/${createdProxyUUID}`, {
          headers,
          data: {
            name: `PR3 Route Regression Updated ${unique}`,
            domain_names: `pr3-route-${unique}.example.test`,
            forward_host: 'localhost',
            forward_port: 8081,
            forward_scheme: 'http',
            websocket_support: false,
            enabled: true,
          },
        });
        expectCanonicalSuccess(updateResponse.status(), `PUT /api/v1/proxy-hosts/${createdProxyUUID}`);

        const wrongProxyMethod = await page.request.post(`/api/v1/proxy-hosts/${createdProxyUUID}`, {
          headers,
          data: {},
        });
        expectPredictableRouteMiss(wrongProxyMethod.status());

        const wrongProxyPath = await page.request.put('/api/v1/proxy-host', {
          headers,
          data: {},
        });
        expectPredictableRouteMiss(wrongProxyPath.status());
      });
    } finally {
      if (createdProxyUUID) {
        await test.step('Cleanup created proxy host', async () => {
          const cleanup = await page.request.delete(`/api/v1/proxy-hosts/${createdProxyUUID}`, { headers });
          expectCanonicalSuccess(cleanup.status(), `DELETE /api/v1/proxy-hosts/${createdProxyUUID}`);
        });
      }
    }
  });
});

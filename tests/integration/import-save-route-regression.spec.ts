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

function expectCanonicalNon404(status: number): void {
  expect(status).not.toBe(404);
  expect(status).toBeLessThan(500);
}

async function readSessionId(response: import('@playwright/test').APIResponse): Promise<string> {
  const data = (await response.json()) as SessionResponse;
  const sessionId = data?.session?.id;
  expect(sessionId).toBeTruthy();
  return sessionId as string;
}

test.describe('Import/Save Route Regression Coverage', () => {
  test('Caddy import flow stages use canonical routes and reject route drift', async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    const headers = getStorageStateAuthHeaders();

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
      expectCanonicalNon404(statusResponse.status());

      const uploadForCancel = await page.request.post('/api/v1/import/upload', {
        headers,
        data: { content: SAMPLE_CADDYFILE },
      });
      expectCanonicalNon404(uploadForCancel.status());
      const cancelSessionId = await readSessionId(uploadForCancel);

      const cancelResponse = await page.request.delete('/api/v1/import/cancel', {
        headers,
        params: { session_uuid: cancelSessionId },
      });
      expectCanonicalNon404(cancelResponse.status());
    });

    await test.step('Run canonical Caddy preview/backup-before-commit/commit/post-state path', async () => {
      const uploadForCommit = await page.request.post('/api/v1/import/upload', {
        headers,
        data: { content: SAMPLE_CADDYFILE },
      });
      expectCanonicalNon404(uploadForCommit.status());
      const commitSessionId = await readSessionId(uploadForCommit);

      const previewResponse = await page.request.get('/api/v1/import/preview', { headers });
      expectCanonicalNon404(previewResponse.status());

      const backupBeforeCommit = await page.request.post('/api/v1/backups', {
        headers,
        data: {},
      });
      expectCanonicalNon404(backupBeforeCommit.status());

      const commitResponse = await page.request.post('/api/v1/import/commit', {
        headers,
        data: {
          session_uuid: commitSessionId,
          resolutions: {},
          names: {},
        },
      });
      expectCanonicalNon404(commitResponse.status());

      const postState = await page.request.get('/api/v1/import/status', { headers });
      expectCanonicalNon404(postState.status());
    });
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
      expectCanonicalNon404(npmUploadForCancel.status());
      const npmCancelSession = await readSessionId(npmUploadForCancel);

      const npmCancel = await page.request.post('/api/v1/import/npm/cancel', {
        headers,
        data: { session_uuid: npmCancelSession },
      });
      expectCanonicalNon404(npmCancel.status());

      const npmUploadForCommit = await page.request.post('/api/v1/import/npm/upload', {
        headers,
        data: { content: SAMPLE_NPM_OR_JSON_EXPORT },
      });
      expectCanonicalNon404(npmUploadForCommit.status());
      const npmCommitSession = await readSessionId(npmUploadForCommit);

      const npmCommit = await page.request.post('/api/v1/import/npm/commit', {
        headers,
        data: {
          session_uuid: npmCommitSession,
          resolutions: {},
          names: {},
        },
      });
      expectCanonicalNon404(npmCommit.status());
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
      expectCanonicalNon404(jsonUploadForCancel.status());
      const jsonCancelSession = await readSessionId(jsonUploadForCancel);

      const jsonCancel = await page.request.post('/api/v1/import/json/cancel', {
        headers,
        data: { session_uuid: jsonCancelSession },
      });
      expectCanonicalNon404(jsonCancel.status());

      const jsonUploadForCommit = await page.request.post('/api/v1/import/json/upload', {
        headers,
        data: { content: SAMPLE_NPM_OR_JSON_EXPORT },
      });
      expectCanonicalNon404(jsonUploadForCommit.status());
      const jsonCommitSession = await readSessionId(jsonUploadForCommit);

      const jsonCommit = await page.request.post('/api/v1/import/json/commit', {
        headers,
        data: {
          session_uuid: jsonCommitSession,
          resolutions: {},
          names: {},
        },
      });
      expectCanonicalNon404(jsonCommit.status());
    });
  });

  test('Save flow routes for settings and proxy-host paths detect 404 regressions', async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    const headers = getStorageStateAuthHeaders();
    let createdProxyUUID = '';

    await test.step('System settings save path succeeds on canonical route', async () => {
      await page.goto('/settings/system', { waitUntil: 'domcontentloaded' });
      await expect(page.getByRole('heading', { name: /system settings/i })).toBeVisible();

      const saveButton = page.getByRole('button', { name: /save settings/i }).first();
      await expect(saveButton).toBeEnabled();

      const saveResponsePromise = page.waitForResponse(
        (response) =>
          response.url().includes('/api/v1/settings') &&
          ['POST', 'PATCH'].includes(response.request().method())
      );

      await saveButton.click();
      const saveResponse = await saveResponsePromise;
      expectCanonicalNon404(saveResponse.status());

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
      expectCanonicalNon404(createResponse.status());
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
      expectCanonicalNon404(updateResponse.status());

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

    if (createdProxyUUID) {
      await test.step('Cleanup created proxy host', async () => {
        const cleanup = await page.request.delete(`/api/v1/proxy-hosts/${createdProxyUUID}`, { headers });
        expectCanonicalNon404(cleanup.status());
      });
    }
  });
});

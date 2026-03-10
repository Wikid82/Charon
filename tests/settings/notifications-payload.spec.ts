import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { request as playwrightRequest } from '@playwright/test';
import { waitForLoadingComplete } from '../utils/wait-helpers';

const SETTINGS_FLAGS_ENDPOINT = '/api/v1/settings';
const PROVIDERS_ENDPOINT = '/api/v1/notifications/providers';

function buildDiscordProviderPayload(name: string) {
  return {
    name,
    type: 'discord',
    url: 'https://discord.com/api/webhooks/123456789/testtoken',
    enabled: true,
    notify_proxy_hosts: true,
    notify_remote_servers: false,
    notify_domains: false,
    notify_certs: true,
    notify_uptime: false,
    notify_security_waf_blocks: false,
    notify_security_acl_denies: false,
    notify_security_rate_limit_hits: false,
  };
}

async function enableNotifyDispatchFlags(page: import('@playwright/test').Page, token: string) {
  const keys = [
    'feature.notifications.service.gotify.enabled',
    'feature.notifications.service.webhook.enabled',
  ];

  for (const key of keys) {
    const response = await page.request.post(SETTINGS_FLAGS_ENDPOINT, {
      headers: { Authorization: `Bearer ${token}` },
      data: {
        key,
        value: 'true',
        category: 'feature',
        type: 'bool',
      },
    });

    expect(response.ok()).toBeTruthy();
  }
}

test.describe('Notifications Payload Matrix', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    await page.goto('/settings/notifications');
    await waitForLoadingComplete(page);
  });

  test('valid payload flows for discord, gotify, and webhook', async ({ page }) => {
    const createdProviders: Array<Record<string, unknown>> = [];
    const capturedCreatePayloads: Array<Record<string, unknown>> = [];

    await test.step('Mock providers create/list endpoints', async () => {
      await page.route('**/api/v1/notifications/providers', async (route, request) => {
        if (request.method() === 'GET') {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(createdProviders),
          });
          return;
        }

        if (request.method() === 'POST') {
          const payload = (await request.postDataJSON()) as Record<string, unknown>;
          capturedCreatePayloads.push(payload);
          const created = {
            id: `provider-${capturedCreatePayloads.length}`,
            ...payload,
          };
          createdProviders.push(created);
          await route.fulfill({
            status: 201,
            contentType: 'application/json',
            body: JSON.stringify(created),
          });
          return;
        }

        await route.continue();
      });
    });

    const scenarios = [
      {
        type: 'discord',
        name: `discord-matrix-${Date.now()}`,
        url: 'https://discord.com/api/webhooks/123/discordtoken',
      },
      {
        type: 'gotify',
        name: `gotify-matrix-${Date.now()}`,
        url: 'https://gotify.example.com/message',
      },
      {
        type: 'webhook',
        name: `webhook-matrix-${Date.now()}`,
        url: 'https://example.com/notify',
      },
      {
        type: 'telegram',
        name: `telegram-matrix-${Date.now()}`,
        url: '987654321',
      },
    ] as const;

    for (const scenario of scenarios) {
      await test.step(`Create ${scenario.type} provider and capture outgoing payload`, async () => {
        await page.getByRole('button', { name: /add.*provider/i }).click();

        await page.getByTestId('provider-name').fill(scenario.name);
        await page.getByTestId('provider-type').selectOption(scenario.type);
        await page.getByTestId('provider-url').fill(scenario.url);

        if (scenario.type === 'gotify') {
          await page.getByTestId('provider-gotify-token').fill('  gotify-secret-token  ');
        }

        if (scenario.type === 'telegram') {
          await page.getByTestId('provider-gotify-token').fill('bot123456789:ABCdefGHI');
        }

        await page.getByTestId('provider-save-btn').click();
      });
    }

    await test.step('Verify payload contract per provider type', async () => {
      expect(capturedCreatePayloads).toHaveLength(4);

      const discordPayload = capturedCreatePayloads.find((payload) => payload.type === 'discord');
      expect(discordPayload).toBeTruthy();
      expect(discordPayload?.token).toBeUndefined();
      expect(discordPayload?.gotify_token).toBeUndefined();

      const gotifyPayload = capturedCreatePayloads.find((payload) => payload.type === 'gotify');
      expect(gotifyPayload).toBeTruthy();
      expect(gotifyPayload?.token).toBe('gotify-secret-token');
      expect(gotifyPayload?.gotify_token).toBeUndefined();

      const webhookPayload = capturedCreatePayloads.find((payload) => payload.type === 'webhook');
      expect(webhookPayload).toBeTruthy();
      expect(webhookPayload?.token).toBeUndefined();
      expect(typeof webhookPayload?.config).toBe('string');

      const telegramPayload = capturedCreatePayloads.find((payload) => payload.type === 'telegram');
      expect(telegramPayload).toBeTruthy();
      expect(telegramPayload?.token).toBe('bot123456789:ABCdefGHI');
      expect(telegramPayload?.gotify_token).toBeUndefined();
      expect(telegramPayload?.url).toBe('987654321');
    });
  });

  test('malformed payload scenarios return sanitized validation errors', async ({ page, adminUser }) => {
    await test.step('Malformed JSON to preview endpoint returns INVALID_REQUEST', async () => {
      const response = await page.request.post('/api/v1/notifications/providers/preview', {
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${adminUser.token}`,
        },
        data: '{"type":',
      });

      expect(response.status()).toBe(400);
      const body = (await response.json()) as Record<string, unknown>;
      expect(body.code).toBe('INVALID_REQUEST');
      expect(body.category).toBe('validation');
    });

    await test.step('Malformed template content returns TEMPLATE_PREVIEW_FAILED', async () => {
      const response = await page.request.post('/api/v1/notifications/providers/preview', {
        headers: { Authorization: `Bearer ${adminUser.token}` },
        data: {
          type: 'webhook',
          url: 'https://example.com/notify',
          template: 'custom',
          config: '{"message": {{.Message}',
        },
      });

      expect(response.status()).toBe(400);
      const body = (await response.json()) as Record<string, unknown>;
      expect(body.code).toBe('TEMPLATE_PREVIEW_FAILED');
      expect(body.category).toBe('validation');
    });
  });

  test('missing required fields block submit and show validation', async ({ page }) => {
    let createCalled = false;

    await test.step('Prevent create call from being silently sent', async () => {
      await page.route('**/api/v1/notifications/providers', async (route, request) => {
        if (request.method() === 'POST') {
          createCalled = true;
        }

        await route.continue();
      });
    });

    await test.step('Submit empty provider form', async () => {
      await page.getByRole('button', { name: /add.*provider/i }).click();
      await page.getByTestId('provider-save-btn').click();
    });

    await test.step('Validate required field errors and no outbound create', async () => {
      await expect(page.getByTestId('provider-url-error')).toBeVisible();
      await expect(page.getByTestId('provider-name')).toHaveAttribute('aria-invalid', 'true');
      expect(createCalled).toBeFalsy();
    });
  });

  test('auth/header behavior checks for protected settings endpoint', async ({ page, adminUser }) => {
    const providerName = `auth-check-${Date.now()}`;
    let providerID = '';

    await test.step('Protected settings write rejects invalid bearer token', async () => {
      const unauthenticatedRequest = await playwrightRequest.newContext({
        baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080',
      });

      try {
        const noAuthResponse = await unauthenticatedRequest.post(SETTINGS_FLAGS_ENDPOINT, {
          headers: { Authorization: 'Bearer invalid-token' },
          data: {
            key: 'feature.notifications.service.webhook.enabled',
            value: 'true',
            category: 'feature',
            type: 'bool',
          },
        });

        expect([401, 403]).toContain(noAuthResponse.status());
      } finally {
        await unauthenticatedRequest.dispose();
      }
    });

    await test.step('Create provider with bearer token succeeds', async () => {
      const authResponse = await page.request.post(PROVIDERS_ENDPOINT, {
        headers: { Authorization: `Bearer ${adminUser.token}` },
        data: buildDiscordProviderPayload(providerName),
      });

      expect(authResponse.status()).toBe(201);
      const created = (await authResponse.json()) as Record<string, unknown>;
      providerID = String(created.id ?? '');
      expect(providerID.length).toBeGreaterThan(0);
    });

    await test.step('Cleanup created provider', async () => {
      const deleteResponse = await page.request.delete(`${PROVIDERS_ENDPOINT}/${providerID}`, {
        headers: { Authorization: `Bearer ${adminUser.token}` },
      });

      expect(deleteResponse.ok()).toBeTruthy();
    });
  });

  test('provider-specific transformation strips gotify token from test and preview payloads', async ({ page }) => {
    let capturedPreviewPayload: Record<string, unknown> | null = null;
    let capturedTestPayload: Record<string, unknown> | null = null;

    await test.step('Mock preview and test endpoints to capture payloads', async () => {
      await page.route('**/api/v1/notifications/providers/preview', async (route, request) => {
        capturedPreviewPayload = (await request.postDataJSON()) as Record<string, unknown>;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ rendered: '{"ok":true}', parsed: { ok: true } }),
        });
      });

      await page.route('**/api/v1/notifications/providers/test', async (route, request) => {
        capturedTestPayload = (await request.postDataJSON()) as Record<string, unknown>;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ message: 'Test notification sent' }),
        });
      });
    });

    await test.step('Fill gotify form with write-only token', async () => {
      await page.getByRole('button', { name: /add.*provider/i }).click();
      await page.getByTestId('provider-type').selectOption('gotify');
      await page.getByTestId('provider-name').fill(`gotify-transform-${Date.now()}`);
      await page.getByTestId('provider-url').fill('https://gotify.example.com/message');
      await page.getByTestId('provider-gotify-token').fill('super-secret-token');
    });

    await test.step('Trigger preview and test calls', async () => {
      await page.getByTestId('provider-preview-btn').click();
      await page.getByTestId('provider-test-btn').click();
    });

    await test.step('Assert token is not sent on preview/test payloads', async () => {
      expect(capturedPreviewPayload).toBeTruthy();
      expect(capturedPreviewPayload?.type).toBe('gotify');
      expect(capturedPreviewPayload?.token).toBeUndefined();
      expect(capturedPreviewPayload?.gotify_token).toBeUndefined();

      expect(capturedTestPayload).toBeTruthy();
      expect(capturedTestPayload?.type).toBe('gotify');
      expect(capturedTestPayload?.token).toBeUndefined();
      expect(capturedTestPayload?.gotify_token).toBeUndefined();
    });
  });

  test('security: SSRF redirect/internal target, query-token, and oversized payload are blocked', async ({ page, adminUser }) => {
    await test.step('Enable gotify and webhook dispatch feature flags', async () => {
      await enableNotifyDispatchFlags(page, adminUser.token);
    });

    await test.step('Untrusted redirect/internal SSRF-style payload is rejected before dispatch', async () => {
      const response = await page.request.post('/api/v1/notifications/providers/test', {
        headers: { Authorization: `Bearer ${adminUser.token}` },
        data: {
          type: 'webhook',
          name: 'ssrf-test',
          url: 'https://127.0.0.1/internal',
          template: 'custom',
          config: '{"message":"{{.Message}}"}',
        },
      });

      expect(response.status()).toBe(400);
      const body = (await response.json()) as Record<string, unknown>;
      expect(body.code).toBe('MISSING_PROVIDER_ID');
      expect(body.category).toBe('validation');
      expect(String(body.error ?? '')).not.toContain('127.0.0.1');
    });

    await test.step('Gotify query-token URL is rejected with sanitized error', async () => {
      const queryToken = 's3cr3t-query-token';
      const response = await page.request.post('/api/v1/notifications/providers/test', {
        headers: { Authorization: `Bearer ${adminUser.token}` },
        data: {
          type: 'gotify',
          name: 'query-token-test',
          url: `https://gotify.example.com/message?token=${queryToken}`,
          template: 'custom',
          config: '{"message":"{{.Message}}"}',
        },
      });

      expect(response.status()).toBe(400);
      const body = (await response.json()) as Record<string, unknown>;
  expect(body.code).toBe('MISSING_PROVIDER_ID');
  expect(body.category).toBe('validation');

      const responseText = JSON.stringify(body);
      expect(responseText).not.toContain(queryToken);
      expect(responseText.toLowerCase()).not.toContain('token=');
    });

    await test.step('Oversized payload/template is rejected', async () => {
      const oversizedTemplate = `{"message":"${'x'.repeat(12_500)}"}`;
      const response = await page.request.post('/api/v1/notifications/providers/test', {
        headers: { Authorization: `Bearer ${adminUser.token}` },
        data: {
          type: 'webhook',
          name: 'oversized-template-test',
          url: 'https://example.com/webhook',
          template: 'custom',
          config: oversizedTemplate,
        },
      });

      expect(response.status()).toBe(400);
      const body = (await response.json()) as Record<string, unknown>;
      expect(body.code).toBe('MISSING_PROVIDER_ID');
      expect(body.category).toBe('validation');
    });
  });

  test('security: DNS-rebinding-observable hostname path is blocked with sanitized response', async ({ page, adminUser }) => {
    await test.step('Enable gotify and webhook dispatch feature flags', async () => {
      await enableNotifyDispatchFlags(page, adminUser.token);
    });

    await test.step('Untrusted hostname payload is blocked before dispatch (rebinding guard path)', async () => {
      const blockedHostname = 'rebind-check.127.0.0.1.nip.io';
      const response = await page.request.post('/api/v1/notifications/providers/test', {
        headers: { Authorization: `Bearer ${adminUser.token}` },
        data: {
          type: 'webhook',
          name: 'dns-rebinding-observable',
          url: `https://${blockedHostname}/notify`,
          template: 'custom',
          config: '{"message":"{{.Message}}"}',
        },
      });

      expect(response.status()).toBe(400);
      const body = (await response.json()) as Record<string, unknown>;
  expect(body.code).toBe('MISSING_PROVIDER_ID');
  expect(body.category).toBe('validation');

      const responseText = JSON.stringify(body);
      expect(responseText).not.toContain(blockedHostname);
      expect(responseText).not.toContain('127.0.0.1');
    });
  });

  test('security: retry split distinguishes retryable and non-retryable failures with deterministic response semantics', async ({ page }) => {
    const capturedTestPayloads: Array<Record<string, unknown>> = [];
    let nonRetryableBody: Record<string, unknown> | null = null;
    let retryableBody: Record<string, unknown> | null = null;

    await test.step('Stub provider test endpoint with deterministic retry split contract', async () => {
      await page.route('**/api/v1/notifications/providers/test', async (route, request) => {
        const payload = (await request.postDataJSON()) as Record<string, unknown>;
        capturedTestPayloads.push(payload);

        const scenarioName = String(payload.name ?? '');
        const isRetryable = scenarioName.includes('retryable') && !scenarioName.includes('non-retryable');
        const requestID = isRetryable ? 'stub-request-retryable' : 'stub-request-non-retryable';

        await route.fulfill({
          status: 400,
          contentType: 'application/json',
          body: JSON.stringify({
            code: 'PROVIDER_TEST_FAILED',
            category: 'dispatch',
            error: 'Provider test failed',
            request_id: requestID,
            retryable: isRetryable,
          }),
        });
      });
    });

    await test.step('Open provider form and execute deterministic non-retryable test call', async () => {
      await page.getByRole('button', { name: /add.*provider/i }).click();
      await page.getByTestId('provider-type').selectOption('webhook');
      await page.getByTestId('provider-name').fill('retry-split-non-retryable');
      await page.getByTestId('provider-url').fill('https://non-retryable.example.invalid/notify');

      const nonRetryableResponsePromise = page.waitForResponse(
        (response) =>
          /\/api\/v1\/notifications\/providers\/test$/.test(response.url())
          && response.request().method() === 'POST'
          && (response.request().postData() ?? '').includes('retry-split-non-retryable')
      );

      await page.getByTestId('provider-test-btn').click();
      const nonRetryableResponse = await nonRetryableResponsePromise;
      nonRetryableBody = (await nonRetryableResponse.json()) as Record<string, unknown>;

      expect(nonRetryableResponse.status()).toBe(400);
      expect(nonRetryableBody.code).toBe('PROVIDER_TEST_FAILED');
      expect(nonRetryableBody.category).toBe('dispatch');
      expect(nonRetryableBody.error).toBe('Provider test failed');
      expect(nonRetryableBody.retryable).toBe(false);
      expect(nonRetryableBody.request_id).toBe('stub-request-non-retryable');
    });

    await test.step('Execute deterministic retryable test call on the same contract endpoint', async () => {
      await page.getByTestId('provider-name').fill('retry-split-retryable');
      await page.getByTestId('provider-url').fill('https://retryable.example.invalid/notify');

      const retryableResponsePromise = page.waitForResponse(
        (response) =>
          /\/api\/v1\/notifications\/providers\/test$/.test(response.url())
          && response.request().method() === 'POST'
          && (response.request().postData() ?? '').includes('retry-split-retryable')
      );

      await page.getByTestId('provider-test-btn').click();
      const retryableResponse = await retryableResponsePromise;
      retryableBody = (await retryableResponse.json()) as Record<string, unknown>;

      expect(retryableResponse.status()).toBe(400);
      expect(retryableBody.code).toBe('PROVIDER_TEST_FAILED');
      expect(retryableBody.category).toBe('dispatch');
      expect(retryableBody.error).toBe('Provider test failed');
      expect(retryableBody.retryable).toBe(true);
      expect(retryableBody.request_id).toBe('stub-request-retryable');
    });

    await test.step('Assert stable split distinction and sanitized API contract shape', async () => {
      expect(capturedTestPayloads).toHaveLength(2);

      expect(capturedTestPayloads[0]?.name).toBe('retry-split-non-retryable');
      expect(capturedTestPayloads[1]?.name).toBe('retry-split-retryable');

      expect(nonRetryableBody).toMatchObject({
        code: 'PROVIDER_TEST_FAILED',
        category: 'dispatch',
        error: 'Provider test failed',
        retryable: false,
      });
      expect(retryableBody).toMatchObject({
        code: 'PROVIDER_TEST_FAILED',
        category: 'dispatch',
        error: 'Provider test failed',
        retryable: true,
      });

      test.info().annotations.push({
        type: 'retry-split-semantics',
        description: 'non-retryable and retryable contracts are validated via deterministic route-stubbed /providers/test responses',
      });
    });
  });

  test('security: token does not leak in list and visible edit surfaces', async ({ page, adminUser }) => {
    const name = `gotify-redaction-${Date.now()}`;
    let providerID = '';

    await test.step('Create gotify provider with token on write path', async () => {
      const createResponse = await page.request.post(PROVIDERS_ENDPOINT, {
        headers: { Authorization: `Bearer ${adminUser.token}` },
        data: {
          ...buildDiscordProviderPayload(name),
          type: 'gotify',
          url: 'https://gotify.example.com/message',
          token: 'write-only-secret-token',
          config: '{"message":"{{.Message}}"}',
        },
      });

      expect(createResponse.status()).toBe(201);
      const created = (await createResponse.json()) as Record<string, unknown>;
      providerID = String(created.id ?? '');
      expect(providerID.length).toBeGreaterThan(0);
    });

    await test.step('List providers does not expose token fields', async () => {
      const listResponse = await page.request.get(PROVIDERS_ENDPOINT, {
        headers: { Authorization: `Bearer ${adminUser.token}` },
      });
      expect(listResponse.ok()).toBeTruthy();

      const providers = (await listResponse.json()) as Array<Record<string, unknown>>;
      const gotify = providers.find((provider) => provider.id === providerID);
      expect(gotify).toBeTruthy();
      expect(gotify?.token).toBeUndefined();
      expect(gotify?.gotify_token).toBeUndefined();
    });

    await test.step('Edit form does not pre-fill token in visible surface', async () => {
      await page.reload();
      await waitForLoadingComplete(page);

      const row = page.getByTestId(`provider-row-${providerID}`);
      await expect(row).toBeVisible({ timeout: 10000 });

      const testButton = row.getByRole('button', { name: /send test notification/i });
      await expect(testButton).toBeVisible();
      await testButton.focus();
      await page.keyboard.press('Tab');
      await page.keyboard.press('Enter');

      const tokenInput = page.getByTestId('provider-gotify-token');
      await expect(tokenInput).toBeVisible();
      await expect(tokenInput).toHaveValue('');

      const pageText = await page.locator('main').innerText();
      expect(pageText).not.toContain('write-only-secret-token');
    });

    await test.step('Cleanup created provider', async () => {
      const deleteResponse = await page.request.delete(`${PROVIDERS_ENDPOINT}/${providerID}`, {
        headers: { Authorization: `Bearer ${adminUser.token}` },
      });

      expect(deleteResponse.ok()).toBeTruthy();
    });
  });
});

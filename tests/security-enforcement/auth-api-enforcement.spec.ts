import { test, expect, request as playwrightRequest } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

test.describe('Security Enforcement API', () => {
  let unauthContext: any;

  test.beforeAll(async () => {
    unauthContext = await playwrightRequest.newContext({
      baseURL: BASE_URL,
      storageState: { cookies: [], origins: [] },
      extraHTTPHeaders: {},
    });
  });

  test.afterAll(async () => {
    await unauthContext?.dispose();
  });

  test('should reject request with missing bearer token (401)', async () => {
    const response = await unauthContext.get('/api/v1/proxy-hosts');
    expect(response.status()).toBe(401);

    const data = await response.json();
    expect(data).toHaveProperty('error');
  });

  test('should reject request with invalid bearer token (401)', async () => {
    const response = await unauthContext.get('/api/v1/proxy-hosts', {
      headers: { Authorization: 'Bearer invalid.token.here' },
    });
    expect(response.status()).toBe(401);
  });

  test('health endpoint stays public', async () => {
    const response = await unauthContext.get('/api/v1/health');
    expect(response.status()).toBe(200);
  });
});

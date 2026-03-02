import { test, expect } from './test';
import { request as playwrightRequest } from '@playwright/test';
import { TestDataManager } from '../utils/TestDataManager';

const TEST_EMAIL = process.env.E2E_TEST_EMAIL || 'e2e-test@example.com';
const TEST_PASSWORD = process.env.E2E_TEST_PASSWORD || 'TestPassword123!';

test.describe('API helper authorization', () => {
  test('TestDataManager createUser succeeds with explicit bearer token only', async ({ request, baseURL }) => {
    await test.step('Acquire admin bearer token via login API', async () => {
      const loginResponse = await request.post('/api/v1/auth/login', {
        data: {
          email: TEST_EMAIL,
          password: TEST_PASSWORD,
        },
      });

      expect(loginResponse.ok()).toBe(true);
      const loginBody = (await loginResponse.json()) as { token?: string };
      expect(loginBody.token).toBeTruthy();

      const token = loginBody.token as string;
      const bareContext = await playwrightRequest.newContext({
        baseURL,
        extraHTTPHeaders: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
      });

      const manager = new TestDataManager(bareContext, 'api-helper-auth', token);

      try {
        await test.step('Create user through helper using bearer-authenticated API calls', async () => {
          const createdUser = await manager.createUser({
            name: `Helper Auth User ${Date.now()}`,
            email: `helper-auth-${Date.now()}@test.local`,
            password: 'TestPass123!',
            role: 'user',
          });

          expect(createdUser.id).toBeTruthy();
          expect(createdUser.email).toContain('@');
        });
      } finally {
        await manager.cleanup();
        await bareContext.dispose();
      }
    });
  });
});

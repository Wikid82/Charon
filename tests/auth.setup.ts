import { test as setup } from './fixtures/test';
import { request as playwrightRequest } from '@playwright/test';
import { STORAGE_STATE } from './constants';
import { readFileSync, writeFileSync, existsSync } from 'fs';
import { dirname } from 'path';
import { TestDataManager } from './utils/TestDataManager';

/**
 * Authentication Setup for E2E Tests
 *
 * This setup handles authentication before running tests:
 * 1. Checks if initial setup is required
 * 2. If required, creates an admin user via the setup API
 * 3. Logs in and stores the auth state for reuse
 *
 * Environment variables:
 * - E2E_TEST_EMAIL: Email for test user (default: e2e-test@example.com)
 * - E2E_TEST_PASSWORD: Password for test user (default: TestPassword123!)
 * - E2E_TEST_NAME: Name for test user (default: E2E Test User)
 */

const TEST_EMAIL = process.env.E2E_TEST_EMAIL || 'e2e-test@example.com';
const TEST_PASSWORD = process.env.E2E_TEST_PASSWORD || 'TestPassword123!';
const TEST_NAME = process.env.E2E_TEST_NAME || 'E2E Test User';

const EMERGENCY_TOKEN = process.env.CHARON_EMERGENCY_TOKEN;

// Re-export STORAGE_STATE for backwards compatibility with playwright.config.js
export { STORAGE_STATE };

/**
 * Performs login and stores auth state
 */
async function resetAdminCredentials(baseURL: string | undefined): Promise<boolean> {
  if (!baseURL || !EMERGENCY_TOKEN) {
    return false;
  }

  const recoveryContext = await playwrightRequest.newContext({
    baseURL,
    extraHTTPHeaders: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      'X-Emergency-Token': EMERGENCY_TOKEN,
    },
  });

  try {
    const usersResponse = await recoveryContext.get('/api/v1/users');
    if (!usersResponse.ok()) {
      return false;
    }

    const users = await usersResponse.json();
    const normalizedEmail = TEST_EMAIL.toLowerCase();
    const existingUser = users.find((user: { email?: string }) =>
      (user.email || '').toLowerCase() === normalizedEmail
    );

    if (!existingUser) {
      const manager = new TestDataManager(recoveryContext, 'auth-credentials');
      await manager.createUser(
        {
          name: TEST_NAME,
          email: TEST_EMAIL,
          password: TEST_PASSWORD,
          role: 'admin',
        },
        { useNamespace: false }
      );
      return true;
    }

    const updates: Record<string, unknown> = {
      password: TEST_PASSWORD,
    };

    if (existingUser.enabled === false) {
      updates.enabled = true;
    }

    if (existingUser.role && existingUser.role !== 'admin') {
      updates.role = 'admin';
    }

    const updateResponse = await recoveryContext.put(`/api/v1/users/${existingUser.id}`, {
      data: updates,
    });

    if (!updateResponse.ok()) {
      const errorBody = await updateResponse.text();
      throw new Error(`Credential reset failed: ${updateResponse.status()} - ${errorBody}`);
    }

    return true;
  } catch (err) {
    console.warn('⚠️ Admin credential reset failed:', err instanceof Error ? err.message : err);
    return false;
  } finally {
    await recoveryContext.dispose();
  }
}

async function performLoginAndSaveState(
  request: APIRequestContext,
  setupRequired: boolean,
  baseURL: string | undefined,
  retryAttempted = false
): Promise<void> {
  console.log('Logging in as test user...');
  const loginResponse = await request.post('/api/v1/auth/login', {
    data: {
      email: TEST_EMAIL,
      password: TEST_PASSWORD,
    },
  });

  if (!loginResponse.ok()) {
    const status = loginResponse.status();
    const errorBody = await loginResponse.text();
    console.log(`Login failed: ${status} - ${errorBody}`);

    if (status === 401 && !retryAttempted) {
      const recovered = await resetAdminCredentials(baseURL);
      if (recovered) {
        console.log('Admin recovery completed, retrying login...');
        return performLoginAndSaveState(request, setupRequired, baseURL, true);
      }
    }

    if (!setupRequired) {
      console.log('Login failed - existing user may have different credentials.');
      console.log('Please set E2E_TEST_EMAIL and E2E_TEST_PASSWORD environment variables');
      console.log('to match an existing user, or clear the database for fresh setup.');
    }
    throw new Error(`Login failed: ${status} - ${errorBody}`);
  }

  console.log('Login successful');

  // Extract token from response body
  const loginData = await loginResponse.json();
  const token = loginData.token;

  if (!token) {
    throw new Error('Login response did not include a token');
  }

  // Store the authentication state (cookies)
  await request.storageState({ path: STORAGE_STATE });
  console.log(`Auth state saved to ${STORAGE_STATE}`);

  // Add localStorage with token to the storage state
  // This is required because the frontend checks localStorage for the token
  try {
    const savedState = JSON.parse(readFileSync(STORAGE_STATE, 'utf-8'));

    // Add localStorage data
    if (!baseURL) {
      throw new Error('baseURL is required to save localStorage in storage state');
    }

    savedState.origins = [{
      origin: baseURL,
      localStorage: [
        { name: 'charon_auth_token', value: token }
      ]
    }];

    // Write updated storage state
    const storageDir = dirname(STORAGE_STATE);
    if (!existsSync(storageDir)) {
      throw new Error(`Storage directory does not exist: ${storageDir}`);
    }

    writeFileSync(STORAGE_STATE, JSON.stringify(savedState, null, 2));
    console.log('✅ Added auth token to localStorage in storage state');

    // Verify cookie domain matches expected base URL
    const cookies = savedState.cookies || [];
    const authCookie = cookies.find((c: { name: string }) => c.name === 'auth_token');

    if (authCookie?.domain) {
      const expectedHost = new URL(baseURL).hostname;
      if (authCookie.domain !== expectedHost && authCookie.domain !== `.${expectedHost}`) {
        console.warn(`⚠️ Cookie domain mismatch: cookie domain "${authCookie.domain}" does not match baseURL host "${expectedHost}"`);
        console.warn('TestDataManager API calls may fail with 401. Ensure PLAYWRIGHT_BASE_URL uses localhost.');
      } else {
        console.log(`✅ Cookie domain "${authCookie.domain}" matches baseURL host "${expectedHost}"`);
      }
    }
  } catch (err) {
    console.warn('⚠️ Could not validate storage state:', err instanceof Error ? err.message : err);
  }
}

setup('authenticate', async ({ request, baseURL }) => {
  // Step 1: Check if setup is required
  const setupStatusResponse = await request.get('/api/v1/setup');

  // Accept 200 (normal) or 401 (already initialized/auth required)
  // Provide diagnostic info on unexpected status for actionable failures
  const status = setupStatusResponse.status();
  if (![200, 401].includes(status)) {
    const body = await setupStatusResponse.text();
    throw new Error(
      `GET /api/v1/setup failed with unexpected status ${status}: ${body}\n` +
      `Verify PLAYWRIGHT_BASE_URL (${baseURL}) points to a running backend.`
    );
  }

  // If 401, setup is already complete - proceed to login
  if (status === 401) {
    console.log('Setup endpoint returned 401 - setup already complete, proceeding to login');
    return await performLoginAndSaveState(request, false, baseURL);
  }

  const setupStatus = await setupStatusResponse.json();

  if (setupStatus.setupRequired) {
    // Step 2: Run initial setup to create admin user
    console.log('Running initial setup to create test admin user...');
    const setupResponse = await request.post('/api/v1/setup', {
      data: {
        name: TEST_NAME,
        email: TEST_EMAIL,
        password: TEST_PASSWORD,
      },
    });

    if (!setupResponse.ok()) {
      const errorBody = await setupResponse.text();
      throw new Error(`Setup failed: ${setupResponse.status()} - ${errorBody}`);
    }
    console.log('Initial setup completed successfully');
  }

  // Step 3: Login and save auth state
  await performLoginAndSaveState(request, setupStatus.setupRequired, baseURL);
});

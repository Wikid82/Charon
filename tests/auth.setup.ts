import { test as setup, expect } from '@bgotink/playwright-coverage';
import { STORAGE_STATE } from './constants';
import { readFileSync } from 'fs';

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

// Re-export STORAGE_STATE for backwards compatibility with playwright.config.js
export { STORAGE_STATE };

setup('authenticate', async ({ request, baseURL }) => {
  // Step 1: Check if setup is required
  const setupStatusResponse = await request.get('/api/v1/setup');
  expect(setupStatusResponse.ok()).toBeTruthy();

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

  // Step 3: Login to get auth token
  console.log('Logging in as test user...');
  const loginResponse = await request.post('/api/v1/auth/login', {
    data: {
      email: TEST_EMAIL,
      password: TEST_PASSWORD,
    },
  });

  if (!loginResponse.ok()) {
    const errorBody = await loginResponse.text();
    console.log(`Login failed: ${loginResponse.status()} - ${errorBody}`);

    // If login fails and setup wasn't required, the user might already exist with different credentials
    // This can happen in dev environments
    if (!setupStatus.setupRequired) {
      console.log('Login failed - existing user may have different credentials.');
      console.log('Please set E2E_TEST_EMAIL and E2E_TEST_PASSWORD environment variables');
      console.log('to match an existing user, or clear the database for fresh setup.');
    }
    throw new Error(`Login failed: ${loginResponse.status()} - ${errorBody}`);
  }

  const loginData = await loginResponse.json();
  console.log('Login successful');

  // Step 4: Store the authentication state
  // The login endpoint sets an auth_token cookie, we need to save the storage state
  await request.storageState({ path: STORAGE_STATE });
  console.log(`Auth state saved to ${STORAGE_STATE}`);

  // Step 5: Verify cookie domain matches expected base URL
  try {
    const savedState = JSON.parse(readFileSync(STORAGE_STATE, 'utf-8'));
    const cookies = savedState.cookies || [];
    const authCookie = cookies.find((c: { name: string }) => c.name === 'auth_token');

    if (authCookie?.domain && baseURL) {
      const expectedHost = new URL(baseURL).hostname;
      if (authCookie.domain !== expectedHost && authCookie.domain !== `.${expectedHost}`) {
        console.warn(`⚠️ Cookie domain mismatch: cookie domain "${authCookie.domain}" does not match baseURL host "${expectedHost}"`);
        console.warn('TestDataManager API calls may fail with 401. Ensure PLAYWRIGHT_BASE_URL uses localhost.');
      } else {
        console.log(`✅ Cookie domain "${authCookie.domain}" matches baseURL host "${expectedHost}"`);
      }
    }
  } catch (err) {
    console.warn('⚠️ Could not validate cookie domain:', err instanceof Error ? err.message : err);
  }
});

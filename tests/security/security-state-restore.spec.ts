/**
 * Security State Restore E2E Test
 *
 * Proves the enforcement helpers put the module switches back after a test turns one on.
 * The check runs the same path the enforcement specs use: capture the snapshot, enable a module through
 * withSecurityEnabled, run its cleanup, then compare the switches with expectModuleSwitchesRestored.
 *
 * Limit: the settings API cannot delete a key. A key that was absent before the test is written back with the
 * effective boolean the app resolved before it, so this asserts effective state, not absence of the key.
 */

import { test, request as playwrightRequest, type APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import { captureSecurityState, withSecurityEnabled } from '../utils/security-helpers';
import { expectModuleSwitchesRestored } from '../utils/security-settings-api';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';

test.describe('Security state restore', () => {
  let requestContext: APIRequestContext;

  test.beforeAll(async () => {
    requestContext = await playwrightRequest.newContext({ baseURL: BASE_URL, storageState: STORAGE_STATE });
  });

  test.afterAll(async () => {
    await requestContext.dispose();
  });

  test('module switches match the captured state after enable and cleanup', async () => {
    const snapshot = await captureSecurityState(requestContext);

    // Cerberus is the only switch turned on: it does not gate requests by itself, so the runner stays reachable.
    const cleanup = await withSecurityEnabled(requestContext, { cerberus: true });
    await cleanup();

    await expectModuleSwitchesRestored(requestContext, snapshot.modules);
  });
});

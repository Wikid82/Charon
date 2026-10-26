/**
 * CrowdSec Lifecycle Consistency E2E Tests
 *
 * Verifies that every switch that controls the CrowdSec engine drives the
 * engine itself, not only the stored setting:
 * - The security PATCH endpoint starts and stops the engine
 * - The settings API starts and stops the engine when the enabled key changes
 * - The settings API refuses the deprecated mode key
 * - The emergency security reset stops the engine
 *
 * Runs only under `--project=security-tests`.
 */

import { test, expect } from '../fixtures/test';
import type { APIRequestContext } from '@playwright/test';
import { EMERGENCY_TOKEN } from '../fixtures/security';

const STATUS_ROUTE = '/api/v1/admin/crowdsec/status';
const SECURITY_CROWDSEC_ROUTE = '/api/v1/security/crowdsec';
const SETTINGS_ROUTE = '/api/v1/settings';
const EMERGENCY_RESET_ROUTE = '/api/v1/emergency/security-reset';

/** Upper bound for the engine to reach the expected state after a reset. */
const ENGINE_STATE_TIMEOUT_MS = 20_000;

/** Reads the `running` flag from the CrowdSec status endpoint. */
async function isEngineRunning(request: APIRequestContext): Promise<boolean> {
  const response = await request.get(STATUS_ROUTE);
  expect(response.ok()).toBeTruthy();
  const body = (await response.json()) as { running: boolean };
  return body.running;
}

/** Sets the CrowdSec security module through the PATCH endpoint and expects success. */
async function patchCrowdsecEnabled(request: APIRequestContext, enabled: boolean): Promise<void> {
  const response = await request.patch(SECURITY_CROWDSEC_ROUTE, { data: { enabled }, timeout: 120_000 });
  expect(response.ok()).toBeTruthy();
}

/** Writes one key through the settings API and returns the raw response. */
async function postSetting(request: APIRequestContext, key: string, value: string) {
  return request.post(SETTINGS_ROUTE, { data: { key, value } });
}

test.describe('CrowdSec lifecycle consistency', () => {
  test.describe.configure({ mode: 'serial', timeout: 150_000 });

  test('security PATCH enable starts the engine', async ({ request }) => {
    await test.step('Ensure the engine is stopped', async () => {
      await patchCrowdsecEnabled(request, false);
      await expect.poll(() => isEngineRunning(request), { timeout: ENGINE_STATE_TIMEOUT_MS }).toBe(false);
    });

    await test.step('Enable CrowdSec through the security PATCH endpoint', async () => {
      await patchCrowdsecEnabled(request, true);
    });

    await test.step('Verify the engine reports running', async () => {
      await expect.poll(() => isEngineRunning(request), { timeout: ENGINE_STATE_TIMEOUT_MS }).toBe(true);
    });
  });

  test('security PATCH disable stops the engine', async ({ request }) => {
    await test.step('Ensure the engine is running', async () => {
      await patchCrowdsecEnabled(request, true);
      await expect.poll(() => isEngineRunning(request), { timeout: ENGINE_STATE_TIMEOUT_MS }).toBe(true);
    });

    await test.step('Disable CrowdSec through the security PATCH endpoint', async () => {
      await patchCrowdsecEnabled(request, false);
    });

    await test.step('Verify the engine reports stopped', async () => {
      await expect.poll(() => isEngineRunning(request), { timeout: ENGINE_STATE_TIMEOUT_MS }).toBe(false);
    });
  });

  test('settings change of the enabled key stops the engine', async ({ request }) => {
    await test.step('Ensure the engine is running', async () => {
      await patchCrowdsecEnabled(request, true);
      await expect.poll(() => isEngineRunning(request), { timeout: ENGINE_STATE_TIMEOUT_MS }).toBe(true);
    });

    await test.step('Set the enabled key to false through the settings API', async () => {
      const response = await postSetting(request, 'security.crowdsec.enabled', 'false');
      expect(response.ok()).toBeTruthy();
    });

    await test.step('Verify the engine reports stopped', async () => {
      await expect.poll(() => isEngineRunning(request), { timeout: ENGINE_STATE_TIMEOUT_MS }).toBe(false);
    });
  });

  test('settings API refuses the deprecated CrowdSec mode key', async ({ request }) => {
    await test.step('Set the mode key to local through the settings API', async () => {
      const response = await postSetting(request, 'security.crowdsec.mode', 'local');
      expect(response.status()).toBe(400);
    });
  });

  test('emergency security reset stops the engine', async ({ request }) => {
    await test.step('Ensure the engine is running', async () => {
      await patchCrowdsecEnabled(request, true);
      await expect.poll(() => isEngineRunning(request), { timeout: ENGINE_STATE_TIMEOUT_MS }).toBe(true);
    });

    await test.step('Run the emergency security reset', async () => {
      const response = await request.post(EMERGENCY_RESET_ROUTE, {
        headers: { 'X-Emergency-Token': EMERGENCY_TOKEN },
      });
      expect(response.ok()).toBeTruthy();
    });

    await test.step('Verify the engine reports stopped', async () => {
      await expect.poll(() => isEngineRunning(request), { timeout: ENGINE_STATE_TIMEOUT_MS }).toBe(false);
    });
  });
});

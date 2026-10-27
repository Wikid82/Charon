/**
 * Security Config Field Update E2E Tests
 *
 * The security config save endpoint (POST /api/v1/security/config) applies only the fields a request carries.
 * Each test seeds a known security state through the API, performs one save (through the API, or through the
 * Rate Limiting and Security pages), then compares the stored row and /security/status before and after.
 *
 * A save can also change the module switch keys: waf_mode and rate_limit_enable write the key only when the value
 * differs from the stored row (the echo rule), so a GET -> edit -> POST round trip never flips a module.
 *
 * The admin allow-list always keeps the test runner's ranges so the runner stays reachable while Cerberus is on.
 */

import { test, expect } from '../fixtures/test';
import { request as playwrightRequest, type APIRequestContext, type Locator, type Page } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import {
  CONFIG_PATH,
  RUNNER_WHITELIST,
  SECURITY_MODULE_SWITCH_KEYS,
  type SecurityConfigRow,
  bodyFromRow,
  getConfigRow,
  readModuleSettings,
  restoreModuleSwitches,
  saveConfig,
  writeSettingValue,
} from '../utils/security-settings-api';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
const STATUS_PATH = '/api/v1/security/status';
const DEFAULT_CONFIG_NAME = 'default';

const EXTRA_WHITELIST_CIDR = '198.51.100.0/24';

/** Known state every test starts from. Fields not listed here keep whatever the stored row holds. */
const SEED_STATE = {
  name: DEFAULT_CONFIG_NAME,
  enabled: true,
  admin_whitelist: RUNNER_WHITELIST,
  waf_mode: 'monitor',
  waf_learning: true,
  waf_paranoia_level: 2,
  waf_exclusions: '[]',
  rate_limit_enable: true,
  rate_limit_mode: 'enabled',
  rate_limit_requests: 50,
  rate_limit_burst: 7,
  rate_limit_window_sec: 90,
  rate_limit_bypass_list: '10.20.30.0/24',
};

/** Returns a request context that carries the project's authenticated storage state. */
async function createAuthedContext(): Promise<APIRequestContext> {
  return playwrightRequest.newContext({ baseURL: BASE_URL, storageState: STORAGE_STATE });
}

/** Retries a request that the API rate-limits (HTTP 429), like the other security helpers. */
async function withRateLimitRetry<T extends { status(): number }>(send: () => Promise<T>): Promise<T> {
  const maxRetries = 5;
  const retryDelayMs = 1000;
  let response = await send();
  for (let attempt = 0; attempt < maxRetries && response.status() === 429; attempt += 1) {
    await new Promise((resolve) => setTimeout(resolve, retryDelayMs));
    response = await send();
  }
  return response;
}

/**
 * Returns the module status flags. config_apply is left out: its applied_at timestamp moves on every save,
 * so it says nothing about which fields were stored.
 */
async function getStatus(context: APIRequestContext): Promise<Record<string, unknown>> {
  const response = await withRateLimitRetry(() => context.get(STATUS_PATH));
  expect(response.ok(), `GET ${STATUS_PATH} failed with ${response.status()}`).toBeTruthy();
  const { config_apply: _configApply, ...flags } = (await response.json()) as Record<string, unknown>;
  return flags;
}

/**
 * Maps the module switches to their effective values. An absent security switch and a false one both leave the module
 * off, so they compare equal; an absent key cannot be restored byte for byte (see restoreModuleSwitches).
 */
function effectiveOf(settings: Record<string, string>): Record<string, string> {
  return Object.fromEntries(
    Object.entries(settings).map(([key, value]) => [
      key,
      (SECURITY_MODULE_SWITCH_KEYS as readonly string[]).includes(key) && value !== 'true' ? 'false' : value,
    ]),
  );
}

/** Seeds SEED_STATE (with optional overrides) and returns the row and status to compare against. */
async function seedState(context: APIRequestContext, overrides: Record<string, unknown> = {}) {
  await saveConfig(context, { ...SEED_STATE, ...overrides });
  const row = await getConfigRow(context);
  expect(row).not.toBeNull();
  const status = await getStatus(context);
  return { row: row as SecurityConfigRow, status };
}

/**
 * Asserts the stored row equals the row captured before the save, with only the listed fields replaced.
 * With no listed fields the row must be identical, including updated_at.
 */
function expectOnlyChanged(
  before: SecurityConfigRow,
  after: SecurityConfigRow | null,
  changes: Record<string, unknown>,
): void {
  if (after === null) {
    throw new Error('security config row is missing after the save');
  }
  if (Object.keys(changes).length === 0) {
    expect(after).toEqual(before);
    return;
  }
  expect(after).toEqual({ ...before, ...changes, updated_at: after.updated_at });
}

/** Returns the row without updated_at, which moves on every write. */
function withoutUpdatedAt(row: SecurityConfigRow): Omit<SecurityConfigRow, 'updated_at'> {
  return Object.fromEntries(Object.entries(row).filter(([key]) => key !== 'updated_at')) as Omit<
    SecurityConfigRow,
    'updated_at'
  >;
}

test.describe('Security config save applies only the fields in the request', () => {
  test.describe.configure({ mode: 'serial' });

  let context: APIRequestContext;
  let originalRow: SecurityConfigRow | null;
  let originalSettings: Record<string, string>;

  test.beforeAll(async () => {
    context = await createAuthedContext();
    originalRow = await getConfigRow(context);
    originalSettings = await readModuleSettings(context);
  });

  test.afterAll(async () => {
    try {
      // Force both module keys off first, so the restore below always has a real change to put back.
      await saveConfig(context, { name: DEFAULT_CONFIG_NAME, waf_mode: 'disabled', rate_limit_enable: false });
      // The row goes first: a save that enables a module also turns on the framework, so the switches are written last.
      if (originalRow) {
        await saveConfig(context, bodyFromRow(originalRow));
        const restored = await getConfigRow(context);
        expect(restored && withoutUpdatedAt(restored)).toEqual(withoutUpdatedAt(originalRow));
      }
      await restoreModuleSwitches(context, originalSettings);
      expect(effectiveOf(await readModuleSettings(context))).toEqual(effectiveOf(originalSettings));
    } finally {
      await context.dispose();
    }
  });

  /** Turns on the settings overrides the Rate Limiting and Security pages need before they show their controls. */
  async function showConfigControls(): Promise<void> {
    await writeSettingValue(context, 'feature.cerberus.enabled', 'true');
    await writeSettingValue(context, 'security.rate_limit.enabled', 'true');
  }

  test('API save of only the three rate-limit numbers changes only those numbers', async () => {
    const before = await seedState(context);

    await test.step('Save the three rate-limit numbers only', async () => {
      await saveConfig(context, {
        rate_limit_requests: 20,
        rate_limit_burst: 4,
        rate_limit_window_sec: 30,
      });
    });

    await test.step('Stored row changes only in the three numbers', async () => {
      const after = await getConfigRow(context);
      expectOnlyChanged(before.row, after, {
        rate_limit_requests: 20,
        rate_limit_burst: 4,
        rate_limit_window_sec: 30,
      });
    });

    await test.step('Status is unchanged', async () => {
      expect(await getStatus(context)).toEqual(before.status);
    });
  });

  test('API save of only the admin allow-list changes only that field', async () => {
    const before = await seedState(context);
    const newWhitelist = `${RUNNER_WHITELIST},${EXTRA_WHITELIST_CIDR}`;

    await test.step('Save the admin allow-list only', async () => {
      await saveConfig(context, { name: DEFAULT_CONFIG_NAME, admin_whitelist: newWhitelist });
    });

    await test.step('Stored row changes only in the allow-list', async () => {
      const after = await getConfigRow(context);
      expectOnlyChanged(before.row, after, { admin_whitelist: newWhitelist });
    });

    await test.step('Status is unchanged', async () => {
      expect(await getStatus(context)).toEqual(before.status);
    });
  });

  test('Saving the stored row back with one field edited changes no module switch', async () => {
    const before = await seedState(context);
    const switchesBefore = await readModuleSettings(context);

    await test.step('Save the stored row back with one unrelated field edited', async () => {
      const row = await getConfigRow(context);
      expect(row).not.toBeNull();
      await saveConfig(context, { ...bodyFromRow(row as SecurityConfigRow), waf_learning: !before.row.waf_learning });
    });

    await test.step('Module switches are unchanged and only the edited field is stored', async () => {
      expect(await readModuleSettings(context)).toEqual(switchesBefore);
      const after = await getConfigRow(context);
      expectOnlyChanged(before.row, after, { waf_learning: !before.row.waf_learning });
    });
  });

  test('Rate Limiting page save changes only the three rate-limit numbers', async ({ page }) => {
    await showConfigControls();
    const before = await seedState(context);

    await test.step('Open the Rate Limiting page', async () => {
      await page.goto('/security/rate-limiting');
      await waitForLoadingComplete(page);
      await expect(page.getByRole('heading', { name: 'Rate Limiting Configuration', exact: true })).toBeVisible();
    });

    await test.step('Enter new rate-limit numbers and save', async () => {
      await page.getByLabel(/requests per second/i).fill('20');
      await page.getByLabel('Burst', { exact: true }).fill('4');
      await page.getByLabel(/window \(seconds\)/i).fill('30');
      // The button's translated label is not in the English catalogue, so it is located by its test id.
      await saveAndWait(page, page.getByTestId('save-rate-limit-btn'));
    });

    await test.step('Stored row changes only in the three numbers', async () => {
      const after = await getConfigRow(context);
      expectOnlyChanged(before.row, after, {
        rate_limit_requests: 20,
        rate_limit_burst: 4,
        rate_limit_window_sec: 30,
      });
    });

    await test.step('Status is unchanged', async () => {
      expect(await getStatus(context)).toEqual(before.status);
    });
  });

  test('Security page allow-list save changes only the admin allow-list', async ({ page }) => {
    await showConfigControls();
    const before = await seedState(context);
    const newWhitelist = `${RUNNER_WHITELIST},${EXTRA_WHITELIST_CIDR}`;

    await test.step('Open the Security page', async () => {
      await page.goto('/security');
      await waitForLoadingComplete(page);
    });

    const allowListInput = page.getByPlaceholder('192.168.1.0/24, 10.0.0.1');

    await test.step('Edit the admin allow-list and save', async () => {
      await allowListInput.fill(newWhitelist);
      // The Save button sits beside the input in the same card; there is no label association to query on.
      const allowListSave = allowListInput.locator('xpath=..').getByRole('button', { name: 'Save', exact: true });
      await saveAndWait(page, allowListSave);
    });

    await test.step('Stored row changes only in the allow-list', async () => {
      const after = await getConfigRow(context);
      expectOnlyChanged(before.row, after, { admin_whitelist: newWhitelist });
    });

    await test.step('Status is unchanged', async () => {
      expect(await getStatus(context)).toEqual(before.status);
    });
  });

  test('explicit empty allow-list clears it and changes no other field', async () => {
    // Cerberus is off for this test so clearing the allow-list cannot block the runner.
    const before = await seedState(context, { enabled: false });

    await test.step('Save an empty allow-list', async () => {
      await saveConfig(context, { admin_whitelist: '' });
    });

    await test.step('Stored row changes only in the allow-list', async () => {
      const after = await getConfigRow(context);
      expectOnlyChanged(before.row, after, { admin_whitelist: '' });
    });

    await test.step('Status is unchanged', async () => {
      expect(await getStatus(context)).toEqual(before.status);
    });
  });

  test('explicit false and zero values are stored', async () => {
    const before = await seedState(context);

    await test.step('Save false and zero values', async () => {
      await saveConfig(context, {
        enabled: false,
        waf_learning: false,
        rate_limit_requests: 0,
      });
    });

    await test.step('Stored row changes only in the explicit values', async () => {
      const after = await getConfigRow(context);
      expectOnlyChanged(before.row, after, {
        enabled: false,
        waf_learning: false,
        rate_limit_requests: 0,
      });
    });
  });

  test('an invalid WAF mode is refused and changes nothing', async () => {
    const before = await seedState(context);

    await test.step('Save an unknown WAF mode', async () => {
      const response = await withRateLimitRetry(() =>
        context.post(CONFIG_PATH, { data: { name: DEFAULT_CONFIG_NAME, waf_mode: 'foo' } }),
      );
      expect(response.status()).toBe(400);
    });

    await test.step('Stored row and status are unchanged', async () => {
      expectOnlyChanged(before.row, await getConfigRow(context), {});
      expect(await getStatus(context)).toEqual(before.status);
    });
  });

  test('a WAF rule set name that matches no rule set is refused and changes nothing', async () => {
    const before = await seedState(context);

    await test.step('Save an unknown rule set name', async () => {
      const response = await withRateLimitRetry(() =>
        context.post(CONFIG_PATH, { data: { name: DEFAULT_CONFIG_NAME, waf_rules_source: `missing-rule-set-${Date.now()}` } }),
      );
      expect(response.status()).toBe(422);
    });

    await test.step('Stored row and status are unchanged', async () => {
      expectOnlyChanged(before.row, await getConfigRow(context), {});
      expect(await getStatus(context)).toEqual(before.status);
    });
  });

  test('a body whose keys are all null changes nothing', async () => {
    const before = await seedState(context);

    await test.step('Save null values only', async () => {
      await saveConfig(context, {
        enabled: null,
        admin_whitelist: null,
        rate_limit_requests: null,
        rate_limit_burst: null,
        rate_limit_window_sec: null,
      });
    });

    await test.step('Stored row is identical, including updated_at', async () => {
      expectOnlyChanged(before.row, await getConfigRow(context), {});
    });
  });

  test('a body with no updatable key changes nothing', async () => {
    const before = await seedState(context);

    for (const body of [{}, { name: DEFAULT_CONFIG_NAME }, { crowdsec_mode: 'local' }]) {
      await test.step(`Save ${JSON.stringify(body)}`, async () => {
        await saveConfig(context, body);
        expectOnlyChanged(before.row, await getConfigRow(context), {});
      });
    }

    await test.step('Status is unchanged', async () => {
      expect(await getStatus(context)).toEqual(before.status);
    });
  });
});

/**
 * Clicks a save control and waits for the config save response, so the assertions run after the write lands.
 */
async function saveAndWait(page: Page, saveControl: Locator): Promise<void> {
  const saveResponse = page.waitForResponse(
    (response) =>
      response.url().includes(CONFIG_PATH) && response.request().method() === 'POST',
  );
  await saveControl.click();
  const response = await saveResponse;
  expect(response.ok(), `save failed with ${response.status()}`).toBeTruthy();
}

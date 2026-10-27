/**
 * Security settings API helpers for specs that change the stored security configuration.
 *
 * Writes go through the same endpoints the admin UI uses. {@link useSecurityStateRestore} captures the
 * module switches, the stored config row and the rule sets before each test and puts them back after it,
 * so a test never leaves the emergency endpoint's changes or its own seeded values behind.
 */

import { expect, request as playwrightRequest, test, type APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import { EMERGENCY_TOKEN } from '../fixtures/security';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
export const PROXY_HOSTS_PATH = '/api/v1/proxy-hosts';
export const CONFIG_PATH = '/api/v1/security/config';
export const STATUS_PATH = '/api/v1/security/status';
export const SETTINGS_PATH = '/api/v1/settings';
export const RULESETS_PATH = '/api/v1/security/rulesets';
export const EMERGENCY_RESET_PATH = '/api/v1/emergency/security-reset';
export const WAF_ENABLE_PATH = '/api/v1/security/waf/enable';
export const RATE_LIMIT_ENABLE_PATH = '/api/v1/security/rate-limit/enable';

/** Whitelist that keeps the test runner reachable while Cerberus is on. */
export const RUNNER_WHITELIST = '127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16';

/** Settings keys that hold the security module switches. The settings API accepts only true or false for them. */
export const SECURITY_MODULE_SWITCH_KEYS = [
  'security.waf.enabled',
  'security.rate_limit.enabled',
  'security.acl.enabled',
] as const;

/** Settings keys that hold the module switches, including the Cerberus feature switch. */
export const MODULE_SETTING_KEYS = ['feature.cerberus.enabled', ...SECURITY_MODULE_SWITCH_KEYS] as const;

/** Stored config fields a save may carry. The restore body is built from these. */
export const CONFIG_FIELDS = [
  'name',
  'enabled',
  'admin_whitelist',
  'crowdsec_api_url',
  'waf_mode',
  'waf_rules_source',
  'waf_learning',
  'waf_paranoia_level',
  'waf_exclusions',
  'rate_limit_mode',
  'rate_limit_enable',
  'rate_limit_burst',
  'rate_limit_requests',
  'rate_limit_window_sec',
  'rate_limit_bypass_list',
] as const;

export type SecurityConfigRow = Record<string, unknown> & { updated_at: string };
export type SettingValues = Record<string, string>;

export interface EnforcementState {
  enabled: boolean;
  enforced?: boolean;
  reason?: string;
}

export interface SecurityStatusBody {
  waf: EnforcementState;
  rate_limit: EnforcementState;
  [key: string]: unknown;
}

export interface SecurityStateSnapshot {
  config: SecurityConfigRow | null;
  settings: SettingValues;
  ruleSetNames: string[];
}

interface RuleSetListEntry {
  /** The list exposes the uuid; the numeric row id is not serialised. */
  uuid?: string;
  name: string;
}

/** Retries a request the API rate-limits (HTTP 429), like the other security helpers. */
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

export async function getConfigRow(request: APIRequestContext): Promise<SecurityConfigRow | null> {
  const response = await withRateLimitRetry(() => request.get(CONFIG_PATH));
  expect(response.ok(), `GET ${CONFIG_PATH} failed with ${response.status()}`).toBeTruthy();
  const body = (await response.json()) as { config: SecurityConfigRow | null };
  return body.config;
}

export async function saveConfig(request: APIRequestContext, body: Record<string, unknown>): Promise<void> {
  const response = await withRateLimitRetry(() => request.post(CONFIG_PATH, { data: body }));
  expect(response.ok(), `POST ${CONFIG_PATH} failed with ${response.status()}: ${await response.text()}`).toBeTruthy();
}

export async function getSecurityStatus(request: APIRequestContext): Promise<SecurityStatusBody> {
  const response = await withRateLimitRetry(() => request.get(STATUS_PATH));
  expect(response.ok(), `GET ${STATUS_PATH} failed with ${response.status()}`).toBeTruthy();
  return (await response.json()) as SecurityStatusBody;
}

/** Reads the module switch keys. An absent key is returned as an empty string, the same way the settings API stores it. */
export async function readModuleSettings(request: APIRequestContext): Promise<SettingValues> {
  const response = await withRateLimitRetry(() => request.get(SETTINGS_PATH));
  expect(response.ok(), `GET ${SETTINGS_PATH} failed with ${response.status()}`).toBeTruthy();
  const body = (await response.json()) as Record<string, unknown>;
  return Object.fromEntries(
    MODULE_SETTING_KEYS.map((key) => [key, typeof body[key] === 'string' ? body[key] : '']),
  );
}

/**
 * Puts the module switches back to the values captured before a test. A security module key captured as '' is absent:
 * the settings API cannot delete it, so it is left alone here and the stored row restore decides its final value.
 */
export async function restoreModuleSwitches(request: APIRequestContext, settings: SettingValues): Promise<void> {
  for (const [key, value] of Object.entries(settings)) {
    const isSecurityModuleKey = (SECURITY_MODULE_SWITCH_KEYS as readonly string[]).includes(key);
    if (isSecurityModuleKey && value === '') {
      continue;
    }
    await writeSettingValue(request, key, value);
  }
}

export async function writeSettingValue(request: APIRequestContext, key: string, value: string): Promise<void> {
  const response = await withRateLimitRetry(() => request.post(SETTINGS_PATH, { data: { key, value } }));
  expect(response.ok(), `POST ${SETTINGS_PATH} ${key} failed with ${response.status()}`).toBeTruthy();
}

/** Calls the emergency endpoint with the break-glass token. Only the endpoint's own status is checked. */
export async function runEmergencySecurityReset(request: APIRequestContext): Promise<void> {
  const response = await withRateLimitRetry(() =>
    request.post(EMERGENCY_RESET_PATH, { headers: { 'X-Emergency-Token': EMERGENCY_TOKEN } }),
  );
  expect(response.ok(), `POST ${EMERGENCY_RESET_PATH} failed with ${response.status()}`).toBeTruthy();
}

/** Enables a module through its toggle endpoint, the same call the admin page makes. */
export async function enableModule(request: APIRequestContext, path: string): Promise<void> {
  const response = await withRateLimitRetry(() => request.post(path));
  expect(response.ok(), `POST ${path} failed with ${response.status()}: ${await response.text()}`).toBeTruthy();
}

export async function createRuleSet(request: APIRequestContext, name: string): Promise<void> {
  const content = `SecRule ARGS "@contains e2e-probe" "id:990001,phase:2,deny,status:403,msg:'e2e test rule'"`;
  const response = await withRateLimitRetry(() => request.post(RULESETS_PATH, { data: { name, content } }));
  expect(response.ok(), `POST ${RULESETS_PATH} failed with ${response.status()}`).toBeTruthy();
}

async function listRuleSets(request: APIRequestContext): Promise<RuleSetListEntry[]> {
  const response = await withRateLimitRetry(() => request.get(RULESETS_PATH));
  expect(response.ok(), `GET ${RULESETS_PATH} failed with ${response.status()}`).toBeTruthy();
  const body = (await response.json()) as { rulesets: RuleSetListEntry[] };
  return body.rulesets;
}

/**
 * Deletes a rule set by name. DELETE takes the rule set uuid, so the list must expose `uuid`;
 * the helper fails loudly when it does not instead of leaving the rule set behind.
 */
async function deleteRuleSetByName(request: APIRequestContext, name: string): Promise<void> {
  const entry = (await listRuleSets(request)).find((ruleSet) => ruleSet.name === name);
  if (!entry) {
    return;
  }
  if (!entry.uuid) {
    throw new Error(`Rule set list does not expose a uuid, so "${name}" cannot be deleted`);
  }
  const response = await withRateLimitRetry(() => request.delete(`${RULESETS_PATH}/${entry.uuid}`));
  expect(response.ok(), `DELETE rule set ${name} failed with ${response.status()}`).toBeTruthy();
}

/** Builds a save body from a stored row, limited to the fields a config save accepts. */
export function bodyFromRow(row: SecurityConfigRow): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  for (const field of CONFIG_FIELDS) {
    body[field] = row[field];
  }
  return body;
}

export async function snapshotSecurityState(request: APIRequestContext): Promise<SecurityStateSnapshot> {
  const ruleSets = await listRuleSets(request);
  return {
    config: await getConfigRow(request),
    settings: await readModuleSettings(request),
    ruleSetNames: ruleSets.map((ruleSet) => ruleSet.name),
  };
}

/**
 * Restores the module switches, then the stored row, then removes rule sets the test added.
 * The rule set order matters: a selected rule set cannot be deleted, so the row is restored first.
 */
export async function restoreSecurityState(
  request: APIRequestContext,
  snapshot: SecurityStateSnapshot,
): Promise<void> {
  // The row goes first: a save that enables a module also turns on the framework, so the switches are written last.
  if (snapshot.config) {
    await saveConfig(request, bodyFromRow(snapshot.config));
  }
  await restoreModuleSwitches(request, snapshot.settings);
  for (const ruleSet of await listRuleSets(request)) {
    if (!snapshot.ruleSetNames.includes(ruleSet.name)) {
      await deleteRuleSetByName(request, ruleSet.name);
    }
  }
}

/**
 * Captures the security state before each test in the enclosing describe and restores it after each test,
 * including when the test fails. Call it inside a `test.describe` block.
 */
export function useSecurityStateRestore(): void {
  let snapshot: SecurityStateSnapshot | undefined;

  test.beforeEach(async ({ request }) => {
    snapshot = await snapshotSecurityState(request);
  });

  test.afterEach(async ({ request }) => {
    if (snapshot) {
      await restoreSecurityState(request, snapshot);
      snapshot = undefined;
    }
  });
}

/**
 * Creates one enabled proxy host for the enclosing describe and deletes it afterwards. The status reports a reason or
 * `enforced` only for an eligible host, so the enforcement assertions need one. Call it inside a `test.describe` block.
 */
export function useEligibleProxyHost(): void {
  const domain = `security-status-${Date.now()}.e2e.test`;
  let uuid: string | undefined;

  test.beforeAll(async () => {
    const context = await playwrightRequest.newContext({ baseURL: BASE_URL, storageState: STORAGE_STATE });
    try {
      // The host only has to exist and be enabled: the status reads the proxy configuration, nothing is sent to it.
      const response = await context.post(PROXY_HOSTS_PATH, {
        data: { name: domain, domain_names: domain, forward_scheme: 'http', forward_host: '127.0.0.1', forward_port: 9, enabled: true },
      });
      expect(response.ok(), `POST ${PROXY_HOSTS_PATH} failed with ${response.status()}: ${await response.text()}`).toBeTruthy();
      uuid = ((await response.json()) as { uuid: string }).uuid;
    } finally {
      await context.dispose();
    }
  });

  test.afterAll(async () => {
    if (!uuid) {
      return;
    }
    const context = await playwrightRequest.newContext({ baseURL: BASE_URL, storageState: STORAGE_STATE });
    try {
      const response = await context.delete(`${PROXY_HOSTS_PATH}/${uuid}`);
      expect(response.ok() || response.status() === 404, `DELETE ${PROXY_HOSTS_PATH} failed with ${response.status()}`).toBeTruthy();
    } finally {
      await context.dispose();
    }
  });
}

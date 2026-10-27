/**
 * WAF and rate limit enforcement status
 *
 * Covers the status fields that report whether the proxy applies the WAF and rate limit settings, and the
 * configuration writes that feed them. Every test restores the module switches, the stored config row and
 * any rule set it created.
 *
 * All tests are fixme until the behaviour lands later in this PR (see PENDING_DETAILS).
 */

import { test, expect } from '@playwright/test';
import {
  PENDING_DETAILS,
  RATE_LIMIT_ENABLE_PATH,
  RUNNER_WHITELIST,
  type SecurityConfigRow,
  type SecurityStatusBody,
  bodyFromRow,
  createRuleSet,
  enableModule,
  getConfigRow,
  getSecurityStatus,
  readModuleSettings,
  saveConfig,
  useSecurityStateRestore,
} from '../utils/security-settings-api';

const RULE_SET_PREFIX = `waf-status-${Date.now()}`;

test.describe('WAF and rate limit enforcement status', () => {
  test.describe.configure({ mode: 'serial' });
  useSecurityStateRestore();

  test.fixme(
    'Second rule set with none selected reports the reason, and choosing one reports the WAF as applied',
    PENDING_DETAILS,
    async ({ request }) => {
      const firstName = `${RULE_SET_PREFIX}-a`;
      const secondName = `${RULE_SET_PREFIX}-b`;

      await test.step('Create two rule sets', async () => {
        await createRuleSet(request, firstName);
        await createRuleSet(request, secondName);
      });

      await test.step('Save WAF block mode without choosing a rule set', async () => {
        await saveConfig(request, { name: 'default', admin_whitelist: RUNNER_WHITELIST, enabled: true, waf_mode: 'block' });

        const status = await getSecurityStatus(request);
        expect(status.waf.enabled).toBe(true);
        expect(status.waf.enforced).toBe(false);
        expect(status.waf.reason).toBe('ruleset_not_selected');
      });

      await test.step('Choose one rule set and verify the WAF is reported as applied', async () => {
        await saveConfig(request, { name: 'default', waf_rules_source: firstName });

        const status = await getSecurityStatus(request);
        expect(status.waf.enforced).toBe(true);
        expect(status.waf.reason).toBeUndefined();
      });
    },
  );

  test.fixme(
    'Rate limit toggled on with unset thresholds writes the standard values and reports them as applied',
    PENDING_DETAILS,
    async ({ request }) => {
      await test.step('Seed unset thresholds with the rate limit off', async () => {
        await saveConfig(request, {
          name: 'default',
          admin_whitelist: RUNNER_WHITELIST,
          rate_limit_enable: false,
          rate_limit_requests: 0,
          rate_limit_window_sec: 0,
          rate_limit_burst: 0,
        });
      });

      await test.step('Turn the rate limit on through its toggle', async () => {
        await enableModule(request, RATE_LIMIT_ENABLE_PATH);
      });

      await test.step('Stored row carries the standard values and the status reports them as applied', async () => {
        const row = await getConfigRow(request);
        expect(row).toMatchObject({
          rate_limit_requests: 100,
          rate_limit_window_sec: 60,
          rate_limit_burst: 20,
        });

        const status = await getSecurityStatus(request);
        expect(status.rate_limit.enabled).toBe(true);
        expect(status.rate_limit.enforced).toBe(true);
      });
    },
  );

  test.fixme(
    'Rate limit already on with unset thresholds reports the thresholds-unset reason and keeps the stored values',
    PENDING_DETAILS,
    async ({ request }) => {
      await test.step('Save the rate limit on with zero thresholds', async () => {
        await saveConfig(request, {
          name: 'default',
          admin_whitelist: RUNNER_WHITELIST,
          rate_limit_enable: true,
          rate_limit_requests: 0,
          rate_limit_window_sec: 0,
          rate_limit_burst: 0,
        });
      });

      await test.step('Status reports the rate limit as not applied with the thresholds-unset reason', async () => {
        const status = await getSecurityStatus(request);
        expect(status.rate_limit.enabled).toBe(true);
        expect(status.rate_limit.enforced).toBe(false);
        expect(status.rate_limit.reason).toBe('thresholds_unset');
      });

      await test.step('Stored thresholds are left as they were', async () => {
        const row = await getConfigRow(request);
        expect(row).toMatchObject({
          rate_limit_requests: 0,
          rate_limit_window_sec: 0,
          rate_limit_burst: 0,
        });
      });
    },
  );

  test.fixme(
    'Saving the stored config back with one field edited leaves the module switches unchanged',
    PENDING_DETAILS,
    async ({ request }) => {
      let switchesBefore: Record<string, string> = {};
      let statusBefore: SecurityStatusBody | undefined;
      let editedParanoiaLevel = 0;

      await test.step('Capture the module switches, the status and the stored row', async () => {
        switchesBefore = await readModuleSettings(request);
        statusBefore = await getSecurityStatus(request);
      });

      await test.step('Save the stored row back with one unrelated field edited', async () => {
        const row: SecurityConfigRow | null = await getConfigRow(request);
        expect(row).not.toBeNull();
        if (!row) {
          return;
        }
        const currentLevel = Number(row.waf_paranoia_level ?? 1);
        editedParanoiaLevel = currentLevel === 1 ? 2 : 1;

        await saveConfig(request, { ...bodyFromRow(row), waf_paranoia_level: editedParanoiaLevel });
      });

      await test.step('Module switches and the module status are unchanged', async () => {
        expect(await readModuleSettings(request)).toEqual(switchesBefore);

        const statusAfter = await getSecurityStatus(request);
        expect(statusAfter.waf.enabled).toBe(statusBefore?.waf.enabled);
        expect(statusAfter.rate_limit.enabled).toBe(statusBefore?.rate_limit.enabled);
      });

      await test.step('The edited field is stored', async () => {
        const row = await getConfigRow(request);
        expect(row).toMatchObject({ waf_paranoia_level: editedParanoiaLevel });
      });
    },
  );
});

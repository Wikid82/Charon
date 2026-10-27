/**
 * WAF and rate limit enforcement status
 *
 * Covers the status fields that report whether the proxy applies the WAF and rate limit settings, the passive
 * lines the admin pages show for them, and the configuration writes that feed them. The status reports a reason
 * only for an eligible host that did not get the handler, so the suite creates one proxy host for its lifetime.
 * Every test restores the module switches, the stored config row and any rule set it created.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import {
  type SecurityConfigRow,
  type SecurityStatusBody,
  RUNNER_WHITELIST,
  RATE_LIMIT_ENABLE_PATH,
  bodyFromRow,
  createRuleSet,
  enableModule,
  getConfigRow,
  getSecurityStatus,
  readModuleSettings,
  saveConfig,
  useEligibleProxyHost,
  useSecurityStateRestore,
  writeSettingValue,
} from '../utils/security-settings-api';

const RULE_SET_PREFIX = `waf-status-${Date.now()}`;

const WAF_PASSIVE_LINE_SEVERAL_RULE_SETS =
  'The WAF is on, but several rule sets exist and none is selected. Choose the rule set on the WAF page.';

test.describe('WAF and rate limit enforcement status', () => {
  test.describe.configure({ mode: 'serial' });
  useEligibleProxyHost();

  test.describe('Status fields', () => {
    useSecurityStateRestore();

    test('Second rule set with none selected reports the reason, and choosing one reports the WAF as applied', async ({ request }) => {
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
    });

    test('Rate limit toggled on with unset thresholds writes the standard values and reports them as applied', async ({ request }) => {
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
    });

    test('Rate limit already on with unset thresholds reports the thresholds-unset reason and keeps the stored values', async ({ request }) => {
      await test.step('Turn the rate limit on with valid thresholds', async () => {
        await saveConfig(request, {
          name: 'default',
          admin_whitelist: RUNNER_WHITELIST,
          rate_limit_enable: true,
          rate_limit_requests: 100,
          rate_limit_window_sec: 60,
          rate_limit_burst: 20,
        });
      });

      await test.step('Save zero thresholds while the rate limit stays on', async () => {
        // The switch is already on, so this save writes no key and no thresholds are filled in.
        await saveConfig(request, {
          name: 'default',
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
    });

    test('Saving the stored config back with one field edited leaves the module switches unchanged', async ({ request }) => {
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
    });
  });

  test.describe('Passive lines on the admin pages', () => {
    useSecurityStateRestore();

    test.beforeEach(async ({ page, adminUser }) => {
      await loginUser(page, adminUser);
      await waitForLoadingComplete(page);
    });

    test('Security page shows the passive reason line for a WAF with several rule sets and none selected', async ({ page, request }) => {
      await test.step('Create two rule sets and save WAF block mode with none selected', async () => {
        await createRuleSet(request, `${RULE_SET_PREFIX}-ui-a`);
        await createRuleSet(request, `${RULE_SET_PREFIX}-ui-b`);
        await writeSettingValue(request, 'feature.cerberus.enabled', 'true');
        await saveConfig(request, { name: 'default', admin_whitelist: RUNNER_WHITELIST, enabled: true, waf_mode: 'block', waf_rules_source: '' });
      });

      await test.step('The WAF card on the Security page carries the passive line', async () => {
        await page.goto('/security');
        await waitForLoadingComplete(page);
        await expect(page.getByText(WAF_PASSIVE_LINE_SEVERAL_RULE_SETS)).toBeVisible();
      });
    });

    test('WAF page shows the chosen rule set source, and the passive reason once it is cleared', async ({ page, request }) => {
      const firstName = `${RULE_SET_PREFIX}-page-a`;

      await test.step('Create two rule sets and choose the first one', async () => {
        await createRuleSet(request, firstName);
        await createRuleSet(request, `${RULE_SET_PREFIX}-page-b`);
        await saveConfig(request, { name: 'default', admin_whitelist: RUNNER_WHITELIST, enabled: true, waf_mode: 'block', waf_rules_source: firstName });
      });

      await test.step('The rule set source select shows the chosen rule set', async () => {
        await page.goto('/security/waf');
        await waitForLoadingComplete(page);
        await expect(page.getByLabel('Rule set source')).toHaveValue(firstName);
      });

      await test.step('Clearing the rule set source shows the passive reason on the WAF page', async () => {
        await saveConfig(request, { name: 'default', waf_rules_source: '' });
        await page.reload();
        await waitForLoadingComplete(page);
        await expect(page.getByLabel('Rule set source')).toHaveValue('');
        await expect(page.getByText(WAF_PASSIVE_LINE_SEVERAL_RULE_SETS)).toBeVisible();
      });
    });
  });
});

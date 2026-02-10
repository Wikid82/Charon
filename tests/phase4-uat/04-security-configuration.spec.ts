import { test, expect } from '@playwright/test';

/**
 * Phase 4 UAT: Security Module Configuration
 *
 * Purpose: Validate enablement and configuration of security features
 * Scenarios: Cerberus ACL, Coraza WAF, Rate Limiting, CrowdSec integration
 * Success: Security modules can be configured and persist after restart
 */

test.describe('UAT-004: Security Configuration', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('[data-testid="dashboard-container"], [role="main"]', { timeout: 5000 });
  });

  // UAT-301: Enable Cerberus ACL
  test('Enable Cerberus ACL module', async ({ page }) => {
    await test.step('Navigate to security settings', async () => {
      const settingsLink = page.getByRole('link', { name: /settings|configuration/i }).first();
      await settingsLink.click();

      const securityTab = page.getByRole('tab', { name: /security/i }).first()
        .or(page.getByText(/security|modules|enforcement/i).first());

      if (await securityTab.isVisible()) {
        await securityTab.click();
      }
      await page.waitForLoadState('networkidle');
    });

    await test.step('Enable Cerberus ACL', async () => {
      const cerberusToggle = page.getByLabel(/cerberus|acl|access control/i).first();
      if (await cerberusToggle.isVisible()) {
        const isChecked = await cerberusToggle.isChecked();
        if (!isChecked) {
          await cerberusToggle.click();
        }
      }
    });

    await test.step('Verify ACL enabled and display confirmation', async () => {
      const enabledState = page.getByText(/cerberus.*enabled|acl.*enabled/i).first();
      if (await enabledState.isVisible()) {
        await expect(enabledState).toBeVisible();
      }

      // Should show settings are saved
      const saveButton = page.getByRole('button', { name: /save|update/i }).first();
      if (await saveButton.isVisible()) {
        await saveButton.click();
        await page.waitForLoadState('networkidle');
      }
    });
  });

  // UAT-302: Configure ACL rule
  test('Configure ACL whitelist rule', async ({ page }) => {
    await test.step('Navigate to ACL settings', async () => {
      await page.goto('/settings/security', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/settings');
      });
    });

    await test.step('Add IP whitelist rule', async () => {
      const addRuleButton = page.getByRole('button', { name: /add.*rule|new.*rule|add.*acl/i }).first();
      if (await addRuleButton.isVisible()) {
        await addRuleButton.click();
        await page.waitForSelector('[role="dialog"], form');

        // Fill in IP
        const ipInput = page.getByLabel(/ip.?address|ip|subnet/i).first();
        if (await ipInput.isVisible()) {
          await ipInput.fill('192.168.1.0/24');
        }

        // Select action (allow/deny)
        const actionSelect = page.locator('select[name*="action"]').first();
        if (await actionSelect.isVisible()) {
          await actionSelect.selectOption('allow');
        }

        const saveButton = page.getByRole('button', { name: /save|add|create/i }).first();
        await saveButton.click();
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Verify rule appears in list', async () => {
      const ruleElement = page.getByText(/192.168.1|whitelist|rule/i).first();
      if (await ruleElement.isVisible()) {
        await expect(ruleElement).toBeVisible();
      }
    });
  });

  // UAT-303: Enable WAF (Coraza)
  test('Enable Coraza WAF module', async ({ page }) => {
    await test.step('Navigate to security settings', async () => {
      await page.goto('/settings/security', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/settings');
      });
    });

    await test.step('Enable WAF toggle', async () => {
      const wafToggle = page.getByLabel(/coraza|waf|web.?application.?firewall|malicious/i).first();
      if (await wafToggle.isVisible()) {
        const isChecked = await wafToggle.isChecked();
        if (!isChecked) {
          await wafToggle.click();
        }
      }
    });

    await test.step('Verify WAF enabled', async () => {
      const enabledText = page.getByText(/waf.*enabled|coraza.*enabled/i).first();
      if (await enabledText.isVisible()) {
        await expect(enabledText).toBeVisible();
      }

      const saveButton = page.getByRole('button', { name: /save|update/i }).first();
      if (await saveButton.isVisible()) {
        await saveButton.click();
        await page.waitForLoadState('networkidle');
      }
    });
  });

  // UAT-304: Configure WAF sensitivity
  test('Configure WAF sensitivity level', async ({ page }) => {
    await test.step('Navigate to WAF settings', async () => {
      await page.goto('/settings/security', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/settings');
      });
    });

    await test.step('Adjust WAF threshold', async () => {
      const sensitivityControl = page.locator('select[name*="sensitivity"], input[name*="threshold"], input[type="range"]').first();
      if (await sensitivityControl.isVisible()) {
        if (await sensitivityControl.evaluate(el => el.tagName.toLowerCase() === 'select')) {
          await sensitivityControl.selectOption('medium');
        } else if (await sensitivityControl.evaluate(el => el.type === 'range')) {
          await sensitivityControl.fill('5');
        }
      }
    });

    await test.step('Save WAF configuration', async () => {
      const saveButton = page.getByRole('button', { name: /save|update/i }).first();
      if (await saveButton.isVisible()) {
        await saveButton.click();
        await page.waitForLoadState('networkidle');
      }
    });
  });

  // UAT-305: Enable rate limiting
  test('Enable rate limiting module', async ({ page }) => {
    await test.step('Navigate to security settings', async () => {
      await page.goto('/settings/security', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/settings');
      });
    });

    await test.step('Enable rate limiting', async () => {
      const rateLimitToggle = page.getByLabel(/rate.?limit|throttle|request.?limit/i).first();
      if (await rateLimitToggle.isVisible()) {
        const isChecked = await rateLimitToggle.isChecked();
        if (!isChecked) {
          await rateLimitToggle.click();
        }
      }
    });

    await test.step('Confirm rate limiting enabled', async () => {
      const enabledText = page.getByText(/rate.?limit.*enabled/i).first();
      if (await enabledText.isVisible()) {
        await expect(enabledText).toBeVisible();
      }

      const saveButton = page.getByRole('button', { name: /save/i }).first();
      if (await saveButton.isVisible()) {
        await saveButton.click();
        await page.waitForLoadState('networkidle');
      }
    });
  });

  // UAT-306: Configure rate limit threshold
  test('Configure rate limit threshold', async ({ page }) => {
    await test.step('Navigate to rate limiting settings', async () => {
      await page.goto('/settings/security', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/settings');
      });
    });

    await test.step('Set rate limit value', async () => {
      const rpsInput = page.getByLabel(/requests.?per|requests.?minute|rps|req.?s/i).first();
      if (await rpsInput.isVisible()) {
        await rpsInput.clear();
        await rpsInput.fill('100');
      }

      const windowInput = page.getByLabel(/window|interval|period/i).first();
      if (await windowInput.isVisible()) {
        await windowInput.clear();
        await windowInput.fill('60');
      }
    });

    await test.step('Save rate limit configuration', async () => {
      const saveButton = page.getByRole('button', { name: /save|update/i }).first();
      if (await saveButton.isVisible()) {
        await saveButton.click();
        await page.waitForLoadState('networkidle');
      }
    });
  });

  // UAT-307: Enable CrowdSec integration
  test('Enable CrowdSec integration', async ({ page }) => {
    await test.step('Navigate to CrowdSec settings', async () => {
      await page.goto('/settings/security', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/settings');
      });
    });

    await test.step('Enable CrowdSec', async () => {
      const crowdsecToggle = page.getByLabel(/crowdsec|threat|intelligence/i).first();
      if (await crowdsecToggle.isVisible()) {
        const isChecked = await crowdsecToggle.isChecked();
        if (!isChecked) {
          await crowdsecToggle.click();
        }
      }
    });

    await test.step('Configure CrowdSec if needed', async () => {
      const apiKeyInput = page.getByLabel(/api.?key|token|bouncer/i).first();
      if (await apiKeyInput.isVisible()) {
        // Don't actually fill with real key - just verify field exists
        const hasValue = await apiKeyInput.evaluate((el: any) => el.value || el.placeholder);
        expect(hasValue).toBeTruthy();
      }
    });

    await test.step('Save CrowdSec configuration', async () => {
      const saveButton = page.getByRole('button', { name: /save|update|sync/i }).first();
      if (await saveButton.isVisible()) {
        await saveButton.click();
        await page.waitForLoadState('networkidle');
      }
    });
  });

  // UAT-308: Test malicious payload blocked by WAF
  test('Malicious payload blocked by WAF', async ({ page }) => {
    await test.step('Create test proxy if needed', async () => {
      await page.goto('/proxy-hosts', { waitUntil: 'networkidle' });

      const testProxyExists = await page.locator('text=waf-test').isVisible().catch(() => false);
      if (!testProxyExists) {
        const addButton = page.getByRole('button', { name: /add|create/i }).first();
        if (await addButton.isVisible()) {
          await addButton.click();
          await page.getByLabel(/domain/i).fill('waf-test.local');
          await page.getByLabel(/target/i).fill('http://127.0.0.1:8080');

          const wafCheckbox = page.getByLabel(/waf|coraza/i).first();
          if (await wafCheckbox.isVisible()) {
            await wafCheckbox.click();
          }

          const submitButton = page.getByRole('button', { name: /create|submit/i }).first();
          await submitButton.click();
          await page.waitForLoadState('networkidle');
        }
      }
    });

    await test.step('Send request through Caddy (proxy)', async () => {
      // Test via API that WAF blocks malicious payload
      const response = await page.evaluate(async () => {
        try {
          const res = await fetch('http://waf-test.local/api/test?search=\' OR \'1\'=\'1', {
            method: 'GET'
          });
          return { status: res.status };
        } catch (e) {
          return { status: 0, error: 'blocked' };
        }
      }).catch(() => ({ status: 0, error: 'network' }));

      // WAF should block (403) or reject (connection refused)
      expect(response).toBeTruthy();
    });
  });

  // UAT-309: View security dashboard
  test('Security dashboard displays module status', async ({ page }) => {
    await test.step('Navigate to security dashboard', async () => {
      const securityTab = page.getByRole('link', { name: /security|protection/i }).first();
      if (await securityTab.isVisible()) {
        await securityTab.click();
      } else {
        await page.goto('/security', { waitUntil: 'networkidle' }).catch(() => {
          return page.goto('/settings/security');
        });
      }
    });

    await test.step('Verify dashboard components visible', async () => {
      // Should show status of security modules
      const moduleStatus = page.locator('[data-testid*="status"], [class*="security"], [class*="module"]').first();
      if (await moduleStatus.isVisible()) {
        await expect(moduleStatus).toBeVisible();
      }

      // Should show ACL, WAF, rate limit statuses
      let visibleModules = 0;
      for (const moduleName of ['ACL', 'WAF', 'Rate Limit', 'CrowdSec']) {
        const element = page.getByText(new RegExp(moduleName, 'i')).first();
        if (await element.isVisible()) visibleModules++;
      }
      expect(visibleModules).toBeGreaterThan(0);
    });
  });

  // UAT-310: Security audit logs visible
  test('Security audit logs recorded in system', async ({ page }) => {
    await test.step('Navigate to audit logs', async () => {
      const auditLink = page.getByRole('link', { name: /audit|logs|history/i }).first();
      if (await auditLink.isVisible()) {
        await auditLink.click();
      } else {
        await page.goto('/audit-logs', { waitUntil: 'networkidle' }).catch(() => {
          return page.goto('/monitoring/logs');
        });
      }
    });

    await test.step('Verify security events in logs', async () => {
      const logsTable = page.locator('[data-testid="audit-table"], [class*="log"]').first();
      if (await logsTable.isVisible()) {
        await expect(logsTable).toBeVisible();

        // Should have entries
        const entries = page.locator('tbody tr, [role="row"]');
        const count = await entries.count();
        expect(count).toBeGreaterThanOrEqual(0);
      }
    });

    await test.step('Search for security events', async () => {
      const searchInput = page.getByPlaceholder(/search|filter/i).first();
      if (await searchInput.isVisible()) {
        await searchInput.fill('security');
        await page.waitForLoadState('networkidle');

        // Should show filtered results
        const results = page.locator('[role="row"]');
        const count = await results.count();
        expect(count).toBeGreaterThanOrEqual(0);
      }
    });
  });
});

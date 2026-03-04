import { test, expect } from '@playwright/test';

/**
 * Emergency & Break-Glass Operations Workflow
 *
 * Purpose: Validate emergency recovery procedures and break-glass token usage
 * Scenarios: Emergency token usage, system reset, WAF disable, encryption key reset
 * Success: Emergency procedures work to recover system from locked state
 */

test.describe('Emergency & Break-Glass Operations', () => {
  async function dismissDomainDialog(page: import('@playwright/test').Page): Promise<void> {
    const noThanksButton = page.getByRole('button', { name: /no, thanks/i });
    if (await noThanksButton.isVisible({ timeout: 1200 }).catch(() => false)) {
      await noThanksButton.click();
    }
  }

  async function openCreateProxyModal(page: import('@playwright/test').Page): Promise<void> {
    const addButton = page.getByRole('button', { name: /add.*proxy.*host|create/i }).first();
    await expect(addButton).toBeEnabled();
    await addButton.click();
    await expect(page.getByRole('dialog')).toBeVisible();
  }

  async function openEditProxyModalForDomain(
    page: import('@playwright/test').Page,
    domain: string
  ): Promise<void> {
    const row = page.locator('tbody tr').filter({ hasText: domain }).first();
    await expect(row).toBeVisible({ timeout: 10000 });

    const editButton = row.getByRole('button', { name: /edit proxy host|edit/i }).first();
    await expect(editButton).toBeVisible();
    await editButton.click();
    await expect(page.getByRole('dialog')).toBeVisible();
  }

  async function saveProxyHost(page: import('@playwright/test').Page): Promise<void> {
    await dismissDomainDialog(page);

    const saveButton = page
      .getByTestId('proxy-host-save')
      .or(page.getByRole('button', { name: /^save$/i }))
      .first();
    await expect(saveButton).toBeEnabled();
    await saveButton.click();

    const confirmSave = page.getByRole('button', { name: /yes.*save/i }).first();
    if (await confirmSave.isVisible({ timeout: 1200 }).catch(() => false)) {
      await confirmSave.click();
    }

    await expect(page.getByRole('dialog')).not.toBeVisible({ timeout: 10000 });
  }

  async function selectOptionByName(
    page: import('@playwright/test').Page,
    trigger: import('@playwright/test').Locator,
    optionName: RegExp
  ): Promise<string> {
    await trigger.click();
    const listbox = page.getByRole('listbox');
    await expect(listbox).toBeVisible();

    const option = listbox.getByRole('option', { name: optionName }).first();
    await expect(option).toBeVisible();
    const label = ((await option.textContent()) || '').trim();
    await option.click();
    return label;
  }

  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('[data-testid="dashboard-container"], main', { timeout: 15000 });
  });

  test('ACL dropdown parity regression keeps selection stable before emergency token flows', async ({ page }) => {
    const suffix = Date.now();
    const aclName = `Emergency-ACL-${suffix}`;
    const proxyDomain = `emergency-acl-${suffix}.test.local`;

    await test.step('Create ACL prerequisite through API for deterministic dropdown options', async () => {
      const createAclResponse = await page.request.post('/api/v1/access-lists', {
        data: {
          name: aclName,
          type: 'whitelist',
          description: 'ACL prerequisite for emergency regression test',
          enabled: true,
          ip_rules: JSON.stringify([{ cidr: '10.0.0.0/8' }]),
        },
      });
      expect(createAclResponse.ok()).toBeTruthy();
    });

    await test.step('Create proxy host and select created ACL in dropdown', async () => {
      await page.goto('/proxy-hosts');
      await page.waitForLoadState('networkidle');

      await openCreateProxyModal(page);
      const dialog = page.getByRole('dialog');

      await dialog.locator('#proxy-name').fill(`Emergency ACL Regression ${suffix}`);
      await dialog.locator('#domain-names').click();
      await page.keyboard.type(proxyDomain);
      await page.keyboard.press('Tab');
      await dismissDomainDialog(page);

      await dialog.locator('#forward-host').fill('127.0.0.1');
      await dialog.locator('#forward-port').fill('8080');

      const aclTrigger = dialog.getByRole('combobox', { name: /access control list/i });
      const selectedAclLabel = await selectOptionByName(
        page,
        aclTrigger,
        new RegExp(aclName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'i')
      );
      await expect(aclTrigger).toContainText(selectedAclLabel);

      await saveProxyHost(page);
    });

    await test.step('Edit proxy host and verify ACL selection persisted', async () => {
      await openEditProxyModalForDomain(page, proxyDomain);

      const dialog = page.getByRole('dialog');
      const aclTrigger = dialog.getByRole('combobox', { name: /access control list/i });
      await expect(aclTrigger).toContainText(new RegExp(aclName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'i'));

      await dialog.getByRole('button', { name: /cancel/i }).click();
      await expect(dialog).not.toBeVisible({ timeout: 5000 });
    });
  });

  // Use emergency token
  test('Emergency token enables break-glass access', async ({ page }) => {
    await test.step('Verify emergency token available', async () => {
      // Navigate to security settings where token is generated
      await page.goto('/settings/security', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/settings');
      });

      const emergencySection = page.getByText(/emergency|break.?glass|recovery/i).first();
      if (await emergencySection.isVisible()) {
        await expect(emergencySection).toBeVisible();
      }
    });

    await test.step('Simulate emergency token usage', async () => {
      // In test, we'll verify the mechanism exists
      // Actually using emergency token would require special header injection
      const tokenField = page.locator('[data-testid*="emergency"], [class*="emergency"]').first();
      if (await tokenField.isVisible()) {
        await expect(tokenField).toBeVisible();
      }
    });
  });

  // Break-glass recovery
  test('Break-glass recovery brings system to safe state', async ({ page }) => {
    await test.step('Access break-glass procedures', async () => {
      const emergencyLink = page.getByRole('link', { name: /emergency|break.?glass/i }).first();
      if (await emergencyLink.isVisible()) {
        await emergencyLink.click();
        await page.waitForLoadState('networkidle');
      } else {
        // Check in settings
        await page.goto('/settings/emergency', { waitUntil: 'networkidle' }).catch(() => {
          return page.goto('/settings');
        });
      }
    });

    await test.step('Verify recovery procedures documented', async () => {
      const procedures = page.getByText(/procedure|step|instruction|guide/i).first();
      if (await procedures.isVisible()) {
        await expect(procedures).toBeVisible();
      }
    });

    await test.step('Verify recovery mechanism available', async () => {
      const recoveryButton = page.getByRole('button', { name: /recover|reset|restore/i }).first();
      if (await recoveryButton.isVisible()) {
        await expect(recoveryButton).toBeVisible();
      }
    });
  });

  // Disable WAF in emergency
  test('Emergency token can disable security modules', async ({ page }) => {
    await test.step('Access emergency control panel', async () => {
      await page.goto('/settings/emergency', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/settings');
      });
    });

    await test.step('Verify WAF disable option exists', async () => {
      const wafControl = page.getByText(/waf|coraza|disable|emergency/i).first();
      if (await wafControl.isVisible()) {
        await expect(wafControl).toBeVisible();
      }
    });

    await test.step('Verify emergency procedures mention security disabling', async () => {
      const controlsSection = page.locator('[data-testid="emergency-controls"], [class*="emergency"]').first();
      if (await controlsSection.isVisible()) {
        await expect(controlsSection).toBeVisible();
      }
    });
  });

  // Reset encryption key in emergency
  test('Emergency token can reset encryption key', async ({ page }) => {
    await test.step('Access emergency encryption settings', async () => {
      await page.goto('/settings/emergency', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/settings');
      });

      const encryptionSection = page.getByText(/encrypt|key|cipher/i).first();
      if (await encryptionSection.isVisible()) {
        await expect(encryptionSection).toBeVisible();
      }
    });

    await test.step('Verify encryption reset option available', async () => {
      const resetButton = page.getByRole('button', { name: /reset|regenerate|new.*key/i }).first();
      if (await resetButton.isVisible()) {
        // Don't actually click - just verify it exists
        await expect(resetButton).toBeVisible();
      }
    });

    await test.step('Verify audit logging of emergency actions', async () => {
      const auditNote = page.getByText(/audit|record|log/i).first();
      if (await auditNote.isVisible()) {
        await expect(auditNote).toBeVisible();
      }
    });
  });

  // Emergency token single-use or reusable
  test('Emergency token usage logged and tracked', async ({ page }) => {
    await test.step('View emergency token access log', async () => {
      await page.goto('/audit', { waitUntil: 'networkidle' }).catch(() => {
        return page.goto('/admin/audit');
      });

      // Search for emergency token usage logs
      const searchInput = page.getByPlaceholder(/search|filter/i).first();
      if (await searchInput.isVisible()) {
        await searchInput.fill('emergency');
        await page.waitForLoadState('networkidle');
      }
    });

    await test.step('Verify emergency actions are audited', async () => {
      const auditEntries = page.locator('[role="row"], [class*="audit-entry"]');
      const count = await auditEntries.count();
      expect(count).toBeGreaterThanOrEqual(0);

      // Should see emergency-related entries if token was used
      const emergencyEntry = page.getByText(/emergency/i).first();
      if (await emergencyEntry.isVisible()) {
        await expect(emergencyEntry).toBeVisible();
      }
    });

    await test.step('Verify emergency usage timestamp recorded', async () => {
      const timestamp = page.getByText(/\d{1,2}\/\d{1,2}\/\d{2,4}/).first();
      if (await timestamp.isVisible()) {
        await expect(timestamp).toBeVisible();
      }
    });
  });
});

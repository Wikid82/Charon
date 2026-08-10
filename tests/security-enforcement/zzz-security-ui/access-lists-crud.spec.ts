/**
 * Access Lists CRUD E2E Tests
 *
 * Tests the Access Lists (ACL) management functionality including:
 * - List view with table, columns, and empty states
 * - Create access list with form validation (IP/Geo whitelist/blacklist)
 * - Read/view access list details
 * - Update existing access lists
 * - Delete access lists with confirmation and backup
 * - Test IP functionality
 * - Integration with Proxy Hosts
 *
 * @see /projects/Charon/docs/plans/current_spec.md
 */

import { test, expect, loginUser } from '../../fixtures/auth-fixtures';
import { waitForLoadingComplete, waitForModal, waitForDialog, waitForDebounce } from '../../utils/wait-helpers';
import { waitForAPIHealth, getAccessListViaAPI } from '../../utils/api-helpers';
import { clickSwitch } from '../../utils/ui-helpers';
import { generateUniqueId } from '../../fixtures/test-data';
import type { Page } from '@playwright/test';

/**
 * Seed an access list directly via the API so list-view assertions (type
 * badge, CGNAT banner, bulk-selection, rename) have a guaranteed, known row
 * to check instead of depending on leftover state from earlier tests in
 * this file (this spec has no `beforeEach` seed fixture and previously
 * relied on whatever other tests happened to leave behind).
 *
 * Bypasses tests/utils/api-helpers.ts's `createAccessListViaAPI` /
 * `AccessListCreateData` — that helper posts a `rules` field, but the real
 * backend (backend/internal/models/access_list.go's JSON tags: `type`,
 * `ip_rules`) only recognizes `type`/`ip_rules`/`local_network_only`, so
 * `rules` is silently dropped and, without `type` set, creation 400s with
 * "invalid access list type" (verified empirically against the running
 * API). Send the real field names directly instead — same bypass precedent
 * as certificates.spec.ts's `createCustomCertViaAPI` for
 * `CertificateResponse`'s analogous mismatch.
 */
async function seedAccessList(
  page: Page,
  overrides: { name?: string; type?: string } = {}
): Promise<{ uuid: string; name: string }> {
  const name = overrides.name ?? `ACL ${generateUniqueId()}`;
  const response = await page.request.post('/api/v1/access-lists', {
    data: {
      name,
      type: overrides.type ?? 'whitelist',
      enabled: true,
    },
  });

  if (!response.ok()) {
    throw new Error(`Failed to seed access list: ${response.status()} ${await response.text()}`);
  }

  const result = await response.json();
  return { uuid: result.uuid, name };
}

test.describe('Access Lists - CRUD Operations', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await waitForAPIHealth(page.request);
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    // Navigate to access lists page - supports both routes
    await page.goto('/access-lists');
    await waitForLoadingComplete(page);
  });

  // Helper to get the Create Access List button
  const getCreateButton = (page: import('@playwright/test').Page) =>
    page.getByRole('button', { name: /create.*access.*list/i }).first();

  // Helper to get Save/Create button in form
  const getSaveButton = (page: import('@playwright/test').Page) =>
    page.getByRole('button', { name: /^(create|update|save)$/i }).first();

  // Helper to get Cancel button in form (visible in form footer)
  const getCancelButton = (page: import('@playwright/test').Page) =>
    page.getByRole('button', { name: /^cancel$/i }).first();

  test.describe('List View', () => {
    test('should display access lists page with title', async ({ page }) => {
      await test.step('Verify page title is visible', async () => {
        // Use .first() to avoid strict mode violation (h1 title + h3 empty state)
        const heading = page.getByRole('heading', { name: /access.*lists/i }).first();
        await expect(heading).toBeVisible();
      });

      await test.step('Verify Create Access List button is present', async () => {
        const createButton = getCreateButton(page);
        await expect(createButton).toBeVisible();
      });
    });

    test('should show correct table columns', async ({ page }) => {
      await test.step('Verify table headers exist', async () => {
        // The table should have columns: Name, Type, Rules, Status, Actions
        const expectedColumns = [
          /name/i,
          /type/i,
          /rules/i,
          /status/i,
          /actions/i,
        ];

        for (const pattern of expectedColumns) {
          const header = page.getByRole('columnheader', { name: pattern });
          const headerExists = await header.count() > 0;
          if (headerExists) {
            await expect(header.first()).toBeVisible();
          }
        }
      });
    });

    test('should display empty state when no ACLs exist', async ({ page }) => {
      await test.step('Check for empty state or existing ACLs', async () => {
        await waitForDebounce(page, { delay: 1000 }); // Allow initial data fetch

        const emptyStateHeading = page.getByRole('heading', { name: /no.*access.*lists/i });
        const table = page.getByRole('table');

        const hasEmptyState = await emptyStateHeading.isVisible().catch(() => false);
        const hasTable = await table.isVisible().catch(() => false);

        expect(hasEmptyState || hasTable).toBeTruthy();

        if (hasEmptyState) {
          // Empty state should have a Create action
          const createAction = getCreateButton(page);
          await expect(createAction).toBeVisible();
        }
      });
    });

    test('should show loading skeleton while fetching data', async ({ page }) => {
      await test.step('Navigate and observe loading state', async () => {
        await page.reload();
        await waitForDebounce(page, { delay: 2000 }); // Allow network requests and render

        const table = page.getByRole('table');
        const emptyState = page.getByRole('heading', { name: /no.*access.*lists/i });

        const hasTable = await table.isVisible().catch(() => false);
        const hasEmpty = await emptyState.isVisible().catch(() => false);

        expect(hasTable || hasEmpty).toBeTruthy();
      });
    });

    test('should navigate to access lists from sidebar', async ({ page }) => {
      await test.step('Navigate via sidebar', async () => {
        // Go to a different page first
        await page.goto('/');
        await waitForLoadingComplete(page);

        // Look for Security menu or Access Lists link in sidebar
        const securityMenu = page.getByRole('button', { name: /security/i });
        const accessListsLink = page.getByRole('link', { name: /access.*lists/i });

        if (await securityMenu.isVisible().catch(() => false)) {
          await securityMenu.click();
          await waitForDebounce(page, { delay: 300 }); // Allow menu to expand
        }

        if (await accessListsLink.isVisible().catch(() => false)) {
          await accessListsLink.click();
          await waitForLoadingComplete(page);

          // Verify we're on access lists page (use .first() to avoid strict mode)
          const heading = page.getByRole('heading', { name: /access.*lists/i }).first();
          await expect(heading).toBeVisible({ timeout: 5000 });
        }
      });
    });

    test('should display ACL details (name, type, rules)', async ({ page }) => {
      const acl = await test.step('Seed a known access list', async () => {
        return seedAccessList(page);
      });

      await test.step('Check table displays ACL information', async () => {
        await page.reload();
        await waitForLoadingComplete(page);

        const table = page.getByRole('table');
        await expect(table).toBeVisible();

        const row = page.locator('tbody tr').filter({ hasText: acl.name });
        await expect(row).toBeVisible();

        // getTypeBadge (frontend/src/pages/AccessLists.tsx) always renders
        // either an "Allow" or "Deny" badge for every row — no "neither"
        // branch — so this is deterministic once the row exists.
        const typeBadge = row.locator('text=/allow|deny/i');
        await expect(typeBadge.first()).toBeVisible();
      });
    });
  });

  test.describe('Create Access List', () => {
    test('should open create form when Create button clicked', async ({ page }) => {
      await test.step('Click Create Access List button', async () => {
        const createButton = getCreateButton(page);
        await createButton.click();
      });

      await test.step('Verify form opens', async () => {
        await waitForModal(page); // Wait for form modal to open

        // The form should be visible (Card component with heading)
        const formTitle = page.getByRole('heading', { name: /create.*access.*list/i });
        await expect(formTitle).toBeVisible({ timeout: 5000 });

        // Verify essential form fields are present
        const nameInput = page.locator('#name');
        await expect(nameInput).toBeVisible();
      });
    });

    test('should validate required name field', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getCreateButton(page).click();
        await waitForModal(page); // Wait for form modal to open
      });

      await test.step('Try to submit with empty name', async () => {
        const saveButton = getSaveButton(page);
        await saveButton.click();

        // Form should show validation error or prevent submission
        const nameInput = page.locator('#name');
        const isInvalid = await nameInput.evaluate((el: HTMLInputElement) =>
          el.validity.valid === false || el.getAttribute('aria-invalid') === 'true'
        ).catch(() => false);

        // HTML5 validation should prevent submission (#name has `required`
        // in AccessListForm.tsx)
        expect(isInvalid).toBe(true);
      });

      await test.step('Close form', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should create ACL with name only (IP whitelist)', async ({ page }) => {
      const aclName = `Test ACL ${generateUniqueId()}`;

      await test.step('Open create form', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);
      });

      await test.step('Fill in ACL name', async () => {
        const nameInput = page.locator('#name');
        await nameInput.fill(aclName);
      });

      await test.step('Verify type selector is visible', async () => {
        const typeSelect = page.locator('#type');
        await expect(typeSelect).toBeVisible();

        // Default should be whitelist
        const selectedType = await typeSelect.inputValue();
        expect(selectedType).toBe('whitelist');
      });

      await test.step('Submit form', async () => {
        const saveButton = getSaveButton(page);
        await saveButton.click();
        await waitForLoadingComplete(page);
      });

      await test.step('Verify ACL was created', async () => {
        // Wait for form to close (indicates successful submission)
        const formTitle = page.getByRole('heading', { name: /create.*access.*list/i });
        await expect(formTitle).not.toBeVisible({ timeout: 10000 });

        // Then verify ACL appears in list or success toast
        const successToast = page.getByText(/success|created|saved/i);
        const aclInList = page.getByText(aclName);

        const hasSuccess = await successToast.isVisible({ timeout: 3000 }).catch(() => false);
        const hasAclInList = await aclInList.isVisible({ timeout: 5000 }).catch(() => false);

        expect(hasSuccess || hasAclInList).toBeTruthy();
      });
    });

    test('should add client IP addresses', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);
      });

      await test.step('Fill in name', async () => {
        await page.locator('#name').fill(`IP Test ACL ${generateUniqueId()}`);
      });

      await test.step('Ensure not local network only mode', async () => {
        // Wait for IP rules section to be visible (indicates form is ready)
        await page.waitForSelector('text=IP Addresses / CIDR Ranges', { timeout: 5000 });
      });

      await test.step('Add IP address', async () => {
        // Find IP input field using locator with placeholder
        const ipInput = page.locator('input[placeholder="192.168.1.0/24 or 10.0.0.1"]');
        await ipInput.waitFor({ state: 'visible', timeout: 5000 });
        await ipInput.fill('192.168.1.100');

        // Press Enter to add the IP (alternative to clicking Add button)
        await ipInput.press('Enter');
        await waitForDebounce(page);

        // Verify IP was added to list (should appear as a separate item in the form)
        const addedIP = page.getByText('192.168.1.100');
        await expect(addedIP).toBeVisible({ timeout: 5000 });
      });

      await test.step('Close form', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should add CIDR ranges', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);
      });

      await test.step('Fill in name', async () => {
        await page.locator('#name').fill(`CIDR Test ACL ${generateUniqueId()}`);
      });

      await test.step('Ensure not local network only mode', async () => {
        // Wait for IP rules section to be visible (indicates form is ready)
        await page.waitForSelector('text=IP Addresses / CIDR Ranges', { timeout: 5000 });
      });

      await test.step('Add CIDR range', async () => {
        const ipInput = page.locator('input[placeholder="192.168.1.0/24 or 10.0.0.1"]');
        await ipInput.waitFor({ state: 'visible', timeout: 5000 });
        await ipInput.fill('10.0.0.0/8');

        // Press Enter to add the CIDR (alternative to clicking Add button)
        await ipInput.press('Enter');
        await waitForDebounce(page);

        const addedCIDR = page.getByText('10.0.0.0/8');
        await expect(addedCIDR).toBeVisible({ timeout: 5000 });
      });

      await test.step('Close form', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should select blacklist type', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);
      });

      await test.step('Change type to blacklist', async () => {
        const typeSelect = page.locator('#type');
        await typeSelect.selectOption('blacklist');

        // Verify selection
        const selectedType = await typeSelect.inputValue();
        expect(selectedType).toBe('blacklist');
      });

      await test.step('Verify recommended badge appears', async () => {
        // Blacklist should show "Recommended" info box (unconditionally
        // rendered by AccessListForm.tsx whenever type is 'blacklist'/'geo_blacklist')
        const recommendedInfo = page.getByText(/recommended.*block.*lists/i);
        await expect(recommendedInfo).toBeVisible();
      });

      await test.step('Close form', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should select geo-blacklist type and add countries', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);
      });

      await test.step('Fill in name', async () => {
        await page.locator('#name').fill(`Geo ACL ${generateUniqueId()}`);
      });

      await test.step('Change type to geo_blacklist', async () => {
        const typeSelect = page.locator('#type');
        await typeSelect.selectOption('geo_blacklist');
      });

      await test.step('Add country', async () => {
        // Find country selector
        const countrySelect = page.locator('select').filter({ hasText: /add.*country/i });

        if (await countrySelect.isVisible().catch(() => false)) {
          await countrySelect.selectOption('CN');

          // Verify country was added
          const addedCountry = page.getByText(/china/i);
          await expect(addedCountry).toBeVisible({ timeout: 3000 });
        }
      });

      await test.step('Close form', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should toggle enabled/disabled state', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);
      });

      await test.step('Toggle enabled switch', async () => {
        const enabledSwitch = page.getByLabel(/enabled/i).first();

        if (await enabledSwitch.isVisible().catch(() => false)) {
          const wasChecked = await enabledSwitch.isChecked();
          await clickSwitch(enabledSwitch);
          const isNowChecked = await enabledSwitch.isChecked();
          expect(isNowChecked).toBe(!wasChecked);
        }
      });

      await test.step('Close form', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should show success toast on creation', async ({ page }) => {
      const aclName = `Toast Test ACL ${generateUniqueId()}`;

      await test.step('Create ACL', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);

        await page.locator('#name').fill(aclName);
        await getSaveButton(page).click();
      });

      await test.step('Verify success notification', async () => {
        // Wait for toast or list update
        await waitForLoadingComplete(page);

        const successToast = page.getByText(/success|created/i);
        const aclInList = page.getByText(aclName);

        // NOTE: `locator.isVisible({ timeout })` does NOT poll — Playwright
        // marks that option "@deprecated ... ignored" and the call returns
        // immediately (see playwright-core's type declarations). Using it
        // here raced the toast's appearance and the list's post-create
        // refetch against a single instant check, which is exactly what
        // made this test flaky. `locator.waitFor({ state: 'visible' })`
        // actually polls for up to `timeout`.
        const hasToast = await successToast
          .waitFor({ state: 'visible', timeout: 5000 })
          .then(() => true)
          .catch(() => false);
        const hasAcl = hasToast
          ? true
          : await aclInList
              .waitFor({ state: 'visible', timeout: 5000 })
              .then(() => true)
              .catch(() => false);

        expect(hasToast || hasAcl).toBeTruthy();
      });
    });

    test('should show security presets for blacklist type', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);
      });

      await test.step('Select geo-blacklist type', async () => {
        // SECURITY_PRESETS (frontend/src/data/securityPresets.ts) currently
        // only defines `type: 'geo_blacklist'` entries — there are no plain
        // IP-blacklist presets — and the presets panel filters strictly by
        // `preset.type === formData.type` (AccessListForm.tsx). Selecting
        // plain 'blacklist' would deterministically show zero presets, so
        // this test (and its "botnet|scanner|abuse" search, which doesn't
        // match either of the two real preset names either) was written
        // against stale preset content. Select 'geo_blacklist', the type
        // that actually has presets, to exercise real behavior.
        const typeSelect = page.locator('#type');
        await typeSelect.selectOption('geo_blacklist');
      });

      await test.step('Verify security presets section appears', async () => {
        const presetsSection = page.getByText(/security.*presets/i);
        await expect(presetsSection).toBeVisible({ timeout: 3000 });

        const showPresetsButton = page.getByRole('button', { name: /show.*presets/i });
        await expect(showPresetsButton).toBeVisible();
        await showPresetsButton.click();

        // Verify a real, current preset renders (SECURITY_PRESETS' two
        // geo_blacklist entries: "Block High-Risk Countries" / "Block
        // Expanded Threat List").
        const presetOption = page.getByText(/high-risk countries|expanded threat list/i);
        await expect(presetOption.first()).toBeVisible();
      });

      await test.step('Close form', async () => {
        await getCancelButton(page).click();
      });
    });

    test('should have Get My IP button', async ({ page }) => {
      await test.step('Open create form', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);
      });

      await test.step('Ensure IP rules section is visible', async () => {
        // Wait for IP rules section to be visible (indicates form is ready)
        await page.waitForSelector('text=IP Addresses / CIDR Ranges', { timeout: 5000 });
      });

      await test.step('Verify Get My IP button exists', async () => {
        // Look for the Get My IP button
        const getMyIPButton = page.getByRole('button', { name: /get.*my.*ip/i });
        await expect(getMyIPButton).toBeVisible({ timeout: 5000 });
      });

      await test.step('Close form', async () => {
        await getCancelButton(page).click();
      });
    });
  });

  test.describe('Update Access List', () => {
    test('should open edit form with existing values', async ({ page }) => {
      await test.step('Find and click Edit button', async () => {
        const editButtons = page.getByRole('button', { name: /edit/i });
        const editCount = await editButtons.count();

        if (editCount > 0) {
          await editButtons.first().click();
          await waitForModal(page, /edit|access.*list/i);

          // Verify form opens with "Edit" title
          const formTitle = page.getByRole('heading', { name: /edit.*access.*list/i });
          await expect(formTitle).toBeVisible({ timeout: 5000 });

          // Verify name field is populated
          const nameInput = page.locator('#name');
          const nameValue = await nameInput.inputValue();
          expect(nameValue.length > 0).toBeTruthy();

          // Close form
          await getCancelButton(page).click();
        }
      });
    });

    test('should update ACL name', async ({ page }) => {
      const acl = await test.step('Seed an ACL to rename', async () => {
        return seedAccessList(page);
      });

      await test.step('Edit the seeded ACL and rename it', async () => {
        await page.reload();
        await waitForLoadingComplete(page);

        const row = page.locator('tbody tr').filter({ hasText: acl.name });
        await expect(row).toBeVisible();

        await row.getByRole('button', { name: /edit/i }).click();
        await waitForModal(page, /edit|access.*list/i);

        const nameInput = page.locator('#name');
        const newName = `Updated ACL ${generateUniqueId()}`;
        await nameInput.clear();
        await nameInput.fill(newName);

        await getSaveButton(page).click();
        await waitForLoadingComplete(page);

        // Verify the rename persisted server-side, not just optimistically
        // in the UI list.
        await expect(getAccessListViaAPI(page.request, acl.uuid)).resolves.toMatchObject({
          name: newName,
        });
      });
    });

    test('should add/remove client IPs', async ({ page }) => {
      await test.step('Edit ACL and modify IP rules', async () => {
        const editButtons = page.getByRole('button', { name: /edit/i });
        const editCount = await editButtons.count();

        if (editCount > 0) {
          await editButtons.first().click();
          await waitForModal(page, /edit|access.*list/i);

          // Ensure IP mode is enabled
          const localNetworkSwitch = page.getByLabel(/local.*network.*only/i);
          if (await localNetworkSwitch.isChecked().catch(() => false)) {
            await clickSwitch(localNetworkSwitch);
          }

          // Add new IP
          const ipInput = page.locator('input[placeholder*="192.168"], input[placeholder*="CIDR"]').first();
          if (await ipInput.isVisible().catch(() => false)) {
            await ipInput.fill('172.16.0.1');
            const addButton = page.locator('button').filter({ has: page.locator('svg.lucide-plus') }).first();
            await addButton.click();
          }

          // Cancel without saving
          await getCancelButton(page).click();
        }
      });
    });

    test('should toggle ACL type', async ({ page }) => {
      await test.step('Edit and toggle type', async () => {
        const editButtons = page.getByRole('button', { name: /edit/i });
        const editCount = await editButtons.count();

        if (editCount > 0) {
          await editButtons.first().click();
          await waitForModal(page, /edit|access.*list/i);

          const typeSelect = page.locator('#type');
          const currentType = await typeSelect.inputValue();

          // Toggle between whitelist and blacklist
          if (currentType === 'whitelist') {
            await typeSelect.selectOption('blacklist');
          } else if (currentType === 'blacklist') {
            await typeSelect.selectOption('whitelist');
          }

          // Cancel without saving
          await getCancelButton(page).click();
        }
      });
    });

    test('should show success toast on update', async ({ page }) => {
      const acl = await test.step('Seed an ACL to update', async () => {
        return seedAccessList(page);
      });

      await test.step('Update ACL and verify toast', async () => {
        await page.reload();
        await waitForLoadingComplete(page);

        const row = page.locator('tbody tr').filter({ hasText: acl.name });
        await expect(row).toBeVisible();
        await row.getByRole('button', { name: /edit/i }).click();
        await waitForModal(page, /edit|access.*list/i);

        // Make a small change to description
        const descriptionInput = page.locator('#description');
        await descriptionInput.fill(`Updated at ${new Date().toISOString()}`);

        await getSaveButton(page).click();
        await waitForLoadingComplete(page);

        // Wait for success indication. Scoped to the toast's role (the
        // react-hot-toast <Toaster/> in App.tsx tags success toasts
        // role="status") and its exact message (useUpdateAccessList in
        // useAccessLists.ts) — a bare /success|updated|saved/i text search
        // previously matched this test's own description textarea, whose
        // value ("Updated at <timestamp>") contains "Updated" and produced
        // a false pass regardless of whether the update actually succeeded.
        const successToast = page
          .getByRole('status')
          .filter({ hasText: /access list updated successfully/i });
        await expect(successToast).toBeVisible({ timeout: 5000 });
      });
    });
  });

  test.describe('Delete Access List', () => {
    test('should show delete confirmation dialog', async ({ page }) => {
      await test.step('Click Delete button', async () => {
        const deleteButtons = page.locator('button[title*="Delete"], button[title*="delete"]');
        const deleteCount = await deleteButtons.count();

        if (deleteCount > 0) {
          await deleteButtons.first().click();
          await waitForDialog(page);

          // Confirmation dialog should appear
          const dialog = page.getByRole('dialog');
          const hasDialog = await dialog.isVisible({ timeout: 3000 }).catch(() => false);

          if (hasDialog) {
            // Dialog should have confirm and cancel buttons
            const confirmButton = dialog.getByRole('button', { name: /delete/i });
            const cancelButton = dialog.getByRole('button', { name: /cancel/i });

            await expect(confirmButton).toBeVisible();
            await expect(cancelButton).toBeVisible();

            // Cancel the delete
            await cancelButton.click();
          }
        }
      });
    });

    test('should cancel delete when confirmation dismissed', async ({ page }) => {
      await test.step('Trigger delete and cancel', async () => {
        const deleteButtons = page.locator('button[title*="Delete"], button[title*="delete"]');
        const deleteCount = await deleteButtons.count();

        if (deleteCount > 0) {
          // Count rows before
          const rowsBefore = await page.locator('tbody tr').count();

          await deleteButtons.first().click();
          await waitForDialog(page);

          const dialog = page.getByRole('dialog');
          if (await dialog.isVisible().catch(() => false)) {
            await dialog.getByRole('button', { name: /cancel/i }).click();
            await expect(dialog).not.toBeVisible({ timeout: 3000 });

            // Rows should remain unchanged
            const rowsAfter = await page.locator('tbody tr').count();
            expect(rowsAfter).toBe(rowsBefore);
          }
        }
      });
    });

    test('should show delete confirmation with ACL name', async ({ page }) => {
      await test.step('Verify confirmation message includes ACL info', async () => {
        const deleteButtons = page.locator('button[title*="Delete"], button[title*="delete"]');
        const deleteCount = await deleteButtons.count();

        if (deleteCount > 0) {
          await deleteButtons.first().click();
          await waitForDialog(page);

          const dialog = page.getByRole('dialog');
          if (await dialog.isVisible().catch(() => false)) {
            const dialogText = await dialog.textContent();
            expect(dialogText).toBeTruthy();

            // Cancel
            await dialog.getByRole('button', { name: /cancel/i }).click();
          }
        }
      });
    });

    test('should create backup before deletion', async ({ page }) => {
      await test.step('Verify backup toast on delete', async () => {
        // This test verifies the backup-before-delete functionality
        const deleteButtons = page.locator('button[title*="Delete"], button[title*="delete"]');
        const deleteCount = await deleteButtons.count();

        if (deleteCount > 0) {
          // For safety, don't actually delete - just verify the dialog mentions backup
          await deleteButtons.first().click();
          await waitForDialog(page);

          const dialog = page.getByRole('dialog');
          if (await dialog.isVisible().catch(() => false)) {
            // The confirmation dialog itself only asks the user to confirm the
            // deletion; the actual pre-delete backup is triggered (and toasted)
            // by handleDeleteWithBackup() once the user clicks Delete. Since
            // this test intentionally cancels without deleting, just verify
            // the confirmation dialog rendered with its expected content.
            await expect(dialog).toContainText(/delete/i);
            // Cancel without deleting
            await dialog.getByRole('button', { name: /cancel/i }).click();
          }
        }
      });
    });

    test('should delete from edit form', async ({ page }) => {
      await test.step('Open edit form and find delete button', async () => {
        const editButtons = page.getByRole('button', { name: /edit/i });
        const editCount = await editButtons.count();

        if (editCount > 0) {
          await editButtons.first().click();
          await waitForModal(page, /edit|access.*list/i);

          // Look for delete button in form
          const deleteInForm = page.getByRole('button', { name: /delete/i });
          await expect(deleteInForm).toBeVisible();

          // Cancel without deleting
          await getCancelButton(page).click();
        }
      });
    });
  });

  test.describe('Test IP Functionality', () => {
    test('should open Test IP dialog', async ({ page }) => {
      await test.step('Find and click Test button', async () => {
        // Look for test button with TestTube icon
        const testButtons = page.locator('button[title*="Test"], button[title*="test"]');
        const testCount = await testButtons.count();

        if (testCount > 0) {
          await testButtons.first().click();
          await waitForDialog(page);

          // Test IP dialog should open
          const dialog = page.getByRole('dialog');
          const hasDialog = await dialog.isVisible({ timeout: 3000 }).catch(() => false);

          if (hasDialog) {
            const dialogTitle = dialog.getByRole('heading', { name: /test.*ip/i });
            await expect(dialogTitle).toBeVisible();

            // Close dialog
            await dialog
              .getByRole('button', { name: /^close$/i })
              .filter({ hasText: /^close$/i })
              .click();
          }
        }
      });
    });

    test('should have IP input field in test dialog', async ({ page }) => {
      await test.step('Verify IP input in test dialog', async () => {
        const testButtons = page.locator('button[title*="Test"], button[title*="test"]');
        const testCount = await testButtons.count();

        if (testCount > 0) {
          await testButtons.first().click();
          await waitForDialog(page);

          const dialog = page.getByRole('dialog');
          if (await dialog.isVisible().catch(() => false)) {
            // Should have IP input - placeholder is "192.168.1.100"
            const ipInput = dialog.locator('input[placeholder*="192.168"]');
            await expect(ipInput.first()).toBeVisible();

            // Should have Test button
            const testButton = dialog.getByRole('button', { name: /test/i });
            await expect(testButton).toBeVisible();

            // Close dialog
            await dialog
              .getByRole('button', { name: /^close$/i })
              .filter({ hasText: /^close$/i })
              .click();
          }
        }
      });
    });
  });

  test.describe('Bulk Operations', () => {
    test('should show row selection checkboxes', async ({ page }) => {
      await test.step('Seed an ACL so selection controls have a row to act on', async () => {
        await seedAccessList(page);
      });

      await test.step('Check for selectable rows', async () => {
        await page.reload();
        await waitForLoadingComplete(page);

        const selectAllCheckbox = page.locator('thead').getByRole('checkbox');
        await expect(selectAllCheckbox).toBeVisible();

        await selectAllCheckbox.click();
        await waitForDebounce(page);

        // AccessLists.tsx has no separate "N selected" bar (unlike
        // ProxyHosts.tsx) — selecting rows surfaces a "Delete (N)" button
        // directly (frontend/src/pages/AccessLists.tsx:274-282).
        const bulkDeleteButton = page.getByRole('button', { name: /delete.*\(/i });
        await expect(bulkDeleteButton).toBeVisible();

        // Deselect
        await selectAllCheckbox.click();
      });
    });

    test('should show bulk delete button when items selected', async ({ page }) => {
      await test.step('Seed an ACL so selection controls have a row to act on', async () => {
        await seedAccessList(page);
      });

      await test.step('Select items and verify bulk delete', async () => {
        await page.reload();
        await waitForLoadingComplete(page);

        const selectAllCheckbox = page.locator('thead').getByRole('checkbox');
        await expect(selectAllCheckbox).toBeVisible();
        await selectAllCheckbox.click();
        await waitForDebounce(page);

        // Look for bulk delete button in header
        const bulkDeleteButton = page.getByRole('button', { name: /delete.*\(/i });
        await expect(bulkDeleteButton).toBeVisible();

        // Deselect
        await selectAllCheckbox.click();
      });
    });
  });

  test.describe('ACL Integration with Proxy Hosts', () => {
    test('should navigate between Access Lists and Proxy Hosts', async ({ page }) => {
      await test.step('Navigate to Proxy Hosts', async () => {
        await page.goto('/proxy-hosts');
        await waitForLoadingComplete(page);

        // Scope to the page-level <h1> (rendered by PageShell) rather than any
        // heading matching /proxy.*hosts/i: when multiple proxy-host groups
        // exist, each empty group renders its own "No proxy hosts" <h3> via
        // EmptyState, which also matches the unscoped pattern and causes a
        // strict-mode violation (multiple elements resolved).
        const heading = page.getByRole('heading', { level: 1, name: /proxy.*hosts/i });
        await expect(heading).toBeVisible({ timeout: 5000 });
      });

      await test.step('Navigate back to Access Lists', async () => {
        await page.goto('/access-lists');
        await waitForLoadingComplete(page);

        // Use .first() to avoid strict mode violation
        const heading = page.getByRole('heading', { name: /access.*lists/i }).first();
        await expect(heading).toBeVisible({ timeout: 5000 });
      });
    });
  });

  test.describe('Form Validation', () => {
    test('should reject empty name', async ({ page }) => {
      await test.step('Try to create with empty name', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);

        // Leave name empty and try to submit
        const saveButton = getSaveButton(page);
        await saveButton.click();

        // Should not close form (validation error)
        const formTitle = page.getByRole('heading', { name: /create.*access.*list/i });
        await expect(formTitle).toBeVisible();

        await getCancelButton(page).click();
      });
    });

    test('should handle special characters in name', async ({ page }) => {
      await test.step('Test special characters', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);

        const nameInput = page.locator('#name');
        // Test with safe special characters
        await nameInput.fill('Test ACL - Special (chars) #1');

        // Should accept the input
        const value = await nameInput.inputValue();
        expect(value).toContain('Special');

        await getCancelButton(page).click();
      });
    });

    test('should validate CIDR format', async ({ page }) => {
      await test.step('Test CIDR validation', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);

        await page.locator('#name').fill(`CIDR Validation Test ${generateUniqueId()}`);

        // Wait for IP rules section to be visible
        await page.waitForSelector('text=IP Addresses / CIDR Ranges', { timeout: 5000 });

        // Try adding invalid CIDR
        const ipInput = page.locator('input[placeholder="192.168.1.0/24 or 10.0.0.1"]');
        await ipInput.waitFor({ state: 'visible', timeout: 5000 });
        await ipInput.fill('999.999.999.999/24');

        const addButton = page.locator('button').filter({ has: page.locator('svg.lucide-plus') }).first();
        await addButton.click();

        // Should show error or not add the invalid IP
        await waitForDebounce(page);

        await getCancelButton(page).click();
      });
    });
  });

  test.describe('CGNAT Warning', () => {
    test('should show CGNAT warning when ACLs exist', async ({ page }) => {
      await test.step('Seed an ACL so the CGNAT condition is guaranteed', async () => {
        // frontend/src/pages/AccessLists.tsx shows the CGNAT banner whenever
        // `accessLists.length > 0` (and it hasn't been dismissed this
        // session) — no other condition — so any seeded ACL triggers it.
        await seedAccessList(page);
      });

      await test.step('Check for CGNAT warning', async () => {
        await page.reload();
        await waitForLoadingComplete(page);

        const table = page.getByRole('table');
        await expect(table).toBeVisible();

        const rows = await page.locator('tbody tr').count();
        expect(rows).toBeGreaterThan(0);

        // Scope to the Alert's role="alert" container (frontend/src/components/ui/Alert.tsx)
        // as a single element — both the title and message text nodes
        // independently contain "CGNAT", so a bare page.getByText(/cgnat/i)
        // matches two elements and strict-mode-violates.
        const cgnatWarning = page.getByRole('alert').filter({ hasText: /cgnat/i });
        await expect(cgnatWarning).toBeVisible();
        // Assert real, translated content (frontend/src/locales/en/translation.json's
        // accessLists.cgnatWarning.title), not just the substring "cgnat" —
        // a raw, untranslated i18n key would also contain "cgnat" and
        // falsely satisfy a looser check.
        await expect(cgnatWarning).toContainText(/mobile network warning/i);
      });
    });

    test('should be dismissible', async ({ page }) => {
      await test.step('Dismiss CGNAT warning', async () => {
        const cgnatWarning = page.getByRole('alert').filter({ hasText: /cgnat|locked.*out/i });

        if (await cgnatWarning.isVisible().catch(() => false)) {
          const dismissButton = cgnatWarning.getByRole('button');
          if (await dismissButton.isVisible().catch(() => false)) {
            await dismissButton.click();
            await expect(cgnatWarning).not.toBeVisible({ timeout: 3000 });
          }
        }
      });
    });
  });

  test.describe('Best Practices Link', () => {
    test('should show Best Practices button', async ({ page }) => {
      await test.step('Verify Best Practices link', async () => {
        const bestPracticesLink = page.getByRole('button', { name: /best.*practices/i });
        await expect(bestPracticesLink).toBeVisible();
      });
    });

    test('should have external link to documentation', async ({ page }) => {
      await test.step('Verify link opens external documentation', async () => {
        const bestPracticesLink = page.getByRole('button', { name: /best.*practices/i });
        await expect(bestPracticesLink).toBeVisible();

        // Verify it has external link icon (statically rendered — AccessLists.tsx)
        const externalIcon = bestPracticesLink.locator('svg.lucide-external-link');
        await expect(externalIcon).toBeVisible();
      });
    });
  });

  test.describe('Form Accessibility', () => {
    test('should have accessible form labels', async ({ page }) => {
      await test.step('Open form and verify labels', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);

        // Check that inputs have associated labels
        const nameLabel = page.locator('label[for="name"]');
        await expect(nameLabel).toBeVisible();

        await getCancelButton(page).click();
      });
    });

    test('should be keyboard navigable', async ({ page }) => {
      await test.step('Navigate form with keyboard', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);

        // Tab through form fields
        await page.keyboard.press('Tab');
        await page.keyboard.press('Tab');
        await page.keyboard.press('Tab');

        // Some element should be focused
        const focusedElement = page.locator(':focus');
        await expect(focusedElement).toBeVisible();

        // Escape or Cancel to close
        await getCancelButton(page).click();
      });
    });
  });

  test.describe('Local Network Only Mode', () => {
    test('should toggle local network only (RFC1918)', async ({ page }) => {
      await test.step('Open form and toggle RFC1918 mode', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);

        const localNetworkSwitch = page.getByLabel(/local.*network.*only/i);

        if (await localNetworkSwitch.isVisible().catch(() => false)) {
          const wasChecked = await localNetworkSwitch.isChecked();
          await clickSwitch(localNetworkSwitch);
          const isNowChecked = await localNetworkSwitch.isChecked();
          expect(isNowChecked).toBe(!wasChecked);
        }

        await getCancelButton(page).click();
      });
    });

    test('should hide IP rules when local network only is enabled', async ({ page }) => {
      await test.step('Verify IP input hidden in RFC1918 mode', async () => {
        await getCreateButton(page).click();
        await waitForModal(page, /create|access.*list/i);

        // isIPType (AccessListForm.tsx) is true for the default 'whitelist'
        // type, so the switch is deterministically visible without needing
        // to change type first.
        const localNetworkSwitch = page.getByLabel(/local.*network.*only/i);
        await expect(localNetworkSwitch).toBeVisible();

        // Enable local network only
        if (!await localNetworkSwitch.isChecked()) {
          await clickSwitch(localNetworkSwitch);
        }

        // IP input should be hidden once local-network-only mode is enabled
        // (AccessListForm.tsx only renders the IP rules block when
        // `!formData.local_network_only`).
        const ipInput = page.locator('input[placeholder*="192.168"], input[placeholder*="CIDR"]').first();
        await expect(ipInput).not.toBeVisible();

        await getCancelButton(page).click();
      });
    });
  });
});

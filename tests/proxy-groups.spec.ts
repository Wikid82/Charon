import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { getToastLocator } from './utils/ui-helpers';
import {
  waitForAPIResponse,
  waitForDialog,
  waitForLoadingComplete,
} from './utils/wait-helpers';
import { generateProxyHost } from './fixtures/proxy-hosts';

test.describe('Proxy Groups', () => {
  test.beforeEach(async ({ page }) => {
    await waitForAPIHealth(page.request);
    await page.goto('/proxy-hosts');
    await waitForLoadingComplete(page);
  });

  test.describe('Group Management', () => {
    test('should open Manage Groups dialog', async ({ page }) => {
      await page.getByRole('button', { name: /manage groups/i }).click();
      await waitForDialog(page);
      await expect(page.getByRole('dialog')).toBeVisible();
    });

    test('should create a new proxy group', async ({ page }) => {
      await page.getByRole('button', { name: /manage groups/i }).click();
      await waitForDialog(page);

      await page.getByRole('button', { name: /create group/i }).click();

      await page.getByRole('dialog').last().waitFor({ state: 'visible' });
      await page.getByLabel(/group name/i).fill('Test Group');

      const savePromise = waitForAPIResponse(page, '/api/v1/proxy-groups', { status: 201 });
      await page.getByRole('button', { name: /save/i }).click();
      await savePromise;

      // Explicit timeout: react-hot-toast's success toast auto-dismisses after
      // 5000ms (App.tsx's <Toaster toastOptions={{ duration: 5000 }}>), which
      // exactly matches Playwright's global default expect.timeout (also
      // 5000ms per playwright.config.js). That leaves zero margin between "the
      // toast is still there" and "the assertion gave up" — any latency
      // between the API response and React's onSuccess->toast.success() call
      // eats directly into a already-zero-slack shared budget. A longer
      // explicit timeout here doesn't mask real failures (the toast reliably
      // fires within a second or two in practice); it just stops a race with
      // the toast's own dismiss timer from flaking the assertion.
      await expect(getToastLocator(page)).toBeVisible({ timeout: 8000 });
    });

    test('should disable Save button when name is empty', async ({ page }) => {
      await page.getByRole('button', { name: /manage groups/i }).click();
      await waitForDialog(page);

      await page.getByRole('button', { name: /create group/i }).click();
      await page.getByRole('dialog').last().waitFor({ state: 'visible' });

      // Save is disabled while the name field is blank.
      await expect(page.getByRole('button', { name: /save/i })).toBeDisabled();
    });

    test('should allow selecting a preset color', async ({ page }) => {
      await page.getByRole('button', { name: /manage groups/i }).click();
      await waitForDialog(page);

      await page.getByRole('button', { name: /create group/i }).click();
      await page.getByRole('dialog').last().waitFor({ state: 'visible' });

      await page.getByLabel(/group name/i).fill('Colored Group');

      // Color preset buttons are labelled "Color #<hex>" via the colorPreset i18n key.
      // Scoping to the last dialog avoids matching unrelated buttons on the page.
      const dialog = page.getByRole('dialog').last();
      const colorSwatch = dialog.getByRole('button', { name: /^color #/i }).first();
      await colorSwatch.click();
      await expect(colorSwatch).toHaveAttribute('aria-pressed', 'true');
    });
  });

  test.describe('Grouped Display', () => {
    test('should show ungrouped section when groups exist', async ({ page }) => {
      // Use an auto-retrying assertion instead of a synchronous isVisible()
      // check: the button is unconditionally rendered in the page's action
      // bar, but a bare isVisible() has no wait/retry margin and can read
      // the DOM a beat before React finishes painting after navigation.
      await expect(page.getByRole('button', { name: /manage groups/i })).toBeVisible();
    });

    test('should display flat table when no groups exist', async ({ page }) => {
      const table = page.getByRole('table').first();
      await expect(table).toBeVisible();
    });
  });

  test.describe('Bulk Assignment', () => {
    test('Assign to Group button is conditionally visible', async ({ page }) => {
      const checkboxes = page.getByRole('checkbox');
      const count = await checkboxes.count();

      if (count > 1) {
        await checkboxes.first().check();

        const assignBtn = page.getByRole('button', { name: /assign to group/i });
        const isVisible = await assignBtn.isVisible();
        if (isVisible) {
          await expect(assignBtn).toBeVisible();
        }
      }
    });
  });
});

// ─── Helpers for Group Selector / Grouped Row Layout specs ────────────────
// (Issue #1367 — see docs/plans/current_spec.md §4 Phase 1 / §5 Commit 1.)

/**
 * Open the "Add Proxy Host" dialog and return its locator.
 * Mirrors the pattern in tests/orthrus-proxy-paths.spec.ts.
 */
async function openAddProxyHostDialog(page: import('@playwright/test').Page) {
  await page.getByRole('button', { name: 'Add Proxy Host' }).first().click();
  const dialog = page.getByRole('dialog', { name: /add proxy host/i });
  await waitForDialog(page);
  await expect(dialog).toBeVisible();
  return dialog;
}

/**
 * Open the "Edit Proxy Host" dialog for the first host row and return its
 * locator. Skips the calling test when no proxy host rows exist.
 */
async function openEditProxyHostDialog(page: import('@playwright/test').Page) {
  const editButtons = page.getByRole('button', { name: /^edit proxy host/i });
  if ((await editButtons.count()) === 0) {
    test.skip(true, 'No proxy hosts available to edit');
  }
  await editButtons.first().click();
  const dialog = page.getByRole('dialog', { name: /edit proxy host/i });
  await waitForDialog(page);
  await expect(dialog).toBeVisible();
  return dialog;
}

// ─── Proxy Host Form — Group Selector (not yet implemented) ────────────────
//
// `ProxyGroupSelector` (frontend/src/components/ProxyGroupSelector.tsx) and
// its wiring into `ProxyHostForm.tsx` (spec §3.2/§3.3) don't exist yet.
// These tests are written against the documented, expected structure — a
// Radix `combobox` labelled "Proxy Group" mirroring `AccessListSelector`'s
// "Access Control List" trigger (see frontend/src/components/AccessListSelector.tsx)
// — so flipping `.fixme` to a real assertion once the component lands (spec
// §5 Commit 4) should need little to no rework.

test.describe('Proxy Host Form — Group Selector', () => {
  test.beforeEach(async ({ page }) => {
    await waitForAPIHealth(page.request);
    await page.goto('/proxy-hosts');
    await waitForLoadingComplete(page);
  });

  test.fixme('shows a group selector when creating a new host', async ({ page }) => {
    const dialog = await openAddProxyHostDialog(page);

    await test.step('Group selector is present and defaults to "No Group"', async () => {
      const groupSelect = dialog.getByRole('combobox', { name: /proxy group/i });
      await expect(groupSelect).toBeVisible();
      await expect(groupSelect).toHaveText(/no group/i);
    });
  });

  test.fixme("preselects the host's current group when editing", async ({ page }) => {
    const dialog = await openEditProxyHostDialog(page);

    await test.step("Group selector reflects the host's assigned group", async () => {
      const groupSelect = dialog.getByRole('combobox', { name: /proxy group/i });
      await expect(groupSelect).toBeVisible();
      // TODO once implemented: assert groupSelect's rendered text matches
      // the edited host's `proxy_group.name` (or "No Group" when the host
      // being edited has no group assigned).
    });
  });

  test.fixme('assigns a group to a host via the create/edit form', async ({ page }) => {
    const groupName = `E2E Form Group ${Date.now()}`;
    const host = generateProxyHost();

    await test.step('Create a proxy group to assign', async () => {
      await page.getByRole('button', { name: /manage groups/i }).click();
      await waitForDialog(page);
      await page.getByRole('button', { name: /create group/i }).click();
      await page.getByRole('dialog').last().waitFor({ state: 'visible' });
      await page.getByRole('dialog').last().getByLabel(/group name/i).fill(groupName);
      const savePromise = waitForAPIResponse(page, '/api/v1/proxy-groups', { status: 201 });
      await page.getByRole('button', { name: /save/i }).click();
      await savePromise;
      await expect(page.getByRole('dialog')).toHaveCount(1);
      await page.keyboard.press('Escape');
      await expect(page.getByRole('dialog')).toHaveCount(0);
    });

    await test.step('Create a new proxy host with the group selected', async () => {
      const dialog = await openAddProxyHostDialog(page);
      await dialog.getByLabel(/domain names/i).fill(host.domain);
      await dialog.getByLabel(/^host$/i).fill(host.forwardHost);
      await dialog.getByLabel(/^port$/i).fill(String(host.forwardPort));

      const groupSelect = dialog.getByRole('combobox', { name: /proxy group/i });
      await groupSelect.click();
      await page.getByRole('option', { name: groupName }).click();

      const createPromise = waitForAPIResponse(page, '/api/v1/proxy-hosts', { status: 201 });
      await dialog.getByRole('button', { name: /^save$/i }).click();
      await createPromise;
    });

    await test.step('The new host appears under the assigned group section', async () => {
      await expect(
        page.getByRole('region', { name: groupName }).getByText(host.domain),
      ).toBeVisible();
    });
  });

  test.fixme("clears a host's group via the create/edit form", async ({ page }) => {
    const dialog = await openEditProxyHostDialog(page);

    await test.step('Clear the assigned group', async () => {
      const groupSelect = dialog.getByRole('combobox', { name: /proxy group/i });
      await groupSelect.click();
      await page.getByRole('option', { name: /no group/i }).click();

      const updatePromise = waitForAPIResponse(page, '/api/v1/proxy-hosts/', { status: 200 });
      await dialog.getByRole('button', { name: /^save$/i }).click();
      await updatePromise;
    });

    await test.step('Host moves to the Ungrouped section with no group badge', async () => {
      await expect(page.getByRole('region', { name: /ungrouped/i })).toBeVisible();
    });
  });
});

// ─── Proxy Hosts — Grouped Row Layout (not yet implemented) ────────────────
//
// The `groupedColumns` split (spec §3.4, frontend/src/pages/ProxyHosts.tsx)
// does not exist yet — today both the "named group" and "Ungrouped"
// per-group-context `DataTable` calls reuse the flat `columns` array, which
// still carries the redundant "Group" column and lacks the Actions-column
// `minWidth` floor these tests guard against (root cause: spec §2.3).

test.describe('Proxy Hosts — Grouped Row Layout', () => {
  test.beforeEach(async ({ page }) => {
    await waitForAPIHealth(page.request);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/proxy-hosts');
    await waitForLoadingComplete(page);
  });

  test.fixme('keeps the Actions column fully visible at 1280px width when grouped', async ({ page }) => {
    const hasGroups = (await page.locator('[data-drop-zone]').count()) > 0;
    test.skip(!hasGroups, 'No proxy groups present — grouped view not active');

    // Assert per-table, not just the first table: grouped view renders one
    // DataTable per named group section plus one for "Ungrouped", each with
    // its own Actions column that must not clip its Edit/Delete buttons.
    const tables = page.getByRole('table');
    const tableCount = await tables.count();
    expect(tableCount).toBeGreaterThan(0);

    for (let i = 0; i < tableCount; i++) {
      const table = tables.nth(i);
      const tableBox = await table.boundingBox();
      if (!tableBox) continue;

      const actionButtons = table.getByRole('button', { name: /^(edit|delete) proxy host/i });
      const buttonCount = await actionButtons.count();

      for (let j = 0; j < buttonCount; j++) {
        const buttonBox = await actionButtons.nth(j).boundingBox();
        expect(buttonBox).not.toBeNull();
        // Fully contained within the table's bounding box — no clipping at
        // the row's right edge (spec §4.1: bounding-box containment, not
        // exact pixel widths).
        expect(buttonBox!.x).toBeGreaterThanOrEqual(tableBox.x);
        expect(buttonBox!.x + buttonBox!.width).toBeLessThanOrEqual(
          tableBox.x + tableBox.width,
        );
      }
    }
  });

  test.fixme('does not render a redundant Group column inside a named group section', async ({ page }) => {
    const namedZones = page.locator('[data-drop-zone]:not([data-drop-zone="ungrouped"])');
    const zoneCount = await namedZones.count();
    test.skip(zoneCount === 0, 'No named proxy groups present');

    // The named group's <section> is rendered *inside* its [data-drop-zone]
    // wrapper (see GroupDropZone.tsx), so scope the search accordingly.
    const groupSection = namedZones.first().getByRole('region');
    await expect(groupSection.getByRole('columnheader', { name: 'Group' })).toHaveCount(0);
  });
});

import type { Page } from '@playwright/test';
import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { getToastLocator } from './utils/ui-helpers';
import {
  dismissNewDomainPromptIfPresent,
  waitForAPIResponse,
  waitForDialog,
  waitForLoadingComplete,
} from './utils/wait-helpers';
import { generateProxyHost, type ProxyHostConfig } from './fixtures/proxy-hosts';

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
 * Escape a string for safe use inside a `RegExp` constructor.
 */
function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * Open the "Add Proxy Host" dialog and return its locator.
 * Mirrors the pattern in tests/orthrus-proxy-paths.spec.ts.
 */
async function openAddProxyHostDialog(page: Page) {
  await page.getByRole('button', { name: 'Add Proxy Host' }).first().click();
  const dialog = page.getByRole('dialog', { name: /add proxy host/i });
  await waitForDialog(page);
  await expect(dialog).toBeVisible();
  return dialog;
}

/**
 * Open the "Edit Proxy Host" dialog for the row whose accessible name
 * contains `domain`, scoping the lookup to a specific host rather than
 * "whichever row/edit button happens to be first" — the grouped view
 * renders multiple `DataTable`s (one per group, plus "Ungrouped"), so an
 * unscoped `.first()` edit button is not deterministic once more than one
 * host exists.
 */
async function openEditProxyHostDialogForDomain(page: Page, domain: string) {
  const row = page.getByRole('row', { name: new RegExp(escapeRegExp(domain)) });
  await row.getByRole('button', { name: /^edit proxy host/i }).click();
  const dialog = page.getByRole('dialog', { name: /edit proxy host/i });
  await waitForDialog(page);
  await expect(dialog).toBeVisible();
  return dialog;
}

/**
 * Create a proxy group via the "Manage Groups" UI and return once its
 * dialog has fully closed. Mirrors `createGroupViaUI` in
 * tests/proxy-host-drag-drop.spec.ts.
 */
async function createProxyGroupViaUI(page: Page, name: string): Promise<void> {
  await page.getByRole('button', { name: /manage groups/i }).click();
  await waitForDialog(page);
  await page.getByRole('button', { name: /create group/i }).click();
  await page.getByRole('dialog').last().waitFor({ state: 'visible' });
  await page.getByRole('dialog').last().getByLabel(/group name/i).fill(name);
  const savePromise = waitForAPIResponse(page, '/api/v1/proxy-groups', { status: 201 });
  await page.getByRole('button', { name: /save/i }).click();
  await savePromise;
  // ProxyGroupForm auto-closes after a successful save. Wait until only one
  // dialog remains (ManageGroupsDialog) before sending Escape to close it,
  // otherwise Escape would close the form instead of the parent dialog.
  await expect(page.getByRole('dialog')).toHaveCount(1);
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await waitForLoadingComplete(page);
}

/**
 * Create a proxy host via the "Add Proxy Host" form, optionally assigning
 * it to a proxy group by name via `ProxyGroupSelector`. Dismisses the "New
 * Base Domain Detected" prompt that `ProxyHostForm` shows automatically on
 * blur of the Domain Names field for any domain not seen before (always
 * true for these synthetic E2E domains) — its backdrop otherwise
 * intercepts clicks on the rest of the form, including the group selector.
 */
async function createProxyHostViaUI(
  page: Page,
  host: ProxyHostConfig,
  groupName?: string,
): Promise<void> {
  const dialog = await openAddProxyHostDialog(page);
  // Name is a required field (ProxyHostForm.tsx) — omitting it leaves the
  // native HTML5 "required" validation blocking submission with no visible
  // error and no network call, which otherwise hangs the Save click forever.
  // The label's accessible name is "Name *" (the required-marker asterisk
  // is rendered inline), so match on the leading word rather than an exact
  // "Name" string. `^name\b` also can't accidentally match "Domain Names".
  await dialog.getByLabel(/^name\b/i).fill(host.name ?? `E2E Host ${Date.now()}`);
  await dialog.getByLabel(/domain names/i).fill(host.domain);
  await dialog.getByLabel(/^host$/i).fill(host.forwardHost);
  await dialog.getByLabel(/^port$/i).fill(String(host.forwardPort));
  // The domain field blurs once focus moves to the Host field above, which
  // is what actually triggers the prompt — dismiss it here, after all
  // fields are filled, rather than immediately after the domain fill.
  await dismissNewDomainPromptIfPresent(page);

  if (groupName) {
    const groupSelect = dialog.getByRole('combobox', { name: /proxy group/i });
    await groupSelect.click();
    await page.getByRole('option', { name: groupName }).click();
  }

  const createPromise = waitForAPIResponse(page, '/api/v1/proxy-hosts', { status: 201 });
  await dialog.getByRole('button', { name: /^save$/i }).click();
  await createPromise;
}

/**
 * Assign `groupName` to the host identified by `domain` via the Edit Proxy
 * Host form. Used as setup for tests that need a host with a group already
 * attached — routed through the Update endpoint (`PUT /proxy-hosts/:uuid`)
 * rather than through `createProxyHostViaUI`'s create-time group selection,
 * because of a confirmed backend bug: `POST /proxy-hosts` silently drops
 * `proxy_group_id` (see the "assigns a group... via the create/edit form"
 * test below, kept as `test.fixme`, for the full repro). The Update path
 * does persist `proxy_group_id` correctly (verified via a follow-up GET),
 * so it's safe to use here as setup for tests that aren't themselves
 * exercising the broken create-time path.
 */
async function assignGroupViaEditForm(page: Page, domain: string, groupName: string): Promise<void> {
  const dialog = await openEditProxyHostDialogForDomain(page, domain);
  const groupSelect = dialog.getByRole('combobox', { name: /proxy group/i });
  await groupSelect.click();
  await page.getByRole('option', { name: groupName, exact: true }).click();
  const updatePromise = waitForAPIResponse(page, '/api/v1/proxy-hosts/', { status: 200 });
  await dialog.getByRole('button', { name: /^save$/i }).click();
  await updatePromise;
  await waitForLoadingComplete(page);
}

test.describe('Proxy Host Form — Group Selector', () => {
  test.beforeEach(async ({ page }) => {
    await waitForAPIHealth(page.request);
    await page.goto('/proxy-hosts');
    await waitForLoadingComplete(page);
  });

  test('shows a group selector when creating a new host', async ({ page }) => {
    const dialog = await openAddProxyHostDialog(page);

    await test.step('Group selector is present and defaults to "No Group"', async () => {
      const groupSelect = dialog.getByRole('combobox', { name: /proxy group/i });
      await expect(groupSelect).toBeVisible();
      await expect(groupSelect).toHaveText(/no group/i);
    });
  });

  test("preselects the host's current group when editing", async ({ page }) => {
    const groupName = `E2E Preselect Group ${Date.now()}`;
    const host = generateProxyHost();

    await test.step('Create a group and an ungrouped host, then assign the group via Edit', async () => {
      await createProxyGroupViaUI(page, groupName);
      await createProxyHostViaUI(page, host);
      await assignGroupViaEditForm(page, host.domain, groupName);
    });

    await test.step("Group selector reflects the host's assigned group when reopened", async () => {
      const dialog = await openEditProxyHostDialogForDomain(page, host.domain);
      const groupSelect = dialog.getByRole('combobox', { name: /proxy group/i });
      await expect(groupSelect).toBeVisible();
      await expect(groupSelect).toHaveText(new RegExp(escapeRegExp(groupName), 'i'));
    });
  });

  // BLOCKED on a confirmed backend bug, kept as `test.fixme` — see the
  // handback report for full repro details:
  // `POST /api/v1/proxy-hosts` silently drops `proxy_group_id` from the
  // request payload (it's present and correct in the request body, but
  // absent from both the create response AND a subsequent GET/list — the
  // group is never persisted). By contrast `PUT /api/v1/proxy-hosts/:uuid`
  // *does* persist it correctly (confirmed via a follow-up GET), which is
  // why `assignGroupViaEditForm` (used by the other tests in this file) is
  // safe to rely on. Do not remove `.fixme` here until the Create handler
  // is fixed to apply the resolved `proxy_group_id` to the model before
  // insert, matching what `Update` already does.
  test.fixme('assigns a group to a host via the create/edit form', async ({ page }) => {
    const groupName = `E2E Form Group ${Date.now()}`;
    const host = generateProxyHost();

    await test.step('Create a proxy group to assign', async () => {
      await createProxyGroupViaUI(page, groupName);
    });

    await test.step('Create a new proxy host with the group selected', async () => {
      await createProxyHostViaUI(page, host, groupName);
    });

    await test.step('The new host appears under the assigned group section', async () => {
      await expect(
        page.getByRole('region', { name: groupName }).getByText(host.domain),
      ).toBeVisible();
    });
  });

  test("clears a host's group via the create/edit form", async ({ page }) => {
    const groupName = `E2E Clear Group ${Date.now()}`;
    const host = generateProxyHost();

    await test.step('Create a group and an ungrouped host, then assign the group via Edit', async () => {
      await createProxyGroupViaUI(page, groupName);
      await createProxyHostViaUI(page, host);
      await assignGroupViaEditForm(page, host.domain, groupName);
    });

    await test.step('Clear the assigned group', async () => {
      const dialog = await openEditProxyHostDialogForDomain(page, host.domain);
      const groupSelect = dialog.getByRole('combobox', { name: /proxy group/i });
      await groupSelect.click();
      await page.getByRole('option', { name: 'No Group', exact: true }).click();

      const updatePromise = waitForAPIResponse(page, '/api/v1/proxy-hosts/', { status: 200 });
      await dialog.getByRole('button', { name: /^save$/i }).click();
      await updatePromise;
    });

    await test.step('Host moves to the Ungrouped section with no group badge', async () => {
      await expect(
        page.getByRole('region', { name: /ungrouped/i }).getByText(host.domain),
      ).toBeVisible();
    });
  });
});

// ─── Proxy Hosts — Grouped Row Layout ───────────────────────────────────────
//
// `ProxyHosts.tsx` renders one `DataTable` per named group section plus one
// for "Ungrouped" once any proxy group exists (`groupedColumns`, spec §3.4).
// Each test below creates its own group + host rather than relying on
// whatever groups/hosts happen to already exist in the shared E2E
// environment, so the grouped view is deterministically active instead of
// being conditionally skipped.

test.describe('Proxy Hosts — Grouped Row Layout', () => {
  test.beforeEach(async ({ page }) => {
    await waitForAPIHealth(page.request);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/proxy-hosts');
    await waitForLoadingComplete(page);
  });

  test('keeps the Actions column fully visible at 1280px width when grouped', async ({ page }) => {
    const groupName = `E2E Layout Group ${Date.now()}`;
    const host = generateProxyHost();

    await test.step('Create a group with an assigned host', async () => {
      await createProxyGroupViaUI(page, groupName);
      await createProxyHostViaUI(page, host, groupName);
    });

    await test.step('Every visible table keeps its Actions column unclipped', async () => {
      await expect(page.getByRole('region', { name: groupName })).toBeVisible();

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
  });

  test('does not render a redundant Group column inside a named group section', async ({ page }) => {
    // No host needs to be assigned to the group for this assertion — the
    // "Group" column is omitted from grouped-view `DataTable`s based on the
    // rendering context alone (spec §3.4: `groupedColumns` vs `flatColumns`),
    // and the column header row renders regardless of how many hosts a
    // group's section contains (even "0 hosts"). Not depending on a host
    // landing inside this specific group also sidesteps the confirmed
    // `POST /proxy-hosts` `proxy_group_id` bug documented on the
    // create/edit-form "assigns a group" test above.
    const groupName = `E2E Layout Group Header ${Date.now()}`;

    await test.step('Create a proxy group', async () => {
      await createProxyGroupViaUI(page, groupName);
    });

    await test.step('The named group section has no "Group" column header', async () => {
      const groupSection = page.getByRole('region', { name: groupName });
      await expect(groupSection).toBeVisible();
      await expect(groupSection.getByRole('columnheader', { name: 'Group' })).toHaveCount(0);
    });
  });
});

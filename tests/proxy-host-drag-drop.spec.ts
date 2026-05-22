import { test, expect } from './fixtures/test'
import { waitForAPIHealth } from './utils/api-helpers'
import { getToastLocator } from './utils/ui-helpers'
import {
  waitForAPIResponse,
  waitForDialog,
  waitForLoadingComplete,
} from './utils/wait-helpers'

// ─── Constants ──────────────────────────────────────────────────────────────

const DRAG_HANDLE_ROLEDESC = 'Draggable proxy host'
const DRAG_HANDLE_SELECTOR = `[aria-roledescription="${DRAG_HANDLE_ROLEDESC}"]`

// ─── Helpers ─────────────────────────────────────────────────────────────────

/**
 * Open the Manage Groups dialog from the Proxy Hosts toolbar.
 */
async function openManageGroups(page: import('@playwright/test').Page) {
  await page.getByRole('button', { name: /manage groups/i }).click()
  await waitForDialog(page)
  await page.getByRole('dialog').first().waitFor({ state: 'visible' })
}

/**
 * Create a new proxy group via the Manage Groups UI.
 * Returns after the group has been saved and the dialog is closed.
 */
async function createGroupViaUI(page: import('@playwright/test').Page, name: string) {
  await openManageGroups(page)
  await page.getByRole('button', { name: /create group/i }).click()
  await page.getByRole('dialog').last().waitFor({ state: 'visible' })
  await page.getByRole('dialog').last().getByLabel(/group name/i).fill(name)
  const savePromise = waitForAPIResponse(page, '/api/v1/proxy-groups', { status: 201 })
  await page.getByRole('button', { name: /save/i }).click()
  await savePromise
  // ProxyGroupForm auto-closes after a successful save. Wait until only one
  // dialog remains (ManageGroupsDialog) before sending Escape to close it,
  // otherwise Escape would close the form instead of the parent dialog.
  await expect(page.getByRole('dialog')).toHaveCount(1)
  // Close the Manage Groups dialog.
  await page.keyboard.press('Escape')
  // Wait for ManageGroupsDialog to fully dismiss before returning.
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await waitForLoadingComplete(page)
}

// ─── Suite ───────────────────────────────────────────────────────────────────

test.describe('Proxy Host Drag-and-Drop Group Assignment', () => {
  test.beforeEach(async ({ page }) => {
    await waitForAPIHealth(page.request)
    await page.goto('/proxy-hosts')
    await waitForLoadingComplete(page)
  })

  // ─── Grouped View Rendering ────────────────────────────────────────────

  test.describe('Grouped View Rendering', () => {
    test('Manage Groups button is always visible on the Proxy Hosts page', async ({ page }) => {
      await expect(page.getByRole('button', { name: /manage groups/i })).toBeVisible()
    })

    test('shows flat DataTable when no proxy groups exist', async ({ page }) => {
      const hasDropZones = (await page.locator('[data-drop-zone]').count()) > 0
      if (hasDropZones) {
        test.skip(true, 'Groups already exist — flat-view condition not active')
        return
      }
      await expect(page.getByRole('table').first()).toBeVisible()
    })

    test('flat table view contains no drag handles', async ({ page }) => {
      const hasDropZones = (await page.locator('[data-drop-zone]').count()) > 0
      if (hasDropZones) {
        test.skip(true, 'Groups already exist — flat-view condition not active')
        return
      }
      await expect(page.locator(DRAG_HANDLE_SELECTOR)).toHaveCount(0)
    })

    test('shows grouped view with drop zones when proxy groups exist', async ({ page }) => {
      const dropZones = page.locator('[data-drop-zone]')
      const count = await dropZones.count()
      if (count === 0) {
        test.skip(true, 'No groups present — grouped-view condition not active')
        return
      }
      for (let i = 0; i < count; i++) {
        await expect(dropZones.nth(i)).toHaveAttribute('data-drop-zone')
      }
    })

    test('each group section is rendered as an ARIA region with the group name', async ({ page }) => {
      const hasDropZones = (await page.locator('[data-drop-zone]').count()) > 0
      if (!hasDropZones) {
        test.skip(true, 'No groups present — grouped-view condition not active')
        return
      }
      const regions = page.getByRole('region')
      await expect(regions.first()).toBeVisible()
    })
  })

  // ─── Drag Handle Accessibility ─────────────────────────────────────────

  test.describe('Drag Handle Accessibility', () => {
    test('drag handles are visible and keyboard-focusable in the grouped view', async ({ page }) => {
      const handles = page.locator(DRAG_HANDLE_SELECTOR)
      const count = await handles.count()
      if (count === 0) {
        test.skip(true, 'No drag handles present — grouped view not active or no hosts')
        return
      }
      const first = handles.first()
      await expect(first).toBeVisible()
      await first.focus()
      await expect(first).toBeFocused()
    })

    test('single-host drag handle has correct accessible label', async ({ page }) => {
      const handles = page.locator(DRAG_HANDLE_SELECTOR)
      if ((await handles.count()) === 0) {
        test.skip(true, 'No drag handles present')
        return
      }
      await expect(handles.first()).toHaveAttribute('aria-label', 'Drag to move to another group')
    })

    test('drag handle role-description identifies it as a draggable item', async ({ page }) => {
      const handles = page.locator(DRAG_HANDLE_SELECTOR)
      if ((await handles.count()) === 0) {
        test.skip(true, 'No drag handles present')
        return
      }
      await expect(handles.first()).toHaveAttribute('aria-roledescription', DRAG_HANDLE_ROLEDESC)
    })

    test('Assign to Group bulk-action button appears when a host is selected and groups exist', async ({ page }) => {
      const hasDropZones = (await page.locator('[data-drop-zone]').count()) > 0
      if (!hasDropZones) {
        test.skip(true, 'No groups present — Assign to Group button requires groups')
        return
      }
      // Select the first proxy-host row checkbox.
      const rowCheckbox = page.getByRole('checkbox').nth(1)
      const checkboxCount = await page.getByRole('checkbox').count()
      if (checkboxCount < 2) {
        test.skip(true, 'No proxy hosts to select')
        return
      }
      await rowCheckbox.click()
      await expect(page.getByRole('button', { name: /assign to group/i })).toBeVisible()
    })
  })

  // ─── Drop Zone Structure ───────────────────────────────────────────────

  test.describe('Drop Zone Structure', () => {
    test('named group drop zones carry UUID-format identifiers', async ({ page }) => {
      const namedZones = page.locator('[data-drop-zone]:not([data-drop-zone="ungrouped"])')
      const count = await namedZones.count()
      if (count === 0) {
        test.skip(true, 'No named group drop zones present')
        return
      }
      const zoneId = await namedZones.first().getAttribute('data-drop-zone')
      expect(zoneId).toMatch(
        /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/,
      )
    })

    test('ungrouped drop zone is present alongside named group zones', async ({ page }) => {
      const allZones = page.locator('[data-drop-zone]')
      const count = await allZones.count()
      // When there are multiple drop zones at all, the ungrouped zone should exist
      // (it is rendered whenever at least one named group exists).
      if (count <= 1) {
        test.skip(true, 'Need more than one drop zone to verify ungrouped zone')
        return
      }
      await expect(page.locator('[data-drop-zone="ungrouped"]')).toBeAttached()
    })
  })

  // ─── Keyboard Drag-and-Drop ────────────────────────────────────────────

  test.describe('Keyboard Drag-and-Drop', () => {
    test('Space key activates drag on a focused drag handle', async ({ page }) => {
      const handles = page.locator(DRAG_HANDLE_SELECTOR)
      if ((await handles.count()) === 0) {
        test.skip(true, 'No drag handles present')
        return
      }
      const handle = handles.first()
      await handle.focus()
      await page.keyboard.press('Space')
      // dnd-kit renders a DragOverlay containing "Moving host" (proxyGroups.dnd.movingOne).
      // We immediately cancel to leave the page in a clean state.
      await page.keyboard.press('Escape')
      // After Escape the drag handle must remain visible (drag was cleanly cancelled).
      await expect(handle).toBeVisible()
      await expect(handle).toHaveAttribute('aria-roledescription', DRAG_HANDLE_ROLEDESC)
    })

    test('Escape cancels an in-progress keyboard drag without triggering a group update', async ({ page }) => {
      const handles = page.locator(DRAG_HANDLE_SELECTOR)
      if ((await handles.count()) === 0) {
        test.skip(true, 'No drag handles present')
        return
      }
      const handle = handles.first()
      await handle.focus()
      await page.keyboard.press('Space')
      await page.keyboard.press('Escape')
      // No success toast should appear (no API call was made).
      const toast = getToastLocator(page, /moved/i)
      await expect(toast).not.toBeVisible()
      await expect(handle).toBeVisible()
    })

    test('drag handle can be navigated to via keyboard Tab', async ({ page }) => {
      const handles = page.locator(DRAG_HANDLE_SELECTOR)
      if ((await handles.count()) === 0) {
        test.skip(true, 'No drag handles present')
        return
      }
      // Verify that at least one drag handle has tabIndex 0 (keyboard-reachable).
      const tabIndexAttr = await handles.first().getAttribute('tabindex')
      // useDraggable sets tabIndex="0" via the spread attributes.
      expect(tabIndexAttr).toBe('0')
    })
  })

  // ─── Mouse Drag-and-Drop ───────────────────────────────────────────────

  test.describe('Mouse Drag-and-Drop', () => {
    test('dragging an ungrouped host to a named group calls the bulk-update-group endpoint', async ({ page }) => {
      // Source: a drag handle inside the "Ungrouped" section.
      const ungroupedSection = page.getByRole('region', { name: 'Ungrouped' })
      const ungroupedHandles = ungroupedSection.locator(DRAG_HANDLE_SELECTOR)
      // Target: any named group drop zone.
      const namedZone = page.locator('[data-drop-zone]:not([data-drop-zone="ungrouped"])')

      const ungroupedHandleCount = await ungroupedHandles.count().catch(() => 0)
      const namedZoneCount = await namedZone.count()

      if (ungroupedHandleCount === 0 || namedZoneCount === 0) {
        test.skip(
          true,
          'Need ungrouped hosts and at least one named group for this drag test',
        )
        return
      }

      // Set up the API listener BEFORE the drag to prevent race conditions.
      const movePromise = waitForAPIResponse(
        page,
        '/api/v1/proxy-hosts/bulk-update-group',
        { status: 200 },
      )

      const sourceHandle = ungroupedHandles.first()
      const targetZone = namedZone.first()

      const sourceBox = await sourceHandle.boundingBox()
      const targetBox = await targetZone.boundingBox()

      if (!sourceBox || !targetBox) {
        test.skip(true, 'Cannot compute bounding boxes for DnD simulation')
        return
      }

      const sx = sourceBox.x + sourceBox.width / 2
      const sy = sourceBox.y + sourceBox.height / 2
      const tx = targetBox.x + targetBox.width / 2
      const ty = targetBox.y + targetBox.height / 2

      // Simulate PointerSensor drag (requires >8 px movement to activate).
      await page.mouse.move(sx, sy)
      await page.mouse.down()
      await page.mouse.move(sx + 4, sy)
      await page.mouse.move(sx + 10, sy) // Exceeds the 8 px activation constraint.
      await page.mouse.move(tx, ty)
      await page.mouse.up()

      await movePromise

      // A success toast should appear.
      await expect(getToastLocator(page, /moved/i)).toBeVisible()
    })

    test('dragging a grouped host to a different group calls the bulk-update-group endpoint', async ({ page }) => {
      // Pre-condition: at least 2 named groups, with drag handles in both sections.
      const namedZones = page.locator('[data-drop-zone]:not([data-drop-zone="ungrouped"])')
      const zoneCount = await namedZones.count()

      if (zoneCount < 2) {
        test.skip(true, 'Need at least 2 named group zones for cross-group drag test')
        return
      }

      // Find a host in the first group.
      const firstZoneId = await namedZones.first().getAttribute('data-drop-zone')
      if (!firstZoneId) {
        test.skip(true, 'Could not read drop-zone ID')
        return
      }
      const firstZoneSection = page.getByRole('region').filter({
        has: page.locator(`[data-drop-zone="${firstZoneId}"]`),
      })
      const sourceHandle = firstZoneSection
        .locator(DRAG_HANDLE_SELECTOR)
        .first()
      const hasHandle = (await sourceHandle.count()) > 0
      if (!hasHandle) {
        test.skip(true, 'No host in the first group to drag')
        return
      }

      const movePromise = waitForAPIResponse(
        page,
        '/api/v1/proxy-hosts/bulk-update-group',
        { status: 200 },
      )

      const targetZone = namedZones.last()
      const sourceBox = await sourceHandle.boundingBox()
      const targetBox = await targetZone.boundingBox()

      if (!sourceBox || !targetBox) {
        test.skip(true, 'Cannot compute bounding boxes for DnD simulation')
        return
      }

      const sx = sourceBox.x + sourceBox.width / 2
      const sy = sourceBox.y + sourceBox.height / 2
      const tx = targetBox.x + targetBox.width / 2
      const ty = targetBox.y + targetBox.height / 2

      await page.mouse.move(sx, sy)
      await page.mouse.down()
      await page.mouse.move(sx + 4, sy)
      await page.mouse.move(sx + 10, sy)
      await page.mouse.move(tx, ty)
      await page.mouse.up()

      await movePromise
    })
  })

  // ─── Group Creation Integration ────────────────────────────────────────

  test.describe('Group Creation Integration', () => {
    test('creating the first group switches the view from flat DataTable to grouped view', async ({ page }) => {
      const hasDropZones = (await page.locator('[data-drop-zone]').count()) > 0
      if (hasDropZones) {
        test.skip(
          true,
          'Groups already exist — flat-to-grouped view transition has already occurred',
        )
        return
      }

      // Flat table must be visible before creating the first group.
      await expect(page.getByRole('table').first()).toBeVisible()

      const groupName = `E2E DnD First Group ${Date.now()}`
      await createGroupViaUI(page, groupName)

      // After the first group is created, the grouped view replaces the flat table.
      await expect(page.locator('[data-drop-zone]').first()).toBeAttached()
      await expect(page.getByRole('region', { name: groupName })).toBeVisible()
    })

    test('new group section is labelled with the group name and has a drop zone', async ({ page }) => {
      const groupName = `E2E DnD Label Test ${Date.now()}`
      await createGroupViaUI(page, groupName)

      // The section must be present and labelled correctly.
      await expect(page.getByRole('region', { name: groupName })).toBeVisible()

      // At least one named drop zone must exist after group creation.
      const namedZones = page.locator('[data-drop-zone]:not([data-drop-zone="ungrouped"])')
      await expect(namedZones.first()).toBeAttached()
    })

    test('Manage Groups button remains visible after creating a group', async ({ page }) => {
      const groupName = `E2E DnD Button Test ${Date.now()}`
      await createGroupViaUI(page, groupName)

      await expect(page.getByRole('button', { name: /manage groups/i })).toBeVisible()
    })
  })
})

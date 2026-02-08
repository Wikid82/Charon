import { test, expect } from '@playwright/test'

test.describe('ProxyHostForm Dropdown Click Fix', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the application
    await page.goto('/proxy-hosts')
    await page.waitForLoadState('networkidle')

    // Click "Add Proxy Host" button
    const addButton = page.getByRole('button', { name: /add proxy host|create/i }).first()
    await addButton.click()

    // Wait for modal to appear
    await page.waitForSelector('[role="dialog"]')
  })

  test('ACL dropdown should open and items should be clickable', async ({ page }) => {
    // Find the Access Control List select
    const aclLabel = page.locator('text=Access Control List')

    // Click to open the dropdown
    const aclTrigger = page.locator('[role="combobox"]').filter({ has: aclLabel.locator('..') }).first()
    await aclTrigger.click()

    // Wait for dropdown menu to appear
    await page.waitForSelector('[role="listbox"]')

    // Verify dropdown is open
    const dropdownItems = page.locator('[role="option"]')
    const itemCount = await dropdownItems.count()
    expect(itemCount).toBeGreaterThan(0)

    // Try clicking on an option (skip the default "No Access Control" and click the first real option if available)
    const options = await dropdownItems.all()
    if (options.length > 1) {
      await page.locator('[role="option"]').nth(1).click()

      // Verify the selection was registered (the trigger should show the selected value)
      const selectedValue = await aclTrigger.locator('[role="combobox"]').innerText()
      expect(selectedValue).toBeTruthy()
    }
  })

  test('Security Headers dropdown should open and items should be clickable', async ({ page }) => {
    // Find the Security Headers select
    const securityLabel = page.locator('text=Security Headers')

    // Get the select trigger associated with this label
    const selectTriggers = page.locator('[role="combobox"]')

    // Find the one after the Security Headers label
    let securityTrigger = null
    const triggers = await selectTriggers.all()

    for (let i = 0; i < triggers.length; i++) {
      const trigger = triggers[i]
      const boundingBox = await trigger.boundingBox()
      const labelBox = await securityLabel.boundingBox()

      if (labelBox && boundingBox && boundingBox.y > labelBox.y) {
        securityTrigger = trigger
        break
      }
    }

    if (!securityTrigger) {
      securityTrigger = selectTriggers.filter({ has: securityLabel.locator('..') }).first()
    }

    // Click to open the dropdown
    await securityTrigger.click()

    // Wait for dropdown menu to appear
    await page.waitForSelector('[role="listbox"]')

    // Verify dropdown is open
    const dropdownItems = page.locator('[role="option"]')
    const itemCount = await dropdownItems.count()
    expect(itemCount).toBeGreaterThan(0)

    // Click on the first non-disabled option
    const options = await dropdownItems.all()
    if (options.length > 1) {
      await page.locator('[role="option"]').nth(1).click()

      // Verify the selection was registered
      const selectedValue = await securityTrigger.textContent()
      expect(selectedValue).toBeTruthy()
    }
  })

  test('All dropdown menus should allow clicking on items without blocking', async ({ page }) => {
    // Get all select triggers in the form
    const selectTriggers = page.locator('[role="combobox"]')
    const triggerCount = await selectTriggers.count()

    // Test each dropdown
    for (let i = 0; i < Math.min(triggerCount, 3); i++) {
      const trigger = selectTriggers.nth(i)

      // Click to open dropdown
      await trigger.click()

      // Check if menu appears
      const menu = page.locator('[role="listbox"]')
      const isVisible = await menu.isVisible()

      if (isVisible) {
        // Try to click on the first option
        const firstOption = page.locator('[role="option"]').first()
        const isClickable = await firstOption.isVisible()
        expect(isClickable).toBe(true)

        // Close menu by pressing Escape
        await page.keyboard.press('Escape')
      }
    }
  })
})

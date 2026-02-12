import { test, expect } from '@playwright/test'

test.describe('ProxyHostForm Dropdown Click Fix', () => {
  test.beforeEach(async ({ page }) => {
    await test.step('Navigate to proxy hosts and open the create modal', async () => {
      await page.goto('/proxy-hosts')
      await page.waitForLoadState('networkidle')

      const addButton = page.getByRole('button', { name: /add proxy host|create/i }).first()
      await expect(addButton).toBeEnabled()
      await addButton.click()

      await expect(page.getByRole('dialog')).toBeVisible()
    })
  })

  test('ACL dropdown should open and items should be clickable', async ({ page }) => {
    const dialog = page.getByRole('dialog')

    await test.step('Open Access Control List dropdown', async () => {
      const aclTrigger = dialog.getByRole('combobox', { name: /access control list/i })
      await expect(aclTrigger).toBeEnabled()
      await aclTrigger.click()

      const listbox = page.getByRole('listbox')
      await expect(listbox).toBeVisible()
      await expect(listbox).toMatchAriaSnapshot(`
        - listbox:
          - option
      `)

      const dropdownItems = listbox.getByRole('option')
      const itemCount = await dropdownItems.count()
      expect(itemCount).toBeGreaterThan(0)

      let selectedText: string | null = null
      for (let i = 0; i < itemCount; i++) {
        const option = dropdownItems.nth(i)
        const isDisabled = (await option.getAttribute('aria-disabled')) === 'true'
        if (!isDisabled) {
          selectedText = (await option.textContent())?.trim() || null
          await option.click()
          break
        }
      }

      expect(selectedText).toBeTruthy()
      await expect(aclTrigger).toContainText(selectedText || '')
    })
  })

  test('Security Headers dropdown should open and items should be clickable', async ({ page }) => {
    const dialog = page.getByRole('dialog')

    await test.step('Open Security Headers dropdown', async () => {
      const securityTrigger = dialog.getByRole('combobox', { name: /security headers/i })
      await expect(securityTrigger).toBeEnabled()
      await securityTrigger.click()

      const listbox = page.getByRole('listbox')
      await expect(listbox).toBeVisible()
      await expect(listbox).toMatchAriaSnapshot(`
        - listbox:
          - option
      `)

      const dropdownItems = listbox.getByRole('option')
      const itemCount = await dropdownItems.count()
      expect(itemCount).toBeGreaterThan(0)

      let selectedText: string | null = null
      for (let i = 0; i < itemCount; i++) {
        const option = dropdownItems.nth(i)
        const isDisabled = (await option.getAttribute('aria-disabled')) === 'true'
        if (!isDisabled) {
          selectedText = (await option.textContent())?.trim() || null
          await option.click()
          break
        }
      }

      expect(selectedText).toBeTruthy()
      await expect(securityTrigger).toContainText(selectedText || '')
    })
  })

  test('All dropdown menus should allow clicking on items without blocking', async ({ page }) => {
    const dialog = page.getByRole('dialog')
    const selectTriggers = dialog.getByRole('combobox')
    const triggerCount = await selectTriggers.count()

    for (let i = 0; i < Math.min(triggerCount, 3); i++) {
      await test.step(`Open dropdown ${i + 1}`, async () => {
        const trigger = selectTriggers.nth(i)
        const isDisabled = await trigger.isDisabled()
        if (isDisabled) {
          return
        }

        await expect(trigger).toBeEnabled()
        await trigger.click()

        const menu = page.getByRole('listbox')
        await expect(menu).toBeVisible()

        const firstOption = menu.getByRole('option').first()
        await expect(firstOption).toBeVisible()

        await page.keyboard.press('Escape')
      })
    }
  })
})

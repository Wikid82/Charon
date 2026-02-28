import { test, expect } from '@playwright/test'

type SelectionPair = {
  aclLabel: string
  securityHeadersLabel: string
}

async function dismissDomainDialog(page: import('@playwright/test').Page): Promise<void> {
  const noThanksButton = page.getByRole('button', { name: /no, thanks/i })
  if (await noThanksButton.isVisible({ timeout: 1200 }).catch(() => false)) {
    await noThanksButton.click()
  }
}

async function openCreateModal(page: import('@playwright/test').Page): Promise<void> {
  const addButton = page.getByRole('button', { name: /add.*proxy.*host|create/i }).first()
  await expect(addButton).toBeEnabled()
  await addButton.click()
  await expect(page.getByRole('dialog')).toBeVisible()
}

async function selectFirstUsableOption(
  page: import('@playwright/test').Page,
  trigger: import('@playwright/test').Locator,
  skipPattern: RegExp
): Promise<string> {
  await trigger.click()
  const listbox = page.getByRole('listbox')
  await expect(listbox).toBeVisible()

  const options = listbox.getByRole('option')
  const optionCount = await options.count()
  expect(optionCount).toBeGreaterThan(0)

  for (let i = 0; i < optionCount; i++) {
    const option = options.nth(i)
    const rawLabel = (await option.textContent())?.trim() || ''
    const isDisabled = (await option.getAttribute('aria-disabled')) === 'true'

    if (isDisabled || !rawLabel || skipPattern.test(rawLabel)) {
      continue
    }

    await option.click()
    return rawLabel
  }

  throw new Error('No selectable non-default option found in dropdown')
}

async function selectOptionByName(
  page: import('@playwright/test').Page,
  trigger: import('@playwright/test').Locator,
  optionName: RegExp
): Promise<string> {
  await trigger.click()
  const listbox = page.getByRole('listbox')
  await expect(listbox).toBeVisible()

  const option = listbox.getByRole('option', { name: optionName }).first()
  await expect(option).toBeVisible()
  const label = ((await option.textContent()) || '').trim()
  await option.click()
  return label
}

async function saveProxyHost(page: import('@playwright/test').Page): Promise<void> {
  await dismissDomainDialog(page)

  const saveButton = page
    .getByTestId('proxy-host-save')
    .or(page.getByRole('button', { name: /^save$/i }))
    .first()
  await expect(saveButton).toBeEnabled()
  await saveButton.click()

  const confirmSave = page.getByRole('button', { name: /yes.*save/i }).first()
  if (await confirmSave.isVisible({ timeout: 1200 }).catch(() => false)) {
    await confirmSave.click()
  }

  await expect(page.getByRole('dialog')).not.toBeVisible({ timeout: 10000 })
}

async function openEditModalForDomain(page: import('@playwright/test').Page, domain: string): Promise<void> {
  const row = page.locator('tbody tr').filter({ hasText: domain }).first()
  await expect(row).toBeVisible({ timeout: 10000 })

  const editButton = row.getByRole('button', { name: /edit proxy host|edit/i }).first()
  await expect(editButton).toBeVisible()
  await editButton.click()
  await expect(page.getByRole('dialog')).toBeVisible()
}

async function selectNonDefaultPair(
  page: import('@playwright/test').Page,
  dialog: import('@playwright/test').Locator
): Promise<SelectionPair> {
  const aclTrigger = dialog.getByRole('combobox', { name: /access control list/i })
  const securityHeadersTrigger = dialog.getByRole('combobox', { name: /security headers/i })

  const aclLabel = await selectFirstUsableOption(page, aclTrigger, /no access control|public/i)
  await expect(aclTrigger).toContainText(aclLabel)

  const securityHeadersLabel = await selectFirstUsableOption(page, securityHeadersTrigger, /none \(no security headers\)/i)
  await expect(securityHeadersTrigger).toContainText(securityHeadersLabel)

  return { aclLabel, securityHeadersLabel }
}

test.describe.skip('ProxyHostForm ACL/Security Headers Regression (moved to security shard)', () => {
  test('should keep ACL and Security Headers behavior equivalent across create/edit flows', async ({ page }) => {
    const suffix = Date.now()
    const proxyName = `Dropdown Regression ${suffix}`
    const proxyDomain = `dropdown-${suffix}.test.local`

    await test.step('Navigate to Proxy Hosts', async () => {
      await page.goto('/proxy-hosts')
      await page.waitForLoadState('networkidle')
      await expect(page.getByRole('heading', { name: /proxy hosts/i })).toBeVisible()
    })

    await test.step('Create flow: select ACL + Security Headers and verify immediate form state', async () => {
      await openCreateModal(page)
      const dialog = page.getByRole('dialog')

      await dialog.locator('#proxy-name').fill(proxyName)
      await dialog.locator('#domain-names').click()
      await page.keyboard.type(proxyDomain)
      await page.keyboard.press('Tab')
      await dismissDomainDialog(page)

      await dialog.locator('#forward-host').fill('127.0.0.1')
      await dialog.locator('#forward-port').fill('8080')

      const initialSelection = await selectNonDefaultPair(page, dialog)

      await saveProxyHost(page)

      await openEditModalForDomain(page, proxyDomain)
      const reopenDialog = page.getByRole('dialog')
      await expect(reopenDialog.getByRole('combobox', { name: /access control list/i })).toContainText(initialSelection.aclLabel)
      await expect(reopenDialog.getByRole('combobox', { name: /security headers/i })).toContainText(initialSelection.securityHeadersLabel)
      await reopenDialog.getByRole('button', { name: /cancel/i }).click()
      await expect(reopenDialog).not.toBeVisible({ timeout: 5000 })
    })

    await test.step('Edit flow: change ACL + Security Headers and verify persisted updates', async () => {
      await openEditModalForDomain(page, proxyDomain)
      const dialog = page.getByRole('dialog')

      const updatedSelection = await selectNonDefaultPair(page, dialog)
      await saveProxyHost(page)

      await openEditModalForDomain(page, proxyDomain)
      const reopenDialog = page.getByRole('dialog')
      await expect(reopenDialog.getByRole('combobox', { name: /access control list/i })).toContainText(updatedSelection.aclLabel)
      await expect(reopenDialog.getByRole('combobox', { name: /security headers/i })).toContainText(updatedSelection.securityHeadersLabel)
      await reopenDialog.getByRole('button', { name: /cancel/i }).click()
      await expect(reopenDialog).not.toBeVisible({ timeout: 5000 })
    })

    await test.step('Edit flow: clear both to none/null and verify persisted clearing', async () => {
      await openEditModalForDomain(page, proxyDomain)
      const dialog = page.getByRole('dialog')

      const aclTrigger = dialog.getByRole('combobox', { name: /access control list/i })
      const securityHeadersTrigger = dialog.getByRole('combobox', { name: /security headers/i })

      const aclNoneLabel = await selectOptionByName(page, aclTrigger, /no access control \(public\)/i)
      await expect(aclTrigger).toContainText(aclNoneLabel)

      const securityNoneLabel = await selectOptionByName(page, securityHeadersTrigger, /none \(no security headers\)/i)
      await expect(securityHeadersTrigger).toContainText(securityNoneLabel)

      await saveProxyHost(page)

      await openEditModalForDomain(page, proxyDomain)
      const reopenDialog = page.getByRole('dialog')
      await expect(reopenDialog.getByRole('combobox', { name: /access control list/i })).toContainText(/no access control \(public\)/i)
      await expect(reopenDialog.getByRole('combobox', { name: /security headers/i })).toContainText(/none \(no security headers\)/i)
      await reopenDialog.getByRole('button', { name: /cancel/i }).click()
      await expect(reopenDialog).not.toBeVisible({ timeout: 5000 })
    })
  })
})

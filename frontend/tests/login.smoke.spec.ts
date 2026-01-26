import { test, expect } from '@bgotink/playwright-coverage'

test.describe('Login - smoke', () => {
  test('renders and has no console errors on load', async ({ page }) => {
    const consoleErrors: string[] = []

    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text())
      }
    })

    await page.goto('/login', { waitUntil: 'domcontentloaded' })

    await expect(page).toHaveURL(/\/login(?:\?|$)/)

    const emailInput = page.getByRole('textbox', { name: /email/i })
    const passwordInput = page.getByLabel(/password/i)

    await expect(emailInput).toBeVisible()
    await expect(passwordInput).toBeVisible()
    await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible()

    expect(consoleErrors, 'Console errors during /login load').toEqual([])
  })
})

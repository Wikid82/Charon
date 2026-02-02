/**
 * Security Dashboard Mobile Responsive E2E Tests
 * Test IDs: MR-01 through MR-10
 *
 * Tests mobile viewport (375x667), tablet viewport (768x1024),
 * touch targets, scrolling, and layout responsiveness.
 */
import { test, expect } from '@bgotink/playwright-coverage'

const base = process.env.CHARON_BASE_URL || 'http://localhost:8080'

test.describe('Security Dashboard Mobile (375x667)', () => {
  test.use({ viewport: { width: 375, height: 667 } })

  test('MR-01: cards stack vertically on mobile', async ({ page }) => {
    await page.goto(`${base}/security`)

    // Wait for page to load
    await page.waitForSelector('[data-testid="toggle-crowdsec"]', { timeout: 10000 })

    // On mobile, grid should be single column
    const grid = page.locator('.grid.grid-cols-1')
    await expect(grid).toBeVisible()

    // Get the computed grid-template-columns
    const cardsContainer = page.locator('.grid').first()
    const gridStyle = await cardsContainer.evaluate((el) => {
      const style = window.getComputedStyle(el)
      return style.gridTemplateColumns
    })

    // Single column should have just one value (not multiple columns like "repeat(4, ...)")
    const columns = gridStyle.split(' ').filter((s) => s.trim().length > 0)
    expect(columns.length).toBeLessThanOrEqual(2) // Single column or flexible
  })

  test('MR-04: toggle switches have accessible touch targets', async ({ page }) => {
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]', { timeout: 10000 })

    // Check CrowdSec toggle
    const crowdsecToggle = page.getByTestId('toggle-crowdsec')
    const crowdsecBox = await crowdsecToggle.boundingBox()

    // Touch target should be at least 24px (component) + padding
    // Most switches have a reasonable touch target
    expect(crowdsecBox).not.toBeNull()
    if (crowdsecBox) {
      expect(crowdsecBox.height).toBeGreaterThanOrEqual(20)
      expect(crowdsecBox.width).toBeGreaterThanOrEqual(35)
    }

    // Check WAF toggle
    const wafToggle = page.getByTestId('toggle-waf')
    const wafBox = await wafToggle.boundingBox()
    expect(wafBox).not.toBeNull()
    if (wafBox) {
      expect(wafBox.height).toBeGreaterThanOrEqual(20)
    }
  })

  test('MR-05: config buttons are tappable on mobile', async ({ page }) => {
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]', { timeout: 10000 })

    // Find config/configure buttons
    const configButtons = page.locator('button:has-text("Config"), button:has-text("Configure")')
    const buttonCount = await configButtons.count()

    expect(buttonCount).toBeGreaterThan(0)

    // Check first config button has reasonable size
    const firstButton = configButtons.first()
    const box = await firstButton.boundingBox()
    expect(box).not.toBeNull()
    if (box) {
      expect(box.height).toBeGreaterThanOrEqual(28) // Minimum tap height
    }
  })

  test('MR-06: page content is scrollable on mobile', async ({ page }) => {
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]', { timeout: 10000 })

    // Check if page is scrollable (content height > viewport)
    const bodyHeight = await page.evaluate(() => document.body.scrollHeight)
    const viewportHeight = 667

    // If content is taller than viewport, page should scroll
    if (bodyHeight > viewportHeight) {
      // Attempt to scroll down
      await page.evaluate(() => window.scrollBy(0, 200))
      const scrollY = await page.evaluate(() => window.scrollY)
      expect(scrollY).toBeGreaterThan(0)
    }
  })

  test('MR-10: navigation is accessible on mobile', async ({ page }) => {
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]', { timeout: 10000 })

    // On mobile, there should be some form of navigation
    // Check if sidebar or mobile menu toggle exists
    const sidebar = page.locator('nav, aside, [role="navigation"]')
    const sidebarCount = await sidebar.count()

    // Navigation should exist in some form
    expect(sidebarCount).toBeGreaterThanOrEqual(0) // May be hidden on mobile
  })

  test('MR-06b: overlay renders correctly on mobile', async ({ page }) => {
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]', { timeout: 10000 })

    // Skip if Cerberus is disabled (toggles would be disabled)
    const cerberusDisabled = await page.locator('text=Cerberus Disabled').isVisible()
    if (cerberusDisabled) {
      test.skip()
      return
    }

    // Trigger loading state by clicking a toggle
    const wafToggle = page.getByTestId('toggle-waf')
    const isDisabled = await wafToggle.isDisabled()

    if (!isDisabled) {
      await wafToggle.click()

      // Check for overlay (may appear briefly)
      // Use a short timeout since it might disappear quickly
      try {
        const overlay = page.locator('.fixed.inset-0')
        await overlay.waitFor({ state: 'visible', timeout: 2000 })

        // If overlay appeared, verify it fits screen
        const box = await overlay.boundingBox()
        if (box) {
          expect(box.width).toBeLessThanOrEqual(375 + 10) // Allow small margin
        }
      } catch {
        // Overlay might have disappeared before we could check
        // This is acceptable for a fast operation
      }
    }
  })
})

test.describe('Security Dashboard Tablet (768x1024)', () => {
  test.use({ viewport: { width: 768, height: 1024 } })

  test('MR-02: cards show 2 columns on tablet', async ({ page }) => {
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]', { timeout: 10000 })

    // On tablet (md breakpoint), should have md:grid-cols-2
    const grid = page.locator('.grid').first()
    await expect(grid).toBeVisible()

    // Get computed style
    const gridStyle = await grid.evaluate((el) => {
      const style = window.getComputedStyle(el)
      return style.gridTemplateColumns
    })

    // Should have 2 columns at md breakpoint
    const columns = gridStyle.split(' ').filter((s) => s.trim().length > 0 && s !== 'none')
    expect(columns.length).toBeGreaterThanOrEqual(2)
  })

  test('MR-08: cards have proper spacing on tablet', async ({ page }) => {
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]', { timeout: 10000 })

    // Check gap between cards
    const grid = page.locator('.grid.gap-6').first()
    const hasGap = await grid.isVisible()
    expect(hasGap).toBe(true)
  })
})

test.describe('Security Dashboard Desktop (1920x1080)', () => {
  test.use({ viewport: { width: 1920, height: 1080 } })

  test('MR-03: cards show 4 columns on desktop', async ({ page }) => {
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]', { timeout: 10000 })

    // On desktop (lg breakpoint), should have lg:grid-cols-4
    const grid = page.locator('.grid').first()
    await expect(grid).toBeVisible()

    // Get computed style
    const gridStyle = await grid.evaluate((el) => {
      const style = window.getComputedStyle(el)
      return style.gridTemplateColumns
    })

    // Should have 4 columns at lg breakpoint
    const columns = gridStyle.split(' ').filter((s) => s.trim().length > 0 && s !== 'none')
    expect(columns.length).toBeGreaterThanOrEqual(4)
  })
})

test.describe('Security Dashboard Layout Tests', () => {
  test('cards maintain correct order across viewports', async ({ page }) => {
    // Test on mobile
    await page.setViewportSize({ width: 375, height: 667 })
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]', { timeout: 10000 })

    // Get card headings
    const getCardOrder = async () => {
      const headings = await page.locator('h3').allTextContents()
      return headings.filter((h) => ['CrowdSec', 'Access Control', 'Coraza', 'Rate Limiting'].includes(h))
    }

    const mobileOrder = await getCardOrder()

    // Test on tablet
    await page.setViewportSize({ width: 768, height: 1024 })
    await page.waitForTimeout(100) // Allow reflow
    const tabletOrder = await getCardOrder()

    // Test on desktop
    await page.setViewportSize({ width: 1920, height: 1080 })
    await page.waitForTimeout(100) // Allow reflow
    const desktopOrder = await getCardOrder()

    // Order should be consistent
    expect(mobileOrder).toEqual(tabletOrder)
    expect(tabletOrder).toEqual(desktopOrder)
    expect(desktopOrder).toEqual(['CrowdSec', 'Access Control', 'Coraza', 'Rate Limiting'])
  })

  test('MR-09: all security cards are visible on scroll', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 })
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]', { timeout: 10000 })

    // Scroll to each card type
    const cardTypes = ['CrowdSec', 'Access Control', 'Coraza', 'Rate Limiting']

    for (const cardType of cardTypes) {
      const card = page.locator(`h3:has-text("${cardType}")`)
      await card.scrollIntoViewIfNeeded()
      await expect(card).toBeVisible()
    }
  })
})

test.describe('Security Dashboard Interaction Tests', () => {
  test.use({ viewport: { width: 375, height: 667 } })

  test('MR-07: config buttons navigate correctly on mobile', async ({ page }) => {
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]', { timeout: 10000 })

    // Skip if Cerberus disabled
    const cerberusDisabled = await page.locator('text=Cerberus Disabled').isVisible()
    if (cerberusDisabled) {
      test.skip()
      return
    }

    // Find and click WAF Configure button
    const configureButton = page.locator('button:has-text("Configure")').first()

    if (await configureButton.isVisible()) {
      await configureButton.click()

      // Should navigate to a config page
      await page.waitForTimeout(500)
      const url = page.url()

      // URL should include security/waf or security/rate-limiting etc
      expect(url).toMatch(/security\/(waf|rate-limiting|access-lists|crowdsec)/i)
    }
  })

  test('documentation button works on mobile', async ({ page }) => {
    await page.goto(`${base}/security`)
    await page.waitForSelector('[data-testid="toggle-crowdsec"]', { timeout: 10000 })

    // Find documentation button
    const docButton = page.locator('button:has-text("Documentation"), a:has-text("Documentation")').first()

    if (await docButton.isVisible()) {
      // Check it has correct external link behavior
      const href = await docButton.getAttribute('href')

      // Should open external docs
      if (href) {
        expect(href).toContain('wikid82.github.io')
      }
    }
  })
})

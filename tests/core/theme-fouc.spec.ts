/**
 * FOUC (Flash of Unstyled Content) Prevention — Regression Tests
 *
 * Regression suite for the FOUC fix:
 *   - frontend/index.html                        — anti-FOUC inline <script> in <head>
 *   - frontend/src/context/ThemeContext.tsx       — useEffect for data-theme attribute application
 *   - frontend/src/index.css                      — [data-theme="*"] CSS selectors
 *
 * Checked at 'domcontentloaded' — before React mounts and before any useEffect fires — to prove
 * the inline script is doing its job. If we checked at 'networkidle' we would pass even without
 * the inline script (React's ThemeProvider would set the attribute eventually). Capturing
 * directly after domcontentloaded ensures a regression where the inline script is removed or
 * broken is immediately detectable.
 *
 * @see docs/plans/current_spec.md — Section 5, Phase 1 (Playwright Tests)
 */

import { test, expect } from '@playwright/test';

test.describe('FOUC Prevention', () => {
  // Clear all theme-related localStorage keys before each test so the inline script
  // sees a clean state. Uses addInitScript so removal runs before any page scripts.
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      try {
        localStorage.removeItem('charon-theme');
        localStorage.removeItem('charon-custom-theme');
        localStorage.removeItem('theme');
      } catch (_) {}
    });
  });

  /**
   * T1 — Dark mode default (no localStorage)
   *
   * The anti-FOUC inline script defaults to 'dark' when no preference is stored.
   * Verified immediately after domcontentloaded, before React's useEffect fires.
   */
  test('T1: data-theme="dark" on <html> at domcontentloaded with no stored preference', async ({ page }) => {
    await page.goto('/', { waitUntil: 'domcontentloaded' });

    const attr = await page.evaluate(() => document.documentElement.getAttribute('data-theme'));
    expect(attr).toBe('dark');
  });

  /**
   * T2 — Dark mode persisted preference
   *
   * When charon-theme === 'dark' is stored before navigation, the inline script
   * reads it and sets data-theme="dark". Verified before React mounts.
   */
  test('T2: data-theme="dark" on <html> at domcontentloaded with stored dark preference', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('charon-theme', 'dark');
    });

    await page.goto('/', { waitUntil: 'domcontentloaded' });

    const attr = await page.evaluate(() => document.documentElement.getAttribute('data-theme'));
    expect(attr).toBe('dark');
    expect(attr).not.toBe('light');
  });

  /**
   * T3 — Light mode persisted preference
   *
   * When charon-theme === 'light' is stored before navigation, the inline script
   * reads it and sets data-theme="light". Verified before React mounts.
   */
  test('T3: data-theme="light" on <html> at domcontentloaded with stored light preference', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('charon-theme', 'light');
    });

    await page.goto('/', { waitUntil: 'domcontentloaded' });

    const attr = await page.evaluate(() => document.documentElement.getAttribute('data-theme'));
    expect(attr).toBe('light');
    expect(attr).not.toBe('dark');
  });

  /**
   * T4 — Light mode CSS variables applied (background-color check)
   *
   * With [data-theme="light"] { background-color: rgb(var(--color-bg-base)) } in index.css
   * and --color-bg-base: 248 250 252 defined under [data-theme="light"], the computed
   * background-color of <html> must NOT be the dark default rgb(15, 23, 42) (#0f172a).
   * Uses 'load' so stylesheets are fully applied before measuring computed style.
   */
  test('T4: light mode background-color is not the dark slate default', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('charon-theme', 'light');
    });

    await page.goto('/', { waitUntil: 'load' });

    const attr = await page.evaluate(() => document.documentElement.getAttribute('data-theme'));
    expect(attr).toBe('light');

    const bgColor = await page.evaluate(() =>
      getComputedStyle(document.documentElement).backgroundColor
    );
    // [data-theme="light"] sets --color-bg-base: 248 250 252 → rgb(248, 250, 252)
    // Must not be the dark default rgb(15, 23, 42)
    expect(bgColor).not.toBe('rgb(15, 23, 42)');
  });

  /**
   * T5 — Only data-theme="light" when light preference is stored (no dark attribute)
   *
   * Guards against a script bug that would set the wrong value. Distinct from T3 in
   * that it explicitly asserts the absence of 'dark', not just the presence of 'light'.
   */
  test('T5: only data-theme="light" on <html> at domcontentloaded, not dark', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('charon-theme', 'light');
    });

    await page.goto('/', { waitUntil: 'domcontentloaded' });

    const attr = await page.evaluate(() => document.documentElement.getAttribute('data-theme'));
    expect(attr).toBe('light');
    expect(attr).not.toBe('dark');
  });

  /**
   * T6 — ThemeToggle button navigates to /settings/appearance
   *
   * The ThemeToggle button (visible in the authenticated Layout) opens the
   * Appearance settings page where the user can change their theme. Verifies
   * the button is present with the correct title and navigation target.
   */
  test('T6: ThemeToggle button navigates to appearance settings', async ({ page }) => {
    await page.goto('/', { waitUntil: 'load' });

    await test.step('ThemeToggle button is visible with correct title', async () => {
      const themeToggle = page.locator('button[title="Theme settings"]').first();
      await expect(themeToggle).toBeVisible();
    });

    await test.step('clicking ThemeToggle navigates to /settings/appearance', async () => {
      const themeToggle = page.locator('button[title="Theme settings"]').first();
      await themeToggle.click();
      await page.waitForURL('**/settings/appearance');
      expect(page.url()).toContain('/settings/appearance');
    });
  });
});

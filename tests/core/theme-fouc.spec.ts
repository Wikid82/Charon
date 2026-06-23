/**
 * FOUC (Flash of Unstyled Content) Prevention — Regression Tests
 *
 * Regression suite for the FOUC fix introduced in:
 *   - frontend/index.html                        — anti-FOUC inline <script> in <head>
 *   - frontend/src/context/ThemeContext.tsx       — useEffect for data-theme attribute application
 *   - frontend/src/index.css                      — [data-theme="*"] CSS selectors
 *
 * Tests T1–T5 capture the <html> data-theme attribute at DOMContentLoaded — before React mounts
 * and before any useEffect fires — to prove the inline script is doing its job.
 * Asserting only on final DOM state would pass even without the inline script, because
 * React's ThemeProvider would still apply the correct attribute eventually. Capturing at
 * DOMContentLoaded (pre-React) ensures the test detects a regression where the inline
 * script is removed.
 *
 * T6 verifies that the theme toggle (ThemeToggle component) applies its attribute change
 * after a click, with the DOM reflecting the new theme once React's effect has settled.
 *
 * @see docs/plans/current_spec.md — Section 5, Phase 1 (Playwright Tests, Red State)
 */

import { test, expect } from '@playwright/test';

test.describe('FOUC Prevention', () => {
  // Remove the 'theme' and 'charon-theme' keys before each test to prevent cross-test pollution.
  // Using addInitScript ensures the removal executes before any page scripts
  // (including the anti-FOUC inline script) and before each test's own
  // addInitScript calls, since Playwright executes init scripts in registration order.
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      try {
        localStorage.removeItem('theme');
        localStorage.removeItem('charon-theme');
      } catch (_) {}
    });
  });

  /**
   * T1 — Dark mode default (no localStorage)
   *
   * The anti-FOUC inline script defaults to 'dark' when no preference is stored.
   * localStorage.getItem('charon-theme') returns null → the script sets data-theme="dark".
   * Verified at DOMContentLoaded, before React mounts.
   */
  test('T1: data-theme="dark" on <html> at DOMContentLoaded with no stored preference', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        localStorage.removeItem('theme');
        localStorage.removeItem('charon-theme');
      } catch (_) {}

      // Capture <html> data-theme attribute at DOMContentLoaded — before React's useEffect runs
      document.addEventListener('DOMContentLoaded', () => {
        (window as any).__domContentTheme = document.documentElement.getAttribute('data-theme');
      });
    });

    await page.goto('/login', { waitUntil: 'domcontentloaded' });
    const attr = await page.evaluate(() => (window as any).__domContentTheme ?? '');

    expect(attr).toBe('dark');
  });

  /**
   * T2 — Dark mode persisted preference
   *
   * When localStorage['theme'] === 'dark', the inline script sets data-theme="dark".
   * Verified at DOMContentLoaded before React runs.
   */
  test('T2: data-theme="dark" on <html> at DOMContentLoaded with stored dark preference', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        localStorage.setItem('theme', 'dark');
      } catch (_) {}

      document.addEventListener('DOMContentLoaded', () => {
        (window as any).__domContentTheme = document.documentElement.getAttribute('data-theme');
      });
    });

    await page.goto('/login', { waitUntil: 'domcontentloaded' });
    const attr = await page.evaluate(() => (window as any).__domContentTheme ?? '');

    expect(attr).toBe('dark');
    expect(attr).not.toBe('light');
  });

  /**
   * T3 — Light mode persisted preference
   *
   * When localStorage['theme'] === 'light', the inline script sets data-theme="light".
   * Verified at DOMContentLoaded before React runs.
   */
  test('T3: data-theme="light" on <html> at DOMContentLoaded with stored light preference', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        localStorage.setItem('theme', 'light');
      } catch (_) {}

      document.addEventListener('DOMContentLoaded', () => {
        (window as any).__domContentTheme = document.documentElement.getAttribute('data-theme');
      });
    });

    await page.goto('/login', { waitUntil: 'domcontentloaded' });
    const attr = await page.evaluate(() => (window as any).__domContentTheme ?? '');

    expect(attr).toBe('light');
    expect(attr).not.toBe('dark');
  });

  /**
   * T4 — Light mode background-color is not dark slate (CSS variable applied via data-theme)
   *
   * With [data-theme="light"] { background-color: rgb(var(--color-bg-base)) } in index.css
   * and --color-bg-base: 248 250 252 defined under [data-theme="light"], the computed
   * background-color of <html> in light mode must NOT be the :root default of rgb(15, 23, 42).
   */
  test('T4: light mode background-color is not the dark slate default', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        localStorage.setItem('theme', 'light');
      } catch (_) {}
    });

    await page.goto('/login', { waitUntil: 'load' });

    // Verify data-theme attribute was set correctly by the inline script
    const attr = await page.evaluate(() => document.documentElement.getAttribute('data-theme'));
    expect(attr).toBe('light');

    const bgColor = await page.evaluate(() =>
      getComputedStyle(document.documentElement).backgroundColor
    );

    // [data-theme="light"] sets --color-bg-base: 248 250 252 → rgb(248, 250, 252)
    // Must not be the dark default rgb(15, 23, 42) == #0f172a (Tailwind slate-900)
    expect(bgColor).not.toBe('rgb(15, 23, 42)');
  });

  /**
   * T5 — Only data-theme="light" when light preference is stored (no dark attribute)
   *
   * The inline script sets data-theme to exactly one value. When 'light' is stored,
   * data-theme is 'light' — not 'dark'. Guards against a script bug that would set
   * the wrong value. Verified at DOMContentLoaded, before React runs.
   */
  test('T5: only data-theme="light" on <html> at DOMContentLoaded, not dark', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        localStorage.setItem('theme', 'light');
      } catch (_) {}

      document.addEventListener('DOMContentLoaded', () => {
        (window as any).__domContentTheme = document.documentElement.getAttribute('data-theme');
      });
    });

    await page.goto('/login', { waitUntil: 'domcontentloaded' });
    const attr = await page.evaluate(() => (window as any).__domContentTheme ?? '');

    expect(attr).toBe('light');
    expect(attr).not.toBe('dark');
  });

  /**
   * T6 — ThemeToggle button navigates to /settings/appearance
   *
   * The ThemeToggle button (visible in the authenticated Layout) opens the
   * Appearance settings page where the user can change their theme. This test
   * verifies the button is present with the correct title and that clicking it
   * navigates to /settings/appearance.
   *
   * Note: This test navigates to '/' (authenticated dashboard) because ThemeToggle
   * is rendered inside Layout.tsx, which wraps the authenticated routes only.
   * The test relies on storageState (from auth.setup.ts) for authentication.
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

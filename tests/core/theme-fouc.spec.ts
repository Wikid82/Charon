/**
 * FOUC (Flash of Unstyled Content) Prevention — Regression Tests
 *
 * Regression suite for the FOUC fix introduced in:
 *   - frontend/index.html                        — anti-FOUC inline <script> in <head>
 *   - frontend/src/context/ThemeContext.tsx       — useLayoutEffect for sync class application
 *   - frontend/src/index.css                      — explicit light-mode background/color overrides
 *
 * Tests T1–T5 capture the <html> className at DOMContentLoaded — before React mounts
 * and before useLayoutEffect fires — to prove the inline script is doing its job.
 * Asserting only on final DOM state would pass even without the inline script, because
 * React's useLayoutEffect would still apply the correct class eventually. Capturing at
 * DOMContentLoaded (pre-React) ensures the test detects a regression where the inline
 * script is removed.
 *
 * T6 verifies that the theme toggle (ThemeToggle component) applies its class change
 * synchronously via useLayoutEffect, with no intermediate paint between click and update.
 *
 * @see docs/plans/current_spec.md — Section 5, Phase 1 (Playwright Tests, Red State)
 */

import { test, expect } from '@playwright/test';

test.describe('FOUC Prevention', () => {
  // Remove the 'theme' key before each test to prevent cross-test pollution.
  // Using addInitScript ensures the removal executes before any page scripts
  // (including the anti-FOUC inline script) and before each test's own
  // addInitScript calls, since Playwright executes init scripts in registration order.
  // Only 'theme' is removed (not a full clear) to preserve auth tokens in localStorage.
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      try {
        localStorage.removeItem('theme');
      } catch (_) {}
    });
  });

  /**
   * T1 — Dark mode default (no localStorage)
   *
   * The anti-FOUC inline script defaults to 'dark' when no preference is stored.
   * localStorage.getItem('theme') returns null → the script applies classList.add('dark').
   * Verified at DOMContentLoaded, before React mounts.
   */
  test('T1: dark class on <html> at DOMContentLoaded with no stored preference', async ({ page }) => {
    await page.addInitScript(() => {
      // Simulate a first-time visitor: no stored theme preference
      try {
        localStorage.removeItem('theme');
      } catch (_) {}

      // Capture <html> className at DOMContentLoaded — before React's useLayoutEffect runs
      document.addEventListener('DOMContentLoaded', () => {
        (window as any).__domContentClasses = document.documentElement.className;
      });
    });

    await page.goto('/login', { waitUntil: 'domcontentloaded' });
    const classes = await page.evaluate(() => (window as any).__domContentClasses ?? '');

    expect(classes).toContain('dark');
  });

  /**
   * T2 — Dark mode persisted preference
   *
   * When localStorage['theme'] === 'dark', the inline script applies 'dark'
   * and must NOT apply 'light'. Verified at DOMContentLoaded before React runs.
   */
  test('T2: dark class on <html> at DOMContentLoaded with stored dark preference', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        localStorage.setItem('theme', 'dark');
      } catch (_) {}

      document.addEventListener('DOMContentLoaded', () => {
        (window as any).__domContentClasses = document.documentElement.className;
      });
    });

    await page.goto('/login', { waitUntil: 'domcontentloaded' });
    const classes = await page.evaluate(() => (window as any).__domContentClasses ?? '');

    expect(classes).toContain('dark');
    expect(classes).not.toContain('light');
  });

  /**
   * T3 — Light mode persisted preference
   *
   * When localStorage['theme'] === 'light', the inline script applies 'light'
   * and must NOT apply 'dark'. Verified at DOMContentLoaded before React runs.
   */
  test('T3: light class on <html> at DOMContentLoaded with stored light preference', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        localStorage.setItem('theme', 'light');
      } catch (_) {}

      document.addEventListener('DOMContentLoaded', () => {
        (window as any).__domContentClasses = document.documentElement.className;
      });
    });

    await page.goto('/login', { waitUntil: 'domcontentloaded' });
    const classes = await page.evaluate(() => (window as any).__domContentClasses ?? '');

    expect(classes).toContain('light');
    expect(classes).not.toContain('dark');
  });

  /**
   * T4 — Light mode background color is not dark slate (depends on CSS fix)
   *
   * After the .light { background-color: #f0f4f8 } override is added to index.css,
   * the computed background-color of <html> in light mode must NOT be the :root
   * default of rgb(15, 23, 42) (#0f172a, Tailwind slate-900).
   *
   * Requires Fix 4.4 (index.css .light block background-color override) to pass.
   * If the fix is not applied, :root's background-color: #0f172a wins and this fails.
   */
  test('T4: light mode background-color is not the dark slate default', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        localStorage.setItem('theme', 'light');
      } catch (_) {}
    });

    await page.goto('/login', { waitUntil: 'load' });

    const bgColor = await page.evaluate(() =>
      getComputedStyle(document.documentElement).backgroundColor
    );

    // :root dark default is rgb(15, 23, 42) == #0f172a (Tailwind slate-900)
    // With .light { background-color: #f0f4f8 } applied, this must differ
    expect(bgColor).not.toBe('rgb(15, 23, 42)');
  });

  /**
   * T5 — No dual-class false positive (only .light when light preference is stored)
   *
   * The inline script adds exactly one class. When 'light' is stored, only 'light'
   * is present — not both 'dark' and 'light' simultaneously.
   * Guards against a script bug that would add both classes at the same time.
   * Verified at DOMContentLoaded, before React runs.
   */
  test('T5: only light class on <html> at DOMContentLoaded, no dark class present', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        localStorage.setItem('theme', 'light');
      } catch (_) {}

      document.addEventListener('DOMContentLoaded', () => {
        (window as any).__domContentClasses = document.documentElement.className;
      });
    });

    await page.goto('/login', { waitUntil: 'domcontentloaded' });
    const classes = await page.evaluate(() => (window as any).__domContentClasses ?? '');

    expect(classes).toContain('light');
    expect(classes).not.toContain('dark');
  });

  /**
   * T6 — Theme toggle applies class change synchronously (no flicker)
   *
   * useLayoutEffect fires synchronously in React's commit phase — before the browser
   * paints. After clicking the ThemeToggle button, the <html> class must be updated
   * in the same tick, observable immediately after the click resolves before any
   * animation frame or repaint.
   *
   * Starting from the dark default, clicking the toggle must atomically switch to 'light'.
   *
   * Note: This test navigates to '/' (authenticated dashboard) because ThemeToggle
   * is rendered inside Layout.tsx, which wraps the authenticated routes only.
   * The test relies on storageState (from auth.setup.ts) for authentication.
   */
  test('T6: theme toggle applies class change synchronously via useLayoutEffect', async ({ page }) => {
    // Ensure dark mode as the starting state (the default when no preference is stored)
    await page.addInitScript(() => {
      try {
        localStorage.removeItem('theme');
      } catch (_) {}
    });

    // Navigate to authenticated dashboard — ThemeToggle lives inside Layout
    await page.goto('/', { waitUntil: 'load' });

    await test.step('verify initial dark mode state', async () => {
      const initialClasses = await page.evaluate(() => document.documentElement.className);
      expect(initialClasses).toContain('dark');
    });

    await test.step('click toggle and assert synchronous class update', async () => {
      // ThemeToggle renders: <Button title="Switch to light mode"> when in dark mode
      // Located in Layout.tsx (both desktop header and mobile drawer)
      const themeToggle = page.locator('button[title="Switch to light mode"]').first();
      await expect(themeToggle).toBeVisible();

      await themeToggle.click();

      // useLayoutEffect fires synchronously within React's commit phase — before paint.
      // By the time this evaluate() runs, the class must already be updated.
      const classesAfterToggle = await page.evaluate(() => document.documentElement.className);
      expect(classesAfterToggle).toContain('light');
      expect(classesAfterToggle).not.toContain('dark');
    });
  });
});

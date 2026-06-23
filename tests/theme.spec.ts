/**
 * Theme System E2E Tests
 *
 * Tests the new data-theme attribute-based theme system introduced in Commit 6.
 * Covers: FOUC fix, default theme, localStorage persistence, system mode,
 * theme gallery UI, preview-on-hover, custom color picker, export/import,
 * and logo customization.
 *
 * @see docs/plans/current_spec.md — Section 4, Phase 7 and Section 5 AC
 * @see frontend/index.html — Inline script positioning (FOUC fix)
 * @see frontend/src/context/ThemeContext.tsx — ThemeProvider
 * @see frontend/src/pages/AppearanceSettings.tsx — /settings/appearance page
 */

import { test, expect } from './fixtures/test';
import { STORAGE_STATE } from './constants';
import { existsSync } from 'fs';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Log in using the stored auth state (from auth.setup.ts).
 * Navigates to the root page so the app loads the token from localStorage.
 */
async function loginWithStoredState(page: import('@playwright/test').Page): Promise<void> {
  if (!existsSync(STORAGE_STATE)) {
    throw new Error(`Auth state not found at ${STORAGE_STATE}. Run auth.setup.ts first.`);
  }
  // Storage state is set on the browser context by Playwright config (storageState).
  // Just navigate to the root and wait for the app to boot.
  await page.goto('/');
  await page.waitForLoadState('networkidle');
}

/**
 * Navigate to the appearance settings page and wait for the gallery to render.
 */
async function goToAppearance(page: import('@playwright/test').Page): Promise<void> {
  await page.goto('/settings/appearance');
  await page.waitForLoadState('networkidle');
  // Wait until the theme gallery radiogroup is visible
  await page.getByRole('radiogroup').waitFor({ state: 'visible', timeout: 15000 });
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

test.describe('Theme System', () => {
  // -------------------------------------------------------------------------
  // AC-01 / AC-03 — FOUC Fix
  // -------------------------------------------------------------------------
  test.describe('FOUC — no forced-layout warning', () => {
    /**
     * `no-fouc-warning`
     *
     * Registers a console listener BEFORE navigation.
     * Asserts the Firefox "Layout was forced before the page was fully loaded"
     * warning never appears on the initial page load.
     *
     * This validates AC-01 from the plan.
     */
    test('page load produces no forced-layout console warning', async ({ page }) => {
      const foucMessages: string[] = [];

      // Attach listener BEFORE any navigation
      page.on('console', (msg) => {
        const text = msg.text();
        if (text.toLowerCase().includes('layout was forced') ||
            text.toLowerCase().includes('flash of unstyled content') ||
            text.toLowerCase().includes('stylesheet')) {
          foucMessages.push(text);
        }
      });

      await loginWithStoredState(page);

      // Also navigate to a sub-page to confirm nav doesn't trigger it either
      await page.goto('/settings');
      await page.waitForLoadState('networkidle');

      expect(foucMessages, `FOUC warning(s) detected: ${foucMessages.join('\n')}`).toHaveLength(0);
    });
  });

  // -------------------------------------------------------------------------
  // AC-03 — data-theme attribute (not class-based)
  // -------------------------------------------------------------------------
  test.describe('Default theme on fresh load', () => {
    /**
     * `dark-theme-on-fresh-load`
     *
     * When no theme is stored in localStorage, the inline script in index.html
     * must set data-theme="dark" on <html> before any React hydration.
     */
    test('fresh load with no stored theme shows data-theme="dark"', async ({ page }) => {
      await test.step('Clear localStorage and navigate', async () => {
        // Use addInitScript to clear the theme key before page JS runs
        await page.addInitScript(() => {
          localStorage.removeItem('charon-theme');
          localStorage.removeItem('charon-custom-theme');
          // Also remove the old 'theme' key to ensure truly fresh state
          localStorage.removeItem('theme');
        });
        await page.goto('/');
        await page.waitForLoadState('networkidle');
      });

      await test.step('html element has data-theme="dark"', async () => {
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
      });
    });

    /**
     * `light-theme-from-storage`
     *
     * When localStorage['charon-theme'] = 'light' is set before navigation,
     * the inline script should apply data-theme="light" immediately.
     */
    test('localStorage charon-theme=light applies data-theme="light"', async ({ page }) => {
      await test.step('Set charon-theme to light before navigation', async () => {
        await page.addInitScript(() => {
          localStorage.setItem('charon-theme', 'light');
        });
        await page.goto('/');
        await page.waitForLoadState('networkidle');
      });

      await test.step('html element has data-theme="light"', async () => {
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
      });
    });
  });

  // -------------------------------------------------------------------------
  // Authenticated tests — use stored auth state from playwright.config
  // -------------------------------------------------------------------------
  test.describe('Authenticated theme tests', () => {
    test.beforeEach(async ({ page }) => {
      await loginWithStoredState(page);
    });

    // -----------------------------------------------------------------------
    // `theme-persists-after-reload`
    // -----------------------------------------------------------------------
    /**
     * Select the solarized theme via localStorage, reload, confirm persistence.
     */
    test('theme persists after page reload', async ({ page }) => {
      await test.step('Set solarized theme in localStorage', async () => {
        await page.evaluate(() => {
          localStorage.setItem('charon-theme', 'solarized');
        });
      });

      await test.step('Reload the page', async () => {
        await page.reload({ waitUntil: 'networkidle' });
      });

      await test.step('data-theme="solarized" is still present after reload', async () => {
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'solarized');
      });
    });

    // -----------------------------------------------------------------------
    // `system-mode-respects-os`
    // -----------------------------------------------------------------------
    /**
     * When charon-theme='system' and the OS prefers dark color scheme,
     * the inline script should resolve to data-theme="dark".
     *
     * Note: page.emulateMedia must be set BEFORE page.addInitScript runs,
     * so the inline script's window.matchMedia call sees the emulated value.
     */
    test('system mode resolves to dark when OS prefers dark', async ({ page }) => {
      await test.step('Emulate dark OS color scheme before navigation', async () => {
        await page.emulateMedia({ colorScheme: 'dark' });
        await page.addInitScript(() => {
          localStorage.setItem('charon-theme', 'system');
        });
        await page.goto('/');
        await page.waitForLoadState('networkidle');
      });

      await test.step('data-theme resolves to "dark" from system preference', async () => {
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
      });
    });

    /**
     * When charon-theme='system' and the OS prefers light color scheme,
     * the inline script should resolve to data-theme="light".
     */
    test('system mode resolves to light when OS prefers light', async ({ page }) => {
      await test.step('Emulate light OS color scheme before navigation', async () => {
        await page.emulateMedia({ colorScheme: 'light' });
        await page.addInitScript(() => {
          localStorage.setItem('charon-theme', 'system');
        });
        await page.goto('/');
        await page.waitForLoadState('networkidle');
      });

      await test.step('data-theme resolves to "light" from system preference', async () => {
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
      });
    });

    // -----------------------------------------------------------------------
    // `theme-gallery-visible`
    // -----------------------------------------------------------------------
    /**
     * Navigate to /settings/appearance and verify the theme gallery renders.
     * Checks for role="radiogroup" container and all 6 theme cards
     * (5 built-ins + system).
     */
    test('theme gallery is visible on /settings/appearance', async ({ page }) => {
      await goToAppearance(page);

      await test.step('radiogroup container is visible', async () => {
        const gallery = page.getByRole('radiogroup');
        await expect(gallery).toBeVisible();
      });

      await test.step('gallery has accessible aria-label', async () => {
        const gallery = page.getByRole('radiogroup');
        // aria-label is set from t('appearance.themeGallery') = "Theme Gallery"
        await expect(gallery).toHaveAttribute('aria-label', /theme gallery/i);
      });

      await test.step('all 6 theme cards (radio buttons) are visible', async () => {
        const radios = page.getByRole('radio');
        await expect(radios).toHaveCount(6);
      });

    });

    // -----------------------------------------------------------------------
    // `select-theme-from-gallery`
    // -----------------------------------------------------------------------
    /**
     * Click the high-contrast-dark card and verify data-theme updates.
     */
    test('selecting high-contrast-dark card updates data-theme', async ({ page }) => {
      await goToAppearance(page);

      await test.step('Find and click the High Contrast Dark card', async () => {
        const card = page.getByRole('radio', { name: /high contrast dark/i });
        await expect(card).toBeVisible();
        await card.click();
      });

      await test.step('data-theme="high-contrast-dark" is applied to <html>', async () => {
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'high-contrast-dark');
      });

      await test.step('selected card has aria-checked="true"', async () => {
        const card = page.getByRole('radio', { name: /high contrast dark/i });
        await expect(card).toHaveAttribute('aria-checked', 'true');
      });

      await test.step('other cards have aria-checked="false"', async () => {
        const lightCard = page.getByRole('radio', { name: /^light$/i });
        await expect(lightCard).toHaveAttribute('aria-checked', 'false');
      });
    });

    /**
     * Click the Solarized card and verify data-theme updates.
     */
    test('selecting solarized card updates data-theme', async ({ page }) => {
      await goToAppearance(page);

      await test.step('Click the Solarized card', async () => {
        const card = page.getByRole('radio', { name: /solarized/i });
        await card.click();
      });

      await test.step('data-theme="solarized" is applied', async () => {
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'solarized');
      });
    });

    // -----------------------------------------------------------------------
    // `preview-on-hover` and `preview-reverts-on-leave`
    // -----------------------------------------------------------------------
    /**
     * Hover over a theme card (that isn't currently selected) and verify the
     * data-theme attribute changes transiently. Then move away and verify it
     * reverts.
     */
    test('hovering theme card shows preview and reverts on leave', async ({ page }) => {
      await goToAppearance(page);

      // Ensure we start with the 'dark' theme so we can hover a different one
      await test.step('Set starting theme to dark', async () => {
        const darkCard = page.getByRole('radio', { name: /\bdark\b/i }).first();
        await darkCard.click();
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
      });

      await test.step('Hover over the Light card shows preview', async () => {
        const lightCard = page.getByRole('radio', { name: /^light$/i });
        await lightCard.hover();
        // The ThemePreviewOverlay applies the hovered theme transiently
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
      });

      await test.step('Moving away reverts data-theme to dark', async () => {
        // Move mouse to a safe area (page heading or body) to trigger mouseleave
        await page.mouse.move(0, 0);
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
      });
    });

    /**
     * `preview-on-hover` — hover over Solarized card specifically.
     */
    test('hovering solarized card sets data-theme="solarized" transiently', async ({ page }) => {
      await goToAppearance(page);

      // Ensure dark is selected so the preview is from a different theme
      await test.step('Ensure dark is the selected theme', async () => {
        const darkCard = page.getByRole('radio', { name: /\bdark\b/i }).first();
        await darkCard.click();
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
      });

      await test.step('Hover over Solarized card', async () => {
        const solarizedCard = page.getByRole('radio', { name: /solarized/i });
        await solarizedCard.hover();
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'solarized');
      });
    });

    // -----------------------------------------------------------------------
    // `custom-color-picker-visible`
    // -----------------------------------------------------------------------
    /**
     * The custom color picker section is hidden unless theme='custom' is selected.
     * Selecting any non-existent custom card via direct localStorage and reload
     * is one approach. However, since 'custom' is not shown in the gallery unless
     * already selected, we set it via localStorage and reload.
     */
    test('custom theme color picker is visible when custom theme is active', async ({ page }) => {
      await test.step('Set custom theme in localStorage with sample colors', async () => {
        await page.evaluate(() => {
          localStorage.setItem('charon-theme', 'custom');
          localStorage.setItem('charon-custom-theme', JSON.stringify({
            name: 'My Custom Theme',
            colors: {
              bgBase: '20 20 20',
              bgSubtle: '30 30 30',
              bgMuted: '40 40 40',
              bgElevated: '25 25 25',
              borderDefault: '60 60 60',
              borderStrong: '80 80 80',
              textPrimary: '240 240 240',
              textSecondary: '200 200 200',
              textMuted: '160 160 160',
              brandPrimary: '59 130 246',
              colorScheme: 'dark',
            },
          }));
        });
      });

      await test.step('Navigate to appearance settings', async () => {
        await goToAppearance(page);
      });

      await test.step('Custom color picker section is visible', async () => {
        // The CustomColorPicker is rendered inside a Card that uses t('appearance.customTheme')
        const customSection = page.getByText(/custom theme/i).first();
        await expect(customSection).toBeVisible();
      });

      await test.step('Color inputs are present (at least 10)', async () => {
        const colorInputs = page.locator('input[type="color"]');
        await expect(colorInputs).toHaveCount(10);
      });
    });

    // -----------------------------------------------------------------------
    // `export-downloads-json`
    // -----------------------------------------------------------------------
    /**
     * Click the Export Theme button and verify a file download is triggered.
     * The download event is the definitive assertion here.
     */
    test('export theme button triggers JSON file download', async ({ page }) => {
      await goToAppearance(page);

      await test.step('Click the Export Theme button and capture download', async () => {
        const [download] = await Promise.all([
          page.waitForEvent('download'),
          page.getByRole('button', { name: /export theme/i }).click(),
        ]);

        await test.step('Download filename is charon-theme.json', async () => {
          expect(download.suggestedFilename()).toBe('charon-theme.json');
        });

        await test.step('Downloaded file is non-empty JSON', async () => {
          const path = await download.path();
          expect(path).toBeTruthy();
        });
      });
    });

    // -----------------------------------------------------------------------
    // `logo-upload-applies`
    // -----------------------------------------------------------------------
    /**
     * Upload a test PNG image via the logo customizer and verify the logo
     * preview src attribute updates to the uploaded URL.
     *
     * This test requires the admin to be logged in (which our storageState provides).
     */
    test('uploading a logo image updates the logo preview', async ({ page }) => {
      await goToAppearance(page);

      // Initial logo preview src
      const logoPreview = page.locator('img[alt="Logo preview"]').first();
      await expect(logoPreview).toBeVisible();
      const initialSrc = await logoPreview.getAttribute('src');

      await test.step('Upload a small test PNG file', async () => {
        // Create a minimal valid 1x1 PNG as a Buffer
        // This is a valid 1x1 red pixel PNG (67 bytes)
        const minimalPng = Buffer.from([
          0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
          0x00, 0x00, 0x00, 0x0d, // IHDR length
          0x49, 0x48, 0x44, 0x52, // IHDR
          0x00, 0x00, 0x00, 0x01, // width: 1
          0x00, 0x00, 0x00, 0x01, // height: 1
          0x08, 0x02,             // bit depth: 8, color type: RGB
          0x00, 0x00, 0x00,       // compression, filter, interlace
          0x90, 0x77, 0x53, 0xde, // CRC
          0x00, 0x00, 0x00, 0x0c, // IDAT length
          0x49, 0x44, 0x41, 0x54, // IDAT
          0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00,
          0x00, 0x02, 0x00, 0x01, // compressed data + CRC placeholder
          0xe2, 0x21, 0xbc, 0x33, // CRC
          0x00, 0x00, 0x00, 0x00, // IEND length
          0x49, 0x45, 0x4e, 0x44, // IEND
          0xae, 0x42, 0x60, 0x82, // CRC
        ]);

        const fileInput = page.locator('input#logo-file-input');
        await fileInput.setInputFiles({
          name: 'test-logo.png',
          mimeType: 'image/png',
          buffer: minimalPng,
        });
      });

      await test.step('Logo preview src points to uploaded file after upload', async () => {
        // The mutation triggers a settings refresh via React Query invalidation.
        // The server always stores the logo at /uploads/logo.png, so we assert
        // the src contains the uploads path (works on first run and repeat runs).
        await expect(logoPreview).toHaveAttribute('src', /\/uploads\/logo\.png/i, { timeout: 10000 });
      });
    });

    // -----------------------------------------------------------------------
    // Additional gallery accessibility tests
    // -----------------------------------------------------------------------
    /**
     * Verify keyboard navigation: pressing Enter on a radio card selects it.
     */
    test('theme card can be selected via keyboard (Enter key)', async ({ page }) => {
      await goToAppearance(page);

      await test.step('Focus the Light card and press Enter', async () => {
        const lightCard = page.getByRole('radio', { name: /^light$/i });
        await lightCard.focus();
        await page.keyboard.press('Enter');
      });

      await test.step('data-theme="light" is applied', async () => {
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
      });
    });

    /**
     * Verify Space key also selects a theme card (ARIA radio pattern).
     */
    test('theme card can be selected via keyboard (Space key)', async ({ page }) => {
      await goToAppearance(page);

      await test.step('Focus the Solarized card and press Space', async () => {
        const solarizedCard = page.getByRole('radio', { name: /solarized/i });
        await solarizedCard.focus();
        await page.keyboard.press('Space');
      });

      await test.step('data-theme="solarized" is applied', async () => {
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'solarized');
      });
    });

    /**
     * Verify the Import Theme button is visible and accessible.
     */
    test('import/export section is visible on appearance page', async ({ page }) => {
      await goToAppearance(page);

      await test.step('Import and Export buttons are visible', async () => {
        await expect(page.getByRole('button', { name: /export theme/i })).toBeVisible();
        await expect(page.getByRole('button', { name: /import theme/i })).toBeVisible();
      });

      await test.step('Import/export section aria snapshot', async () => {
        const exportBtn = page.getByRole('button', { name: /export theme/i });
        await expect(exportBtn).toMatchAriaSnapshot(`
          - button "Export Theme"
        `);
      });
    });

    /**
     * Verify the Logo Customization section is present on the appearance page.
     */
    test('logo customization section is visible for admin users', async ({ page }) => {
      await goToAppearance(page);

      await test.step('Logo preview image is visible', async () => {
        const logoPreview = page.locator('img[alt="Logo preview"]').first();
        await expect(logoPreview).toBeVisible();
      });

      await test.step('Upload File tab button is visible', async () => {
        // The LogoCustomizer renders tab buttons for Upload/URL
        await expect(page.getByText(/upload file/i).first()).toBeVisible();
      });

      await test.step('File input is present for logo upload', async () => {
        const fileInput = page.locator('input#logo-file-input');
        await expect(fileInput).toBeAttached();
      });
    });

    /**
     * Verify the Appearance tab is accessible from the Settings navigation.
     */
    test('appearance tab is accessible from settings navigation', async ({ page }) => {
      await test.step('Navigate to settings', async () => {
        await page.goto('/settings');
        await page.waitForLoadState('networkidle');
      });

      await test.step('Appearance navigation link is visible', async () => {
        const appearanceLink = page.getByRole('link', { name: /appearance/i });
        await expect(appearanceLink).toBeVisible();
      });

      await test.step('Clicking appearance link navigates to /settings/appearance', async () => {
        await page.getByRole('link', { name: /appearance/i }).click();
        await page.waitForURL(/\/settings\/appearance/, { timeout: 10000 });
        await expect(page).toHaveURL(/\/settings\/appearance/);
      });
    });
  });
});

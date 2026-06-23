/**
 * Theme System — Banner Upload & Named User Themes E2E Tests
 *
 * Tests two features added to the Appearance settings page:
 * 1. Banner Customization — upload, preview, and delete a custom sidebar banner
 * 2. Named User Themes  — create, apply, edit colors, and delete persisted themes
 *
 * @see frontend/src/components/theme/BannerCustomizer.tsx
 * @see frontend/src/components/theme/UserThemeManager.tsx
 * @see frontend/src/pages/AppearanceSettings.tsx
 */

import { test, expect } from './fixtures/test';
import { existsSync } from 'fs';
import { STORAGE_STATE } from './constants';

// ---------------------------------------------------------------------------
// Shared test data
// ---------------------------------------------------------------------------

/**
 * Default colors for a new custom theme (dark base).
 * The backend stores `colors` as a JSON string — always JSON.stringify before
 * passing to page.request.post / page.request.put.
 */
const DEFAULT_THEME_COLORS_JSON = JSON.stringify({
  bgBase: '15 23 42',
  bgSubtle: '30 41 59',
  bgMuted: '51 65 85',
  bgElevated: '30 41 59',
  borderDefault: '51 65 85',
  borderStrong: '71 85 105',
  textPrimary: '248 250 252',
  textSecondary: '203 213 225',
  textMuted: '148 163 184',
  brandPrimary: '59 130 246',
  colorScheme: 'dark',
});

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Navigate to the root page using the stored auth state.
 * The storageState is injected by the playwright.config project definition,
 * so calling goto('/') is sufficient to land in an authenticated session.
 */
async function loginWithStoredState(page: import('@playwright/test').Page): Promise<void> {
  if (!existsSync(STORAGE_STATE)) {
    throw new Error(`Auth state not found at ${STORAGE_STATE}. Run auth.setup.ts first.`);
  }
  await page.goto('/');
  await page.waitForLoadState('networkidle');
}

/**
 * Navigate to /settings/appearance and wait until the theme gallery is visible.
 * This ensures the whole page (including React Query data) has loaded before
 * individual tests begin interacting with sections.
 */
async function goToAppearance(page: import('@playwright/test').Page): Promise<void> {
  await page.goto('/settings/appearance');
  await page.waitForLoadState('networkidle');
  await page.getByRole('radiogroup').waitFor({ state: 'visible', timeout: 15000 });
}

/**
 * A minimal valid 1×1 PNG (red pixel, 67 bytes).
 * Used wherever a real image file is required by the upload input.
 */
const MINIMAL_PNG = Buffer.from([
  0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
  0x00, 0x00, 0x00, 0x0d,                           // IHDR chunk length
  0x49, 0x48, 0x44, 0x52,                           // "IHDR"
  0x00, 0x00, 0x00, 0x01,                           // width: 1
  0x00, 0x00, 0x00, 0x01,                           // height: 1
  0x08, 0x02,                                       // bit depth: 8, RGB
  0x00, 0x00, 0x00,                                 // compression / filter / interlace
  0x90, 0x77, 0x53, 0xde,                           // IHDR CRC
  0x00, 0x00, 0x00, 0x0c,                           // IDAT chunk length
  0x49, 0x44, 0x41, 0x54,                           // "IDAT"
  0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00,
  0x00, 0x02, 0x00, 0x01,
  0xe2, 0x21, 0xbc, 0x33,                           // IDAT CRC
  0x00, 0x00, 0x00, 0x00,                           // IEND chunk length
  0x49, 0x45, 0x4e, 0x44,                           // "IEND"
  0xae, 0x42, 0x60, 0x82,                           // IEND CRC
]);

// ---------------------------------------------------------------------------
// Test suites
// ---------------------------------------------------------------------------

test.describe('Banner Customization', () => {
  test.beforeEach(async ({ page }) => {
    await loginWithStoredState(page);
    // Clean up any existing banner before each test to guarantee a known state
    await page.request.delete('/api/v1/settings/banner').catch(() => {
      // Ignore 404 — no banner to delete
    });
    await goToAppearance(page);
  });

  // -------------------------------------------------------------------------
  // Banner upload section is visible
  // -------------------------------------------------------------------------
  test('banner customization section is visible on appearance page', async ({ page }) => {
    await test.step('Section heading "Sidebar Banner" is visible', async () => {
      await expect(page.getByText(/sidebar banner/i).first()).toBeVisible();
    });

    await test.step('Upload File tab button is visible', async () => {
      // BannerCustomizer renders two tab buttons: "Upload File" and "Enter URL"
      // The t('appearance.bannerUploadTab') key resolves to "Upload File"
      const uploadTabs = page.getByRole('button', { name: /upload file/i });
      await expect(uploadTabs.first()).toBeVisible();
    });

    await test.step('File input for banner upload is attached to the DOM', async () => {
      const fileInput = page.locator('input#banner-file-input');
      await expect(fileInput).toBeAttached();
    });

    await test.step('Reset to Default button is present', async () => {
      // t('appearance.bannerResetButton') = "Reset to Default"
      const resetBtn = page.getByRole('button', { name: /reset to default/i });
      // There may be multiple (logo and banner sections share the label); first is logo
      await expect(resetBtn.first()).toBeVisible();
    });
  });

  // -------------------------------------------------------------------------
  // Upload a banner image — verify preview appears
  // -------------------------------------------------------------------------
  test('uploading a PNG file shows the banner preview', async ({ page }) => {
    await test.step('Set file on the hidden banner file input', async () => {
      const fileInput = page.locator('input#banner-file-input');
      await fileInput.setInputFiles({
        name: 'test-banner.png',
        mimeType: 'image/png',
        buffer: MINIMAL_PNG,
      });
    });

    await test.step('Banner preview image appears after upload', async () => {
      // After a successful upload the settings query is invalidated and the
      // BannerCustomizer renders an <img alt="Banner preview"> once the URL is set.
      const bannerPreview = page.locator('img[alt="Banner preview"]');
      await expect(bannerPreview).toBeVisible({ timeout: 10000 });
    });

    await test.step('Banner preview src points to the uploaded path', async () => {
      const bannerPreview = page.locator('img[alt="Banner preview"]');
      await expect(bannerPreview).toHaveAttribute('src', /\/uploads\/banner\./i, { timeout: 10000 });
    });
  });

  // -------------------------------------------------------------------------
  // After upload the banner is visible in the sidebar
  // -------------------------------------------------------------------------
  test('uploaded banner appears in the sidebar header', async ({ page }) => {
    await test.step('Upload banner file', async () => {
      const fileInput = page.locator('input#banner-file-input');
      await fileInput.setInputFiles({
        name: 'test-banner.png',
        mimeType: 'image/png',
        buffer: MINIMAL_PNG,
      });
      // Wait for the preview to confirm the upload completed
      await page.locator('img[alt="Banner preview"]').waitFor({ state: 'visible', timeout: 10000 });
    });

    await test.step('Navigate away and back to verify the banner persists in the sidebar', async () => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');
    });

    await test.step('Sidebar contains the uploaded banner image', async () => {
      // The Layout sidebar uses <img alt="Charon"> when a customBannerUrl is set.
      // We locate by the src pointing to the uploads path.
      const sidebarBanner = page.locator('aside img[alt="Charon"]');
      await expect(sidebarBanner).toHaveAttribute('src', /\/uploads\/banner\./i, { timeout: 10000 });
    });
  });

  // -------------------------------------------------------------------------
  // Delete / reset the banner
  // -------------------------------------------------------------------------
  test('deleting the banner removes it from the preview and sidebar', async ({ page }) => {
    await test.step('Upload a banner first', async () => {
      const fileInput = page.locator('input#banner-file-input');
      await fileInput.setInputFiles({
        name: 'test-banner.png',
        mimeType: 'image/png',
        buffer: MINIMAL_PNG,
      });
      await page.locator('img[alt="Banner preview"]').waitFor({ state: 'visible', timeout: 10000 });
    });

    await test.step('Click the Reset to Default button in the banner section', async () => {
      // The Banner section is the last Card on the page; locate its reset button
      // by its accessible name (t('appearance.bannerResetButton') = "Reset to Default")
      // We use the last match because the Logo section also has a "Reset to Default" button.
      const resetButtons = page.getByRole('button', { name: /reset to default/i });
      const count = await resetButtons.count();
      // Click the last "Reset to Default" button which belongs to the banner section
      await resetButtons.nth(count - 1).click();
    });

    await test.step('Banner preview image is no longer visible', async () => {
      const bannerPreview = page.locator('img[alt="Banner preview"]');
      await expect(bannerPreview).not.toBeVisible({ timeout: 10000 });
    });
  });

  // -------------------------------------------------------------------------
  // Enter URL tab — save a banner via HTTPS URL
  // -------------------------------------------------------------------------
  test('saving a banner via HTTPS URL updates the preview', async ({ page }) => {
    await test.step('Switch to the Enter URL tab in the banner section', async () => {
      // Both Logo and Banner sections have "Enter URL" tab buttons.
      // The banner section is rendered last, so take the last one.
      const urlTabButtons = page.getByRole('button', { name: /enter url/i });
      const count = await urlTabButtons.count();
      await urlTabButtons.nth(count - 1).click();
    });

    await test.step('Fill in a valid HTTPS URL and save', async () => {
      const urlInput = page.locator('input#banner-url-input');
      await urlInput.fill('https://example.com/banner.png');

      // Wait for the API response after clicking Save Banner
      const [response] = await Promise.all([
        page.waitForResponse((r) => r.url().includes('/api/v1/settings') && r.request().method() !== 'GET'),
        page.getByRole('button', { name: /save banner/i }).click(),
      ]);
      expect(response.status()).toBeLessThan(400);
    });

    await test.step('Banner preview shows the saved URL', async () => {
      const bannerPreview = page.locator('img[alt="Banner preview"]');
      await expect(bannerPreview).toBeVisible({ timeout: 10000 });
      await expect(bannerPreview).toHaveAttribute('src', 'https://example.com/banner.png');
    });
  });

  // -------------------------------------------------------------------------
  // Accessibility snapshot of the banner section
  // -------------------------------------------------------------------------
  test('banner customization section has accessible structure', async ({ page }) => {
    await test.step('Upload File tab button has correct accessible name', async () => {
      const uploadTabButtons = page.getByRole('button', { name: /upload file/i });
      await expect(uploadTabButtons.first()).toMatchAriaSnapshot(`
        - button "Upload File"
      `);
    });
  });
});

// ---------------------------------------------------------------------------

test.describe('Named User Themes', () => {
  const TEST_THEME_NAME = `E2E Test Theme ${Date.now()}`;

  test.beforeEach(async ({ page }) => {
    await loginWithStoredState(page);
    await goToAppearance(page);
  });

  // -------------------------------------------------------------------------
  // "Your Themes" section is present
  // -------------------------------------------------------------------------
  test('user themes section is visible on appearance page', async ({ page }) => {
    await test.step('"Your Themes" heading is visible', async () => {
      // t('appearance.userThemes') = "Your Themes"
      await expect(page.getByText(/your themes/i).first()).toBeVisible();
    });

    await test.step('"Create New Theme" button is present', async () => {
      // t('appearance.createNewTheme') = "Create New Theme"
      await expect(page.getByRole('button', { name: /create new theme/i })).toBeVisible();
    });
  });

  // -------------------------------------------------------------------------
  // Create a named theme
  // -------------------------------------------------------------------------
  test('can create a new named theme and it appears in the list', async ({ page }) => {
    await test.step('Open the Create New Theme dialog', async () => {
      await page.getByRole('button', { name: /create new theme/i }).click();
      // Dialog should appear with its heading
      await expect(page.getByRole('dialog')).toBeVisible({ timeout: 5000 });
    });

    await test.step('Fill in the theme name', async () => {
      const nameInput = page.getByLabel(/theme name/i);
      await nameInput.fill(TEST_THEME_NAME);
    });

    await test.step('Save the theme and wait for API response', async () => {
      const [response] = await Promise.all([
        page.waitForResponse((r) => r.url().includes('/api/v1/themes') && r.request().method() === 'POST'),
        page.getByRole('button', { name: /save theme/i }).click(),
      ]);
      expect(response.status()).toBeLessThan(400);
    });

    await test.step('New theme card appears in the theme list', async () => {
      await expect(page.getByText(TEST_THEME_NAME)).toBeVisible({ timeout: 10000 });
    });

    // Cleanup: delete the theme via API so subsequent runs start clean
    await page.request
      .get('/api/v1/themes')
      .then(async (r) => {
        if (!r.ok()) return;
        type ThemeEntry = { id: string; name: string };
        const themes = (await r.json()) as ThemeEntry[];
        const created = themes.find((t) => t.name === TEST_THEME_NAME);
        if (created) {
          await page.request.delete(`/api/v1/themes/${created.id}`);
        }
      })
      .catch(() => {});
  });

  // -------------------------------------------------------------------------
  // Apply a theme — becomes active
  // -------------------------------------------------------------------------
  test('activating a user theme marks it as active', async ({ page }) => {
    // Create the theme via API first so the test is independent of the create UI.
    // NOTE: the backend expects `colors` as a JSON string, not a nested object.
    const uniqueName = `E2E Activate ${Date.now()}`;
    const createResp = await page.request.post('/api/v1/themes', {
      data: {
        name: uniqueName,
        colors: DEFAULT_THEME_COLORS_JSON,
      },
    });
    expect(createResp.status()).toBeLessThan(400);
    type CreatedTheme = { id: string };
    const created = (await createResp.json()) as CreatedTheme;

    try {
      // Reload so the theme list shows the newly created theme
      await goToAppearance(page);

      await test.step('Activate button is visible for the theme', async () => {
        // t('appearance.activateTheme') = "Activate"
        const activateBtn = page.getByRole('button', { name: new RegExp(`activate\\s+${uniqueName}`, 'i') });
        await expect(activateBtn).toBeVisible({ timeout: 10000 });
      });

      await test.step('Click Activate and verify the theme becomes active', async () => {
        const activateBtn = page.getByRole('button', { name: new RegExp(`activate\\s+${uniqueName}`, 'i') });
        await activateBtn.click();

        // The card should now show an "Active" badge (aria-label="Active theme")
        await expect(page.getByText(/^active$/i).first()).toBeVisible({ timeout: 5000 });
      });

      await test.step('data-theme attribute on html is set to "custom" for user themes', async () => {
        // The ThemeContext always maps any user:* ThemeId to data-theme="custom"
        // on the <html> element — it never exposes the internal user:<uuid> in the DOM.
        // (See frontend/src/context/ThemeContext.tsx — resolveDataTheme returns 'custom'
        // for isUserThemeId() inputs.)
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'custom', { timeout: 5000 });
      });
    } finally {
      await page.request.delete(`/api/v1/themes/${created.id}`).catch(() => {});
    }
  });

  // -------------------------------------------------------------------------
  // Edit a theme's colors
  // -------------------------------------------------------------------------
  test('editing a user theme persists the updated name', async ({ page }) => {
    const uniqueName = `E2E Edit ${Date.now()}`;
    const createResp = await page.request.post('/api/v1/themes', {
      data: {
        name: uniqueName,
        colors: DEFAULT_THEME_COLORS_JSON,
      },
    });
    expect(createResp.status()).toBeLessThan(400);
    type CreatedTheme = { id: string };
    const created = (await createResp.json()) as CreatedTheme;

    try {
      await goToAppearance(page);

      await test.step('Click the Edit (pencil) button for the theme', async () => {
        // aria-label is set to t('appearance.editTheme') + ' ' + theme.name
        const editBtn = page.getByRole('button', { name: new RegExp(`edit theme\\s+${uniqueName}`, 'i') });
        await expect(editBtn).toBeVisible({ timeout: 10000 });
        await editBtn.click();
        await expect(page.getByRole('dialog')).toBeVisible({ timeout: 5000 });
      });

      const updatedName = `${uniqueName} UPDATED`;

      await test.step('Change the theme name in the edit dialog', async () => {
        const nameInput = page.getByLabel(/theme name/i);
        await nameInput.clear();
        await nameInput.fill(updatedName);
      });

      await test.step('Save the updated theme and wait for API response', async () => {
        const [response] = await Promise.all([
          page.waitForResponse((r) => r.url().includes('/api/v1/themes') && r.request().method() === 'PUT'),
          page.getByRole('button', { name: /save theme/i }).click(),
        ]);
        expect(response.status()).toBeLessThan(400);
      });

      await test.step('Updated theme name appears in the list', async () => {
        await expect(page.getByText(updatedName)).toBeVisible({ timeout: 10000 });
      });

      await test.step('Original name is no longer in the list', async () => {
        await expect(page.getByText(uniqueName, { exact: true })).not.toBeVisible();
      });
    } finally {
      await page.request.delete(`/api/v1/themes/${created.id}`).catch(() => {});
    }
  });

  // -------------------------------------------------------------------------
  // Delete a theme
  // -------------------------------------------------------------------------
  test('deleting a user theme removes it from the list', async ({ page }) => {
    const uniqueName = `E2E Delete ${Date.now()}`;
    const createResp = await page.request.post('/api/v1/themes', {
      data: {
        name: uniqueName,
        colors: DEFAULT_THEME_COLORS_JSON,
      },
    });
    expect(createResp.status()).toBeLessThan(400);
    type CreatedTheme = { id: string };
    const created = (await createResp.json()) as CreatedTheme;

    await goToAppearance(page);

    await test.step('Theme card is visible before deletion', async () => {
      await expect(page.getByText(uniqueName)).toBeVisible({ timeout: 10000 });
    });

    await test.step('Click the Delete (trash) button for the theme', async () => {
      // aria-label is t('appearance.deleteTheme') + ' ' + theme.name
      const deleteBtn = page.getByRole('button', { name: new RegExp(`delete theme\\s+${uniqueName}`, 'i') });
      await deleteBtn.click();
      // Confirmation dialog opens
      await expect(page.getByRole('alertdialog')).toBeVisible({ timeout: 5000 });
    });

    await test.step('Confirm deletion in the alertdialog', async () => {
      const [response] = await Promise.all([
        page.waitForResponse((r) => r.url().includes(`/api/v1/themes/${created.id}`) && r.request().method() === 'DELETE'),
        // The confirm button text is t('appearance.deleteTheme') = "Delete Theme"
        page.getByRole('alertdialog').getByRole('button', { name: /delete theme/i }).click(),
      ]);
      expect(response.status()).toBeLessThan(400);
    });

    await test.step('Deleted theme is no longer in the list', async () => {
      await expect(page.getByText(uniqueName)).not.toBeVisible({ timeout: 10000 });
    });
  });

  // -------------------------------------------------------------------------
  // Accessibility snapshot of the "Create New Theme" button
  // -------------------------------------------------------------------------
  test('create new theme button has correct accessible role and name', async ({ page }) => {
    const createBtn = page.getByRole('button', { name: /create new theme/i });
    await expect(createBtn).toMatchAriaSnapshot(`
      - button "Create New Theme"
    `);
  });
});

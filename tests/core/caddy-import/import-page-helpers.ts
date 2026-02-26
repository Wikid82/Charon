import { expect, test, type Page } from '@playwright/test';
import { loginUser, type TestUser } from '../../fixtures/auth-fixtures';

const IMPORT_PAGE_PATH = '/tasks/import/caddyfile';

export async function resetImportSession(page: Page): Promise<void> {
  try {
    if (!page.url().includes(IMPORT_PAGE_PATH)) {
      await page.goto(IMPORT_PAGE_PATH, { waitUntil: 'domcontentloaded' });
    }
  } catch {
    // Best-effort navigation only
  }

  try {
    const statusResponse = await page.request.get('/api/v1/import/status');
    if (statusResponse.ok()) {
      const statusBody = await statusResponse.json();
      if (statusBody?.has_pending) {
        await page.request.post('/api/v1/import/cancel');
      }
    }
  } catch {
    // Best-effort cleanup only
  }

  try {
    await page.goto(IMPORT_PAGE_PATH, { waitUntil: 'domcontentloaded' });
  } catch {
    // Best-effort return to import page only
  }
}

export async function ensureImportFormReady(page: Page): Promise<void> {
  const currentUrl = page.url();
  const currentPath = await page.evaluate(() => window.location.pathname).catch(() => '');
  if (currentUrl.includes('/login') || currentPath.includes('/login')) {
    throw new Error(
      `Auth state lost: import form is unavailable because the page is on login (url=${currentUrl}, path=${currentPath})`
    );
  }

  const headingByRole = page.getByRole('heading', { name: /import|caddyfile/i }).first();
  const headingLike = page
    .locator('h1, h2, [data-testid="page-title"], [aria-label*="import" i], [aria-label*="caddyfile" i]')
    .first();

  if (await headingByRole.count()) {
    await expect(headingByRole).toBeVisible();
  } else if (await headingLike.count()) {
    await expect(headingLike).toBeVisible();
  } else {
    await expect(page.locator('main, body').first()).toContainText(/import|caddyfile/i);
  }

  await expect(page.locator('textarea')).toBeVisible();
  await expect(page.getByRole('button', { name: /parse|review/i }).first()).toBeVisible();
}

async function hasLoginUiMarkers(page: Page): Promise<boolean> {
  const currentUrl = page.url();
  const currentPath = await page.evaluate(() => window.location.pathname).catch(() => '');
  if (currentUrl.includes('/login') || currentPath.includes('/login')) {
    return true;
  }

  const signInHeading = page.getByRole('heading', { name: /sign in|login/i }).first();
  const signInButton = page.getByRole('button', { name: /sign in|login/i }).first();
  const emailTextbox = page.getByRole('textbox', { name: /email/i }).first();

  const [headingVisible, buttonVisible, emailVisible] = await Promise.all([
    signInHeading.isVisible().catch(() => false),
    signInButton.isVisible().catch(() => false),
    emailTextbox.isVisible().catch(() => false),
  ]);

  return headingVisible || buttonVisible || emailVisible;
}

export async function ensureAuthenticatedImportFormReady(page: Page, adminUser?: TestUser): Promise<void> {
  const recoverIfNeeded = async (): Promise<boolean> => {
    const loginDetected = await test.step('Auth precheck: detect login redirect or sign-in controls', async () => {
      return hasLoginUiMarkers(page);
    });
    if (!loginDetected) {
      return false;
    }

    if (!adminUser) {
      throw new Error('Import auth recovery failed: login UI detected but no admin user fixture was provided.');
    }

    return test.step('Auth recovery: perform one deterministic login and return to import page', async () => {
      try {
        await loginUser(page, adminUser);
        await page.goto(IMPORT_PAGE_PATH, { waitUntil: 'domcontentloaded' });

        if (await hasLoginUiMarkers(page) && adminUser.token) {
          await test.step('Auth recovery fallback: restore fixture token and reload import page', async () => {
            await page.goto('/', { waitUntil: 'domcontentloaded' });
            await page.evaluate((token: string) => {
              localStorage.setItem('charon_auth_token', token);
            }, adminUser.token);
            await page.reload({ waitUntil: 'domcontentloaded' });
            await page.waitForLoadState('networkidle').catch(() => {});
            await page.goto(IMPORT_PAGE_PATH, { waitUntil: 'domcontentloaded' });
          });
        }

        await ensureImportFormReady(page);
        return true;
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        throw new Error(`Import auth recovery failed after one re-auth attempt: ${message}`);
      }
    });
  };

  if (await recoverIfNeeded()) {
    return;
  }

  try {
    await ensureImportFormReady(page);
  } catch (error) {
    if (await recoverIfNeeded()) {
      return;
    }

    throw error;
  }
}

export async function ensureImportUiPreconditions(page: Page, adminUser?: TestUser): Promise<void> {
  await test.step('Precondition: open Caddy import page', async () => {
    await page.goto(IMPORT_PAGE_PATH, { waitUntil: 'domcontentloaded' });
  });

  await ensureAuthenticatedImportFormReady(page, adminUser);

  await test.step('Precondition: verify import textarea is visible', async () => {
    await expect(page.locator('textarea')).toBeVisible();
  });
}

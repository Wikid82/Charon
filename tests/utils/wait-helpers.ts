/**
 * Wait Helpers - Deterministic wait utilities for flaky test prevention
 *
 * These utilities replace arbitrary `page.waitForTimeout()` calls with
 * condition-based waits that poll for specific states.
 *
 * @example
 * ```typescript
 * // Instead of:
 * await page.waitForTimeout(1000);
 *
 * // Use:
 * await waitForToast(page, 'Success');
 * await waitForLoadingComplete(page);
 * ```
 */

import { expect } from '@bgotink/playwright-coverage';
import type { Page, Locator, Response } from '@playwright/test';

/**
 * Click an element and wait for an API response atomically.
 * Prevents race condition where response completes before wait starts.
 * @param page - Playwright Page instance
 * @param clickTarget - Locator or selector string for element to click
 * @param urlPattern - URL string or RegExp to match
 * @param options - Configuration options
 * @returns The matched response
 */
export async function clickAndWaitForResponse(
  page: Page,
  clickTarget: Locator | string,
  urlPattern: string | RegExp,
  options: { status?: number; timeout?: number } = {}
): Promise<Response> {
  const { status = 200, timeout = 30000 } = options;

  const locator =
    typeof clickTarget === 'string' ? page.locator(clickTarget) : clickTarget;

  const [response] = await Promise.all([
    page.waitForResponse(
      (resp) => {
        const urlMatch =
          typeof urlPattern === 'string'
            ? resp.url().includes(urlPattern)
            : urlPattern.test(resp.url());
        return urlMatch && resp.status() === status;
      },
      { timeout }
    ),
    locator.click(),
  ]);

  return response;
}

/**
 * Options for waitForToast
 */
export interface ToastOptions {
  /** Maximum time to wait for toast (default: 10000ms) */
  timeout?: number;
  /** Toast type to match (success, error, info, warning) */
  type?: 'success' | 'error' | 'info' | 'warning';
}

/**
 * Wait for a toast notification with specific text
 * Supports both custom ToastContainer (data-testid) and react-hot-toast
 * @param page - Playwright Page instance
 * @param text - Text or RegExp to match in toast
 * @param options - Configuration options
 */
export async function waitForToast(
  page: Page,
  text: string | RegExp,
  options: ToastOptions = {}
): Promise<void> {
  const { timeout = 10000, type } = options;

  // Build selectors prioritizing our custom toast system which uses data-testid
  // This avoids matching generic [role="alert"] elements like security notices
  let selector: string;

  if (type) {
    // Type-specific toast: match data-testid exactly
    selector = `[data-testid="toast-${type}"]`;
  } else {
    // Any toast: match our custom toast container or react-hot-toast
    // Avoid matching static [role="alert"] elements by being more specific
    selector = '[data-testid^="toast-"]:not([data-testid="toast-container"])';
  }

  const toast = page.locator(selector);
  await expect(toast).toContainText(text, { timeout });
}

/**
 * Options for waitForAPIResponse
 */
export interface APIResponseOptions {
  /** Expected HTTP status code */
  status?: number;
  /** Maximum time to wait (default: 30000ms) */
  timeout?: number;
}

/**
 * Wait for a specific API response
 * @param page - Playwright Page instance
 * @param urlPattern - URL string or RegExp to match
 * @param options - Configuration options
 * @returns The matched response
 */
export async function waitForAPIResponse(
  page: Page,
  urlPattern: string | RegExp,
  options: APIResponseOptions = {}
): Promise<Response> {
  const { status, timeout = 30000 } = options;

  const responsePromise = page.waitForResponse(
    (response) => {
      const matchesURL =
        typeof urlPattern === 'string'
          ? response.url().includes(urlPattern)
          : urlPattern.test(response.url());

      const matchesStatus = status ? response.status() === status : true;

      return matchesURL && matchesStatus;
    },
    { timeout }
  );

  return await responsePromise;
}

/**
 * Options for waitForLoadingComplete
 */
export interface LoadingOptions {
  /** Maximum time to wait (default: 10000ms) */
  timeout?: number;
}

/**
 * Wait for loading spinner/indicator to disappear
 * @param page - Playwright Page instance
 * @param options - Configuration options
 */
export async function waitForLoadingComplete(
  page: Page,
  options: LoadingOptions = {}
): Promise<void> {
  const { timeout = 10000 } = options;

  // Wait for any loading indicator to disappear
  const loader = page.locator(
    '[role="progressbar"], [aria-busy="true"], .loading-spinner, .loading, .spinner, [data-loading="true"]'
  );
  await expect(loader).toHaveCount(0, { timeout });
}

/**
 * Options for waitForElementCount
 */
export interface ElementCountOptions {
  /** Maximum time to wait (default: 10000ms) */
  timeout?: number;
}

/**
 * Wait for a specific element count
 * @param locator - Playwright Locator to count
 * @param count - Expected number of elements
 * @param options - Configuration options
 */
export async function waitForElementCount(
  locator: Locator,
  count: number,
  options: ElementCountOptions = {}
): Promise<void> {
  const { timeout = 10000 } = options;
  await expect(locator).toHaveCount(count, { timeout });
}

/**
 * Options for waitForWebSocketConnection
 */
export interface WebSocketConnectionOptions {
  /** Maximum time to wait (default: 10000ms) */
  timeout?: number;
}

/**
 * Wait for WebSocket connection to be established
 * @param page - Playwright Page instance
 * @param urlPattern - URL string or RegExp to match
 * @param options - Configuration options
 */
export async function waitForWebSocketConnection(
  page: Page,
  urlPattern: string | RegExp,
  options: WebSocketConnectionOptions = {}
): Promise<void> {
  const { timeout = 10000 } = options;

  await page.waitForEvent('websocket', {
    predicate: (ws) => {
      const matchesURL =
        typeof urlPattern === 'string'
          ? ws.url().includes(urlPattern)
          : urlPattern.test(ws.url());
      return matchesURL;
    },
    timeout,
  });
}

/**
 * Options for waitForWebSocketMessage
 */
export interface WebSocketMessageOptions {
  /** Maximum time to wait (default: 10000ms) */
  timeout?: number;
}

/**
 * Wait for WebSocket message with specific content
 * @param page - Playwright Page instance
 * @param matcher - Function to match message data
 * @param options - Configuration options
 * @returns The matched message data
 */
export async function waitForWebSocketMessage(
  page: Page,
  matcher: (data: string | Buffer) => boolean,
  options: WebSocketMessageOptions = {}
): Promise<string | Buffer> {
  const { timeout = 10000 } = options;

  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      reject(new Error(`WebSocket message not received within ${timeout}ms`));
    }, timeout);

    const cleanup = () => {
      clearTimeout(timer);
    };

    page.on('websocket', (ws) => {
      ws.on('framereceived', (event) => {
        const data = event.payload;
        if (matcher(data)) {
          cleanup();
          resolve(data);
        }
      });
    });
  });
}

/**
 * Options for waitForProgressComplete
 */
export interface ProgressOptions {
  /** Maximum time to wait (default: 30000ms) */
  timeout?: number;
}

/**
 * Wait for progress bar to complete
 * @param page - Playwright Page instance
 * @param options - Configuration options
 */
export async function waitForProgressComplete(
  page: Page,
  options: ProgressOptions = {}
): Promise<void> {
  const { timeout = 30000 } = options;

  const progressBar = page.locator('[role="progressbar"]');

  // Wait for progress to reach 100% or disappear
  await page.waitForFunction(
    () => {
      const bar = document.querySelector('[role="progressbar"]');
      if (!bar) return true; // Progress bar gone = complete
      const value = bar.getAttribute('aria-valuenow');
      return value === '100';
    },
    { timeout }
  );

  // Wait for progress bar to disappear
  await expect(progressBar).toHaveCount(0, { timeout: 5000 });
}

/**
 * Options for waitForModal
 */
export interface ModalOptions {
  /** Maximum time to wait (default: 10000ms) */
  timeout?: number;
}

/**
 * Wait for modal dialog to open
 * @param page - Playwright Page instance
 * @param titleText - Text or RegExp to match in modal title
 * @param options - Configuration options
 * @returns Locator for the modal
 */
export async function waitForModal(
  page: Page,
  titleText: string | RegExp,
  options: ModalOptions = {}
): Promise<Locator> {
  const { timeout = 10000 } = options;

  // Try to find a modal dialog first, then fall back to a slide-out panel with matching heading
  const dialogModal = page.locator('[role="dialog"], .modal');
  const slideOutPanel = page.locator('h2, h3').filter({ hasText: titleText });

  // Wait for either the dialog modal or the slide-out panel heading to be visible
  try {
    await expect(dialogModal.or(slideOutPanel)).toBeVisible({ timeout });
  } catch {
    // If neither is found, throw a more helpful error
    throw new Error(
      `waitForModal: Could not find modal dialog or slide-out panel matching "${titleText}"`
    );
  }

  // If dialog modal is visible, verify its title
  if (await dialogModal.isVisible()) {
    if (titleText) {
      const titleLocator = dialogModal.locator(
        '[role="heading"], .modal-title, .dialog-title, h1, h2, h3'
      );
      await expect(titleLocator).toContainText(titleText);
    }
    return dialogModal;
  }

  // Return the parent container of the heading for slide-out panels
  return slideOutPanel.locator('..');
}

/**
 * Options for waitForDropdown
 */
export interface DropdownOptions {
  /** Maximum time to wait (default: 5000ms) */
  timeout?: number;
}

/**
 * Wait for dropdown/listbox to open
 * @param page - Playwright Page instance
 * @param triggerId - ID of the dropdown trigger element
 * @param options - Configuration options
 * @returns Locator for the listbox
 */
export async function waitForDropdown(
  page: Page,
  triggerId: string,
  options: DropdownOptions = {}
): Promise<Locator> {
  const { timeout = 5000 } = options;

  const trigger = page.locator(`#${triggerId}`);
  const expanded = await trigger.getAttribute('aria-expanded');

  if (expanded !== 'true') {
    throw new Error(`Dropdown ${triggerId} is not expanded`);
  }

  const listboxId = await trigger.getAttribute('aria-controls');
  if (!listboxId) {
    // Try finding listbox by common patterns
    const listbox = page.locator('[role="listbox"], [role="menu"]').first();
    await expect(listbox).toBeVisible({ timeout });
    return listbox;
  }

  const listbox = page.locator(`#${listboxId}`);
  await expect(listbox).toBeVisible({ timeout });

  return listbox;
}

/**
 * Options for waitForTableLoad
 */
export interface TableLoadOptions {
  /** Minimum number of rows expected (default: 0) */
  minRows?: number;
  /** Maximum time to wait (default: 10000ms) */
  timeout?: number;
}

/**
 * Wait for table to finish loading and render rows
 * @param page - Playwright Page instance
 * @param tableRole - ARIA role for the table (default: 'table')
 * @param options - Configuration options
 */
export async function waitForTableLoad(
  page: Page,
  tableRole: string = 'table',
  options: TableLoadOptions = {}
): Promise<void> {
  const { minRows = 0, timeout = 10000 } = options;

  const table = page.getByRole(tableRole as 'table');
  await expect(table).toBeVisible({ timeout });

  // Wait for loading state to clear
  await waitForLoadingComplete(page, { timeout });

  // If minimum rows specified, wait for them
  if (minRows > 0) {
    const rows = table.locator('tbody tr, [role="row"]').filter({ hasNot: page.locator('th') });
    await expect(rows).toHaveCount(minRows, { timeout });
  }
}

/**
 * Options for retryAction
 */
export interface RetryOptions {
  /** Maximum number of attempts (default: 5) */
  maxAttempts?: number;
  /** Delay between attempts in ms (default: 1000) */
  interval?: number;
  /** Maximum total time in ms (default: 30000) */
  timeout?: number;
}

/**
 * Retry an action until it succeeds or timeout
 * @param action - Async function to retry
 * @param options - Configuration options
 * @returns Result of the successful action
 */
export async function retryAction<T>(
  action: () => Promise<T>,
  options: RetryOptions = {}
): Promise<T> {
  const { maxAttempts = 5, interval = 1000, timeout = 30000 } = options;

  const startTime = Date.now();
  let lastError: Error | undefined;

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    if (Date.now() - startTime > timeout) {
      throw new Error(`Retry timeout after ${timeout}ms`);
    }

    try {
      return await action();
    } catch (error) {
      lastError = error as Error;
      if (attempt < maxAttempts) {
        await new Promise((resolve) => setTimeout(resolve, interval));
      }
    }
  }

  throw lastError || new Error('Retry failed after max attempts');
}

/**
 * Options for waitForResourceInUI
 */
export interface WaitForResourceOptions {
  /** Maximum time to wait (default: 15000ms) */
  timeout?: number;
  /** Whether to reload the page if resource not found initially (default: true) */
  reloadIfNotFound?: boolean;
  /** Delay after API call before checking UI (default: 500ms) */
  initialDelay?: number;
}

/**
 * Wait for a resource created via API to appear in the UI
 * This handles the common case where API creates a resource but UI needs time to reflect it.
 * Will attempt to find the resource, and if not found, will reload the page and retry.
 *
 * @param page - Playwright Page instance
 * @param identifier - Text or RegExp to identify the resource in UI (e.g., domain name)
 * @param options - Configuration options
 *
 * @example
 * ```typescript
 * // After creating a proxy host via API
 * const { domain } = await testData.createProxyHost(config);
 * await waitForResourceInUI(page, domain);
 * ```
 */
export async function waitForResourceInUI(
  page: Page,
  identifier: string | RegExp,
  options: WaitForResourceOptions = {}
): Promise<void> {
  const { timeout = 15000, reloadIfNotFound = true, initialDelay = 500 } = options;

  // Small initial delay to allow API response to propagate
  await page.waitForTimeout(initialDelay);

  const startTime = Date.now();
  let reloadAttempted = false;

  // For long strings, search for a significant portion (first 40 chars after any prefix)
  // to handle cases where UI truncates long domain names
  let searchPattern: string | RegExp;
  if (typeof identifier === 'string' && identifier.length > 50) {
    // Extract the unique part after the namespace prefix (usually after the first .)
    const dotIndex = identifier.indexOf('.');
    if (dotIndex > 0 && dotIndex < identifier.length - 10) {
      // Use the part after the first dot (the unique domain portion)
      const uniquePart = identifier.substring(dotIndex + 1, dotIndex + 40);
      searchPattern = new RegExp(uniquePart.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
    } else {
      // Fallback: use first 40 chars
      searchPattern = identifier.substring(0, 40);
    }
  } else {
    searchPattern = identifier;
  }

  while (Date.now() - startTime < timeout) {
    // Wait for any loading to complete first
    await waitForLoadingComplete(page, { timeout: 5000 }).catch(() => {
      // Ignore loading timeout - might not have a loader
    });

    // Try to find the resource using the search pattern
    const resourceLocator = page.getByText(searchPattern);
    const isVisible = await resourceLocator.first().isVisible().catch(() => false);

    if (isVisible) {
      return; // Resource found
    }

    // If not found and we haven't reloaded yet, try reloading
    if (reloadIfNotFound && !reloadAttempted) {
      reloadAttempted = true;
      await page.reload();
      await waitForLoadingComplete(page, { timeout: 5000 }).catch(() => {});
      continue;
    }

    // Wait a bit before retrying
    await page.waitForTimeout(500);
  }

  // Take a screenshot for debugging before throwing
  const screenshotPath = `test-results/debug-resource-not-found-${Date.now()}.png`;
  await page.screenshot({ path: screenshotPath, fullPage: true }).catch(() => {});

  throw new Error(
    `Resource with identifier "${identifier}" not found in UI after ${timeout}ms. Screenshot saved to: ${screenshotPath}`
  );
}

/**
 * Navigate to a page and wait for resources to load after an API mutation.
 * Use this after creating/updating resources via API to ensure UI is ready.
 *
 * @param page - Playwright Page instance
 * @param url - URL to navigate to
 * @param options - Configuration options
 */
export async function navigateAndWaitForData(
  page: Page,
  url: string,
  options: { timeout?: number } = {}
): Promise<void> {
  const { timeout = 10000 } = options;

  await page.goto(url);
  await waitForLoadingComplete(page, { timeout });

  // Wait for any data-loading states to clear
  const dataLoading = page.locator('[data-loading], [aria-busy="true"]');
  await expect(dataLoading).toHaveCount(0, { timeout: 5000 }).catch(() => {
    // Ignore if no data-loading elements exist
  });
}

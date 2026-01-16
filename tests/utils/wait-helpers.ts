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

import { Page, Locator, expect, Response } from '@playwright/test';

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

  const toastSelector = type
    ? `[role="alert"][data-type="${type}"], [role="status"][data-type="${type}"], .toast.${type}, .toast-${type}`
    : '[role="alert"], [role="status"], .toast, .Toastify__toast';

  const toast = page.locator(toastSelector);
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

  const modal = page.locator('[role="dialog"], [role="alertdialog"], .modal');
  await expect(modal).toBeVisible({ timeout });

  if (titleText) {
    const titleLocator = modal.locator(
      '[role="heading"], .modal-title, .dialog-title, h1, h2, h3'
    );
    await expect(titleLocator).toContainText(titleText);
  }

  return modal;
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

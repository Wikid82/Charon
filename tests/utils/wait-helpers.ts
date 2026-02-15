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

import { expect } from '../fixtures/test';
import type { Page, Locator, Response } from '@playwright/test';
import { clickSwitch } from './ui-helpers';

/**
 * Click an element and wait for an API response atomically.
 * Prevents race condition where response completes before wait starts.
 *
 * ✅ FIX P0: Added overlay detection and switch component handling
 *
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

  // ✅ FIX P0: Wait for config reload overlay to disappear
  const overlay = page.locator('[data-testid="config-reload-overlay"]');
  await overlay.waitFor({ state: 'hidden', timeout: 10000 }).catch(() => {
    // Overlay not present or already hidden - continue
  });

  const locator =
    typeof clickTarget === 'string' ? page.locator(clickTarget) : clickTarget;

  // ✅ FIX P0: Detect if clicking a switch component and use proper method
  const role = await locator.getAttribute('role').catch(() => null);
  const isSwitch = role === 'switch' ||
    (await locator.getAttribute('type').catch(() => null) === 'checkbox' &&
     await locator.getAttribute('aria-label').then(l => (l || '').includes('toggle')).catch(() => false));

  if (isSwitch) {
    // Use clickSwitch helper for switch components
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
      clickSwitch(locator, { timeout }),
    ]);
    return response;
  }

  // Regular click for non-switch elements
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
 * Click a Switch/Toggle component and wait for an API response atomically.
 * Uses the clickSwitch helper to handle the hidden input structure.
 * @param page - Playwright Page instance
 * @param switchLocator - Locator for the switch element
 * @param urlPattern - URL string or RegExp to match
 * @param options - Configuration options
 * @returns The matched response
 */
export async function clickSwitchAndWaitForResponse(
  page: Page,
  switchLocator: Locator,
  urlPattern: string | RegExp,
  options: { status?: number; timeout?: number; scrollPadding?: number } = {}
): Promise<Response> {
  const { status = 200, timeout = 30000, scrollPadding = 100 } = options;

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
    clickSwitch(switchLocator, { scrollPadding, timeout }),
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

  // react-hot-toast uses:
  // - role="status" for success/info toasts
  // - role="alert" for error toasts
  let toast: Locator;

  if (type === 'error') {
    // Error toasts use role="alert"
    toast = page.locator(`[data-testid="toast-${type}"]`)
      .or(page.getByRole('alert'))
      .filter({ hasText: text })
      .first();
  } else if (type === 'success' || type === 'info') {
    // Success/info toasts use role="status"
    toast = page.locator(`[data-testid="toast-${type}"]`)
      .or(page.getByRole('status'))
      .filter({ hasText: text })
      .first();
  } else {
    // Any toast: check both roles
    toast = page.locator('[data-testid^="toast-"]:not([data-testid="toast-container"])')
      .or(page.getByRole('status'))
      .or(page.getByRole('alert'))
      .filter({ hasText: text })
      .first();
  }

  await expect(toast).toBeVisible({ timeout });
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
  // Increase default timeout to 60s to tolerate slower CI/backends; individual
  // tests may override this if they expect faster responses.
  const { status, timeout = 60000 } = options;

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

  if (page.isClosed()) {
    return;
  }

  // Wait for visible loading indicators to disappear.
  // Avoid broad class-based selectors (e.g. .loading, .spinner) to prevent
  // false positives from persistent layout/status elements.
  const loader = page.locator([
    '[role="progressbar"]:not([aria-label*="Challenge timeout progress"])',
    '[aria-busy="true"]',
    '[data-loading="true"]',
    '[data-testid="config-reload-overlay"]',
    '[data-testid="loading-spinner"]',
    '[role="status"][aria-label="Loading"]',
    '[role="status"][aria-label="Authenticating"]',
    '[role="status"][aria-label="Security Loading"]'
  ].join(', '));

  try {
    await expect
      .poll(async () => {
        const count = await loader.count();
        if (count === 0) {
          return 0;
        }

        let visibleCount = 0;
        for (let index = 0; index < count; index += 1) {
          if (await loader.nth(index).isVisible().catch(() => false)) {
            visibleCount += 1;
          }
        }

        return visibleCount;
      }, { timeout })
      .toBe(0);
  } catch (error) {
    if (page.isClosed()) {
      return;
    }

    throw error;
  }
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
  // Use .first() to avoid specific strict mode violations if multiple exist in DOM
  const dialogModal = page
    .locator('[role="dialog"], .modal')
    .filter({ hasText: titleText })
    .first();

  const slideOutPanel = page
    .locator('h2, h3')
    .filter({ hasText: titleText })
    .first();

  // Wait for either the dialog modal or the slide-out panel heading to be visible
  try {
    // FIX STRICT MODE VIOLATION:
    // If we match both the dialog AND the heading inside it, .or() returns 2 elements.
    // We strictly want to wait until *at least one* is visible.
    // Using .first() on the combined locator prevents 'strict mode violation' when both match.
    await expect(dialogModal.or(slideOutPanel).first()).toBeVisible({ timeout });
  } catch (e) {
    // If neither is found, throw a more helpful error
    throw new Error(
      `waitForModal: Could not find visible modal dialog or slide-out panel matching "${titleText}". Error: ${e instanceof Error ? e.message : String(e)}`
    );
  }

  // If dialog modal is visible, use it
  if (await dialogModal.isVisible()) {
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
 * Options for waitForFeatureFlagPropagation
 */
export interface FeatureFlagPropagationOptions {
  /** Polling interval in ms (default: 500ms) */
  interval?: number;
  /** Maximum time to wait (default: 30000ms) */
  timeout?: number;
  /** Maximum number of polling attempts (calculated from timeout/interval) */
  maxAttempts?: number;
}

// ✅ FIX 1.3: Cache for in-flight requests (per-worker isolation)
// Prevents duplicate API calls when multiple tests wait for same flag state
// See: E2E Test Timeout Remediation Plan (Sprint 1, Fix 1.3)
const inflightRequests = new Map<string, Promise<Record<string, boolean>>>();

// ✅ FIX 3.2: Track API call metrics for performance monitoring
// See: E2E Test Timeout Remediation Plan (Phase 3, Fix 3.2)
const apiMetrics = {
  featureFlagCalls: 0,
  cacheHits: 0,
  cacheMisses: 0,
};

/**
 * Get current API call metrics
 * Returns a copy to prevent external mutation
 */
export function getAPIMetrics() {
  return { ...apiMetrics };
}

/**
 * Reset all API call metrics to zero
 * Useful for cleanup between test suites
 */
export function resetAPIMetrics() {
  apiMetrics.featureFlagCalls = 0;
  apiMetrics.cacheHits = 0;
  apiMetrics.cacheMisses = 0;
}

/**
 * Normalize feature flag keys to handle API prefix inconsistencies.
 * Accepts both "cerberus.enabled" and "feature.cerberus.enabled" formats.
 *
 * ✅ FIX P0: Handles API key format mismatch where tests expect "cerberus.enabled"
 * but API returns "feature.cerberus.enabled"
 *
 * @param key - Feature flag key (with or without "feature." prefix)
 * @returns Normalized key with "feature." prefix
 */
function normalizeKey(key: string): string {
  // If key already has "feature." prefix, return as-is
  if (key.startsWith('feature.')) {
    return key;
  }
  // Otherwise, add the "feature." prefix
  return `feature.${key}`;
}

/**
 * Generate stable cache key with worker isolation
 * Prevents cache collisions between parallel workers
 *
 * ✅ FIX P0: Uses normalized keys to ensure cache hits work correctly
 */
function generateCacheKey(
  expectedFlags: Record<string, boolean>,
  workerIndex: number
): string {
  // Sort keys and normalize them to ensure consistent cache keys
  // {cerberus.enabled:true} === {feature.cerberus.enabled:true}
  const sortedFlags = Object.keys(expectedFlags)
    .sort()
    .reduce((acc, key) => {
      const normalizedKey = normalizeKey(key);
      acc[normalizedKey] = expectedFlags[key];
      return acc;
    }, {} as Record<string, boolean>);

  // Include worker index to isolate parallel processes
  return `${workerIndex}:${JSON.stringify(sortedFlags)}`;
}

/**
 * Polls the /feature-flags endpoint until expected state is returned.
 * Replaces hard-coded waits with condition-based verification.
 * Includes request coalescing to reduce API load.
 *
 * ✅ FIX P1: Increased timeout from 30s to 60s and added overlay detection
 * to handle config reload delays during feature flag propagation.
 *
 * ✅ FIX 2.3: Quick check for expected state before polling
 * Skips polling if flags are already in expected state (50% fewer iterations).
 *
 * @param page - Playwright page object
 * @param expectedFlags - Map of flag names to expected boolean values
 * @param options - Polling configuration
 * @returns The response once expected state is confirmed
 *
 * @example
 * ```typescript
 * // Wait for Cerberus flag to be disabled
 * await waitForFeatureFlagPropagation(page, {
 *   'cerberus.enabled': false
 * });
 * ```
 */
export async function waitForFeatureFlagPropagation(
  page: Page,
  expectedFlags: Record<string, boolean>,
  options: FeatureFlagPropagationOptions = {}
): Promise<Record<string, boolean>> {
  // ✅ FIX 3.2: Track feature flag API calls
  apiMetrics.featureFlagCalls++;

  // ✅ FIX P1: Wait for config reload overlay to disappear first
  // The overlay delays feature flag propagation when Caddy reloads config
  const overlay = page.locator('[data-testid="config-reload-overlay"]');
  await overlay.waitFor({ state: 'hidden', timeout: 10000 }).catch(() => {
    // Overlay not present or already hidden - continue
  });

  // ✅ FIX 2.3: Quick check - are we already in expected state?
  const currentState = await page.evaluate(async () => {
    const res = await fetch('/api/v1/feature-flags');
    return res.json();
  });

  const alreadyMatches = Object.entries(expectedFlags).every(
    ([key, expectedValue]) => {
      const normalizedKey = normalizeKey(key);
      return currentState[normalizedKey] === expectedValue;
    }
  );

  if (alreadyMatches) {
    console.log('[POLL] Feature flags already in expected state - skipping poll');
    return currentState;
  }

  // ✅ FIX 1.3: Request coalescing with worker isolation
  const { test } = await import('@playwright/test');
  const workerIndex = test.info().parallelIndex;
  const cacheKey = generateCacheKey(expectedFlags, workerIndex);

  // Return cached promise if request already in flight for this worker
  if (inflightRequests.has(cacheKey)) {
    console.log(`[CACHE HIT] Worker ${workerIndex}: ${cacheKey}`);
    // ✅ FIX 3.2: Track cache hit
    apiMetrics.cacheHits++;
    return inflightRequests.get(cacheKey)!;
  }

  console.log(`[CACHE MISS] Worker ${workerIndex}: ${cacheKey}`);
  // ✅ FIX 3.2: Track cache miss
  apiMetrics.cacheMisses++;

  const interval = options.interval ?? 500;
  const timeout = options.timeout ?? 60000; // ✅ FIX P1: Increased from 30s to 60s
  const maxAttempts = options.maxAttempts ?? Math.ceil(timeout / interval);

  // Create new polling promise
  const pollingPromise = (async () => {
    let lastResponse: Record<string, boolean> | null = null;
    let attemptCount = 0;

    while (attemptCount < maxAttempts) {
      attemptCount++;

      // GET /feature-flags via page context to respect CORS and auth
      const response = await page.evaluate(async () => {
        const res = await fetch('/api/v1/feature-flags', {
          method: 'GET',
          headers: { 'Content-Type': 'application/json' },
        });
        return {
          ok: res.ok,
          status: res.status,
          data: await res.json(),
        };
      });

      lastResponse = response.data as Record<string, boolean>;

      // ✅ FIX P0: Check if all expected flags match (with normalization)
      const allMatch = Object.entries(expectedFlags).every(
        ([key, expectedValue]) => {
          const normalizedKey = normalizeKey(key);
          const actualValue = response.data[normalizedKey];

          if (actualValue === undefined) {
            console.log(`[WARN] Key "${normalizedKey}" not found in API response`);
            return false;
          }

          const matches = actualValue === expectedValue;
          if (!matches) {
            console.log(`[MISMATCH] ${normalizedKey}: expected ${expectedValue}, got ${actualValue}`);
          }
          return matches;
        }
      );

      if (allMatch) {
        console.log(
          `[POLL] Feature flags propagated after ${attemptCount} attempts (${attemptCount * interval}ms)`
        );
        return lastResponse;
      }

      // Wait before next attempt
      await page.waitForTimeout(interval);
    }

    // Timeout: throw error with diagnostic info
    throw new Error(
      `Feature flag propagation timeout after ${attemptCount} attempts (${timeout}ms).\n` +
        `Expected: ${JSON.stringify(expectedFlags)}\n` +
        `Actual: ${JSON.stringify(lastResponse)}`
    );
  })();

  // Cache the promise
  inflightRequests.set(cacheKey, pollingPromise);

  try {
    const result = await pollingPromise;
    return result;
  } finally {
    // Remove from cache after completion
    inflightRequests.delete(cacheKey);
  }
}

/**
 * Options for retryAction
 */
export interface RetryOptions {
  /** Maximum number of attempts (default: 3) */
  maxAttempts?: number;
  /** Base delay between attempts in ms for exponential backoff (default: 2000ms) */
  baseDelay?: number;
  /** Maximum delay cap in ms (default: 10000ms) */
  maxDelay?: number;
  /** Maximum total time in ms (default: 15000ms per attempt) */
  timeout?: number;
}

/**
 * Retries an action with exponential backoff.
 * Handles transient network/DB failures gracefully.
 *
 * Retry sequence with defaults: 2s, 4s, 8s (capped at maxDelay)
 *
 * @param action - Async function to retry
 * @param options - Retry configuration
 * @returns Result of successful action
 *
 * @example
 * ```typescript
 * await retryAction(async () => {
 *   const response = await clickAndWaitForResponse(page, toggle, /\/feature-flags/);
 *   expect(response.ok()).toBeTruthy();
 * });
 * ```
 */
export async function retryAction<T>(
  action: () => Promise<T>,
  options: RetryOptions = {}
): Promise<T> {
  const maxAttempts = options.maxAttempts ?? 3;
  const baseDelay = options.baseDelay ?? 2000;
  const maxDelay = options.maxDelay ?? 10000;

  let lastError: Error | null = null;

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      console.log(`[RETRY] Attempt ${attempt}/${maxAttempts}`);
      return await action(); // Success!
    } catch (error) {
      lastError = error as Error;
      console.log(`[RETRY] Attempt ${attempt} failed: ${lastError.message}`);

      if (attempt < maxAttempts) {
        // Exponential backoff: 2s, 4s, 8s (capped at maxDelay)
        const delay = Math.min(baseDelay * Math.pow(2, attempt - 1), maxDelay);
        console.log(`[RETRY] Waiting ${delay}ms before retry...`);
        await new Promise((resolve) => setTimeout(resolve, delay));
      }
    }
  }

  // All attempts failed
  throw new Error(
    `Action failed after ${maxAttempts} attempts.\n` +
      `Last error: ${lastError?.message}`
  );
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

/**
 * Clear the feature flag cache
 * Useful for cleanup or resetting cache state in test hooks
 */
export function clearFeatureFlagCache(): void {
  inflightRequests.clear();
  console.log('[CACHE] Cleared all cached feature flag requests');
}

// ============================================================================
// Phase 2.1: Semantic Wait Helpers for Browser Alignment Triage
// ============================================================================

/**
 * Options for waitForDialog
 */
export interface DialogOptions {
  /** ARIA role to match (default: 'dialog') */
  role?: 'dialog' | 'alertdialog';
  /** Maximum time to wait (default: 5000ms) */
  timeout?: number;
}

/**
 * Wait for dialog to be visible and interactive.
 * Replaces: await page.waitForTimeout(500) after dialog open
 *
 * This function ensures the dialog is fully rendered and ready for interaction,
 * handling loading states and ensuring no aria-busy attributes remain.
 *
 * @param page - Playwright Page instance
 * @param options - Configuration options
 * @returns Locator for the dialog
 *
 * @example
 * ```typescript
 * // Instead of:
 * await getAddCertButton(page).click();
 * await page.waitForTimeout(500);
 *
 * // Use:
 * await getAddCertButton(page).click();
 * const dialog = await waitForDialog(page);
 * await expect(dialog).toBeVisible();
 * ```
 */
export async function waitForDialog(
  page: Page,
  options: DialogOptions = {}
): Promise<Locator> {
  const { role = 'dialog', timeout = 5000 } = options;

  const dialog = page.getByRole(role);

  // Wait for dialog to be visible
  await expect(dialog).toBeVisible({ timeout });

  // Ensure dialog is fully rendered and interactive (not busy)
  await expect(dialog).not.toHaveAttribute('aria-busy', 'true', { timeout: 1000 }).catch(() => {
    // aria-busy might not be present, which is fine
  });

  // Wait for any loading states within the dialog to clear
  const dialogLoader = dialog.locator('[role="progressbar"], [aria-busy="true"], .loading-spinner');
  await expect(dialogLoader).toHaveCount(0, { timeout: 2000 }).catch(() => {
    // No loaders present is acceptable
  });

  return dialog;
}

/**
 * Options for waitForFormFields
 */
export interface FormFieldsOptions {
  /** Maximum time to wait (default: 5000ms) */
  timeout?: number;
  /** Whether field should be enabled (default: true) */
  shouldBeEnabled?: boolean;
}

/**
 * Wait for dynamically loaded form fields to be ready.
 * Replaces: await page.waitForTimeout(1000) after selecting form type
 *
 * This function waits for form fields to be visible and enabled,
 * handling dynamic field rendering based on form selection.
 *
 * @param page - Playwright Page instance
 * @param fieldSelector - Selector for the field to wait for
 * @param options - Configuration options
 *
 * @example
 * ```typescript
 * // Instead of:
 * await providerSelect.selectOption('manual');
 * await page.waitForTimeout(1000);
 *
 * // Use:
 * await providerSelect.selectOption('manual');
 * await waitForFormFields(page, 'input[name="domain"]');
 * ```
 */
export async function waitForFormFields(
  page: Page,
  fieldSelector: string,
  options: FormFieldsOptions = {}
): Promise<void> {
  const { timeout = 5000, shouldBeEnabled = true } = options;

  const field = page.locator(fieldSelector);

  // Wait for field to be visible
  await expect(field).toBeVisible({ timeout });

  // Wait for field to be enabled if required
  if (shouldBeEnabled) {
    await expect(field).toBeEnabled({ timeout: 1000 });
  }

  // Ensure field is attached to DOM (not detached during render)
  await expect(field).toBeAttached({ timeout: 1000 });
}

/**
 * Options for waitForDebounce
 */
export interface DebounceOptions {
  /** Selector for loading indicator (optional) */
  indicatorSelector?: string;
  /** Maximum time to wait (default: 3000ms) */
  timeout?: number;
  /** Optional delay for debounce settling (default: 300ms) */
  delay?: number;
}

/**
 * Wait for debounced input to settle (e.g., search, autocomplete).
 * Replaces: await page.waitForTimeout(500) after input typing
 *
 * This function waits for either a loading indicator to appear/disappear
 * or for the network to be idle, handling debounced search scenarios.
 *
 * @param page - Playwright Page instance
 * @param options - Configuration options
 *
 * @example
 * ```typescript
 * // Instead of:
 * await searchInput.fill('test');
 * await page.waitForTimeout(500);
 *
 * // Use:
 * await searchInput.fill('test');
 * await waitForDebounce(page, { indicatorSelector: '.search-loading' });
 * ```
 */
export async function waitForDebounce(
  page: Page,
  options: DebounceOptions = {}
): Promise<void> {
  const { indicatorSelector, timeout = 3000, delay = 300 } = options;

  if (indicatorSelector) {
    // Wait for loading indicator to appear and disappear
    const indicator = page.locator(indicatorSelector);
    await indicator.waitFor({ state: 'visible', timeout: 1000 }).catch(() => {
      // Indicator might not appear if response is very fast
    });
    await indicator.waitFor({ state: 'hidden', timeout });
  } else {
    // Manually wait for the debounce delay to ensure subsequent requests are triggered
    if (delay > 0) {
      await page.waitForTimeout(delay);
    }
    // Wait for network to be idle (default debounce strategy)
    await page.waitForLoadState('networkidle', { timeout });
  }
}

/**
 * Options for waitForConfigReload
 */
export interface ConfigReloadOptions {
  /** Maximum time to wait (default: 10000ms) */
  timeout?: number;
}

/**
 * Wait for config reload overlay to appear and disappear.
 * Replaces: await page.waitForTimeout(500) after settings change
 *
 * This function handles the "Reloading configuration..." overlay that appears
 * when Caddy configuration is reloaded after settings changes.
 *
 * @param page - Playwright Page instance
 * @param options - Configuration options
 *
 * @example
 * ```typescript
 * // Instead of:
 * await saveButton.click();
 * await page.waitForTimeout(2000);
 *
 * // Use:
 * await saveButton.click();
 * await waitForConfigReload(page);
 * ```
 */
export async function waitForConfigReload(
  page: Page,
  options: ConfigReloadOptions = {}
): Promise<void> {
  const { timeout = 10000 } = options;

  // Config reload shows overlay with "Reloading configuration..." or similar
  const overlay = page.locator(
    '[data-testid="config-reload-overlay"], [role="status"]'
  ).filter({ hasText: /reloading|loading/i });

  // Wait for overlay to appear (may be very fast)
  await overlay.waitFor({ state: 'visible', timeout: 2000 }).catch(() => {
    // Overlay may not appear if reload is instant
  });

  // Wait for overlay to disappear
  await overlay.waitFor({ state: 'hidden', timeout }).catch(() => {
    // If overlay never appeared, continue
  });

  // Verify page is interactive again
  await page.waitForLoadState('domcontentloaded', { timeout: 3000 });
}

/**
 * Options for waitForNavigation
 */
export interface NavigationOptions {
  /** Maximum time to wait (default: 10000ms) */
  timeout?: number;
  /** Wait for load state (default: 'load') */
  waitUntil?: 'load' | 'domcontentloaded' | 'networkidle' | 'commit';
}

/**
 * Wait for URL change with proper assertions.
 * Replaces: await page.waitForTimeout(1000) then checking URL
 *
 * This function waits for navigation to complete and verifies the URL,
 * handling SPA-style navigation and page loads.
 *
 * @param page - Playwright Page instance
 * @param expectedUrl - Expected URL (string or RegExp)
 * @param options - Configuration options
 *
 * @example
 * ```typescript
 * // Instead of:
 * await link.click();
 * await page.waitForTimeout(1000);
 * expect(page.url()).toContain('/settings');
 *
 * // Use:
 * await link.click();
 * await waitForNavigation(page, /\/settings/);
 * ```
 */
export async function waitForNavigation(
  page: Page,
  expectedUrl: string | RegExp,
  options: NavigationOptions = {}
): Promise<void> {
  const { timeout = 10000, waitUntil = 'load' } = options;

  // Wait for URL to change to expected value (commit-level avoids SPA timeouts)
  await page.waitForURL(expectedUrl, { timeout, waitUntil: 'commit' });

  // Additional verification using auto-waiting assertion
  if (typeof expectedUrl === 'string') {
    await expect(page).toHaveURL(expectedUrl, { timeout: 1000 });
  } else {
    await expect(page).toHaveURL(expectedUrl, { timeout: 1000 });
  }

  // Ensure page is fully loaded when a navigation actually triggers a load event
  if (waitUntil !== 'commit') {
    await page.waitForLoadState(waitUntil, { timeout }).catch(() => {
      // Same-document navigations (SPA) may not fire the requested load state.
    });
  }
}

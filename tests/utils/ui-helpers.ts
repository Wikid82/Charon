/**
 * UI Helpers - Shared utilities for common UI interactions
 *
 * These helpers provide reusable, robust locator strategies for common UI patterns
 * to reduce duplication and prevent flaky tests.
 */

import { Page, Locator, expect } from '@playwright/test';

/**
 * Options for toast helper
 */
export interface ToastHelperOptions {
  /** Maximum time to wait for toast (default: 5000ms) */
  timeout?: number;
  /** Toast type to match (success, error, info, warning) */
  type?: 'success' | 'error' | 'info' | 'warning';
}

/**
 * Get a toast locator with proper role-based selection and short retries.
 * Uses data-testid for our custom toast system to avoid strict-mode violations.
 *
 * react-hot-toast uses:
 * - role="status" for success/info toasts
 * - role="alert" for error toasts
 *
 * @param page - Playwright Page instance
 * @param text - Text or RegExp to match in toast (optional for type-only match)
 * @param options - Configuration options
 * @returns Locator for the toast
 *
 * @example
 * ```typescript
 * const toast = getToastLocator(page, /success/i, { type: 'success' });
 * await expect(toast).toBeVisible({ timeout: 5000 });
 * ```
 */
export function getToastLocator(
  page: Page,
  text?: string | RegExp,
  options: ToastHelperOptions = {}
): Locator {
  const { type } = options;

  // Build selector with fallbacks for reliability
  // react-hot-toast: role="status" for success/info, role="alert" for errors
  let baseLocator: Locator;

  if (type === 'error') {
    // Error toasts use role="alert"
    baseLocator = page.locator(`[data-testid="toast-${type}"]`)
      .or(page.getByRole('alert'));
  } else if (type === 'success' || type === 'info') {
    // Success/info toasts use role="status"
    baseLocator = page.locator(`[data-testid="toast-${type}"]`)
      .or(page.getByRole('status'));
  } else if (type === 'warning') {
    // Warning toasts - check both roles as fallback
    baseLocator = page.locator(`[data-testid="toast-${type}"]`)
      .or(page.getByRole('status'))
      .or(page.getByRole('alert'));
  } else {
    // Any toast: match our custom toast container with fallbacks for both roles
    baseLocator = page.locator('[data-testid^="toast-"]')
      .or(page.getByRole('status'))
      .or(page.getByRole('alert'));
  }

  // Filter by text if provided
  if (text) {
    return baseLocator.filter({ hasText: text }).first();
  }

  return baseLocator.first();
}

/**
 * Wait for a toast to appear with specific text and type.
 * Wrapper around getToastLocator with built-in wait.
 *
 * @param page - Playwright Page instance
 * @param text - Text or RegExp to match in toast
 * @param options - Configuration options
 */
export async function waitForToast(
  page: Page,
  text: string | RegExp,
  options: ToastHelperOptions = {}
): Promise<void> {
  const { timeout = 5000 } = options;
  const toast = getToastLocator(page, text, options);
  await expect(toast).toBeVisible({ timeout });
}

/**
 * Options for row-scoped button locator
 */
export interface RowScopedButtonOptions {
  /** Maximum time to wait for button (default: 5000ms) */
  timeout?: number;
  /** Button role (default: 'button') */
  role?: 'button' | 'link';
}

/**
 * Get a button locator scoped to a specific table row, avoiding strict-mode violations.
 * Use this when multiple rows have buttons with the same name (e.g., "Invite", "Resend").
 *
 * @param page - Playwright Page instance
 * @param rowIdentifier - Text to identify the row (e.g., email, name)
 * @param buttonName - Button name/label or accessible name pattern
 * @param options - Configuration options
 * @returns Locator for the button within the row
 *
 * @example
 * ```typescript
 * // Find "Invite" button in row containing "user@example.com"
 * const inviteBtn = getRowScopedButton(page, 'user@example.com', /invite/i);
 * await inviteBtn.click();
 * ```
 */
export function getRowScopedButton(
  page: Page,
  rowIdentifier: string | RegExp,
  buttonName: string | RegExp,
  options: RowScopedButtonOptions = {}
): Locator {
  const { role = 'button' } = options;

  // Find the row containing the identifier
  const row = page.getByRole('row').filter({ hasText: rowIdentifier });

  // Find the button within that row
  return row.getByRole(role, { name: buttonName });
}

/**
 * Get an action button in a table row by icon class (e.g., lucide-mail for resend).
 * Use when buttons don't have proper accessible names.
 *
 * @param page - Playwright Page instance
 * @param rowIdentifier - Text to identify the row
 * @param iconClass - Icon class to match (e.g., 'lucide-mail', 'lucide-trash-2')
 * @returns Locator for the button
 *
 * @example
 * ```typescript
 * // Find resend button (mail icon) in row containing "user@example.com"
 * const resendBtn = getRowScopedIconButton(page, 'user@example.com', 'lucide-mail');
 * await resendBtn.click();
 * ```
 */
export function getRowScopedIconButton(
  page: Page,
  rowIdentifier: string | RegExp,
  iconClass: string
): Locator {
  const row = page.getByRole('row').filter({ hasText: rowIdentifier });
  return row.locator(`button:has(svg.${iconClass})`).first();
}

/**
 * Wait for a certificate form validation message (email field).
 * Targets the visible validation message with proper role/text.
 *
 * @param page - Playwright Page instance
 * @param messagePattern - Pattern to match in validation message
 * @param options - Configuration options
 * @returns Locator for the validation message
 *
 * @example
 * ```typescript
 * const validationMsg = getCertificateValidationMessage(page, /valid.*email/i);
 * await expect(validationMsg).toBeVisible();
 * ```
 */
export function getCertificateValidationMessage(
  page: Page,
  messagePattern: string | RegExp
): Locator {
  // Look for validation message in common locations:
  // 1. Adjacent to input with aria-describedby
  // 2. Role="alert" or "status" for live region
  // 3. Common validation message containers
  return page
    .locator('[role="alert"], [role="status"], .text-red-500, [class*="error"]')
    .filter({ hasText: messagePattern })
    .first();
}

/**
 * Refresh a list/table and wait for it to stabilize.
 * Use after creating resources via API or UI to ensure list reflects changes.
 *
 * @param page - Playwright Page instance
 * @param options - Configuration options
 */
export async function refreshListAndWait(
  page: Page,
  options: { timeout?: number } = {}
): Promise<void> {
  const { timeout = 5000 } = options;

  // Reload the page
  await page.reload();

  // Wait for list to be visible (supports table, grid, or card layouts)
  // Try table first, then grid, then card container
  let listElement = page.getByRole('table');
  let isVisible = await listElement.isVisible({ timeout: 1000 }).catch(() => false);

  if (!isVisible) {
    // Fallback to grid layout (e.g., DNS providers in grid)
    listElement = page.locator('.grid > div, [data-testid="list-container"]');
    isVisible = await listElement.first().isVisible({ timeout: 1000 }).catch(() => false);
  }

  // If still not visible, wait for the page to stabilize with any content
  if (!isVisible) {
    await page.waitForLoadState('networkidle', { timeout });
  }

  // Wait for any loading indicators to clear
  const loader = page.locator('[role="progressbar"], [aria-busy="true"], .loading-spinner');
  await expect(loader).toHaveCount(0, { timeout: 3000 }).catch(() => {
    // Ignore if no loader exists
  });
}

/**
 * Options for switch helper functions
 */
export interface SwitchOptions {
  /** Timeout for waiting operations (default: 5000ms) */
  timeout?: number;
  /** Padding to add above element when scrolling (default: 100px for sticky header) */
  scrollPadding?: number;
}

/**
 * Click a Switch/Toggle component reliably across all browsers.
 *
 * The Switch component uses a hidden input with a styled sibling div.
 * This helper clicks the parent <label> to trigger the toggle.
 *
 * ✅ FIX P0: Wait for ConfigReloadOverlay to disappear before clicking
 * The overlay intercepts pointer events during Caddy config reloads.
 *
 * @param locator - Locator for the switch (e.g., page.getByRole('switch'))
 * @param options - Configuration options
 *
 * @example
 * ```typescript
 * // By role with name
 * await clickSwitch(page.getByRole('switch', { name: /cerberus/i }));
 *
 * // By test ID
 * await clickSwitch(page.getByTestId('toggle-acl'));
 *
 * // By label
 * await clickSwitch(page.getByLabel(/enabled/i));
 * ```
 */
export async function clickSwitch(
  locator: Locator,
  options: SwitchOptions = {}
): Promise<void> {
  const { scrollPadding = 100, timeout = 5000 } = options;

  // ✅ FIX P0: Wait for config reload overlay to disappear
  // The ConfigReloadOverlay component (z-50) intercepts pointer events
  // during Caddy config reloads, blocking all interactions
  const page = locator.page();
  const overlay = page.locator('[data-testid="config-reload-overlay"]');
  await overlay.waitFor({ state: 'hidden', timeout: 10000 }).catch(() => {
    // Overlay not present or already hidden - continue
  });

  // Wait for the switch to be visible
  await expect(locator).toBeVisible({ timeout });

  // Get the parent label element
  // Switch structure: <label><input sr-only /><div /></label>
  const labelElement = locator.locator('xpath=ancestor::label').first();

  // Scroll with padding to clear sticky header
  await labelElement.evaluate((el, padding) => {
    el.scrollIntoView({ block: 'center' });
    // Additional scroll if near top
    const rect = el.getBoundingClientRect();
    if (rect.top < padding) {
      window.scrollBy(0, -(padding - rect.top));
    }
  }, scrollPadding);

  // Click the label (which triggers the input)
  await labelElement.click();
}

/**
 * Assert a Switch/Toggle component's checked state.
 *
 * @param locator - Locator for the switch
 * @param expected - Expected checked state (true/false)
 * @param options - Configuration options
 */
export async function expectSwitchState(
  locator: Locator,
  expected: boolean,
  options: SwitchOptions = {}
): Promise<void> {
  const { timeout = 5000 } = options;

  if (expected) {
    await expect(locator).toBeChecked({ timeout });
  } else {
    await expect(locator).not.toBeChecked({ timeout });
  }
}

/**
 * Toggle a Switch/Toggle component and verify the state changed.
 * Returns the new checked state.
 *
 * @param locator - Locator for the switch
 * @param options - Configuration options
 * @returns The new checked state after toggle
 */
export async function toggleSwitch(
  locator: Locator,
  options: SwitchOptions = {}
): Promise<boolean> {
  const { timeout = 5000 } = options;

  // Get current state
  const wasChecked = await locator.isChecked();

  // Click to toggle
  await clickSwitch(locator, options);

  // Verify state changed and return new state
  const newState = !wasChecked;
  await expectSwitchState(locator, newState, { timeout });

  return newState;
}

/**
 * Options for form field helper
 */
export interface FormFieldOptions {
  /** Placeholder text to use as fallback */
  placeholder?: string | RegExp;
  /** Field ID to use as fallback */
  fieldId?: string;
}

/**
 * Get form field with cross-browser label matching.
 * Tries multiple strategies: label, placeholder, id, aria-label.
 *
 * ✅ FIX 2.2: Cross-browser label matching for Firefox/WebKit compatibility
 * Implements fallback chain to handle browser differences in label association.
 *
 * @param page - Playwright Page instance
 * @param labelPattern - Text or RegExp to match label
 * @param options - Configuration options with fallback strategies
 * @returns Locator for the form field
 *
 * @example
 * ```typescript
 * // Basic usage with label only
 * const nameInput = getFormFieldByLabel(page, /name/i);
 *
 * // With fallbacks for robustness
 * const scriptField = getFormFieldByLabel(
 *   page,
 *   /script.*path/i,
 *   {
 *     placeholder: /dns-challenge\.sh/i,
 *     fieldId: 'field-script_path'
 *   }
 * );
 * ```
 */
export function getFormFieldByLabel(
  page: Page,
  labelPattern: string | RegExp,
  options: FormFieldOptions = {}
): Locator {
  const baseLocator = page.getByLabel(labelPattern);

  // Build fallback chain
  let locator = baseLocator;

  if (options.placeholder) {
    locator = locator.or(page.getByPlaceholder(options.placeholder));
  }

  if (options.fieldId) {
    locator = locator.or(page.locator(`#${options.fieldId}`));
  }

  // Fallback: role + label text nearby
  if (typeof labelPattern === 'string') {
    locator = locator.or(
      page.getByRole('textbox').filter({
        has: page.locator(`label:has-text("${labelPattern}")`),
      })
    );
  }

  return locator;
}

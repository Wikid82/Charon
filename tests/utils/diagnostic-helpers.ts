import { Page, ConsoleMessage, Request } from '@playwright/test';

/**
 * Diagnostic Helpers for E2E Test Debugging
 *
 * These helpers enable comprehensive browser console logging and state capture
 * to diagnose test interruptions and failures. Use during Phase 1 investigation
 * to identify root causes of browser context closures.
 *
 * @see docs/reports/phase1_diagnostics.md
 */

/**
 * Enable comprehensive browser console logging for diagnostic purposes
 * Captures console logs, page errors, request failures, and unhandled rejections
 *
 * @param page - Playwright Page instance
 * @param options - Optional configuration for logging behavior
 *
 * @example
 * ```typescript
 * test.beforeEach(async ({ page }) => {
 *   enableDiagnosticLogging(page);
 *   // ... test setup
 * });
 * ```
 */
export function enableDiagnosticLogging(
  page: Page,
  options: {
    captureConsole?: boolean;
    captureErrors?: boolean;
    captureRequests?: boolean;
    captureDialogs?: boolean;
  } = {}
): void {
  const {
    captureConsole = true,
    captureErrors = true,
    captureRequests = true,
    captureDialogs = true,
  } = options;

  // Console messages (all levels)
  if (captureConsole) {
    page.on('console', (msg: ConsoleMessage) => {
      const type = msg.type().toUpperCase();
      const text = msg.text();
      const location = msg.location();

      // Special formatting for errors and warnings
      if (type === 'ERROR' || type === 'WARNING') {
        console.error(`[BROWSER ${type}] ${text}`);
      } else {
        console.log(`[BROWSER ${type}] ${text}`);
      }

      if (location.url) {
        console.log(
          `  Location: ${location.url}:${location.lineNumber}:${location.columnNumber}`
        );
      }
    });
  }

  // Page errors (JavaScript exceptions)
  if (captureErrors) {
    page.on('pageerror', (error: Error) => {
      console.error('═══════════════════════════════════════════');
      console.error('PAGE ERROR DETECTED');
      console.error('═══════════════════════════════════════════');
      console.error('Message:', error.message);
      console.error('Stack:', error.stack);
      console.error('Timestamp:', new Date().toISOString());
      console.error('═══════════════════════════════════════════');
    });
  }

  // Request failures (network errors)
  if (captureRequests) {
    page.on('requestfailed', (request: Request) => {
      const failure = request.failure();
      console.error('─────────────────────────────────────────');
      console.error('REQUEST FAILED');
      console.error('─────────────────────────────────────────');
      console.error('URL:', request.url());
      console.error('Method:', request.method());
      console.error('Error:', failure?.errorText || 'Unknown');
      console.error('Timestamp:', new Date().toISOString());
      console.error('─────────────────────────────────────────');
    });
  }

  // Unhandled promise rejections
  if (captureErrors) {
    page.on('console', (msg: ConsoleMessage) => {
      if (msg.type() === 'error' && msg.text().includes('Unhandled')) {
        console.error('╔═══════════════════════════════════════════╗');
        console.error('║   UNHANDLED PROMISE REJECTION DETECTED    ║');
        console.error('╚═══════════════════════════════════════════╝');
        console.error(msg.text());
        console.error('Timestamp:', new Date().toISOString());
      }
    });
  }

  // Dialog events (if supported)
  if (captureDialogs) {
    page.on('dialog', async (dialog) => {
      console.log(`[DIALOG] Type: ${dialog.type()}, Message: ${dialog.message()}`);
      console.log(`[DIALOG] Timestamp: ${new Date().toISOString()}`);
      // Auto-dismiss to prevent blocking
      await dialog.dismiss();
    });
  }
}

/**
 * Capture page state snapshot for debugging
 * Logs current URL, title, and HTML content length
 *
 * @param page - Playwright Page instance
 * @param label - Descriptive label for this snapshot
 *
 * @example
 * ```typescript
 * await capturePageState(page, 'Before dialog open');
 * // ... perform action
 * await capturePageState(page, 'After dialog close');
 * ```
 */
export async function capturePageState(page: Page, label: string): Promise<void> {
  const url = page.url();
  const title = await page.title();
  const html = await page.content();

  console.log(`\n========== PAGE STATE: ${label} ==========`);
  console.log(`URL: ${url}`);
  console.log(`Title: ${title}`);
  console.log(`HTML Length: ${html.length} characters`);
  console.log(`Timestamp: ${new Date().toISOString()}`);
  console.log(`===========================================\n`);
}

/**
 * Track dialog lifecycle events for resource leak detection
 * Logs when dialogs open and close to identify cleanup issues
 *
 * @param page - Playwright Page instance
 * @param dialogSelector - Selector for the dialog element
 *
 * @example
 * ```typescript
 * test('dialog test', async ({ page }) => {
 *   const tracker = trackDialogLifecycle(page, '[role="dialog"]');
 *
 *   await openDialog(page);
 *   await closeDialog(page);
 *
 *   tracker.stop();
 * });
 * ```
 */
export function trackDialogLifecycle(
  page: Page,
  dialogSelector: string = '[role="dialog"]'
): { stop: () => void } {
  let dialogCount = 0;
  let isRunning = true;

  const checkDialog = async () => {
    if (!isRunning) return;

    const dialogCount = await page.locator(dialogSelector).count();

    if (dialogCount > 0) {
      console.log(`[DIALOG LIFECYCLE] ${dialogCount} dialog(s) detected on page`);
      console.log(`[DIALOG LIFECYCLE] Timestamp: ${new Date().toISOString()}`);
    }

    setTimeout(() => checkDialog(), 1000);
  };

  // Start monitoring
  checkDialog();

  return {
    stop: () => {
      isRunning = false;
      console.log('[DIALOG LIFECYCLE] Tracking stopped');
    },
  };
}

/**
 * Monitor browser context health during test execution
 * Detects when browser context is closed unexpectedly
 *
 * @param page - Playwright Page instance
 *
 * @example
 * ```typescript
 * test.beforeEach(async ({ page }) => {
 *   monitorBrowserContext(page);
 * });
 * ```
 */
export function monitorBrowserContext(page: Page): void {
  const context = page.context();
  const browser = context.browser();

  context.on('close', () => {
    console.error('╔═══════════════════════════════════════════╗');
    console.error('║   BROWSER CONTEXT CLOSED UNEXPECTEDLY     ║');
    console.error('╚═══════════════════════════════════════════╝');
    console.error('Timestamp:', new Date().toISOString());
    console.error('This may indicate a resource leak or crash.');
  });

  if (browser) {
    browser.on('disconnected', () => {
      console.error('╔═══════════════════════════════════════════╗');
      console.error('║   BROWSER DISCONNECTED UNEXPECTEDLY       ║');
      console.error('╚═══════════════════════════════════════════╝');
      console.error('Timestamp:', new Date().toISOString());
    });
  }

  page.on('close', () => {
    console.warn('[PAGE CLOSED]', new Date().toISOString());
  });
}

/**
 * Performance monitoring helper
 * Tracks test execution time and identifies slow operations
 *
 * @example
 * ```typescript
 * test('my test', async ({ page }) => {
 *   const perf = startPerformanceMonitoring('My Test');
 *
 *   perf.mark('Dialog open start');
 *   await openDialog(page);
 *   perf.mark('Dialog open end');
 *
 *   perf.measure('Dialog open', 'Dialog open start', 'Dialog open end');
 *   perf.report();
 * });
 * ```
 */
export function startPerformanceMonitoring(testName: string) {
  const startTime = performance.now();
  const marks: Map<string, number> = new Map();
  const measures: Array<{ name: string; duration: number }> = [];

  return {
    mark(name: string): void {
      marks.set(name, performance.now());
      console.log(`[PERF MARK] ${name} at ${marks.get(name)! - startTime}ms`);
    },

    measure(name: string, startMark: string, endMark: string): void {
      const start = marks.get(startMark);
      const end = marks.get(endMark);

      if (start !== undefined && end !== undefined) {
        const duration = end - start;
        measures.push({ name, duration });
        console.log(`[PERF MEASURE] ${name}: ${duration.toFixed(2)}ms`);
      } else {
        console.warn(`[PERF WARN] Missing marks for measure: ${name}`);
      }
    },

    report(): void {
      const totalTime = performance.now() - startTime;

      console.log('\n========== PERFORMANCE REPORT ==========');
      console.log(`Test: ${testName}`);
      console.log(`Total Duration: ${totalTime.toFixed(2)}ms`);
      console.log('\nMeasurements:');
      measures.forEach(({ name, duration }) => {
        console.log(`  ${name}: ${duration.toFixed(2)}ms`);
      });
      console.log('=========================================\n');
    },
  };
}

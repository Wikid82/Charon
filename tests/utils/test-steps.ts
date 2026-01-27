/**
 * Test Step Logging Helpers
 *
 * Wrapper around test.step() that automatically logs step execution
 * with duration tracking, error handling, and integration with DebugLogger.
 *
 * Usage:
 *   import { testStep } from './test-steps';
 *   await testStep('Navigate to home page', async () => {
 *     await page.goto('/');
 *   });
 */

import { test, Page, expect } from '@playwright/test';
import { DebugLogger } from './debug-logger';

export interface TestStepOptions {
  timeout?: number;
  retries?: number;
  soft?: boolean;
  logger?: DebugLogger;
}

/**
 * Wrapper around test.step() with automatic logging and metrics
 */
export async function testStep<T>(
  name: string,
  fn: () => Promise<T>,
  options: TestStepOptions = {}
): Promise<T> {
  const startTime = performance.now();
  let duration = 0;

  try {
    const result = await test.step(name, fn, {
      timeout: options.timeout,
      box: false,
    });

    duration = performance.now() - startTime;

    if (options.logger) {
      options.logger.step(name, Math.round(duration));
    }

    return result;
  } catch (error) {
    duration = performance.now() - startTime;

    if (options.logger) {
      options.logger.error(name, error as Error, options.retries);
    }

    if (options.soft) {
      // In soft assertion mode, log but don't throw
      console.warn(`⚠️  Soft failure in step "${name}": ${error}`);
      return undefined as any;
    }

    throw error;
  }
}

/**
 * Page interaction helper with automatic logging
 */
export class LoggedPage {
  private logger: DebugLogger;
  private page: Page;

  constructor(page: Page, logger: DebugLogger) {
    this.page = page;
    this.logger = logger;
  }

  async click(selector: string): Promise<void> {
    return testStep(`Click: ${selector}`, async () => {
      const locator = this.page.locator(selector);
      const isVisible = await locator.isVisible().catch(() => false);
      this.logger.locator(selector, 'click', isVisible, 0);
      await locator.click();
    }, { logger: this.logger });
  }

  async fill(selector: string, text: string): Promise<void> {
    return testStep(`Fill: ${selector}`, async () => {
      const locator = this.page.locator(selector);
      const isVisible = await locator.isVisible().catch(() => false);
      this.logger.locator(selector, 'fill', isVisible, 0);
      await locator.fill(text);
    }, { logger: this.logger });
  }

  async goto(url: string): Promise<void> {
    return testStep(`Navigate to: ${url}`, async () => {
      await this.page.goto(url);
    }, { logger: this.logger });
  }

  async waitForNavigation(fn: () => Promise<void>): Promise<void> {
    return testStep('Wait for navigation', async () => {
      await Promise.all([
        this.page.waitForNavigation(),
        fn(),
      ]);
    }, { logger: this.logger });
  }

  async screenshot(name: string): Promise<Buffer> {
    return testStep(`Screenshot: ${name}`, async () => {
      return this.page.screenshot({ fullPage: true });
    }, { logger: this.logger });
  }

  getBaseLogger(): DebugLogger {
    return this.logger;
  }

  getPage(): Page {
    return this.page;
  }
}

/**
 * Assertion helper with automatic logging
 */
export async function testAssert(
  condition: string,
  assertion: () => Promise<void>,
  logger?: DebugLogger
): Promise<void> {
  try {
    await assertion();
    logger?.assertion(condition, true);
  } catch (error) {
    logger?.assertion(condition, false);
    throw error;
  }
}

/**
 * Create a logged page wrapper for a test
 */
export function createLoggedPage(page: Page, logger: DebugLogger): LoggedPage {
  return new LoggedPage(page, logger);
}

/**
 * Run a test step with retry logic and logging
 */
export async function testStepWithRetry<T>(
  name: string,
  fn: () => Promise<T>,
  maxRetries: number = 2,
  options: TestStepOptions = {}
): Promise<T> {
  let lastError: Error | undefined;

  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      return await testStep(
        attempt === 1 ? name : `${name} (Retry ${attempt - 1}/${maxRetries - 1})`,
        fn,
        options
      );
    } catch (error) {
      lastError = error as Error;

      if (attempt < maxRetries) {
        const backoff = Math.pow(2, attempt - 1) * 100; // Exponential backoff
        await new Promise(resolve => setTimeout(resolve, backoff));
      }
    }
  }

  throw new Error(`Failed after ${maxRetries} attempts: ${lastError?.message}`);
}

/**
 * Measure and log the duration of an async operation
 */
export async function measureStep<T>(
  name: string,
  fn: () => Promise<T>,
  logger?: DebugLogger
): Promise<{ result: T; duration: number }> {
  const startTime = performance.now();
  const result = await fn();
  const duration = performance.now() - startTime;

  if (logger) {
    logger.step(name, Math.round(duration));
  }

  return { result, duration };
}

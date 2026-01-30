/**
 * Debug Logger Utility for Playwright E2E Tests
 *
 * Provides structured logging for test execution with:
 * - Color-coded console output for local runs
 * - Structured JSON output for CI parsing
 * - Automatic duration tracking
 * - Sensitive data sanitization (auth tokens, headers)
 * - Integration with Playwright HTML report
 *
 * Usage:
 *   const logger = new DebugLogger('test-name');
 *   logger.step('User login', async () => {
 *     await page.click('[role="button"]');
 *   });
 *   logger.assertion('Button is visible', visible);
 *   logger.error('Network failed', error);
 */

import { test } from '@playwright/test';

export interface DebugLoggerOptions {
  testName?: string;
  browser?: string;
  shard?: string;
  file?: string;
}

export interface NetworkLogEntry {
  method: string;
  url: string;
  status?: number;
  elapsedMs: number;
  requestHeaders?: Record<string, string>;
  responseContentType?: string;
  responseBodySize?: number;
  error?: string;
  timestamp: string;
}

export interface LocatorLogEntry {
  selector: string;
  action: string;
  found: boolean;
  elapsedMs: number;
  timestamp: string;
}

// ANSI color codes for console output
const COLORS = {
  reset: '\x1b[0m',
  dim: '\x1b[2m',
  bold: '\x1b[1m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
};

export class DebugLogger {
  private testName: string;
  private browser: string;
  private shard: string;
  private file: string;
  private isCI: boolean;
  private logs: string[] = [];
  private networkLogs: NetworkLogEntry[] = [];
  private locatorLogs: LocatorLogEntry[] = [];
  private startTime: number;
  private stepStack: string[] = [];

  constructor(options: DebugLoggerOptions = {}) {
    this.testName = options.testName || 'unknown';
    this.browser = options.browser || 'chromium';
    this.shard = options.shard || 'unknown';
    this.file = options.file || 'unknown';
    this.isCI = !!process.env.CI;
    this.startTime = Date.now();
  }

  /**
   * Log a test step with automatic duration tracking
   */
  step(name: string, duration?: number): void {
    const indentation = '  '.repeat(this.stepStack.length);
    const prefix = `${indentation}├─`;
    const durationStr = duration ? ` (${duration}ms)` : '';

    const message = `${prefix} ${name}${durationStr}`;
    this.logMessage(message, 'step');

    // Report to Playwright's test.step system
    test.step(name, async () => {
      // Step already logged
    }).catch(() => {
      // Ignore if not in test context
    });
  }

  /**
   * Log network activity (requests/responses)
   */
  network(entry: Partial<NetworkLogEntry>): void {
    const fullEntry: NetworkLogEntry = {
      method: entry.method || 'UNKNOWN',
      url: this.sanitizeURL(entry.url || ''),
      status: entry.status,
      elapsedMs: entry.elapsedMs || 0,
      error: entry.error,
      timestamp: new Date().toISOString(),
      requestHeaders: this.sanitizeHeaders(entry.requestHeaders),
      responseContentType: entry.responseContentType,
      responseBodySize: entry.responseBodySize,
    };

    this.networkLogs.push(fullEntry);

    const statusIcon = this.getStatusIcon(fullEntry.status);
    const statusStr = fullEntry.status ? `[${fullEntry.status}]` : '[no-status]';
    const message = `   ${statusIcon} ${fullEntry.method} ${this.truncateURL(fullEntry.url)} ${statusStr} ${fullEntry.elapsedMs}ms`;

    this.logMessage(message, 'network');
  }

  /**
   * Log page state information
   */
  pageState(label: string, state: Record<string, any>): void {
    const sanitized = this.sanitizeObject(state);
    const message = `   📄 Page State: ${label}`;
    this.logMessage(message, 'page-state');

    if (this.isCI) {
      // In CI, log structured format
      this.logs.push(JSON.stringify({
        type: 'page-state',
        label,
        state: sanitized,
        timestamp: new Date().toISOString(),
      }));
    }
  }

  /**
   * Log locator activity
   */
  locator(selector: string, action: string, found: boolean, elapsedMs: number): void {
    const entry: LocatorLogEntry = {
      selector,
      action,
      found,
      elapsedMs,
      timestamp: new Date().toISOString(),
    };

    this.locatorLogs.push(entry);

    const icon = found ? '✓' : '✗';
    const message = `   ${icon} ${action} "${selector}" ${elapsedMs}ms`;
    this.logMessage(message, found ? 'locator-found' : 'locator-missing');
  }

  /**
   * Log assertion result
   */
  assertion(condition: string, passed: boolean, actual?: any, expected?: any): void {
    const icon = passed ? '✓' : '✗';
    const color = passed ? COLORS.green : COLORS.red;
    const baseMessage = `   ${icon} Assert: ${condition}`;

    if (actual !== undefined && expected !== undefined) {
      const actualStr = this.formatValue(actual);
      const expectedStr = this.formatValue(expected);
      const message = `${baseMessage} | expected: ${expectedStr}, actual: ${actualStr}`;
      this.logMessage(message, passed ? 'assertion-pass' : 'assertion-fail');
    } else {
      this.logMessage(baseMessage, passed ? 'assertion-pass' : 'assertion-fail');
    }
  }

  /**
   * Log error with context
   */
  error(context: string, error: Error | string, recoveryAttempts?: number): void {
    const errorMessage = typeof error === 'string' ? error : error.message;
    const errorStack = typeof error === 'string' ? '' : error.stack;

    const message = `   ❌ ERROR: ${context} - ${errorMessage}`;
    this.logMessage(message, 'error');

    if (recoveryAttempts) {
      const recoveryMsg = `   🔄 Recovery: ${recoveryAttempts} attempts remaining`;
      this.logMessage(recoveryMsg, 'recovery');
    }

    if (this.isCI && errorStack) {
      this.logs.push(JSON.stringify({
        type: 'error',
        context,
        message: errorMessage,
        stack: errorStack,
        timestamp: new Date().toISOString(),
      }));
    }
  }

  /**
   * Get test duration in milliseconds
   */
  getDuration(): number {
    return Date.now() - this.startTime;
  }

  /**
   * Get all log entries as structured JSON
   */
  getStructuredLogs(): any {
    return {
      test: {
        name: this.testName,
        browser: this.browser,
        shard: this.shard,
        file: this.file,
        durationMs: this.getDuration(),
        timestamp: new Date().toISOString(),
      },
      network: this.networkLogs,
      locators: this.locatorLogs,
      rawLogs: this.logs,
    };
  }

  /**
   * Export network logs as CSV for analysis
   */
  getNetworkCSV(): string {
    const headers = ['Timestamp', 'Method', 'URL', 'Status', 'Duration (ms)', 'Content-Type', 'Body Size', 'Error'];
    const rows = this.networkLogs.map(entry => [
      entry.timestamp,
      entry.method,
      entry.url,
      entry.status || '',
      entry.elapsedMs,
      entry.responseContentType || '',
      entry.responseBodySize || '',
      entry.error || '',
    ]);

    return [headers, ...rows].map(row => row.map(cell => `"${cell}"`).join(',')).join('\n');
  }

  /**
   * Get a summary of slow operations
   */
  getSlowOperations(threshold: number = 1000): { type: string; name: string; duration: number }[] {
    // Note: We'd need to track operations with names in step() for this to be fully useful
    // For now, return slow network requests
    return this.networkLogs
      .filter(entry => entry.elapsedMs > threshold)
      .map(entry => ({
        type: 'network',
        name: `${entry.method} ${entry.url}`,
        duration: entry.elapsedMs,
      }));
  }

  /**
   * Print all logs to console with colors
   */
  printSummary(): void {
    const duration = this.getDuration();
    const durationStr = this.formatDuration(duration);

    const summary = `
${COLORS.cyan}📊 Test Summary${COLORS.reset}
${COLORS.dim}${'─'.repeat(60)}${COLORS.reset}
Test:          ${this.testName}
Browser:       ${this.browser}
Shard:         ${this.shard}
Duration:      ${durationStr}
Network Reqs:  ${this.networkLogs.length}
Locator Calls: ${this.locatorLogs.length}
${COLORS.dim}${'─'.repeat(60)}${COLORS.reset}`;

    console.log(summary);

    // Show slowest operations
    const slowOps = this.getSlowOperations(500);
    if (slowOps.length > 0) {
      console.log(`${COLORS.yellow}⚠️  Slow Operations (>500ms):${COLORS.reset}`);
      slowOps.forEach(op => {
        console.log(`   ${op.type.padEnd(10)} ${op.name.substring(0, 40)} ${op.duration}ms`);
      });
    }
  }

  // ────────────────────────────────────────────────────────────────────
  // Private helper methods
  // ────────────────────────────────────────────────────────────────────

  private logMessage(message: string, type: string): void {
    if (this.isCI) {
      // In CI, store as structured JSON
      this.logs.push(JSON.stringify({
        type,
        message,
        timestamp: new Date().toISOString(),
      }));
    } else {
      // Locally, output with colors
      const colorCode = this.getColorForType(type);
      console.log(`${colorCode}${message}${COLORS.reset}`);
    }
  }

  private getColorForType(type: string): string {
    const colorMap: Record<string, string> = {
      step: COLORS.blue,
      network: COLORS.cyan,
      'page-state': COLORS.magenta,
      'locator-found': COLORS.green,
      'locator-missing': COLORS.yellow,
      'assertion-pass': COLORS.green,
      'assertion-fail': COLORS.red,
      error: COLORS.red,
      recovery: COLORS.yellow,
    };
    return colorMap[type] || COLORS.reset;
  }

  private getStatusIcon(status?: number): string {
    if (!status) return '❓';
    if (status >= 200 && status < 300) return '✅';
    if (status >= 300 && status < 400) return '➡️';
    if (status >= 400 && status < 500) return '⚠️';
    return '❌';
  }

  private sanitizeURL(url: string): string {
    try {
      const parsed = new URL(url);
      // Remove sensitive query params
      const sensitiveParams = ['token', 'key', 'secret', 'password', 'auth'];
      sensitiveParams.forEach(param => {
        parsed.searchParams.delete(param);
      });
      return parsed.toString();
    } catch {
      return url;
    }
  }

  private sanitizeHeaders(headers?: Record<string, string>): Record<string, string> | undefined {
    if (!headers) return undefined;

    const sanitized = { ...headers };
    const sensitiveHeaders = [
      'authorization',
      'cookie',
      'x-api-key',
      'x-emergency-token',
      'x-auth-token',
    ];

    sensitiveHeaders.forEach(header => {
      Object.keys(sanitized).forEach(key => {
        if (key.toLowerCase() === header) {
          sanitized[key] = '[REDACTED]';
        }
      });
    });

    return sanitized;
  }

  private sanitizeObject(obj: any): any {
    if (typeof obj !== 'object' || obj === null) {
      return obj;
    }

    if (Array.isArray(obj)) {
      return obj.map(item => this.sanitizeObject(item));
    }

    const sanitized: any = {};
    const sensitiveKeys = ['password', 'token', 'secret', 'key', 'auth'];

    for (const [key, value] of Object.entries(obj)) {
      if (sensitiveKeys.some(sk => key.toLowerCase().includes(sk))) {
        sanitized[key] = '[REDACTED]';
      } else if (typeof value === 'object') {
        sanitized[key] = this.sanitizeObject(value);
      } else {
        sanitized[key] = value;
      }
    }

    return sanitized;
  }

  private truncateURL(url: string, maxLength: number = 50): string {
    if (url.length > maxLength) {
      return url.substring(0, maxLength - 3) + '...';
    }
    return url;
  }

  private formatValue(value: any): string {
    if (typeof value === 'string') {
      return `"${value}"`;
    }
    if (typeof value === 'boolean') {
      return value ? 'true' : 'false';
    }
    if (typeof value === 'number') {
      return value.toString();
    }
    if (typeof value === 'object') {
      return JSON.stringify(value, null, 2).substring(0, 100);
    }
    return String(value);
  }

  private formatDuration(ms: number): string {
    if (ms < 1000) {
      return `${ms}ms`;
    }
    const seconds = (ms / 1000).toFixed(2);
    return `${seconds}s`;
  }
}

/**
 * Create a logger for the current test context
 */
export function createLogger(filename: string): DebugLogger {
  const testInfo = test.info?.();

  return new DebugLogger({
    testName: testInfo?.title || 'unknown',
    browser: testInfo?.project?.name || 'chromium',
    shard: testInfo?.parallelIndex?.toString() || '0',
    file: filename,
  });
}

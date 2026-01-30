/**
 * Network Interceptor Fixture
 *
 * Intercepts all HTTP requests and responses to capture network metrics,
 * log network activity, and export data for analysis.
 *
 * Usage in fixtures:
 *   import { networkInterceptor } from '../fixtures/network';
 *
 *   test.beforeEach(async ({ page }, testInfo) => {
 *     const interceptor = new NetworkInterceptor();
 *     interceptor.attach(page);
 *   });
 *
 *   test.afterEach(async ({ }, testInfo) => {
 *     const csv = interceptor.exportCSV();
 *     // Save to artifacts
 *   });
 */

import { Page, Request, Response } from '@playwright/test';
import { DebugLogger, NetworkLogEntry } from '../utils/debug-logger';
import { WriteStream, createWriteStream } from 'fs';
import { join } from 'path';

interface NetworkMetrics {
  url: string;
  method: string;
  startTime: number;
  status?: number;
  errorMessage?: string;
  requestSize: number;
  responseSize: number;
  duration: number;
  redirectChain: string[];
  requestHeaders?: Record<string, string>;
  responseHeaders?: Record<string, string>;
}

export class NetworkInterceptor {
  private requests = new Map<string, NetworkMetrics>();
  private redirectChains = new Map<string, string[]>();
  private logger?: DebugLogger;
  private csvStream?: WriteStream;

  constructor(logger?: DebugLogger) {
    this.logger = logger;
  }

  /**
   * Attach interceptor to a page
   */
  attach(page: Page): void {
    // Track request start times
    page.on('request', (request: Request) => {
      const url = request.url();
      const method = request.method();

      const metrics: NetworkMetrics = {
        url,
        method,
        startTime: Date.now(),
        requestSize: this.estimateSize(request.postDataBuffer()),
        responseSize: 0,
        duration: 0,
        redirectChain: [],
        requestHeaders: this.sanitizeHeaders(request.headers()),
      };

      this.requests.set(url, metrics);

      // Log request
      if (this.logger) {
        this.logger.network({
          method,
          url,
          elapsedMs: 0,
        });
      }
    });

    // Track response metrics
    page.on('response', (response: Response) => {
      const url = response.url();
      const metrics = this.requests.get(url);

      if (metrics) {
        metrics.status = response.status();
        metrics.duration = Date.now() - metrics.startTime;
        metrics.responseHeaders = this.sanitizeHeaders(response.headers() as Record<string, string>);

        const contentType = response.headers()['content-type'] || 'unknown';
        const contentLength = response.headers()['content-length'];
        if (contentLength) {
          metrics.responseSize = parseInt(contentLength, 10);
        }

        // Log response
        if (this.logger) {
          this.logger.network({
            method: metrics.method,
            url: metrics.url,
            status: metrics.status,
            elapsedMs: metrics.duration,
            responseContentType: contentType,
            responseBodySize: metrics.responseSize,
          });
        }
      }
    });

    // Track request failures
    page.on('requestfailed', (request: Request) => {
      const url = request.url();
      const metrics = this.requests.get(url);

      if (metrics) {
        metrics.duration = Date.now() - metrics.startTime;
        metrics.errorMessage = request.failure()?.errorText;

        if (this.logger) {
          this.logger.network({
            method: metrics.method,
            url: metrics.url,
            elapsedMs: metrics.duration,
            error: metrics.errorMessage,
          });
        }
      }
    });

    // Track redirects
    page.on('requestfinished', (request: Request) => {
      const redirectChain = request.redirectedFrom();
      if (redirectChain) {
        const chain = [];
        let current: Request | null = redirectChain;
        while (current) {
          chain.push(current.url());
          current = current.redirectedFrom();
        }
        this.redirectChains.set(request.url(), chain.reverse());
      }
    });
  }

  /**
   * Export all network metrics as CSV
   */
  exportCSV(): string {
    const headers = [
      'Timestamp',
      'Method',
      'URL',
      'Status',
      'Duration (ms)',
      'Request Size (bytes)',
      'Response Size (bytes)',
      'Error',
    ];

    const rows: string[][] = [];

    this.requests.forEach((metrics) => {
      const timestamp = new Date(metrics.startTime).toISOString();
      const row = [
        timestamp,
        metrics.method,
        metrics.url,
        metrics.status?.toString() || 'N/A',
        metrics.duration.toString(),
        metrics.requestSize.toString(),
        metrics.responseSize.toString(),
        metrics.errorMessage || '',
      ];
      rows.push(row);
    });

    // CSV format
    return [headers, ...rows].map(row => row.map(cell => `"${cell}"`).join(',')).join('\n');
  }

  /**
   * Export metrics in JSON format
   */
  exportJSON(): any {
    const data: any = {
      summary: {
        totalRequests: this.requests.size,
        timestamp: new Date().toISOString(),
      },
      requests: Array.from(this.requests.values()).map(metrics => ({
        method: metrics.method,
        url: metrics.url,
        status: metrics.status,
        duration: metrics.duration,
        requestSize: metrics.requestSize,
        responseSize: metrics.responseSize,
        requestHeaders: metrics.requestHeaders,
        responseHeaders: metrics.responseHeaders,
        error: metrics.errorMessage,
        timestamp: new Date(metrics.startTime).toISOString(),
      })),
    };
    return data;
  }

  /**
   * Get slow requests (above threshold)
   */
  getSlowRequests(thresholdMs: number = 1000): NetworkMetrics[] {
    return Array.from(this.requests.values())
      .filter(m => m.duration > thresholdMs)
      .sort((a, b) => b.duration - a.duration);
  }

  /**
   * Get failed requests
   */
  getFailedRequests(): NetworkMetrics[] {
    return Array.from(this.requests.values())
      .filter(m => m.status && (m.status >= 400 || m.errorMessage));
  }

  /**
   * Get request count by status code
   */
  getStatusCodeDistribution(): Record<string, number> {
    const distribution: Record<string, number> = {};

    this.requests.forEach((metrics) => {
      const code = metrics.status?.toString() || 'error';
      distribution[code] = (distribution[code] || 0) + 1;
    });

    return distribution;
  }

  /**
   * Get average response time by URL pattern
   */
  getAverageResponseTimeByPattern(): Record<string, number> {
    const patterns: Record<string, { total: number; count: number }> = {};

    this.requests.forEach((metrics) => {
      const pattern = this.getURLPattern(metrics.url);
      if (!patterns[pattern]) {
        patterns[pattern] = { total: 0, count: 0 };
      }
      patterns[pattern].total += metrics.duration;
      patterns[pattern].count += 1;
    });

    const averages: Record<string, number> = {};
    Object.entries(patterns).forEach(([pattern, data]) => {
      averages[pattern] = Math.round(data.total / data.count);
    });

    return averages;
  }

  /**
   * Save metrics to file
   */
  async saveMetrics(filepath: string, format: 'csv' | 'json' = 'csv'): Promise<void> {
    const fs = await import('fs').then(m => m.promises);

    let data: string;
    if (format === 'csv') {
      data = this.exportCSV();
    } else {
      data = JSON.stringify(this.exportJSON(), null, 2);
    }

    await fs.writeFile(filepath, data);
  }

  // ────────────────────────────────────────────────────────────────────
  // Private helpers
  // ────────────────────────────────────────────────────────────────────

  private sanitizeHeaders(headers: Record<string, string>): Record<string, string> {
    const sanitized = { ...headers };
    const sensitiveHeaders = [
      'authorization',
      'cookie',
      'x-api-key',
      'x-emergency-token',
      'x-auth-token',
      'set-cookie',
    ];

    Object.keys(sanitized).forEach(key => {
      if (sensitiveHeaders.some(sh => key.toLowerCase().includes(sh))) {
        sanitized[key] = '[REDACTED]';
      }
    });

    return sanitized;
  }

  private estimateSize(buffer?: Buffer): number {
    return buffer ? buffer.length : 0;
  }

  private getURLPattern(url: string): string {
    try {
      const parsed = new URL(url);
      // Return path pattern (remove specific IDs)
      const path = parsed.pathname.replace(/\/\d+/g, '/{id}');
      return `${parsed.origin}${path}`;
    } catch {
      return url;
    }
  }
}

/**
 * Create a network interceptor and attach to page
 */
export function createNetworkInterceptor(page: Page, logger?: DebugLogger): NetworkInterceptor {
  const interceptor = new NetworkInterceptor(logger);
  interceptor.attach(page);
  return interceptor;
}

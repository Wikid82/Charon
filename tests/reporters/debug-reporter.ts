/**
 * Debug Reporter for Playwright E2E Tests
 *
 * Custom reporter that:
 * - Tracks test step timing and identifies slow operations
 * - Aggregates failures by type (timeout, assertion, network)
 * - Outputs structured summary to stdout for CI consumption
 * - Logs timing statistics and slowest tests
 */

import { Reporter, TestCase, TestResult, Suite, FullResult } from '@playwright/test/reporter';

interface StepMetrics {
  name: string;
  duration: number;
  status: 'passed' | 'failed' | 'skipped';
}

interface TestMetrics {
  title: string;
  duration: number;
  steps: StepMetrics[];
  status: 'passed' | 'failed' | 'skipped';
  error?: string;
}

export default class DebugReporter implements Reporter {
  private tests: TestMetrics[] = [];
  private failuresByType = new Map<string, number>();
  private slowTests: { title: string; duration: number }[] = [];

  onTestEnd(test: TestCase, result: TestResult): void {
    // Parse step information from result
    const steps: StepMetrics[] = [];

    if (result.steps && result.steps.length > 0) {
      result.steps.forEach(step => {
        steps.push({
          name: step.title,
          duration: step.duration,
          status: step.error ? 'failed' : 'passed',
        });
      });
    }

    const metrics: TestMetrics = {
      title: test.title,
      duration: result.duration,
      steps,
      status: result.status as any,
      error: result.error?.message,
    };

    this.tests.push(metrics);

    // Track failure types
    if (result.status === 'failed' && result.error) {
      const errorMsg = result.error.message || '';
      let failureType = 'other';

      if (errorMsg.includes('timeout') || errorMsg.includes('Timeout')) {
        failureType = 'timeout';
      } else if (errorMsg.includes('assertion') || errorMsg.includes('Assertion')) {
        failureType = 'assertion';
      } else if (errorMsg.includes('network') || errorMsg.includes('Network')) {
        failureType = 'network';
      } else if (errorMsg.includes('not found') || errorMsg.includes('Cannot find')) {
        failureType = 'locator';
      }

      this.failuresByType.set(failureType, (this.failuresByType.get(failureType) || 0) + 1);
    }

    // Track slow tests
    if (result.duration > 5000) {
      // Tests slower than 5 seconds
      this.slowTests.push({
        title: test.title,
        duration: result.duration,
      });
    }
  }

  onEnd(result: FullResult): void {
    // Sort slow tests by duration
    this.slowTests.sort((a, b) => b.duration - a.duration);

    // Print summary to stdout for CI parsing
    this.printSummary();
    this.printSlowTests();
    this.printFailureAnalysis();
  }

  // ────────────────────────────────────────────────────────────────────
  // Private methods
  // ────────────────────────────────────────────────────────────────────

  private printSummary(): void {
    const total = this.tests.length;
    const passed = this.tests.filter(t => t.status === 'passed').length;
    const failed = this.tests.filter(t => t.status === 'failed').length;
    const skipped = this.tests.filter(t => t.status === 'skipped').length;
    const passRate = total > 0 ? Math.round((passed / total) * 100) : 0;

    console.log('\n╔════════════════════════════════════════════════════════════╗');
    console.log('║              E2E Test Execution Summary                      ║');
    console.log('╠════════════════════════════════════════════════════════════╣');
    console.log(`║ Total Tests:        ${String(total).padEnd(42)}║`);
    console.log(`║ ✅ Passed:          ${String(`${passed} (${passRate}%)`).padEnd(42)}║`);
    console.log(`║ ❌ Failed:          ${String(failed).padEnd(42)}║`);
    console.log(`║ ⏭️  Skipped:         ${String(skipped).padEnd(42)}║`);
    console.log('╚════════════════════════════════════════════════════════════╝\n');
  }

  private printSlowTests(): void {
    if (this.slowTests.length === 0) {
      return;
    }

    console.log('⏱️  Slow Tests (>5s):');
    console.log('─'.repeat(60));

    // Show top 10 slowest tests
    this.slowTests.slice(0, 10).forEach((test, index) => {
      const duration = (test.duration / 1000).toFixed(2);
      const name = test.title.substring(0, 40).padEnd(40);
      console.log(`${index + 1}. ${name} ${duration}s`);
    });

    console.log('');
  }

  private printFailureAnalysis(): void {
    if (this.failuresByType.size === 0) {
      return;
    }

    console.log('🔍 Failure Analysis by Type:');
    console.log('─'.repeat(60));

    const total = Array.from(this.failuresByType.values()).reduce((a, b) => a + b, 0);

    this.failuresByType.forEach((count, type) => {
      const percent = Math.round((count / total) * 100);
      const bar = '█'.repeat(Math.round(percent / 5));
      console.log(`${type.padEnd(12)} │ ${bar.padEnd(20)} ${count}/${total} (${percent}%)`);
    });

    console.log('');
  }
}

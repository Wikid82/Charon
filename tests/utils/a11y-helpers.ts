import { expect } from '../fixtures/test';
import type { AxeResults, Result } from 'axe-core';

export type ViolationImpact = 'critical' | 'serious' | 'moderate' | 'minor';

export interface A11yAssertionOptions {
  failOn?: ViolationImpact[];
  knownViolations?: string[];
}

const DEFAULT_FAIL_ON: ViolationImpact[] = ['critical', 'serious'];

export function getFailingViolations(
  results: AxeResults,
  options: A11yAssertionOptions = {},
): Result[] {
  const failOn = options.failOn ?? DEFAULT_FAIL_ON;
  const knownViolations = new Set(options.knownViolations ?? []);

  return results.violations.filter(
    (v) =>
      failOn.includes(v.impact as ViolationImpact) &&
      !knownViolations.has(v.id),
  );
}

export function formatViolation(violation: Result): string {
  const nodes = violation.nodes
    .map((node, i) => {
      const selector = node.target.join(' ');
      const html = node.html.length > 200
        ? `${node.html.slice(0, 200)}…`
        : node.html;
      const fix = node.failureSummary ?? '';
      return `    Node ${i + 1}: ${selector}\n      HTML: ${html}\n      Fix: ${fix}`;
    })
    .join('\n');

  return [
    `[${violation.impact?.toUpperCase()}] ${violation.id}: ${violation.description}`,
    `  Help: ${violation.helpUrl}`,
    `  Affected nodes (${violation.nodes.length}):`,
    nodes,
  ].join('\n');
}

export function expectNoA11yViolations(
  results: AxeResults,
  options: A11yAssertionOptions = {},
): void {
  const failing = getFailingViolations(results, options);

  const message = failing.length > 0
    ? `Found ${failing.length} accessibility violation(s):\n\n${failing.map(formatViolation).join('\n\n')}`
    : '';

  expect(failing, message).toEqual([]);
}

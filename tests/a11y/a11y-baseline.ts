export interface BaselineEntry {
  ruleId: string;
  pages: string[];
  reason: string;
  ticket?: string;
  expiresAt?: string;
}

export const A11Y_BASELINE: BaselineEntry[] = [
  {
    ruleId: 'color-contrast',
    pages: ['/'],
    reason: 'Tailwind blue-500 buttons (#3b82f6) have 3.67:1 contrast with white text; requires design system update',
    ticket: '#929',
    expiresAt: '2026-07-31',
  },
  {
    ruleId: 'label',
    pages: ['/settings/users', '/security', '/tasks/backups', '/tasks/import/caddyfile', '/tasks/import/crowdsec'],
    reason: 'Form inputs missing associated labels; requires frontend component fixes',
    ticket: '#929',
    expiresAt: '2026-07-31',
  },
  {
    ruleId: 'button-name',
    pages: ['/settings', '/security/headers'],
    reason: 'Icon-only buttons missing accessible names; requires aria-label additions',
    ticket: '#929',
    expiresAt: '2026-07-31',
  },
  {
    ruleId: 'select-name',
    pages: ['/tasks/logs'],
    reason: 'Select element missing associated label',
    ticket: '#929',
    expiresAt: '2026-07-31',
  },
  {
    ruleId: 'scrollable-region-focusable',
    pages: ['/tasks/logs'],
    reason: 'Log output container is scrollable but not keyboard-focusable',
    ticket: '#929',
    expiresAt: '2026-07-31',
  },
];

export function getBaselinedRuleIds(currentPage: string): string[] {
  return A11Y_BASELINE
    .filter((entry) => entry.pages.some((p) => currentPage.startsWith(p)))
    .map((entry) => entry.ruleId);
}

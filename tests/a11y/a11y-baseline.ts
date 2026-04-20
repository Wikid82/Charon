export interface BaselineEntry {
  ruleId: string;
  pages: string[];
  reason: string;
  ticket?: string;
  expiresAt?: string;
}

export const A11Y_BASELINE: BaselineEntry[] = [];

export function getBaselinedRuleIds(currentPage: string): string[] {
  return A11Y_BASELINE
    .filter((entry) => entry.pages.some((p) => currentPage.startsWith(p)))
    .map((entry) => entry.ruleId);
}

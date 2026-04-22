import { test as base } from './auth-fixtures';
import AxeBuilder from '@axe-core/playwright';

interface A11yFixtures {
  makeAxeBuilder: () => AxeBuilder;
}

export const test = base.extend<A11yFixtures>({
  makeAxeBuilder: async ({ page }, use) => {
    const makeAxeBuilder = () => {
      const builder = new AxeBuilder({ page })
        .withTags(['wcag2a', 'wcag2aa', 'wcag22aa'])
        .exclude('.chart-container canvas');

      // Wrap analyze() to wait for network idle first.
      // Without this, React Query background re-fetches can keep the JS event
      // loop busy while axe injects its analysis script, causing page.evaluate()
      // to hang until the test timeout is exceeded.
      const originalAnalyze = builder.analyze.bind(builder);
      builder.analyze = async () => {
        await page.waitForLoadState('networkidle', { timeout: 15000 }).catch(() => {
          // Acceptable: long-polling / SSE connections may prevent full idle
        });
        return originalAnalyze();
      };

      return builder;
    };
    await use(makeAxeBuilder);
  },
});

export { expect } from './auth-fixtures';

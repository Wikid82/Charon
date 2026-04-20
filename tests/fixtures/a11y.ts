import { test as base } from './auth-fixtures';
import AxeBuilder from '@axe-core/playwright';

interface A11yFixtures {
  makeAxeBuilder: () => AxeBuilder;
}

export const test = base.extend<A11yFixtures>({
  makeAxeBuilder: async ({ page }, use) => {
    const makeAxeBuilder = () =>
      new AxeBuilder({ page })
        .withTags(['wcag2a', 'wcag2aa', 'wcag22aa'])
        .exclude('.chart-container canvas');
    await use(makeAxeBuilder);
  },
});

export { expect } from './auth-fixtures';

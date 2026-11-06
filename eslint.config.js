// @ts-check
// Standalone ESLint flat config for the root Playwright E2E suite (tests/).
// Deliberately does NOT extend frontend/eslint.config.js: that config is React/Vitest/CSS oriented.
import js from '@eslint/js';
import playwright from 'eslint-plugin-playwright';
import tseslint from 'typescript-eslint';

/** Generated output and dependency directories that must never be linted. */
const GENERATED_IGNORES = [
  '**/node_modules/**',
  'test-results/**',
  'playwright-report/**',
  'playwright-output/**',
  'playwright/.auth/**',
  '**/.auth/**',
];

/** TypeScript specs: linted with type information via tests/tsconfig.json. */
const TS_FILES = ['tests/**/*.ts'];

/** Untyped JavaScript: tests/example.spec.js and the root Playwright config. */
const JS_FILES = ['tests/**/*.js', 'playwright.config.js'];

export default tseslint.config(
  { name: 'charon/root-ignores', ignores: GENERATED_IGNORES },

  js.configs.recommended,
  tseslint.configs.recommended,

  // Typed linting for TypeScript specs. Limited to rules whose findings are correctness bugs in
  // E2E code (an un-awaited Playwright call silently passes). The broad no-unsafe-* family is excluded.
  {
    name: 'charon/tests-typed',
    files: TS_FILES,
    languageOptions: {
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      '@typescript-eslint/no-floating-promises': 'error',
      '@typescript-eslint/no-misused-promises': 'error',
      '@typescript-eslint/await-thenable': 'error',
    },
  },

  // Untyped JavaScript cannot use type-aware rules.
  {
    name: 'charon/untyped-js',
    files: JS_FILES,
    extends: [tseslint.configs.disableTypeChecked],
    rules: {
      'no-unused-vars': 'warn',
    },
  },

  // Playwright rules (flat recommended) for every root E2E file.
  {
    ...playwright.configs['flat/recommended'],
    name: 'charon/playwright',
    files: [...TS_FILES, ...JS_FILES],
  },

  // Severity policy (see docs/plans/current_spec.md, Decision 3).
  // Correctness rules are 'error'. Style-oriented or noisy rules are 'warn' so they can be ratcheted down
  // over time. Rules are 'off' only where the pattern is intentional and fixing it would rewrite test logic.
  {
    name: 'charon/severity-policy',
    files: [...TS_FILES, ...JS_FILES],
    rules: {
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': 'warn',
      'playwright/no-skipped-test': 'warn',
      'playwright/no-force-option': 'warn',
      'playwright/prefer-locator': 'warn',
      'playwright/no-wait-for-timeout': 'warn',
      // Snapshot-then-compare checks (toggle before/after state) need a judgement call per site.
      'playwright/prefer-web-first-assertions': 'warn',
      // Intentional: environment guards (feature flags, optional UI) branch inside tests by design.
      'playwright/no-conditional-in-test': 'off',
      // Intentional: expects inside those guarded branches are the assertion for that environment.
      'playwright/no-conditional-expect': 'off',
      // Cosmetic spacing between blocks; not a correctness signal.
      'playwright/consistent-spacing-between-blocks': 'off',
      // Nested test.step is used on purpose to group sub-flows under a parent step.
      'playwright/no-nested-step': 'off',
      // Assertions in these tests live in shared helpers (e.g. expectNoA11yViolations, waitFor* checks) the rule
      // cannot see. Allow-listing helper names would hide genuinely assertion-free tests, so the rule is off.
      'playwright/expect-expect': 'off',
      // Restored to error: findings cleared in the lint sweep (commit 13).
      'prefer-const': 'error',
      'no-empty': 'error',
      'no-useless-escape': 'error',
      'preserve-caught-error': 'error',
      'no-useless-assignment': 'error',
      'playwright/missing-playwright-await': 'error',
      'playwright/valid-title': 'error',
      'playwright/no-networkidle': 'error',
      'playwright/no-wait-for-navigation': 'error',
      'playwright/no-unnecessary-assertions': 'error',
      'playwright/no-unused-locators': 'error',
      'playwright/no-wait-for-selector': 'error',
    },
  },
);

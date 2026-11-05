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
      // Temporarily 'warn' until commit 13 clears the baseline findings (see severity-policy below).
      '@typescript-eslint/no-floating-promises': 'warn',
      '@typescript-eslint/no-misused-promises': 'warn',
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
  // Noisy or style-oriented rules are warnings so they can be ratcheted down over time.
  {
    name: 'charon/severity-policy',
    files: [...TS_FILES, ...JS_FILES],
    rules: {
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': 'warn',
      'playwright/no-skipped-test': 'warn',
      // Baseline-only downgrades: these rules have error-level findings in tests/ today.
      // Commit 13 fixes the findings and restores each rule to 'error'.
      'prefer-const': 'warn',
      'no-empty': 'warn',
      'no-useless-escape': 'warn',
      'preserve-caught-error': 'warn',
      'no-useless-assignment': 'warn',
      'playwright/missing-playwright-await': 'warn',
      'playwright/valid-title': 'warn',
      'playwright/no-networkidle': 'warn',
      'playwright/prefer-web-first-assertions': 'warn',
      'playwright/no-wait-for-navigation': 'warn',
      'playwright/no-unnecessary-assertions': 'warn',
      'playwright/no-unused-locators': 'warn',
    },
  },
);

import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import reactRefresh from 'eslint-plugin-react-refresh';
import reactHooks from 'eslint-plugin-react-hooks';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import importX from 'eslint-plugin-import-x';
import unusedImports from 'eslint-plugin-unused-imports';
import promise from 'eslint-plugin-promise';
import unicorn from 'eslint-plugin-unicorn';
import sonarjs from 'eslint-plugin-sonarjs';
import security from 'eslint-plugin-security';
import noUnsanitized from 'eslint-plugin-no-unsanitized';
import testingLibrary from 'eslint-plugin-testing-library';
import vitest from '@vitest/eslint-plugin';
import css from '@eslint/css';
import json from '@eslint/json';
import markdown from '@eslint/markdown';

export default tseslint.config(
  { ignores: ['dist/**', 'node_modules/**', 'coverage/**'] },

  // ── Base configs (scoped to JS/TS to avoid breaking non-JS parsers) ──
  { ...js.configs.recommended, files: ['**/*.{ts,tsx,js,jsx,mjs,cjs}'] },
  ...tseslint.configs.recommended.map(config => ({
    ...config,
    files: config.files ?? ['**/*.{ts,tsx,js,jsx,mjs,cjs}'],
  })),

  // ── TypeScript + React (main source files) ────────────────────────────
  {
    files: ['**/*.{ts,tsx}'],
    plugins: {
      'react-refresh': reactRefresh,
      'react-hooks': reactHooks,
      'jsx-a11y': jsxA11y,
      'import-x': importX,
      'unused-imports': unusedImports,
      promise,
      unicorn,
      sonarjs,
      security,
      'no-unsanitized': noUnsanitized,
    },
    settings: {
      'import-x/resolver': {
        typescript: true,
        node: true,
      },
    },
    rules: {
      // ── React ──
      'react-refresh/only-export-components': 'warn',
      // React Compiler diagnostics now ship with eslint-plugin-react-hooks
      // (the standalone eslint-plugin-react-compiler is deprecated); keep
      // them at 'warn' to match the old react-compiler/react-compiler rule.
      ...Object.fromEntries(
        Object.keys(reactHooks.configs['recommended-latest'].rules).map(
          name => [name, 'warn']
        )
      ),
      'react-hooks/rules-of-hooks': 'error',
      'react-hooks/exhaustive-deps': 'warn',

      // ── TypeScript ──
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': 'off', // handled by unused-imports
      '@typescript-eslint/consistent-type-imports': [
        'warn',
        { prefer: 'type-imports', fixStyle: 'inline-type-imports' },
      ],

      // ── Unused imports ──
      'unused-imports/no-unused-imports': 'error',
      'unused-imports/no-unused-vars': [
        'warn',
        {
          vars: 'all',
          varsIgnorePattern: '^_',
          args: 'after-used',
          argsIgnorePattern: '^_',
        },
      ],

      // ── Import organization ──
      'import-x/order': [
        'warn',
        {
          groups: [
            'builtin',
            'external',
            'internal',
            ['parent', 'sibling', 'index'],
            'type',
          ],
          'newlines-between': 'always',
          alphabetize: { order: 'asc', caseInsensitive: true },
        },
      ],
      'import-x/no-duplicates': ['warn', { 'prefer-inline': true }],
      'import-x/no-cycle': ['warn', { maxDepth: 4 }],
      'import-x/no-self-import': 'error',

      // ── Accessibility ──
      ...jsxA11y.flatConfigs.recommended.rules,
      'jsx-a11y/label-has-associated-control': 'warn',
      'jsx-a11y/no-static-element-interactions': 'warn',
      'jsx-a11y/click-events-have-key-events': 'warn',
      'jsx-a11y/no-autofocus': 'warn',
      'jsx-a11y/role-has-required-aria-props': 'warn',
      'jsx-a11y/heading-has-content': 'warn',

      // ── Promises ──
      'promise/always-return': 'warn',
      'promise/no-return-wrap': 'error',
      'promise/catch-or-return': 'warn',
      'promise/no-nesting': 'warn',

      // ── Unicorn (cherry-picked) ──
      'unicorn/prefer-node-protocol': 'error',
      'unicorn/no-array-for-each': 'warn',
      'unicorn/prefer-array-find': 'warn',
      'unicorn/prefer-array-flat-map': 'warn',
      'unicorn/prefer-array-some': 'warn',
      'unicorn/prefer-includes': 'warn',
      'unicorn/prefer-string-starts-ends-with': 'warn',
      'unicorn/no-useless-spread': 'warn',
      'unicorn/no-useless-undefined': 'warn',
      'unicorn/prefer-optional-catch-binding': 'warn',
      'unicorn/prefer-ternary': ['warn', 'only-single-line'],
      'unicorn/no-lonely-if': 'warn',

      // ── Sonar (code smells) ──
      'sonarjs/no-identical-functions': 'warn',
      'sonarjs/no-duplicated-branches': 'warn',
      'sonarjs/no-collapsible-if': 'warn',
      'sonarjs/prefer-immediate-return': 'warn',

      // ── Security ──
      'security/detect-object-injection': 'off', // too noisy for frontend
      'security/detect-non-literal-regexp': 'warn',
      'security/detect-unsafe-regex': 'warn',
      'no-unsanitized/method': 'error',
      'no-unsanitized/property': 'error',
    },
  },

  // ── Test files ────────────────────────────────────────────────────────
  {
    files: ['**/*.test.{ts,tsx}', '**/*.spec.{ts,tsx}', '**/__tests__/**/*.{ts,tsx}'],
    plugins: {
      'testing-library': testingLibrary,
      vitest,
    },
    rules: {
      ...testingLibrary.configs['flat/react'].rules,
      ...vitest.configs.recommended.rules,
      // relax rules that are too noisy for the existing test suite
      '@typescript-eslint/no-explicit-any': 'off',
      'sonarjs/no-identical-functions': 'off',
      'testing-library/no-node-access': 'warn',
      'testing-library/prefer-find-by': 'warn',
      'testing-library/no-container': 'warn',
      'testing-library/no-wait-for-multiple-assertions': 'warn',
      'testing-library/no-unnecessary-act': 'warn',
      'testing-library/no-manual-cleanup': 'warn',
      'testing-library/render-result-naming-convention': 'warn',
      'vitest/expect-expect': 'warn',
    },
  },

  // ── CSS files ─────────────────────────────────────────────────────────
  {
    files: ['**/*.css'],
    language: 'css/css',
    plugins: { css },
    rules: {
      'css/no-duplicate-imports': 'error',
      'css/no-empty-blocks': 'warn',
    },
  },

  // ── JSON files ────────────────────────────────────────────────────────
  {
    files: ['**/*.json'],
    ignores: ['package-lock.json', 'tsconfig*.json'],
    language: 'json/json',
    plugins: { json },
    rules: {
      'json/no-duplicate-keys': 'error',
    },
  },

  // ── Markdown files ────────────────────────────────────────────────────
  {
    files: ['**/*.md'],
    plugins: { markdown },
    language: 'markdown/gfm',
    rules: {
      'markdown/no-html': 'off',
    },
  }
);

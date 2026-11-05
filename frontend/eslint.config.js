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
import { fixupPluginRules } from '@eslint/compat';
import i18nJson from 'eslint-plugin-i18n-json';
import path from 'node:path';

// Legacy testing-library warnings tracked in GH #1853. These files keep
// testing-library/no-node-access and no-container at "off" until their hits are
// fixed. Remove entries as files are cleaned up; the list may only shrink. Every
// other test file keeps the rules at "warn" (see the test-file block below).
const LEGACY_TESTING_LIBRARY_FILES = [
  'src/components/__tests__/AccessListForm.test.tsx',
  'src/components/__tests__/AccessListSelector-token-coverage.test.tsx',
  'src/components/__tests__/ActiveDecisionsTable.test.tsx',
  'src/components/__tests__/CSPBuilder.test.tsx',
  'src/components/__tests__/CertificateList.test.tsx',
  'src/components/__tests__/CertificateStatusCard.test.tsx',
  'src/components/__tests__/CredentialManager.test.tsx',
  'src/components/__tests__/CrowdSecBouncerKeyDisplay.test.tsx',
  'src/components/__tests__/DatabaseMaintenance.test.tsx',
  'src/components/__tests__/FeedbackWidget.test.tsx',
  'src/components/__tests__/GroupDropZone.test.tsx',
  'src/components/__tests__/LanguageSelector.test.tsx',
  'src/components/__tests__/Layout.test.tsx',
  'src/components/__tests__/LiveLogViewer.test.tsx',
  'src/components/__tests__/LoadingStates-overlays.test.tsx',
  'src/components/__tests__/LoadingStates.security.test.tsx',
  'src/components/__tests__/LogTable.test.tsx',
  'src/components/__tests__/ManualDNSChallenge.test.tsx',
  'src/components/__tests__/ProxyGroupBadge.test.tsx',
  'src/components/__tests__/ProxyHostDragHandle.test.tsx',
  'src/components/__tests__/ProxyHostForm-token-coverage.test.tsx',
  'src/components/__tests__/ProxyHostForm-uptime.test.tsx',
  'src/components/__tests__/ProxyHostForm.test.tsx',
  'src/components/__tests__/RedirectionHostForm-token-coverage.test.tsx',
  'src/components/__tests__/RemoteServerForm.test.tsx',
  'src/components/__tests__/SecurityHeaderProfileForm.test.tsx',
  'src/components/__tests__/SecurityNotificationSettingsModal.test.tsx',
  'src/components/__tests__/SecurityScoreDisplay.test.tsx',
  'src/components/__tests__/WebSocketStatusCard.test.tsx',
  'src/components/backups/__tests__/BackupEncryptionCard.test.tsx',
  'src/components/dialogs/WhatsNewModal.test.tsx',
  'src/components/dialogs/__tests__/CertificateExportDialog.test.tsx',
  'src/components/dialogs/__tests__/CertificateUploadDialog.test.tsx',
  'src/components/dialogs/__tests__/ImportSuccessModal.test.tsx',
  'src/components/hecate/__tests__/ConnectionTypeSelector.test.tsx',
  'src/components/hecate/__tests__/ProviderDevicePicker.test.tsx',
  'src/components/hecate/__tests__/TunnelStatusBadge.test.tsx',
  'src/components/stats/__tests__/TopHostsChart.test.tsx',
  'src/components/theme/__tests__/BannerCustomizer.test.tsx',
  'src/components/theme/__tests__/LogoCustomizer.test.tsx',
  'src/components/theme/__tests__/ThemePreviewOverlay.test.tsx',
  'src/components/theme/__tests__/UserThemeManager.test.tsx',
  'src/components/ui/__tests__/Alert.test.tsx',
  'src/components/ui/__tests__/DataTable.test.tsx',
  'src/components/ui/__tests__/FileDropZone.test.tsx',
  'src/components/ui/__tests__/Input.test.tsx',
  'src/components/ui/__tests__/Skeleton.test.tsx',
  'src/components/ui/__tests__/StatsCard.test.tsx',
  'src/pages/__tests__/AccessLists.test.tsx',
  'src/pages/__tests__/AuditLogs.test.tsx',
  'src/pages/__tests__/CrowdSecConfig.branchCoverage.test.tsx',
  'src/pages/__tests__/CrowdSecConfig.coverage.test.tsx',
  'src/pages/__tests__/CrowdSecConfig.spec.tsx',
  'src/pages/__tests__/DNS.test.tsx',
  'src/pages/__tests__/EncryptionManagement.test.tsx',
  'src/pages/__tests__/ImportCaddy-multifile-modal.test.tsx',
  'src/pages/__tests__/ImportCrowdSec.spec.tsx',
  'src/pages/__tests__/Login.overlay.audit.test.tsx',
  'src/pages/__tests__/Plugins.test.tsx',
  'src/pages/__tests__/ProxyHosts-bulk-acl.test.tsx',
  'src/pages/__tests__/ProxyHosts-bulk-apply-all-settings.test.tsx',
  'src/pages/__tests__/ProxyHosts-bulk-apply-progress.test.tsx',
  'src/pages/__tests__/ProxyHosts-bulk-apply.test.tsx',
  'src/pages/__tests__/ProxyHosts-bulk-delete.test.tsx',
  'src/pages/__tests__/ProxyHosts-cert-cleanup.test.tsx',
  'src/pages/__tests__/ProxyHosts-coverage-isolated.test.tsx',
  'src/pages/__tests__/ProxyHosts-coverage.test.tsx',
  'src/pages/__tests__/ProxyHosts-extra.test.tsx',
  'src/pages/__tests__/ProxyHosts-groups.test.tsx',
  'src/pages/__tests__/ProxyHosts-progress.test.tsx',
  'src/pages/__tests__/ProxyHosts.bulkApplyHeaders.test.tsx',
  'src/pages/__tests__/RemoteServers.test.tsx',
  'src/pages/__tests__/SMTPSettings.test.tsx',
  'src/pages/__tests__/Security.audit.test.tsx',
  'src/pages/__tests__/Security.functional.test.tsx',
  'src/pages/__tests__/Security.loading.test.tsx',
  'src/pages/__tests__/Security.test.tsx',
  'src/pages/__tests__/SecurityHeaders.test.tsx',
  'src/pages/__tests__/SystemSettings.test.tsx',
  'src/pages/__tests__/Uptime.spec.tsx',
  'src/pages/__tests__/Uptime.tcp-ux.test.tsx',
  'src/pages/__tests__/Uptime.test.tsx',
  'src/pages/__tests__/UsersPage.test.tsx',
  'src/pages/__tests__/WafConfig.spec.tsx',
];

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
      // React Compiler diagnostics ship with eslint-plugin-react-hooks; every
      // rule is an error so the cleared findings cannot regress.
      ...Object.fromEntries(
        Object.keys(reactHooks.configs['recommended-latest'].rules).map(
          name => [name, 'error']
        )
      ),

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

  // ── Legacy testing-library exemptions (GH #1853). Must follow the test-file block ──
  {
    files: LEGACY_TESTING_LIBRARY_FILES,
    rules: {
      'testing-library/no-node-access': 'off',
      'testing-library/no-container': 'off',
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
    ignores: ['package-lock.json', 'tsconfig*.json', 'src/locales/**'],
    language: 'json/json',
    plugins: { json },
    rules: {
      'json/no-duplicate-keys': 'error',
    },
  },

  // ── Locale files (i18n-json uses its own processor, so they are excluded above) ──
  {
    files: ['src/locales/**/*.json'],
    // eslint-plugin-i18n-json still calls context.getFilename(), removed in ESLint 10
    plugins: { 'i18n-json': fixupPluginRules(i18nJson) },
    processor: {
      meta: { name: '.json' },
      ...i18nJson.processors['.json'],
    },
    rules: {
      'i18n-json/valid-json': 'error',
      // i18next interpolates {{var}}, which is not ICU syntax
      'i18n-json/valid-message-syntax': ['error', { syntax: 'non-empty-string' }],
      // en is the source of truth. Missing keys are expected until Crowdin catches up
      // (i18next falls back to en), so identical-keys only warns; placeholders must always match.
      'i18n-json/identical-keys': [
        'warn',
        { filePath: path.resolve(import.meta.dirname, 'src/locales/en/translation.json') },
      ],
      'i18n-json/identical-placeholders': [
        'error',
        { filePath: path.resolve(import.meta.dirname, 'src/locales/en/translation.json') },
      ],
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

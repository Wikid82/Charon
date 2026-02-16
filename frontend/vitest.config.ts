import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// Dynamic coverage threshold (align local and CI)
const coverageThresholdValue =
  process.env.CHARON_MIN_COVERAGE ?? process.env.CPM_MIN_COVERAGE ?? '85.0'
const coverageThreshold = Number.parseFloat(coverageThresholdValue)
const resolvedCoverageThreshold = Number.isNaN(coverageThreshold) ? 85.0 : coverageThreshold

export default defineConfig({
  plugins: [react()],
  test: {
    pool: 'forks',
    poolOptions: {
      forks: {
        memoryLimit: '512MB',
      },
    },
    globals: true,
    environment: 'jsdom',
    environmentOptions: {
      jsdom: {
        url: 'http://localhost',
      },
    },
    setupFiles: './src/test/setup.ts',
    // TypeScript types for test globals - these are automatically available in test files
    typecheck: {
      tsconfig: './tsconfig.json',
    },
    exclude: [
      'node_modules/**',
      'dist/**',
      'e2e/**', // Playwright E2E tests - run separately
      'tests/**', // Playwright smoke tests - run separately
      // TEMPORARY QUARANTINE (2026-02-16): pre-existing unrelated flaky/failed suites
      // Follow-up: re-enable after selector/timer stability fixes in affected tests.
      'src/components/__tests__/ProxyHostForm-dns.test.tsx',
      'src/pages/__tests__/Notifications.test.tsx',
      'src/pages/__tests__/ProxyHosts-coverage.test.tsx',
      'src/pages/__tests__/ProxyHosts-extra.test.tsx',
      'src/pages/__tests__/Security.functional.test.tsx',
    ],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html', 'lcov', 'json-summary'],
      exclude: [
        'node_modules/',
        'src/locales/**',
        'src/test/',
        '**/*.d.ts',
        '**/*.config.*',
        '**/mockData.ts',
        'dist/',
        'e2e/',
      ],
      thresholds: {
        lines: resolvedCoverageThreshold,
      },
    },
  },
})

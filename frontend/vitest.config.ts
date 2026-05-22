import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// Dynamic coverage threshold (align local and CI)
const coverageThresholdValue =
  process.env.CHARON_MIN_COVERAGE ?? process.env.CPM_MIN_COVERAGE ?? '87.0'
const coverageThreshold = Number.parseFloat(coverageThresholdValue)
const resolvedCoverageThreshold = Number.isNaN(coverageThreshold) ? 87.0 : coverageThreshold
const coverageReportsDirectory = process.env.VITEST_COVERAGE_REPORTS_DIR ?? './coverage'

export default defineConfig({
  plugins: [react()],
  test: {
    pool: 'forks',
    maxWorkers: 1,
    minWorkers: 1,
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
    ],
    coverage: {
      provider: 'v8',
      clean: false,
      reportOnFailure: true,
      reportsDirectory: coverageReportsDirectory,
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

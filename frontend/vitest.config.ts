import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// Dynamic coverage threshold (align local and CI)
const coverageThresholdValue =
  process.env.CHARON_MIN_COVERAGE ?? process.env.CPM_MIN_COVERAGE ?? '88.0'
const coverageThreshold = Number.parseFloat(coverageThresholdValue)
const resolvedCoverageThreshold = Number.isNaN(coverageThreshold) ? 88.0 : coverageThreshold

export default defineConfig({
  plugins: [react()],
  test: {
    pool: 'threads',
    globals: true,
    environment: 'jsdom',
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
        functions: resolvedCoverageThreshold,
        branches: resolvedCoverageThreshold,
        statements: resolvedCoverageThreshold,
      },
    },
  },
})

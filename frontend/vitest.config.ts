import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

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
      reporter: ['text', 'json', 'html'],
      exclude: [
        'node_modules/',
        'src/test/',
        '**/*.d.ts',
        '**/*.config.*',
        '**/mockData.ts',
        'dist/',
        'e2e/',
      ],
    },
  },
})

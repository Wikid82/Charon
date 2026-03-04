// @ts-check
import { defineConfig, devices } from '@playwright/test';

/**
 * Standalone config for Caddy Import Debug tests
 * Runs without security-tests dependency for faster iteration
 */
export default defineConfig({
  testDir: './tests',
  testMatch: '**/caddy-import-{debug,gaps}.spec.ts',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: 0,
  workers: 1,
  reporter: [
   ['list'],
    ['html', { outputFolder: 'playwright-report/caddy-debug', open: 'never' }],
  ],
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL ||'http://localhost:8080',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    storageState: 'playwright/.auth/user.json', // Use existing auth state
  },

  projects: [
    {
      name: 'caddy-import-debug',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});

// @ts-check
import { defineConfig, devices } from '@playwright/test';
import { defineCoverageReporterConfig } from '@bgotink/playwright-coverage';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

/**
 * Read environment variables from file (local development only).
 * In CI, environment variables are provided by GitHub secrets.
 * https://github.com/motdotla/dotenv
 */
import dotenv from 'dotenv';
if (!process.env.CI) {
  dotenv.config({ path: join(dirname(fileURLToPath(import.meta.url)), '.env') });
}

/**
 * Auth state storage path - shared across all browser projects
 */
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const STORAGE_STATE = join(__dirname, 'playwright/.auth/user.json');

/**
 * Coverage reporter configuration for E2E tests
 * Only loaded when PLAYWRIGHT_COVERAGE=1
 */
const enableCoverage = process.env.PLAYWRIGHT_COVERAGE === '1';

const coverageReporterConfig = enableCoverage ? defineCoverageReporterConfig({
  sourceRoot: __dirname,
  exclude: [
    '**/node_modules/**',
    '**/playwright/**',
    '**/tests/**',
    '**/*.spec.ts',
    '**/*.spec.js',
    '**/*.test.ts',
    '**/coverage/**',
    '**/dist/**',
    '**/build/**',
  ],
  resultDir: join(__dirname, 'coverage/e2e'),
  reports: [
    ['html'],
    ['lcovonly', { file: 'lcov.info' }],
    ['json', { file: 'coverage.json' }],
    ['text-summary', { file: null }],
  ],
  watermarks: {
    statements: [50, 80],
    branches: [50, 80],
    functions: [50, 80],
    lines: [50, 80],
  },
  rewritePath: ({ absolutePath }) => {
    if (absolutePath.startsWith('/app/')) {
      return absolutePath.replace('/app/', `${__dirname}/`);
    }
    if (absolutePath.startsWith('/src/')) {
      return join(__dirname, 'frontend', absolutePath);
    }
    if (!absolutePath.startsWith('/') && !absolutePath.includes('/')) {
      return join(__dirname, 'frontend/src', absolutePath);
    }
    return absolutePath;
  },
}) : null;

/**
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: './tests',
  testIgnore: ['**/frontend/**', '**/node_modules/**', '**/backend/**'],

  /* Standard globalSetup - runs once before all tests */
  globalSetup: './tests/global-setup.ts',

  /* Timeouts */
  timeout: process.env.CI ? 60000 : 90000,
  expect: { timeout: 5000 },

  /* Parallelization */
  fullyParallel: true,
  workers: process.env.CI ? 1 : undefined,

  /* CI settings */
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,

  /* Reporters - simplified for CI */
  reporter: [
    process.env.CI ? ['github'] : ['list'],
    ['html', { open: process.env.CI ? 'never' : 'on-failure' }],
    ...(enableCoverage ? [['@bgotink/playwright-coverage', coverageReporterConfig]] : []),
  ],
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL Configuration
     *
     * CRITICAL: Authentication cookies are domain-scoped. The auth.setup.ts
     * stores cookies for the domain in this baseURL. TestDataManager and
     * browser tests must use the SAME domain for cookies to be sent.
     *
     * E2E tests verify UI/UX on the Charon management interface (port 8080).
     * Middleware enforcement is tested separately via integration tests (backend/integration/).
     * CI can override with PLAYWRIGHT_BASE_URL environment variable if needed.
     *
     * IMPORTANT: Using 127.0.0.1 (IPv4 loopback) instead of localhost to avoid
     * IPv6/IPv4 resolution issues where Node.js/Playwright might prefer ::1 (IPv6)
     * but the Docker container binds to 0.0.0.0 (IPv4).
     */
    baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080',

    /* Traces: Capture execution traces for debugging
     *
     * Options:
     *   'off'              - No trace capture
     *   'on'               - Always capture (large files, use only for debugging)
     *   'on-first-retry'   - Capture on first retry only (good balance)
     *   'retain-on-failure'- Capture only for failed tests (smallest overhead)
     */
    trace: 'on-first-retry',

    /* Videos: Capture video recordings for visual debugging
     *
     * Options:
     *   'off'              - No recording
     *   'on'               - Always record (high disk usage)
     *   'retain-on-failure'- Record only failed tests (recommended)
     */
    video: 'retain-on-failure',

    /* Screenshots: Capture screenshots of page state
     *
     * Options:
     *   'off'              - No screenshots
     *   'only-on-failure'  - Screenshot on failure (recommended)
     *   'on'               - Always screenshot (high disk usage)
     */
    screenshot: 'only-on-failure',
  },

  /* Configure projects for major browsers */
  projects: [
    // Setup project - authentication (runs FIRST)
    {
      name: 'setup',
      testMatch: /auth\.setup\.ts/,
    },

    // Security Tests - Run WITH security enabled (SEQUENTIAL, Chromium only)
    {
      name: 'security-tests',
      testDir: './tests',
      testMatch: [
        /security-enforcement\/.*\.spec\.(ts|js)/,
        /security\/.*\.spec\.(ts|js)/,
      ],
      dependencies: ['setup'],
      teardown: 'security-teardown',
      fullyParallel: false,
      workers: 1,
      use: {
        ...devices['Desktop Chrome'],
        headless: true,
        storageState: STORAGE_STATE,
      },
    },

    // Security Teardown - Disable ALL security modules
    {
      name: 'security-teardown',
      testMatch: /security-teardown\.setup\.ts/,
    },

    // Browser projects - standard Playwright pattern
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        storageState: STORAGE_STATE,
      },
      dependencies: ['setup'],
    },

    {
      name: 'firefox',
      use: {
        ...devices['Desktop Firefox'],
        storageState: STORAGE_STATE,
      },
      dependencies: ['setup', 'security-tests'],
    },

    {
      name: 'webkit',
      use: {
        ...devices['Desktop Safari'],
        storageState: STORAGE_STATE,
      },
      dependencies: ['setup', 'security-tests'],
    },

    /* Test against mobile viewports. */
    // {
    //   name: 'Mobile Chrome',
    //   use: { ...devices['Pixel 5'] },
    // },
    // {
    //   name: 'Mobile Safari',
    //   use: { ...devices['iPhone 12'] },
    // },

    /* Test against branded browsers. */
    // {
    //   name: 'Microsoft Edge',
    //   use: { ...devices['Desktop Edge'], channel: 'msedge' },
    // },
    // {
    //   name: 'Google Chrome',
    //   use: { ...devices['Desktop Chrome'], channel: 'chrome' },
    // },
  ],

  /* Run your local dev server before starting the tests */
  // webServer: {
  //   command: 'cd frontend && npm run dev',
  //   url: 'http://localhost:5173',
  //   reuseExistingServer: !process.env.CI,
  //   timeout: 120000,
  //   stdout: 'pipe',  // PHASE 1: Enable log visibility
  //   stderr: 'pipe',  // PHASE 1: Enable log visibility
  // },
});

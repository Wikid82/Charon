// @ts-check
import { defineConfig, devices } from '@playwright/test';
import { defineCoverageReporterConfig } from '@bgotink/playwright-coverage';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
import dotenv from 'dotenv';
dotenv.config({ path: join(dirname(fileURLToPath(import.meta.url)), '.env') });

/**
 * Auth state storage path - shared across all browser projects
 */
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const STORAGE_STATE = join(__dirname, 'playwright/.auth/user.json');

/**
 * Coverage reporter configuration for E2E tests
 * Tracks V8 coverage during Playwright test execution
 */
const coverageReporterConfig = defineCoverageReporterConfig({
  // Root directory for source file resolution
  sourceRoot: __dirname,

  // Exclude non-application code from coverage
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

  // Output directory for coverage reports
  resultDir: join(__dirname, 'coverage/e2e'),

  // Generate multiple report formats
  reports: [
    // HTML report for visual inspection
    ['html'],
    // LCOV for Codecov upload
    ['lcovonly', { file: 'lcov.info' }],
    // JSON for programmatic access
    ['json', { file: 'coverage.json' }],
    // Text summary in console
    ['text-summary', { file: null }],
  ],

  // Coverage watermarks (visual thresholds in HTML report)
  watermarks: {
    statements: [50, 80],
    branches: [50, 80],
    functions: [50, 80],
    lines: [50, 80],
  },
  // Path rewriting for source file resolution
  rewritePath: ({ absolutePath, relativePath }) => {
    // Handle paths from Docker container
    if (absolutePath.startsWith('/app/')) {
      return absolutePath.replace('/app/', `${__dirname}/`);
    }

    // Handle Vite dev server paths (relative to frontend/src)
    // Vite serves files like "/src/components/Button.tsx"
    if (absolutePath.startsWith('/src/')) {
      return join(__dirname, 'frontend', absolutePath);
    }

    // If path doesn't start with /, prepend frontend/src
    if (!absolutePath.startsWith('/') && !absolutePath.includes('/')) {
      // Bare filenames like "Button.tsx" - try to resolve to frontend/src
      return join(__dirname, 'frontend/src', absolutePath);
    }

    return absolutePath;
  },
});

const enableCoverage = process.env.PLAYWRIGHT_COVERAGE === '1';

/**
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: './tests',
  /* Global setup - runs once before all tests to clean up orphaned data */
  globalSetup: './tests/global-setup.ts',
  /* Global timeout for each test */
  timeout: 30000,
  /* Timeout for expect() assertions */
  expect: {
    timeout: 5000,
  },
  /* Run tests in files in parallel */
  fullyParallel: true,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!process.env.CI,
  /* Retry on CI only */
  retries: process.env.CI ? 2 : 0,
  /* Opt out of parallel tests on CI. */
  workers: process.env.CI ? 1 : undefined,
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: [
    ...(process.env.CI ? [['blob'], ['github']] : [['list']]),
    ['html', { open: process.env.CI ? 'never' : 'on-failure' }],
    ...(enableCoverage ? [['@bgotink/playwright-coverage', coverageReporterConfig]] : []),
    ['./tests/reporters/debug-reporter.ts'],
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
     */
    baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080',

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
    // 1. Setup project - authentication (runs FIRST)
    {
      name: 'setup',
      testMatch: /auth\.setup\.ts/,
    },

    // 2. Security Tests - Run WITH security enabled (SEQUENTIAL, headless Chromium)
    // These tests enable security modules, verify enforcement, then teardown disables all.
    {
      name: 'security-tests',
      testDir: './tests',
      testMatch: [
        /security-enforcement\/.*\.spec\.(ts|js)/,
        /security\/.*\.spec\.(ts|js)/,
      ],
      dependencies: ['setup'],
      teardown: 'security-teardown',
      fullyParallel: false, // Force sequential - modules share state
      workers: 1, // Force single worker to prevent race conditions on security settings
      use: {
        ...devices['Desktop Chrome'],
        headless: true, // Security tests are API-level, don't need headed
        storageState: STORAGE_STATE,
      },
    },

    // 3. Security Teardown - Disable ALL security modules after security-tests
    {
      name: 'security-teardown',
      testMatch: /security-teardown\.setup\.ts/,
    },

    // 4. Browser projects - Depend on setup and security-tests (with teardown) for order
    // Note: Security modules are re-disabled by teardown before these projects execute
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        // Use stored authentication state
        storageState: STORAGE_STATE,
      },
      dependencies: ['setup', 'security-tests'],
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
  // },
});

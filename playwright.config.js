// @ts-check
import { defineConfig, devices } from '@playwright/test';
import { defineCoverageReporterConfig } from '@bgotink/playwright-coverage';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
// import dotenv from 'dotenv';
// dotenv.config({ path: join(dirname(fileURLToPath(import.meta.url)), '.env') });

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

  // Coverage threshold enforcement
  check: {
    global: {
      statements: 85,
      branches: 85,
      functions: 85,
      lines: 85,
    },
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
  reporter: process.env.CI
    ? [
        ['github'],
        ['html', { open: 'never' }],
        ['@bgotink/playwright-coverage', coverageReporterConfig],
      ]
    : [
        ['list'],
        ['html', { open: 'on-failure' }],
        ['@bgotink/playwright-coverage', coverageReporterConfig],
      ],
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL Configuration
     *
     * CRITICAL: Authentication cookies are domain-scoped. The auth.setup.ts
     * stores cookies for the domain in this baseURL. TestDataManager and
     * browser tests must use the SAME domain for cookies to be sent.
     *
     * For local testing, always use http://localhost:8080 (not IP addresses).
     * CI sets PLAYWRIGHT_BASE_URL=http://localhost:8080 automatically.
     */
    baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080',

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',
  },

  /* Configure projects for major browsers */
  projects: [
    // 1. Setup project - authentication (runs FIRST)
    {
      name: 'setup',
      testMatch: /auth\.setup\.ts/,
    },

    // 2. Security Tests - Run WITH security enabled (SEQUENTIAL, headless Chromium)
    // These tests enable security modules, verify blocking behavior, then teardown disables all.
    {
      name: 'security-tests',
      testDir: './tests/security-enforcement',
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

    // 4. Browser projects - Depend on setup and security-tests
    // Note: Browser projects run AFTER security-tests complete (and its teardown runs)
    // This ordering ensures security modules are disabled before browser tests run.
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        // Use stored authentication state
        storageState: STORAGE_STATE,
      },
      testIgnore: /security-enforcement\//,
      dependencies: ['setup', 'security-tests'],
    },

    {
      name: 'firefox',
      use: {
        ...devices['Desktop Firefox'],
        storageState: STORAGE_STATE,
      },
      testIgnore: /security-enforcement\//,
      dependencies: ['setup', 'security-tests'],
    },

    {
      name: 'webkit',
      use: {
        ...devices['Desktop Safari'],
        storageState: STORAGE_STATE,
      },
      testIgnore: /security-enforcement\//,
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

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
 * Enabled by default, disable with PLAYWRIGHT_COVERAGE=0
 */
const enableCoverage = process.env.PLAYWRIGHT_COVERAGE !== '0';
const resolvedBaseURL = process.env.PLAYWRIGHT_BASE_URL || (enableCoverage ? 'http://localhost:5173' : 'http://127.0.0.1:8080');
if (!process.env.PLAYWRIGHT_BASE_URL) {
  process.env.PLAYWRIGHT_BASE_URL = resolvedBaseURL;
}
// Skip security-test dependencies by default to avoid running them as a
// prerequisite for non-security test runs. Set PLAYWRIGHT_SKIP_SECURITY_DEPS=0
// to restore the legacy dependency behavior when needed.
const skipSecurityDeps = process.env.PLAYWRIGHT_SKIP_SECURITY_DEPS !== '0';
const browserDependencies = skipSecurityDeps ? ['setup'] : ['setup', 'security-tests'];

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

// Preflight: when the Playwright UI is requested on a headless Linux machine,
// attempt to start an Xvfb instance automatically (developer convenience).
// - If Xvfb is not available, fail with a clear, actionable message.
// - In CI we avoid auto-starting; CI should either use the project's E2E Docker
//   image or run tests in headless mode.
if (process.argv.includes('--ui')) {
  if (process.env.CI) {
    // In CI, running the interactive UI is unsupported — provide guidance.
    throw new Error(
      "Playwright UI (--ui) is not supported in CI.\n" +
      "Use the project's E2E Docker image or run tests headless: `npm run e2e`"
    );
  }

  if (!process.env.DISPLAY) {
    try {
      // Use child_process to probe for Xvfb and start it if present.
      const { spawnSync, spawn } = await import('child_process');
      const probe = spawnSync('Xvfb', ['-version']);
      if (probe.error) throw probe.error;

      // Start Xvfb on :99 and detach so it survives after the spawn call.
      const xvfb = spawn('Xvfb', [':99', '-screen', '0', '1280x720x24'], {
        detached: true,
        stdio: 'ignore',
      });
      xvfb.unref();
      process.env.DISPLAY = ':99';
      // eslint-disable-next-line no-console
      console.log('Started Xvfb on :99 to support Playwright UI (auto-start).');
    } catch (err) {
      throw new Error(
        'Playwright UI requires an X server but none was found.\n' +
        "Options:\n" +
        "  1) Install Xvfb and retry (Debian/Ubuntu: `sudo apt install xvfb`)\n" +
        "  2) Run the UI under Xvfb: `xvfb-run --auto-servernum npx playwright test --ui`\n" +
        "  3) Run headless tests: `npm run e2e`\n\n" +
        "See docs/development/running-e2e.md for details.\n" +
        `Underlying error: ${err && err.message ? err.message : err}`
      );
    }
  }
}

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
     * Coverage mode (enableCoverage=true):
     *   - Vite dev server automatically started via webServer block
     *   - Uses http://localhost:5173 (source maps for V8 coverage)
     *   - PLAYWRIGHT_BASE_URL override still respected
     *
    * Non-coverage mode (enableCoverage=false):
     *   - Tests run against Docker container (faster, no coverage)
    *   - Uses http://127.0.0.1:8080 (IPv4 loopback to avoid ::1 connection flakiness)
     *   - PLAYWRIGHT_BASE_URL override still respected
     *
     * CRITICAL: Authentication cookies are domain-scoped. The auth.setup.ts
     * stores cookies for the domain in this baseURL. TestDataManager and
     * browser tests must use the SAME domain for cookies to be sent.
     *
    * Cookie domain note:
    * Cookies are domain-scoped. Auth setup and browser/API contexts must use
    * the same resolved base URL host.
     *
     * E2E tests verify UI/UX on the Charon management interface.
     * Middleware enforcement is tested separately via integration tests (backend/integration/).
     *
     * For remote SSH development, use PLAYWRIGHT_BASE_URL with your Tailscale IP:
     *   export PLAYWRIGHT_BASE_URL=http://100.98.12.109:9323
     *   npx playwright test --ui
     */
    baseURL: resolvedBaseURL,

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

  /* Run your local dev server before starting the tests */
  webServer: enableCoverage ? {
    command: 'cd frontend && npm run dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
    timeout: 120000,
    stdout: 'pipe',
    stderr: 'pipe',
  } : undefined,

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
    // Conditionally disabled when skipSecurityDeps is true
    // When disabled, tests cannot find this project and it won't run
    {
      name: 'security-teardown',
      testMatch: skipSecurityDeps ? [] : /security-teardown\.setup\.ts/,
    },

    // Browser projects - standard Playwright pattern
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        storageState: STORAGE_STATE,
      },
      dependencies: browserDependencies,
      testIgnore: ['**/frontend/**', '**/node_modules/**', '**/backend/**', '**/security-enforcement/**', '**/security/**'],
    },

    {
      name: 'firefox',
      use: {
        ...devices['Desktop Firefox'],
        storageState: STORAGE_STATE,
      },
      dependencies: browserDependencies,
      testIgnore: ['**/frontend/**', '**/node_modules/**', '**/backend/**', '**/security-enforcement/**', '**/security/**'],
    },

    {
      name: 'webkit',
      use: {
        ...devices['Desktop Safari'],
        storageState: STORAGE_STATE,
      },
      dependencies: browserDependencies,
      testIgnore: ['**/frontend/**', '**/node_modules/**', '**/backend/**', '**/security-enforcement/**', '**/security/**'],
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

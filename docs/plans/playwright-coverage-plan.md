# Playwright E2E Coverage Integration Plan

**Date:** January 18, 2026
**Status:** 🔄 In Progress - Requires Vite Dev Server for source coverage
**Priority:** High - Enables coverage visibility for E2E tests
**Objective:** Integrate `@bgotink/playwright-coverage` to track frontend code coverage during E2E tests

---

## ⚠️ CRITICAL ARCHITECTURE NOTE

**The original plan assumed V8 coverage would work against Docker production builds. This is INCORRECT.**

### Why Docker Build Coverage Doesn't Work

1. **V8 coverage** captures execution data for the bundled/minified JS served by Docker
2. **Source maps** in Docker container map to paths like `/app/frontend/src/...`
3. These source files **don't exist on the test host** (they're inside the container)
4. The coverage reporter can't resolve paths → 0/0 coverage

### Correct Approach: Vite Dev Server

For accurate source-level coverage:

1. **Backend**: Docker container at `localhost:8080` (unchanged)
2. **Frontend**: Vite dev server at `localhost:3000` (`npm run dev`)
3. **Coverage tests**: Hit `localhost:3000` (Vite proxies API to Docker)

**Why this works:**
- Vite serves actual `.tsx`/`.ts` files (not bundled)
- Source files exist on disk at project root
- V8 coverage maps directly to source without path rewriting
- Accurate line-level coverage for Codecov

---

## Table of Contents

1. [Overview](#1-overview)
2. [Current State Analysis](#2-current-state-analysis)
3. [Installation Steps](#3-installation-steps)
4. [Configuration Changes](#4-configuration-changes)
5. [Test File Modifications](#5-test-file-modifications)
6. [CI/CD Integration](#6-cicd-integration)
7. [Coverage Threshold Enforcement](#7-coverage-threshold-enforcement)
8. [Implementation Checklist](#8-implementation-checklist)
9. [Risks and Mitigations](#9-risks-and-mitigations)
10. [References](#10-references)

---

## 1. Overview

### What is `@bgotink/playwright-coverage`?

`@bgotink/playwright-coverage` is a Playwright reporter that tracks JavaScript code coverage using V8 coverage without requiring any instrumentation. It:

- Hooks into Playwright's Page objects to track V8 coverage
- Collects coverage data as test attachments
- Merges coverage from all tests into Istanbul format
- Generates reports in HTML, LCOV, JSON, and other Istanbul formats

### Why Integrate E2E Coverage?

- **Visibility**: Understand which frontend code paths are exercised by E2E tests
- **Gap Analysis**: Identify untested user journeys and critical paths
- **Quality Gate**: Ensure new features have E2E test coverage
- **Codecov Integration**: Unified coverage view across unit and E2E tests

### Package Details

| Property | Value |
|----------|-------|
| Package | `@bgotink/playwright-coverage` |
| Version | `0.3.2` (latest) |
| License | MIT |
| NPM Weekly Downloads | ~5,300 |
| Playwright Compatibility | ≥1.40.0 |

---

## 2. Current State Analysis

### Existing Playwright Setup

**Configuration File:** [playwright.config.js](../../playwright.config.js)

```javascript
// Current reporter configuration
reporter: process.env.CI
  ? [['github'], ['html', { open: 'never' }]]
  : [['list'], ['html', { open: 'on-failure' }]],
```

**Current Test Imports:**
```typescript
// Tests currently import directly from @playwright/test
import { test, expect } from '@playwright/test';
```

**package.json Dependencies:**
```json
{
  "devDependencies": {
    "@playwright/test": "^1.57.0",
    "@types/node": "^25.0.9"
  }
}
```

### Existing CI Workflows

Two workflows handle Playwright tests:

1. **[playwright.yml](../../.github/workflows/playwright.yml)** - Runs on workflow_run after Docker build
2. **[e2e-tests.yml](../../.github/workflows/e2e-tests.yml)** - Runs with sharding on PR/push

### Existing Test Structure

```
tests/
├── auth.setup.ts                 # Uses @playwright/test
├── core/                         # Feature tests
├── dns-provider-*.spec.ts        # Use @playwright/test
├── manual-dns-provider.spec.ts   # Uses @playwright/test
├── fixtures/                     # Shared fixtures
└── utils/                        # Helper utilities
```

### Codecov Integration

Current coverage uploads exist in [codecov-upload.yml](../../.github/workflows/codecov-upload.yml):
- `backend` flag for Go coverage
- `frontend` flag for Vitest/unit test coverage
- **Missing:** E2E coverage flag

---

## 3. Installation Steps

### Step 1: Install the Package

```bash
npm install -D @bgotink/playwright-coverage
```

### Step 2: Verify Dependencies

The package requires:
- Playwright ≥1.40.0 ✅ (Current: 1.57.0)
- Node.js ≥18 ✅ (Current: using LTS)

### Step 3: Update package.json

After installation, `package.json` should have:

```json
{
  "devDependencies": {
    "@bgotink/playwright-coverage": "^0.3.2",
    "@playwright/test": "^1.57.0",
    "@types/node": "^25.0.9",
    "markdownlint-cli2": "^0.20.0"
  }
}
```

---

## 4. Configuration Changes

### 4.1 playwright.config.js Modifications

Replace the current configuration with coverage-enabled version:

```javascript
// @ts-check
import { defineConfig, devices } from '@playwright/test';
import { defineCoverageReporterConfig } from '@bgotink/playwright-coverage';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const STORAGE_STATE = join(__dirname, 'playwright/.auth/user.json');

// Coverage reporter configuration
const coverageReporterConfig = defineCoverageReporterConfig({
  // Root directory for source file resolution
  sourceRoot: __dirname,

  // Exclude non-application code from coverage
  exclude: [
    '**/node_modules/**',
    '**/playwright/**',
    '**/tests/**',
    '**/*.spec.ts',
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

  // Path rewriting for Docker/CI environments
  rewritePath: ({ absolutePath, relativePath }) => {
    // Handle paths from Docker container
    if (absolutePath.startsWith('/app/')) {
      return absolutePath.replace('/app/', `${__dirname}/`);
    }
    return absolutePath;
  },
});

/**
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: './tests',
  timeout: 30000,
  expect: {
    timeout: 5000,
  },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,

  // Updated reporter configuration with coverage
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

  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080',
    trace: 'on-first-retry',
  },

  projects: [
    {
      name: 'setup',
      testMatch: /auth\.setup\.ts/,
    },
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
      dependencies: ['setup'],
    },
    {
      name: 'webkit',
      use: {
        ...devices['Desktop Safari'],
        storageState: STORAGE_STATE,
      },
      dependencies: ['setup'],
    },
  ],
});
```

### 4.2 Add Coverage Directory to .gitignore

Ensure coverage outputs are not committed:

```gitignore
# E2E Coverage
coverage/e2e/
```

---

## 5. Test File Modifications

### 5.1 Create Shared Test Fixture with Coverage

Create a base test fixture that wraps `@bgotink/playwright-coverage`:

```typescript
// tests/fixtures/coverage-test.ts

import { test as coverageTest, expect } from '@bgotink/playwright-coverage';
import { mergeTests } from '@playwright/test';
import { test as authTest } from './auth-fixtures';

// Merge coverage tracking with auth fixtures
export const test = mergeTests(coverageTest, authTest);
export { expect };
```

### 5.2 Update Test Files to Use Coverage-Enabled Test

**Option A: Update Each Test File (Recommended for gradual rollout)**

```typescript
// Before
import { test, expect } from '@playwright/test';

// After
import { test, expect } from '@bgotink/playwright-coverage';
```

**Option B: Use Merged Fixture (Recommended for projects with custom fixtures)**

```typescript
// Before
import { test, expect } from '../fixtures/auth-fixtures';

// After
import { test, expect } from '../fixtures/coverage-test';
```

### 5.3 Update auth.setup.ts

```typescript
// tests/auth.setup.ts

// Use coverage-enabled test for setup
import { test as setup, expect } from '@bgotink/playwright-coverage';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

// ... rest of the file remains the same
```

### 5.4 Files Requiring Updates

The following test files need import changes:

| File | Current Import | New Import |
|------|----------------|------------|
| `tests/auth.setup.ts` | `@playwright/test` | `@bgotink/playwright-coverage` |
| `tests/manual-dns-provider.spec.ts` | `@playwright/test` | `@bgotink/playwright-coverage` |
| `tests/dns-provider-crud.spec.ts` | `@playwright/test` | `@bgotink/playwright-coverage` |
| `tests/dns-provider-types.spec.ts` | `@playwright/test` | `@bgotink/playwright-coverage` |
| `tests/example.spec.js` | `@playwright/test` | `@bgotink/playwright-coverage` |
| All files in `tests/core/` | `@playwright/test` | `@bgotink/playwright-coverage` |
| All files in `tests/fixtures/` | `@playwright/test` | `@bgotink/playwright-coverage` |

---

## 6. CI/CD Integration

### 6.1 Update Skill Script

Create or update the skill script for E2E tests with coverage:

**File:** `.github/skills/scripts/test-e2e-playwright-coverage.sh`

```bash
#!/usr/bin/env bash
# test-e2e-playwright-coverage.sh
# Run Playwright E2E tests with coverage collection

set -euo pipefail

# Source helpers
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/_logging_helpers.sh"
source "${SCRIPT_DIR}/_error_handling_helpers.sh"

log_info "🎭 Running Playwright E2E tests with coverage..."

# Ensure coverage directory exists
mkdir -p coverage/e2e

# Run Playwright tests
PLAYWRIGHT_HTML_OPEN=never npx playwright test --project=chromium

# Check if coverage was generated
if [[ -f "coverage/e2e/lcov.info" ]]; then
    log_success "✅ E2E coverage generated: coverage/e2e/lcov.info"

    # Print summary
    if [[ -f "coverage/e2e/coverage.json" ]]; then
        log_info "📊 Coverage Summary:"
        cat coverage/e2e/coverage.json | jq '.total'
    fi
else
    log_warning "⚠️ No coverage data generated (tests may have failed)"
fi
```

### 6.2 Update e2e-tests.yml Workflow

Add coverage upload step to the existing workflow:

```yaml
# .github/workflows/e2e-tests.yml - Add after test execution

      - name: Run E2E tests (Shard ${{ matrix.shard }}/${{ matrix.total-shards }})
        run: |
          npx playwright test \
            --project=${{ matrix.browser }} \
            --shard=${{ matrix.shard }}/${{ matrix.total-shards }} \
            --reporter=html,json,github
        env:
          PLAYWRIGHT_BASE_URL: http://localhost:8080
          CI: true
          TEST_WORKER_INDEX: ${{ matrix.shard }}

      # NEW: Upload E2E coverage
      - name: Upload E2E coverage artifact
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: e2e-coverage-shard-${{ matrix.shard }}
          path: coverage/e2e/
          retention-days: 7
```

### 6.3 Add Coverage Merge Job

Add a job to merge sharded coverage and upload to Codecov:

```yaml
  # Add after merge-reports job
  upload-coverage:
    name: Upload E2E Coverage
    runs-on: ubuntu-latest
    needs: e2e-tests
    if: always() && needs.e2e-tests.result == 'success'

    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: ${{ env.NODE_VERSION }}
          cache: 'npm'

      - name: Download all coverage artifacts
        uses: actions/download-artifact@v4
        with:
          pattern: e2e-coverage-*
          path: all-coverage
          merge-multiple: false

      - name: Merge LCOV coverage files
        run: |
          # Install lcov for merging
          sudo apt-get update && sudo apt-get install -y lcov

          # Create merged coverage directory
          mkdir -p coverage/e2e-merged

          # Find all lcov.info files and merge them
          LCOV_FILES=$(find all-coverage -name "lcov.info" -type f)

          if [[ -n "$LCOV_FILES" ]]; then
            # Build merge command
            MERGE_ARGS=""
            for file in $LCOV_FILES; do
              MERGE_ARGS="$MERGE_ARGS -a $file"
            done

            lcov $MERGE_ARGS -o coverage/e2e-merged/lcov.info
            echo "✅ Merged $(echo "$LCOV_FILES" | wc -w) coverage files"
          else
            echo "⚠️ No coverage files found to merge"
            exit 0
          fi

      - name: Upload E2E coverage to Codecov
        uses: codecov/codecov-action@v5
        with:
          token: ${{ secrets.CODECOV_TOKEN }}
          files: ./coverage/e2e-merged/lcov.info
          flags: e2e
          name: e2e-coverage
          fail_ci_if_error: false  # Don't fail build on upload error

      - name: Upload merged coverage artifact
        uses: actions/upload-artifact@v4
        with:
          name: e2e-coverage-merged
          path: coverage/e2e-merged/
          retention-days: 30
```

### 6.4 Update codecov.yml Configuration

Add E2E flag configuration:

```yaml
# codecov.yml

coverage:
  status:
    project:
      default:
        target: auto
        threshold: 1%
    patch:
      default:
        target: 100%

flags:
  backend:
    paths:
      - backend/
    carryforward: true

  frontend:
    paths:
      - frontend/
    carryforward: true

  # NEW: E2E coverage flag
  e2e:
    paths:
      - frontend/  # E2E tests cover frontend code
    carryforward: true

component_management:
  individual_components:
    - component_id: backend
      paths:
        - backend/**
    - component_id: frontend
      paths:
        - frontend/**
    - component_id: e2e
      paths:
        - frontend/**
```

---

## 7. Coverage Threshold Enforcement

### 7.1 Phase 1: Visibility Only (Current)

Initially, collect coverage without failing builds:

```javascript
// playwright.config.js - No threshold enforcement
// Coverage reporter only generates reports
```

### 7.2 Phase 2: Add Thresholds (After Baseline Established)

Once baseline coverage is established (after ~2 weeks of data), add thresholds:

```javascript
// Update playwright.config.js

import { execSync } from 'child_process';

// Optional: Fail if coverage drops below threshold
const coverageReporterConfig = defineCoverageReporterConfig({
  // ... existing config ...

  // Add coverage check (Phase 2)
  onEnd: async (coverageResults) => {
    const summary = coverageResults.total;

    // Define minimum thresholds
    const thresholds = {
      statements: 40,
      branches: 30,
      functions: 40,
      lines: 40,
    };

    let failed = false;
    const failures = [];

    for (const [metric, threshold] of Object.entries(thresholds)) {
      const actual = summary[metric].pct;
      if (actual < threshold) {
        failed = true;
        failures.push(`${metric}: ${actual.toFixed(1)}% < ${threshold}%`);
      }
    }

    if (failed && process.env.ENFORCE_COVERAGE === 'true') {
      console.error('\n❌ E2E Coverage thresholds not met:');
      failures.forEach(f => console.error(`  - ${f}`));
      process.exit(1);
    }
  },
});
```

### 7.3 Phase 3: Strict Enforcement (Future)

After comprehensive E2E coverage is achieved:

```yaml
# .github/workflows/e2e-tests.yml

      - name: Run E2E tests with coverage enforcement
        run: npx playwright test --project=chromium
        env:
          ENFORCE_COVERAGE: 'true'  # Enable threshold enforcement
```

---

## 8. Implementation Checklist

### Phase 1: Installation and Configuration

- [ ] Install `@bgotink/playwright-coverage` package
- [ ] Update `playwright.config.js` with coverage reporter
- [ ] Add `coverage/e2e/` to `.gitignore`
- [ ] Create coverage output directory structure

### Phase 2: Test File Updates

- [ ] Update `tests/auth.setup.ts` to use coverage-enabled test
- [ ] Update `tests/manual-dns-provider.spec.ts`
- [ ] Update `tests/dns-provider-crud.spec.ts`
- [ ] Update `tests/dns-provider-types.spec.ts`
- [ ] Update all files in `tests/core/`
- [ ] Update `tests/fixtures/auth-fixtures.ts`
- [ ] Create `tests/fixtures/coverage-test.ts` (merged fixture)

### Phase 3: CI/CD Integration

- [ ] Create `test-e2e-playwright-coverage.sh` skill script
- [ ] Update `.github/workflows/e2e-tests.yml` with coverage upload
- [ ] Add coverage merge job to workflow
- [ ] Update `codecov.yml` with `e2e` flag
- [ ] Test workflow on feature branch

### Phase 4: Validation

- [ ] Run tests locally and verify coverage generated
- [ ] Verify coverage appears in Codecov dashboard
- [ ] Verify HTML report is viewable
- [ ] Verify LCOV format is valid

### Phase 5: Documentation

- [ ] Update `docs/plans/current_spec.md` with coverage integration
- [ ] Add coverage information to `CONTRIBUTING.md`
- [ ] Document how to view local coverage reports

---

## 9. Risks and Mitigations

### Risk 1: Performance Impact

**Risk:** Coverage collection may slow down test execution.

**Mitigation:**
- V8 coverage is native and has minimal overhead (~5-10%)
- Coverage is only collected in CI, not blocking local development
- Monitor CI run times after integration

### Risk 2: Incomplete Coverage Data

**Risk:** Coverage may not capture all executed code paths.

**Mitigation:**
- Ensure all test files use the coverage-enabled `test` function
- Verify source maps are correctly configured in frontend build
- Use `rewritePath` option to handle Docker path differences

### Risk 3: Sharding Coverage Merge Issues

**Risk:** Sharded test runs may produce incomplete merged coverage.

**Mitigation:**
- Use `lcov` tool for reliable LCOV merging
- Verify merged coverage includes all shards
- Add validation step to check coverage completeness

### Risk 4: Codecov Upload Failures

**Risk:** Coverage uploads may fail intermittently.

**Mitigation:**
- Set `fail_ci_if_error: false` initially
- Archive coverage artifacts for manual inspection
- Add retry logic if needed

### Risk 5: Package Stability

**Risk:** `@bgotink/playwright-coverage` is marked as "experimental".

**Mitigation:**
- Package has been stable since 2019 with regular updates
- Pin to specific version in `package.json`
- Have fallback plan to remove if issues arise

---

## 10. References

- [bgotink/playwright-coverage GitHub](https://github.com/bgotink/playwright-coverage)
- [NPM Package](https://www.npmjs.com/package/@bgotink/playwright-coverage)
- [Playwright Test Configuration](https://playwright.dev/docs/test-configuration)
- [Istanbul Report Formats](https://istanbul.js.org/docs/advanced/alternative-reporters/)
- [Codecov Flags Documentation](https://docs.codecov.com/docs/flags)
- [LCOV Format Specification](https://ltp.sourceforge.net/coverage/lcov/geninfo.1.php)

---

## Appendix A: Quick Start Commands

```bash
# Install package
npm install -D @bgotink/playwright-coverage

# Run tests locally with coverage
npx playwright test --project=chromium

# View coverage HTML report
open coverage/e2e/index.html

# Run specific test file with coverage
npx playwright test tests/manual-dns-provider.spec.ts

# Generate only LCOV (for CI)
npx playwright test --project=chromium --reporter=@bgotink/playwright-coverage
```

## Appendix B: Troubleshooting

### Coverage is Empty

**Cause:** Tests are using `@playwright/test` instead of `@bgotink/playwright-coverage`.

**Solution:** Update imports in all test files:
```typescript
import { test, expect } from '@bgotink/playwright-coverage';
```

### Source Files Not Found in Report

**Cause:** Path mismatch between test environment and source files.

**Solution:** Configure `rewritePath` in coverage reporter config:
```javascript
rewritePath: ({ absolutePath }) => {
  return absolutePath.replace('/app/', process.cwd() + '/');
},
```

### LCOV Merge Fails

**Cause:** Missing `lcov` tool or malformed LCOV files.

**Solution:** Install lcov and validate files:
```bash
sudo apt-get install lcov
lcov --version
head -20 coverage/e2e/lcov.info
```

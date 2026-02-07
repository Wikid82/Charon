---
# agentskills.io specification v1.0
name: "test-e2e-playwright-coverage"
version: "1.0.0"
description: "Run Playwright E2E tests with code coverage collection using @bgotink/playwright-coverage"
author: "Charon Project"
license: "MIT"
tags:
  - "testing"
  - "e2e"
  - "playwright"
  - "coverage"
  - "integration"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "node"
    version: ">=18.0"
    optional: false
  - name: "npx"
    version: ">=1.0"
    optional: false
environment_variables:
  - name: "PLAYWRIGHT_BASE_URL"
    description: "Base URL of the Charon application under test"
    default: "http://localhost:8080"
    required: false
  - name: "PLAYWRIGHT_HTML_OPEN"
    description: "Controls HTML report auto-open behavior (set to 'never' for CI/non-interactive)"
    default: "never"
    required: false
  - name: "CI"
    description: "Set to 'true' when running in CI environment"
    default: ""
    required: false
parameters:
  - name: "project"
    type: "string"
    description: "Browser project to run (chromium, firefox, webkit)"
    default: "chromium"
    required: false
outputs:
  - name: "coverage-e2e"
    type: "directory"
    description: "E2E coverage output directory with LCOV and HTML reports"
    path: "coverage/e2e/"
  - name: "playwright-report"
    type: "directory"
    description: "HTML test report directory"
    path: "playwright-report/"
  - name: "test-results"
    type: "directory"
    description: "Test artifacts and traces"
    path: "test-results/"
metadata:
  category: "test"
  subcategory: "e2e-coverage"
  execution_time: "medium"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Test E2E Playwright Coverage

## Overview

Runs Playwright end-to-end tests with code coverage collection using `@bgotink/playwright-coverage`. This skill collects V8 coverage data during test execution and generates reports in LCOV, HTML, and JSON formats suitable for upload to Codecov.

**IMPORTANT**: This skill starts the **Vite dev server** (not Docker) because V8 coverage requires access to source files. Running coverage against the Docker container will result in `0%` coverage.

| Mode | Base URL | Coverage Support |
|------|----------|-----------------|
| Docker (`localhost:8080`) | ❌ No - Shows "Unknown% (0/0)" |
| Vite Dev (`localhost:5173`) | ✅ Yes - Real coverage data |

## Prerequisites

- Node.js 18.0 or higher installed and in PATH
- Playwright browsers installed (`npx playwright install`)
- `@bgotink/playwright-coverage` package installed
- Charon application running (default: `http://localhost:8080`)
- Test files in `tests/` directory using coverage-enabled imports

## Usage

### Basic Usage

Run E2E tests with coverage collection:

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage
```

### Browser Selection

Run tests in a specific browser:

```bash
# Firefox (default)
.github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage --project=firefox

# Firefox
.github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage --project=firefox
```

### CI/CD Integration

For use in GitHub Actions or other CI/CD pipelines:

```yaml
- name: Run E2E Tests with Coverage
  run: .github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage
  env:
    PLAYWRIGHT_BASE_URL: http://localhost:8080
    CI: true

- name: Upload E2E Coverage to Codecov
  uses: codecov/codecov-action@v5
  with:
    files: ./coverage/e2e/lcov.info
    flags: e2e
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| project   | string | No | firefox | Browser project: chromium, firefox, webkit |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| PLAYWRIGHT_BASE_URL | No | http://localhost:8080 | Application URL to test against |
| PLAYWRIGHT_HTML_OPEN | No | never | HTML report auto-open behavior |
| CI | No | "" | Set to "true" for CI environment behavior |

## Outputs

### Success Exit Code
- **0**: All tests passed and coverage generated

### Error Exit Codes
- **1**: One or more tests failed
- **Non-zero**: Configuration or execution error

### Output Directories
- **coverage/e2e/**: Coverage reports (LCOV, HTML, JSON)
  - `lcov.info` - LCOV format for Codecov upload
  - `coverage.json` - JSON format for programmatic access
  - `index.html` - HTML report for visual inspection
- **playwright-report/**: HTML test report with results and traces
- **test-results/**: Test artifacts, screenshots, and trace files

## Viewing Coverage Reports

### Coverage HTML Report

```bash
# Open coverage HTML report
open coverage/e2e/index.html
```

### Playwright Test Report

```bash
npx playwright show-report --port 9323
```

## Coverage Data Format

The skill generates coverage in multiple formats:

| Format | File | Purpose |
|--------|------|---------|
| LCOV | `coverage/e2e/lcov.info` | Codecov upload |
| HTML | `coverage/e2e/index.html` | Visual inspection |
| JSON | `coverage/e2e/coverage.json` | Programmatic access |

## Related Skills

- test-e2e-playwright - E2E tests without coverage
- test-frontend-coverage - Frontend unit test coverage with Vitest
- test-backend-coverage - Backend unit test coverage with Go

## Notes

- **Coverage Source**: Uses V8 coverage (native, no instrumentation needed)
- **Performance**: ~5-10% overhead compared to tests without coverage
- **Sharding**: When running sharded tests in CI, coverage files must be merged
- **LCOV Merge**: Use `lcov -a file1.info -a file2.info -o merged.info` to merge

---

**Last Updated**: 2026-01-18
**Maintained by**: Charon Project Team

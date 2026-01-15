---
# agentskills.io specification v1.0
name: "test-e2e-playwright"
version: "1.0.0"
description: "Run Playwright E2E tests against the Charon application with browser selection and filtering"
author: "Charon Project"
license: "MIT"
tags:
  - "testing"
  - "e2e"
  - "playwright"
  - "integration"
  - "browser"
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
    description: "Browser project to run (chromium, firefox, webkit, all)"
    default: "chromium"
    required: false
  - name: "headed"
    type: "boolean"
    description: "Run tests in headed mode (visible browser)"
    default: "false"
    required: false
  - name: "grep"
    type: "string"
    description: "Filter tests by title pattern (regex)"
    default: ""
    required: false
outputs:
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
  subcategory: "e2e"
  execution_time: "medium"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Test E2E Playwright

## Overview

Executes Playwright end-to-end tests against the Charon application. This skill supports browser selection, headed mode for debugging, and test filtering by name pattern.

The skill runs non-interactively by default (HTML report does not auto-open), making it suitable for CI/CD pipelines and automated testing scenarios.

## Prerequisites

- Node.js 18.0 or higher installed and in PATH
- Playwright browsers installed (`npx playwright install`)
- Charon application running (default: `http://localhost:8080`)
- Test files in `tests/` directory

## Usage

### Basic Usage

Run E2E tests with default settings (Chromium, headless):

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright
```

### Browser Selection

Run tests in a specific browser:

```bash
# Chromium (default)
.github/skills/scripts/skill-runner.sh test-e2e-playwright --project=chromium

# Firefox
.github/skills/scripts/skill-runner.sh test-e2e-playwright --project=firefox

# WebKit (Safari)
.github/skills/scripts/skill-runner.sh test-e2e-playwright --project=webkit

# All browsers
.github/skills/scripts/skill-runner.sh test-e2e-playwright --project=all
```

### Headed Mode (Debugging)

Run tests with a visible browser window:

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright --headed
```

### Filter Tests

Run only tests matching a pattern:

```bash
# Run tests with "login" in the title
.github/skills/scripts/skill-runner.sh test-e2e-playwright --grep="login"

# Run tests with "DNS" in the title
.github/skills/scripts/skill-runner.sh test-e2e-playwright --grep="DNS"
```

### Combined Options

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright --project=firefox --headed --grep="dashboard"
```

### CI/CD Integration

For use in GitHub Actions or other CI/CD pipelines:

```yaml
- name: Run E2E Tests
  run: .github/skills/scripts/skill-runner.sh test-e2e-playwright
  env:
    PLAYWRIGHT_BASE_URL: http://localhost:8080
    CI: true
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| project   | string | No | chromium | Browser project: chromium, firefox, webkit, all |
| headed    | boolean | No | false | Run with visible browser window |
| grep      | string | No | "" | Filter tests by title pattern (regex) |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| PLAYWRIGHT_BASE_URL | No | http://localhost:8080 | Application URL to test against |
| PLAYWRIGHT_HTML_OPEN | No | never | HTML report auto-open behavior |
| CI | No | "" | Set to "true" for CI environment behavior |

## Outputs

### Success Exit Code
- **0**: All tests passed

### Error Exit Codes
- **1**: One or more tests failed
- **Non-zero**: Configuration or execution error

### Output Directories
- **playwright-report/**: HTML report with test results and traces
- **test-results/**: Test artifacts, screenshots, and trace files

## Viewing the Report

After test execution, view the HTML report using VS Code Simple Browser:

### Method 1: Start Report Server

```bash
npx playwright show-report --port 9323
```

Then open in VS Code Simple Browser: `http://127.0.0.1:9323`

### Method 2: VS Code Task

Use the VS Code task "Test: E2E Playwright - View Report" to start the report server as a background task, then open `http://127.0.0.1:9323` in Simple Browser.

### Method 3: Direct File Access

Open `playwright-report/index.html` directly in a browser.

## Examples

### Example 1: Quick Smoke Test

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright --grep="smoke"
```

### Example 2: Debug Failing Test

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright --headed --grep="failing-test-name"
```

### Example 3: Cross-Browser Validation

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright --project=all
```

## Test Structure

Tests are located in the `tests/` directory and follow Playwright conventions:

```
tests/
├── auth.setup.ts          # Authentication setup (runs first)
├── dashboard.spec.ts      # Dashboard tests
├── dns-records.spec.ts    # DNS management tests
├── login.spec.ts          # Login flow tests
└── ...
```

## Error Handling

### Common Errors

#### Error: Target page, context or browser has been closed
**Solution**: Ensure the application is running at the configured base URL

#### Error: page.goto: net::ERR_CONNECTION_REFUSED
**Solution**: Start the Charon application before running tests

#### Error: browserType.launch: Executable doesn't exist
**Solution**: Run `npx playwright install` to install browser binaries

## Related Skills

- test-frontend-unit - Frontend unit tests with Vitest
- docker-start-dev - Start development environment
- integration-test-all - Run all integration tests

## Notes

- **Authentication**: Tests use stored auth state from `playwright/.auth/user.json`
- **Parallelization**: Tests run in parallel locally, sequential in CI
- **Retries**: CI automatically retries failed tests twice
- **Traces**: Traces are collected on first retry for debugging
- **Report**: HTML report is generated at `playwright-report/index.html`

---

**Last Updated**: 2026-01-15
**Maintained by**: Charon Project Team
**Source**: `tests/` directory

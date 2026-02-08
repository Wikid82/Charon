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

### Quick Start: Ensure E2E Environment is Ready

Before running tests, ensure the Docker E2E environment is running. Rebuild when application or Docker build inputs change. If only tests or docs changed and the container is already healthy, skip rebuild.

```bash
# Start/rebuild E2E Docker container (required when app/runtime inputs change)
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e

# Or for a complete clean rebuild:
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean --no-cache
```

## Usage

### Basic Usage

Run E2E tests with default settings (Firefox, headless):

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright
```

### Browser Selection

Run tests in a specific browser:

```bash
# Firefox (default)
.github/skills/scripts/skill-runner.sh test-e2e-playwright --project=firefox

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
| project   | string | No | firefox | Browser project: chromium, firefox, webkit, all |
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
**Solution**: Ensure the application is running at the configured base URL. Rebuild if needed:
```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
```

#### Error: page.goto: net::ERR_CONNECTION_REFUSED
**Solution**: Start the Charon application before running tests:
```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
```

#### Error: browserType.launch: Executable doesn't exist
**Solution**: Run `npx playwright install` to install browser binaries

#### Error: Timeout waiting for selector
**Solution**: The application may be slow or in an unexpected state. Try:
```bash
# Rebuild with clean state
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean

# Or debug the test to see what's happening
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug --grep="failing test"
```

#### Error: Authentication state is stale
**Solution**: Remove stored auth and let setup recreate it:
```bash
rm -rf playwright/.auth/user.json
.github/skills/scripts/skill-runner.sh test-e2e-playwright
```

## Troubleshooting Workflow

When E2E tests fail, follow this workflow:

1. **Check container health**:
   ```bash
   docker ps --filter "name=charon-playwright"
   docker logs charon-playwright --tail 50
   ```

2. **Verify the application is accessible**:
   ```bash
   curl -sf http://localhost:8080/api/v1/health
   ```

3. **Rebuild with clean state if needed**:
   ```bash
   .github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean
   ```

4. **Debug specific failing test**:
   ```bash
   .github/skills/scripts/skill-runner.sh test-e2e-playwright-debug --grep="test name"
   ```

5. **View the HTML report for details**:
   ```bash
   npx playwright show-report --port 9323
   ```

## Key File Locations

| Path | Purpose |
|------|---------|
| `tests/` | All E2E test files |
| `tests/auth.setup.ts` | Authentication setup fixture |
| `playwright.config.js` | Playwright configuration |
| `playwright/.auth/user.json` | Stored authentication state |
| `playwright-report/` | HTML test reports |
| `test-results/` | Test artifacts and traces |
| `.docker/compose/docker-compose.playwright.yml` | E2E Docker compose config |
| `Dockerfile` | Application Docker image |

## Related Skills

- [docker-rebuild-e2e](./docker-rebuild-e2e.SKILL.md) - Rebuild Docker image and restart E2E container
- [test-e2e-playwright-debug](./test-e2e-playwright-debug.SKILL.md) - Debug E2E tests in headed mode
- [test-e2e-playwright-coverage](./test-e2e-playwright-coverage.SKILL.md) - Run E2E tests with coverage
- [test-frontend-unit](./test-frontend-unit.SKILL.md) - Frontend unit tests with Vitest
- [docker-start-dev](./docker-start-dev.SKILL.md) - Start development environment
- [integration-test-all](./integration-test-all.SKILL.md) - Run all integration tests

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

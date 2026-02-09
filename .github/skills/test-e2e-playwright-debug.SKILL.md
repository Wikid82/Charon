---
# agentskills.io specification v1.0
name: "test-e2e-playwright-debug"
version: "1.0.0"
description: "Run Playwright E2E tests in headed/debug mode for troubleshooting with slowMo and trace collection"
author: "Charon Project"
license: "MIT"
tags:
  - "testing"
  - "e2e"
  - "playwright"
  - "debug"
  - "troubleshooting"
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
  - name: "PWDEBUG"
    description: "Enable Playwright Inspector (set to '1' for step-by-step debugging)"
    default: ""
    required: false
  - name: "DEBUG"
    description: "Enable verbose Playwright logging (e.g., 'pw:api')"
    default: ""
    required: false
parameters:
  - name: "file"
    type: "string"
    description: "Specific test file to run (relative to tests/ directory)"
    default: ""
    required: false
  - name: "grep"
    type: "string"
    description: "Filter tests by title pattern (regex)"
    default: ""
    required: false
  - name: "slowmo"
    type: "number"
    description: "Slow down operations by specified milliseconds"
    default: "500"
    required: false
  - name: "inspector"
    type: "boolean"
    description: "Open Playwright Inspector for step-by-step debugging"
    default: "false"
    required: false
  - name: "project"
    type: "string"
    description: "Browser project to run (chromium, firefox, webkit)"
    default: "chromium"
    required: false
outputs:
  - name: "playwright-report"
    type: "directory"
    description: "HTML test report directory"
    path: "playwright-report/"
  - name: "test-results"
    type: "directory"
    description: "Test artifacts, screenshots, and traces"
    path: "test-results/"
metadata:
  category: "test"
  subcategory: "e2e-debug"
  execution_time: "variable"
  risk_level: "low"
  ci_cd_safe: false
  requires_network: true
  idempotent: true
---

# Test E2E Playwright Debug

## Overview

Runs Playwright E2E tests in headed/debug mode for troubleshooting. This skill provides enhanced debugging capabilities including:

- **Headed Mode**: Visible browser window to watch test execution
- **Slow Motion**: Configurable delay between actions for observation
- **Playwright Inspector**: Step-by-step debugging with breakpoints
- **Trace Collection**: Always captures traces for post-mortem analysis
- **Single Test Focus**: Run individual tests or test files

**Use this skill when:**
- Debugging failing E2E tests
- Understanding test flow and interactions
- Developing new E2E tests
- Investigating flaky tests

## Prerequisites

- Node.js 18.0 or higher installed and in PATH
- Playwright browsers installed (`npx playwright install chromium`)
- Charon application running at localhost:8080 (use `docker-rebuild-e2e` when app/runtime inputs change or the container is not running)
- Display available (X11 or Wayland on Linux, native on macOS)
- Test files in `tests/` directory

## Usage

### Basic Debug Mode

Run all tests in headed mode with slow motion:

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug
```

### Debug Specific Test File

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug --file=login.spec.ts
```

### Debug Test by Name Pattern

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug --grep="should login with valid credentials"
```

### With Playwright Inspector

Open the Playwright Inspector for step-by-step debugging:

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug --inspector
```

### Custom Slow Motion

Adjust the delay between actions (in milliseconds):

```bash
# Slower for detailed observation
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug --slowmo=1000

# Faster but still visible
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug --slowmo=200
```

### Different Browser

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug --project=firefox
```

### Combined Options

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug \
    --file=dashboard.spec.ts \
    --grep="navigation" \
    --slowmo=750 \
    --project=chromium
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| file      | string | No | "" | Specific test file to run |
| grep      | string | No | "" | Filter tests by title pattern |
| slowmo    | number | No | 500 | Delay between actions (ms) |
| inspector | boolean | No | false | Open Playwright Inspector |
| project   | string | No | chromium | Browser to use |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| PLAYWRIGHT_BASE_URL | No | http://localhost:8080 | Application URL |
| PWDEBUG | No | "" | Set to "1" for Inspector mode |
| DEBUG | No | "" | Verbose logging (e.g., "pw:api") |

## Debugging Techniques

### Using Playwright Inspector

The Inspector provides:
- **Step-through Execution**: Execute one action at a time
- **Locator Playground**: Test and refine selectors
- **Call Log**: View all Playwright API calls
- **Console**: Access browser console

```bash
# Enable Inspector
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug --inspector
```

In the Inspector:
1. Use **Resume** to continue to next action
2. Use **Step** to execute one action
3. Use the **Locator** tab to test selectors
4. Check **Console** for JavaScript errors

### Adding Breakpoints in Tests

Add `await page.pause()` in your test code:

```typescript
test('debug this test', async ({ page }) => {
    await page.goto('/');
    await page.pause(); // Opens Inspector here
    await page.click('button');
});
```

### Verbose Logging

Enable detailed Playwright API logging:

```bash
DEBUG=pw:api .github/skills/scripts/skill-runner.sh test-e2e-playwright-debug
```

### Screenshot on Failure

Tests automatically capture screenshots on failure. Find them in:
```
test-results/<test-name>/
├── test-failed-1.png
├── trace.zip
└── ...
```

## Analyzing Traces

Traces are always captured in debug mode. View them with:

```bash
# Open trace viewer for a specific test
npx playwright show-trace test-results/<test-name>/trace.zip

# Or view in browser
npx playwright show-trace --port 9322
```

Traces include:
- DOM snapshots at each step
- Network requests/responses
- Console logs
- Screenshots
- Action timeline

## Examples

### Example 1: Debug Login Flow

```bash
# Rebuild environment with clean state
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean

# Debug login tests
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug \
    --file=login.spec.ts \
    --slowmo=800
```

### Example 2: Investigate Flaky Test

```bash
# Run with Inspector to step through
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug \
    --grep="flaky test name" \
    --inspector

# After identifying the issue, view the trace
npx playwright show-trace test-results/*/trace.zip
```

### Example 3: Develop New Test

```bash
# Run in headed mode while developing
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug \
    --file=new-feature.spec.ts \
    --slowmo=500
```

### Example 4: Cross-Browser Debug

```bash
# Debug in Firefox
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug \
    --project=firefox \
    --grep="cross-browser issue"
```

## Test File Locations

| Path | Description |
|------|-------------|
| `tests/` | All E2E test files |
| `tests/auth.setup.ts` | Authentication setup |
| `tests/login.spec.ts` | Login flow tests |
| `tests/dashboard.spec.ts` | Dashboard tests |
| `tests/dns-records.spec.ts` | DNS management tests |
| `playwright/.auth/` | Stored auth state |

## Troubleshooting

### No Browser Window Opens

**Linux**: Ensure X11/Wayland display is available
```bash
echo $DISPLAY  # Should show :0 or similar
```

**Remote/SSH**: Use X11 forwarding or VNC
```bash
ssh -X user@host
```

**WSL2**: Install and configure WSLg or X server

### Test Times Out

Increase timeout for debugging:
```bash
# In your test file
test.setTimeout(120000); // 2 minutes
```

### Inspector Doesn't Open

Ensure PWDEBUG is set:
```bash
PWDEBUG=1 npx playwright test --headed
```

### Cannot Find Test File

Check the file exists:
```bash
ls -la tests/*.spec.ts
```

Use relative path from tests/ directory:
```bash
--file=login.spec.ts  # Not tests/login.spec.ts
```

## Common Issues and Solutions

| Issue | Solution |
|-------|----------|
| "Target closed" | Application crashed - check container logs |
| "Element not found" | Use Inspector to verify selector |
| "Timeout exceeded" | Increase timeout or check if element is hidden |
| "Net::ERR_CONNECTION_REFUSED" | Ensure Docker container is running |
| Flaky test | Add explicit waits or use Inspector to find race condition |

## Related Skills

- [test-e2e-playwright](./test-e2e-playwright.SKILL.md) - Run tests normally
- [docker-rebuild-e2e](./docker-rebuild-e2e.SKILL.md) - Rebuild E2E environment
- [test-e2e-playwright-coverage](./test-e2e-playwright-coverage.SKILL.md) - Run with coverage

## Notes

- **Not CI/CD Safe**: Headed mode requires a display
- **Resource Usage**: Browser windows consume significant memory
- **Slow Motion**: Default 500ms delay; adjust based on needs
- **Traces**: Always captured for post-mortem analysis
- **Single Worker**: Runs one test at a time for clarity

---

**Last Updated**: 2026-01-21
**Maintained by**: Charon Project Team
**Test Directory**: `tests/`

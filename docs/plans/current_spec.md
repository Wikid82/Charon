# Playwright E2E Skill with VS Code Simple Browser Integration

**Date**: 2026-01-15
**Status**: Planning
**Category**: Testing / Developer Experience

---

## Overview

Create a new agent skill that runs Playwright E2E tests non-interactively and opens the HTML report in VS Code's Simple Browser instead of launching an external browser.

---

## Analysis of Current Playwright Setup

### Configuration (`playwright.config.js`)

| Setting | Value | Notes |
|---------|-------|-------|
| Test Directory | `./tests` | All E2E tests reside here |
| Timeout | 30,000ms | Per-test timeout |
| Parallel | `fullyParallel: true` | Tests run in parallel locally |
| Retries | 0 (local), 2 (CI) | CI has retries enabled |
| Workers | unlimited (local), 1 (CI) | CI runs single worker |
| Base URL | `PLAYWRIGHT_BASE_URL` or `http://localhost:8080` | Configurable via env |
| Report Location | `playwright-report/` | HTML report output |

### Reporter Configuration

```javascript
// CI mode
reporter: [['github'], ['html', { open: 'never' }]]

// Local mode
reporter: [['list'], ['html', { open: 'on-failure' }]]
```

**Key Finding**: The `open` option controls whether Playwright automatically opens the report:
- `'never'` - Never auto-open (used in CI)
- `'on-failure'` - Open only if tests fail (current local default)
- `'always'` - Always open after tests

### Existing npm Scripts (`package.json`)

| Script | Command | Description |
|--------|---------|-------------|
| `e2e` | `PLAYWRIGHT_HTML_OPEN=never npx playwright test --project=chromium` | Run Chromium only, no report open |
| `e2e:all` | `PLAYWRIGHT_HTML_OPEN=never npx playwright test` | Run all browsers, no report open |
| `e2e:headed` | `npx playwright test --project=chromium --headed` | Run headed (visible browser) |
| `e2e:report` | `npx playwright show-report` | **Opens report in default browser** |

### Existing VS Code Tasks (`.vscode/tasks.json`)

| Task Label | Command | Notes |
|------------|---------|-------|
| `Test: E2E Playwright (Chromium)` | `npm run e2e` | Non-interactive ✅ |
| `Test: E2E Playwright (All Browsers)` | `npm run e2e:all` | Non-interactive ✅ |
| `Test: E2E Playwright (Headed)` | `npm run e2e:headed` | Interactive/visual |

**Gap Identified**: No task/skill exists to open the report in VS Code Simple Browser.

### Current Skill Directory Structure

Skills follow this pattern:
```
.github/skills/
├── {skill-name}.SKILL.md           # Skill definition (frontmatter + docs)
├── {skill-name}-scripts/
│   └── run.sh                      # Execution script
```

---

## Solution Design

### Running Playwright Non-Interactively

Playwright can be run without watching/waiting for report using:

1. **Environment Variable**: `PLAYWRIGHT_HTML_OPEN=never`
2. **CLI Flag**: `--reporter=html` with no `--open` flag
3. **Config Override**: Already handled in `package.json` scripts

The existing `npm run e2e` command already achieves this with `PLAYWRIGHT_HTML_OPEN=never`.

### Opening HTML Report in VS Code Simple Browser

VS Code provides the `simpleBrowser.show` command that can be invoked:

1. **From Shell**: Using VS Code CLI: `code --goto file:///path/to/report`
2. **Using VS Code Command**: `simpleBrowser.api.open` or via tasks
3. **Using `open_simple_browser` tool**: For agent automation

**Report Path**: `playwright-report/index.html` (relative to project root)

### Skill Implementation Strategy

Create a new skill `test-e2e-playwright` that:

1. Runs Playwright tests non-interactively
2. Captures test results
3. Outputs the report path for VS Code integration
4. Optionally opens report in Simple Browser via VS Code command

---

## Implementation Plan

### Phase 1: Create Skill Files

#### 1.1 Create Skill Definition File

**File**: `.github/skills/test-e2e-playwright.SKILL.md`

```yaml
---
# agentskills.io specification v1.0
name: "test-e2e-playwright"
version: "1.0.0"
description: "Run Playwright E2E tests non-interactively with HTML report generation"
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
    version: ">=8.0"
    optional: false
environment_variables:
  - name: "PLAYWRIGHT_BASE_URL"
    description: "Base URL for tests (default: http://localhost:8080)"
    required: false
  - name: "PLAYWRIGHT_PROJECT"
    description: "Browser project to run (chromium, firefox, webkit, or all)"
    required: false
parameters:
  - name: "project"
    type: "string"
    description: "Browser project (chromium, firefox, webkit, all)"
    default: "chromium"
    required: false
  - name: "headed"
    type: "boolean"
    description: "Run in headed mode (visible browser)"
    default: "false"
    required: false
  - name: "grep"
    type: "string"
    description: "Filter tests by title pattern"
    required: false
outputs:
  - name: "test_results"
    type: "stdout"
    description: "Playwright test output"
  - name: "report_path"
    type: "file"
    description: "Path to HTML report (playwright-report/index.html)"
metadata:
  category: "test"
  subcategory: "e2e"
  execution_time: "medium"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---
```

#### 1.2 Create Execution Script

**File**: `.github/skills/test-e2e-playwright-scripts/run.sh`

```bash
#!/usr/bin/env bash
# Test E2E Playwright - Execution Script
#
# Runs Playwright E2E tests non-interactively and generates HTML report.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_SCRIPTS_DIR="$(cd "${SCRIPT_DIR}/../scripts" && pwd)"

source "${SKILLS_SCRIPTS_DIR}/_logging_helpers.sh"
source "${SKILLS_SCRIPTS_DIR}/_error_handling_helpers.sh"
source "${SKILLS_SCRIPTS_DIR}/_environment_helpers.sh"

PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Parse arguments
PROJECT="${PLAYWRIGHT_PROJECT:-chromium}"
HEADED="${PLAYWRIGHT_HEADED:-false}"
GREP_PATTERN=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --project=*)
            PROJECT="${1#*=}"
            shift
            ;;
        --headed)
            HEADED="true"
            shift
            ;;
        --grep=*)
            GREP_PATTERN="${1#*=}"
            shift
            ;;
        *)
            break
            ;;
    esac
done

# Validate environment
log_step "ENVIRONMENT" "Validating prerequisites"
validate_node_environment "18" || error_exit "Node.js 18+ is required"

cd "${PROJECT_ROOT}"

# Check Playwright is installed
if ! npx playwright --version &>/dev/null; then
    log_error "Playwright not installed. Run: npm install"
    exit 1
fi

# Build command
log_step "EXECUTION" "Running Playwright E2E tests"

CMD_ARGS=("npx" "playwright" "test")

# Add project filter
if [[ "${PROJECT}" != "all" ]]; then
    CMD_ARGS+=("--project=${PROJECT}")
fi

# Add headed mode
if [[ "${HEADED}" == "true" ]]; then
    CMD_ARGS+=("--headed")
fi

# Add grep pattern
if [[ -n "${GREP_PATTERN}" ]]; then
    CMD_ARGS+=("--grep=${GREP_PATTERN}")
fi

# Ensure report doesn't auto-open
export PLAYWRIGHT_HTML_OPEN=never

log_info "Command: ${CMD_ARGS[*]}"
log_info "Base URL: ${PLAYWRIGHT_BASE_URL:-http://localhost:8080}"

# Run tests
if "${CMD_ARGS[@]}" "$@"; then
    EXIT_CODE=0
    log_success "Playwright E2E tests passed"
else
    EXIT_CODE=$?
    log_warning "Playwright E2E tests completed with failures (exit code: ${EXIT_CODE})"
fi

# Output report path
REPORT_PATH="${PROJECT_ROOT}/playwright-report/index.html"
if [[ -f "${REPORT_PATH}" ]]; then
    log_info "HTML Report: ${REPORT_PATH}"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📊 Report available at: file://${REPORT_PATH}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
fi

exit "${EXIT_CODE}"
```

### Phase 2: Update VS Code Tasks

Add new tasks to `.vscode/tasks.json`:

```jsonc
{
    "label": "Test: E2E Playwright (Skill)",
    "type": "shell",
    "command": ".github/skills/scripts/skill-runner.sh test-e2e-playwright",
    "group": "test",
    "problemMatcher": [],
    "presentation": {
        "reveal": "always",
        "panel": "dedicated",
        "close": false
    }
},
{
    "label": "Test: E2E Playwright - View Report",
    "type": "shell",
    "command": "echo 'Opening report...'",
    "group": "test",
    "problemMatcher": [],
    "runOptions": {
        "runOn": "default"
    },
    "presentation": {
        "reveal": "never"
    }
}
```

### Phase 3: VS Code Simple Browser Integration

The VS Code Simple Browser can be opened via:

#### Option A: VS Code Command (Preferred)

Create a compound task that:
1. Runs tests
2. Opens Simple Browser with report

**Implementation**: Use `vscode.commands.executeCommand('simpleBrowser.show', uri)` from an extension or the `simpleBrowser.api.open` command.

For agent usage, the `open_simple_browser` tool can be used:

```
URL: file:///projects/Charon/playwright-report/index.html
```

#### Option B: HTTP Server Approach

Start a local HTTP server to serve the report:

```bash
npx playwright show-report --host 127.0.0.1 --port 9323
```

Then open in Simple Browser: `http://127.0.0.1:9323`

**Note**: This requires the server to stay running.

#### Option C: File Protocol

Open directly as file URL (simplest):

```
file:///projects/Charon/playwright-report/index.html
```

---

## Files to Create/Modify

### New Files

| File | Purpose |
|------|---------|
| `.github/skills/test-e2e-playwright.SKILL.md` | Skill definition |
| `.github/skills/test-e2e-playwright-scripts/run.sh` | Execution script |

### Modified Files

| File | Changes |
|------|---------|
| `.vscode/tasks.json` | Add new Playwright skill task |

---

## Usage Examples

### Basic Usage (Agent)

```bash
# Run E2E tests
.github/skills/scripts/skill-runner.sh test-e2e-playwright

# Then open report in VS Code Simple Browser
# (Agent can use open_simple_browser tool with file:///projects/Charon/playwright-report/index.html)
```

### Specific Browser

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright --project=firefox
```

### Headed Mode (Debugging)

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright --headed
```

### Filter by Test Name

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright --grep="login"
```

---

## Integration with Agent Workflow

When an agent needs to run E2E tests and view results:

1. **Run skill**: `skill-runner.sh test-e2e-playwright`
2. **Check exit code**: 0 = pass, non-zero = failures
3. **Open report**: Use `open_simple_browser` tool with URL:
   - File URL: `file:///projects/Charon/playwright-report/index.html`
   - Or start server and use: `http://127.0.0.1:9323`

---

## Acceptance Criteria

- [ ] Skill runs Playwright tests without interactive report watcher
- [ ] HTML report is generated at `playwright-report/index.html`
- [ ] Skill outputs report path for easy consumption
- [ ] Exit code reflects test pass/fail status
- [ ] Report can be opened in VS Code Simple Browser
- [ ] Supports all Playwright projects (chromium, firefox, webkit)
- [ ] Supports headed mode for debugging
- [ ] Supports grep filtering for specific tests

---

## Notes

1. **File Protocol Limitation**: Some browsers restrict `file://` protocol. VS Code Simple Browser handles this well.

2. **Report Serving**: For interactive debugging, `npx playwright show-report` starts a server but blocks. The skill approach avoids this.

3. **CI/CD Safety**: The skill is CI/CD safe since it doesn't depend on UI interaction.

4. **Base URL**: Tests use `PLAYWRIGHT_BASE_URL` environment variable, defaulting to `http://localhost:8080`.

---

## Related Documentation

- [Playwright Test Configuration](https://playwright.dev/docs/test-configuration)
- [VS Code Simple Browser](https://code.visualstudio.com/docs/editor/extension-marketplace#_simple-browser)
- [Agent Skills Specification](https://agentskills.io/specification)
- [Existing Playwright Tasks](.vscode/tasks.json)

---

**Last Updated**: 2026-01-15
**Author**: GitHub Copilot
**Status**: Ready for Implementation

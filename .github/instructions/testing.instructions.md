---
applyTo: '**'
description: 'Strict protocols for test execution, debugging, and coverage validation.'
---
# Testing Protocols

## 0. E2E Verification First (Playwright)

**MANDATORY**: Before running unit tests, verify the application functions correctly end-to-end.

### Two Modes: Docker vs Vite

Playwright E2E tests can run in two modes with different capabilities:

| Mode | Base URL | Coverage Support | When to Use |
|------|----------|-----------------|-------------|
| **Docker** | `http://localhost:8080` | ❌ No (0% reported) | Integration testing, CI validation |
| **Vite Dev** | `http://localhost:5173` | ✅ Yes (real coverage) | Local development, coverage collection |

**Why?** The `@bgotink/playwright-coverage` library uses V8 coverage which requires access to source files. Only the Vite dev server exposes source maps and raw source files needed for coverage instrumentation.

### Running E2E Tests (Integration Mode)

For general integration testing without coverage:

```bash
# Against Docker container (default)
npx playwright test --project=chromium

# With explicit base URL
PLAYWRIGHT_BASE_URL=http://localhost:8080 npx playwright test --project=chromium
```

### Running E2E Tests with Coverage

**IMPORTANT**: Use the dedicated skill for coverage collection:

```bash
# Recommended: Uses skill that starts Vite and runs against localhost:5173
.github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage
```

The coverage skill:
1. Starts Vite dev server on port 5173
2. Sets `PLAYWRIGHT_BASE_URL=http://localhost:5173`
3. Runs tests with V8 coverage collection
4. Generates reports in `coverage/e2e/` (LCOV, HTML, JSON)

**DO NOT** expect coverage when running against Docker:
```bash
# ❌ WRONG: Coverage will show "Unknown% (0/0)"
PLAYWRIGHT_BASE_URL=http://localhost:8080 npx playwright test --coverage

# ✅ CORRECT: Use the coverage skill
.github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage
```

### Verifying Coverage Locally Before CI

Before pushing code, verify E2E coverage:

1. Run the coverage skill:
   ```bash
   .github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage
   ```

2. Check coverage output:
   ```bash
   # View HTML report
   open coverage/e2e/index.html

   # Check LCOV file exists for Codecov
   ls -la coverage/e2e/lcov.info
   ```

3. Verify non-zero coverage:
   ```bash
   # Should show real percentages, not "0%"
   head -20 coverage/e2e/lcov.info
   ```

### General Guidelines

* **No Truncation**: Never pipe Playwright test output through `head`, `tail`, or other truncating commands. Playwright runs interactively and requires user input to quit when piped, causing the command to hang indefinitely.
* **Why First**: If the application is broken at the E2E level, unit tests may need updates. Playwright catches integration issues early.
* **On Failure**: Analyze failures, trace root cause through frontend → backend flow, then fix before proceeding to unit tests.
* **Scope**: Run relevant test files for the feature being modified (e.g., `tests/manual-dns-provider.spec.ts`).

## 1. Execution Environment
* **No Truncation:** Never use pipe commands (e.g., `head`, `tail`) or flags that limit stdout/stderr. If a test hangs, it likely requires an interactive input or is caught in a loop; analyze the full output to identify the block.
* **Task-Based Execution:** Do not manually construct test strings. Use existing project tasks (e.g., `npm test`, `go test ./...`). If a specific sub-module requires frequent testing, generate a new task definition in the project's configuration file (e.g., `.vscode/tasks.json`) before proceeding.

## 2. Failure Analysis & Logic Integrity
* **Evidence-Based Debugging:** When a test fails, you must quote the specific error message or stack trace before suggesting a fix.
* **Bug vs. Test Flaw:** Treat the test as the "Source of Truth." If a test fails, assume the code is broken until proven otherwise. Research the original requirement or PR description to verify if the test logic itself is outdated before modifying it.
* **Zero-Hallucination Policy:** Only use file paths and identifiers discovered via the `ls` or `search` tools. Never guess a path based on naming conventions.

## 3. Coverage & Completion
* **Coverage Gate:** A task is not "Complete" until a coverage report is generated.
* **Threshold Compliance:** You must compare the final coverage percentage against the project's threshold (Default: 85% unless specified otherwise). If coverage drops, you must identify the "uncovered lines" and add targeted tests.
* **Patch Coverage Gate (Codecov):** If production code is modified, Codecov **patch coverage must be 100%** for the modified lines. Do not relax thresholds; add targeted tests.
* **Patch Triage Requirement:** Plans must include the exact missing/partial patch line ranges copied from Codecov’s **Patch** view.

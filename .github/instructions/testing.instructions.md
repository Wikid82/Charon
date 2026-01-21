---
applyTo: '**'
description: 'Strict protocols for test execution, debugging, and coverage validation.'
---
# Testing Protocols

## 0. E2E Verification First (Playwright)

**MANDATORY**: Before running unit tests, verify the application functions correctly end-to-end.

* **Run Playwright E2E Tests**: Execute `npx playwright test --project=chromium` from the project root.
* **No Truncation**: Never pipe Playwright test output through `head`, `tail`, or other truncating commands. Playwright tests run interactively and require user input to quit when piped, causing the command to hang indefinitely.
* **Why First**: If the application is broken at the E2E level, unit tests may need updates. Playwright catches integration issues early.
* **Base URL**: Tests use `PLAYWRIGHT_BASE_URL` env var or default from `playwright.config.js` (Tailscale IP: `http://100.98.12.109:8080`).
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

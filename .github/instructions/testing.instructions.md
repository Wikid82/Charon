---
applyTo: '**'
description: 'Strict protocols for test execution, debugging, and coverage validation.'
---
# Testing Protocols

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

---
applyTo: '**'
description: 'Strict protocols for test execution, debugging, and coverage validation.'
---
# Testing Protocols

## 0. E2E Verification First (Playwright)

**MANDATORY**: Before running unit tests, verify the application UI/UX functions correctly end-to-end.

### PREREQUISITE: Start E2E Environment

**CRITICAL**: Rebuild the E2E container when application or Docker build inputs change. If changes are test-only and the container is already healthy, reuse it. If the container is not running or state is suspect, rebuild.

**Rebuild required (application/runtime changes):**
- Application code or dependencies: backend/**, frontend/**, backend/go.mod, backend/go.sum, package.json, package-lock.json.
- Container build/runtime configuration: Dockerfile, .docker/**, .docker/compose/docker-compose.playwright-*.yml, .docker/docker-entrypoint.sh.
- Runtime behavior changes baked into the image.

**Rebuild optional (test-only changes):**
- Playwright tests and fixtures: tests/**.
- Playwright config and runners: playwright.config.js, playwright.caddy-debug.config.js.
- Documentation or planning files: docs/**, requirements.md, design.md, tasks.md.
- CI/workflow changes that do not affect runtime images: .github/workflows/**.

When a rebuild is required (or the container is not running), use:

```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
```

This step:
- Builds the latest Docker image with your code changes
- Starts the `charon-e2e` container with proper environment variables from `.env`
- Exposes required ports: 8080 (app), 2020 (emergency), 2019 (Caddy admin)
- Waits for health check to pass

**Without this step**, tests will fail with:
- `connect ECONNREFUSED ::1:2020` - Emergency server not running
- `connect ECONNREFUSED ::1:8080` - Application not running
- `501 Not Implemented` - Container missing required env vars

### Testing Scope Clarification

**Playwright E2E Tests (UI/UX):**
- Test user interactions with the React frontend
- Verify UI state changes when settings are toggled
- Ensure forms submit correctly
- Check navigation and page rendering
- **Port: 8080 (Charon Management Interface)**
- **Default Browser: Firefox** (provides best cross-browser compatibility baseline)

**Integration Tests (Middleware Enforcement):**
- Test Cerberus security module enforcement
- Verify ACL, WAF, Rate Limiting, CrowdSec actually block/allow requests
- Test requests routing through Caddy proxy with full middleware
- **Port: 80 (User Traffic via Caddy)**
- **Location: `backend/integration/` with `//go:build integration` tag**
- **CI: Runs in separate workflows (cerberus-integration.yml, waf-integration.yml, etc.)**

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
cd /projects/Charon npx playwright test --project=firefox --project=firefox --project=webkit

# With explicit base URL
PLAYWRIGHT_BASE_URL=http://localhost:8080 npx playwright test --project=chromium --project=firefox --project=webkit
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
## 4. GORM Security Validation (Manual Stage)

**Requirement:** All backend changes involving GORM models or database interactions must pass the GORM Security Scanner.

### When to Run

* **Before Committing:** When modifying GORM models (files in `backend/internal/models/`)
* **Before Opening PR:** Verify no security issues introduced
* **After Code Review:** If model-related changes were requested
* **Definition of Done:** Scanner must pass with zero CRITICAL/HIGH issues

### Running the Scanner

**Via VS Code (Recommended for Development):**
1. Open Command Palette (`Cmd/Ctrl+Shift+P`)
2. Select "Tasks: Run Task"
3. Choose "Lint: GORM Security Scan"

**Via Pre-commit (Manual Stage):**
```bash
# Run on all Go files
pre-commit run --hook-stage manual gorm-security-scan --all-files

# Run on staged files only
pre-commit run --hook-stage manual gorm-security-scan
```

**Direct Execution:**
```bash
# Report mode - Show all issues, exit 0 (always)
./scripts/scan-gorm-security.sh --report

# Check mode - Exit 1 if issues found (use in CI)
./scripts/scan-gorm-security.sh --check
```

### Expected Behavior

**Pass (Exit Code 0):**
- No security issues detected
- Proceed with commit/PR

**Fail (Exit Code 1):**
- Issues detected (ID leaks, exposed secrets, DTO embedding, etc.)
- Review scanner output for file:line references
- Fix issues before committing
- See [GORM Security Scanner Documentation](../docs/implementation/gorm_security_scanner_complete.md)

### Common Issues Detected

1. **🔴 CRITICAL: ID Leak** — Numeric ID with `json:"id"` tag
   - Fix: Change to `json:"-"`, use UUID for external reference

2. **🔴 CRITICAL: Exposed Secret** — APIKey/Token/Password with JSON tag
   - Fix: Change to `json:"-"` to hide sensitive field

3. **🟡 HIGH: DTO Embedding** — Response struct embeds model with exposed ID
   - Fix: Use explicit field definitions instead of embedding

### Integration Status

**Current Stage:** Manual (soft launch)
- Scanner available for manual invocation
- Does not block commits automatically
- Developers should run proactively

**Future Stage:** Blocking (after remediation)
- Scanner will block commits with CRITICAL/HIGH issues
- CI integration will enforce on all PRs
- See [GORM Scanner Roadmap](../docs/implementation/gorm_security_scanner_complete.md#remediation-roadmap)

### Performance

- **Execution Time:** ~2 seconds per full scan
- **Fast enough** for pre-commit use
- **No impact** on commit workflow when passing

### Documentation

- **Implementation Details:** [docs/implementation/gorm_security_scanner_complete.md](../docs/implementation/gorm_security_scanner_complete.md)
- **Specification:** [docs/plans/gorm_security_scanner_spec.md](../docs/plans/gorm_security_scanner_spec.md)
- **QA Report:** [docs/reports/gorm_scanner_qa_report.md](../docs/reports/gorm_scanner_qa_report.md)

# Charon Copilot Instructions

## Code Quality Guidelines

Every session should improve the codebase, not just add to it. Actively refactor code you encounter, even outside of your immediate task scope. Think about long-term maintainability and consistency. Make a detailed plan before writing code. Always create unit tests for new code coverage.

- **MANDATORY**: Read all relevant instructions in `.github/instructions/` for the specific task before starting.
- **ARCHITECTURE AWARENESS**: Always consult `ARCHITECTURE.md` at the repository root before making significant changes to:
  - Core components (Backend API, Frontend, Caddy Manager, Security layers)
  - System architecture or data flow
  - Technology stack or dependencies
  - Deployment configuration
  - Directory structure or file organization
- **DRY**: Consolidate duplicate patterns into reusable functions, types, or components after the second occurrence.
- **CLEAN**: Delete dead code immediately. Remove unused imports, variables, functions, types, commented code, and console logs.
- **LEVERAGE**: Use battle-tested packages over custom implementations.
- **READABLE**: Maintain comments and clear naming for complex logic. Favor clarity over cleverness.
- **CONVENTIONAL COMMITS**: Write commit messages using `feat:`, `fix:`, `chore:`, `refactor:`, or `docs:` prefixes.

## 🚨 CRITICAL ARCHITECTURE RULES 🚨

- **Single Frontend Source**: All frontend code MUST reside in `frontend/`. NEVER create `backend/frontend/` or any other nested frontend directory.
- **Single Backend Source**: All backend code MUST reside in `backend/`.
- **No Python**: This is a Go (Backend) + React/TypeScript (Frontend) project. Do not introduce Python scripts or requirements.

 ## 🛑 Root Cause Analysis Protocol (MANDATORY)
**Constraint:** You must NEVER patch a symptom without tracing the root cause.
If a bug is reported, do NOT stop at the first error message found. Use Playwright MCP to trace the entire flow from frontend action to backend processing. Identify the true origin of the issue.

**The "Context First" Rule:**
Before proposing ANY code change or fix, you must build a mental map of the feature:
1.  **Entry Point:** Where does the data enter? (API Route / UI Event)
2.  **Transformation:** How is the data modified? (Handlers / Middleware)
3.  **Persistence:** Where is it stored? (DB Models / Files)
4.  **Exit Point:** How is it returned to the user?

**Anti-Pattern Warning:** - Do not assume the error log is the *cause*; it is often just the *victim* of an upstream failure.
- If you find an error, search for "upstream callers" to see *why* that data was bad in the first place.

## Big Picture

- Charon is a self-hosted web app for managing reverse proxy host configurations with the novice user in mind. Everything should prioritize simplicity, usability, reliability, and security, all rolled into one simple binary + static assets deployment. No external dependencies.
- Users should feel like they have enterprise-level security and features with zero effort.
- `backend/cmd/api` loads config, opens SQLite, then hands off to `internal/server`.
- `internal/config` respects `CHARON_ENV`, `CHARON_HTTP_PORT`, `CHARON_DB_PATH` and creates the `data/` directory.
- `internal/server` mounts the built React app (via `attachFrontend`) whenever `frontend/dist` exists.
- Persistent types live in `internal/models`; GORM auto-migrates them.

## Backend Workflow

- **Run**: `cd backend && go run ./cmd/api`.
- **Test**: `go test ./...`.
- **Static Analysis (BLOCKING)**: Fast linters run automatically on every commit via pre-commit hooks.
  - **Staticcheck errors MUST be fixed** - commits are BLOCKED until resolved
  - Manual run: `make lint-fast` or VS Code task "Lint: Staticcheck (Fast)"
  - Staticcheck-only: `make lint-staticcheck-only`
  - Runtime: ~11s (measured: 10.9s) (acceptable for commit gate)
  - Full golangci-lint (all linters): Use `make lint-backend` before PR (manual stage)
- **API Response**: Handlers return structured errors using `gin.H{"error": "message"}`.
- **JSON Tags**: All struct fields exposed to the frontend MUST have explicit `json:"snake_case"` tags.
- **IDs**: UUIDs (`github.com/google/uuid`) are generated server-side; clients never send numeric IDs.
- **Security**: Sanitize all file paths using `filepath.Clean`. Use `fmt.Errorf("context: %w", err)` for error wrapping.
- **Graceful Shutdown**: Long-running work must respect `server.Run(ctx)`.

### Troubleshooting Pre-Commit Staticcheck Failures

**Common Issues:**

1. **"golangci-lint not found"**
   - Install: See README.md Development Setup section
   - Verify: `golangci-lint --version`
   - Ensure `$GOPATH/bin` is in PATH

2. **Staticcheck reports deprecated API usage (SA1019)**
   - Fix: Replace deprecated function with recommended alternative
   - Check Go docs for migration path
   - Example: `filepath.HasPrefix` → use `strings.HasPrefix` with cleaned paths

3. **"This value is never used" (SA4006)**
   - Fix: Remove unused assignment or use the value
   - Common in test setup code

4. **"Should replace if statement with..." (S10xx)**
   - Fix: Apply suggested simplification
   - These improve readability and performance

5. **Emergency bypass (use sparingly):**
   - `git commit --no-verify -m "Emergency hotfix"`
   - **MUST** create follow-up issue to fix staticcheck errors
   - Only for production incidents

## Frontend Workflow

- **Location**: Always work within `frontend/`.
- **Stack**: React 18 + Vite + TypeScript + TanStack Query (React Query).
- **State Management**: Use `src/hooks/use*.ts` wrapping React Query.
- **API Layer**: Create typed API clients in `src/api/*.ts` that wrap `client.ts`.
- **Forms**: Use local `useState` for form fields, submit via `useMutation`, then `invalidateQueries` on success.

## Cross-Cutting Notes

- **VS Code Integration**: If you introduce new repetitive CLI actions (e.g., scans, builds, scripts), register them in .vscode/tasks.json to allow for easy manual verification.
- **Sync**: React Query expects the exact JSON produced by GORM tags (snake_case). Keep API and UI field names aligned.
- **Migrations**: When adding models, update `internal/models` AND `internal/api/routes/routes.go` (AutoMigrate).
- **Testing**: All new code MUST include accompanying unit tests.
- **Ignore Files**: Always check `.gitignore`, `.dockerignore`, and `.codecov.yml` when adding new file or folders.

## Documentation

- **Architecture**: Update `ARCHITECTURE.md` when making changes to:
  - System architecture or component interactions
  - Technology stack (major version upgrades, library replacements)
  - Directory structure or organizational conventions
  - Deployment model or infrastructure
  - Security architecture or data flow
  - Integration points or external dependencies
- **Features**: Update `docs/features.md` when adding capabilities. This is a short "marketing" style list. Keep details to their individual docs.
- **Links**: Use GitHub Pages URLs (`https://wikid82.github.io/charon/`) for docs and GitHub blob links for repo files.

## CI/CD & Commit Conventions

- **Triggers**: Use `feat:`, `fix:`, or `perf:` to trigger Docker builds. `chore:` skips builds.
- **Beta**: `feature/beta-release` always builds.
- **History-Rewrite PRs**: If a PR touches files in `scripts/history-rewrite/` or `docs/plans/history_rewrite.md`, the PR description MUST include the history-rewrite checklist from `.github/PULL_REQUEST_TEMPLATE/history-rewrite.md`. This is enforced by CI.

## PR Sizing & Decomposition

- **Default Rule**: Prefer smaller, reviewable PRs over one large PR when work spans multiple domains.
- **Split into Multiple PRs When**:
    - The change touches backend + frontend + infrastructure/security in one effort
    - The estimated diff is large enough to reduce review quality or increase rollback risk
    - The work can be delivered in independently testable slices without breaking behavior
    - A foundational refactor is needed before feature delivery
- **Suggested PR Sequence**:
    1. Foundation PR (types/contracts/refactors, no behavior change)
    2. Backend PR (API/model/service changes + tests)
    3. Frontend PR (UI integration + tests)
    4. Hardening PR (security/CI/docs/follow-up fixes)
- **Per-PR Requirement**: Every PR must remain deployable, pass DoD checks, and include a clear dependency note on prior PRs.

## ✅ Task Completion Protocol (Definition of Done)

Before marking an implementation task as complete, perform the following in order:

1. **Playwright E2E Tests** (MANDATORY - Run First):
    - **Run**: `cd /projects/Charon npx playwright test --project=firefox` from project root
    - **Why First**: If the app is broken at E2E level, unit tests may need updates. Catch integration issues early.
    - **Scope**: Run tests relevant to modified features (e.g., `tests/manual-dns-provider.spec.ts`)
    - **On Failure**: Trace root cause through frontend → backend flow before proceeding
    - **Base URL**: Uses `PLAYWRIGHT_BASE_URL` or default from `playwright.config.js`
    - All E2E tests must pass before proceeding to unit tests

2. **Local Patch Coverage Preflight** (MANDATORY - Run Before Unit/Coverage Tests):
    - **Run**: VS Code task `Test: Local Patch Report` or `bash scripts/local-patch-report.sh` from repo root.
    - **Purpose**: Surface exact changed files and uncovered changed lines before adding/refining unit tests.
    - **Required Artifacts**: `test-results/local-patch-report.md` and `test-results/local-patch-report.json`.
    - **Expected Behavior**: Report may warn (non-blocking rollout), but artifact generation is mandatory.

3. **Security Scans** (MANDATORY - Zero Tolerance):
    - **CodeQL Go Scan**: Run VS Code task "Security: CodeQL Go Scan (CI-Aligned)" OR `pre-commit run codeql-go-scan --all-files`
        - Must use `security-and-quality` suite (CI-aligned)
        - **Zero high/critical (error-level) findings allowed**
        - Medium/low findings should be documented and triaged
    - **CodeQL JS Scan**: Run VS Code task "Security: CodeQL JS Scan (CI-Aligned)" OR `pre-commit run codeql-js-scan --all-files`
        - Must use `security-and-quality` suite (CI-aligned)
        - **Zero high/critical (error-level) findings allowed**
        - Medium/low findings should be documented and triaged
    - **Validate Findings**: Run `pre-commit run codeql-check-findings --all-files` to check for HIGH/CRITICAL issues
    - **Trivy Container Scan**: Run VS Code task "Security: Trivy Scan" for container/dependency vulnerabilities
    - **Results Viewing**:
        - Primary: VS Code SARIF Viewer extension (`MS-SarifVSCode.sarif-viewer`)
        - Alternative: `jq` command-line parsing: `jq '.runs[].results' codeql-results-*.sarif`
        - CI: GitHub Security tab for automated uploads
    - **⚠️ CRITICAL:** CodeQL scans are NOT run by default pre-commit hooks (manual stage for performance). You MUST run them explicitly via VS Code tasks or pre-commit manual commands before completing any task.
    - **Why:** CI enforces security-and-quality suite and blocks HIGH/CRITICAL findings. Local verification prevents CI failures and ensures security compliance.
    - **CI Alignment:** Local scans now use identical parameters to CI:
        - Query suite: `security-and-quality` (61 Go queries, 204 JS queries)
        - Database creation: `--threads=0 --overwrite`
        - Analysis: `--sarif-add-baseline-file-info`

4. **Pre-Commit Triage**: Run `pre-commit run --all-files`.
    - If errors occur, **fix them immediately**.
    - If logic errors occur, analyze and propose a fix.
    - Do not output code that violates pre-commit standards.

5. **Staticcheck BLOCKING Validation**: Pre-commit hooks automatically run fast linters including staticcheck.
    - **CRITICAL:** Staticcheck errors are BLOCKING - you MUST fix them before commit succeeds.
    - Manual verification: Run VS Code task "Lint: Staticcheck (Fast)" or `make lint-fast`
    - To check only staticcheck: `make lint-staticcheck-only`
    - Test files (`_test.go`) are excluded from staticcheck (matches CI behavior)
    - If pre-commit fails: Fix the reported issues, then retry commit
    - **Do NOT** use `--no-verify` to bypass this check unless emergency hotfix

6. **Coverage Testing** (MANDATORY - Non-negotiable):
    - **Overall Coverage**: Minimum 85% coverage is MANDATORY and will fail the PR if not met.
    - **Patch Coverage**: Developers should aim for 100% coverage of modified lines (Codecov Patch view). If patch coverage is incomplete, add targeted tests. However, patch coverage is a suggestion and will not block PR approval.
    - **Backend Changes**: Run the VS Code task "Test: Backend with Coverage" or execute `scripts/go-test-coverage.sh`.
        - Minimum coverage: 85% (set via `CHARON_MIN_COVERAGE` or `CPM_MIN_COVERAGE`).
        - If coverage drops below threshold, write additional tests to restore coverage.
        - All tests must pass with zero failures.
    - **Frontend Changes**: Run the VS Code task "Test: Frontend with Coverage" or execute `scripts/frontend-test-coverage.sh`.
        - Minimum coverage: 85% (set via `CHARON_MIN_COVERAGE` or `CPM_MIN_COVERAGE`).
        - If coverage drops below threshold, write additional tests to restore coverage.
        - All tests must pass with zero failures.
    - **Critical**: Coverage tests are NOT run by default pre-commit hooks (they are in manual stage for performance). You MUST run them explicitly via VS Code tasks or scripts before completing any task.
    - **Why**: CI enforces coverage in GitHub Actions. Local verification prevents CI failures and maintains code quality.

7. **Type Safety** (Frontend only):
    - Run the VS Code task "Lint: TypeScript Check" or execute `cd frontend && npm run type-check`.
    - Fix all type errors immediately. This is non-negotiable.
    - This check is also in manual stage for performance but MUST be run before completion.

8. **Verify Build**: Ensure the backend compiles and the frontend builds without errors.
    - Backend: `cd backend && go build ./...`
    - Frontend: `cd frontend && npm run build`

9. **Fixed and New Code Testing**:
    - Ensure all existing and new unit tests pass with zero failures.
    - When failures and errors are found, deep-dive into root causes. Using the correct `subAgent`, update the working plan, review the implementation, and fix the issues.
    - No issue is out of scope for investigation and resolution. All issues must be addressed before task completion.

10. **Clean Up**: Ensure no debug print statements or commented-out blocks remain.
    - Remove `console.log`, `fmt.Println`, and similar debugging statements.
    - Delete commented-out code blocks.
    - Remove unused imports.

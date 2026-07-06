# Charon — Claude Code Instructions

Do NOT use worktrees. Make all changes directly on the current working branch.

## Code Quality Guidelines

Every session should improve the codebase, not just add to it. Actively refactor code you encounter, even outside of your immediate task scope. Think about long-term maintainability and consistency. Make a detailed plan before writing code. Always create unit tests for new code coverage.

- **ARCHITECTURE AWARENESS**: Always consult `ARCHITECTURE.md` at the repository root before making significant changes to core components, system architecture, technology stack, deployment configuration, or directory structure.
- **DRY**: Consolidate duplicate patterns into reusable functions, types, or components after the second occurrence.
- **CLEAN**: Delete dead code immediately. Remove unused imports, variables, functions, types, commented code, and console logs.
- **LEVERAGE**: Use battle-tested packages over custom implementations.
- **READABLE**: Maintain comments and clear naming for complex logic. Favor clarity over cleverness.
- **CONVENTIONAL COMMITS**: Write commit messages using `feat:`, `fix:`, `chore:`, `refactor:`, or `docs:` prefixes.

## Governance & Precedence

When policy statements conflict across documentation sources:

1. **Highest Precedence**: This `CLAUDE.md` file (canonical source of truth for Claude Code)
2. **Agent Overrides**: `.claude/agents/**` files (agent-specific customizations)
3. **Operator Documentation**: `SECURITY.md`, `docs/security.md`, `docs/features/notifications.md`

**Reconciliation Rule**: When conflicts arise, the stricter security requirement wins.

## 🚨 CRITICAL ARCHITECTURE RULES 🚨

- **Single Frontend Source**: All frontend code MUST reside in `frontend/`. NEVER create `backend/frontend/` or any other nested frontend directory.
- **Single Backend Source**: All backend code MUST reside in `backend/`.
- **No Python**: This is a Go (Backend) + React/TypeScript (Frontend) project. Do not introduce Python scripts or requirements.

## 🛑 Root Cause Analysis Protocol (MANDATORY)

**Constraint:** You must NEVER patch a symptom without tracing the root cause.
If a bug is reported, do NOT stop at the first error message found. Trace the entire flow from frontend action to backend processing. Identify the true origin of the issue.

**The "Context First" Rule:**
Before proposing ANY code change or fix, build a mental map of the feature:
1. **Entry Point:** Where does the data enter? (API Route / UI Event)
2. **Transformation:** How is the data modified? (Handlers / Middleware)
3. **Persistence:** Where is it stored? (DB Models / Files)
4. **Exit Point:** How is it returned to the user?

**Anti-Pattern Warning:**
- Do not assume the error log is the *cause*; it is often just the *victim* of an upstream failure.
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
- **Static Analysis (BLOCKING)**: Fast linters run automatically on every commit via lefthook pre-commit-phase hooks.
  - **Staticcheck errors MUST be fixed** — commits are BLOCKED until resolved
  - Manual run: `make lint-fast` or `make lint-staticcheck-only`
  - Full golangci-lint (all linters): Use `make lint-backend` before PR (manual stage)
- **API Response**: Handlers return structured errors using `gin.H{"error": "message"}`.
- **JSON Tags**: All struct fields exposed to the frontend MUST have explicit `json:"snake_case"` tags.
- **IDs**: UUIDs (`github.com/google/uuid`) are generated server-side; clients never send numeric IDs.
- **Security**: Sanitize all file paths using `filepath.Clean`. Use `fmt.Errorf("context: %w", err)` for error wrapping.
- **Graceful Shutdown**: Long-running work must respect `server.Run(ctx)`.

### Troubleshooting Lefthook Staticcheck Failures

1. **"golangci-lint not found"** — Install and ensure `$GOPATH/bin` is in PATH.
2. **Staticcheck SA1019 (deprecated API)** — Replace deprecated function with recommended alternative.
3. **"This value is never used" (SA4006)** — Remove unused assignment or use the value.
4. **"Should replace if statement with..." (S10xx)** — Apply suggested simplification.
5. **Emergency bypass (use sparingly)**: `git commit --no-verify -m "Emergency hotfix"` — MUST create follow-up issue.

## Frontend Workflow

- **Location**: Always work within `frontend/`.
- **Stack**: React 18 + Vite + TypeScript + TanStack Query (React Query).
- **State Management**: Use `src/hooks/use*.ts` wrapping React Query.
- **API Layer**: Create typed API clients in `src/api/*.ts` that wrap `client.ts`.
- **Forms**: Use local `useState` for form fields, submit via `useMutation`, then `invalidateQueries` on success.

## Cross-Cutting Notes

- **Sync**: React Query expects the exact JSON produced by GORM tags (snake_case). Keep API and UI field names aligned.
- **Migrations**: When adding models, update `internal/models` AND `internal/api/routes/routes.go` (AutoMigrate).
- **Testing**: All new code MUST include accompanying unit tests.
- **Ignore Files**: Always check `.gitignore`, `.dockerignore`, and `.codecov.yml` when adding new files or folders.

## Documentation

- **Architecture**: Update `ARCHITECTURE.md` when making changes to system architecture, technology stack, directory structure, deployment model, security architecture, or integration points.
- **Features**: Update `docs/features.md` when adding capabilities — keep it brief, link to individual docs.

## CI/CD & Commit Conventions

- **Triggers**: Use `feat:`, `fix:`, or `perf:` to trigger Docker builds. `chore:` skips builds.
- **Beta**: `feature/beta-release` always builds.
- **Weekly Promotion PRs** (`nightly → main`): ALWAYS merge using **"Create a merge commit"** — NEVER squash or rebase. Squash merging collapses all commits into bullet lines that the `auto-versioning` workflow cannot parse, silently preventing minor version bumps and producing empty release notes.
- **History-Rewrite PRs**: If a PR touches files in `scripts/history-rewrite/` or `docs/plans/history_rewrite.md`, the PR description MUST include the history-rewrite checklist from `.github/PULL_REQUEST_TEMPLATE/history-rewrite.md`.

## PR Sizing & Decomposition

- **Default Rule**: Prefer smaller, reviewable PRs over one large PR when work spans multiple domains.
- **Split into Multiple PRs When**: the change touches backend + frontend + infrastructure/security; the diff is large enough to reduce review quality; work can be delivered in independently testable slices; a foundational refactor is needed before feature delivery.
- **Suggested PR Sequence**:
  1. Foundation PR (types/contracts/refactors, no behavior change)
  2. Backend PR (API/model/service changes + tests)
  3. Frontend PR (UI integration + tests)
  4. Hardening PR (security/CI/docs/follow-up fixes)
- **Per-PR Requirement**: Every PR must remain deployable, pass DoD checks, and include a clear dependency note on prior PRs.

## ✅ Task Completion Protocol (Definition of Done)

Before marking an implementation task as complete, perform the following in order:

1. **Playwright E2E Tests** (MANDATORY — Run First):
   - **Run**: `cd /projects/Charon && npx playwright test --project=firefox` from project root
   - **Scope**: Run tests relevant to modified features
   - **On Failure**: Trace root cause through frontend → backend flow before proceeding
   - All E2E tests must pass before proceeding to unit tests

1.5. **GORM Security Scan** (CONDITIONAL, BLOCKING):
   - **Trigger**: Execute when changes include `backend/internal/models/**`, GORM queries, or migrations
   - **Exclusions**: Skip for docs-only or frontend-only changes
   - **Run**: `./scripts/scan-gorm-security.sh --check` — must report zero CRITICAL/HIGH findings

2. **Local Patch Coverage Preflight** (MANDATORY):
   - **Run**: `bash scripts/local-patch-report.sh` from repo root
   - **Required Artifacts**: `test-results/local-patch-report.md` and `test-results/local-patch-report.json`

3. **Security Scans** (MANDATORY — Zero Tolerance):
   - **CodeQL Go Scan**: `lefthook run pre-commit` — zero high/critical findings allowed
   - **CodeQL JS Scan**: `lefthook run pre-commit` — zero high/critical findings allowed
   - **Trivy Container Scan**: `make trivy` or equivalent for container/dependency vulnerabilities
   - Results viewed via `jq '.runs[].results' codeql-results-*.sarif`

4. **Lefthook Triage**: Run `lefthook run pre-commit`. Fix all errors immediately.

5. **Staticcheck BLOCKING Validation**: Pre-commit hooks automatically run fast linters including staticcheck.
   - Manual verification: `make lint-fast` or `make lint-staticcheck-only`
   - **Do NOT** use `--no-verify` unless emergency hotfix

6. **Coverage Testing** (MANDATORY — Non-negotiable):
   - **Overall Coverage**: Minimum 85% coverage — will fail the PR if not met
   - **Backend**: `scripts/go-test-coverage.sh` — minimum 85% (`CHARON_MIN_COVERAGE`)
   - **Frontend**: `scripts/frontend-test-coverage.sh` — minimum 85%
   - All tests must pass with zero failures

7. **Type Safety** (Frontend only):
   - Run `cd frontend && npm run type-check` — fix all type errors immediately

8. **Verify Build**:
   - Backend: `cd backend && go build ./...`
   - Frontend: `cd frontend && npm run build`

9. **Fixed and New Code Testing**:
   - Ensure all existing and new unit tests pass with zero failures
   - Deep-dive into root causes when failures occur — all issues must be addressed

10. **Clean Up**: Remove debug print statements, commented-out blocks, `console.log`, `fmt.Println`, unused imports.

## Subagents

Use the specialized agents in `.claude/agents/` for complex tasks:

- **management** — Engineering Director; orchestrates all other agents for large features
- **planning** — Principal Architect; creates `docs/plans/current_spec.md`
- **supervisor** — Code Review Lead; reviews plans and implementations
- **backend-dev** — Senior Go Engineer; implements backend tasks (TDD)
- **frontend-dev** — Senior React/TypeScript Engineer; implements frontend tasks
- **qa-security** — QA & Security Engineer; testing and vulnerability assessment
- **devops** — CI/CD specialist; deployment debugging and GitOps
- **docs-writer** — Technical Writer; user-facing documentation
- **playwright-dev** — E2E Testing Specialist; Playwright test automation

## Skills (Common Tasks)

Run project skills via the skill runner:

```bash
.github/skills/scripts/skill-runner.sh <skill-name>
```

Available skills (see `.github/skills/*.SKILL.md` for full docs):

| Skill | Purpose |
|-------|---------|
| `docker-start-dev` | Start dev Docker Compose environment |
| `docker-stop-dev` | Stop dev environment |
| `docker-rebuild-e2e` | Rebuild E2E test container |
| `docker-prune` | Clean up Docker resources |
| `test-backend-unit` | Run backend unit tests |
| `test-backend-coverage` | Run backend tests with coverage |
| `test-frontend-unit` | Run frontend unit tests |
| `test-frontend-coverage` | Run frontend tests with coverage |
| `test-e2e-playwright` | Run Playwright E2E tests |
| `security-scan-codeql` | Run CodeQL security scan |
| `security-scan-trivy` | Run Trivy vulnerability scan |
| `security-scan-gorm` | Run GORM security scan |
| `security-scan-go-vuln` | Run Go vulnerability check |
| `integration-test-all` | Run all integration tests |
| `utility-version-check` | Check tool versions |

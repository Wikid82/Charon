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
- **`(security)` SCOPE**: For genuinely security-relevant `feat`/`fix` commits (real vulnerability fixes, new protective mechanisms — not general bug fixes), use `feat(security): <subject>` or `fix(security): <subject>`. This scope feeds a dedicated "Security" category in the What's New changelog, so it's reserved for real security work — overusing it for visibility on ordinary fixes dilutes the category's signal. **Vague by default**: the subject line must describe the *category* of issue and mitigation in general terms, and must NEVER reveal the specific vulnerability class, attack vector, or exact vulnerable code path — the changelog displays it verbatim to every self-hosted user, including ones running un-upgraded, still-vulnerable instances. Good: `fix(security): harden input validation in the API layer`. Bad: `fix(security): fix SQL injection in host search filter`.

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

### Troubleshooting CodeQL (Local Dev / Sandbox)

`lefthook run codeql` (`scripts/pre-commit-hooks/codeql-go-scan.sh` /
`codeql-js-scan.sh`) calls a bare `codeql` binary with no version pinning —
it assumes a working CLI is already on PATH. Sandbox/dev-container images
can ship a stale system `codeql` (observed: 2.16.0) that fails against this
repo's pinned query packs (`codeql/go-queries:codeql-suites/go-security-and-quality.qls`
and the JS equivalent) with `extensions-by-pack` resolution / version-mismatch
errors. **This is a local/dev-environment issue only** — CI
(`.github/workflows/codeql.yml`) drives CodeQL entirely through
`github/codeql-action@v4` (pinned by SHA), which downloads and manages its
own CLI toolchain independent of whatever is on the runner's PATH, so CI is
never affected by this.

1. **Fix**: Run `bash scripts/install-codeql.sh`. It installs the official
   `gh-codeql` GitHub CLI extension (if missing), pins it to a known-working
   CodeQL CLI version for this repo's query packs, and installs a `codeql`
   shim on PATH (`gh codeql install-stub`) that forwards to `gh codeql` —
   every existing script that calls bare `codeql` then works unmodified.
   Safe to re-run.
2. **Manual equivalent**, if you need to do it by hand:

   ```bash
   gh extension install github/gh-codeql   # once
   gh codeql set-version v2.26.0           # matches this repo's query pack pins
   gh codeql install-stub "$HOME/.local/bin"
   ```

3. **Verify**: `codeql version` should report `2.26.0` or later and
   `lefthook run codeql` should complete without pack-resolution errors.
4. If a newer/older pin is ever needed, update `CODEQL_VERSION` at the top
   of `scripts/install-codeql.sh` and re-validate `lefthook run codeql`
   locally before committing the bump.

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

## Commit Slicing & PR Strategy

- **One Feature = One PR**: A feature merges only when it is complete. NEVER split a single feature across multiple PRs (e.g., separate backend/frontend/security PRs) — plans must produce a **Commit Slicing Strategy**, not a PR slicing strategy.
- **Slice Commits, Not PRs**: Decompose the work into ordered, logical commits within the single feature PR. Each commit defines its scope, files, dependencies, and validation gate.
- **Suggested Commit Sequence** within the PR:
  1. E2E specs for new behavior (as `test.fixme` until implemented)
  2. Foundation (types/contracts/refactors, no behavior change)
  3. Backend (API/model/service changes + tests)
  4. Frontend (UI integration + tests)
  5. Hardening + enable E2E + docs
- **Per-Commit Requirement**: Each commit should build and pass its validation gate; the PR as a whole must pass the full Definition of Done before merge.

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

## Execution Discipline: Foreground-Only Commands (MANDATORY)

**All agents — Management and every subagent — MUST run commands in the foreground and block until they complete. Never background a long-running command (`run_in_background: true`, `&`, `nohup`, or any detached/async invocation) and end your turn to "check back later" or "wait for the notification."**

**Why:** Backgrounding a command and pausing your turn to wait for it does not reliably resume you. In practice this has repeatedly caused agents in this pipeline to go silently idle indefinitely — no report, no error, just gone — leaving Management (or whoever dispatched them) waiting on a result that never arrives on its own.

**Rule:**
- Run tests, builds, coverage scripts, E2E suites, linters, and Docker builds as blocking, foreground calls with a generous timeout (up to the maximum allowed per call).
- If a command genuinely needs longer than a single call's timeout allows, re-issue a blocking wait within your own turn until you have a real result. Do not end your turn assuming something else will wake you back up.
- **If a call auto-backgrounds anyway** (the tool's own timeout forces this, e.g. a 300s/600s cap you didn't choose): that is NOT permission to end your turn and wait for a notification. Immediately re-attach to it — poll or block on it again in the same turn, repeatedly if necessary — until you have a real result. Ending the turn at that point is exactly the disallowed pattern this rule exists to prevent, even though it wasn't your explicit choice to background it.
- Never report a task as "running, will report when it lands" and then go idle. Either finish with a real result in the same turn, or explicitly hand off incomplete work with a clearly stated reason — never stall silently.
- Applies to every long-running step across the pipeline: `npx playwright test`, `npx vitest run`, `go test`, `scripts/go-test-coverage.sh`, `scripts/local-patch-report.sh`, Docker image builds, `lefthook run pre-commit`, etc.

## Subagents

**MANDATORY**: All work performed in this repository — features, bug fixes, refactors, and investigations alike — MUST go through the **management** agent pipeline. Do not implement changes directly in the main session; dispatch to the `management` agent, which orchestrates planning, implementation, review, and QA via the other subagents below.

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

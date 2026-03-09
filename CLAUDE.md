# Charon — Claude Code Instructions

> **Governance Precedence** (highest → lowest)
> 1. `.github/instructions/**` files — canonical source of truth
> 2. `.claude/agents/**` files — agent-specific overrides
> 3. `SECURITY.md`, `docs/security.md`, `docs/features/**` — operator docs

When conflicts arise, the stricter security requirement always wins. Update downstream docs to match canonical text.

---

## Project Overview

Charon is a self-hosted web app for managing reverse proxy host configurations, aimed at novice users. Everything prioritises simplicity, usability, reliability, and security — delivered as a single binary + static assets with no external dependencies.

- **Backend**: `backend/cmd/api` → loads config, opens SQLite, hands off to `internal/server`
- **Frontend**: React app built to `frontend/dist`, mounted via `attachFrontend`
- **Config**: `internal/config` respects `CHARON_ENV`, `CHARON_HTTP_PORT`, `CHARON_DB_PATH`; creates `data/`
- **Models**: Persistent types in `internal/models`; GORM auto-migrates them

---

## Code Quality Rules

Every session should improve the codebase, not just add to it.

- **MANDATORY**: Before starting any task, read relevant files in `.github/instructions/` for that domain
- **ARCHITECTURE**: Consult `ARCHITECTURE.md` before changing core components, data flow, tech stack, deployment config, or directory structure
- **DRY**: Consolidate duplicate patterns into reusable functions/types after the second occurrence
- **CLEAN**: Delete dead code immediately — unused imports, variables, functions, commented blocks, console logs
- **LEVERAGE**: Use battle-tested packages over custom implementations
- **READABLE**: Maintain comments and clear naming for complex logic
- **CONVENTIONAL COMMITS**: Use `feat:`, `fix:`, `chore:`, `refactor:`, or `docs:` prefixes

---

## Critical Architecture Rules

- **Single Frontend Source**: All frontend code MUST reside in `frontend/`. NEVER create `backend/frontend/` or nested directories
- **Single Backend Source**: All backend code MUST reside in `backend/`
- **No Python**: Go (Backend) + React/TypeScript (Frontend) only

---

## Root Cause Analysis Protocol (MANDATORY)

**Never patch a symptom without tracing the root cause.**

Before any code change, build a mental map of the feature:
1. **Entry Point** — Where does the data enter? (API Route / UI Event)
2. **Transformation** — How is data modified? (Handlers / Middleware)
3. **Persistence** — Where is it stored? (DB Models / Files)
4. **Exit Point** — How is it returned to the user?

The error log is often the *victim*, not the *cause*. Search upstream callers to find the origin.

---

## Backend Workflow

- **Run**: `cd backend && go run ./cmd/api`
- **Test**: `go test ./...`
- **Lint (BLOCKING)**: `make lint-fast` or `make lint-staticcheck-only` — staticcheck errors block commits
- **Full lint** (before PR): `make lint-backend`
- **API Responses**: `gin.H{"error": "message"}` structured errors
- **JSON Tags**: All struct fields exposed to frontend MUST have `json:"snake_case"` tags
- **IDs**: UUIDs (`github.com/google/uuid`), generated server-side
- **Error wrapping**: `fmt.Errorf("context: %w", err)`
- **File paths**: Sanitise with `filepath.Clean`

---

## Frontend Workflow

- **Location**: Always work inside `frontend/`
- **Stack**: React 18 + Vite + TypeScript + TanStack Query
- **State**: `src/hooks/use*.ts` wrapping React Query
- **API Layer**: Typed clients in `src/api/*.ts` wrapping `client.ts`
- **Forms**: Local `useState` → `useMutation` → `invalidateQueries` on success

---

## Cross-Cutting Notes

- **VS Code**: Register new repetitive CLI actions in `.vscode/tasks.json`
- **Sync**: React Query expects exact JSON from GORM tags (snake_case) — keep API and UI aligned
- **Migrations**: When adding models, update `internal/models` AND `internal/api/routes/routes.go` (AutoMigrate)
- **Testing**: All new code MUST include unit tests
- **Ignore files**: Check `.gitignore`, `.dockerignore`, `.codecov.yml` when adding files/folders

---

## Documentation

Update `ARCHITECTURE.md` when changing: system architecture, tech stack, directory structure, deployment, security, integrations.
Update `docs/features.md` when adding capabilities (short marketing-style list only).

---

## CI/CD & Commit Conventions

- `feat:`, `fix:`, `perf:` → trigger Docker builds; `chore:` → skips builds
- `feature/beta-release` branch always builds
- History-rewrite PRs (touching `scripts/history-rewrite/`) MUST include checklist from `.github/PULL_REQUEST_TEMPLATE/history-rewrite.md`

---

## PR Sizing

Prefer smaller, reviewable PRs. Split when changes span backend + frontend + infra, or diff is large.

**Suggested PR sequence**:
1. Foundation PR (types/contracts/refactors, no behaviour change)
2. Backend PR (API/model/service + tests)
3. Frontend PR (UI integration + tests)
4. Hardening PR (security/CI/docs/follow-ups)

Each PR must remain deployable and pass DoD checks.

---

## Definition of Done (MANDATORY — in order)

1. **Playwright E2E** (run first): `cd /projects/Charon && npx playwright test --project=firefox`
   - Scope to modified features; fix root cause before proceeding on failure
2. **GORM Security Scan** (conditional — if models/DB changed): `./scripts/scan-gorm-security.sh --check` — zero CRITICAL/HIGH
3. **Local Patch Coverage Preflight**: `bash scripts/local-patch-report.sh` — produces `test-results/local-patch-report.md`
4. **Security Scans** (zero high/critical):
   - CodeQL Go: VS Code task "Security: CodeQL Go Scan (CI-Aligned)"
   - CodeQL JS: VS Code task "Security: CodeQL JS Scan (CI-Aligned)"
   - Trivy: VS Code task "Security: Trivy Scan"
5. **Lefthook**: `lefthook run pre-commit` — fix all errors immediately
6. **Staticcheck (BLOCKING)**: `make lint-fast` — must pass before commit
7. **Coverage (MANDATORY — 85% minimum)**:
   - Backend: VS Code task "Test: Backend with Coverage" or `scripts/go-test-coverage.sh`
   - Frontend: VS Code task "Test: Frontend with Coverage" or `scripts/frontend-test-coverage.sh`
8. **Type Safety** (frontend): `cd frontend && npm run type-check` — fix all errors
9. **Build verification**: `cd backend && go build ./...` + `cd frontend && npm run build`
10. **All tests pass**: `go test ./...` + `npm test`
11. **Clean up**: No `console.log`, `fmt.Println`, debug statements, or commented-out blocks

---

## Agents

Specialised subagents live in `.claude/agents/`. Invoke with `@agent-name` or let Claude Code route automatically based on task type:

| Agent | Role |
|---|---|
| `management` | Engineering Director — delegates all work, never implements directly |
| `planning` | Principal Architect — research, technical specs, implementation plans |
| `supervisor` | Code Review Lead — PR reviews, quality assurance |
| `backend-dev` | Senior Go Engineer — Gin/GORM/SQLite implementation |
| `frontend-dev` | Senior React/TypeScript Engineer — UI implementation |
| `qa-security` | QA & Security Engineer — testing, vulnerability assessment |
| `doc-writer` | Technical Writer — user-facing documentation |
| `playwright-dev` | E2E Testing Specialist — Playwright test automation |
| `devops` | DevOps Specialist — CI/CD, GitHub Actions, deployments |

## Commands

Slash commands in `.claude/commands/` — invoke with `/command-name`:

| Command | Purpose |
|---|---|
| `/create-implementation-plan` | Draft a structured implementation plan file |
| `/update-implementation-plan` | Update an existing plan |
| `/breakdown-feature` | Break a feature into implementable tasks |
| `/create-technical-spike` | Research a technical question |
| `/create-github-issues` | Generate GitHub issues from an implementation plan |
| `/sa-plan` | Structured Autonomy — planning phase |
| `/sa-generate` | Structured Autonomy — generate phase |
| `/sa-implement` | Structured Autonomy — implement phase |
| `/debug-web-console` | Debug browser console errors |
| `/playwright-explore` | Explore a website with Playwright |
| `/playwright-generate-test` | Generate a Playwright test |
| `/sql-code-review` | Review SQL / stored procedures |
| `/sql-optimization` | Optimise a SQL query |
| `/codecov-patch-fix` | Fix Codecov patch coverage gaps |
| `/supply-chain-remediation` | Remediate supply chain vulnerabilities |
| `/ai-prompt-safety-review` | Review AI prompt for safety/security |
| `/prompt-builder` | Build a structured AI prompt |

## Skills (Docker / Testing / Security)

Skill runner: `.github/skills/scripts/skill-runner.sh <skill-name>`

| Skill | Command |
|---|---|
| Start dev environment | `/docker-start-dev` |
| Stop dev environment | `/docker-stop-dev` |
| Rebuild E2E container | `/docker-rebuild-e2e` |
| Prune Docker resources | `/docker-prune` |
| Run all integration tests | `/integration-test-all` |
| Backend unit tests | `/test-backend-unit` |
| Backend coverage | `/test-backend-coverage` |
| Frontend unit tests | `/test-frontend-unit` |
| Frontend coverage | `/test-frontend-coverage` |
| E2E Playwright tests | `/test-e2e-playwright` |
| CodeQL scan | `/security-scan-codeql` |
| Trivy scan | `/security-scan-trivy` |
| Docker image scan | `/security-scan-docker-image` |
| GORM security scan | `/security-scan-gorm` |
| Go vulnerability scan | `/security-scan-go-vuln` |

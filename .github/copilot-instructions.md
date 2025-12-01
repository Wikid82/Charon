# Charon Copilot Instructions

## Code Quality Guidelines
Every session should improve the codebase, not just add to it. Actively refactor code you encounter, even outside of your immediate task scope. Think about long-term maintainability and consistency. Make a detailed plan before writing code. Always create unit tests for new code coverage.

- **DRY**: Consolidate duplicate patterns into reusable functions, types, or components after the second occurrence.
- **CLEAN**: Delete dead code immediately. Remove unused imports, variables, functions, types, commented code, and console logs.
- **LEVERAGE**: Use battle-tested packages over custom implementations.
- **READABLE**: Maintain comments and clear naming for complex logic. Favor clarity over cleverness.
- **CONVENTIONAL COMMITS**: Write commit messages using `feat:`, `fix:`, `chore:`, `refactor:`, or `docs:` prefixes.

## 🚨 CRITICAL ARCHITECTURE RULES 🚨
- **Single Frontend Source**: All frontend code MUST reside in `frontend/`. NEVER create `backend/frontend/` or any other nested frontend directory.
- **Single Backend Source**: All backend code MUST reside in `backend/`.
- **No Python**: This is a Go (Backend) + React/TypeScript (Frontend) project. Do not introduce Python scripts or requirements.

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
- **API Response**: Handlers return structured errors using `gin.H{"error": "message"}`.
- **JSON Tags**: All struct fields exposed to the frontend MUST have explicit `json:"snake_case"` tags.
- **IDs**: UUIDs (`github.com/google/uuid`) are generated server-side; clients never send numeric IDs.
- **Security**: Sanitize all file paths using `filepath.Clean`. Use `fmt.Errorf("context: %w", err)` for error wrapping.
- **Graceful Shutdown**: Long-running work must respect `server.Run(ctx)`.

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
- **Features**: Update `docs/features.md` when adding capabilities.
- **Links**: Use GitHub Pages URLs (`https://wikid82.github.io/charon/`) for docs and GitHub blob links for repo files.

## CI/CD & Commit Conventions
- **Triggers**: Use `feat:`, `fix:`, or `perf:` to trigger Docker builds. `chore:` skips builds.
- **Beta**: `feature/beta-release` always builds.

## ✅ Task Completion Protocol (Definition of Done)
Before marking an implementation task as complete, perform the following:
1.  **Pre-Commit Triage**: Run `pre-commit run --all-files`.
    -   If errors occur, **fix them immediately**.
    -   If logic errors occur, analyze and propose a fix.
    -   Do not output code that violates pre-commit standards.
2.  **Verify Build**: Ensure the backend compiles and the frontend builds without errors.
3.  **Clean Up**: Ensure no debug print statements or commented-out blocks remain.

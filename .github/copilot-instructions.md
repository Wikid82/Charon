# CaddyProxyManager+ Copilot Instructions

## 🚨 CRITICAL ARCHITECTURE RULES 🚨
- **Single Frontend Source**: All frontend code MUST reside in `frontend/`. NEVER create `backend/frontend/` or any other nested frontend directory.
- **Single Backend Source**: All backend code MUST reside in `backend/`.
- **No Python**: This is a Go (Backend) + React/TypeScript (Frontend) project. Do not introduce Python scripts or requirements.

## Big Picture
- `backend/cmd/api` loads config, opens SQLite, then hands off to `internal/server` where routes from `internal/api/routes` are registered.
- `internal/config` respects `CPM_ENV`, `CPM_HTTP_PORT`, `CPM_DB_PATH`, `CPM_FRONTEND_DIR` and creates the `data/` directory; lean on these instead of hard-coded paths.
- All HTTP endpoints live under `/api/v1/*`; keep new handlers inside `internal/api/handlers` and register them via `routes.Register` so `db.AutoMigrate` runs for their models.
- `internal/server` also mounts the built React app (via `attachFrontend`) whenever `frontend/dist` exists, falling back to JSON `{"error": ...}` for any `/api/*` misses.
- Persistent types live in `internal/models`; GORM auto-migrates them each boot, so evolve schemas there before touching handlers or the frontend.

## Backend Workflow
- Run locally with `cd backend && go run ./cmd/api`; run tests with `go test ./...` (see `proxy_host_handler_test.go` for the in-memory SQLite/Gin harness pattern).
- Handlers return structured errors using `gin.H{"error": "message"}` and standard HTTP codes—mirror the `ProxyHostHandler` lifecycle for new CRUD endpoints.
- UUIDs (`github.com/google/uuid`) are generated server-side and exposed as `uuid` fields; clients never send numeric IDs.
- Query lists sorted by `updated_at desc` (see `.Order("updated_at desc")` in `List`); match that ordering for user-visible collections.
- Long-running work must respect the graceful shutdown flow in `server.Run(ctx)`—avoid background goroutines that ignore the context.

## Frontend Workflow
- **Location**: Always work within `frontend/`.
- **Stack**: React 18 + Vite + TypeScript + TanStack Query (React Query).
- **State Management**: Use `src/hooks/use*.ts` wrapping React Query. Do not use raw `useEffect` for data fetching.
- **API Layer**: Create typed API clients in `src/api/*.ts` that wrap `client.ts`.
- **Development**: Run `cd frontend && npm run dev`. Vite proxies `/api` to `http://localhost:8080`.
- **Components**: Screens live in `src/pages`. Reusable UI in `src/components`.
- **Forms**: Use local `useState` for form fields, submit via `useMutation` from custom hooks, then `invalidateQueries` on success.

## Cross-Cutting Notes
- Run the backend before the frontend; React Query expects the exact JSON produced by GORM tags (snake_case), so keep API and UI field names aligned.
- When adding models, update both `internal/models` and the `AutoMigrate` call inside `internal/api/routes/routes.go`; register new Gin routes right after migrations for clarity.
- Tests belong beside handlers (`*_test.go`); reuse the `setupTestRouter` helper structure (in-memory SQLite, Gin router, httptest requests) for fast feedback.
- **Testing Requirement**: All new code (features, bug fixes, refactors) MUST include accompanying unit tests. Ensure tests cover happy paths and error conditions.
- **Ignore Files**: When creating new file types, directories, or build artifacts, ALWAYS check and update `.gitignore`, `.dockerignore`, and `.codecov.yml` to ensure they are properly excluded or included as required.
- The root `Dockerfile` builds the Go binary and the React static assets (multi-stage build).
- Branch from `feature/**` and target `development`.

## CI/CD & Commit Conventions
- **Docker Builds**: The `docker-publish` workflow skips builds for commits starting with `chore:`.
- **Triggering Builds**: To ensure a new Docker image is built (e.g., for testing on VPS), use `feat:`, `fix:`, or `perf:` prefixes.
- **Beta Branch**: The `feature/beta-release` branch is configured to ALWAYS build, overriding the skip logic.

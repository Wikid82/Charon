# CaddyProxyManager+ Copilot Instructions

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
- React 18 + Vite + React Query; start with `cd frontend && npm install && npm run dev` so Vite proxies `/api` calls to `http://localhost:8080` (configured in `vite.config.ts`).
- Consolidate HTTP calls via `src/api/client.ts`; wrap them in hooks under `src/hooks` and expose query keys like `['proxy-hosts']` to keep cache invalidation simple.
- Screens live in `src/pages` and render inside `components/Layout`; navigation + active styles rely on React Router + `clsx`, so extend the `links` array instead of hard-coding routes elsewhere.
- Forms follow `pages/ProxyHosts.tsx`: local `useState` per field, submit via `useMutation`, then reset state and `invalidateQueries` for the affected list on success.
- Styling remains a single `src/index.css` grid/aside theme; keep additions lightweight and avoid new design systems until shadcn/ui lands.

## Cross-Cutting Notes
- Run the backend before the frontend; React Query expects the exact JSON produced by GORM tags (snake_case), so keep API and UI field names aligned.
- When adding models, update both `internal/models` and the `AutoMigrate` call inside `internal/api/routes/routes.go`; register new Gin routes right after migrations for clarity.
- Tests belong beside handlers (`*_test.go`); reuse the `setupTestRouter` helper structure (in-memory SQLite, Gin router, httptest requests) for fast feedback.
- The root `Dockerfile` is still the legacy Python scaffold—do not assume it builds this stack until it is replaced with the Go/React pipeline.
- Branch from `feature/**` and target `development`; CI currently lints/tests placeholders, so run `go test ./...` and `npm run build` locally before opening PRs.

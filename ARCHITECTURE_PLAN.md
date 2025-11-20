# CaddyProxyManager+ Architecture Plan

## Stack Overview
- **Backend**: Go 1.24, Gin HTTP framework, GORM ORM, SQLite for local/stateful storage.
- **Frontend**: React 18 + TypeScript with Vite, React Query for data fetching, React Router for navigation.
- **API Contract**: REST/JSON over `/api/v1`, versioned to keep room for breaking changes.
- **Deployment**: Container-first via multi-stage Docker build (Node → Go), future compose bundle for Caddy runtime.

## Backend
- `backend/cmd/api`: Entry point wires configuration, database, and HTTP server lifecycle.
- `internal/config`: Reads environment variables (`CPM_ENV`, `CPM_HTTP_PORT`, `CPM_DB_PATH`). Defaults to `development`, `8080`, `./data/cpm.db` respectively.
- `internal/database`: Wraps GORM + SQLite connection handling and enforces data-directory creation.
- `internal/server`: Creates Gin engine, registers middleware, wires graceful shutdown, and exposes `Run(ctx)` for signal-aware lifecycle.
- `internal/api`: Versioned routing layer. Initial resources:
  - `GET /api/v1/health`: Simple status response for readiness checks.
  - CRUD `/api/v1/proxy-hosts`: Minimal data model used to validate persistence, shape matches Issue #1 requirements (name, domain, upstream target, toggles).
- `internal/models`: Source of truth for persistent entities. Future migrations will extend `ProxyHost` with SSL, ACL, audit metadata.
- Testing: In-memory SQLite harness verifies handler lifecycle via unit tests (`go test ./...`).

## Frontend
- Vite dev server with proxy to `http://localhost:8080` for `/api` paths keeps CORS trivial.
- React Router organizes initial pages (Dashboard, Proxy Hosts, System Status) to mirror Issue roadmap.
- React Query centralizes API caching, invalidation, and loading states.
- Basic layout shell provides left-nav reminiscent of NPM while keeping styling simple (CSS utility file, no design system yet). Future work will slot shadcn/ui components without rewriting data layer.
- Build outputs static assets in `frontend/dist` consumed by Docker multi-stage for production.

## Data & Persistence
- SQLite chosen for Alpha milestone simplicity; GORM migrates schema automatically on boot (`AutoMigrate`).
- Database path configurable via env to allow persistent volumes in Docker or alternative DB (PostgreSQL/MySQL) when scaling.

## API Principles
1. **Version Everything** (`/api/v1`).
2. **Stateless**: Each request carries all context; session/story features will rely on cookies/JWT later.
3. **Dependable validation**: Gin binding ensures HTTP 400 responses include validation errors.
4. **Observability**: Gin logging + structured error responses keep early debugging simple; plan to add Zap/zerolog instrumentation during Beta.

## Local Development Workflow
1. Start backend: `cd backend && go run ./cmd/api`.
2. Start frontend: `cd frontend && npm run dev` (Vite proxy sends API calls to backend automatically).
3. Optional: run both via Docker (see updated Dockerfile) once containers land.
4. Tests:
   - Backend: `cd backend && go test ./...`
   - Frontend build check: `cd frontend && npm run build`

## Next Steps
- Layer authentication (Issue #7) once scaffolding lands.
- Expand data model (certificates, access lists) and add migrations.
- Replace basic CSS with component system (e.g., shadcn/ui) + design tokens.
- Compose file bundling backend, frontend assets, Caddy runtime, and SQLite volume.

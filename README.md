# CaddyProxyManager+

CaddyProxyManager+ is a modern web UI and management layer that brings Nginx Proxy Manager-style simplicity to Caddy, with extra security add-ons (CrowdSec, WAF, SSO, etc.).

This repository now ships the first working slices of the Go backend and Vite/React frontend described in `ARCHITECTURE_PLAN.md`.

Quick links
- Project board: https://github.com/users/Wikid82/projects/7
- Issues: https://github.com/Wikid82/CaddyProxyManagerPlus/issues

## Tech stack
- **Backend**: Go 1.22, Gin, GORM, SQLite (configurable path via env vars)
- **Frontend**: React 18 + TypeScript, Vite bundler, React Query, React Router
- **API**: REST over `/api/v1`, currently exposes `health` + proxy host CRUD

See `ARCHITECTURE_PLAN.md` for the detailed rationale and roadmap for each tier.

## Getting started

### Prerequisites
- Go 1.22+
- Node.js 20+
- SQLite3

### Quick Start (using Makefile)
```bash
# Install all dependencies
make install

# Run tests
make test

# Run backend
make run

# Run frontend (in another terminal)
make run-frontend

# Or run both with tmux
make dev
```

### Manual Setup

#### Backend API
```bash
cd backend
cp .env.example .env # optional overrides
go run ./cmd/api
```

Run tests:
```bash
cd backend
go test ./...
```

#### Frontend UI
```bash
cd frontend
npm install
npm run dev
```

The Vite dev server proxies `/api/*` to `http://localhost:8080` so long as the backend is running locally.

Build for production:
```bash
cd frontend
npm run build
```

### Docker Deployment (Recommended)

CaddyProxyManager+ is designed to run in Docker with Caddy as a sidecar container.

```bash
# Production deployment
docker-compose up -d

# Development mode (exposes Caddy admin API on :2019)
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up
```

The docker-compose stack includes:
- **app**: CaddyProxyManager+ management interface (`:8080`)
- **caddy**: Caddy reverse proxy (`:80`, `:443`, `:443/udp` for HTTP/3)

Data is persisted in Docker volumes:
- `app_data`: CPM+ database and config snapshots
- `caddy_data`: Caddy certificates and data
- `caddy_config`: Caddy configuration

**Docker images** are published to GitHub Container Registry with automatic semantic versioning:
```bash
# Latest stable (from main branch)
docker pull ghcr.io/wikid82/caddyproxymanagerplus:latest

# Development (from development branch)
docker pull ghcr.io/wikid82/caddyproxymanagerplus:development

# Specific version (recommended for production)
docker pull ghcr.io/wikid82/caddyproxymanagerplus:v1.0.0

# Major/minor version (auto-updates to latest patch)
docker pull ghcr.io/wikid82/caddyproxymanagerplus:1.0
```

See `VERSION.md` for complete versioning documentation.

### Tooling
- **Build system**: `Makefile` provides common development tasks (`make help` for all commands)
- **Branching model**: `development` is the integration branch; open PRs from `feature/**`
- **CI**: `.github/workflows/ci.yml` runs Go tests, ESLint, and frontend builds
- **Docker**: Multi-stage build with Node (frontend) → Go (backend) → Alpine runtime
- **Pre-commit**: `.pre-commit-config.yaml` runs formatters, linters, and now `go test` with coverage enforcement (`CPM_MIN_COVERAGE=75` by default)

## Contributing
- See `CONTRIBUTING.md` (coming soon) for contribution guidelines.

## License
- MIT License – see `LICENSE`.

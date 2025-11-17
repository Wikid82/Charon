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

### Docker Deployment
```bash
# Build the image
make docker-build

# Run the container
make docker-run

# Or manually:
docker build -t caddyproxymanager-plus .
docker run -p 8080:8080 -v cpm-data:/app/data caddyproxymanager-plus
```

### Tooling
- **Build system**: `Makefile` provides common development tasks (`make help` for all commands)
- **Branching model**: `development` is the integration branch; open PRs from `feature/**`
- **CI**: `.github/workflows/ci.yml` runs Go tests, ESLint, and frontend builds
- **Docker**: Multi-stage build with Node (frontend) → Go (backend) → Alpine runtime

## Contributing
- See `CONTRIBUTING.md` (coming soon) for contribution guidelines.

## License
- MIT License – see `LICENSE`.

.PHONY: help install test build run clean docker-build docker-run release

# Default target
help:
	@echo "Charon Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  install                - Install all dependencies (backend + frontend)"
	@echo "  test                   - Run all tests (backend + frontend)"
	@echo "  build                  - Build backend and frontend"
	@echo "  run                    - Run backend in development mode"
	@echo "  clean                  - Clean build artifacts"
	@echo "  docker-build           - Build Docker image"
	@echo "  docker-build-versioned - Build Docker image with version from .version file"
	@echo "  docker-run             - Run Docker container"
	@echo "  docker-dev             - Run Docker in development mode"
	@echo "  release                - Create a new semantic version release (interactive)"
	@echo "  dev                    - Run both backend and frontend in dev mode (requires tmux)"
	@echo ""
	@echo "Security targets:"
	@echo "  security-scan          - Quick security scan (govulncheck on Go deps)"
	@echo "  security-scan-full     - Full container scan with Trivy"
	@echo "  security-scan-deps     - Check for outdated Go dependencies"

# Install all dependencies
install:
	@echo "Installing backend dependencies..."
	cd backend && go mod download
	@echo "Installing frontend dependencies..."
	cd frontend && npm install

# Run all tests
test:
	@echo "Running backend tests..."
	cd backend && go test -v ./...
	@echo "Running frontend lint..."
	cd frontend && npm run lint

# Build backend and frontend
build:
	@echo "Building frontend..."
	cd frontend && npm run build
	@echo "Building backend..."
	cd backend && go build -o bin/api ./cmd/api

build-versioned:
	@echo "Building frontend (versioned)..."
	cd frontend && VITE_APP_VERSION=$$(git describe --tags --always --dirty) npm run build
	@echo "Building backend (versioned)..."
	cd backend && \
	VERSION=$$(git describe --tags --always --dirty); \
	GIT_COMMIT=$$(git rev-parse --short HEAD); \
	BUILD_DATE=$$(date -u +'%Y-%m-%dT%H:%M:%SZ'); \
	go build -ldflags "-X github.com/Wikid82/charon/backend/internal/version.Version=$$VERSION -X github.com/Wikid82/charon/backend/internal/version.GitCommit=$$GIT_COMMIT -X github.com/Wikid82/charon/backend/internal/version.BuildTime=$$BUILD_DATE" -o bin/api ./cmd/api

# Run backend in development mode
run:
	cd backend && go run ./cmd/api

# Run frontend in development mode
run-frontend:
	cd frontend && npm run dev

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf backend/bin backend/data
	rm -rf frontend/dist frontend/node_modules
	go clean -cache

# Build Docker image
docker-build:
	docker-compose build

# Build Docker image with version
docker-build-versioned:
 	@VERSION=$$(cat .version 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	BUILD_DATE=$$(date -u +'%Y-%m-%dT%H:%M:%SZ'); \
	VCS_REF=$$(git rev-parse HEAD 2>/dev/null || echo "unknown"); \
	docker build \
		--build-arg VERSION=$$VERSION \
		--build-arg BUILD_DATE=$$BUILD_DATE \
		--build-arg VCS_REF=$$VCS_REF \
		-t charon:$$VERSION \
		-t charon:latest \
		.

# Run Docker containers (production)
docker-run:
	docker-compose up -d

# Run Docker containers (development)
docker-dev:
	docker-compose -f docker-compose.yml -f docker-compose.dev.yml up

# Stop Docker containers
docker-stop:
	docker-compose down

# View Docker logs
docker-logs:
	docker-compose logs -f

# Development mode (requires tmux)
dev:
	@command -v tmux >/dev/null 2>&1 || { echo "tmux is required for dev mode"; exit 1; }
	tmux new-session -d -s charon 'cd backend && go run ./cmd/api'
	tmux split-window -h -t charon 'cd frontend && npm run dev'
	tmux attach -t charon

# Create a new release (interactive script)
release:
	@./scripts/release.sh

# Security scanning targets
security-scan:
	@echo "Running security scan (govulncheck)..."
	@./scripts/security-scan.sh

security-scan-full:
	@echo "Building local Docker image for security scan..."
	docker build --build-arg VCS_REF=$(shell git rev-parse HEAD) -t charon:local .
	@echo "Running Trivy container scan..."
	docker run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(HOME)/.cache/trivy:/root/.cache/trivy \
		aquasec/trivy:latest image \
		--severity CRITICAL,HIGH \
		charon:local

security-scan-deps:
	@echo "Scanning Go dependencies..."
	cd backend && go list -m -json all | docker run --rm -i aquasec/trivy:latest sbom --format json - 2>/dev/null || true
	@echo "Checking for Go module updates..."
	cd backend && go list -m -u all | grep -E '\[.*\]' || echo "All modules up to date"

# Quality Assurance targets
lint-backend:
	@echo "Running golangci-lint..."
	cd backend && docker run --rm -v $(PWD)/backend:/app -w /app golangci/golangci-lint:latest golangci-lint run -v

lint-docker:
	@echo "Running Hadolint..."
	docker run --rm -i hadolint/hadolint < Dockerfile

test-race:
	@echo "Running Go tests with race detection..."
	cd backend && go test -race -v ./...

check-module-coverage:
	@echo "Running module-specific coverage checks (backend + frontend)"
	@bash scripts/check-module-coverage.sh

benchmark:
	@echo "Running Go benchmarks..."
	cd backend && go test -bench=. -benchmem ./...

integration-test:
	@echo "Running integration tests..."
	@./scripts/integration-test.sh

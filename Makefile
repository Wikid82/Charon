.PHONY: help install test build run clean docker-build docker-run

# Default target
help:
	@echo "CaddyProxyManager+ Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  install       - Install all dependencies (backend + frontend)"
	@echo "  test          - Run all tests (backend + frontend)"
	@echo "  build         - Build backend and frontend"
	@echo "  run           - Run backend in development mode"
	@echo "  clean         - Clean build artifacts"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  dev           - Run both backend and frontend in dev mode (requires tmux)"

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
	tmux new-session -d -s cpm 'cd backend && go run ./cmd/api'
	tmux split-window -h -t cpm 'cd frontend && npm run dev'
	tmux attach -t cpm

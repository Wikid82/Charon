# Project Map

## Purpose
Charon: self-hosted web app for managing reverse proxy host configs (novice-friendly), shipped as one binary + static assets (per CLAUDE.md).

## Requirements
Simplicity, usability, reliability, security; no external dependencies (per CLAUDE.md).

## Components
- backend/ — Go (module github.com/Wikid82/charon/backend); cmd/api entry, internal/ (config, server, models, api/routes)
- frontend/ — React 19 + Vite + TanStack Query, TypeScript
- agent/, plugins/, configs/ — not yet inspected
- docs/, docs-site/, ARCHITECTURE.md — documentation
- tests/, playwright* — E2E tests

## Main Flow
Browser (React) --API--> Gin handlers (internal/api) --> services? --> GORM/SQLite (internal/models). Details unverified beyond CLAUDE.md.

## Data and Trust Boundaries
SQLite via GORM per CLAUDE.md; auth details unknown (unverified).

## Build and Deployment
Backend: `cd backend && go run ./cmd/api`, `go test ./...`. Frontend: `cd frontend && npm run build`. Dockerfile, Makefile, lefthook.yml at root.

## Unknowns
Service layer structure, Caddy integration, agent/ role, auth model.

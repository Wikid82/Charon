#!/bin/bash
# Deterministic, fast backend build step for CI/CodeQL extraction
# Use `go list` to avoid long-running builds and network downloads.
# Set GOPROXY to a standard proxy to avoid interactive network issues.
set -euo pipefail
cd backend
export GOPROXY=${GOPROXY:-https://proxy.golang.org}
export GOMODCACHE=${GOMODCACHE:-$(go env GOMODCACHE)}
# First, list packages for fast JS extraction/diagnostics
go list ./...
# Ensure dependencies are downloaded and run a proper Go build so CodeQL can extract symbols
go mod download
go build ./...

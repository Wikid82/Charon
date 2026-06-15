#!/bin/bash

# This script updates Go module dependencies for all modules in the project.

set -euo pipefail

GOPATH_BIN="$(go env GOPATH)/bin"
export PATH="$GOPATH_BIN:$PATH"
command -v govulncheck >/dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest

MODULES=(
    "/projects/Charon/backend"
    "/projects/Charon/agent"
)

for MODULE in "${MODULES[@]}"; do
    echo "============================================================================"
    echo "Updating: $MODULE"
    echo "============================================================================"

    cd "$MODULE" || exit 1

    # Update go/toolchain directives so Renovate's golang updates have nothing to do
    go get go@latest toolchain@latest
    # -t includes test-only dependencies, which Renovate also tracks
    go get -u -t ./...
    go mod tidy
    go mod verify
    go vet ./...
    go build ./...
    go test ./... > /dev/null
    govulncheck ./...

    echo "Done: $MODULE"
done

cd /projects/Charon || exit 1
go work sync

echo ""
echo "All Go module dependencies updated successfully."

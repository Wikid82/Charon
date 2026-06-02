#!/bin/bash

# This script updates Go module dependencies for all modules in the project.

set -euo pipefail

MODULES=(
    "/projects/Charon/backend"
    "/projects/Charon/agent"
)

for MODULE in "${MODULES[@]}"; do
    echo "============================================================================"
    echo "Updating: $MODULE"
    echo "============================================================================"

    cd "$MODULE" || exit 1

    go get -u ./...
    go mod tidy
    go mod verify
    go vet ./...
    go list -m -u all
    go build ./...

    echo "Done: $MODULE"
done

cd /projects/Charon || exit 1
go work sync

echo ""
echo "All Go module dependencies updated successfully."

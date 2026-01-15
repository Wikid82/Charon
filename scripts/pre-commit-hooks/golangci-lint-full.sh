#!/usr/bin/env bash
set -euo pipefail

# Wrapper script for golangci-lint full linters in pre-commit
# This ensures golangci-lint works in both terminal and VS Code pre-commit integration

# Find golangci-lint in common locations
GOLANGCI_LINT=""

# Check if already in PATH
if command -v golangci-lint >/dev/null 2>&1; then
    GOLANGCI_LINT="golangci-lint"
else
    # Check common installation locations
    COMMON_PATHS=(
        "$HOME/go/bin/golangci-lint"
        "/usr/local/bin/golangci-lint"
        "/usr/bin/golangci-lint"
        "${GOPATH:-$HOME/go}/bin/golangci-lint"
    )

    for path in "${COMMON_PATHS[@]}"; do
        if [[ -x "$path" ]]; then
            GOLANGCI_LINT="$path"
            break
        fi
    done
fi

# Exit if not found
if [[ -z "$GOLANGCI_LINT" ]]; then
    echo "ERROR: golangci-lint not found in PATH or common locations"
    echo "Searched:"
    echo "  - PATH: $PATH"
    echo "  - $HOME/go/bin/golangci-lint"
    echo "  - /usr/local/bin/golangci-lint"
    echo "  - /usr/bin/golangci-lint"
    echo ""
    echo "Install from: https://golangci-lint.run/usage/install/"
    exit 1
fi

# Change to backend directory and run golangci-lint
cd "$(dirname "$0")/../../backend" || exit 1
exec "$GOLANGCI_LINT" run -v ./...

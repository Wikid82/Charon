#!/usr/bin/env bash
set -euo pipefail

# Wrapper script for golangci-lint fast linters in pre-commit
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

# Version compatibility check
# Extract Go versions from golangci-lint and system Go
LINT_GO_VERSION=$("$GOLANGCI_LINT" version 2>/dev/null | grep -oP 'go\K[0-9]+\.[0-9]+(?:\.[0-9]+)?' || echo "")
SYSTEM_GO_VERSION=$(go version 2>/dev/null | grep -oP 'go\K[0-9]+\.[0-9]+(?:\.[0-9]+)?' || echo "")

if [[ -n "$LINT_GO_VERSION" && -n "$SYSTEM_GO_VERSION" && "$LINT_GO_VERSION" != "$SYSTEM_GO_VERSION" ]]; then
    echo "⚠️  golangci-lint Go version mismatch detected:"
    echo "   golangci-lint: $LINT_GO_VERSION"
    echo "   system Go:     $SYSTEM_GO_VERSION"
    echo ""
    echo "🔧 Auto-rebuilding golangci-lint with current Go version..."

    if go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest 2>&1; then
        echo "✅ golangci-lint rebuilt successfully"

        # Re-verify the tool is still accessible
        if command -v golangci-lint >/dev/null 2>&1; then
            GOLANGCI_LINT="golangci-lint"
        else
            GOLANGCI_LINT="$HOME/go/bin/golangci-lint"
        fi
    else
        echo "❌ Failed to rebuild golangci-lint"
        echo "   Please run manually: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
        exit 1
    fi
    echo ""
fi

# Change to backend directory and run golangci-lint
cd "$(dirname "$0")/../../backend" || exit 1
exec "$GOLANGCI_LINT" run --config .golangci-fast.yml ./...

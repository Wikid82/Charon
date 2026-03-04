#!/usr/bin/env bash
set -euo pipefail

# Wrapper script for golangci-lint fast linters in pre-commit
# This ensures golangci-lint works in both terminal and VS Code pre-commit integration

preferred_bin="${GOBIN:-${GOPATH:-$HOME/go}/bin}/golangci-lint"

lint_major_version() {
    local binary_path="$1"
    "$binary_path" version 2>/dev/null | sed -nE 's/.*version[[:space:]]+([0-9]+)\..*/\1/p' | sed -n '1p'
}

install_v2_linter() {
    echo "🔧 Installing golangci-lint v2 with current Go toolchain..."
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
}

resolve_v2_linter() {
    local candidates=()
    local path_linter=""

    if path_linter=$(command -v golangci-lint 2>/dev/null); then
        candidates+=("$path_linter")
    fi

    candidates+=(
        "$preferred_bin"
        "$HOME/go/bin/golangci-lint"
        "/usr/local/bin/golangci-lint"
        "/usr/bin/golangci-lint"
    )

    for candidate in "${candidates[@]}"; do
        if [[ -x "$candidate" && "$(lint_major_version "$candidate")" == "2" ]]; then
            printf '%s\n' "$candidate"
            return 0
        fi
    done

    install_v2_linter

    if [[ -x "$preferred_bin" && "$(lint_major_version "$preferred_bin")" == "2" ]]; then
        printf '%s\n' "$preferred_bin"
        return 0
    fi

    return 1
}

if ! GOLANGCI_LINT="$(resolve_v2_linter)"; then
    echo "ERROR: failed to resolve golangci-lint v2"
    echo "PATH: $PATH"
    echo "Expected v2 binary at: $preferred_bin"
    exit 1
fi

echo "Using golangci-lint: $GOLANGCI_LINT"
echo "Version: $($GOLANGCI_LINT version)"

# Change to backend directory and run golangci-lint
cd "$(dirname "$0")/../../backend" || exit 1
exec "$GOLANGCI_LINT" run --config .golangci-fast.yml ./...

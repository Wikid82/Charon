#!/usr/bin/env bash
set -euo pipefail

# Auto-fix linting issues before blocking the commit.
# Applies gocritic (simplifyCompositeLit, etc.) and other fixable linters,
# re-stages the corrected files, then exits non-zero so the user can review
# and re-commit. If nothing was changed, exits 0 silently.

preferred_bin="${GOBIN:-${GOPATH:-$HOME/go}/bin}/golangci-lint"

lint_major_version() {
    local binary_path="$1"
    "$binary_path" version 2>/dev/null | sed -nE 's/.*version[[:space:]]+([0-9]+)\..*/\1/p' | sed -n '1p'
}

install_v2_linter() {
    echo "🔧 Installing golangci-lint v2..." >&2
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest >&2
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
    echo "ERROR: failed to resolve golangci-lint v2" >&2
    exit 1
fi

cd "$(dirname "$0")/../../backend" || exit 1

# Capture which files exist before the fix run
before_hash=$(git diff --name-only 2>/dev/null || true)

# Run with --fix — only applies auto-fixable issues (gocritic, gofmt, etc.)
# --new-from-rev HEAD keeps scope to lines changed in this commit
"$GOLANGCI_LINT" run --config .golangci-fast.yml --fix --new-from-rev HEAD ./... 2>&1 || true

after_hash=$(git diff --name-only 2>/dev/null || true)

if [[ "$before_hash" != "$after_hash" ]]; then
    # Re-stage all modified Go files
    git add -- "$(git diff --name-only "*.go" 2>/dev/null | tr '\n' ' ')" 2>/dev/null || \
        git add -u -- "**/*.go" 2>/dev/null || true

    echo ""
    echo "go-lint-fix: auto-fixed linting issues in the following files:"
    git diff --cached --name-only -- "*.go" 2>/dev/null || true
    echo ""
    echo "Review the changes above, then re-run: git commit"
    exit 1
fi

exit 0

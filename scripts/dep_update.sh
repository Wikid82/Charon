#!/bin/bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ---------------------------------------------------------------------------
# Go modules
# ---------------------------------------------------------------------------

GOPATH_BIN="$(go env GOPATH)/bin"
export PATH="$GOPATH_BIN:$PATH"
command -v govulncheck >/dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest

GO_MODULES=(
    "$REPO_ROOT/backend"
    "$REPO_ROOT/agent"
)

for MODULE in "${GO_MODULES[@]}"; do
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
    go test ./...
    govulncheck ./...

    echo "Done: $MODULE"
done

cd "$REPO_ROOT" || exit 1
go work sync

echo ""
echo "All Go module dependencies updated successfully."

# ---------------------------------------------------------------------------
# npm modules
# ---------------------------------------------------------------------------

export PATH="/usr/share/nodejs/corepack/shims:$PATH"

NPM_MODULES=(
    "$REPO_ROOT"
    "$REPO_ROOT/frontend"
)

for MODULE in "${NPM_MODULES[@]}"; do
    echo "============================================================================"
    echo "Updating: $MODULE"
    echo "============================================================================"

    cd "$MODULE" || exit 1

    # Update prod, dev, optional, peer, and packageManager dependencies.
    npx npm-check-updates -u

    # Also update flat (string-valued) entries in the "overrides" section.
    # npm-check-updates excludes "overrides" from its default --dep list, so
    # packages declared there (e.g. smol-toml, js-yaml, markdown-it) are
    # silently skipped without this extra pass.
    #
    # The frontend package.json contains nested object overrides
    # (e.g. { "eslint-plugin-react-hooks": { "eslint": "^x.y" } }) which
    # cause ncu to crash when --dep overrides is used without a filter.
    # To avoid that, we run a separate targeted pass that only touches the
    # known flat top-level override ("typescript") in the frontend.
    if [ "$MODULE" = "$REPO_ROOT/frontend" ]; then
        # Update only the flat top-level override; skip nested object entries.
        npx npm-check-updates -u --dep overrides --filter typescript
    else
        # Root package.json has only flat string overrides — safe to update all
        # except js-yaml, which has breaking changes in v6+; keep pinned to ^5.
        npx npm-check-updates -u --dep overrides --reject js-yaml
    fi

    rm -rf node_modules package-lock.json
    npm install --ignore-scripts
    npm dedupe
    npm run --if-present build
    npm run --if-present type-check
    npm run --if-present test -- --run
    npm audit --audit-level=high
    npm audit fix || true
    npm outdated || true

    echo "Done: $MODULE"
done

echo ""
echo "All npm dependencies updated successfully."

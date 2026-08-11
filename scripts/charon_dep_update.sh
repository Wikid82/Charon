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

echo "============================================================================"
echo "Updating Global npm Environment"
echo "============================================================================"

echo "Current local versions (npm / npx):"
npm -v && npx -v

echo "Latest available npm version on registry: "
npm view npm version

echo "Installing latest global npm..."
npm install -g npm@latest
echo ""

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
    # Exclude typescript: v7 crashes @typescript-eslint until upstream
    # catches up; keep pinned to ^6.0.3 until that's resolved.
    # Exclude @types/eslint-plugin-jsx-a11y: 6.10.1+ regressed to a real
    # eslint@^9 dependency (vs. the types-only @types/eslint@* in 6.10.0),
    # reintroducing GHSA-mh99-v99m-4gvg (brace-expansion DoS). Keep
    # exact-pinned to 6.10.0 until upstream ships a fixed release.
    npx --yes npm-check-updates -u --reject typescript,@types/eslint-plugin-jsx-a11y

    # Also update flat (string-valued) entries in the "overrides" section.
    # npm-check-updates excludes "overrides" from its default --dep list, so
    # packages declared there (e.g. smol-toml, js-yaml, markdown-it) are
    # silently skipped without this extra pass.
    #
    # The frontend package.json contains nested object overrides
    # (e.g. { "eslint-plugin-react-hooks": { "eslint": "^x.y" } }) which
    # cause ncu to crash when --dep overrides is used without a filter, and
    # its only flat override (typescript) is already pinned above — so
    # there's nothing left to safely update there; skip frontend entirely.
    if [ "$MODULE" != "$REPO_ROOT/frontend" ]; then
        # Root package.json has only flat string overrides — safe to update all
        # except js-yaml, which has breaking changes in v6+; keep pinned to ^5.
        npx --yes npm-check-updates -u --dep overrides --reject js-yaml
    fi

    rm -rf node_modules package-lock.json
    npm install
    npm dedupe
    npm run --if-present build
    npm run --if-present type-check
    # Both modules gate on their own audit-ci.json rather than a blanket
    # --audit-level: root's allowlist is empty today, frontend's allowlists
    # exactly one known-unfixable finding (eslint-plugin-jsx-a11y's real
    # dependency on minimatch@3.1.5 -> brace-expansion@1.1.16,
    # GHSA-mh99-v99m-4gvg — see SECURITY.md). Any other new high/critical
    # finding in either module still fails the script.
    npm run audit:ci
    npm audit fix || true
    npm outdated || true

    echo "Done: $MODULE"
done

echo ""
echo "All npm dependencies updated successfully."

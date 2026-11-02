#!/bin/bash


set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"


# ---------------------------------------------------------------------------
# Go modules
# ---------------------------------------------------------------------------

    GOPATH_BIN="$(go env GOPATH)/bin"
    export PATH="$GOPATH_BIN:$PATH"
    command -v govulncheck >/dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest

    GO_MODULES=(
        "$REPO_ROOT/backend"
        "$REPO_ROOT/agent"
        "$REPO_ROOT/plugins/powerdns"
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

        # Always (re)install after the toolchain bump above, not just when missing:
        # a staticcheck/govulncheck binary built with the old toolchain can fail to
        # parse export data from a newer one (e.g. "export data version N is greater
        # than maximum supported version"). Installing here picks up the just-bumped
        # go directive so the tool is built with a matching toolchain.
        go install honnef.co/go/tools/cmd/staticcheck@latest
        go install golang.org/x/vuln/cmd/govulncheck@latest

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

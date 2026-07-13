#!/usr/bin/env bash
# Pin GOTOOLCHAIN to the latest Go release, prune stale cached toolchains,
# and kick gopls so it picks up the new version without a manual editor reload.
#
# gopls is a long-running process: it resolves its Go toolchain once at
# startup and never re-reads GOTOOLCHAIN afterwards. Without a restart here,
# the editor keeps reporting "go.work requires go >= X (running go Y)" even
# though `go version` in a fresh shell is already correct.

set -euo pipefail

LATEST_VERSION="$(go list -m -f '{{.Version}}' go@latest)"
echo "Latest Go release: go${LATEST_VERSION}"

go env -w "GOTOOLCHAIN=go${LATEST_VERSION}+auto"
echo "GOTOOLCHAIN pinned to go${LATEST_VERSION}+auto"

TOOLCHAIN_CACHE="$(go env GOMODCACHE)/golang.org"
if [ -d "$TOOLCHAIN_CACHE" ]; then
    echo "Pruning cached toolchains older than go${LATEST_VERSION}..."
    for dir in "$TOOLCHAIN_CACHE"/toolchain@v*; do
        [ -e "$dir" ] || continue
        case "$dir" in
            *"go${LATEST_VERSION}."*) continue ;;
        esac
        chmod -R u+w "$dir"
        rm -rf "$dir"
        echo "  removed $(basename "$dir")"
    done
fi

if pkill -x gopls 2>/dev/null; then
    echo "Restarted gopls (killed stale process; your editor will respawn it)."
else
    echo "No running gopls process found to restart."
fi

echo ""
go version
echo ""
echo "Done. If your editor still shows a stale Go version, run 'Developer: Reload Window'."

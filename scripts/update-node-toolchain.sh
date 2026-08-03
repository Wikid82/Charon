#!/usr/bin/env bash
# Install fnm (if missing) and switch the local Node toolchain to the version
# pinned in frontend/.nvmrc — the same version CI pins via NODE_VERSION across
# .github/workflows/*.yml. Mirrors update-go-toolchain.sh's role for Go: keep
# local dev on the version the project actually targets instead of whatever
# the OS package manager happens to ship.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NVMRC="$REPO_ROOT/frontend/.nvmrc"
TARGET_VERSION="$(tr -d '[:space:]' <"$NVMRC")"

echo "Target Node version (frontend/.nvmrc): ${TARGET_VERSION}"

FNM_DIR="${FNM_DIR:-$HOME/.local/share/fnm}"
if ! command -v fnm >/dev/null 2>&1; then
    echo "fnm not found, installing..."
    FNM_INSTALLER="$(mktemp)"
    trap 'rm -f "$FNM_INSTALLER"' EXIT
    curl -fsSL https://fnm.vercel.app/install -o "$FNM_INSTALLER"
    bash "$FNM_INSTALLER" --skip-shell
    rm -f "$FNM_INSTALLER"
    trap - EXIT
fi

if ! command -v fnm >/dev/null 2>&1 && [ -x "$FNM_DIR/fnm" ]; then
    export PATH="$FNM_DIR:$PATH"
fi

eval "$(fnm env --shell bash)"

fnm install "$TARGET_VERSION"
fnm use "$TARGET_VERSION"
fnm default "$TARGET_VERSION"

echo ""
node -v
echo ""
echo "Done. Add 'eval \"\$(fnm env --use-on-cd)\"' to your shell rc (once) so new"
echo "shells auto-switch when you cd into a directory with a .nvmrc."

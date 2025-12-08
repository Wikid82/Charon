#!/usr/bin/env bash
set -euo pipefail

# Clear Go caches and gopls cache
echo "Clearing Go build and module caches..."
go clean -cache -testcache -modcache || true

echo "Clearing gopls cache..."
rm -rf "${XDG_CACHE_HOME:-$HOME/.cache}/gopls" || true

echo "Re-downloading modules..."
cd backend || exit 1
go mod download

echo "Caches cleared and modules re-downloaded."

# Provide instructions for next steps
cat <<'EOF'
Next steps:
- Restart your editor's Go language server (gopls)
  - In VS Code: Command Palette -> 'Go: Restart Language Server'
- Verify the toolchain:
  $ go version
  $ gopls version
EOF

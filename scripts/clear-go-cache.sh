#!/usr/bin/env bash
set -euo pipefail

# ⚠️  DEPRECATED: This script is deprecated and will be removed in v2.0.0
#    Please use: .github/skills/scripts/skill-runner.sh utility-clear-go-cache
#    For more info: docs/AGENT_SKILLS_MIGRATION.md
echo "⚠️  WARNING: This script is deprecated and will be removed in v2.0.0" >&2
echo "    Please use: .github/skills/scripts/skill-runner.sh utility-clear-go-cache" >&2
echo "    For more info: docs/AGENT_SKILLS_MIGRATION.md" >&2
echo "" >&2
sleep 1

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

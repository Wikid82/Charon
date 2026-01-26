#!/usr/bin/env bash
set -euo pipefail
TMP=$(mktemp -d)
REPO_ROOT=$(cd "$(dirname "$0")/../../" && pwd)
cd "$TMP"
git init -q
echo hi > README.md
git add README.md
git commit -q -m init
mkdir -p .venv/bin
cat > .venv/bin/pre-commit <<'PRE'
#!/usr/bin/env sh
exit 0
PRE
chmod +x .venv/bin/pre-commit
echo "temp repo: $TMP"
# Use the configured REPO_ROOT rather than hardcoding /projects/Charon.
# Note: avoid a leading slash before "$REPO_ROOT" which would make the path invalid
# on different hosts; use "$REPO_ROOT/scripts/..." directly.
"$REPO_ROOT/scripts/history-rewrite/validate_after_rewrite.sh" || echo "first run rc $?"
"$REPO_ROOT/scripts/history-rewrite/validate_after_rewrite.sh" --backup-branch backup/main || echo "second run rc $?"
echo exit status $?

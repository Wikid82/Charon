#!/usr/bin/env bash
set -euo pipefail
TMP=$(mktemp -d)
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
/projects/Charon/scripts/history-rewrite/validate_after_rewrite.sh || echo "first run rc $?"
/projects/Charon/scripts/history-rewrite/validate_after_rewrite.sh --backup-branch backup/main || echo "second run rc $?"
echo exit status $?

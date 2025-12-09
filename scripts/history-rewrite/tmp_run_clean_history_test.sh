#!/usr/bin/env bash
set -euo pipefail
TMPREMOTE=$(mktemp -d)
git init --bare "$TMPREMOTE/remote.git"
TMPCLONE=$(mktemp -d)
cd "$TMPCLONE"
git clone "$TMPREMOTE/remote.git" .
# create a commit
mkdir -p backend/codeql-db
echo 'dummy' > backend/codeql-db/foo.txt
git add -A
git commit -m "Add dummy file" -q
git checkout -b feature/test
# set up stub git-filter-repo in PATH
## Resolve the repo root based on the script file location (Bash-safe)
# Use ${BASH_SOURCE[0]} instead of $0 to correctly resolve the script path even
# when invoked from different PWDs or via sourced contexts.
REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../" && pwd -P)
TMPBIN=$(mktemp -d)
cat > "$TMPBIN/git-filter-repo" <<'SH'
#!/usr/bin/env sh
# Minimal stub to simulate git-filter-repo
while [ $# -gt 0 ]; do
  shift
done
exit 0
SH
chmod +x "$TMPBIN/git-filter-repo"
export PATH="$TMPBIN:$PATH"
# run clean_history.sh with dry-run
# NOTE: Avoid hard-coded repo paths like /projects/Charon/
# Use the dynamically-derived REPO_ROOT (above) or a relative path from this script
# so this helper script runs correctly on other machines/CI environments.
# Examples:
#  "$REPO_ROOT/scripts/history-rewrite/clean_history.sh" --dry-run ...
#  "$(dirname "$0")/clean_history.sh" --dry-run ...
"$REPO_ROOT/scripts/history-rewrite/clean_history.sh" --dry-run --paths 'backend/codeql-db' --strip-size 1
# run clean_history.sh with force should attempt to push branch then succeed (requires that remote exists)
"$REPO_ROOT/scripts/history-rewrite/clean_history.sh" --force --paths 'backend/codeql-db' --strip-size 1 <<'IN'
I UNDERSTAND
IN

# test non-interactive with force
"$REPO_ROOT/scripts/history-rewrite/clean_history.sh" --force --non-interactive --paths 'backend/codeql-db' --strip-size 1

# cleanup
rm -rf "$TMPREMOTE" "$TMPCLONE" "$TMPBIN"
echo 'done'

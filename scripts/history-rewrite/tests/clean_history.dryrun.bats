#!/usr/bin/env bats

setup() {
  TMPREPO=$(mktemp -d)
  cd "$TMPREPO"
  git init -q
  # Set local git identity for test commits
  git config user.email "test@example.com"
  git config user.name "Test Runner"
  # create a directory that matches the paths to be pruned
  mkdir -p backend/codeql-db
  # add a large fake blob file
  dd if=/dev/zero of=backend/codeql-db/largefile.bin bs=1M count=2 >/dev/null 2>&1 || true
  git add -A && git commit -m 'add large blob' -q
  git checkout -b feature/test
  # Create a local bare repo to act as origin and allow git push
  TMPORIGIN=$(mktemp -d)
  git init --bare "$TMPORIGIN" >/dev/null
  git remote add origin "$TMPORIGIN"
  git push -u origin feature/test >/dev/null 2>&1 || true
  # Add a stub git-filter-repo to PATH to satisfy requirements without installing
  STUBBIN=$(mktemp -d)
  cat > "$STUBBIN/git-filter-repo" <<'SH'
#!/usr/bin/env bash
echo "stub git-filter-repo called: $@"
exit 0
SH
  chmod +x "$STUBBIN/git-filter-repo"
  PATH="$STUBBIN:$PATH"
}

teardown() {
  rm -rf "$TMPREPO"
}

REPO_ROOT=$(cd "$BATS_TEST_DIRNAME/../../../" && pwd)
SCRIPT="$REPO_ROOT/scripts/history-rewrite/clean_history.sh"

@test "clean_history dry-run prints expected log and exits 0" {
  run bash "$SCRIPT" --dry-run --paths 'backend/codeql-db' --strip-size 1
  [ "$status" -eq 0 ]
  [[ "$output" == *"Dry-run complete"* ]]
}

@test "preview_removals shows commits for the path" {
  run bash "$REPO_ROOT/scripts/history-rewrite/preview_removals.sh" --paths 'backend/codeql-db' --strip-size 1
  [ "$status" -eq 0 ]
  [[ "$output" == *"Path: backend/codeql-db"* ]]
}

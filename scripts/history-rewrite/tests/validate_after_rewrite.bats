#!/usr/bin/env bats

setup() {
  # Create an isolated working repo
  TMPREPO=$(mktemp -d)
  cd "$TMPREPO"
  git init -q
  # Set local git identity for test commits
  git config user.email "test@example.com"
  git config user.name "Test Runner"
  echo 'initial' > README.md
  git add README.md && git commit -m 'init' -q
  # Make a minimal .venv pre-commit stub
  mkdir -p .venv/bin
  cat > .venv/bin/pre-commit <<'SH'
#!/usr/bin/env sh
exit 0
SH
  chmod +x .venv/bin/pre-commit
}

teardown() {
  rm -rf "$TMPREPO"
}

## Prefer deriving the script location from the test directory rather than hard-coding
## repository root paths such as /projects/Charon. This is more portable across
## environments and CI runners (e.g., forks where the repo path is different).
SCRIPT_DIR=$(cd "$BATS_TEST_DIRNAME/.." && pwd -P)
SCRIPT="$SCRIPT_DIR/validate_after_rewrite.sh"

@test "validate_after_rewrite fails when backup branch is missing" {
  run bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *"backup branch not provided"* ]]
}

@test "validate_after_rewrite passes with backup branch argument" {
  run bash "$SCRIPT" --backup-branch backup/main
  [ "$status" -eq 0 ]
}

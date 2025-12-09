#!/usr/bin/env bats

setup() {
  # Create an isolated working repo
  TMPREPO=$(mktemp -d)
  cd "$TMPREPO"
  git init -q
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

SCRIPT="/projects/Charon/scripts/history-rewrite/validate_after_rewrite.sh"

@test "validate_after_rewrite fails when backup branch is missing" {
  run bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *"backup branch not provided"* ]]
}

@test "validate_after_rewrite passes with backup branch argument" {
  run bash "$SCRIPT" --backup-branch backup/main
  [ "$status" -eq 0 ]
}

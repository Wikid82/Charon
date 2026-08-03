#!/usr/bin/env bats
#
# Covers scripts/local-patch-report.sh's three-tier baseline resolution
# (F.3 of docs/plans/current_spec.md's "Follow-up" section):
#   Tier 1 - explicit CHARON_PATCH_BASELINE override always wins.
#   Tier 2 - gh-derived real PR base, when gh is available and succeeds.
#   Tier 3 - static development-preferring heuristic fallback, when gh is
#            absent or errors/times out.
#
# The script under test computes ROOT_DIR from its own file location
# (BASH_SOURCE), not the caller's cwd, so each test copies the real script
# into an isolated fake repo (TMPROOT) laid out as scripts/local-patch-report.sh
# so ROOT_DIR resolves to TMPROOT. The real `go` toolchain and (for some
# tests) `gh` CLI are replaced with stub executables placed earlier in PATH,
# following the same technique used by
# scripts/history-rewrite/tests/*.bats (stub an external command via a fake
# executable directory prepended to PATH).

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT_UNDER_TEST="$REPO_ROOT/scripts/local-patch-report.sh"

  TMPROOT="$(mktemp -d)"
  mkdir -p "$TMPROOT/scripts" "$TMPROOT/backend" "$TMPROOT/frontend/coverage" "$TMPROOT/agent"
  cp "$SCRIPT_UNDER_TEST" "$TMPROOT/scripts/local-patch-report.sh"
  chmod +x "$TMPROOT/scripts/local-patch-report.sh"

  echo "mode: atomic" >"$TMPROOT/backend/coverage.txt"
  echo "TN:" >"$TMPROOT/frontend/coverage/lcov.info"
  echo "mode: atomic" >"$TMPROOT/agent/coverage.txt"

  git -C "$TMPROOT" init -q
  git -C "$TMPROOT" config user.email "test@example.com"
  git -C "$TMPROOT" config user.name "Test Runner"
  git -C "$TMPROOT" add -A
  git -C "$TMPROOT" commit -q -m "initial commit"

  COMMIT_SHA="$(git -C "$TMPROOT" rev-parse HEAD)"
  git -C "$TMPROOT" update-ref refs/remotes/origin/main "$COMMIT_SHA"
  git -C "$TMPROOT" update-ref refs/remotes/origin/development "$COMMIT_SHA"
  # A third ref, distinct from both origin/main and origin/development, so
  # tests can prove gh's exact answer was actually used (Tier 2) rather than
  # coincidentally matching either static heuristic candidate.
  git -C "$TMPROOT" update-ref refs/remotes/origin/feature-base "$COMMIT_SHA"

  # Minimal restricted PATH: only the real binaries the script actually
  # needs, plus a fake `go` that captures the --baseline value it was
  # invoked with instead of building/running the (nonexistent, under
  # TMPROOT) Go module. Tests that need to simulate gh add their own `gh`
  # stub into this same directory; tests that don't leave gh absent from
  # PATH entirely, which is what "gh not installed" means to `command -v`.
  STUBDIR="$(mktemp -d)"
  for bin in bash dirname git mkdir timeout; do
    ln -s "$(command -v "$bin")" "$STUBDIR/$bin"
  done

  cat >"$STUBDIR/go" <<'GOSTUB'
#!/usr/bin/env bash
baseline=""
json_out=""
md_out=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --baseline) baseline="$2"; shift 2 ;;
    --json-out) json_out="$2"; shift 2 ;;
    --md-out) md_out="$2"; shift 2 ;;
    *) shift ;;
  esac
done
if [[ -n "${CAPTURED_BASELINE_FILE:-}" ]]; then
  printf '%s' "$baseline" >"$CAPTURED_BASELINE_FILE"
fi
if [[ -n "$json_out" ]]; then
  mkdir -p "$(dirname "$json_out")"
  printf '{"mode":"strict"}' >"$json_out"
fi
if [[ -n "$md_out" ]]; then
  mkdir -p "$(dirname "$md_out")"
  printf '# report' >"$md_out"
fi
exit 0
GOSTUB
  chmod +x "$STUBDIR/go"

  CAPTURED_BASELINE_FILE="$TMPROOT/captured_baseline.txt"
}

teardown() {
  rm -rf "$TMPROOT" "$STUBDIR"
}

@test "gh available and PR open resolves baseline to gh's exact baseRefName" {
  # gh returns a base ref (origin/feature-base) that matches neither the old
  # hardcoded "always origin/main" default nor the new heuristic's
  # "origin/development" default, so a pass here can only mean gh's answer
  # was genuinely consulted and used (Tier 2), not a coincidental match with
  # either static fallback candidate.
  cat >"$STUBDIR/gh" <<'GHSTUB'
#!/usr/bin/env bash
if [[ "$1" == "pr" && "$2" == "view" ]]; then
  echo "feature-base"
  exit 0
fi
exit 1
GHSTUB
  chmod +x "$STUBDIR/gh"

  # `env` scopes PATH/CAPTURED_BASELINE_FILE to just this child process,
  # leaving the test's own shell PATH (needed by `cat`/`rm` below and in
  # teardown) untouched.
  run env PATH="$STUBDIR" CAPTURED_BASELINE_FILE="$CAPTURED_BASELINE_FILE" \
    bash "$TMPROOT/scripts/local-patch-report.sh"

  [ "$status" -eq 0 ]
  [ "$(cat "$CAPTURED_BASELINE_FILE")" = "origin/feature-base...HEAD" ]
}

@test "gh absent falls back to development-preferring heuristic" {
  # No gh stub created: STUBDIR is the entire PATH for this invocation, so
  # `command -v gh` genuinely fails, exercising the "gh not installed" path.
  run env PATH="$STUBDIR" CAPTURED_BASELINE_FILE="$CAPTURED_BASELINE_FILE" \
    bash "$TMPROOT/scripts/local-patch-report.sh"

  [ "$status" -eq 0 ]
  [ "$(cat "$CAPTURED_BASELINE_FILE")" = "origin/development...HEAD" ]
}

@test "gh present but erroring falls back to development-preferring heuristic" {
  cat >"$STUBDIR/gh" <<'GHSTUB'
#!/usr/bin/env bash
# Simulates an unauthenticated/erroring gh (e.g. "no pull requests found").
exit 1
GHSTUB
  chmod +x "$STUBDIR/gh"

  run env PATH="$STUBDIR" CAPTURED_BASELINE_FILE="$CAPTURED_BASELINE_FILE" \
    bash "$TMPROOT/scripts/local-patch-report.sh"

  [ "$status" -eq 0 ]
  [ "$(cat "$CAPTURED_BASELINE_FILE")" = "origin/development...HEAD" ]
}

@test "explicit CHARON_PATCH_BASELINE wins over gh and the heuristic fallback" {
  cat >"$STUBDIR/gh" <<'GHSTUB'
#!/usr/bin/env bash
if [[ "$1" == "pr" && "$2" == "view" ]]; then
  echo "development"
  exit 0
fi
exit 1
GHSTUB
  chmod +x "$STUBDIR/gh"

  run env PATH="$STUBDIR" CAPTURED_BASELINE_FILE="$CAPTURED_BASELINE_FILE" \
    CHARON_PATCH_BASELINE="origin/main...HEAD" \
    bash "$TMPROOT/scripts/local-patch-report.sh"

  [ "$status" -eq 0 ]
  [ "$(cat "$CAPTURED_BASELINE_FILE")" = "origin/main...HEAD" ]
}

#!/usr/bin/env bash
# Structural parity guard for the Semgrep CI workflow, modeled on
# check-codeql-parity.sh's approach (grep/structural assertions, not full
# YAML parsing). Unlike the CodeQL guard, this script does NOT compare two
# independent rule-config lists between local and CI — the Semgrep CI
# workflow delegates directly to scripts/pre-commit-hooks/semgrep-scan.sh
# for both its SARIF-producing pass and its hard-fail gate pass, so there is
# exactly one place in the repo that defines --config/--exclude/
# --exclude-rule values. See docs/plans/current_spec.md §2.7/§3.4 for the
# full rationale.
#
# This guard instead checks the invariants that remain worth checking even
# with zero config duplication:
#   1. Required files exist.
#   2. The additive SEMGREP_SARIF_OUTPUT hook is still present in
#      semgrep-scan.sh (a future refactor could drop it without realizing
#      CI depends on it).
#   3. semgrep.yml still delegates to the real script for both passes,
#      rather than a future edit reintroducing an inline `semgrep scan`
#      call (which would silently reintroduce config duplication).
#   4. The pinned container image reference has both a tag and a digest
#      (catches an accidental un-pin, e.g. a quick edit to `:latest`).
#   5. pull_request/push trigger branches match [main, nightly, development].
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/workflow-yaml-asserts.sh
source "${SCRIPT_DIR}/lib/workflow-yaml-asserts.sh"

SEMGREP_WORKFLOW=".github/workflows/semgrep.yml"
SEMGREP_SCRIPT="scripts/pre-commit-hooks/semgrep-scan.sh"

fail() {
  local message="$1"
  echo "::error title=Semgrep parity drift::${message}"
  exit 1
}

# --- Check 1: required files exist ---
[[ -f "$SEMGREP_WORKFLOW" ]] || fail "Missing workflow file: $SEMGREP_WORKFLOW"
[[ -f "$SEMGREP_SCRIPT" ]] || fail "Missing pre-commit script: $SEMGREP_SCRIPT"

command -v jq >/dev/null 2>&1 || fail "jq is required for semantic Semgrep parity checks"

# --- Check 2: additive SARIF hook still present in the script ---
grep -Fq 'SEMGREP_SARIF_OUTPUT' "$SEMGREP_SCRIPT" || fail "$SEMGREP_SCRIPT must retain the SEMGREP_SARIF_OUTPUT hook so CI can produce SARIF via the same script CI/local both use"

# --- Check 3: workflow delegates to the real script for both passes, ---
# --- rather than a reimplemented/inlined `semgrep scan ...` invocation ---
grep -Fq 'SEMGREP_SARIF_OUTPUT' "$SEMGREP_WORKFLOW" || fail "$SEMGREP_WORKFLOW must set SEMGREP_SARIF_OUTPUT for its SARIF-producing pass"
grep -Fq "$SEMGREP_SCRIPT" "$SEMGREP_WORKFLOW" || fail "$SEMGREP_WORKFLOW must delegate to $SEMGREP_SCRIPT instead of reimplementing the semgrep scan invocation inline"

# Count distinct delegating calls to the real script; there must be at least
# two (the SARIF pass and the hard-fail gate pass).
DELEGATE_CALL_COUNT="$(grep -Fc "bash ${SEMGREP_SCRIPT}" "$SEMGREP_WORKFLOW" || true)"
if [[ "$DELEGATE_CALL_COUNT" -lt 2 ]]; then
  fail "$SEMGREP_WORKFLOW must call 'bash $SEMGREP_SCRIPT' at least twice (SARIF pass + hard-fail gate pass); found $DELEGATE_CALL_COUNT"
fi

! grep -Eq '^\s*semgrep scan\b' "$SEMGREP_WORKFLOW" || fail "$SEMGREP_WORKFLOW must not contain an inline 'semgrep scan' invocation — delegate to $SEMGREP_SCRIPT"

# --- Check 4: pinned image reference has both tag and digest ---
grep -Eq 'semgrep/semgrep:[0-9]+\.[0-9]+\.[0-9]+@sha256:[0-9a-f]{64}' "$SEMGREP_WORKFLOW" || fail "$SEMGREP_WORKFLOW must pin the semgrep/semgrep image with both an exact tag and a sha256 digest"

# --- Check 5: trigger branches ---
ensure_event_branches_semantic \
  "$SEMGREP_WORKFLOW" \
  "pull_request" \
  "branches: [main, nightly, development]" \
  "main" "nightly" "development" || fail "semgrep.yml pull_request branches must be [main, nightly, development]"
ensure_event_branches_semantic \
  "$SEMGREP_WORKFLOW" \
  "push" \
  "branches: [main, nightly, development]" \
  "main" "nightly" "development" || fail "semgrep.yml push branches must be [main, nightly, development]"

echo "Semgrep parity check passed (script delegation + SARIF hook present + image pin format + trigger branches)"

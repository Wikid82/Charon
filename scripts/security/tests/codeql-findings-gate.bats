#!/usr/bin/env bats
#
# Fixture-driven functional tests for scripts/security/codeql-findings-gate.sh
# (docs/plans/current_spec.md §9.2, Commit 1). This script is new shared
# blocking logic, not yet wired into scripts/pre-commit-hooks/codeql-check-
# findings.sh or .github/workflows/codeql.yml (that's Commit 4) — these
# tests exercise it standalone against fixture SARIF/ignore-list files
# under scripts/security/testdata/, following the same colocation
# convention as scripts/history-rewrite/tests/*.bats.
#
# 7 cases per §9.2:
#   1. error-level, unsuppressed            -> exit non-zero
#   2. warning-level, unsuppressed          -> exit non-zero (regression
#      test for the exact bug this PR closes: the OLD gate only blocked
#      error-level findings, so this fixture would have PASSED under it)
#   3. native in-source suppression         -> exit 0, "SUPPRESSED (in-source)"
#   4. valid, non-expired ignore-list entry -> exit 0, reason/review date shown
#   5. expired ignore-list entry            -> exit non-zero, "EXPIRED SUPPRESSION"
#   6. rule+path match, line drifted        -> exit non-zero, "LIKELY-STALE ENTRY"
#   7. empty results array                  -> exit 0

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../../.." && pwd)"
  SCRIPT_UNDER_TEST="$REPO_ROOT/scripts/security/codeql-findings-gate.sh"
  TESTDATA_DIR="$REPO_ROOT/scripts/security/testdata"
  EMPTY_SUPPRESSIONS="$TESTDATA_DIR/empty-codeql-suppressions.yml"
}

@test "case 1: single error-level unsuppressed result exits non-zero" {
  CODEQL_SUPPRESSIONS_FILE="$EMPTY_SUPPRESSIONS" \
    run "$SCRIPT_UNDER_TEST" "$TESTDATA_DIR/case1-error-unsuppressed.sarif" go

  [ "$status" -ne 0 ]
  [[ "$output" == *"NEW FINDING"* ]]
  [[ "$output" == *"go/example-error-rule"* ]]
}

@test "case 2: single warning-level unsuppressed result exits non-zero (regression test)" {
  # Under the OLD policy (blocking_levels: [error] only), a bare
  # warning-level finding with no exception would have passed. This is the
  # exact fixture proving the new gate no longer lets that ride.
  CODEQL_SUPPRESSIONS_FILE="$EMPTY_SUPPRESSIONS" \
    run "$SCRIPT_UNDER_TEST" "$TESTDATA_DIR/case2-warning-unsuppressed.sarif" go

  [ "$status" -ne 0 ]
  [[ "$output" == *"NEW FINDING"* ]]
  [[ "$output" == *"warning"* ]]
  [[ "$output" == *"go/example-warning-rule"* ]]
}

@test "case 3: result with non-null suppressions is natively suppressed and exits 0" {
  CODEQL_SUPPRESSIONS_FILE="$EMPTY_SUPPRESSIONS" \
    run "$SCRIPT_UNDER_TEST" "$TESTDATA_DIR/case3-native-suppressed.sarif" go

  [ "$status" -eq 0 ]
  [[ "$output" == *"SUPPRESSED (in-source)"* ]]
  [[ "$output" == *"go/cookie-secure-not-set"* ]]
}

@test "case 4: valid non-expired codeql-suppressions.yml entry suppresses and exits 0" {
  CODEQL_SUPPRESSIONS_FILE="$TESTDATA_DIR/case4-codeql-suppressions.yml" \
    run "$SCRIPT_UNDER_TEST" "$TESTDATA_DIR/case4-ignorelist-valid.sarif" go

  [ "$status" -eq 0 ]
  [[ "$output" == *"SUPPRESSED (codeql-suppressions.yml"* ]]
  [[ "$output" == *"reason:"* ]]
  [[ "$output" == *"review by 2099-01-01"* ]]
}

@test "case 5: expired codeql-suppressions.yml entry does not suppress and exits non-zero" {
  CODEQL_SUPPRESSIONS_FILE="$TESTDATA_DIR/case5-codeql-suppressions.yml" \
    run "$SCRIPT_UNDER_TEST" "$TESTDATA_DIR/case5-ignorelist-expired.sarif" go

  [ "$status" -ne 0 ]
  [[ "$output" == *"EXPIRED SUPPRESSION"* ]]
  [[ "$output" == *"review_by 2020-02-01 has passed"* ]]
}

@test "case 6: rule+path match with drifted line exits non-zero as LIKELY-STALE ENTRY" {
  CODEQL_SUPPRESSIONS_FILE="$TESTDATA_DIR/case6-codeql-suppressions.yml" \
    run "$SCRIPT_UNDER_TEST" "$TESTDATA_DIR/case6-ignorelist-stale-line.sarif" go

  [ "$status" -ne 0 ]
  [[ "$output" == *"LIKELY-STALE ENTRY"* ]]
  # Distinguishable from case 1/2's generic "NEW FINDING" message.
  [[ "$output" != *"NEW FINDING"* ]]
}

@test "case 7: empty results array exits 0" {
  CODEQL_SUPPRESSIONS_FILE="$EMPTY_SUPPRESSIONS" \
    run "$SCRIPT_UNDER_TEST" "$TESTDATA_DIR/case7-empty-results.sarif" go

  [ "$status" -eq 0 ]
  [[ "$output" == *"Summary: 0 suppressed, 0 blocking, 0 total"* ]]
}

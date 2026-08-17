#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/workflow-yaml-asserts.sh
source "${SCRIPT_DIR}/lib/workflow-yaml-asserts.sh"

CODEQL_WORKFLOW=".github/workflows/codeql.yml"
TASKS_FILE=".vscode/tasks.json"
GO_PRECOMMIT_SCRIPT="scripts/pre-commit-hooks/codeql-go-scan.sh"
JS_PRECOMMIT_SCRIPT="scripts/pre-commit-hooks/codeql-js-scan.sh"
FINDINGS_CHECK_SCRIPT="scripts/pre-commit-hooks/codeql-check-findings.sh"
SHARED_GATE_SCRIPT="scripts/security/codeql-findings-gate.sh"

fail() {
  local message="$1"
  echo "::error title=CodeQL parity drift::${message}"
  exit 1
}

ensure_task_command() {
  local tasks_file="$1"
  local task_label="$2"
  local expected_command="$3"

  jq -e \
    --arg task_label "$task_label" \
    --arg expected_command "$expected_command" \
    '.tasks | type == "array" and any(.[]; .label == $task_label and .command == $expected_command)' \
    "$tasks_file" >/dev/null
}

[[ -f "$CODEQL_WORKFLOW" ]] || fail "Missing workflow file: $CODEQL_WORKFLOW"
[[ -f "$TASKS_FILE" ]] || fail "Missing tasks file: $TASKS_FILE"
[[ -f "$GO_PRECOMMIT_SCRIPT" ]] || fail "Missing pre-commit script: $GO_PRECOMMIT_SCRIPT"
[[ -f "$JS_PRECOMMIT_SCRIPT" ]] || fail "Missing pre-commit script: $JS_PRECOMMIT_SCRIPT"
[[ -f "$FINDINGS_CHECK_SCRIPT" ]] || fail "Missing pre-commit script: $FINDINGS_CHECK_SCRIPT"
[[ -f "$SHARED_GATE_SCRIPT" ]] || fail "Missing shared findings-gate script: $SHARED_GATE_SCRIPT"

command -v jq >/dev/null 2>&1 || fail "jq is required for semantic CodeQL parity checks"

ensure_event_branches_semantic \
  "$CODEQL_WORKFLOW" \
  "pull_request" \
  "branches: [main, nightly, development]" \
  "main" "nightly" "development" || fail "codeql.yml pull_request branches must be [main, nightly, development]"
ensure_event_branches_semantic \
  "$CODEQL_WORKFLOW" \
  "push" \
  "branches: [main, nightly, development]" \
  "main" "nightly" "development" || fail "codeql.yml push branches must be [main, nightly, development]"
grep -Fq 'security-and-quality' "$CODEQL_WORKFLOW" || fail "codeql.yml must include security-and-quality in init queries"
! grep -Fq 'security-experimental' "$CODEQL_WORKFLOW" || fail "codeql.yml must NOT include security-experimental (produces false positives in test files)"
ensure_task_command "$TASKS_FILE" "Security: CodeQL Go Scan (CI-Aligned) [~60s]" "bash scripts/pre-commit-hooks/codeql-go-scan.sh" || fail "Missing or mismatched CI-aligned Go CodeQL task (label+command)"
ensure_task_command "$TASKS_FILE" "Security: CodeQL JS Scan (CI-Aligned) [~90s]" "bash scripts/pre-commit-hooks/codeql-js-scan.sh" || fail "Missing or mismatched CI-aligned JS CodeQL task (label+command)"
! grep -Fq 'go-security-extended.qls' "$TASKS_FILE" || fail "tasks.json contains deprecated go-security-extended suite; use CI-aligned scripts"
! grep -Fq 'javascript-security-extended.qls' "$TASKS_FILE" || fail "tasks.json contains deprecated javascript-security-extended suite; use CI-aligned scripts"
grep -Fq 'codeql/go-queries:codeql-suites/go-security-and-quality.qls' "$GO_PRECOMMIT_SCRIPT" || fail "Go pre-commit script must use go-security-and-quality suite"
! grep -Fq 'codeql/go-queries:codeql-suites/go-security-experimental.qls' "$GO_PRECOMMIT_SCRIPT" || fail "Go pre-commit script must NOT use go-security-experimental suite"
grep -Fq 'codeql/javascript-queries:codeql-suites/javascript-security-and-quality.qls' "$JS_PRECOMMIT_SCRIPT" || fail "JS pre-commit script must use javascript-security-and-quality suite"
! grep -Fq 'codeql/javascript-queries:codeql-suites/javascript-security-experimental.qls' "$JS_PRECOMMIT_SCRIPT" || fail "JS pre-commit script must NOT use javascript-security-experimental suite"

# Findings-gate blocking logic must live in exactly one place
# (scripts/security/codeql-findings-gate.sh), referenced by both the local
# pre-commit script and the CI workflow — not hand-duplicated inline jq in
# either. This is the structural guard against the drift class described in
# docs/plans/current_spec.md §4.1/§5.5 (the cookie-suppression finding rode
# through PR #1216 unnoticed because local and CI each had their own
# independently-maintained blocking-logic copy that silently agreed on the
# wrong answer).
grep -Fq "$SHARED_GATE_SCRIPT" "$FINDINGS_CHECK_SCRIPT" || fail "$FINDINGS_CHECK_SCRIPT must call the shared gate script ($SHARED_GATE_SCRIPT) instead of reimplementing blocking logic inline"
grep -Fq "$SHARED_GATE_SCRIPT" "$CODEQL_WORKFLOW" || fail "$CODEQL_WORKFLOW must call the shared gate script ($SHARED_GATE_SCRIPT) instead of reimplementing blocking logic inline"

echo "CodeQL parity check passed (workflow triggers + suite pinning [security-and-quality] + local/CI alignment + shared findings-gate script)"

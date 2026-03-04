#!/usr/bin/env bash
set -euo pipefail

CODEQL_WORKFLOW=".github/workflows/codeql.yml"
TASKS_FILE=".vscode/tasks.json"
GO_PRECOMMIT_SCRIPT="scripts/pre-commit-hooks/codeql-go-scan.sh"
JS_PRECOMMIT_SCRIPT="scripts/pre-commit-hooks/codeql-js-scan.sh"

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

ensure_event_branches() {
  local workflow_file="$1"
  local event_name="$2"
  local expected_line="$3"

  awk -v event_name="$event_name" -v expected_line="$expected_line" '
    /^on:/ {
      in_on = 1
      next
    }

    in_on && $1 == event_name ":" {
      in_event = 1
      next
    }

    in_on && in_event && $1 == "branches:" {
      line = $0
      gsub(/^ +/, "", line)
      if (line == expected_line) {
        found = 1
      }
      in_event = 0
      next
    }

    in_on && in_event && $1 ~ /^[a-z_]+:$/ {
      in_event = 0
    }

    END {
      exit found ? 0 : 1
    }
  ' "$workflow_file"
}

ensure_event_branches_with_yq() {
  local workflow_file="$1"
  local event_name="$2"
  shift 2
  local expected_branches=("$@")

  local expected_json
  local actual_json

  expected_json="$(printf '%s\n' "${expected_branches[@]}" | jq -R . | jq -s .)"

  if actual_json="$(yq eval -o=json ".on.${event_name}.branches // []" "$workflow_file" 2>/dev/null)"; then
    :
  elif actual_json="$(yq -o=json ".on.${event_name}.branches // []" "$workflow_file" 2>/dev/null)"; then
    :
  else
    return 1
  fi

  jq -e \
    --argjson expected "$expected_json" \
    'if type != "array" then false else ((map(tostring) | unique | sort) == ($expected | map(tostring) | unique | sort)) end' \
    <<<"$actual_json" >/dev/null
}

ensure_event_branches_semantic() {
  local workflow_file="$1"
  local event_name="$2"
  local fallback_line="$3"
  shift 3
  local expected_branches=("$@")

  if command -v yq >/dev/null 2>&1; then
    if ensure_event_branches_with_yq "$workflow_file" "$event_name" "${expected_branches[@]}"; then
      return 0
    fi
  fi

  ensure_event_branches "$workflow_file" "$event_name" "$fallback_line"
}

[[ -f "$CODEQL_WORKFLOW" ]] || fail "Missing workflow file: $CODEQL_WORKFLOW"
[[ -f "$TASKS_FILE" ]] || fail "Missing tasks file: $TASKS_FILE"
[[ -f "$GO_PRECOMMIT_SCRIPT" ]] || fail "Missing pre-commit script: $GO_PRECOMMIT_SCRIPT"
[[ -f "$JS_PRECOMMIT_SCRIPT" ]] || fail "Missing pre-commit script: $JS_PRECOMMIT_SCRIPT"

command -v jq >/dev/null 2>&1 || fail "jq is required for semantic CodeQL parity checks"

ensure_event_branches_semantic \
  "$CODEQL_WORKFLOW" \
  "pull_request" \
  "branches: [main, nightly, development]" \
  "main" "nightly" "development" || fail "codeql.yml pull_request branches must be [main, nightly, development]"
ensure_event_branches_semantic \
  "$CODEQL_WORKFLOW" \
  "push" \
  "branches: [main]" \
  "main" || fail "codeql.yml push branches must be [main]"
grep -Fq 'queries: security-and-quality' "$CODEQL_WORKFLOW" || fail "codeql.yml must pin init queries to security-and-quality"
ensure_task_command "$TASKS_FILE" "Security: CodeQL Go Scan (CI-Aligned) [~60s]" "bash scripts/pre-commit-hooks/codeql-go-scan.sh" || fail "Missing or mismatched CI-aligned Go CodeQL task (label+command)"
ensure_task_command "$TASKS_FILE" "Security: CodeQL JS Scan (CI-Aligned) [~90s]" "bash scripts/pre-commit-hooks/codeql-js-scan.sh" || fail "Missing or mismatched CI-aligned JS CodeQL task (label+command)"
! grep -Fq 'go-security-extended.qls' "$TASKS_FILE" || fail "tasks.json contains deprecated go-security-extended suite; use CI-aligned scripts"
! grep -Fq 'javascript-security-extended.qls' "$TASKS_FILE" || fail "tasks.json contains deprecated javascript-security-extended suite; use CI-aligned scripts"
grep -Fq 'codeql/go-queries:codeql-suites/go-security-and-quality.qls' "$GO_PRECOMMIT_SCRIPT" || fail "Go pre-commit script must use go-security-and-quality suite"
grep -Fq 'codeql/javascript-queries:codeql-suites/javascript-security-and-quality.qls' "$JS_PRECOMMIT_SCRIPT" || fail "JS pre-commit script must use javascript-security-and-quality suite"

echo "CodeQL parity check passed (workflow triggers + suite pinning + local/pre-commit suite alignment)"

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

[[ -f "$CODEQL_WORKFLOW" ]] || fail "Missing workflow file: $CODEQL_WORKFLOW"
[[ -f "$TASKS_FILE" ]] || fail "Missing tasks file: $TASKS_FILE"
[[ -f "$GO_PRECOMMIT_SCRIPT" ]] || fail "Missing pre-commit script: $GO_PRECOMMIT_SCRIPT"
[[ -f "$JS_PRECOMMIT_SCRIPT" ]] || fail "Missing pre-commit script: $JS_PRECOMMIT_SCRIPT"

ensure_event_branches "$CODEQL_WORKFLOW" "pull_request" "branches: [main, nightly, development]" || fail "codeql.yml pull_request branches must be [main, nightly, development]"
ensure_event_branches "$CODEQL_WORKFLOW" "push" "branches: [main, nightly, development, 'feature/**', 'fix/**']" || fail "codeql.yml push branches must be [main, nightly, development, 'feature/**', 'fix/**']"
grep -Fq 'queries: security-and-quality' "$CODEQL_WORKFLOW" || fail "codeql.yml must pin init queries to security-and-quality"
grep -Fq '"label": "Security: CodeQL Go Scan (CI-Aligned) [~60s]"' "$TASKS_FILE" || fail "Missing CI-aligned Go CodeQL task label"
grep -Fq '"command": "bash scripts/pre-commit-hooks/codeql-go-scan.sh"' "$TASKS_FILE" || fail "CI-aligned Go CodeQL task must invoke scripts/pre-commit-hooks/codeql-go-scan.sh"
grep -Fq '"label": "Security: CodeQL JS Scan (CI-Aligned) [~90s]"' "$TASKS_FILE" || fail "Missing CI-aligned JS CodeQL task label"
grep -Fq '"command": "bash scripts/pre-commit-hooks/codeql-js-scan.sh"' "$TASKS_FILE" || fail "CI-aligned JS CodeQL task must invoke scripts/pre-commit-hooks/codeql-js-scan.sh"
grep -Fq 'codeql/go-queries:codeql-suites/go-security-and-quality.qls' "$GO_PRECOMMIT_SCRIPT" || fail "Go pre-commit script must use go-security-and-quality suite"
grep -Fq 'codeql/javascript-queries:codeql-suites/javascript-security-and-quality.qls' "$JS_PRECOMMIT_SCRIPT" || fail "JS pre-commit script must use javascript-security-and-quality suite"

echo "CodeQL parity check passed (workflow triggers + suite pinning + local/pre-commit suite alignment)"

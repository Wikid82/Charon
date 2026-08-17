#!/usr/bin/env bash
# Shared assertion helpers for CI parity-guard scripts (e.g.
# check-codeql-parity.sh, check-semgrep-parity.sh) that need to verify a
# GitHub Actions workflow's `on.<event>.branches` list matches an expected
# set of branches. Extracted per CLAUDE.md's "consolidate after second
# occurrence" DRY guideline once a second parity script needed the same
# branch-list assertion logic.
#
# Intended usage: source this file, then call ensure_event_branches_semantic.
# This file only defines functions — it has no side effects when sourced and
# does not set -euo pipefail itself (the sourcing script controls that).

# ensure_event_branches: AWK-based fallback branch check. Compares the
# literal `branches:` line under `on.<event_name>:` against expected_line,
# e.g. "branches: [main, nightly, development]". Used when `yq` is
# unavailable or fails.
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

# ensure_event_branches_with_yq: semantic (order-independent) branch check
# using yq + jq to parse the workflow YAML directly, rather than matching a
# literal formatted line.
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

# ensure_event_branches_semantic: prefers the semantic yq-based check when
# `yq` is installed, falling back to the literal-line AWK check otherwise.
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

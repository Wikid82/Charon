#!/bin/bash
# ============================================================================
# codeql-findings-gate.sh
#
# Shared CodeQL SARIF blocking-logic gate. Single source of truth consumed
# by both scripts/pre-commit-hooks/codeql-check-findings.sh (local) and
# .github/workflows/codeql.yml (CI) so the two never drift again (see
# docs/plans/current_spec.md §4.1/§5.4 for the "two independent
# hand-maintained implementations silently drifted" background).
#
# Policy: EVERY CodeQL result blocks by default, regardless of severity
# level, unless it is suppressed:
#   - natively, via a correctly-placed in-source `codeql[rule-id]` comment
#     (SARIF result.suppressions is non-null/non-empty), or
#   - via a matching, non-expired entry in
#     .github/codeql/codeql-suppressions.yml.
#
# Usage: codeql-findings-gate.sh <sarif-file> <language-label>
#
# Exit status: 0 if no result is blocking, non-zero if any result is
# blocking (or on usage/tooling errors).
#
# Env overrides (used by scripts/security/tests/codeql-findings-gate.bats
# to point at fixture ignore-lists instead of the real repo file):
#   CODEQL_SUPPRESSIONS_FILE - path to the ignore-list YAML file.
#                              Defaults to
#                              <repo-root>/.github/codeql/codeql-suppressions.yml
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

SARIF_FILE="${1:-}"
LANG_LABEL="${2:-}"
SUPPRESSIONS_FILE="${CODEQL_SUPPRESSIONS_FILE:-$ROOT_DIR/.github/codeql/codeql-suppressions.yml}"

usage() {
    echo "Usage: $(basename "${BASH_SOURCE[0]}") <sarif-file> <language-label>" >&2
}

if [[ -z "$SARIF_FILE" || -z "$LANG_LABEL" ]]; then
    usage
    exit 2
fi

if [[ ! -f "$SARIF_FILE" ]]; then
    echo "ERROR: SARIF file not found: $SARIF_FILE" >&2
    exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "ERROR: jq is required for CodeQL findings-gate evaluation" >&2
    exit 2
fi

if ! command -v yq >/dev/null 2>&1; then
    echo "ERROR: yq is required for CodeQL findings-gate evaluation" >&2
    exit 2
fi

# Convert the ignore-list YAML to JSON. Tries the mikefarah-yq syntax first
# (`yq eval -o=json`), then falls back to the kislyuk-yq syntax (`yq -o=json`
# / bare `yq '.'`), mirroring the dual-syntax handling already used by
# scripts/ci/check-codeql-parity.sh so this works with either yq
# distribution a dev/CI environment might have installed.
convert_yaml_to_json() {
    local yaml_file="$1"
    local out
    if out="$(yq eval -o=json '.' "$yaml_file" 2>/dev/null)"; then
        printf '%s' "$out"
        return 0
    fi
    if out="$(yq -o=json '.' "$yaml_file" 2>/dev/null)"; then
        printf '%s' "$out"
        return 0
    fi
    if out="$(yq '.' "$yaml_file" 2>/dev/null)"; then
        printf '%s' "$out"
        return 0
    fi
    return 1
}

if [[ -f "$SUPPRESSIONS_FILE" ]]; then
    suppressions_json="$(convert_yaml_to_json "$SUPPRESSIONS_FILE")" || {
        echo "ERROR: failed to parse ignore-list as YAML: $SUPPRESSIONS_FILE" >&2
        exit 2
    }
else
    suppressions_json='{"suppressions": []}'
fi

ignore_entries="$(jq -c '.suppressions // []' <<<"$suppressions_json")"

# Effective level fallback chain (relocated unchanged from
# scripts/pre-commit-hooks/codeql-check-findings.sh's prior inline logic):
#   result.level
#   -> rules[ruleIndex].defaultConfiguration.level
#   -> lookup by ruleId in the rules array
#   -> ""
# Extracted alongside ruleId/path/line/native-suppression flag for each
# result, one compact JSON object per line, for the bash loop below.
results_json="$(jq -c '
    [
        .runs[]? as $run
        | ($run.tool.driver.rules // []) as $rules
        | ($run.results // [])[]
        | . as $result
        | ((
            $result.level
            // (if (($result.ruleIndex | type) == "number") then ($rules[$result.ruleIndex].defaultConfiguration.level // empty) else empty end)
            // ([
                $rules[]?
                | select((.id // "") == ($result.ruleId // ""))
                | (.defaultConfiguration.level // empty)
            ][0] // empty)
            // ""
          ) | ascii_downcase) as $effectiveLevel
        | {
            ruleId: ($result.ruleId // "<unknown-rule>"),
            path: ($result.locations[0].physicalLocation.artifactLocation.uri // "<unknown-file>"),
            line: ($result.locations[0].physicalLocation.region.startLine // 0),
            level: $effectiveLevel,
            nativelySuppressed: (($result.suppressions // []) | length > 0)
        }
    ]
' "$SARIF_FILE")"

result_count="$(jq 'length' <<<"$results_json")"

echo "Checking $LANG_LABEL CodeQL findings in $SARIF_FILE ($result_count result(s))..."

if [[ "$result_count" -eq 0 ]]; then
    echo "No findings."
    echo "Summary: 0 suppressed, 0 blocking, 0 total"
    exit 0
fi

today="$(date -u +%F)"
blocking_count=0
suppressed_count=0

while IFS= read -r result; do
    rule_id="$(jq -r '.ruleId' <<<"$result")"
    path="$(jq -r '.path' <<<"$result")"
    line="$(jq -r '.line' <<<"$result")"
    level="$(jq -r '.level' <<<"$result")"
    natively_suppressed="$(jq -r '.nativelySuppressed' <<<"$result")"

    if [[ "$natively_suppressed" == "true" ]]; then
        echo "SUPPRESSED (in-source): $rule_id $path:$line"
        suppressed_count=$((suppressed_count + 1))
        continue
    fi

    # All ignore-list entries sharing this exact rule_id + path, regardless
    # of line. Per docs/plans/current_spec.md §9.2's reviewer clarification:
    # when multiple entries exist for the same rule+path at different
    # lines, "partial match" (LIKELY-STALE) means none of them cover this
    # line — not just checking the first entry found.
    candidates="$(jq -c --arg rule "$rule_id" --arg path "$path" '
        [.[] | select(.rule_id == $rule and .path == $path)]
    ' <<<"$ignore_entries")"
    candidate_count="$(jq 'length' <<<"$candidates")"

    if [[ "$candidate_count" -eq 0 ]]; then
        echo "NEW FINDING (no exception on file): $level $rule_id $path:$line"
        blocking_count=$((blocking_count + 1))
        continue
    fi

    match="$(jq -c --argjson line "$line" '
        [.[] | select(
            (has("line") and .line == $line)
            or (has("line_range") and $line >= .line_range.start and $line <= .line_range.end)
        )] | first // empty
    ' <<<"$candidates")"

    if [[ -z "$match" || "$match" == "null" ]]; then
        echo "LIKELY-STALE ENTRY (line moved? check codeql-suppressions.yml): $rule_id $path:$line"
        blocking_count=$((blocking_count + 1))
        continue
    fi

    review_by="$(jq -r '.review_by' <<<"$match")"
    reason="$(jq -r '.reason' <<<"$match" | tr '\n' ' ' | sed -E 's/[[:space:]]+/ /g; s/^ //; s/ $//')"

    if [[ "$review_by" < "$today" ]]; then
        echo "EXPIRED SUPPRESSION (review_by $review_by has passed — renew or fix): $rule_id $path:$line"
        blocking_count=$((blocking_count + 1))
    else
        echo "SUPPRESSED (codeql-suppressions.yml, reason: \"$reason\", review by $review_by): $rule_id $path:$line"
        suppressed_count=$((suppressed_count + 1))
    fi
done < <(jq -c '.[]' <<<"$results_json")

echo "Summary: $suppressed_count suppressed, $blocking_count blocking, $result_count total"

if [[ "$blocking_count" -gt 0 ]]; then
    exit 1
fi

exit 0

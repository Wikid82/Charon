#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPORTS_DIR="$ROOT_DIR/docs/reports"
BASELINE_FILE="${PR718_BASELINE_FILE:-$REPORTS_DIR/pr718_open_alerts_baseline.json}"
GO_SARIF="${PR718_GO_SARIF:-$ROOT_DIR/codeql-results-go.sarif}"
JS_SARIF="${PR718_JS_SARIF:-$ROOT_DIR/codeql-results-js.sarif}"

if ! command -v jq >/dev/null 2>&1; then
    echo "Error: jq is required to run freshness gate." >&2
    exit 1
fi

if [[ ! -f "$GO_SARIF" ]]; then
    echo "Error: missing Go SARIF at $GO_SARIF" >&2
    exit 1
fi

if [[ ! -f "$JS_SARIF" ]]; then
    echo "Error: missing JS SARIF at $JS_SARIF" >&2
    exit 1
fi

mkdir -p "$REPORTS_DIR"

TIMESTAMP="$(date -u +"%Y%m%dT%H%M%SZ")"
FRESH_JSON="$REPORTS_DIR/pr718_open_alerts_freshness_${TIMESTAMP}.json"
DELTA_MD="$REPORTS_DIR/pr718_open_alerts_freshness_${TIMESTAMP}.md"

fresh_findings_json() {
    local input_file="$1"
    local source_name="$2"

    jq --arg source "$source_name" '
      [(.runs // [])[]?
       | (.results // [])[]?
       | {
           rule_id: (.ruleId // "unknown"),
           path: (.locations[0].physicalLocation.artifactLocation.uri // ""),
           start_line: (.locations[0].physicalLocation.region.startLine // 0),
           source: $source
         }
      ]
    ' "$input_file"
}

GO_FINDINGS="$(fresh_findings_json "$GO_SARIF" "go")"
JS_FINDINGS="$(fresh_findings_json "$JS_SARIF" "js")"

FRESH_FINDINGS="$(jq -n --argjson go "$GO_FINDINGS" --argjson js "$JS_FINDINGS" '$go + $js')"

BASELINE_STATUS="missing"
BASELINE_NORMALIZED='[]'
if [[ -f "$BASELINE_FILE" ]]; then
    BASELINE_STATUS="present"
    BASELINE_NORMALIZED="$(jq '
      if type == "array" then
        [ .[]
          | {
              alert_number: (.alert_number // .number // null),
              rule_id: (.rule.id // .rule_id // .ruleId // "unknown"),
              path: (.location.path // .path // ""),
              start_line: (.location.start_line // .start_line // .line // 0)
            }
        ]
      elif type == "object" and has("alerts") then
        [ .alerts[]?
          | {
              alert_number: (.alert_number // .number // null),
              rule_id: (.rule.id // .rule_id // .ruleId // "unknown"),
              path: (.location.path // .path // ""),
              start_line: (.location.start_line // .start_line // .line // 0)
            }
        ]
      else
        []
      end
    ' "$BASELINE_FILE")"
fi

DRIFT_STATUS="baseline_missing"
ADDED='[]'
REMOVED='[]'
if [[ "$BASELINE_STATUS" == "present" ]]; then
    ADDED="$(jq -n --argjson fresh "$FRESH_FINDINGS" --argjson base "$BASELINE_NORMALIZED" '
      [ $fresh[]
        | select(
            ([.rule_id, .path, .start_line]
              | @json
            ) as $k
            | ($base
                | map([.rule_id, .path, .start_line] | @json)
                | index($k)
              )
              == null
          )
      ]
    ')"

    REMOVED="$(jq -n --argjson fresh "$FRESH_FINDINGS" --argjson base "$BASELINE_NORMALIZED" '
      [ $base[]
        | select(
            ([.rule_id, .path, .start_line]
              | @json
            ) as $k
            | ($fresh
                | map([.rule_id, .path, .start_line] | @json)
                | index($k)
              )
              == null
          )
      ]
    ')"

    added_count="$(jq 'length' <<<"$ADDED")"
    removed_count="$(jq 'length' <<<"$REMOVED")"

    if [[ "$added_count" == "0" && "$removed_count" == "0" ]]; then
        DRIFT_STATUS="no_drift"
    else
        DRIFT_STATUS="drift_detected"
    fi
fi

jq -n \
  --arg generated_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
  --arg baseline_file "$(basename "$BASELINE_FILE")" \
  --arg baseline_status "$BASELINE_STATUS" \
  --arg drift_status "$DRIFT_STATUS" \
  --arg go_sarif "$(basename "$GO_SARIF")" \
  --arg js_sarif "$(basename "$JS_SARIF")" \
  --argjson findings "$FRESH_FINDINGS" \
  --argjson baseline_alerts "$BASELINE_NORMALIZED" \
  --argjson added "$ADDED" \
  --argjson removed "$REMOVED" \
  '{
    generated_at: $generated_at,
    baseline_file: $baseline_file,
    baseline_status: $baseline_status,
    drift_status: $drift_status,
    sources: {
      go_sarif: $go_sarif,
      js_sarif: $js_sarif
    },
    counts: {
      fresh_total: ($findings | length),
      baseline_total: ($baseline_alerts | length),
      added: ($added | length),
      removed: ($removed | length)
    },
    findings: $findings,
    delta: {
      added: $added,
      removed: $removed
    }
  }' >"$FRESH_JSON"

fresh_total="$(jq '.counts.fresh_total' "$FRESH_JSON")"
baseline_total="$(jq '.counts.baseline_total' "$FRESH_JSON")"
added_total="$(jq '.counts.added' "$FRESH_JSON")"
removed_total="$(jq '.counts.removed' "$FRESH_JSON")"

cat >"$DELTA_MD" <<EOF
# PR718 Freshness Gate Delta Summary

- Generated: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- Baseline status: \`${BASELINE_STATUS}\`
- Drift status: \`${DRIFT_STATUS}\`
- Fresh findings total: ${fresh_total}
- Baseline findings total: ${baseline_total}
- Added findings: ${added_total}
- Removed findings: ${removed_total}
- Freshness JSON artifact: \`$(basename "$FRESH_JSON")\`
EOF

echo "Freshness artifact generated: $FRESH_JSON"
echo "Delta summary generated: $DELTA_MD"

if [[ "$DRIFT_STATUS" == "drift_detected" ]]; then
    echo "Error: drift detected against baseline." >&2
    exit 2
fi

if [[ "$BASELINE_STATUS" == "missing" ]]; then
    echo "Warning: baseline file missing at $BASELINE_FILE; freshness artifact generated with baseline_missing status." >&2
    exit 3
fi

exit 0

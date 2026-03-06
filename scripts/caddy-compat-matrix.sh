#!/usr/bin/env bash

set -euo pipefail

readonly DEFAULT_CANDIDATE_VERSION="2.11.2"
readonly DEFAULT_PATCH_SCENARIOS="A,B,C"
readonly DEFAULT_PLATFORMS="linux/amd64,linux/arm64"
readonly DEFAULT_PLUGIN_SET="caddy-security,coraza-caddy,caddy-crowdsec-bouncer,caddy-geoip2,caddy-ratelimit"
readonly DEFAULT_SMOKE_SET="boot_caddy,plugin_modules,config_validate,admin_api_health"

OUTPUT_DIR="test-results/caddy-compat"
DOCS_REPORT="docs/reports/caddy-compatibility-matrix.md"
CANDIDATE_VERSION="$DEFAULT_CANDIDATE_VERSION"
PATCH_SCENARIOS="$DEFAULT_PATCH_SCENARIOS"
PLATFORMS="$DEFAULT_PLATFORMS"
PLUGIN_SET="$DEFAULT_PLUGIN_SET"
SMOKE_SET="$DEFAULT_SMOKE_SET"
BASE_IMAGE_TAG="charon"
KEEP_IMAGES="0"

REQUIRED_MODULES=(
  "http.handlers.auth_portal"
  "http.handlers.waf"
  "http.handlers.crowdsec"
  "http.handlers.geoip2"
  "http.handlers.rate_limit"
)

usage() {
  cat <<'EOF'
Usage: scripts/caddy-compat-matrix.sh [options]

Options:
  --output-dir <path>         Output directory (default: test-results/caddy-compat)
  --docs-report <path>        Markdown report path (default: docs/reports/caddy-compatibility-matrix.md)
  --candidate-version <ver>   Candidate Caddy version (default: 2.11.2)
  --patch-scenarios <csv>     Patch scenarios CSV (default: A,B,C)
  --platforms <csv>           Platforms CSV (default: linux/amd64,linux/arm64)
  --plugin-set <csv>          Plugin set descriptor for report metadata
  --smoke-set <csv>           Smoke set descriptor for report metadata
  --base-image-tag <name>     Base image tag prefix (default: charon)
  --keep-images               Keep generated local images
  -h, --help                  Show this help

Deterministic pass/fail:
  Promotion gate PASS only if Scenario A passes on linux/amd64 and linux/arm64.
  Scenario B/C are evidence-only and do not fail the promotion gate.
EOF
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "ERROR: Required command not found: $cmd" >&2
    exit 1
  fi
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --output-dir)
        OUTPUT_DIR="$2"
        shift 2
        ;;
      --docs-report)
        DOCS_REPORT="$2"
        shift 2
        ;;
      --candidate-version)
        CANDIDATE_VERSION="$2"
        shift 2
        ;;
      --patch-scenarios)
        PATCH_SCENARIOS="$2"
        shift 2
        ;;
      --platforms)
        PLATFORMS="$2"
        shift 2
        ;;
      --plugin-set)
        PLUGIN_SET="$2"
        shift 2
        ;;
      --smoke-set)
        SMOKE_SET="$2"
        shift 2
        ;;
      --base-image-tag)
        BASE_IMAGE_TAG="$2"
        shift 2
        ;;
      --keep-images)
        KEEP_IMAGES="1"
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        echo "Unknown option: $1" >&2
        usage
        exit 1
        ;;
    esac
  done
}

prepare_dirs() {
  mkdir -p "$OUTPUT_DIR"
  mkdir -p "$(dirname "$DOCS_REPORT")"
}

write_reports_header() {
  local metadata_file="$OUTPUT_DIR/metadata.env"
  local summary_csv="$OUTPUT_DIR/matrix-summary.csv"

  cat > "$metadata_file" <<EOF
generated_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
candidate_version=${CANDIDATE_VERSION}
patch_scenarios=${PATCH_SCENARIOS}
platforms=${PLATFORMS}
plugin_set=${PLUGIN_SET}
smoke_set=${SMOKE_SET}
required_modules=${REQUIRED_MODULES[*]}
EOF

  echo "scenario,platform,image_tag,checked_plugin_modules,boot_caddy,plugin_modules,config_validate,admin_api_health,module_inventory,status" > "$summary_csv"
}

contains_value() {
  local needle="$1"
  shift
  local value
  for value in "$@"; do
    if [[ "$value" == "$needle" ]]; then
      return 0
    fi
  done
  return 1
}

enforce_required_gate_dimensions() {
  local -n scenario_ref=$1
  local -n platform_ref=$2

  if ! contains_value "A" "${scenario_ref[@]}"; then
    echo "[compat] ERROR: Scenario A is required for PR-1 promotion gate" >&2
    return 1
  fi

  if ! contains_value "linux/amd64" "${platform_ref[@]}"; then
    echo "[compat] ERROR: linux/amd64 is required for PR-1 promotion gate" >&2
    return 1
  fi

  if ! contains_value "linux/arm64" "${platform_ref[@]}"; then
    echo "[compat] ERROR: linux/arm64 is required for PR-1 promotion gate" >&2
    return 1
  fi
}

validate_matrix_completeness() {
  local summary_csv="$1"
  local -n scenario_ref=$2
  local -n platform_ref=$3

  local expected_rows
  expected_rows=$(( ${#scenario_ref[@]} * ${#platform_ref[@]} ))

  local actual_rows
  actual_rows="$(tail -n +2 "$summary_csv" | sed '/^\s*$/d' | wc -l | tr -d '[:space:]')"

  if [[ "$actual_rows" != "$expected_rows" ]]; then
    echo "[compat] ERROR: matrix completeness failed (expected ${expected_rows} rows, found ${actual_rows})" >&2
    return 1
  fi

  local scenario
  local platform
  for scenario in "${scenario_ref[@]}"; do
    for platform in "${platform_ref[@]}"; do
      if ! grep -q "^${scenario},${platform}," "$summary_csv"; then
        echo "[compat] ERROR: missing matrix cell scenario=${scenario} platform=${platform}" >&2
        return 1
      fi
    done
  done
}

evaluate_promotion_gate() {
  local summary_csv="$1"

  local scenario_a_failures
  scenario_a_failures="$(tail -n +2 "$summary_csv" | awk -F',' '$1=="A" && $10=="FAIL" {count++} END {print count+0}')"
  local evidence_failures
  evidence_failures="$(tail -n +2 "$summary_csv" | awk -F',' '$1!="A" && $10=="FAIL" {count++} END {print count+0}')"

  if [[ "$evidence_failures" -gt 0 ]]; then
    echo "[compat] Evidence-only failures (Scenario B/C): ${evidence_failures}"
  fi

  if [[ "$scenario_a_failures" -gt 0 ]]; then
    echo "[compat] Promotion gate result: FAIL (Scenario A failures: ${scenario_a_failures})"
    return 1
  fi

  echo "[compat] Promotion gate result: PASS (Scenario A on both required architectures)"
}

build_image_for_cell() {
  local scenario="$1"
  local platform="$2"
  local image_tag="$3"

  docker buildx build \
    --platform "$platform" \
    --load \
    --pull \
    --build-arg CADDY_USE_CANDIDATE=1 \
    --build-arg CADDY_CANDIDATE_VERSION="$CANDIDATE_VERSION" \
    --build-arg CADDY_PATCH_SCENARIO="$scenario" \
    -t "$image_tag" \
    . >/dev/null
}

smoke_boot_caddy() {
  local image_tag="$1"
  docker run --rm --pull=never --entrypoint caddy "$image_tag" version >/dev/null
}

smoke_plugin_modules() {
  local image_tag="$1"
  local output_file="$2"
  docker run --rm --pull=never --entrypoint caddy "$image_tag" list-modules > "$output_file"

  local module
  for module in "${REQUIRED_MODULES[@]}"; do
    grep -q "^${module}$" "$output_file"
  done
}

smoke_config_validate() {
  local image_tag="$1"
  docker run --rm --pull=never --entrypoint sh "$image_tag" -lc '
    cat > /tmp/compat-config.json <<"JSON"
{
  "admin": {"listen": ":2019"},
  "apps": {
    "http": {
      "servers": {
        "compat": {
          "listen": [":2080"],
          "routes": [
            {
              "handle": [
                {
                  "handler": "static_response",
                  "body": "compat-ok",
                  "status_code": 200
                }
              ]
            }
          ]
        }
      }
    }
  }
}
JSON
    caddy validate --config /tmp/compat-config.json >/dev/null
  '
}

smoke_admin_api_health() {
  local image_tag="$1"
  local admin_port="$2"
  local run_id="compat-${admin_port}"

  docker run -d --name "$run_id" --pull=never --entrypoint sh -p "${admin_port}:2019" "$image_tag" -lc '
    cat > /tmp/admin-config.json <<"JSON"
{
  "admin": {"listen": ":2019"},
  "apps": {
    "http": {
      "servers": {
        "admin": {
          "listen": [":2081"],
          "routes": [
            {
              "handle": [
                { "handler": "static_response", "body": "admin-ok", "status_code": 200 }
              ]
            }
          ]
        }
      }
    }
  }
}
JSON
    caddy run --config /tmp/admin-config.json
  ' >/dev/null

  local attempts=0
  until curl -sS "http://127.0.0.1:${admin_port}/config/" >/dev/null 2>&1; do
    attempts=$((attempts + 1))
    if [[ $attempts -ge 30 ]]; then
      docker logs "$run_id" || true
      docker rm -f "$run_id" >/dev/null 2>&1 || true
      return 1
    fi
    sleep 1
  done

  docker rm -f "$run_id" >/dev/null 2>&1 || true
}

extract_module_inventory() {
  local image_tag="$1"
  local output_prefix="$2"

  local container_id
  container_id="$(docker create --pull=never "$image_tag")"
  docker cp "${container_id}:/usr/bin/caddy" "${output_prefix}-caddy"
  docker rm "$container_id" >/dev/null

  if command -v go >/dev/null 2>&1; then
    go version -m "${output_prefix}-caddy" > "${output_prefix}-go-version-m.txt" || true
  else
    echo "go toolchain not available; module inventory skipped" > "${output_prefix}-go-version-m.txt"
  fi

  docker run --rm --pull=never --entrypoint caddy "$image_tag" list-modules > "${output_prefix}-modules.txt"
}

run_cell() {
  local scenario="$1"
  local platform="$2"
  local cell_index="$3"
  local summary_csv="$OUTPUT_DIR/matrix-summary.csv"
  local safe_platform
  safe_platform="${platform//\//-}"

  local image_tag="${BASE_IMAGE_TAG}:caddy-${CANDIDATE_VERSION}-candidate-${scenario}-${safe_platform}"
  local module_prefix="$OUTPUT_DIR/module-inventory-${scenario}-${safe_platform}"
  local modules_list_file="$OUTPUT_DIR/modules-${scenario}-${safe_platform}.txt"
  local admin_port=$((22019 + cell_index))
  local checked_plugins
  checked_plugins="${REQUIRED_MODULES[*]}"
  checked_plugins="${checked_plugins// /;}"

  echo "[compat] building cell scenario=${scenario} platform=${platform}"

  local boot_status="FAIL"
  local modules_status="FAIL"
  local validate_status="FAIL"
  local admin_status="FAIL"
  local inventory_status="FAIL"
  local cell_status="FAIL"

  if build_image_for_cell "$scenario" "$platform" "$image_tag"; then
    smoke_boot_caddy "$image_tag" && boot_status="PASS" || boot_status="FAIL"
    smoke_plugin_modules "$image_tag" "$modules_list_file" && modules_status="PASS" || modules_status="FAIL"
    smoke_config_validate "$image_tag" && validate_status="PASS" || validate_status="FAIL"
    smoke_admin_api_health "$image_tag" "$admin_port" && admin_status="PASS" || admin_status="FAIL"

    if extract_module_inventory "$image_tag" "$module_prefix"; then
      inventory_status="PASS"
    fi
  fi

  if [[ "$boot_status" == "PASS" && "$modules_status" == "PASS" && "$validate_status" == "PASS" && "$admin_status" == "PASS" && "$inventory_status" == "PASS" ]]; then
    cell_status="PASS"
  fi

  echo "${scenario},${platform},${image_tag},${checked_plugins},${boot_status},${modules_status},${validate_status},${admin_status},${inventory_status},${cell_status}" >> "$summary_csv"
  echo "[compat] RESULT scenario=${scenario} platform=${platform} status=${cell_status}"

  if [[ "$KEEP_IMAGES" != "1" ]]; then
    docker image rm "$image_tag" >/dev/null 2>&1 || true
  fi
}

write_docs_report() {
  local summary_csv="$OUTPUT_DIR/matrix-summary.csv"
  local generated_at
  generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

  {
    echo "# PR-1 Caddy Compatibility Matrix Report"
    echo
    echo "- Generated at: ${generated_at}"
    echo "- Candidate Caddy version: ${CANDIDATE_VERSION}"
    echo "- Plugin set: ${PLUGIN_SET}"
    echo "- Smoke set: ${SMOKE_SET}"
    echo "- Matrix dimensions: patch scenario × platform/arch × checked plugin modules"
    echo
    echo "## Deterministic Pass/Fail"
    echo
    echo "A matrix cell is PASS only when every smoke check and module inventory extraction passes."
    echo
    echo "Promotion gate semantics (spec-aligned):"
    echo "- Scenario A on linux/amd64 and linux/arm64 is promotion-gating."
    echo "- Scenario B/C are evidence-only; failures in B/C do not fail the PR-1 promotion gate."
    echo
    echo "## Matrix Output"
    echo
    echo "| Scenario | Platform | Plugins Checked | boot_caddy | plugin_modules | config_validate | admin_api_health | module_inventory | Status |"
    echo "| --- | --- | --- | --- | --- | --- | --- | --- | --- |"

    tail -n +2 "$summary_csv" | while IFS=',' read -r scenario platform _image checked_plugins boot modules validate admin inventory status; do
      local plugins_display
      plugins_display="${checked_plugins//;/, }"
      echo "| ${scenario} | ${platform} | ${plugins_display} | ${boot} | ${modules} | ${validate} | ${admin} | ${inventory} | ${status} |"
    done

    echo
    echo "## Artifacts"
    echo
    echo "- Matrix CSV: ${OUTPUT_DIR}/matrix-summary.csv"
    echo "- Per-cell module inventories: ${OUTPUT_DIR}/module-inventory-*-go-version-m.txt"
    echo "- Per-cell Caddy module listings: ${OUTPUT_DIR}/module-inventory-*-modules.txt"
  } > "$DOCS_REPORT"
}

main() {
  parse_args "$@"

  require_cmd docker
  require_cmd curl

  prepare_dirs
  write_reports_header

  local -a scenario_list
  local -a platform_list

  IFS=',' read -r -a scenario_list <<< "$PATCH_SCENARIOS"
  IFS=',' read -r -a platform_list <<< "$PLATFORMS"

  enforce_required_gate_dimensions scenario_list platform_list

  local cell_index=0
  local scenario
  local platform

  for scenario in "${scenario_list[@]}"; do
    for platform in "${platform_list[@]}"; do
      run_cell "$scenario" "$platform" "$cell_index"
      cell_index=$((cell_index + 1))
    done
  done

  write_docs_report

  local summary_csv="$OUTPUT_DIR/matrix-summary.csv"
  validate_matrix_completeness "$summary_csv" scenario_list platform_list
  evaluate_promotion_gate "$summary_csv"
}

main "$@"

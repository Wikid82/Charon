#!/usr/bin/env bash
# scripts/toolchain-key.sh
#
# Prints the deterministic, content-addressed tag for the prebuilt Caddy +
# CrowdSec toolchain image (spec §3.4.2).
#
#   Output:  caddy-crowdsec-<16 hex>
#
# The tag is a SHA-256 over every security-relevant toolchain input:
#   1. the exact text of the `caddy-inline` Dockerfile stage
#   2. the exact text of the `crowdsec-inline` Dockerfile stage
#   3. the resolved default values of every version ARG the two stages consume
#      (including the two now-pinned xcaddy plugins, B4)
#   4. the `tonistiigi/xx` pin line and the digest-pinned `golang:*-alpine`
#      builder-base lines of both inline stages (N4)
#   5. sha256 of .trivyignore
#   6. a SCHEMA_VERSION constant (bump to force a global rebuild if this
#      extraction logic itself changes)
#
# Because the stage *bodies* only interpolate ${ARG}, a bump to e.g. CADDY_VERSION
# would not change (1)/(2); (3) is what makes such a bump change the key.
#
# Usage:  scripts/toolchain-key.sh [path/to/Dockerfile]

set -euo pipefail

# rev-2: added the two xcaddy plugin pins + the digest-pinned golang base lines
# to the hashed input set.
SCHEMA_VERSION=2

df="${1:-Dockerfile}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=scripts/lib/dockerfile-stage.sh
source "$here/lib/dockerfile-stage.sh"

if [[ ! -f "$df" ]]; then
  echo "toolchain-key: Dockerfile not found: $df" >&2
  exit 2
fi

df_dir="$(cd "$(dirname "$df")" && pwd)"
if [[ -f "$df_dir/.trivyignore" ]]; then
  trivyignore="$df_dir/.trivyignore"
elif [[ -f .trivyignore ]]; then
  trivyignore=".trivyignore"
else
  echo "toolchain-key: .trivyignore not found (looked in $df_dir and CWD)" >&2
  exit 2
fi

caddy_stage="$(extract_stage caddy-inline "$df")"
crowdsec_stage="$(extract_stage crowdsec-inline "$df")"

# Sanity: each stage must be non-trivial and actually build a binary. Guards
# against a future edit that removes a stage or breaks extraction (spec §3.10).
for pair in "caddy-inline:$caddy_stage" "crowdsec-inline:$crowdsec_stage"; do
  name="${pair%%:*}"
  body="${pair#*:}"
  if [[ "$(printf '%s\n' "$body" | wc -l)" -lt 20 ]] \
     || ! printf '%s\n' "$body" | grep -Eq 'go build|xx-go build'; then
    echo "toolchain-key: stage '$name' extraction looks wrong (too short or no build step)" >&2
    exit 3
  fi
done

# ARG names whose default values feed the key. Keep in sync with spec §2.2 / §3.4.2.
arg_re='^ARG (GO_VERSION|ALPINE_IMAGE|CROWDSEC_VERSION|EXPR_LANG_VERSION|XNET_VERSION|XCRYPTO_VERSION|KLAUSPOST_COMPRESS_VERSION|GRPC_VERSION|CADDY_VERSION|CADDY_CANDIDATE_VERSION|CADDY_USE_CANDIDATE|CADDY_PATCH_SCENARIO|CADDY_SECURITY_VERSION|CORAZA_CADDY_VERSION|CADDY_GEOIP2_VERSION|CADDY_RATELIMIT_VERSION)='

key="$(
  {
    echo "schema=$SCHEMA_VERSION"
    printf '%s\n' "$caddy_stage"
    printf '%s\n' "$crowdsec_stage"
    grep -E "$arg_re" "$df"
    grep -E 'tonistiigi/xx:|^FROM .*golang:.*-alpine@sha256:' "$df"
    sha256sum "$trivyignore" | cut -d' ' -f1
  } | sha256sum | cut -c1-16
)"

printf 'caddy-crowdsec-%s\n' "$key"

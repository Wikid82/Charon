#!/usr/bin/env bash
# Prints the default value of a Dockerfile ARG so CI workflows can build from
# the exact base image reference the Dockerfile declares (docker-build.yml and
# nightly-build.yml both resolve ALPINE_IMAGE through this script).
#
# Usage: dockerfile-arg.sh [--require-digest] <Dockerfile> <ARG_NAME>
#
# --require-digest fails unless the value pins an image by sha256 digest
# (name:tag@sha256:<64 hex>), so a build never re-resolves a mutable tag.
#
# Exit codes: 0 value printed, 1 ARG missing or digest requirement not met,
# 2 usage error.
set -euo pipefail

readonly DIGEST_RE='@sha256:[0-9a-f]{64}$'

usage() {
  echo "usage: $0 [--require-digest] <Dockerfile> <ARG_NAME>" >&2
  exit 2
}

require_digest=false
if [[ "${1:-}" == "--require-digest" ]]; then
  require_digest=true
  shift
fi
[[ $# -eq 2 ]] || usage

dockerfile="$1"
arg_name="$2"
[[ "$arg_name" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || usage
if [[ ! -f "$dockerfile" ]]; then
  echo "error: Dockerfile not found: $dockerfile" >&2
  exit 1
fi

# Take the first `ARG NAME=value` line. The trailing "=" keeps ARG_NAME from
# matching a longer name that shares its prefix.
line=$(grep -m1 "^ARG ${arg_name}=" "$dockerfile" || true)
if [[ -z "$line" ]]; then
  echo "error: no 'ARG ${arg_name}=' line in $dockerfile" >&2
  exit 1
fi
value="${line#"ARG ${arg_name}="}"
value="${value%$'\r'}"
value="${value%"${value##*[![:space:]]}"}"
if [[ -z "$value" ]]; then
  echo "error: 'ARG ${arg_name}' has an empty default in $dockerfile" >&2
  exit 1
fi

if [[ "$require_digest" == true && ! "$value" =~ $DIGEST_RE ]]; then
  echo "error: ${arg_name} must be pinned by sha256 digest: ${value}" >&2
  exit 1
fi

printf '%s\n' "$value"

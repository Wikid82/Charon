#!/usr/bin/env bash
# scripts/verify-toolchain-pin.sh
#
# Freshness guard for the prebuilt toolchain image pin (spec §3.4.2, B7).
# Fast, no Docker build. Run on every PR (wired into quality-checks.yml in
# spec Commit 3).
#
# It asserts:
#   1. The recomputed content key (scripts/toolchain-key.sh) equals the
#      ARG CHARON_TOOLCHAIN_TAG pinned in the Dockerfile. A mismatch means a
#      tracked pin / recipe line moved without the toolchain image being
#      rebuilt and re-pinned -> exit 1 with an actionable message.
#   2. On a TRUSTED same-repo run (SAME_REPO=1) it is FAILURE-CLOSED:
#        - `regctl` MUST be installed                       (else exit 1)
#        - GHCR_READ_TOKEN MUST be set                      (else exit 1)
#        - `:$KEY` MUST resolve in GHCR                      (else exit 1)
#        - the resolved digest MUST equal CHARON_TOOLCHAIN_DIGEST (else exit 1)
#      There is NO silent skip on the trusted path.
#   3. Only a FORK run (SAME_REPO=0), which has no registry access, degrades to
#      tag-only equality with a `::warning::`.
#
# SAME_REPO detection (spec §3.4.2):
#   push / same-repo pull_request / workflow_dispatch / schedule -> SAME_REPO=1
#   pull_request whose head repo != GITHUB_REPOSITORY            -> SAME_REPO=0
#
# Env:
#   GITHUB_EVENT_NAME
#   GITHUB_EVENT_PULL_REQUEST_HEAD_REPO_FULL_NAME  (each calling workflow must
#                                                  map github.event.pull_request
#                                                  .head.repo.full_name here)
#   GITHUB_REPOSITORY
#   GHCR_READ_TOKEN  (= secrets.GITHUB_TOKEN with packages:read)
#   TOOLCHAIN_IMAGE  (optional override; default from Dockerfile / hard default)

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$here/.." && pwd)"
df="${1:-$repo_root/Dockerfile}"

if [[ ! -f "$df" ]]; then
  echo "::error::verify-toolchain-pin: Dockerfile not found: $df"
  exit 2
fi

arg_value() { # $1 = ARG name
  grep -E "^ARG $1=" "$df" | head -n1 | cut -d= -f2-
}

KEY="$("$here/toolchain-key.sh" "$df")"
PINNED_TAG="$(arg_value CHARON_TOOLCHAIN_TAG)"
PINNED_DIGEST="$(arg_value CHARON_TOOLCHAIN_DIGEST)"
TOOLCHAIN_IMAGE="${TOOLCHAIN_IMAGE:-$(arg_value CHARON_TOOLCHAIN_IMAGE)}"
TOOLCHAIN_IMAGE="${TOOLCHAIN_IMAGE:-ghcr.io/wikid82/charon-toolchain}"

if [[ -z "$PINNED_TAG" ]]; then
  echo "::error::verify-toolchain-pin: ARG CHARON_TOOLCHAIN_TAG not found in $df"
  exit 1
fi

# --- Trust classification -----------------------------------------------------
SAME_REPO=1
if [[ "${GITHUB_EVENT_NAME:-}" == "pull_request" \
   && "${GITHUB_EVENT_PULL_REQUEST_HEAD_REPO_FULL_NAME:-}" != "${GITHUB_REPOSITORY:-}" ]]; then
  SAME_REPO=0
fi

# --- (1) Tag / key equality (always) ----------------------------------------
if [[ "$KEY" != "$PINNED_TAG" ]]; then
  echo "::error::Toolchain recipe/pins changed (recomputed $KEY, Dockerfile pins $PINNED_TAG)."
  echo "::error::Run the 'Toolchain Image' workflow (workflow_dispatch) or wait for the bot PR,"
  echo "::error::then bump ARG CHARON_TOOLCHAIN_TAG / CHARON_TOOLCHAIN_DIGEST in the Dockerfile."
  exit 1
fi

# --- (2) / (3) Digest verification -----------------------------------------
if [[ "$SAME_REPO" == "1" ]]; then
  if ! command -v regctl >/dev/null 2>&1; then
    echo "::error::regctl missing on a same-repo run — cannot verify the toolchain digest."
    exit 1
  fi
  if [[ -z "${GHCR_READ_TOKEN:-}" ]]; then
    echo "::error::GHCR_READ_TOKEN unset on a same-repo run — cannot verify the toolchain digest."
    exit 1
  fi
  if [[ -z "$PINNED_DIGEST" ]]; then
    echo "::error::ARG CHARON_TOOLCHAIN_DIGEST not found in $df — cannot verify on a same-repo run."
    exit 1
  fi

  # Authenticate regctl to GHCR (best-effort; the digest call below is the real gate).
  regctl registry login ghcr.io \
    --user "${GITHUB_ACTOR:-x-access-token}" \
    --pass-stdin <<<"$GHCR_READ_TOKEN" >/dev/null 2>&1 || true

  if ! REMOTE_DIGEST="$(regctl image digest "${TOOLCHAIN_IMAGE}:${KEY}" 2>/dev/null)"; then
    echo "::error::${TOOLCHAIN_IMAGE}:${KEY} does not resolve in GHCR — the toolchain image"
    echo "::error::was never published for this pin. Dispatch the 'Toolchain Image' workflow."
    exit 1
  fi

  if [[ "$REMOTE_DIGEST" != "$PINNED_DIGEST" ]]; then
    echo "::error::Dockerfile pins CHARON_TOOLCHAIN_DIGEST=$PINNED_DIGEST but"
    echo "::error::${TOOLCHAIN_IMAGE}:${KEY} currently resolves to $REMOTE_DIGEST (hand-edited or stale)."
    exit 1
  fi

  echo "Toolchain pin verified (same-repo): $KEY @ $PINNED_DIGEST"
else
  echo "::warning::Fork PR — toolchain digest existence not verified (no registry access)."
  echo "::warning::Tag matches the recomputed key ($KEY). A maintainer re-running the trusted"
  echo "::warning::same-repo path performs the full digest check."
fi

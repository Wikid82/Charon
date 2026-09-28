#!/usr/bin/env bash
# Guards against the release-please date-ordering stranding bug documented in
# docs/troubleshooting/release-please-skips.md.
#
# Root cause (confirmed 2026-09-28, PR #1411): release-please's commit-
# collection walk on `main` stops once it reaches the previous release
# boundary's *commit date*, not true git ancestry. If a same-week
# release-please cut lands on `main` (creating a new boundary dated "now")
# while `nightly` still has release-worthy commits dated *older* than that
# boundary waiting to be promoted, those commits are silently dropped from
# every future release-please run once the promotion merge finally lands.
#
# This script decides whether the release-please job in
# .github/workflows/release-please.yml should run (defer=false) or be
# skipped for this push (defer=true) to avoid creating that boundary early.
# It is self-healing: the next push to `main` (another interim commit, or
# the eventual nightly promotion merge) re-runs this check. Once nightly's
# commits are ancestors of TARGET_REF, the range is empty and release-please
# resumes normally, correctly bundling everything that was deferred.
#
# Usage:
#   TARGET_REF=<sha-or-ref> SOURCE_REF=<sha-or-ref> scripts/ci/check-nightly-sequencing.sh
#
# Inputs (env vars, all optional):
#   TARGET_REF           Commit that would become the new release boundary
#                         if release-please proceeds. Default: HEAD.
#   SOURCE_REF            Tip of the branch that owns pending promotion work.
#                         Default: origin/nightly.
#   COMMIT_TYPE_PATTERN   Extended-regex subject-line pattern for commit
#                         types release-please treats as release-worthy in
#                         this repo (see CLAUDE.md's CI/CD & Commit
#                         Conventions section: feat/fix/perf trigger builds,
#                         deps: is release-triggering via Renovate, revert is
#                         a standard Conventional Commits release type).
#                         Default matches feat/fix/perf/revert/deps, with
#                         optional (scope) and breaking-change `!`.
#
# Output: two `key=value` lines on stdout (also appended to $GITHUB_OUTPUT
# when set, so this can be used directly as a GitHub Actions step):
#   defer=true|false
#   reason=<single-line human-readable explanation>
#
# Exit codes: 0 for any successful decision (including defer=true — that is
# a valid decision, not a script failure). Non-zero only on genuine usage
# errors (e.g. TARGET_REF does not resolve).

set -euo pipefail

TARGET_REF="${TARGET_REF:-HEAD}"
SOURCE_REF="${SOURCE_REF:-origin/nightly}"
COMMIT_TYPE_PATTERN="${COMMIT_TYPE_PATTERN:-^(feat|fix|perf|revert|deps)(\([^)]*\))?!?:}"
DOCS_LINK="docs/troubleshooting/release-please-skips.md"

emit() {
  local defer="$1"
  local reason="$2"
  echo "defer=${defer}"
  echo "reason=${reason}"
  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    {
      echo "defer=${defer}"
      echo "reason=${reason}"
    } >>"${GITHUB_OUTPUT}"
  fi
}

if ! git rev-parse --verify -q "${TARGET_REF}^{commit}" >/dev/null; then
  echo "check-nightly-sequencing: TARGET_REF '${TARGET_REF}' does not resolve to a commit" >&2
  exit 2
fi

if ! git rev-parse --verify -q "${SOURCE_REF}^{commit}" >/dev/null 2>&1; then
  emit "false" "SOURCE_REF '${SOURCE_REF}' does not resolve (branch missing or not fetched); nothing to guard against"
  exit 0
fi

target_epoch="$(git log -1 --format=%ct "${TARGET_REF}")"

# Commits reachable from SOURCE_REF but not TARGET_REF: nightly's unpromoted
# backlog relative to the boundary this release-please run would create.
# --no-merges excludes sync/promotion merge commits themselves; we only care
# about the real conventional commits they carry.
matches=()
while IFS=$'\x1f' read -r sha epoch iso subject; do
  [[ -z "${sha}" ]] && continue
  if [[ "${subject}" =~ ${COMMIT_TYPE_PATTERN} ]]; then
    matches+=("${sha}"$'\x1f'"${epoch}"$'\x1f'"${iso}"$'\x1f'"${subject}")
  fi
done < <(git log --no-merges --format='%H%x1f%ct%x1f%cI%x1f%s' "${TARGET_REF}..${SOURCE_REF}" 2>/dev/null || true)

if [[ "${#matches[@]}" -eq 0 ]]; then
  emit "false" "nightly has no release-worthy commits ahead of this boundary; safe to release"
  exit 0
fi

# Find the oldest qualifying commit (lowest committer-date epoch).
oldest=""
oldest_epoch=""
at_risk_count=0
for entry in "${matches[@]}"; do
  IFS=$'\x1f' read -r sha epoch iso subject <<<"${entry}"
  if [[ "${epoch}" -lt "${target_epoch}" ]]; then
    at_risk_count=$((at_risk_count + 1))
    if [[ -z "${oldest_epoch}" || "${epoch}" -lt "${oldest_epoch}" ]]; then
      oldest_epoch="${epoch}"
      oldest="${sha:0:7} @ ${iso} - ${subject}"
    fi
  fi
done

if [[ "${at_risk_count}" -eq 0 ]]; then
  emit "false" "nightly has ${#matches[@]} release-worthy commit(s) ahead, but none predate this boundary; safe to release"
  exit 0
fi

emit "true" "nightly has ${at_risk_count} unpromoted release-worthy commit(s) older than this release boundary (oldest: ${oldest}); deferring to avoid the release-please date-ordering stranding bug, see ${DOCS_LINK}"

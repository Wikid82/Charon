#!/usr/bin/env bash
# Install/pin a known-working CodeQL CLI for local dev use.
#
# WHY THIS EXISTS:
#   scripts/pre-commit-hooks/codeql-go-scan.sh and codeql-js-scan.sh (run via
#   `lefthook run codeql`) invoke a bare `codeql` binary with zero version
#   pinning — they simply assume a working CLI is already on PATH. Sandbox /
#   dev-container images can ship a stale system `codeql` (observed: 2.16.0)
#   that fails to resolve this repo's pinned query packs
#   (codeql/go-queries:codeql-suites/go-security-and-quality.qls and the JS
#   equivalent) with "extensions-by-pack" resolution errors.
#
#   CI (.github/workflows/codeql.yml) is NOT affected by this — it drives
#   CodeQL entirely through github/codeql-action@v4 (pinned by SHA), which
#   downloads and manages its own CLI toolchain independent of anything on
#   the runner's PATH. This script only fixes the local/dev-environment gap.
#
# WHAT THIS DOES:
#   1. Ensures the `gh-codeql` GitHub CLI extension is installed. It manages
#      its own versioned CodeQL CLI downloads, independent of any system
#      package.
#   2. Pins it to a known-working version for this repo's query pack pins
#      (see CODEQL_VERSION below).
#   3. Installs a `codeql` shim on PATH (via `gh codeql install-stub`) that
#      forwards to `gh codeql`, so every existing script that calls a bare
#      `codeql` command (codeql-go-scan.sh, codeql-js-scan.sh,
#      tools/codeql_scan.sh, .github/skills/security-scan-codeql-scripts,
#      VS Code tasks) works unmodified.
#
# Safe to re-run; every step is idempotent.
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

# Known-working CodeQL CLI version for this repo's query pack pins
# (codeql/go-queries and codeql/javascript-queries security-and-quality
# suites in scripts/pre-commit-hooks/codeql-{go,js}-scan.sh). Bump
# deliberately and re-validate `lefthook run codeql` locally before raising
# this — an exact pin (not "latest") keeps local scans reproducible, per
# CLAUDE.md's "use exact dependency versions" convention.
CODEQL_VERSION="${CODEQL_VERSION:-v2.26.3}"

# Where to install the `codeql` PATH shim. Defaults to the first writable,
# user-owned directory already on PATH so no sudo is required.
STUB_DIR="${1:-${HOME}/.local/bin}"

echo -e "${BLUE}Installing/pinning CodeQL CLI (target: ${CODEQL_VERSION})...${NC}"

if ! command -v gh >/dev/null 2>&1; then
  echo -e "${RED}gh (GitHub CLI) is required but not found on PATH.${NC}"
  echo "Install it first: https://cli.github.com/"
  exit 1
fi

if ! gh extension list 2>/dev/null | grep -q "github/gh-codeql"; then
  echo "Installing gh-codeql extension..."
  gh extension install github/gh-codeql
else
  echo "gh-codeql extension already installed."
fi

echo "Pinning CodeQL CLI to ${CODEQL_VERSION}..."
gh codeql set-version "${CODEQL_VERSION}"

mkdir -p "${STUB_DIR}"
echo "Installing 'codeql' PATH shim into ${STUB_DIR}..."
gh codeql install-stub "${STUB_DIR}"

if ! echo "${PATH}" | tr ':' '\n' | grep -qx "${STUB_DIR}"; then
  echo -e "${RED}Warning:${NC} ${STUB_DIR} is not on PATH. Add it, e.g.:"
  echo "  export PATH=\"${STUB_DIR}:\$PATH\""
fi

echo ""
echo -e "${GREEN}Verifying...${NC}"
if command -v codeql >/dev/null 2>&1; then
  codeql version
  echo -e "${GREEN}codeql resolves on PATH via the gh-codeql stub.${NC}"
else
  echo -e "${RED}codeql still not found on PATH after install-stub — check STUB_DIR.${NC}"
  exit 1
fi

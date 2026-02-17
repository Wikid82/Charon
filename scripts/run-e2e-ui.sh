#!/usr/bin/env bash
# Lightweight wrapper to run Playwright UI on headless Linux by auto-starting Xvfb when needed.
# Usage: ./scripts/run-e2e-ui.sh [<playwright args>]
set -euo pipefail
cd "$(dirname "$0")/.." || exit 1

LOGFILE="/tmp/xvfb.playwright.log"

if [[ -n "${CI-}" ]]; then
  echo "Playwright UI is not supported in CI. Use the project's E2E Docker image or run headless: npm run e2e" >&2
  exit 1
fi

if [[ -z "${DISPLAY-}" ]]; then
  if command -v Xvfb >/dev/null 2>&1; then
    echo "Starting Xvfb :99 (logs: ${LOGFILE})"
    Xvfb :99 -screen 0 1280x720x24 >"${LOGFILE}" 2>&1 &
    disown
    export DISPLAY=:99
    sleep 0.2
  elif command -v xvfb-run >/dev/null 2>&1; then
    echo "Using xvfb-run to launch Playwright UI"
    exec xvfb-run --auto-servernum --server-args='-screen 0 1280x720x24' npx playwright test --ui "$@"
  else
    echo "No X server found and Xvfb is not installed.\nInstall Xvfb (e.g. sudo apt install xvfb) or run headless tests: npm run e2e" >&2
    exit 1
  fi
fi

# At this point DISPLAY should be set — run Playwright UI
exec npx playwright test --ui "$@"

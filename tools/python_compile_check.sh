#!/usr/bin/env bash
set -euo pipefail

# Run python -m compileall quietly to catch syntax errors in the repo.
if command -v python3 &>/dev/null; then
    python3 -m compileall -q .
elif command -v python &>/dev/null; then
    python -m compileall -q .
else
    echo "Error: neither python3 nor python found."
    exit 1
fi

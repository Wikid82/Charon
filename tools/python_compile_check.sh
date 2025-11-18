#!/usr/bin/env bash
set -euo pipefail

# Run python -m compileall quietly to catch syntax errors in the repo.
python -m compileall -q .

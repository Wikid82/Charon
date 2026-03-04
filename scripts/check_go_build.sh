#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
echo "[charon] repo root: $ROOT_DIR"

echo "-- go version --"
go version || true

echo "-- go env --"
go env || true

echo "-- go list (backend) --"
cd "$ROOT_DIR/backend"
echo "module: $(cat go.mod | sed -n '1p')"
go list -deps ./... | wc -l || true

echo "-- go build backend ./... --"
if go build ./...; then
  echo "BUILD_OK"
  exit 0
else
  echo "BUILD_FAIL"
  echo "Run 'cd backend && go build -v ./...' for verbose output"
  exit 2
fi

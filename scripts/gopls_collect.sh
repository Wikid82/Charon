#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="/tmp/charon-gopls-logs-$(date +%s)"
mkdir -p "$OUT_DIR"
echo "Collecting gopls debug output to $OUT_DIR"

if ! command -v gopls >/dev/null 2>&1; then
  echo "gopls not found in PATH. Install with: go install golang.org/x/tools/gopls@latest"
  exit 2
fi

cd "$ROOT_DIR/backend"
echo "Running: gopls -rpc.trace -v check ./... > $OUT_DIR/gopls.log 2>&1"
gopls -rpc.trace -v check ./... > "$OUT_DIR/gopls.log" 2>&1 || true

echo "Also collecting 'go env' and 'go version'"
go version > "$OUT_DIR/go-version.txt" 2>&1 || true
go env > "$OUT_DIR/go-env.txt" 2>&1 || true

echo "Logs collected at: $OUT_DIR"
echo "Attach the $OUT_DIR contents when filing issues against golang/vscode-go or gopls."

#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"

# Modules to enforce 100% coverage on
BACKEND_PKGS=${BACKEND_PKGS:-"./internal/cerberus ./internal/caddy"}
FRONTEND_FILES=${FRONTEND_FILES:-"src/pages/ProxyHosts.tsx"}

# Optional flags: --backend-only | --frontend-only
ONLY_BACKEND=0
ONLY_FRONTEND=0
for arg in "$@"; do
  case "$arg" in
    --backend-only) ONLY_BACKEND=1 ;;
    --frontend-only) ONLY_FRONTEND=1 ;;
  esac
done

cd "$ROOT_DIR"

echo "== Module coverage enforcement: Backend packages: $BACKEND_PKGS | Frontend files: $FRONTEND_FILES =="

## Backend package coverage checks
if [ $ONLY_FRONTEND -eq 0 ] && [ -d "$BACKEND_DIR" ]; then
  cd "$BACKEND_DIR"
  for pkg in $BACKEND_PKGS; do
    out="coverage.${pkg//\//_}.out"
    echo "-> Running tests for backend package $pkg (coverage -> $out)"
    go test -coverprofile="$out" "$pkg"
    totalPct=$(go tool cover -func="$out" | tail -n 1 | awk '{print $3}')
    totalPctNum=$(echo "$totalPct" | sed 's/%//')
    if [ "$totalPctNum" != "100.0" ] && [ "$totalPctNum" != "100" ]; then
      echo "ERROR: Coverage for package $pkg is ${totalPct} (require 100%)"
      echo "Uncovered file:line ranges (file:startline-endline):"
      awk '$NF==0 {split($1,a,":"); split(a[2],r,","); split(r[1],s,"."); split(r[2],e,"."); printf "%s:%s-%s\n", a[1], s[1], e[1]}' "$out" | sort -u
      exit 1
    else
      echo "OK: package $pkg has 100% coverage"
    fi
  done
fi

## Frontend file coverage checks
if [ $ONLY_BACKEND -eq 0 ] && [ -d "$FRONTEND_DIR" ]; then
  cd "$FRONTEND_DIR"
  # If coverage not present, generate it
  # Only re-run coverage if BOTH common artifact types are missing. Some reporters
  # (e.g. istanbul vs v8) only produce one of these; requiring both missing
  # avoids re-running coverage repeatedly when a single reporter is used.
  if [ ! -f "coverage/coverage-summary.json" ] && [ ! -f "coverage/lcov.info" ]; then
    echo "Frontend coverage artifacts missing, running coverage tests"
    bash "$ROOT_DIR/scripts/frontend-test-coverage.sh"
  fi

  for f in $FRONTEND_FILES; do
    # coverage-summary.json uses relative file keys, so attempt both
    # Try to find the exact file key
    pct=$(python3 - <<PY
import json,sys,os
f = '$f'
try:
  d = json.load(open('coverage/coverage-summary.json'))
except Exception:
  sys.exit(0)
# Try a few different key formats: exact, absolute, and suffix match
val = None
if f in d:
  val = d[f].get('statements', {}).get('pct', None)
else:
  absf = os.path.abspath(f)
  if absf in d:
    val = d[absf].get('statements', {}).get('pct', None)
  else:
    # fallback: find any key that ends with the file path
    for k in d.keys():
      if k.endswith(f) or k.endswith(absf) or k.endswith(os.path.join(os.getcwd(), f)):
        val = d[k].get('statements', {}).get('pct', None)
        break
if val is None:
  sys.exit(0)
print(val)
PY
    )

    if [ -z "$pct" ]; then
      # fallback to lcov parsing: show uncovered lines for the file
      echo "WARNING: Could not find $f in coverage-summary.json; checking lcov.info for uncovered lines"
      # lcov contains SF: <absolute path> lines, attempt to match file ending
      if [ -f coverage/lcov.info ]; then
        awk -v file="$f" '/^SF:/ { inFile = (index($0,file) != 0) } inFile && /^DA:/{ split($0,a,":"); split(a[2],b,","); if (b[2] == "0") print b[1] }' coverage/lcov.info || true
      else
        echo "No lcov.info available to check uncovered lines"
      fi
      echo "Failed to parse file coverage for $f"
      exit 1
    fi

    if [ "$pct" != "100" ] && [ "$pct" != "100.0" ]; then
      echo "ERROR: Frontend file $f coverage is $pct% (require 100%)"
      echo "Uncovered lines in lcov for $f:"
      if [ -f coverage/lcov.info ]; then
        awk -v file="$f" '/^SF:/ { inFile = (index($0,file) != 0) } inFile && /^DA:/{ split($0,a,":"); split(a[2],b,","); if (b[2] == "0") print b[1] }' coverage/lcov.info || true
      else
        echo "No lcov.info available to show uncovered lines"
      fi
      # Show more helpful snippets: lines with 0 hits
      if [ -f coverage/lcov.info ]; then
        awk -v file="$f" '/^SF:/ { inFile = (index($0,file) != 0); next } inFile && /^DA:/{ split($0,a,":"); split(a[2],b,","); if (b[2] == "0") print "line " b[1] " had 0 hits" }' coverage/lcov.info || true
      else
        echo "No lcov.info available to show uncovered lines"
      fi
      exit 1
    else
      echo "OK: frontend file $f has 100% coverage"
    fi
  done
fi

echo "All module coverage checks passed"

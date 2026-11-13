#!/usr/bin/env bash
# Merges Go coverprofiles produced by disjoint `go test` shards into one profile.
#
# Usage: merge-go-coverage.sh <output-file> <input-file> [<input-file> ...]
#
# Every input must start with a `mode:` header and all inputs must share the
# same mode (the backend runs with -race, so this is `atomic`). The output keeps
# a single header from the first input followed by every data line from every
# input. Shards test disjoint package sets, so no block is emitted twice; if a
# future change ever overlapped shards, go tool cover still sums duplicate blocks
# when it parses the merged profile.
set -euo pipefail

if [[ $# -lt 2 ]]; then
    echo "Usage: $(basename "$0") <output-file> <input-file> [<input-file> ...]" >&2
    exit 2
fi

OUTPUT_FILE="$1"
shift

expected_mode=""
for input in "$@"; do
    if [ ! -f "$input" ]; then
        echo "Error: coverage input not found: $input" >&2
        exit 1
    fi
    header="$(head -n 1 "$input")"
    if [[ "$header" != mode:\ * ]]; then
        echo "Error: $input does not start with a 'mode:' header" >&2
        exit 1
    fi
    mode="${header#mode: }"
    if [ -z "$expected_mode" ]; then
        expected_mode="$mode"
    elif [ "$mode" != "$expected_mode" ]; then
        echo "Error: coverage mode mismatch: $input is '$mode', expected '$expected_mode'" >&2
        exit 1
    fi
done

OUTPUT_DIR="$(dirname "$OUTPUT_FILE")"
mkdir -p "$OUTPUT_DIR"
TMP_OUTPUT="$(mktemp "$OUTPUT_DIR/.merge-coverage.XXXXXX")"
trap 'rm -f "$TMP_OUTPUT"' EXIT

printf 'mode: %s\n' "$expected_mode" > "$TMP_OUTPUT"
for input in "$@"; do
    # Skip each file's own header; keep every non-empty data line.
    tail -n +2 "$input" | grep -v '^$' >> "$TMP_OUTPUT" || true
done

mv "$TMP_OUTPUT" "$OUTPUT_FILE"
trap - EXIT

echo "Merged $# coverage profile(s) into $OUTPUT_FILE"

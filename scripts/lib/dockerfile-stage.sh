#!/usr/bin/env bash
# scripts/lib/dockerfile-stage.sh
#
# SHARED helper (spec §3.4.2 / N9). Sourced by BOTH scripts/toolchain-key.sh and
# scripts/verify-toolchain-pin.sh so the "extract one Dockerfile stage" logic
# exists in exactly one place.
#
# extract_stage <stage-name> <dockerfile-path>
#   Prints the body of the named build stage: its `FROM ... AS <name>` header
#   through its last real instruction line. Comment / blank lines that sit
#   between the stage's last instruction and the next `FROM` (i.e. the *next*
#   stage's preamble) are NOT included, so re-wording a stage's header comment
#   does not perturb the *previous* stage's extracted text — and therefore does
#   not perturb the toolchain key for unrelated edits. Comments that appear
#   mid-stage (followed by more instructions before the next `FROM`) ARE kept.
#   Exits 3 if the stage is not found.
#
# This file is meant to be `source`d, not executed.

# shellcheck shell=bash

extract_stage() {
  local stage="$1" file="$2"

  if [[ -z "$stage" || -z "$file" ]]; then
    echo "extract_stage: usage: extract_stage <stage-name> <dockerfile-path>" >&2
    return 2
  fi
  if [[ ! -f "$file" ]]; then
    echo "extract_stage: no such file: $file" >&2
    return 2
  fi

  awk -v s="$stage" '
    # Stage name declared by a FROM line ("" if it has no `AS`). Handles
    # `FROM --platform=$BUILDPLATFORM img:tag@sha256:... AS name`.
    function stagename(line,   n) {
      if (match(line, /[ \t][Aa][Ss][ \t]+[A-Za-z0-9._-]+[ \t]*$/)) {
        n = substr(line, RSTART)
        sub(/^[ \t]+[Aa][Ss][ \t]+/, "", n)
        sub(/[ \t]+$/, "", n)
        return n
      }
      return ""
    }
    function flush_pending(   i) {
      for (i = 1; i <= np; i++) print pending[i]
      np = 0
    }
    /^FROM[ \t]/ {
      nm = stagename($0)
      if (capturing && nm != s) { exit }   # next stage reached; drop pending buffer
      if (nm == s) { found = 1; capturing = 1; np = 0; print $0; next }
    }
    capturing {
      # Hold blank / comment-only lines: they might be the next stage preamble.
      if ($0 ~ /^[ \t]*$/ || $0 ~ /^[ \t]*#/) {
        pending[++np] = $0
      } else {
        flush_pending()
        print $0
      }
      next
    }
    END {
      if (!found) {
        print "extract_stage: no stage \"" s "\"" > "/dev/stderr"
        exit 3
      }
      # Trailing pending lines (comments/blanks before the next FROM or EOF)
      # are intentionally dropped.
    }
  ' "$file"
}

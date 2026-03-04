#!/usr/bin/env bash
set -euo pipefail

# Repo health check script
# Exits 0 when everything is OK, non-zero otherwise.

MAX_MB=${MAX_MB-100} # threshold in MB for detecting large files
LFS_ALLOW_MB=${LFS_ALLOW_MB-50} # threshold for LFS requirement

echo "Running repo health checks..."

echo "Repository path: $(pwd)"

# Git object/pack stats
echo "-- Git pack stats --"
git count-objects -vH || true

# Disk usage for repository (human & bytes)
echo "-- Disk usage (top-level) --"
du -sh . || true
du -sb . | awk '{print "Total bytes:", $1}' || true

echo "-- Largest files (>${MAX_MB}MB) --"
find . -type f -size +"${MAX_MB}"M -not -path "./.git/*" -print -exec du -h {} + | sort -hr | head -n 50 > /tmp/repo_big_files.txt || true
if [ -s /tmp/repo_big_files.txt ]; then
  echo "Large files found:"
  cat /tmp/repo_big_files.txt
else
  echo "No large files found (> ${MAX_MB}MB)"
fi

echo "-- CodeQL DB directories present? --"
if [ -d "codeql-db" ] || ls codeql-db-* >/dev/null 2>&1; then
  echo "Found codeql-db directories. These should not be committed." >&2
  exit 2
else
  echo "No codeql-db directories found in repo root. OK"
fi

echo "-- Detect files > ${LFS_ALLOW_MB}MB not using Git LFS --"
FAILED=0
# Use NUL-separated find results to safely handle filenames with spaces/newlines
found_big_files=0
while IFS= read -r -d '' f; do
  found_big_files=1
    # check if file path is tracked by LFS
    if git ls-files --stage -- "${f}" >/dev/null 2>&1; then
      # check attr filter value
      filter_attr=$(git check-attr --stdin filter <<<"${f}" | awk '{print $3}') || true
      if [ "$filter_attr" != "lfs" ]; then
        echo "Large file not tracked by Git LFS: ${f}" >&2
        FAILED=1
      fi
    else
      # file not in git index yet, still flagged to maintainers
      echo "Large untracked file (in working tree): ${f}" >&2
      FAILED=1
    fi
done < <(find . -type f -size +"${LFS_ALLOW_MB}"M -not -path "./.git/*" -print0)
if [ "$found_big_files" -eq 0 ]; then
  echo "No files larger than ${LFS_ALLOW_MB}MB found"
fi

if [ $FAILED -ne 0 ]; then
  echo "Repository health check failed: Large files not tracked by LFS or codeql-db committed." >&2
  exit 3
fi

echo "Repo health check complete: OK"
exit 0

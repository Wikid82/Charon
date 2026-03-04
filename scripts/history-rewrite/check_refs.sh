#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

usage() {
  cat <<EOF
Usage: $0

Lists branches and tags, saves a tag reference tarball to data/backups.
EOF
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage; exit 0
fi

logdir="data/backups"
mkdir -p "$logdir"
ts=$(date +"%Y%m%d-%H%M%S")
tags_tar="$logdir/tags-$ts.tar.gz"

echo "Branches:"
git branch -a || true

echo "Tags:"
git tag -l || true

tmpdir=$(mktemp -d)
git show-ref --tags > "$tmpdir/tags-show-ref.txt" || true
tar -C "$tmpdir" -czf "$tags_tar" . || { echo "Warning: failed to create tag tarball" >&2; rm -rf "$tmpdir"; exit 1; }
rm -rf "$tmpdir"
echo "Created tags tarball: $tags_tar"

echo "Attempting to push tags to origin under refs/backups/tags/*"
for t in $(git tag --list); do
  if ! git push origin "refs/tags/$t:refs/backups/tags/$t" >/dev/null 2>&1; then
    echo "Warning: pushing tag $t to refs/backups/tags/$t failed" >&2
  fi
done

echo "Done."
exit 0

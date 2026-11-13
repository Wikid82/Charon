#!/usr/bin/env bash
# Prints the Go packages one CI shard should test, one import path per line.
#
# Usage: shard-go-packages.sh <index> <total>   (index is 1-based: 1..total)
#
# The union of all shards is exactly `go list ./...` (each package appears in
# exactly one shard), and the assignment is deterministic for a given package
# set. Packages listed in HEAVY_PACKAGES are slow enough that each gets a
# dedicated shard so they never stack with each other:
#
#   * total > number of heavy packages: heavy package k goes alone to shard k
#     (1-based); every remaining package is spread round-robin (sorted order)
#     over the leftover shards.
#   * total <= number of heavy packages: heavy and remaining packages are
#     dealt round-robin over all shards in sequence (heavy first).
#
# Runs `go list` from backend/ (module root). Heavy entries are matched as a
# path suffix (`/<entry>`) so the module path does not need to be hard-coded.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"

# Ordered by observed test time (slowest first). internal/services takes ~9.5m
# under -race on CI runners; internal/api/handlers is the next heaviest.
HEAVY_PACKAGES=(
    "internal/services"
    "internal/api/handlers"
)

usage() {
    echo "Usage: $(basename "$0") <index> <total>   (1 <= index <= total)" >&2
    exit 2
}

[[ $# -eq 2 ]] || usage
index="$1"
total="$2"
[[ "$index" =~ ^[0-9]+$ && "$total" =~ ^[0-9]+$ ]] || usage
(( total >= 1 && index >= 1 && index <= total )) || usage

# Read the package list through a function so tests can stub `go` on PATH.
list_packages() {
    (cd "$BACKEND_DIR" && go list ./...) | LC_ALL=C sort
}

mapfile -t all_packages < <(list_packages)
if (( ${#all_packages[@]} == 0 )); then
    echo "Error: go list ./... returned no packages" >&2
    exit 1
fi

# Heavy packages in HEAVY_PACKAGES order, skipping entries absent from the list.
heavy=()
for entry in "${HEAVY_PACKAGES[@]}"; do
    for pkg in "${all_packages[@]}"; do
        if [[ "$pkg" == */"$entry" ]]; then
            heavy+=("$pkg")
        fi
    done
done

is_heavy() {
    local candidate="$1" h
    for h in "${heavy[@]+"${heavy[@]}"}"; do
        [[ "$h" == "$candidate" ]] && return 0
    done
    return 1
}

rest=()
for pkg in "${all_packages[@]}"; do
    is_heavy "$pkg" || rest+=("$pkg")
done

heavy_count=${#heavy[@]}
selected=()

if (( total > heavy_count )); then
    if (( index <= heavy_count )); then
        selected+=("${heavy[index - 1]}")
    fi
    # Shards heavy_count+1..total share the remaining packages.
    spread_shards=$(( total - heavy_count ))
    spread_slot=$(( index - heavy_count - 1 ))
    if (( spread_slot >= 0 )); then
        for j in "${!rest[@]}"; do
            if (( j % spread_shards == spread_slot )); then
                selected+=("${rest[j]}")
            fi
        done
    fi
else
    sequence=("${heavy[@]+"${heavy[@]}"}" "${rest[@]+"${rest[@]}"}")
    for n in "${!sequence[@]}"; do
        if (( n % total == index - 1 )); then
            selected+=("${sequence[n]}")
        fi
    done
fi

if (( ${#selected[@]} == 0 )); then
    echo "Error: shard ${index}/${total} has no packages (fewer packages than shards)" >&2
    exit 1
fi

printf '%s\n' "${selected[@]}"

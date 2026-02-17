#!/usr/bin/env bash
# Dockerfile validation script
# Checks for common mismatches between base images and package managers

set -e

DOCKERFILE="${1:-Dockerfile}"

if [ ! -f "$DOCKERFILE" ]; then
    echo "Error: $DOCKERFILE not found"
    exit 1
fi

echo "Checking $DOCKERFILE for base image / package manager mismatches..."

checking_stage=false
current_stage=""
stage_base=""

while IFS= read -r line; do
    if echo "$line" | grep -qE "^FROM\s+"; then
        checking_stage=true
        current_stage="$line"
        stage_base="unknown"

        if echo "$line" | grep -qi "alpine"; then
            stage_base="alpine"
        elif echo "$line" | grep -qiE "debian|ubuntu|trixie|bookworm|bullseye"; then
            stage_base="debian"
        fi
    fi

    if [ "$checking_stage" = true ] && [ "$stage_base" = "alpine" ]; then
        if echo "$line" | grep -qE "(apt-get|apt)\s+(install|update)" || echo "$line" | grep -qE "xx-apt\s+"; then
            echo "❌ ERROR: Found Alpine-based image with Debian package manager (apt/apt-get)"
            echo "   Stage: $current_stage"
            echo "   Command: $line"
            echo "   Fix: Use 'apk add' or 'xx-apk' instead"
            exit 1
        fi
    fi

    if [ "$checking_stage" = true ] && [ "$stage_base" = "debian" ]; then
        if echo "$line" | grep -qE "apk\s+(add|update|del)" || echo "$line" | grep -qE "xx-apk\s+"; then
            echo "❌ ERROR: Found Debian-based image with Alpine package manager (apk)"
            echo "   Stage: $current_stage"
            echo "   Command: $line"
            echo "   Fix: Use 'apt-get' or 'xx-apt' instead"
            exit 1
        fi
    fi
done < "$DOCKERFILE"

echo "✓ Dockerfile validation passed"
exit 0

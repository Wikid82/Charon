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

# Read file content
dockerfile_content=$(cat "$DOCKERFILE")

# Check for golang:latest or golang:1.x (Debian) with apk commands in the same stage
while IFS= read -r line; do
    if echo "$line" | grep -qE "^FROM\s+golang:(latest|[0-9]+(\.[0-9]+)?)\s"; then
        # Found a Debian-based golang image, check the next 20 lines for apk
        current_stage="$line"
        checking_stage=true
    elif echo "$line" | grep -qE "^FROM\s+" && [ "$checking_stage" = true ]; then
        # New FROM statement, reset
        checking_stage=false
    fi

    if [ "$checking_stage" = true ] && echo "$line" | grep -qE "RUN.*apk\s+(add|update|del)"; then
        echo "❌ ERROR: Found Debian-based golang image with Alpine package manager (apk)"
        echo "   Stage: $current_stage"
        echo "   Command: $line"
        echo "   Fix: Use 'golang:alpine' or 'golang:1.x-alpine' instead"
        exit 1
    fi
done < "$DOCKERFILE"

# Check for node:latest or node:XX (Debian) with apk commands
if echo "$dockerfile_content" | grep -E "FROM\s+node:(latest|[0-9]+)\s" > /dev/null; then
    if echo "$dockerfile_content" | grep -A 10 "FROM\s\+node:(latest|[0-9]\+)\s" | grep -E "RUN.*apk\s+(add|update)" > /dev/null; then
        echo "❌ ERROR: Found Debian-based node image (node:latest or node:XX) with Alpine package manager (apk)"
        echo "   Fix: Use 'node:alpine' or 'node:XX-alpine' instead"
        exit 1
    fi
fi

# Check for alpine images with apt/apt-get
if echo "$dockerfile_content" | grep -E "FROM\s+.*:.*alpine" > /dev/null; then
    if echo "$dockerfile_content" | grep -A 10 "FROM\s\+.*:.*alpine" | grep -E "RUN.*(apt-get|apt)\s+(install|update)" > /dev/null; then
        echo "❌ ERROR: Found Alpine-based image with Debian package manager (apt/apt-get)"
        echo "   Fix: Use 'apk add' instead of 'apt install'"
        exit 1
    fi
fi

echo "✓ Dockerfile validation passed"
exit 0

#!/usr/bin/env bash
# Skill runner for utility-update-go-version
# Updates local Go installation to match go.work requirements

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

GO_WORK_FILE="$PROJECT_ROOT/go.work"

if [[ ! -f "$GO_WORK_FILE" ]]; then
    echo "❌ go.work not found at $GO_WORK_FILE"
    exit 1
fi

# Extract required Go version from go.work
REQUIRED_VERSION=$(grep -E '^go [0-9]+\.[0-9]+(\.[0-9]+)?$' "$GO_WORK_FILE" | awk '{print $2}')

if [[ -z "$REQUIRED_VERSION" ]]; then
    echo "❌ Could not parse Go version from go.work"
    exit 1
fi

echo "📋 Required Go version from go.work: $REQUIRED_VERSION"

# Check current installed version
CURRENT_VERSION=$(go version 2>/dev/null | grep -oE 'go[0-9]+\.[0-9]+(\.[0-9]+)?' | sed 's/go//' || echo "none")
echo "📋 Currently installed Go version: $CURRENT_VERSION"

if [[ "$CURRENT_VERSION" == "$REQUIRED_VERSION" ]]; then
    echo "✅ Go version already matches requirement ($REQUIRED_VERSION)"
    exit 0
fi

echo "🔄 Updating Go from $CURRENT_VERSION to $REQUIRED_VERSION..."

# Download the new Go version using the official dl tool
echo "📥 Downloading Go $REQUIRED_VERSION..."
# Exception: golang.org/dl requires @latest to resolve the versioned shim.
# Compensating controls: REQUIRED_VERSION is pinned in go.work, and the dl tool
# downloads the official Go release for that exact version.
go install "golang.org/dl/go${REQUIRED_VERSION}@latest"

# Download the SDK
echo "📦 Installing Go $REQUIRED_VERSION SDK..."
"go${REQUIRED_VERSION}" download

# Update the system symlink
SDK_PATH="$HOME/sdk/go${REQUIRED_VERSION}/bin/go"
if [[ -f "$SDK_PATH" ]]; then
    echo "🔗 Updating system Go symlink..."
    sudo ln -sf "$SDK_PATH" /usr/local/go/bin/go
else
    echo "⚠️  SDK binary not found at expected path: $SDK_PATH"
    echo "   You may need to add go${REQUIRED_VERSION} to your PATH manually"
fi

# Verify the update
NEW_VERSION=$(go version 2>/dev/null | grep -oE 'go[0-9]+\.[0-9]+(\.[0-9]+)?' | sed 's/go//' || echo "unknown")
echo ""
echo "✅ Go updated successfully!"
echo "   Previous: $CURRENT_VERSION"
echo "   Current:  $NEW_VERSION"
echo "   Required: $REQUIRED_VERSION"

if [[ "$NEW_VERSION" != "$REQUIRED_VERSION" ]]; then
    echo ""
    echo "⚠️  Warning: Installed version ($NEW_VERSION) doesn't match required ($REQUIRED_VERSION)"
    echo "   You may need to restart your terminal or IDE"
fi

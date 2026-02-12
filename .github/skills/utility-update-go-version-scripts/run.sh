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

# Phase 1: Rebuild critical development tools with new Go version
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔧 Rebuilding development tools with Go $REQUIRED_VERSION..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# List of critical tools to rebuild
TOOLS=(
    "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    "golang.org/x/tools/gopls@latest"
    "golang.org/x/vuln/cmd/govulncheck@latest"
)

FAILED_TOOLS=()

for tool in "${TOOLS[@]}"; do
    tool_name=$(basename "$(dirname "$tool")")
    echo "📦 Installing $tool_name..."

    if go install "$tool" 2>&1; then
        echo "✅ $tool_name installed successfully"
    else
        echo "❌ Failed to install $tool_name"
        FAILED_TOOLS+=("$tool_name")
    fi
    echo ""
done

if [ ${#FAILED_TOOLS[@]} -eq 0 ]; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "✅ All tools rebuilt successfully!"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
else
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "⚠️  Some tools failed to install:"
    for tool in "${FAILED_TOOLS[@]}"; do
        echo "   - $tool"
    done
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "You can manually rebuild tools later with:"
    echo "  ./scripts/rebuild-go-tools.sh"
fi
